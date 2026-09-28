package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
)

type purchaseRequest struct {
	JWS string `json:"jws"`
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.JWS == "" {
		writeError(w, http.StatusBadRequest, codeInvalidRequest)
		return
	}

	tx, err := s.storekit.VerifyTransaction(req.JWS)
	if err != nil {
		writeError(w, http.StatusBadRequest, codePurchaseInvalid)
		return
	}

	trackID, err := s.repo.GrantEntitlement(r.Context(), domain.PurchaseGrant{
		UserID:                userID,
		ProductID:             tx.ProductID,
		OriginalTransactionID: tx.OriginalTransactionID,
		TransactionID:         tx.TransactionID,
		Environment:           tx.Environment,
		AppAccountToken:       tx.AppAccountToken,
		RawPayload:            req.JWS,
		Restore:               restore,
	})
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
		log.Printf("compra não gravada: user=%s produto=%s erro=%v", userID, tx.ProductID, err)
		writeError(w, http.StatusInternalServerError, codeInternal)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(purchaseResponse{TrackID: trackID})
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
