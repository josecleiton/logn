-- Caixa de saída de e-mail (ADR 0026). Todo e-mail que sai de um pedido do app ou da
-- landing é gravado aqui, na mesma transação da mudança que o pede, e só depois do
-- commit vira tarefa no Cloud Tasks. Antes, o envio saía numa goroutine depois da
-- resposta, e o Cloud Run, com `cpu_idle`, corta a CPU da instância quando a resposta
-- sai: o código podia nunca chegar, sem log nem nova tentativa.
--
-- A linha guarda referência, não endereço, sempre que dá: as boas-vindas apontam para a
-- conta, a confirmação da lista de espera para a inscrição, e as duas somem em cascata
-- com elas. O código de verificação não tem para onde apontar, porque `otps` guarda só o
-- HMAC dele: vai cifrado em `code_ciphertext`, com o endereço e o propósito, e o texto
-- cifrado sai da linha assim que ela deixa de estar pendente.
CREATE TABLE email_outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind            VARCHAR(16) NOT NULL CHECK (kind IN ('otp', 'welcome', 'waitlist')),
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'sent', 'skipped', 'failed')),
    locale          VARCHAR(8)  NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    waitlist_id     UUID REFERENCES waitlist_entries(id) ON DELETE CASCADE,
    email           VARCHAR(255),
    purpose         VARCHAR(50),
    code_ciphertext BYTEA,
    attempts        INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error      VARCHAR(255),
    -- Quando a tarefa foi criada no Cloud Tasks. Vazio e pendente há mais de um minuto
    -- é o que a varredura reenfileira.
    enqueued_at     TIMESTAMP WITH TIME ZONE,
    sent_at         TIMESTAMP WITH TIME ZONE,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Cada tipo com a sua referência, e só ela.
    CHECK (
        (kind = 'otp' AND email IS NOT NULL AND purpose IS NOT NULL
             AND user_id IS NULL AND waitlist_id IS NULL)
        OR (kind = 'welcome' AND user_id IS NOT NULL
             AND email IS NULL AND purpose IS NULL AND waitlist_id IS NULL)
        OR (kind = 'waitlist' AND waitlist_id IS NOT NULL
             AND email IS NULL AND purpose IS NULL AND user_id IS NULL)
    ),
    -- O código cifrado existe só enquanto o e-mail de código está pendente. Enviado,
    -- descartado ou desistido, a linha não guarda mais nada que leve a ele.
    CHECK (
        (kind = 'otp' AND status = 'pending') = (code_ciphertext IS NOT NULL)
    )
);

-- A varredura: pendentes que nunca viraram tarefa.
CREATE INDEX email_outbox_unqueued ON email_outbox (created_at)
    WHERE status = 'pending' AND enqueued_at IS NULL;
-- A exclusão de conta apaga os códigos do endereço, que não têm chave para `users`.
CREATE INDEX email_outbox_otp_email ON email_outbox (email) WHERE kind = 'otp';
-- A poda diária das linhas antigas.
CREATE INDEX email_outbox_created_at ON email_outbox (created_at);

CREATE TRIGGER trg_email_outbox_updated_at
BEFORE UPDATE ON email_outbox
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
