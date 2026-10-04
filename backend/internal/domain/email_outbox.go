package domain

import (
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Caixa de saída de e-mail (ADR 0026). O e-mail que um pedido manda é gravado em
// `email_outbox` na mesma transação da mudança que o pede. Depois do commit, o id vai
// para o OutboxTracker do pedido, e o middleware da camada HTTP põe na fila do Cloud
// Tasks o que o handler deixou ali, antes de a resposta terminar. A fila chama a rota
// interna que envia, e a varredura de cinco em cinco minutos põe na fila o que ficou
// para trás.

// OutboxTracker junta os e-mails que um pedido gravou, para irem à fila quando ele
// terminar. Um por pedido, no `context`.
type OutboxTracker struct {
	mu  sync.Mutex
	ids []string
}

type outboxTrackerKey struct{}

// WithOutboxTracker devolve um `context` com um OutboxTracker novo.
func WithOutboxTracker(ctx context.Context) (context.Context, *OutboxTracker) {
	t := &OutboxTracker{}
	return context.WithValue(ctx, outboxTrackerKey{}, t), t
}

// TrackOutbox anota no OutboxTracker do pedido um e-mail já gravado. Só se chama depois
// do commit: transação que volta não pode ter posto nada na fila. Sem OutboxTracker no
// `context` (teste, rotina fora de pedido), não faz nada, e a varredura leva o e-mail.
func TrackOutbox(ctx context.Context, id string) {
	if t, ok := ctx.Value(outboxTrackerKey{}).(*OutboxTracker); ok {
		t.mu.Lock()
		t.ids = append(t.ids, id)
		t.mu.Unlock()
	}
}

// Drain devolve o que foi anotado e esvazia o OutboxTracker.
func (t *OutboxTracker) Drain() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	ids := t.ids
	t.ids = nil
	return ids
}

// Tipos de e-mail da caixa de saída.
const (
	OutboxOTP      = "otp"
	OutboxWelcome  = "welcome"
	OutboxWaitlist = "waitlist"
)

// OutboxRetention é quanto uma linha fica depois de criada, enviada ou não. Serve para
// responder "esse e-mail saiu?"; a purga diária apaga as mais velhas.
const OutboxRetention = 7 * 24 * time.Hour

// OutboxOTPRetention é o mesmo para os e-mails de código, que guardam o endereço em
// claro: qualquer um pede código para qualquer endereço, inclusive sem conta, e uma
// semana de endereços alheios é dado demais para responder se um código saiu.
const OutboxOTPRetention = 24 * time.Hour

// OutboxStuckAge é quanto uma linha pendente que já virou tarefa espera um desfecho.
// As cinco tentativas da fila cabem em uns oito minutos (cloud_tasks.tf); passado isto,
// a fila desistiu sem a rota gravar nada (erro de banco, 429, timeout do Cloud Run), e a
// varredura dá a linha por perdida.
const OutboxStuckAge = time.Hour

// OutboxSweepAge é quanto uma linha pendente espera virar tarefa antes de a varredura
// pô-la na fila. Menos que isso, quem atendeu o pedido ainda pode estar enfileirando.
const OutboxSweepAge = time.Minute

// outboxSweepBatch é quantas linhas uma varredura põe na fila. O resto fica para a
// próxima, cinco minutos depois.
const outboxSweepBatch = 100

// outboxErrorClip é o tamanho de `last_error`.
const outboxErrorClip = 255

// OutboxMessage é um e-mail pronto para sair: o destinatário já resolvido e, no de
// código, o código já aberto.
type OutboxMessage struct {
	ID     string
	Kind   string
	Locale string
	Email  string
	// Só no de código.
	Purpose string
	Code    string
	// Só no da lista de espera: os links do e-mail saem dele.
	WaitlistID string
}

// OutboxOutcome é o que aconteceu com uma entrega.
type OutboxOutcome int

const (
	// Enviado agora.
	OutboxSent OutboxOutcome = iota
	// Nada a enviar: a linha sumiu, já saiu, ou o e-mail perdeu o sentido (código
	// vencido ou trocado, conta excluída, inscrição confirmada).
	OutboxSkipped
	// O envio falhou e a fila tenta de novo.
	OutboxRetry
	// O envio falhou na última tentativa. A linha fica como `failed`.
	OutboxFailed
)

// outboxAAD amarra o código cifrado ao endereço e ao propósito: copiado para outra
// linha, não abre.
func outboxAAD(email, purpose string) []byte {
	return []byte("email_outbox\x00otp\x00" + email + "\x00" + purpose)
}

// outboxCodeKey é a chave do código guardado na caixa de saída. Sai de TRACK_KEY_SECRET
// por HKDF, com rótulo próprio: nenhum segredo novo no Secret Manager, e quem tem uma
// chave derivada não chega à outra nem à de `track_keys`.
func outboxCodeKey() ([]byte, error) {
	secret, err := trackKeySecret()
	if err != nil {
		return nil, err
	}
	return hkdf.Key(sha256.New, secret, nil, "logn/email-outbox/v1", 32)
}

func sealOutboxCode(email, purpose, code string) ([]byte, error) {
	key, err := outboxCodeKey()
	if err != nil {
		return nil, err
	}
	return seal(key, []byte(code), outboxAAD(email, purpose))
}

func openOutboxCode(email, purpose string, sealed []byte) (string, error) {
	key, err := outboxCodeKey()
	if err != nil {
		return "", err
	}
	plain, err := open(key, sealed, outboxAAD(email, purpose))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// queueOTPTx põe o e-mail de código na caixa de saída, com o código cifrado.
func queueOTPTx(ctx context.Context, tx pgx.Tx, email, purpose, code, lang string) (string, error) {
	sealed, err := sealOutboxCode(email, purpose, code)
	if err != nil {
		return "", err
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO email_outbox (kind, locale, email, purpose, code_ciphertext)
		VALUES ('otp', $1, $2, $3, $4)
		RETURNING id`, lang, email, purpose, sealed).Scan(&id)
	return id, err
}

// queueWelcomeTx põe as boas-vindas da conta na caixa de saída.
func queueWelcomeTx(ctx context.Context, tx pgx.Tx, userID, lang string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO email_outbox (kind, locale, user_id) VALUES ('welcome', $1, $2)
		RETURNING id`, lang, userID).Scan(&id)
	return id, err
}

// queueWaitlistTx põe a confirmação da inscrição na caixa de saída.
func queueWaitlistTx(ctx context.Context, tx pgx.Tx, waitlistID, lang string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO email_outbox (kind, locale, waitlist_id) VALUES ('waitlist', $1, $2)
		RETURNING id`, lang, waitlistID).Scan(&id)
	return id, err
}

// DeliverOutbox envia uma linha da caixa de saída, se ela ainda estiver pendente.
//
// A linha fica travada (`FOR UPDATE`) do começo ao fim, com o envio dentro: a mesma
// tarefa entregue duas vezes pelo Cloud Tasks espera a primeira e encontra a linha já
// enviada. Se o processo morrer no meio, a transação volta, a linha continua pendente e
// a fila tenta de novo; o pior caso é o e-mail sair duas vezes.
//
// `send` recebe o e-mail pronto. O erro que ela devolve vai para `last_error`, então
// não pode trazer endereço. `final` diz que é a última tentativa da fila: falhando, a
// linha vira `failed` e a inscrição da lista de espera volta a aceitar novo envio.
func (r *Repository) DeliverOutbox(ctx context.Context, id string, final bool, send func(OutboxMessage) error) (OutboxOutcome, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	m := OutboxMessage{ID: id}
	var status string
	var userID, waitlistID, email, purpose *string
	var sealed []byte
	err = tx.QueryRow(ctx, `
		SELECT kind, status, locale, user_id::text, waitlist_id::text, email, purpose, code_ciphertext
		FROM email_outbox WHERE id = $1
		FOR UPDATE`, id).Scan(&m.Kind, &status, &m.Locale, &userID, &waitlistID, &email, &purpose, &sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		// Apagada pela poda, pela exclusão da conta ou pela saída da lista.
		return OutboxSkipped, nil
	}
	if err != nil {
		return 0, err
	}
	if status != "pending" {
		return OutboxSkipped, nil
	}

	live, reason, err := resolveOutboxTx(ctx, tx, &m, userID, waitlistID, email, purpose, sealed)
	if err != nil {
		return 0, err
	}
	if !live {
		if _, err := tx.Exec(ctx, `
			UPDATE email_outbox SET status = 'skipped', code_ciphertext = NULL, last_error = NULLIF($2, '')
			WHERE id = $1`, id, reason); err != nil {
			return 0, err
		}
		return OutboxSkipped, tx.Commit(ctx)
	}

	if sendErr := send(m); sendErr != nil {
		msg := sendErr.Error()
		if len(msg) > outboxErrorClip {
			msg = msg[:outboxErrorClip]
		}
		if !final {
			if _, err := tx.Exec(ctx, `
				UPDATE email_outbox SET attempts = attempts + 1, last_error = $2 WHERE id = $1`,
				id, msg); err != nil {
				return 0, err
			}
			return OutboxRetry, tx.Commit(ctx)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE email_outbox
			SET status = 'failed', attempts = attempts + 1, last_error = $2, code_ciphertext = NULL
			WHERE id = $1`, id, msg); err != nil {
			return 0, err
		}
		// A inscrição volta a aceitar envio: o próximo pedido dela manda outro e-mail.
		if m.Kind == OutboxWaitlist {
			if _, err := tx.Exec(ctx, `
				UPDATE waitlist_entries SET sent_at = NULL WHERE id = $1 AND confirmed_at IS NULL`,
				m.WaitlistID); err != nil {
				return 0, err
			}
		}
		return OutboxFailed, tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE email_outbox
		SET status = 'sent', sent_at = CURRENT_TIMESTAMP, attempts = attempts + 1,
		    last_error = NULL, code_ciphertext = NULL
		WHERE id = $1`, id); err != nil {
		return 0, err
	}
	return OutboxSent, tx.Commit(ctx)
}

// resolveOutboxTx completa o e-mail com o destinatário e, no de código, com o código
// aberto. Devolve `false` quando ele perdeu o sentido e não deve sair, com o motivo
// quando não é o caso comum (código trocado ou usado, conta excluída, inscrição
// confirmada).
func resolveOutboxTx(ctx context.Context, tx pgx.Tx, m *OutboxMessage, userID, waitlistID, email, purpose *string, sealed []byte) (bool, string, error) {
	switch m.Kind {
	case OutboxOTP:
		m.Email, m.Purpose = *email, *purpose
		code, err := openOutboxCode(m.Email, m.Purpose, sealed)
		if err != nil {
			// TRACK_KEY_SECRET trocada desde a gravação: este código não sai nunca mais, e
			// a pessoa pede outro. O motivo fica na linha, para a rotação não descartar
			// códigos sem rastro.
			return false, "o código não abre com a chave atual", nil
		}
		// Só sai o código que ainda vale. Pedido outro, o `code_hash` já é o do novo;
		// usado ou vencido, a linha sumiu ou passou do prazo.
		var current bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM otps
				WHERE email = $1 AND purpose = $2 AND code_hash = $3
				  AND expires_at > CURRENT_TIMESTAMP
			)`, m.Email, m.Purpose, otpHash(m.Email, m.Purpose, code)).Scan(&current)
		if err != nil || !current {
			return false, "", err
		}
		m.Code = code
		return true, "", nil
	case OutboxWelcome:
		// Conta que pediu exclusão não recebe boas-vindas.
		err := tx.QueryRow(ctx, `
			SELECT email FROM users WHERE id = $1 AND deletion_requested_at IS NULL`,
			*userID).Scan(&m.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil
		}
		return err == nil, "", err
	case OutboxWaitlist:
		// Confirmada por outro caminho, a inscrição não precisa mais do link.
		m.WaitlistID = *waitlistID
		err := tx.QueryRow(ctx, `
			SELECT email FROM waitlist_entries WHERE id = $1 AND confirmed_at IS NULL`,
			m.WaitlistID).Scan(&m.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil
		}
		return err == nil, "", err
	}
	return false, "tipo desconhecido", nil
}

// MarkOutboxEnqueued registra que a linha virou tarefa no Cloud Tasks. A varredura
// deixa de olhar para ela.
func (r *Repository) MarkOutboxEnqueued(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE email_outbox SET enqueued_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND enqueued_at IS NULL`, id)
	return err
}

// UnqueuedOutbox devolve as linhas pendentes que nunca viraram tarefa, as mais velhas
// primeiro.
//
// Antes, desiste dos códigos que passaram do prazo sem sair: eles não serviriam mais, e
// o código cifrado não fica guardado à espera da poda.
func (r *Repository) UnqueuedOutbox(ctx context.Context) ([]string, error) {
	if _, err := r.db.Exec(ctx, `
		UPDATE email_outbox SET status = 'skipped', code_ciphertext = NULL
		WHERE kind = 'otp' AND status = 'pending'
		  AND created_at <= CURRENT_TIMESTAMP - make_interval(secs => $1)`,
		OTPValidity.Seconds()); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id FROM email_outbox
		WHERE status = 'pending' AND enqueued_at IS NULL
		  AND created_at <= CURRENT_TIMESTAMP - make_interval(secs => $1)
		ORDER BY created_at
		LIMIT $2`, OutboxSweepAge.Seconds(), outboxSweepBatch)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// ExpireStuckOutbox dá por perdidas as linhas pendentes que viraram tarefa há mais de
// OutboxStuckAge: a fila já desistiu delas sem que a rota gravasse o desfecho. Elas
// viram `failed`, como na última tentativa, e a inscrição da lista de espera volta a
// aceitar envio; sem isto, quem se inscreveu ficava sem o link e sem como pedir outro
// até a inscrição vencer. Devolve quantas.
//
// Pôr de novo na fila não serve: o Cloud Tasks guarda o nome de uma tarefa já
// executada por um tempo, e a nova seria recusada como repetida.
func (r *Repository) ExpireStuckOutbox(ctx context.Context) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		UPDATE email_outbox
		SET status = 'failed', code_ciphertext = NULL, last_error = 'a fila desistiu sem desfecho'
		WHERE status = 'pending' AND enqueued_at <= CURRENT_TIMESTAMP - make_interval(secs => $1)
		RETURNING waitlist_id::text`, OutboxStuckAge.Seconds())
	if err != nil {
		return 0, err
	}
	waitlistIDs, err := pgx.CollectRows(rows, pgx.RowTo[*string])
	if err != nil {
		return 0, err
	}
	for _, id := range waitlistIDs {
		if id == nil {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE waitlist_entries SET sent_at = NULL WHERE id = $1 AND confirmed_at IS NULL`, *id); err != nil {
			return 0, err
		}
	}
	return int64(len(waitlistIDs)), tx.Commit(ctx)
}

// PruneOutbox apaga as linhas criadas há mais de OutboxRetention, e as de código há
// mais de OutboxOTPRetention, e devolve quantas.
func (r *Repository) PruneOutbox(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM email_outbox
		WHERE created_at <= CURRENT_TIMESTAMP - make_interval(secs => $1)
		   OR (kind = 'otp' AND created_at <= CURRENT_TIMESTAMP - make_interval(secs => $2))`,
		OutboxRetention.Seconds(), OutboxOTPRetention.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
