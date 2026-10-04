// Package tasks põe os e-mails da caixa de saída na fila do Cloud Tasks (ADR 0026).
//
// A tarefa leva só o id da linha de `email_outbox`. Endereço e código ficam no banco,
// e a rota interna que a fila chama lê de lá.
//
// A chamada é a REST, à mão, e não o cliente gerado de `google.golang.org/api`: o
// gerado puxa um módulo que o servidor ainda não tinha, e dependência nova é ADR. O
// token sai de `golang.org/x/oauth2/google`, que já estava no grafo.
package tasks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"golang.org/x/oauth2/google"
)

// Config é a fila e a rota que ela chama.
type Config struct {
	// O nome completo da fila: projects/P/locations/L/queues/Q.
	Queue string
	// A URL da rota interna de envio, na URL .run.app (como o Scheduler, ADR 0021).
	TargetURL string
	// A audiência do token OIDC que a fila manda: a mesma que o servidor confere em
	// CLOUD_SCHEDULER_AUDIENCE.
	Audience string
	// A conta de serviço que assina o token. A rota de envio aceita só ela.
	ServiceAccount string
}

// DefaultEndpoint é a API do Cloud Tasks.
const DefaultEndpoint = "https://cloudtasks.googleapis.com"

var queuePattern = regexp.MustCompile(`^projects/[a-z][a-z0-9-]{4,28}[a-z0-9]/locations/[a-z0-9-]+/queues/[A-Za-z0-9-]{1,100}$`)

// CloudTasks cria as tarefas. No Cloud Run, a credencial é a da conta do serviço, pelo
// servidor de metadados.
type CloudTasks struct {
	client   *http.Client
	endpoint string
	cfg      Config
}

// New confere a configuração e monta o cliente com a credencial padrão do ambiente.
func New(ctx context.Context, cfg Config) (*CloudTasks, error) {
	if err := cfg.check(); err != nil {
		return nil, err
	}
	client, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, err
	}
	return &CloudTasks{client: client, endpoint: DefaultEndpoint, cfg: cfg}, nil
}

// NewWithClient é New com o cliente HTTP e o endereço da API dados: é o teste que
// aponta para um servidor falso.
func NewWithClient(cfg Config, client *http.Client, endpoint string) (*CloudTasks, error) {
	if err := cfg.check(); err != nil {
		return nil, err
	}
	return &CloudTasks{client: client, endpoint: endpoint, cfg: cfg}, nil
}

func (cfg Config) check() error {
	if !queuePattern.MatchString(cfg.Queue) {
		return fmt.Errorf("fila do Cloud Tasks fora do formato projects/P/locations/L/queues/Q: %q", cfg.Queue)
	}
	if cfg.TargetURL == "" || cfg.Audience == "" || cfg.ServiceAccount == "" {
		return errors.New("fila do Cloud Tasks sem URL de destino, audiência ou conta de serviço")
	}
	return nil
}

type createTaskRequest struct {
	Task task `json:"task"`
}

type task struct {
	Name        string      `json:"name"`
	HTTPRequest httpRequest `json:"httpRequest"`
}

type httpRequest struct {
	URL        string            `json:"url"`
	HTTPMethod string            `json:"httpMethod"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	OIDCToken  oidcToken         `json:"oidcToken"`
}

type oidcToken struct {
	ServiceAccountEmail string `json:"serviceAccountEmail"`
	Audience            string `json:"audience"`
}

// Enqueue cria a tarefa da linha `outboxID`.
//
// O nome da tarefa sai do id, e o Cloud Tasks recusa um segundo com o mesmo nome: a
// varredura que põe de novo na fila uma linha que o pedido já tinha posto recebe 409, e
// isso conta como sucesso.
func (c *CloudTasks) Enqueue(ctx context.Context, outboxID string) error {
	body, err := json.Marshal(map[string]string{"id": outboxID})
	if err != nil {
		return err
	}
	payload, err := json.Marshal(createTaskRequest{Task: task{
		Name: c.cfg.Queue + "/tasks/outbox-" + outboxID,
		HTTPRequest: httpRequest{
			URL:        c.cfg.TargetURL,
			HTTPMethod: http.MethodPost,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       base64.StdEncoding.EncodeToString(body),
			OIDCToken: oidcToken{
				ServiceAccountEmail: c.cfg.ServiceAccount,
				Audience:            c.cfg.Audience,
			},
		},
	}})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.endpoint+"/v2/"+c.cfg.Queue+"/tasks", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK, resp.StatusCode == http.StatusConflict:
		io.Copy(io.Discard, resp.Body)
		return nil
	default:
		// O corpo de erro da API é dela, sem dado do pedido; vai cortado.
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("cloud tasks respondeu %d: %s", resp.StatusCode, bytes.TrimSpace(msg))
	}
}
