package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
)

// Caixa de saída de e-mail (ADR 0026). O domínio grava o e-mail na transação e anota o
// id no OutboxTracker do pedido; withOutbox põe na fila o que foi anotado quando o
// handler termina. A fila chama emailSendHandler, e o Cloud Scheduler chama
// emailSweepHandler de cinco em cinco minutos para o que ficou sem tarefa.

// envTasksAccount é a conta com que o Cloud Tasks assina a entrega (ADR 0021): a rota de
// envio aceita só ela.
const envTasksAccount = "CLOUD_TASKS_SERVICE_ACCOUNT"

// OutboxMaxAttempts é quantas vezes a fila tenta entregar uma tarefa. Tem de bater com
// `max_attempts` da fila no Terraform (cloud_tasks.tf): na última, a rota desiste da
// linha em vez de pedir outra tentativa que não vem.
const OutboxMaxAttempts = 5

// outboxDispatchTimeout é quanto o fim do pedido espera a fila aceitar cada tarefa.
// Passou disso, a varredura leva.
const outboxDispatchTimeout = 5 * time.Second

// smtpSendTimeout é quanto um envio pelo SMTP pode levar, na caixa de saída e no aviso
// de licença. O envio da caixa de saída roda com a linha travada e uma conexão do pool
// presa: um SMTP que segurasse a conexão prendia as duas, e com uma instância só o pool
// acabava para a API inteira. Passado o prazo, o Mailer fecha a conexão (ADR 0027) e a
// entrega conta como falha. Se o servidor já tinha aceitado a mensagem quando o prazo
// acabou, o e-mail sai duas vezes. É variável só para o teste.
var smtpSendTimeout = 20 * time.Second

// outboxSendBodyLimit é o teto do corpo da entrega: `{"id": "<uuid>"}`.
const outboxSendBodyLimit = 1 << 10

// OutboxQueue põe uma linha da caixa de saída na fila. É o cliente do Cloud Tasks; nulo,
// o envio sai na hora, dentro do pedido (desenvolvimento).
type OutboxQueue interface {
	Enqueue(ctx context.Context, outboxID string) error
}

// OutboxMailer manda o e-mail de código e as boas-vindas. É o Mailer; os testes trocam.
type OutboxMailer interface {
	SendOTP(ctx context.Context, toEmail, purpose, code, lang string) error
	SendWelcome(ctx context.Context, toEmail, lang string) error
}

var outboxIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// withOutbox dá a cada pedido um OutboxTracker e, quando o handler termina, põe na fila
// o que ele anotou.
//
// Isso acontece antes de a resposta acabar de sair, ainda dentro do pedido: o Cloud Run
// corta a CPU da instância quando a resposta termina, e foi assim que a goroutine de
// envio de antes podia nunca rodar. O cliente espera a fila aceitar a tarefa, coisa de
// dezenas de milissegundos.
func (s *Server) withOutbox(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, tracker := domain.WithOutboxTracker(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
		// O pedido pode ter sido cancelado pelo cliente; o e-mail já está gravado e sai
		// mesmo assim.
		for _, id := range tracker.Drain() {
			s.dispatchOutbox(context.WithoutCancel(ctx), id)
		}
	})
}

// dispatchOutbox põe a linha na fila e registra que ela virou tarefa. Sem fila, envia na
// hora. Falha só vai para o log: a linha continua pendente, e a varredura a leva.
// Devolve `false` se a fila (ou, sem ela, o envio) não aceitou a linha.
func (s *Server) dispatchOutbox(ctx context.Context, id string) bool {
	ctx, cancel := context.WithTimeout(ctx, outboxDispatchTimeout)
	defer cancel()
	if s.outboxQueue == nil {
		if _, err := s.deliverOutbox(ctx, id, true); err != nil {
			log.Printf("caixa de saída: envio direto falhou: id=%s erro=%v", id, err)
			return false
		}
		return true
	}
	if err := s.outboxQueue.Enqueue(ctx, id); err != nil {
		log.Printf("caixa de saída: tarefa não criada, fica para a varredura: id=%s erro=%v", id, err)
		return false
	}
	if err := s.repo.MarkOutboxEnqueued(ctx, id); err != nil {
		// A tarefa existe; a varredura vai tentar de novo e receber o 409 do nome repetido.
		log.Printf("caixa de saída: tarefa criada sem registro: id=%s erro=%v", id, err)
	}
	return true
}

// deliverOutbox envia a linha pelo mailer certo. O erro do SMTP vai para a linha sem
// endereço.
func (s *Server) deliverOutbox(ctx context.Context, id string, final bool) (domain.OutboxOutcome, error) {
	outcome, err := s.repo.DeliverOutbox(ctx, id, final, func(m domain.OutboxMessage) error {
		sendCtx, cancel := context.WithTimeout(ctx, smtpSendTimeout)
		defer cancel()
		err := s.sendOutbox(sendCtx, m)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, context.DeadlineExceeded):
			return errors.New("smtp sem resposta no prazo")
		}
		return errors.New(redactEmails(err))
	})
	switch {
	case err != nil:
	case outcome == domain.OutboxRetry:
		log.Printf("caixa de saída: envio falhou, a fila tenta de novo: id=%s", id)
	case outcome == domain.OutboxFailed:
		log.Printf("caixa de saída: envio desistido na última tentativa: id=%s", id)
	}
	return outcome, err
}

func (s *Server) sendOutbox(ctx context.Context, m domain.OutboxMessage) error {
	switch m.Kind {
	case domain.OutboxOTP:
		if s.outboxMailer == nil {
			return errors.New("mailer desligado")
		}
		return s.outboxMailer.SendOTP(ctx, m.Email, m.Purpose, m.Code, m.Locale)
	case domain.OutboxWelcome:
		if s.outboxMailer == nil {
			return errors.New("mailer desligado")
		}
		return s.outboxMailer.SendWelcome(ctx, m.Email, m.Locale)
	case domain.OutboxWaitlist:
		if s.waitlist == nil || s.waitlistMailer == nil {
			return errors.New("lista de espera desligada")
		}
		confirmURL := s.waitlist.link(domain.WaitlistActionConfirm, m.WaitlistID)
		leaveURL := s.waitlist.link(domain.WaitlistActionLeave, m.WaitlistID)
		return s.waitlistMailer.SendWaitlistConfirmation(ctx, m.Email, m.Locale, confirmURL, leaveURL)
	}
	return errors.New("tipo de e-mail desconhecido: " + m.Kind)
}

// emailSendHandler entrega uma linha da caixa de saída. Quem chama é o Cloud Tasks.
//
// Enviado, descartado ou desistido responde 200, e a fila esquece a tarefa. Falha do
// SMTP antes da última tentativa responde 503, e a fila tenta de novo com espera
// crescente. A tentativa vem de `X-CloudTasks-TaskRetryCount`, que só o Cloud Tasks
// manda a esta rota: o token é da conta dele.
//
//	@Summary		Entrega um e-mail da caixa de saída
//	@Description	Só com ID token OIDC do Google, da conta `CLOUD_TASKS_SERVICE_ACCOUNT` (ADR 0026). Erros saem em texto.
//	@Tags			internal
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		object{id=string}	true	"A linha de email_outbox"
//	@Success		200		{object}	object{status=string}
//	@Failure		400		{string}	string	"Bad request"
//	@Failure		401		{string}	string	"Unauthorized"
//	@Failure		403		{string}	string	"Forbidden"
//	@Failure		500		{string}	string	"Internal error"
//	@Failure		503		{string}	string	"Retry"
//	@Router			/api/v1/internal/email/send [post]
func (s *Server) emailSendHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.internalCaller(w, r, envTasksAccount); !ok {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || !outboxIDPattern.MatchString(req.ID) {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	retries, _ := strconv.Atoi(r.Header.Get("X-CloudTasks-TaskRetryCount"))
	final := retries+1 >= OutboxMaxAttempts

	outcome, err := s.deliverOutbox(r.Context(), req.ID, final)
	if err != nil {
		log.Printf("caixa de saída: entrega não gravada: id=%s erro=%v", req.ID, err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	status := map[domain.OutboxOutcome]string{
		domain.OutboxSent: "sent", domain.OutboxSkipped: "skipped",
		domain.OutboxRetry: "retry", domain.OutboxFailed: "failed",
	}[outcome]
	if outcome == domain.OutboxRetry {
		http.Error(w, "Retry", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

// emailSweepHandler põe na fila as linhas pendentes que nunca viraram tarefa: a fila
// estava fora do ar, ou a instância caiu entre o commit e o fim do pedido. Antes, dá por
// perdidas as que viraram tarefa e não tiveram desfecho (ExpireStuckOutbox). Quem chama
// é o Cloud Scheduler, de cinco em cinco minutos, com a conta da purga.
//
//	@Summary		Varredura da caixa de saída
//	@Description	Só com ID token OIDC do Google, da conta `CLOUD_SCHEDULER_SERVICE_ACCOUNT` (ADR 0026). Erros saem em texto.
//	@Tags			internal
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	object{status=string,dispatched=int}
//	@Failure		401	{string}	string	"Unauthorized"
//	@Failure		403	{string}	string	"Forbidden"
//	@Failure		500	{string}	string	"Internal error"
//	@Router			/api/v1/internal/email/sweep [post]
func (s *Server) emailSweepHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.internalCaller(w, r, envSchedulerAccount); !ok {
		return
	}
	// Antes de pôr na fila, dá desfecho ao que a fila largou sem desfecho.
	stuck, err := s.repo.ExpireStuckOutbox(r.Context())
	if err != nil {
		log.Printf("caixa de saída: varredura não fechou as perdidas: erro=%v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if stuck > 0 {
		log.Printf("caixa de saída: %d e-mails sem desfecho da fila dados por perdidos", stuck)
	}
	ids, err := s.repo.UnqueuedOutbox(r.Context())
	if err != nil {
		log.Printf("caixa de saída: varredura não leu as pendentes: erro=%v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	// A API do Cloud Tasks cria uma tarefa por chamada. Na primeira recusa a varredura
	// para: a fila provavelmente está fora do ar, e insistir nas outras, a até
	// outboxDispatchTimeout cada, passaria do prazo do pedido. O resto fica para a
	// próxima, cinco minutos depois.
	dispatched := 0
	for _, id := range ids {
		if !s.dispatchOutbox(r.Context(), id) {
			break
		}
		dispatched++
	}
	if len(ids) > 0 {
		log.Printf("caixa de saída: varredura levou %d de %d pendentes", dispatched, len(ids))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "dispatched": dispatched})
}
