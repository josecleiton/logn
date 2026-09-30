-- Revogação manual de licença (ADR 0021): redistribuição e compartilhamento de conta,
-- os casos 3 e 4 da seção 10.5 dos termos, com a contestação que o texto promete.
--
-- A revogação manual sai de `revoked_transactions` e ganha tabela própria. Numa tabela
-- só, um reembolso da Apple que chegasse com a licença revogada à mão não deixava rastro
-- (a revogação que já vale não muda de motivo), e devolver a licença depois da
-- contestação devolvia a trilha a quem foi reembolsado. Com as duas separadas, cada
-- bloqueio sai só pelo caminho dele, e a licença volta quando os dois saíram.

-- O bloqueio manual aberto. Chaveado pela transação, como `revoked_transactions`, e sem
-- chave para `users`: ele sobrevive à exclusão da conta, para a compra não voltar por
-- restauração numa conta nova.
--
-- `revoked` bloqueia. `review` é o compartilhamento de conta com a contestação em
-- análise: a licença volta enquanto analisamos (seção 10.5), e o bloqueio fica aberto
-- até a decisão. Contestação aceita apaga a linha; recusada volta a `revoked`.
CREATE TABLE manual_revocations (
    provider                VARCHAR(32)  NOT NULL CHECK (provider IN ('apple_storekit', 'google_play', 'stripe')),
    original_transaction_id VARCHAR(128) NOT NULL,
    reason                  VARCHAR(32)  NOT NULL CHECK (reason IN ('redistribution', 'account_sharing')),
    status                  VARCHAR(16)  NOT NULL DEFAULT 'revoked' CHECK (status IN ('revoked', 'review')),
    created_at              TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (provider, original_transaction_id),
    CHECK (status = 'revoked' OR reason = 'account_sharing')
);

CREATE TRIGGER trg_manual_revocations_updated_at
BEFORE UPDATE ON manual_revocations
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- O histórico de cada revogação e resposta à contestação, com a evidência e quem fez.
-- Só recebe INSERT. Sai com a conta, como a licença e o registro de aparelhos (política,
-- seção 9).
CREATE TABLE license_actions (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    track_id                UUID         NOT NULL REFERENCES tracks (id),
    provider                VARCHAR(32)  NOT NULL CHECK (provider IN ('apple_storekit', 'google_play', 'stripe')),
    original_transaction_id VARCHAR(128) NOT NULL,
    action                  VARCHAR(16)  NOT NULL CHECK (action IN ('revoke', 'appeal')),
    reason                  VARCHAR(32)  NOT NULL CHECK (reason IN ('redistribution', 'account_sharing')),
    -- Só na contestação: recebida (redistribuição, a licença segue fora), em análise
    -- (compartilhamento, a licença volta), aceita, recusada.
    outcome                 VARCHAR(16)  CHECK (outcome IN ('received', 'review', 'accepted', 'rejected')),
    evidence                TEXT         NOT NULL CHECK (char_length(evidence) BETWEEN 1 AND 2000),
    -- A conta de serviço que chamou a rota, tirada do token (ADR 0021).
    actor                   VARCHAR(320) NOT NULL CHECK (char_length(actor) > 0),
    created_at              TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((action = 'revoke') = (outcome IS NULL)),
    CHECK (outcome IS DISTINCT FROM 'received' OR reason = 'redistribution'),
    CHECK (outcome IS DISTINCT FROM 'review' OR reason = 'account_sharing')
);

CREATE INDEX license_actions_user_idx ON license_actions (user_id, created_at);

-- A revogação manual que já estivesse em `revoked_transactions` muda de tabela. A
-- evidência dela nunca foi guardada, então o histórico começa sem ela.
INSERT INTO manual_revocations (provider, original_transaction_id, reason)
SELECT provider, original_transaction_id, reason
FROM revoked_transactions
WHERE reason IN ('redistribution', 'account_sharing')
ON CONFLICT DO NOTHING;

DELETE FROM revoked_transactions WHERE reason IN ('redistribution', 'account_sharing');

ALTER TABLE revoked_transactions
    ADD CONSTRAINT revoked_transactions_store_reasons
    CHECK (reason IN ('refund', 'store_revoke', 'fraud'));
