-- Limites de abuso pedidos pela revisão de segurança antes de abrir o código.
--
-- `otps`: falhas contadas através dos reenvios. `attempts` zera a cada código novo, e
-- pedir um código por minuto dava cinco palpites por minuto contra a mesma conta, sem
-- fim: 7.200 por dia. `recent_failures` só zera quando a janela de `failures_since`
-- vence ou quando o código certo apaga a linha. Sem `failures_since`, não há falha
-- contada.
ALTER TABLE otps
    ADD COLUMN recent_failures INTEGER NOT NULL DEFAULT 0 CHECK (recent_failures >= 0),
    ADD COLUMN failures_since  TIMESTAMP WITH TIME ZONE;

-- O teto global de envios por hora conta por aqui.
CREATE INDEX otps_sent_at ON otps (sent_at);

-- `login_failures`: tentativas de senha por e-mail, exista a conta ou não. Contar só das
-- contas que existem faria o 429 dizer quem tem conta. A chave é o HMAC do e-mail: o
-- endereço que alguém digita sem ter conta não vira dado guardado.
--
-- `scope` é de onde veio a tentativa: o HMAC do balde do IP (o /64 no IPv6), com teto
-- baixo, ou vazio para a soma de todos os IPs, com teto alto. Contar só a soma deixava
-- qualquer um travar o login de uma conta alheia com dez pedidos; contar só por IP
-- deixava chutar a senha de muitos IPs. A janela começa na primeira tentativa, e a
-- purga diária apaga as vencidas.
CREATE TABLE login_failures (
    email_hmac   CHAR(64)    NOT NULL,
    scope        VARCHAR(64) NOT NULL DEFAULT '',
    failures     INTEGER NOT NULL DEFAULT 0 CHECK (failures >= 0),
    window_start TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (email_hmac, scope)
);

CREATE INDEX login_failures_window_start ON login_failures (window_start);

CREATE TRIGGER trg_login_failures_updated_at
BEFORE UPDATE ON login_failures
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
