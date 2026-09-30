package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const testActor = "logn-admin@example-project.iam.gserviceaccount.com"

func manualAction(p paidTrack, userID, reason string) ManualLicenseAction {
	return ManualLicenseAction{
		UserID: userID, TrackID: p.id, Reason: reason,
		Evidence: "dois aparelhos ativos na mesma semana", Actor: testActor,
	}
}

func appeal(p paidTrack, userID, outcome string) ManualLicenseAction {
	return ManualLicenseAction{
		UserID: userID, TrackID: p.id, Outcome: outcome,
		Evidence: "contestação recebida", Actor: testActor,
	}
}

// boughtTrack é uma conta com a trilha comprada, pronta para revogar.
func boughtTrack(t *testing.T, conn *pgxpool.Pool, repo *Repository) (paidTrack, string, string) {
	t.Helper()
	p := seedPaidTrack(t, conn)
	a := seedUser(t, conn)
	txID := "test-" + testUUID(t)[:12]
	revokedCleanup(t, conn, txID)
	if _, err := repo.GrantEntitlement(context.Background(), grant(p, a, a, txID, false)); err != nil {
		t.Fatalf("compra: %v", err)
	}
	return p, a, txID
}

// must(t)(repo.X(...)) falha o teste se a ação deu erro.
func must(t *testing.T) func(LicenseNotice, error) {
	return func(_ LicenseNotice, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func active(t *testing.T, repo *Repository, p paidTrack, userID string, want bool) {
	t.Helper()
	err := repo.CheckEntitlement(context.Background(), userID, p.id)
	if want && err != nil {
		t.Fatalf("licença devia estar ativa: %v", err)
	}
	if !want && !errors.Is(err, ErrEntitlementRequired) {
		t.Fatalf("licença devia estar fora: %v", err)
	}
}

func TestManualRevocationBlocksRestore(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	p, a, txID := boughtTrack(t, conn, repo)
	b := seedUser(t, conn)

	notice, err := repo.RevokeManually(ctx, manualAction(p, a, ReasonRedistribution))
	if err != nil {
		t.Fatal(err)
	}
	if notice.Email == "" || notice.TrackName != "Trilha T" || notice.Reason != ReasonRedistribution || notice.Outcome != "" {
		t.Fatalf("aviso: %+v", notice)
	}
	active(t, repo, p, a, false)
	if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, true)); !errors.Is(err, ErrTransactionRevoked) {
		t.Fatalf("restauração na mesma conta: %v", err)
	}
	if _, err := conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GrantEntitlement(ctx, grant(p, b, a, txID, true)); !errors.Is(err, ErrTransactionRevoked) {
		t.Fatalf("restauração em conta nova depois de excluir a antiga: %v", err)
	}
}

// Compartilhamento: a licença volta enquanto analisamos, e a decisão fecha a análise.
func TestAccountSharingAppealReviewThenDecide(t *testing.T) {
	for _, decision := range []string{AppealAccepted, AppealRejected} {
		t.Run(decision, func(t *testing.T) {
			conn := setupTestDB(t)
			t.Cleanup(conn.Close)
			repo := NewRepository(conn)
			ctx := context.Background()
			p, a, txID := boughtTrack(t, conn, repo)

			must(t)(repo.RevokeManually(ctx, manualAction(p, a, ReasonAccountSharing)))
			if _, err := repo.AnswerAppeal(ctx, appeal(p, a, AppealRejected)); !errors.Is(err, ErrAppealOutOfOrder) {
				t.Fatalf("recusa antes da análise: %v", err)
			}
			must(t)(repo.AnswerAppeal(ctx, appeal(p, a, AppealReview)))
			active(t, repo, p, a, true)
			// Em análise, reenviar a própria compra não diz "revogada".
			if _, err := repo.GrantEntitlement(ctx, grant(p, a, a, txID, false)); err != nil {
				t.Fatalf("reenvio durante a análise: %v", err)
			}
			if _, err := repo.RevokeManually(ctx, manualAction(p, a, ReasonAccountSharing)); !errors.Is(err, ErrAppealOutOfOrder) {
				t.Fatalf("revogação nova com a análise aberta: %v", err)
			}

			notice, err := repo.AnswerAppeal(ctx, appeal(p, a, decision))
			if err != nil {
				t.Fatalf("decisão depois da análise: %v", err)
			}
			if notice.Outcome != decision || notice.Reason != ReasonAccountSharing {
				t.Fatalf("aviso: %+v", notice)
			}
			active(t, repo, p, a, decision == AppealAccepted)

			var actions int
			conn.QueryRow(ctx, `SELECT count(*) FROM license_actions WHERE user_id = $1 AND actor = $2`, a, testActor).Scan(&actions)
			if actions != 3 {
				t.Fatalf("histórico com %d ações, esperava 3", actions)
			}
		})
	}
}

// Redistribuição: a licença segue fora durante a análise; a contestação é confirmada, e
// a decisão sai com aviso.
func TestRedistributionAppealStaysRevokedUntilDecided(t *testing.T) {
	for _, decision := range []string{AppealAccepted, AppealRejected} {
		t.Run(decision, func(t *testing.T) {
			conn := setupTestDB(t)
			t.Cleanup(conn.Close)
			repo := NewRepository(conn)
			ctx := context.Background()
			p, a, _ := boughtTrack(t, conn, repo)

			must(t)(repo.RevokeManually(ctx, manualAction(p, a, ReasonRedistribution)))
			if _, err := repo.AnswerAppeal(ctx, appeal(p, a, AppealReview)); !errors.Is(err, ErrAppealOutOfOrder) {
				t.Fatalf("redistribuição devolvida durante a análise: %v", err)
			}
			must(t)(repo.AnswerAppeal(ctx, appeal(p, a, AppealReceived)))
			active(t, repo, p, a, false)
			must(t)(repo.AnswerAppeal(ctx, appeal(p, a, decision)))
			active(t, repo, p, a, decision == AppealAccepted)
		})
	}
}

// Um reembolso chegou com a licença revogada à mão: a contestação aceita não a devolve a
// quem foi reembolsado, e reverter o reembolso não desfaz a revogação manual.
func TestARefundDuringAManualRevocationKeepsTheTrackClosed(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	p, a, txID := boughtTrack(t, conn, repo)

	must(t)(repo.RevokeManually(ctx, manualAction(p, a, ReasonAccountSharing)))
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AnswerAppeal(ctx, appeal(p, a, AppealReview)); !errors.Is(err, ErrStillRevokedByStore) {
		t.Fatalf("análise com reembolso valendo: %v", err)
	}
	active(t, repo, p, a, false)

	if err := repo.ReinstateRefund(ctx, ProviderAppleStoreKit, txID, 2000); err != nil {
		t.Fatal(err)
	}
	active(t, repo, p, a, false)
	notice, err := repo.AnswerAppeal(ctx, appeal(p, a, AppealAccepted))
	if err != nil || notice.StoreRevoked {
		t.Fatalf("aceite depois da reversão: %+v %v", notice, err)
	}
	active(t, repo, p, a, true)
}

// Compartilhamento revogado e depois reembolsado: a licença não pode voltar para a
// análise, e a contestação ainda tem resposta, recusada ou aceita, sem mentir no aviso.
func TestAnAppealStillGetsAnAnswerAfterARefund(t *testing.T) {
	for _, decision := range []string{AppealAccepted, AppealRejected} {
		t.Run(decision, func(t *testing.T) {
			conn := setupTestDB(t)
			t.Cleanup(conn.Close)
			repo := NewRepository(conn)
			ctx := context.Background()
			p, a, txID := boughtTrack(t, conn, repo)

			must(t)(repo.RevokeManually(ctx, manualAction(p, a, ReasonAccountSharing)))
			if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
				t.Fatal(err)
			}
			notice, err := repo.AnswerAppeal(ctx, appeal(p, a, decision))
			if err != nil || !notice.StoreRevoked {
				t.Fatalf("resposta com reembolso valendo: %+v %v", notice, err)
			}
			active(t, repo, p, a, false)

			// A Apple reverte o reembolso: volta só se a contestação foi aceita.
			if err := repo.ReinstateRefund(ctx, ProviderAppleStoreKit, txID, 2000); err != nil {
				t.Fatal(err)
			}
			active(t, repo, p, a, decision == AppealAccepted)
		})
	}
}

// Análise, reembolso, recusa, reversão do reembolso: a revogação nossa segue valendo, e
// uma contestação aceita depois ainda devolve a licença.
func TestARefundReversedAfterARejectionLeavesTheAppealOpen(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	p, a, txID := boughtTrack(t, conn, repo)

	must(t)(repo.RevokeManually(ctx, manualAction(p, a, ReasonAccountSharing)))
	must(t)(repo.AnswerAppeal(ctx, appeal(p, a, AppealReview)))
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, "refund", 1000); err != nil {
		t.Fatal(err)
	}
	active(t, repo, p, a, false)
	must(t)(repo.AnswerAppeal(ctx, appeal(p, a, AppealRejected)))
	if err := repo.ReinstateRefund(ctx, ProviderAppleStoreKit, txID, 2000); err != nil {
		t.Fatal(err)
	}
	active(t, repo, p, a, false)
	must(t)(repo.AnswerAppeal(ctx, appeal(p, a, AppealAccepted)))
	active(t, repo, p, a, true)
}

func TestManualActionsRefuseWhatTheTermsDoNotAllow(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	p, a, txID := boughtTrack(t, conn, repo)
	stranger := seedUser(t, conn)

	for name, act := range map[string]ManualLicenseAction{
		"motivo da loja":         manualAction(p, a, "refund"),
		"motivo desconhecido":    manualAction(p, a, "fraud"),
		"resultado na revogação": {UserID: a, TrackID: p.id, Reason: ReasonRedistribution, Outcome: AppealAccepted, Evidence: "x", Actor: testActor},
		"sem evidência":          {UserID: a, TrackID: p.id, Reason: ReasonRedistribution, Evidence: "  ", Actor: testActor},
		"sem autor":              {UserID: a, TrackID: p.id, Reason: ReasonRedistribution, Evidence: "x", Actor: ""},
	} {
		if _, err := repo.RevokeManually(ctx, act); !errors.Is(err, ErrInvalidLicenseAction) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, act := range map[string]ManualLicenseAction{
		"resultado desconhecido": appeal(p, a, "maybe"),
		"motivo na contestação":  {UserID: a, TrackID: p.id, Reason: ReasonRedistribution, Outcome: AppealAccepted, Evidence: "x", Actor: testActor},
	} {
		if _, err := repo.AnswerAppeal(ctx, act); !errors.Is(err, ErrInvalidLicenseAction) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := repo.RevokeManually(ctx, manualAction(p, stranger, ReasonRedistribution)); !errors.Is(err, ErrNoActiveEntitlement) {
		t.Errorf("conta sem a trilha: %v", err)
	}
	if err := repo.RevokeTransaction(ctx, ProviderAppleStoreKit, txID, ReasonRedistribution, 1000); !errors.Is(err, ErrInvalidLicenseAction) {
		t.Errorf("motivo manual pela rota da loja: %v", err)
	}
	if _, err := repo.AnswerAppeal(ctx, appeal(p, a, AppealAccepted)); !errors.Is(err, ErrNotManuallyRevoked) {
		t.Errorf("contestação sem revogação: %v", err)
	}

	must(t)(repo.RevokeManually(ctx, manualAction(p, a, ReasonRedistribution)))
	if _, err := repo.RevokeManually(ctx, manualAction(p, a, ReasonAccountSharing)); !errors.Is(err, ErrAppealOutOfOrder) {
		t.Errorf("revogar duas vezes: %v", err)
	}
	active(t, repo, p, a, false)
}

// O aviso sai na língua do último aceite, com o nome da trilha nela.
func TestTheNoticeUsesTheLanguageOfTheLastAcceptance(t *testing.T) {
	conn := setupTestDB(t)
	t.Cleanup(conn.Close)
	repo := NewRepository(conn)
	ctx := context.Background()
	p, a, _ := boughtTrack(t, conn, repo)

	if _, err := conn.Exec(ctx, `
		INSERT INTO legal_acceptances (user_id, kind, version, locale, created_at) VALUES
		($1, 'terms', 1, 'pt-BR', now() - interval '1 day'), ($1, 'terms', 2, 'es', now())`, a); err != nil {
		t.Fatal(err)
	}
	notice, err := repo.RevokeManually(ctx, manualAction(p, a, ReasonRedistribution))
	if err != nil {
		t.Fatal(err)
	}
	if notice.Locale != "es" || notice.TrackName != "Pista T" {
		t.Fatalf("aviso: %+v", notice)
	}
}
