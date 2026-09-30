package domain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Lista de espera do iPhone (ADR 0022). Quem deixa o e-mail na landing recebe um link de
// confirmação, e só a inscrição confirmada recebe o aviso de lançamento.

// WaitlistPendingTTL é quanto a inscrição não confirmada vive, contado da criação. A
// purga diária a apaga depois disso. Contar do último envio deixava quem reenviasse
// manter a linha viva para sempre.
const WaitlistPendingTTL = 7 * 24 * time.Hour

// WaitlistHourlySendCap é o teto de e-mails de confirmação por hora, somando todos os
// endereços. O rate limit por IP some trocando de IP, e o formulário não pode virar
// disparador de e-mail contra endereço dos outros nem gastar a cota do SMTP. Acima dele
// o pedido recebe a página de erro, e nada é gravado. É variável só para o teste.
var WaitlistHourlySendCap = 100

// ErrWaitlistBusy é o teto de envios por hora atingido.
var ErrWaitlistBusy = errors.New("waitlist send cap reached")

// Ações que um link do e-mail autoriza. Cada uma tem a sua assinatura: o link de
// confirmação não serve para tirar ninguém da lista, e o de saída não confirma.
const (
	WaitlistActionConfirm = "confirm"
	WaitlistActionLeave   = "leave"
)

// WaitlistEntry é o que a página do link precisa saber da inscrição.
type WaitlistEntry struct {
	ID        string
	Locale    string
	Confirmed bool
}

// ErrWaitlistNotFound é o link de uma inscrição que não existe mais: saiu da lista,
// foi purgada ou nunca existiu.
var ErrWaitlistNotFound = errors.New("waitlist entry not found")

// WaitlistToken é o token do link de uma ação: o id da inscrição e o HMAC do id com a
// ação. O e-mail não vai no link, que passa pelos registros de pedidos da hospedagem, e
// eles não guardam e-mail (política, seção 9).
//
// O token não vence sozinho: morre com a linha. A pendente some em WaitlistPendingTTL, e
// sair da lista a apaga.
func WaitlistToken(id, action string) string {
	return id + "." + waitlistMAC(id, action)
}

// ParseWaitlistToken devolve o id de um token da ação, ou `false` se ele não for dela.
func ParseWaitlistToken(token, action string) (string, bool) {
	id, mac, ok := strings.Cut(token, ".")
	if !ok || !isUUID(id) {
		return "", false
	}
	return id, hmac.Equal([]byte(mac), []byte(waitlistMAC(id, action)))
}

// A chave é a do JWT, como no OTP: quem tem uma já tem a outra. O prefixo separa este
// uso dos outros da mesma chave.
func waitlistMAC(id, action string) string {
	mac := hmac.New(sha256.New, JwtSecretKey)
	mac.Write([]byte("waitlist\x00" + action + "\x00" + id))
	return hex.EncodeToString(mac.Sum(nil))
}

// isUUID confere a forma canônica, em minúsculas, que o Postgres devolve.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
				return false
			}
		}
	}
	return true
}

// JoinWaitlist inscreve o e-mail, já normalizado, e diz se é para mandar a confirmação.
//
// Devolve o id e `true` para inscrição nova, e para pendente cujo e-mail não saiu
// (ReleaseWaitlistSend); a língua passa a ser a do pedido novo. A pendente recebe um
// e-mail só: pedir de novo não reenvia, senão quem reenviasse todo dia mandaria um
// e-mail por dia a um endereço que não é dele. Para inscrição já confirmada, pendente
// já avisada, ou vencida, devolve `false` e não muda nada. Quem chama responde igual em
// todos os casos, para a rota não dizer quem está na lista.
//
// Com WaitlistHourlySendCap atingido, devolve ErrWaitlistBusy antes de gravar.
func (r *Repository) JoinWaitlist(ctx context.Context, email, locale string) (string, bool, error) {
	var sent int
	if err := r.db.QueryRow(ctx, `
		SELECT count(*) FROM waitlist_entries WHERE sent_at > CURRENT_TIMESTAMP - interval '1 hour'`).Scan(&sent); err != nil {
		return "", false, err
	}
	if sent >= WaitlistHourlySendCap {
		return "", false, ErrWaitlistBusy
	}

	// A pendente vencida que a purga ainda não levou sai agora, e o pedido vira
	// inscrição nova. Sem isto, quem voltasse no oitavo dia não receberia nada até a
	// purga do dia seguinte. É no máximo um e-mail a cada WaitlistPendingTTL por endereço.
	if _, err := r.db.Exec(ctx, `
		DELETE FROM waitlist_entries
		WHERE email = $1 AND confirmed_at IS NULL
		  AND created_at <= CURRENT_TIMESTAMP - make_interval(secs => $2)`,
		email, WaitlistPendingTTL.Seconds()); err != nil {
		return "", false, err
	}

	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO waitlist_entries (email, locale) VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET locale = EXCLUDED.locale, sent_at = CURRENT_TIMESTAMP
		WHERE waitlist_entries.confirmed_at IS NULL
		  AND waitlist_entries.sent_at IS NULL
		  AND waitlist_entries.created_at > CURRENT_TIMESTAMP - make_interval(secs => $3)
		RETURNING id`,
		email, locale, WaitlistPendingTTL.Seconds()).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// ReleaseWaitlistSend marca que o e-mail de uma inscrição pendente não saiu, para o
// pedido seguinte tentar de novo.
func (r *Repository) ReleaseWaitlistSend(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE waitlist_entries SET sent_at = NULL WHERE id = $1 AND confirmed_at IS NULL`, id)
	return err
}

// GetWaitlistEntry lê a inscrição de um link. Devolve ErrWaitlistNotFound se ela não
// existir, ou se for pendente vencida que a purga ainda não levou.
func (r *Repository) GetWaitlistEntry(ctx context.Context, id string) (WaitlistEntry, error) {
	e := WaitlistEntry{ID: id}
	err := r.db.QueryRow(ctx, `
		SELECT locale, confirmed_at IS NOT NULL FROM waitlist_entries
		WHERE id = $1
		  AND (confirmed_at IS NOT NULL OR created_at > CURRENT_TIMESTAMP - make_interval(secs => $2))`,
		id, WaitlistPendingTTL.Seconds()).Scan(&e.Locale, &e.Confirmed)
	if errors.Is(err, pgx.ErrNoRows) {
		return WaitlistEntry{}, ErrWaitlistNotFound
	}
	return e, err
}

// ConfirmWaitlist confirma a inscrição. Confirmar de novo não muda a data da primeira
// vez. Pendente vencida não confirma: o link dela morreu com o prazo.
func (r *Repository) ConfirmWaitlist(ctx context.Context, id string) (string, error) {
	var locale string
	err := r.db.QueryRow(ctx, `
		UPDATE waitlist_entries
		SET confirmed_at = COALESCE(confirmed_at, CURRENT_TIMESTAMP)
		WHERE id = $1
		  AND (confirmed_at IS NOT NULL OR created_at > CURRENT_TIMESTAMP - make_interval(secs => $2))
		RETURNING locale`,
		id, WaitlistPendingTTL.Seconds()).Scan(&locale)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrWaitlistNotFound
	}
	return locale, err
}

// LeaveWaitlist apaga a inscrição e devolve a língua dela, para a página de saída.
func (r *Repository) LeaveWaitlist(ctx context.Context, id string) (string, error) {
	var locale string
	err := r.db.QueryRow(ctx, `DELETE FROM waitlist_entries WHERE id = $1 RETURNING locale`, id).Scan(&locale)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrWaitlistNotFound
	}
	return locale, err
}

// PurgePendingWaitlist apaga as inscrições não confirmadas criadas há mais de
// WaitlistPendingTTL, e devolve quantas.
func (r *Repository) PurgePendingWaitlist(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM waitlist_entries
		WHERE confirmed_at IS NULL AND created_at <= CURRENT_TIMESTAMP - make_interval(secs => $1)`,
		WaitlistPendingTTL.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
