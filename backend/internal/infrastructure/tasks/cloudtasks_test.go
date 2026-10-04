package tasks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

var testConfig = Config{
	Queue:          "projects/example-project/locations/southamerica-east1/queues/email",
	TargetURL:      "https://logn-abc.a.run.app/api/v1/internal/email/send",
	Audience:       "https://logn-abc.a.run.app",
	ServiceAccount: "logn-tasks@example-project.iam.gserviceaccount.com",
}

// A tarefa leva só o id, com nome tirado dele, e o token OIDC da conta da fila.
func TestEnqueueBuildsTheTask(t *testing.T) {
	var got createTaskRequest
	var path string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("corpo: %v", err)
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()

	c, err := NewWithClient(testConfig, api.Client(), api.URL)
	if err != nil {
		t.Fatal(err)
	}
	const id = "0b8e7c9a-1f2d-4e3b-9a8c-7d6e5f4a3b2c"
	if err := c.Enqueue(context.Background(), id); err != nil {
		t.Fatal(err)
	}

	if path != "/v2/"+testConfig.Queue+"/tasks" {
		t.Errorf("caminho %s", path)
	}
	if got.Task.Name != testConfig.Queue+"/tasks/outbox-"+id {
		t.Errorf("nome %s", got.Task.Name)
	}
	h := got.Task.HTTPRequest
	if h.URL != testConfig.TargetURL || h.HTTPMethod != http.MethodPost {
		t.Errorf("destino %s %s", h.HTTPMethod, h.URL)
	}
	if h.OIDCToken.ServiceAccountEmail != testConfig.ServiceAccount || h.OIDCToken.Audience != testConfig.Audience {
		t.Errorf("token %+v", h.OIDCToken)
	}
	body, _ := base64.StdEncoding.DecodeString(h.Body)
	if string(body) != `{"id":"`+id+`"}` {
		t.Errorf("corpo da tarefa %s", body)
	}
}

// Nome repetido (409) é a tarefa que já existe: sucesso. Outro erro volta.
func TestEnqueueStatuses(t *testing.T) {
	for status, wantErr := range map[int]bool{
		http.StatusOK:                  false,
		http.StatusConflict:            false,
		http.StatusForbidden:           true,
		http.StatusServiceUnavailable:  true,
		http.StatusTooManyRequests:     true,
		http.StatusInternalServerError: true,
	} {
		api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(`{"error":{"message":"x"}}`))
		}))
		c, _ := NewWithClient(testConfig, api.Client(), api.URL)
		err := c.Enqueue(context.Background(), "0b8e7c9a-1f2d-4e3b-9a8c-7d6e5f4a3b2c")
		api.Close()
		if (err != nil) != wantErr {
			t.Errorf("status %d: err=%v", status, err)
		}
	}
}

func TestConfigCheck(t *testing.T) {
	for name, mutate := range map[string]func(*Config){
		"fila sem formato": func(c *Config) { c.Queue = "email" },
		"fila com caminho": func(c *Config) { c.Queue = testConfig.Queue + "/../x" },
		"sem destino":      func(c *Config) { c.TargetURL = "" },
		"sem audiência":    func(c *Config) { c.Audience = "" },
		"sem conta":        func(c *Config) { c.ServiceAccount = "" },
	} {
		cfg := testConfig
		mutate(&cfg)
		if _, err := NewWithClient(cfg, http.DefaultClient, DefaultEndpoint); err == nil {
			t.Errorf("%s: aceitou", name)
		}
	}
	if _, err := NewWithClient(testConfig, http.DefaultClient, DefaultEndpoint); err != nil {
		t.Errorf("configuração válida recusada: %v", err)
	}
}
