package domain

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
)

// Propósitos aceitos para um OTP. Era texto livre: qualquer string de até 50
// caracteres virava linha em `otps`.
const (
	OTPPurposeVerifyEmail   = "verify_email"
	OTPPurposeResetPassword = "reset_password"
)

func ValidOTPPurpose(purpose string) bool {
	return purpose == OTPPurposeVerifyEmail || purpose == OTPPurposeResetPassword
}

// OTPValidity é quanto tempo um código vale depois de enviado.
//
// O e-mail diz esse prazo ao jogador e lê daqui: o texto estava escrito à mão como
// dez minutos enquanto o servidor aceitava por quinze.
const OTPValidity = 15 * time.Minute

// OTPValidityMinutes é OTPValidity como o e-mail escreve.
func OTPValidityMinutes() int { return int(OTPValidity / time.Minute) }

// OTPMaxAttempts é quantos códigos errados um OTP aguenta antes de morrer.
//
// Sem limite, seis dígitos se varriam inteiros pelo `verify-otp` dentro dos quinze
// minutos de validade. Sozinho ele não basta: cada reenvio traz código novo com as
// tentativas zeradas, e um código por OTPResendCooldown dava 7.200 palpites por dia.
// Quem segura isso é OTPMaxFailuresPerWindow.
const OTPMaxAttempts = 5

// OTPMaxFailuresPerWindow é quantos códigos errados o e-mail e propósito aguentam em
// OTPFailureWindow, somando todos os reenvios. Acima disso nem o código certo vale, e
// nenhum código novo sai, até a janela vencer.
//
// Vinte por dia dão ao atacante uma chance em cinquenta mil por dia contra um milhão de
// códigos. O preço é que ele pode, gastando os vinte, travar por um dia a recuperação
// de uma conta (a senha e o login social continuam entrando) ou o cadastro por senha de
// um e-mail que ainda não tem conta (o login social continua criando). A resposta é
// `otp_locked`, com o tempo que falta, e o app diz isso em vez de "espere um minuto".
const OTPMaxFailuresPerWindow = 20

// OTPFailureWindow é a janela de OTPMaxFailuresPerWindow, contada da primeira falha.
const OTPFailureWindow = 24 * time.Hour

// OTPHourlySendCap é o teto de códigos enviados por hora, somando todos os endereços.
// O limite por IP some trocando de IP, e o intervalo por endereço some trocando de
// endereço: sem teto global, a rota disparava e-mail para quem quisesse em nome do
// `noreply`. É variável só para o teste.
var OTPHourlySendCap = 200

// otpSendCapWindow é a janela de OTPHourlySendCap. A purga não apaga linha enviada
// dentro dela: o teto conta as linhas de `otps`, e apagar uma afrouxaria a contagem.
const otpSendCapWindow = time.Hour

// ErrOTPSendCapReached é o teto global de envios por hora atingido.
var ErrOTPSendCapReached = errors.New("otp hourly send cap reached")

// OTPLockedError é a janela de OTPMaxFailuresPerWindow estourada para o e-mail e
// propósito. RetryAfter é quanto falta para ela vencer.
type OTPLockedError struct {
	RetryAfter time.Duration
}

func (e *OTPLockedError) Error() string { return "otp locked by failed attempts" }

// OTPResendCooldown é o intervalo mínimo entre dois envios para o mesmo e-mail e
// propósito. Segura o disparo de e-mail para um endereço só e o atacante que pede
// código novo a cada cinco erros.
const OTPResendCooldown = 60 * time.Second

// ErrOTPCooldown sinaliza pedido de código antes do intervalo mínimo.
var ErrOTPCooldown = errors.New("otp requested too soon")

func GenerateOTP() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

// otpHash é o que vai para o banco no lugar do código.
//
// HMAC, e não SHA-256 puro: com um milhão de códigos possíveis, um hash sem chave se
// inverte na hora. A chave é a mesma do JWT porque quem tem uma já tem a outra; e-mail
// e propósito entram para que o mesmo código em duas linhas não dê o mesmo hash.
func otpHash(email, purpose, code string) string {
	mac := hmac.New(sha256.New, JwtSecretKey)
	mac.Write([]byte("otp\x00" + email + "\x00" + purpose + "\x00" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// SaveOTP grava um código novo para o e-mail e zera as tentativas do código. As falhas
// da janela (OTPMaxFailuresPerWindow) não zeram.
//
// Devolve ErrOTPCooldown se o anterior saiu há menos de OTPResendCooldown, e
// *OTPLockedError se a janela de falhas estourou. Sem esse intervalo, o endpoint mandava e-mail para
// qualquer endereço sem parar, e cada pedido ainda invalidava o código que o dono de
// verdade tinha acabado de receber. Com OTPHourlySendCap atingido, devolve
// ErrOTPSendCapReached antes de gravar.
//
// O e-mail com o código entra na caixa de saída na mesma transação, na língua `lang`
// (ADR 0026).
func (r *Repository) SaveOTP(ctx context.Context, email, code, purpose, lang string, duration time.Duration) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := saveOTPTx(ctx, tx, email, code, purpose, duration); err != nil {
		return err
	}
	outboxID, err := queueOTPTx(ctx, tx, email, purpose, code, lang)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	TrackOutbox(ctx, outboxID)
	return nil
}

func saveOTPTx(ctx context.Context, tx pgx.Tx, email, code, purpose string, duration time.Duration) error {
	// A linha é uma por e-mail e propósito, então isto conta endereços que receberam
	// código na última hora, não e-mails. Um endereço só recebe no máximo um por
	// OTPResendCooldown, e o teto é sobre espalhar o envio por endereços.
	var sent int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM otps WHERE sent_at > CURRENT_TIMESTAMP - make_interval(secs => $1)`,
		otpSendCapWindow.Seconds()).Scan(&sent); err != nil {
		return err
	}
	if sent >= OTPHourlySendCap {
		return ErrOTPSendCapReached
	}

	expiresAt := time.Now().Add(duration)
	query := `
		INSERT INTO otps (email, purpose, code_hash, expires_at, attempts, sent_at)
		VALUES ($1, $2, $3, $4, 0, CURRENT_TIMESTAMP)
		ON CONFLICT (email, purpose)
		DO UPDATE SET code_hash = EXCLUDED.code_hash, expires_at = EXCLUDED.expires_at,
		              attempts = 0, sent_at = CURRENT_TIMESTAMP
		WHERE otps.sent_at <= CURRENT_TIMESTAMP - make_interval(secs => $5)
		  AND NOT (otps.failures_since > CURRENT_TIMESTAMP - make_interval(secs => $6)
		           AND otps.recent_failures >= $7)
	`
	tag, err := tx.Exec(ctx, query,
		email, purpose, otpHash(email, purpose, code), expiresAt, OTPResendCooldown.Seconds(),
		OTPFailureWindow.Seconds(), OTPMaxFailuresPerWindow)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Recusado: pelo intervalo ou pela janela de falhas. A janela tem erro próprio,
		// com o tempo que falta, porque esperar o intervalo não resolve nada.
		var remaining float64
		err := tx.QueryRow(ctx, `
			SELECT EXTRACT(EPOCH FROM failures_since + make_interval(secs => $3) - CURRENT_TIMESTAMP)
			FROM otps
			WHERE email = $1 AND purpose = $2
			  AND failures_since > CURRENT_TIMESTAMP - make_interval(secs => $3)
			  AND recent_failures >= $4`,
			email, purpose, OTPFailureWindow.Seconds(), OTPMaxFailuresPerWindow).Scan(&remaining)
		if err == nil {
			return &OTPLockedError{RetryAfter: time.Duration(remaining * float64(time.Second))}
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return ErrOTPCooldown
	}
	return nil
}

// CheckOTP diz se o código confere, sem gastá-lo, e conta a tentativa se não conferir.
//
// Um único UPDATE faz as duas coisas: com o check separado do incremento, cem pedidos
// em paralelo liam o mesmo `attempts` e passavam todos pelo limite. A falha conta
// também na janela de OTPMaxFailuresPerWindow, que recomeça quando vence; com ela
// estourada, nem o código certo passa.
func (r *Repository) CheckOTP(ctx context.Context, email, code, purpose string) (bool, error) {
	query := `
		UPDATE otps
		SET attempts = attempts + CASE WHEN code_hash = $3 THEN 0 ELSE 1 END,
		    recent_failures = CASE
		        WHEN code_hash = $3 THEN recent_failures
		        WHEN failures_since IS NULL
		          OR failures_since <= CURRENT_TIMESTAMP - make_interval(secs => $5) THEN 1
		        ELSE recent_failures + 1 END,
		    failures_since = CASE
		        WHEN code_hash = $3 THEN failures_since
		        WHEN failures_since IS NULL
		          OR failures_since <= CURRENT_TIMESTAMP - make_interval(secs => $5) THEN CURRENT_TIMESTAMP
		        ELSE failures_since END
		WHERE email = $1 AND purpose = $2
		  AND expires_at > CURRENT_TIMESTAMP
		  AND attempts < $4
		  AND NOT (failures_since > CURRENT_TIMESTAMP - make_interval(secs => $5)
		           AND recent_failures >= $6)
		RETURNING code_hash = $3
	`
	var match bool
	err := r.db.QueryRow(ctx, query, email, purpose, otpHash(email, purpose, code), OTPMaxAttempts,
		OTPFailureWindow.Seconds(), OTPMaxFailuresPerWindow).Scan(&match)
	if err != nil {
		return false, err // pgx.ErrNoRows: sem código, expirado ou esgotado
	}
	return match, nil
}

// ConsumeOTP confere o código e o apaga, numa operação que só um pedido ganha.
//
// Antes era check e depois delete: duas requisições com o mesmo código passavam as
// duas no intervalo entre um e outro. Agora quem apaga a linha é quem ganhou.
func (r *Repository) ConsumeOTP(ctx context.Context, email, code, purpose string) (bool, error) {
	valid, err := r.CheckOTP(ctx, email, code, purpose)
	if err != nil || !valid {
		return false, err
	}

	tag, err := r.db.Exec(ctx, `
		DELETE FROM otps
		WHERE email = $1 AND purpose = $2 AND code_hash = $3
		  AND expires_at > CURRENT_TIMESTAMP AND attempts < $4`,
		email, purpose, otpHash(email, purpose, code), OTPMaxAttempts)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// PurgeStaleOTPs apaga os códigos que não servem a mais nada. Roda na purga diária; sem
// ela, `request-otp` deixava uma linha para todo endereço que alguém digitou, para sempre.
//
// A linha só sai quando as três coisas que ela ainda segura acabaram: o código venceu, o
// envio saiu da janela do teto por hora (que conta estas linhas), e a janela de falhas
// passou, ou a trava do e-mail sumiria junto com a linha.
func (r *Repository) PurgeStaleOTPs(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM otps
		WHERE expires_at <= CURRENT_TIMESTAMP
		  AND sent_at <= CURRENT_TIMESTAMP - make_interval(secs => $1)
		  AND (failures_since IS NULL OR failures_since <= CURRENT_TIMESTAMP - make_interval(secs => $2))`,
		otpSendCapWindow.Seconds(), OTPFailureWindow.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
