package email

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/josecleiton/logn/backend/internal/domain"
)

// fakeSMTP é um servidor SMTP de teste. Anuncia AUTH PLAIN e guarda os comandos que
// recebeu; depois do DATA, o corpo vai para `data`. Sem `tlsConfig`, não fala TLS
// nenhum; com ele, cifra no aceite (`implicit`) ou oferece STARTTLS.
type fakeSMTP struct {
	t    *testing.T
	ln   net.Listener
	mu   sync.Mutex
	cmds []string
	data string
	// rcpt é a resposta ao RCPT TO; vazia, aceita.
	rcpt string
	// hangAfterData não responde ao fim do corpo: o servidor que segura a conexão.
	hangAfterData bool
	tlsConfig     *tls.Config
	implicit      bool
	// authOverTLS diz se o AUTH chegou numa conexão cifrada.
	authOverTLS bool
}

func newFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{t: t, ln: ln}
	t.Cleanup(func() { ln.Close() })
	go f.serve()
	return f
}

// newTLSFakeSMTP é o servidor com certificado de 127.0.0.1, e as raízes que o validam.
func newTLSFakeSMTP(t *testing.T, implicit bool) (*fakeSMTP, *x509.CertPool) {
	t.Helper()
	cert, roots := selfSigned(t)
	f := newFakeSMTP(t)
	f.mu.Lock()
	f.tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
	f.implicit = implicit
	f.mu.Unlock()
	return f, roots
}

// selfSigned gera um certificado para 127.0.0.1, válido por uma hora.
func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "smtp de teste"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots
}

func (f *fakeSMTP) port() int { return f.ln.Addr().(*net.TCPAddr).Port }

func (f *fakeSMTP) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeSMTP) handle(raw net.Conn) {
	defer raw.Close()
	f.mu.Lock()
	cfg, implicit := f.tlsConfig, f.implicit
	f.mu.Unlock()
	var conn net.Conn = raw
	encrypted := false
	if cfg != nil && implicit {
		conn, encrypted = tls.Server(raw, cfg), true
	}
	r := bufio.NewReader(conn)
	reply := func(s string) { conn.Write([]byte(s + "\r\n")) }
	reply("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		f.mu.Lock()
		f.cmds = append(f.cmds, line)
		f.mu.Unlock()
		verb := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		switch verb {
		case "EHLO":
			reply("250-fake")
			if cfg != nil && !encrypted {
				reply("250-STARTTLS")
			}
			reply("250 AUTH PLAIN LOGIN")
		case "STARTTLS":
			reply("220 ready")
			conn, encrypted = tls.Server(raw, cfg), true
			r = bufio.NewReader(conn)
		case "AUTH":
			f.mu.Lock()
			f.authOverTLS = encrypted
			f.mu.Unlock()
			reply("235 ok")
		case "RCPT":
			if f.rcpt != "" {
				reply(f.rcpt)
				continue
			}
			reply("250 ok")
		case "DATA":
			reply("354 go ahead")
			var body strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				body.WriteString(l)
			}
			f.mu.Lock()
			f.data = body.String()
			f.mu.Unlock()
			if f.hangAfterData {
				// Segura até o cliente fechar.
				r.ReadString('\n')
				return
			}
			reply("250 queued")
		case "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

// saw diz se algum comando recebido começa com o verbo.
func (f *fakeSMTP) saw(verb string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.cmds {
		if strings.HasPrefix(strings.ToUpper(c), strings.ToUpper(verb)) {
			return true
		}
	}
	return false
}

func (f *fakeSMTP) body() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data
}

// mailerFor monta o Mailer apontado para o servidor falso.
func mailerFor(t *testing.T, f *fakeSMTP, production bool) *Mailer {
	t.Helper()
	env := map[string]string{"SMTP_HOST": "127.0.0.1", "SMTP_PORT": strconv.Itoa(f.port())}
	if production {
		env["SMTP_USER"] = "resend"
		env["SMTP_PASS"] = "senha-de-teste"
	}
	m, err := NewMailerFromEnv(func(k string) string { return env[k] }, production)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Em produção, servidor sem STARTTLS é recusado antes do login e da mensagem: quem está
// no meio e tira a oferta não lê nem a senha nem o código.
func TestProductionRefusesServerWithoutSTARTTLS(t *testing.T) {
	f := newFakeSMTP(t)
	m := mailerFor(t, f, true)

	err := m.SendOTP(context.Background(), "jogador@example.com", domain.OTPPurposeVerifyEmail, "482913", "pt-BR")
	if err == nil {
		t.Fatal("enviou sem TLS")
	}
	for _, verb := range []string{"AUTH", "MAIL", "RCPT", "DATA"} {
		if f.saw(verb) {
			t.Errorf("o servidor recebeu %s numa conexão em claro", verb)
		}
	}
	if strings.Contains(f.body(), "482913") {
		t.Error("o código saiu em claro")
	}
}

// Na 465, o certificado é conferido antes de qualquer comando: um servidor com
// certificado que as raízes não reconhecem não recebe nem o EHLO.
func TestImplicitTLSRejectsUntrustedCertificate(t *testing.T) {
	f, _ := newTLSFakeSMTP(t, true)
	m := mailerFor(t, f, true)
	m.implicitTLS = true

	err := m.SendOTP(context.Background(), "jogador@example.com", domain.OTPPurposeVerifyEmail, "482913", "pt-BR")
	if err == nil {
		t.Fatal("aceitou certificado sem raiz conhecida")
	}
	if f.saw("EHLO") {
		t.Error("o servidor recebeu comando antes de o certificado ser conferido")
	}
}

// Na 465, com o certificado reconhecido, login e mensagem vão cifrados.
func TestImplicitTLSSendsOverVerifiedConnection(t *testing.T) {
	f, roots := newTLSFakeSMTP(t, true)
	m := mailerFor(t, f, true)
	m.implicitTLS = true
	m.rootCAs = roots

	if err := m.SendOTP(context.Background(), "jogador@example.com", domain.OTPPurposeVerifyEmail, "482913", "pt-BR"); err != nil {
		t.Fatal(err)
	}
	if !f.saw("AUTH PLAIN") || !f.authOverTLS {
		t.Error("o login não foi PLAIN numa conexão cifrada")
	}
	if f.saw("STARTTLS") {
		t.Error("pediu STARTTLS numa conexão que já nasceu cifrada")
	}
}

// Fora da 465, produção sobe para TLS pelo STARTTLS antes do login.
func TestProductionSendsAfterSTARTTLS(t *testing.T) {
	f, roots := newTLSFakeSMTP(t, false)
	m := mailerFor(t, f, true)
	m.rootCAs = roots

	if err := m.SendWelcome(context.Background(), "jogador@example.com", "pt-BR"); err != nil {
		t.Fatal(err)
	}
	if !f.saw("STARTTLS") || !f.authOverTLS {
		t.Error("o login não veio depois do STARTTLS")
	}
}

// Fora da produção, o envio passa pelo Mailpit em claro e leva Date e Message-ID do
// domínio do remetente, mais os cabeçalhos a mais de quem chama.
func TestDevelopmentSendsWithHeaders(t *testing.T) {
	f := newFakeSMTP(t)
	m := mailerFor(t, f, false)

	leave := "https://api.example.com/sair"
	if err := m.SendWaitlistConfirmation(context.Background(), "jogador@example.com", "pt-BR", "https://api.example.com/ok", leave); err != nil {
		t.Fatal(err)
	}
	if f.saw("AUTH") {
		t.Error("tentou login sem usuário")
	}
	body := f.body()
	for _, want := range []string{"Date: ", "Message-ID: <", "@logn.sh>", "List-Unsubscribe: <" + leave + ">", "To: <jogador@example.com>"} {
		if !strings.Contains(body, want) {
			t.Errorf("sem %q no e-mail:\n%s", want, body)
		}
	}
	for _, leak := range []string{"User-Agent:", "X-Mailer:"} {
		if strings.Contains(body, leak) {
			t.Errorf("o e-mail diz qual biblioteca o mandou (%s)", leak)
		}
	}
	if !f.saw("EHLO localhost") {
		t.Error("o HELO não é localhost")
	}
}

// Servidor que aceita o corpo e não responde não segura o envio além do ctx: o Mailer
// fecha a conexão, e o erro diz que foi o prazo.
func TestSendGivesUpWhenTheContextEnds(t *testing.T) {
	f := newFakeSMTP(t)
	f.hangAfterData = true
	m := mailerFor(t, f, false)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := m.SendWelcome(ctx, "jogador@example.com", "pt-BR")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("erro sem o prazo na cadeia: %v", err)
	}
	// O prazo da biblioteca por passo é de 15 s; bem antes disso, foi o ctx.
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("levou %s para desistir", elapsed)
	}
	if !f.saw("DATA") {
		t.Fatal("o teste não chegou ao DATA")
	}
}

// A recusa do servidor repete o destinatário; o erro que sai do Mailer não.
func TestSendErrorHasNoAddress(t *testing.T) {
	f := newFakeSMTP(t)
	f.rcpt = "550 5.1.1 <jogador@example.com>: Recipient address rejected"
	m := mailerFor(t, f, false)

	err := m.SendWelcome(context.Background(), "jogador@example.com", "pt-BR")
	if err == nil {
		t.Fatal("o envio passou com o destinatário recusado")
	}
	if strings.Contains(err.Error(), "@") {
		t.Errorf("sobrou endereço: %v", err)
	}
}

// Em produção, faltar uma variável do SMTP aborta o boot, em vez de cair no Mailpit.
func TestNewMailerFromEnvInProduction(t *testing.T) {
	full := map[string]string{
		"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "465",
		"SMTP_USER": "resend", "SMTP_PASS": "senha-de-teste",
	}
	for missing := range full {
		env := map[string]string{}
		for k, v := range full {
			if k != missing {
				env[k] = v
			}
		}
		_, err := NewMailerFromEnv(func(k string) string { return env[k] }, true)
		if err == nil || !strings.Contains(err.Error(), missing) {
			t.Errorf("sem %s: %v", missing, err)
		}
	}

	m, err := NewMailerFromEnv(func(k string) string { return full[k] }, true)
	if err != nil {
		t.Fatal(err)
	}
	if !m.requireTLS {
		t.Error("produção sem TLS obrigatório")
	}

	// Fora da produção, nada é obrigatório.
	if _, err := NewMailerFromEnv(func(string) string { return "" }, false); err != nil {
		t.Errorf("desenvolvimento: %v", err)
	}
	// Porta que não é número é erro, em qualquer ambiente.
	bad := map[string]string{"SMTP_PORT": "smtp"}
	if _, err := NewMailerFromEnv(func(k string) string { return bad[k] }, false); err == nil {
		t.Error("aceitou SMTP_PORT=smtp")
	}
}
