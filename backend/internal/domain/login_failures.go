package domain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Tetos de tentativa de senha por e-mail, exista a conta ou não, em LoginAttemptWindow.
//
// O limite por IP da rota some trocando de IP, e sem limite por conta o atacante chutava
// a senha de uma conta só de muitos endereços. Os tetos são dois:
//
//   - LoginMaxAttemptsPerSource, do mesmo balde de IP. Dez em quinze minutos sobra para
//     quem erra digitando, e travar a conta de alguém daqui não trava a pessoa, que
//     entra de outro IP.
//   - LoginMaxAttempts, somando todos os IPs. É o que segura o chute distribuído. Só
//     conta a tentativa que o balde do IP deixou passar, então travar a conta por aqui
//     pede dez baldes de IP diferentes a cada quinze minutos; e mesmo assim o login
//     social entra, e trocar a senha pelo código destrava.
const (
	LoginMaxAttemptsPerSource = 10
	LoginMaxAttempts          = 100
)

// LoginAttemptWindow é a janela dos tetos, contada da primeira tentativa.
const LoginAttemptWindow = 15 * time.Minute

// loginKey é a chave da conta em `login_failures`. HMAC, e não o e-mail: o endereço
// que alguém digita sem ter conta não vira dado guardado. O prefixo separa este uso dos
// outros da mesma chave.
func loginKey(email string) string {
	mac := hmac.New(sha256.New, JwtSecretKey)
	mac.Write([]byte("login\x00" + email))
	return hex.EncodeToString(mac.Sum(nil))
}

// loginScope é o balde de IP como `scope`: HMAC, para o IP não ficar guardado.
func loginScope(source string) string {
	mac := hmac.New(sha256.New, JwtSecretKey)
	mac.Write([]byte("login-source\x00" + source))
	return hex.EncodeToString(mac.Sum(nil))
}

// NoteLoginAttempt conta uma tentativa de senha para o e-mail, já normalizado, vinda do
// balde de IP `source`, e diz se ela pode seguir.
//
// Conta antes de conferir a senha, num comando só: conferir e depois contar deixava
// pedidos em paralelo passarem todos pela mesma contagem. A tentativa certa apaga as
// contagens do e-mail (ClearLoginAttempts), então só as erradas acumulam.
//
// A soma de todos os IPs só anda com a tentativa que o balde do IP deixou passar.
// Contar também as recusadas deixava um IP sozinho, a sete pedidos por minuto, levar a
// soma ao teto e travar a conta para todo IP.
func (r *Repository) NoteLoginAttempt(ctx context.Context, email, source string) (bool, error) {
	var perSource int
	var total *int
	err := r.db.QueryRow(ctx, `
		WITH source AS (
		    INSERT INTO login_failures (email_hmac, scope, failures, window_start)
		    VALUES ($1, $2, 1, CURRENT_TIMESTAMP)
		    ON CONFLICT (email_hmac, scope) DO UPDATE SET
		        failures = CASE
		            WHEN login_failures.window_start <= CURRENT_TIMESTAMP - make_interval(secs => $3) THEN 1
		            ELSE login_failures.failures + 1 END,
		        window_start = CASE
		            WHEN login_failures.window_start <= CURRENT_TIMESTAMP - make_interval(secs => $3) THEN CURRENT_TIMESTAMP
		            ELSE login_failures.window_start END
		    RETURNING failures
		), total AS (
		    INSERT INTO login_failures (email_hmac, scope, failures, window_start)
		    SELECT $1, '', 1, CURRENT_TIMESTAMP FROM source WHERE source.failures <= $4
		    ON CONFLICT (email_hmac, scope) DO UPDATE SET
		        failures = CASE
		            WHEN login_failures.window_start <= CURRENT_TIMESTAMP - make_interval(secs => $3) THEN 1
		            ELSE login_failures.failures + 1 END,
		        window_start = CASE
		            WHEN login_failures.window_start <= CURRENT_TIMESTAMP - make_interval(secs => $3) THEN CURRENT_TIMESTAMP
		            ELSE login_failures.window_start END
		    RETURNING failures
		)
		SELECT (SELECT failures FROM source), (SELECT failures FROM total)`,
		loginKey(email), loginScope(source), LoginAttemptWindow.Seconds(), LoginMaxAttemptsPerSource).Scan(&perSource, &total)
	if err != nil {
		return false, err
	}
	// Sem `total`, o balde do IP já tinha recusado e a soma nem foi tocada.
	if perSource > LoginMaxAttemptsPerSource || total == nil {
		return false, nil
	}
	return *total <= LoginMaxAttempts, nil
}

// ClearLoginAttempts zera as contagens do e-mail, de todo IP: depois de um login certo,
// ou de trocar a senha pelo código, as tentativas de antes não contam mais.
func (r *Repository) ClearLoginAttempts(ctx context.Context, email string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM login_failures WHERE email_hmac = $1`, loginKey(email))
	return err
}

// PurgeLoginAttempts apaga as contagens de janela vencida. Roda na purga diária; sem
// ela, todo e-mail que alguém já digitou errado ficava na tabela para sempre.
func (r *Repository) PurgeLoginAttempts(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM login_failures
		WHERE window_start <= CURRENT_TIMESTAMP - make_interval(secs => $1)`,
		LoginAttemptWindow.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
