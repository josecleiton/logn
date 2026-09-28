-- Registro de toda transação da loja que o servidor aceitou, com o JWS como chegou.
--
-- É auditoria, então sobrevive à exclusão da conta: `user_id` vira nulo, e a transação
-- continua dizendo o que foi comprado e quando. Uma transação entra uma vez só; reenviar
-- o mesmo JWS não cria linha nova. O `ON DELETE SET NULL` é um UPDATE, daí o `updated_at`.
CREATE TABLE store_transactions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    track_id    UUID NOT NULL REFERENCES tracks(id),
    provider    VARCHAR(32) NOT NULL CHECK (provider IN ('apple_storekit', 'google_play', 'stripe')),
    provider_transaction_id VARCHAR(128) NOT NULL,
    original_transaction_id VARCHAR(128) NOT NULL,
    environment VARCHAR(16) NOT NULL CHECK (environment IN ('Production', 'Sandbox', 'Xcode')),
    raw_payload TEXT NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (provider, provider_transaction_id)
);

CREATE TRIGGER trg_store_transactions_updated_at
BEFORE UPDATE ON store_transactions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Transações revogadas, fora da conta de propósito.
--
-- Um JWS assinado antes do reembolso não traz `revocationDate` e passa na assinatura para
-- sempre. Se a revogação vivesse só em `entitlements`, que some com a conta, bastava
-- reembolsar, excluir a conta, criar outra e restaurar o JWS antigo. Toda revogação,
-- automática ou manual, entra aqui.
--
-- A Apple reenvia notificação e não garante ordem: um REFUND atrasado pode chegar depois
-- do REFUND_REVERSED dele. Por isso a linha guarda a hora assinada de cada lado, em ms, e
-- a revogação vale enquanto `reversed_at_ms` for nulo. Uma reversão sem revogação
-- conhecida também grava linha, para o REFUND mais velho que chegar depois ser ignorado.
CREATE TABLE revoked_transactions (
    provider    VARCHAR(32) NOT NULL CHECK (provider IN ('apple_storekit', 'google_play', 'stripe')),
    original_transaction_id VARCHAR(128) NOT NULL,
    reason      VARCHAR(32) NOT NULL CHECK (reason IN ('refund', 'store_revoke', 'fraud', 'redistribution', 'account_sharing')),
    revoked_at_ms  BIGINT NOT NULL,
    reversed_at_ms BIGINT,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (provider, original_transaction_id),
    -- Só reembolso se reverte; revogação manual e da loja não.
    CHECK (reversed_at_ms IS NULL OR reason = 'refund')
);

CREATE TRIGGER trg_revoked_transactions_updated_at
BEFORE UPDATE ON revoked_transactions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
