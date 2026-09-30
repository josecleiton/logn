package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
	"github.com/josecleiton/logn/backend/internal/googleplay"
)

// purchaseRequest é a compra de uma das lojas. A da App Store manda só `jws`, como
// sempre mandou; a do Google Play manda `provider`, `product_id` e `purchase_token`
// (ADR 0022).
type purchaseRequest struct {
	JWS           string `json:"jws"`
	Provider      string `json:"provider"`
	ProductID     string `json:"product_id"`
	PurchaseToken string `json:"purchase_token"`
}

// PlayVerifier é o cliente da Google Play Developer API (`internal/googleplay`),
// trocado nos testes.
type PlayVerifier interface {
	Product(ctx context.Context, productID, purchaseToken string) (googleplay.ProductPurchase, error)
	Acknowledge(ctx context.Context, productID, purchaseToken string) error
	Voided(ctx context.Context, since time.Time) ([]googleplay.VoidedPurchase, error)
}

type purchaseResponse struct {
	TrackID string `json:"track_id"`
}

// purchaseHandler liga uma compra recém-feita à conta. A transação tem de trazer no
// `appAccountToken` o id de quem manda.
func (s *Server) purchaseHandler(w http.ResponseWriter, r *http.Request) {
	s.grantPurchase(w, r, false)
}

// restorePurchaseHandler é o "Restaurar compras": aceita transação de uma conta que já
// não está ativa, e recusa a de outra conta ativa.
func (s *Server) restorePurchaseHandler(w http.ResponseWriter, r *http.Request) {
	s.grantPurchase(w, r, true)
}

func (s *Server) grantPurchase(w http.ResponseWriter, r *http.Request, restore bool) {
	userID, ok := s.authenticate(w, r)
	if !ok {
		return
	}

	var req purchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	var grant domain.PurchaseGrant
	var play *googleplay.ProductPurchase
	switch req.Provider {
	case "", domain.ProviderAppleStoreKit:
		if req.JWS == "" {
			writeError(w, http.StatusBadRequest, codeInvalidRequest)
			return
		}
		tx, err := s.storekit.VerifyTransaction(req.JWS)
		if err != nil {
			writeError(w, http.StatusBadRequest, codePurchaseInvalid)
			return
		}
		grant = domain.PurchaseGrant{
			Provider:              domain.ProviderAppleStoreKit,
			ProductID:             tx.ProductID,
			OriginalTransactionID: tx.OriginalTransactionID,
			TransactionID:         tx.TransactionID,
			Environment:           tx.Environment,
			AppAccountToken:       tx.AppAccountToken,
			RawPayload:            req.JWS,
		}
	case domain.ProviderGooglePlay:
		var ok bool
		grant, play, ok = s.verifyPlayPurchase(w, r, req)
		if !ok {
			return
		}
	default:
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}
	grant.UserID = userID
	grant.Restore = restore

	trackID, err := s.repo.GrantEntitlement(r.Context(), grant)
	switch {
	case errors.Is(err, domain.ErrUnknownProduct):
		writeError(w, http.StatusBadRequest, codeUnknownProduct)
		return
	case errors.Is(err, domain.ErrAccountMismatch):
		writeError(w, http.StatusForbidden, codePurchaseAccountMismatch)
		return
	case errors.Is(err, domain.ErrOwnedByOtherAccount):
		writeError(w, http.StatusConflict, codePurchaseOwnedByOtherAccount)
		return
	case errors.Is(err, domain.ErrTransactionRevoked):
		writeError(w, http.StatusForbidden, codePurchaseRevoked)
		return
	case err != nil:
		log.Printf("compra não gravada: user=%s loja=%s produto=%s erro=%v", userID, grant.Provider, grant.ProductID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	// A licença gravada, a compra do Play é reconhecida. Sem reconhecimento em 3 dias o
	// Play estorna sozinho. Se falhar aqui, a resposta é erro e o app manda de novo: a
	// licença já existe, e gravar de novo não muda nada.
	// Se o app não mandar de novo, o job diário reconhece o que ficou (playVoidedHandler).
	if play != nil && play.NeedsAcknowledge() {
		if err := s.acknowledgePlay(r.Context(), req.ProductID, req.PurchaseToken); err != nil {
			log.Printf("compra do Google Play não reconhecida: user=%s produto=%s erro=%v", userID, grant.ProductID, err)
			writeError(w, http.StatusBadGateway, codeInternal)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(purchaseResponse{TrackID: trackID})
}

// acknowledgePlay reconhece a compra. Se o reconhecimento falhar mas a loja já o tiver
// (dois pedidos ao mesmo tempo, ou uma resposta perdida), vale como feito.
func (s *Server) acknowledgePlay(ctx context.Context, productID, purchaseToken string) error {
	err := s.play.Acknowledge(ctx, productID, purchaseToken)
	if err == nil {
		return nil
	}
	if p, rerr := s.play.Product(ctx, productID, purchaseToken); rerr == nil && !p.NeedsAcknowledge() {
		return nil
	}
	return err
}

// verifyPlayPurchase confere a compra na Google Play Developer API e monta o que vira
// licença. Responde e devolve `false` quando ela não vale.
//
// O produto é conferido no banco antes de ir à loja: ele entra no caminho da URL da API,
// e só sai de lista fechada. A conta do `obfuscatedExternalAccountId` passa pela mesma
// regra do `appAccountToken` (GrantEntitlement).
func (s *Server) verifyPlayPurchase(w http.ResponseWriter, r *http.Request, req purchaseRequest) (domain.PurchaseGrant, *googleplay.ProductPurchase, bool) {
	if s.play == nil {
		writeError(w, http.StatusServiceUnavailable, codeStoreUnavailable)
		return domain.PurchaseGrant{}, nil, false
	}
	if !googleplay.ValidInput(req.ProductID, req.PurchaseToken) {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return domain.PurchaseGrant{}, nil, false
	}
	known, err := s.repo.IsPaidProduct(r.Context(), req.ProductID)
	if err != nil {
		log.Printf("produto não conferido: erro=%v", err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return domain.PurchaseGrant{}, nil, false
	}
	if !known {
		writeError(w, http.StatusBadRequest, codeUnknownProduct)
		return domain.PurchaseGrant{}, nil, false
	}

	p, err := s.play.Product(r.Context(), req.ProductID, req.PurchaseToken)
	if errors.Is(err, googleplay.ErrNotFound) || errors.Is(err, googleplay.ErrBadInput) {
		writeError(w, http.StatusBadRequest, codePurchaseInvalid)
		return domain.PurchaseGrant{}, nil, false
	}
	if err != nil {
		log.Printf("Google Play não respondeu: produto=%s erro=%v", req.ProductID, err)
		writeError(w, http.StatusBadGateway, codeInternal)
		return domain.PurchaseGrant{}, nil, false
	}
	if !p.ForProduct(req.ProductID) {
		log.Printf("compra do Google Play de outro produto: pedido=%s loja=%.150s quantidade=%d",
			req.ProductID, p.ProductID, p.Quantity)
		writeError(w, http.StatusBadRequest, codePurchaseInvalid)
		return domain.PurchaseGrant{}, nil, false
	}
	env, err := p.Environment()
	if errors.Is(err, googleplay.ErrPending) {
		writeError(w, http.StatusConflict, codePurchasePending)
		return domain.PurchaseGrant{}, nil, false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, codePurchaseInvalid)
		return domain.PurchaseGrant{}, nil, false
	}

	// O registro guarda o token cru e a resposta da loja, como o JWS da Apple: é o que
	// se mostra numa disputa. O token não vai para o log.
	raw, err := json.Marshal(struct {
		ProductID     string          `json:"product_id"`
		PurchaseToken string          `json:"purchase_token"`
		Purchase      json.RawMessage `json:"purchase"`
	}{req.ProductID, req.PurchaseToken, p.Raw})
	if err != nil {
		writeError(w, http.StatusInternalServerError, codeInternal)
		return domain.PurchaseGrant{}, nil, false
	}
	key := googleplay.TransactionKey(req.PurchaseToken)
	return domain.PurchaseGrant{
		Provider:              domain.ProviderGooglePlay,
		ProductID:             req.ProductID,
		OriginalTransactionID: key,
		TransactionID:         key,
		Environment:           env,
		AppAccountToken:       p.ObfuscatedExternalAccountID,
		RawPayload:            string(raw),
	}, &p, true
}

// appStoreNotificationRoute é a rota pública das notificações, com os tetos dela.
// Quem chama é a Apple, sem credencial nossa: a defesa é a assinatura, mais um teto
// de corpo e de ritmo para a rota não virar porta de DoS.
func (s *Server) appStoreNotificationRoute(rl *rateLimiter) http.HandlerFunc {
	return rl.wrap(limitBody(appStoreNotificationBodyLimit, s.appStoreNotificationHandler))
}

type appStoreNotificationRequest struct {
	SignedPayload string `json:"signedPayload"`
}

// appStoreNotificationHandler recebe as App Store Server Notifications V2.
//
// Erro de banco responde 500 para a Apple mandar de novo. A versão anterior engolia o
// erro e respondia 200, e o reembolso que caísse ali se perdia para sempre.
func (s *Server) appStoreNotificationHandler(w http.ResponseWriter, r *http.Request) {
	var req appStoreNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SignedPayload == "" {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	n, err := s.storekit.VerifyNotification(req.SignedPayload)
	if err != nil {
		writeError(w, http.StatusBadRequest, codePurchaseInvalid)
		return
	}

	switch n.NotificationType {
	case "REFUND", "REVOKE", "REFUND_REVERSED":
	default:
		// O resto (teste, pedido de consumo, renovação) não muda direito de trilha.
		w.WriteHeader(http.StatusOK)
		return
	}

	tx, err := s.storekit.VerifyNotificationTransaction(n.Data.SignedTransactionInfo)
	if err != nil {
		writeError(w, http.StatusBadRequest, codePurchaseInvalid)
		return
	}

	// A ordem entre notificações sai da hora assinada pela Apple, não da chegada.
	signedAt := n.SignedDate
	if signedAt == 0 {
		signedAt = time.Now().UnixMilli()
	}
	switch n.NotificationType {
	case "REFUND_REVERSED":
		err = s.repo.ReinstateRefund(r.Context(), domain.ProviderAppleStoreKit, tx.OriginalTransactionID, signedAt)
	case "REFUND":
		err = s.repo.RevokeTransaction(r.Context(), domain.ProviderAppleStoreKit, tx.OriginalTransactionID, "refund", signedAt)
	default:
		// REVOKE tem motivo próprio: não é reembolso, e um REFUND_REVERSED não o desfaz.
		err = s.repo.RevokeTransaction(r.Context(), domain.ProviderAppleStoreKit, tx.OriginalTransactionID, "store_revoke", signedAt)
	}
	if err != nil {
		log.Printf("notificação da App Store não aplicada: tipo=%s uuid=%s erro=%v", n.NotificationType, n.NotificationUUID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}
	w.WriteHeader(http.StatusOK)
}
