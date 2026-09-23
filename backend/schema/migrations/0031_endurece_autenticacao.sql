-- Consertos do pente fino de segurança no backend.

-- OTP: o código deixa de ficar em claro e ganha contador de tentativas.
--
-- Seis dígitos, quinze minutos e nenhum limite davam para varrer o espaço inteiro
-- pelo `verify-otp` antes do prazo acabar e trocar a senha de qualquer conta. Agora
-- cada código aguenta poucas tentativas e morre.
--
-- `code_hash` é HMAC, não SHA-256 puro: com só um milhão de códigos possíveis, um hash
-- sem chave se inverte na hora e não protegeria nada.
--
-- `sent_at` marca o último envio e dá o intervalo mínimo entre dois pedidos para o
-- mesmo e-mail. Não dá para usar `updated_at`, porque cada tentativa errada também
-- atualiza a linha.
--
-- Os códigos em aberto não sobrevivem à troca de formato. Eles duram quinze minutos, e
-- quem estava no meio do fluxo só precisa pedir outro.
DELETE FROM otps;

ALTER TABLE otps
    DROP COLUMN otp_code,
    ADD COLUMN code_hash VARCHAR(64) NOT NULL,
    ADD COLUMN attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN sent_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD CONSTRAINT chk_otps_purpose CHECK (purpose IN ('verify_email', 'reset_password'));

-- E-mail em minúsculas.
--
-- `A@x.com` e `a@x.com` viravam duas contas, cada uma com seu OTP. O servidor passa a
-- normalizar na entrada, e as linhas antigas vão para a mesma forma aqui. Se duas
-- contas colidirem, a migração falha e a transação volta: é o sinal para resolver a
-- duplicata à mão antes, em vez de uma delas sumir.
UPDATE users SET email = lower(btrim(email)) WHERE email <> lower(btrim(email));

-- O id do evento passa a ser único por usuário, não no banco inteiro.
--
-- O cliente gera ids como `match_<timestamp>`: dois jogadores no mesmo instante
-- colidiam, e qualquer um podia gravar de antemão os ids que outra pessoa ia usar e
-- travar o sync dela na chave primária.
ALTER TABLE game_events
    DROP CONSTRAINT game_events_pkey,
    ADD PRIMARY KEY (user_id, id);

-- Revogar todas as sessões de um usuário (troca de senha, reuso de refresh) filtra por
-- `user_id`.
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens (user_id);
