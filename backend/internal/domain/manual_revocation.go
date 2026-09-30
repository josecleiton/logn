package domain

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/josecleiton/logn/backend/internal/locale"
)

// Revogação manual de licença e a contestação dela (ADR 0021): os casos 3 e 4 da seção
// 10.5 dos termos. O bloqueio mora em `manual_revocations`, separado do da loja em
// `revoked_transactions`, e cada um sai só pelo seu caminho. A licença só volta quando
// nenhum dos dois vale.
//
// O ciclo, pela seção 10.5:
//
//	revoke   licença ativa → bloqueio `revoked`, licença fora.
//	received redistribuição: confirma a contestação, a licença segue fora.
//	review   compartilhamento: a licença volta enquanto analisamos, bloqueio `review`.
//	accepted o bloqueio sai e a licença volta, com o progresso.
//	rejected o bloqueio volta a `revoked` e a licença sai de novo, se tinha voltado.

const (
	ReasonRedistribution = "redistribution"
	ReasonAccountSharing = "account_sharing"

	AppealReceived = "received"
	AppealReview   = "review"
	AppealAccepted = "accepted"
	AppealRejected = "rejected"

	maxEvidenceLen = 2000
	maxActorLen    = 320
)

var (
	ErrInvalidLicenseAction = errors.New("invalid manual licence action")
	ErrNoActiveEntitlement  = errors.New("account has no active licence for this track")
	ErrNotManuallyRevoked   = errors.New("licence has no open manual revocation")
	ErrAppealOutOfOrder     = errors.New("appeal outcome does not follow the current state")
	ErrStillRevokedByStore  = errors.New("transaction is still revoked by the store")
	ErrNoStoreRecord        = errors.New("licence has no store transaction on record")
)

// ManualLicenseAction é o pedido de revogar a licença de uma conta ou de responder à
// contestação dela.
type ManualLicenseAction struct {
	UserID  string
	TrackID string
	// Só na revogação; na contestação vale o motivo gravado.
	Reason string
	// Só na contestação: AppealReceived, AppealReview, AppealAccepted, AppealRejected.
	Outcome  string
	Evidence string
	// A conta de serviço que chamou, tirada do token, nunca do corpo.
	Actor string
}

// LicenseNotice é o que o e-mail de aviso precisa, lido na mesma transação da mudança.
// `Outcome` vazio é a revogação.
type LicenseNotice struct {
	Email     string
	Locale    string
	TrackName string
	Reason    string
	Outcome   string
	// A loja também revogou a transação: a licença segue fora qualquer que seja a
	// resposta, e o aviso não pode dizer que ela voltou.
	StoreRevoked bool
}

func manualReason(reason string) bool {
	return reason == ReasonRedistribution || reason == ReasonAccountSharing
}

func (a ManualLicenseAction) common() (ManualLicenseAction, error) {
	a.Evidence = strings.TrimSpace(a.Evidence)
	a.Actor = strings.TrimSpace(a.Actor)
	if a.UserID == "" || a.TrackID == "" || a.Evidence == "" || a.Actor == "" ||
		len([]rune(a.Evidence)) > maxEvidenceLen || len(a.Actor) > maxActorLen {
		return a, ErrInvalidLicenseAction
	}
	return a, nil
}

// entitlementTransaction acha a transação da licença da conta na trilha, com o
// provedor que a vendeu. Desde a 0066 a licença guarda o provedor; antes ele saía do
// registro da compra, e `GrantEntitlement` usava o da Apple fixo (ADR 0021).
//
// Licença sem registro da compra ao lado é dado montado à mão, e a revogação recusa:
// o bloqueio manual é da transação, e sem registro não há de onde tirar que ela existe.
func entitlementTransaction(ctx context.Context, tx pgx.Tx, userID, trackID string) (provider, originalTransactionID string, err error) {
	var recorded bool
	err = tx.QueryRow(ctx, `
		SELECT e.provider, e.original_transaction_id,
		       EXISTS (SELECT 1 FROM store_transactions st
		               WHERE st.provider = e.provider AND st.original_transaction_id = e.original_transaction_id)
		FROM entitlements e
		WHERE e.user_id = $1 AND e.track_id = $2`,
		userID, trackID).Scan(&provider, &originalTransactionID, &recorded)
	if err != nil {
		return "", "", err
	}
	if !recorded {
		return "", "", ErrNoStoreRecord
	}
	return provider, originalTransactionID, nil
}

// RevokeManually revoga a licença ativa da conta na trilha, grava a evidência e devolve
// o que o aviso por e-mail precisa. Sem licença ativa, ou com uma revogação manual já
// aberta, não muda nada.
func (r *Repository) RevokeManually(ctx context.Context, a ManualLicenseAction) (LicenseNotice, error) {
	a, err := a.common()
	if err != nil {
		return LicenseNotice{}, err
	}
	if !manualReason(a.Reason) || a.Outcome != "" {
		return LicenseNotice{}, ErrInvalidLicenseAction
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return LicenseNotice{}, err
	}
	defer tx.Rollback(ctx)

	provider, otid, err := entitlementTransaction(ctx, tx, a.UserID, a.TrackID)
	if errors.Is(err, pgx.ErrNoRows) {
		return LicenseNotice{}, ErrNoActiveEntitlement
	}
	if err != nil {
		return LicenseNotice{}, err
	}
	if err := lockTransaction(ctx, tx, provider, otid); err != nil {
		return LicenseNotice{}, err
	}

	// Com uma contestação em análise, a licença está ativa e o bloqueio aberto: a
	// decisão sai pela contestação, não por uma revogação nova.
	var open bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM manual_revocations WHERE provider = $1 AND original_transaction_id = $2)`,
		provider, otid).Scan(&open); err != nil {
		return LicenseNotice{}, err
	}
	if open {
		return LicenseNotice{}, ErrAppealOutOfOrder
	}

	// Conferido depois da trava: uma restauração ou um reembolso pode ter mudado a
	// licença entre a leitura de cima e aqui.
	tag, err := tx.Exec(ctx, `
		UPDATE entitlements SET status = 'revoked', revoked_reason = $4
		WHERE user_id = $1 AND track_id = $2 AND original_transaction_id = $3 AND status = 'active'`,
		a.UserID, a.TrackID, otid, a.Reason)
	if err != nil {
		return LicenseNotice{}, err
	}
	if tag.RowsAffected() == 0 {
		return LicenseNotice{}, ErrNoActiveEntitlement
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO manual_revocations (provider, original_transaction_id, reason) VALUES ($1, $2, $3)`,
		provider, otid, a.Reason); err != nil {
		return LicenseNotice{}, err
	}
	if err := insertLicenseAction(ctx, tx, a, provider, otid, "revoke", a.Reason); err != nil {
		return LicenseNotice{}, err
	}
	notice, err := licenseNotice(ctx, tx, a.UserID, a.TrackID, a.Reason, "")
	if err != nil {
		return LicenseNotice{}, err
	}
	return notice, tx.Commit(ctx)
}

// AnswerAppeal registra a resposta à contestação de uma revogação manual e muda a
// licença como a seção 10.5 manda. Se a loja também revogou a transação (um reembolso),
// a licença não volta por aqui: `review` é recusado, e `accepted` fecha a contestação
// deixando a licença fora pelo motivo da loja.
func (r *Repository) AnswerAppeal(ctx context.Context, a ManualLicenseAction) (LicenseNotice, error) {
	a, err := a.common()
	if err != nil {
		return LicenseNotice{}, err
	}
	switch a.Outcome {
	case AppealReceived, AppealReview, AppealAccepted, AppealRejected:
	default:
		return LicenseNotice{}, ErrInvalidLicenseAction
	}
	if a.Reason != "" {
		return LicenseNotice{}, ErrInvalidLicenseAction
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return LicenseNotice{}, err
	}
	defer tx.Rollback(ctx)

	provider, otid, err := entitlementTransaction(ctx, tx, a.UserID, a.TrackID)
	if errors.Is(err, pgx.ErrNoRows) {
		return LicenseNotice{}, ErrNotManuallyRevoked
	}
	if err != nil {
		return LicenseNotice{}, err
	}
	if err := lockTransaction(ctx, tx, provider, otid); err != nil {
		return LicenseNotice{}, err
	}

	var reason, status string
	err = tx.QueryRow(ctx, `
		SELECT reason, status FROM manual_revocations WHERE provider = $1 AND original_transaction_id = $2`,
		provider, otid).Scan(&reason, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return LicenseNotice{}, ErrNotManuallyRevoked
	}
	if err != nil {
		return LicenseNotice{}, err
	}

	// O bloqueio da loja que vale agora, se houver (um reembolso). Ele manda na licença:
	// a contestação responde à revogação nossa, e a licença não volta por ela.
	var storeReason *string
	if err := tx.QueryRow(ctx, `
		SELECT reason FROM revoked_transactions
		WHERE provider = $1 AND original_transaction_id = $2 AND reversed_at_ms IS NULL`,
		provider, otid).Scan(&storeReason); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return LicenseNotice{}, err
	}
	storeRevoked := storeReason != nil

	// A licença volta durante a análise só no compartilhamento de conta; na
	// redistribuição ela segue fora, e a contestação só é confirmada.
	switch a.Outcome {
	case AppealReceived:
		if reason != ReasonRedistribution || status != "revoked" {
			return LicenseNotice{}, ErrAppealOutOfOrder
		}
	case AppealReview:
		if reason != ReasonAccountSharing || status != "revoked" {
			return LicenseNotice{}, ErrAppealOutOfOrder
		}
		if storeRevoked {
			return LicenseNotice{}, ErrStillRevokedByStore
		}
	case AppealRejected:
		// Compartilhamento: depois da análise, com a licença de volta. Se a loja revogou
		// e a licença não pode voltar para a análise, a recusa sai direto.
		if reason == ReasonAccountSharing && status != "review" && !storeRevoked {
			return LicenseNotice{}, ErrAppealOutOfOrder
		}
	}

	switch a.Outcome {
	case AppealReview:
		if err := setManualStatus(ctx, tx, provider, otid, "review"); err != nil {
			return LicenseNotice{}, err
		}
		if err := activate(ctx, tx, a, otid, reason); err != nil {
			return LicenseNotice{}, err
		}
	case AppealAccepted:
		if _, err := tx.Exec(ctx, `
			DELETE FROM manual_revocations WHERE provider = $1 AND original_transaction_id = $2`,
			provider, otid); err != nil {
			return LicenseNotice{}, err
		}
		switch {
		case storeRevoked:
			// A contestação foi aceita, mas a compra foi reembolsada: a licença segue
			// fora, agora pelo motivo da loja, e volta se a Apple reverter o reembolso.
			if _, err := tx.Exec(ctx, `
				UPDATE entitlements SET status = 'revoked', revoked_reason = $4
				WHERE user_id = $1 AND track_id = $2 AND original_transaction_id = $3`,
				a.UserID, a.TrackID, otid, *storeReason); err != nil {
				return LicenseNotice{}, err
			}
		case status == "revoked":
			if err := activate(ctx, tx, a, otid, reason); err != nil {
				return LicenseNotice{}, err
			}
		}
		// Em análise e sem reembolso, a licença já tinha voltado.
	case AppealRejected:
		if status == "review" {
			if err := setManualStatus(ctx, tx, provider, otid, "revoked"); err != nil {
				return LicenseNotice{}, err
			}
			// A licença voltou na análise. Se a loja a revogou nesse meio-tempo, ela já
			// está fora, com o motivo da loja, e fica assim.
			if _, err := tx.Exec(ctx, `
				UPDATE entitlements SET status = 'revoked', revoked_reason = $4
				WHERE user_id = $1 AND track_id = $2 AND original_transaction_id = $3 AND status = 'active'`,
				a.UserID, a.TrackID, otid, reason); err != nil {
				return LicenseNotice{}, err
			}
		}
	}

	if err := insertLicenseAction(ctx, tx, a, provider, otid, "appeal", reason); err != nil {
		return LicenseNotice{}, err
	}
	notice, err := licenseNotice(ctx, tx, a.UserID, a.TrackID, reason, a.Outcome)
	if err != nil {
		return LicenseNotice{}, err
	}
	notice.StoreRevoked = storeRevoked
	return notice, tx.Commit(ctx)
}

func setManualStatus(ctx context.Context, tx pgx.Tx, provider, otid, status string) error {
	_, err := tx.Exec(ctx, `
		UPDATE manual_revocations SET status = $3 WHERE provider = $1 AND original_transaction_id = $2`,
		provider, otid, status)
	return err
}

// activate devolve a licença revogada pelo motivo manual. Se ela não estava revogada
// por esse motivo, algo mudou por fora, e nada é gravado.
func activate(ctx context.Context, tx pgx.Tx, a ManualLicenseAction, otid, reason string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE entitlements SET status = 'active', revoked_reason = NULL
		WHERE user_id = $1 AND track_id = $2 AND original_transaction_id = $3
		  AND status = 'revoked' AND revoked_reason = $4`,
		a.UserID, a.TrackID, otid, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAppealOutOfOrder
	}
	return nil
}

func insertLicenseAction(ctx context.Context, tx pgx.Tx, a ManualLicenseAction, provider, otid, action, reason string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO license_actions
			(user_id, track_id, provider, original_transaction_id, action, reason, outcome, evidence, actor)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9)`,
		a.UserID, a.TrackID, provider, otid, action, reason, a.Outcome, a.Evidence, a.Actor)
	return err
}

// licenseNotice lê o destinatário e a língua do aviso. A conta não guarda língua; a do
// último aceite dos termos é a que a pessoa usava no app. O nome da trilha sai nessa
// língua, ou em português se ela não tiver tradução.
func licenseNotice(ctx context.Context, tx pgx.Tx, userID, trackID, reason, outcome string) (LicenseNotice, error) {
	n := LicenseNotice{Reason: reason, Outcome: outcome, Locale: locale.Default}
	var lang *string
	if err := tx.QueryRow(ctx, `
		SELECT u.email,
		       (SELECT la.locale FROM legal_acceptances la
		        WHERE la.user_id = u.id ORDER BY la.created_at DESC LIMIT 1)
		FROM users u WHERE u.id = $1`, userID).Scan(&n.Email, &lang); err != nil {
		return LicenseNotice{}, err
	}
	if lang != nil {
		if matched, ok := locale.Match(*lang); ok {
			n.Locale = matched
		}
	}
	if err := tx.QueryRow(ctx, `
		SELECT name FROM track_translations
		WHERE track_id = $1 AND locale IN ($2, $3)
		ORDER BY (locale = $2) DESC LIMIT 1`,
		trackID, n.Locale, locale.Default).Scan(&n.TrackName); err != nil {
		return LicenseNotice{}, err
	}
	return n, nil
}
