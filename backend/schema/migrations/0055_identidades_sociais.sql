-- Contas de provedor externo ligadas a uma conta do LogN (ADR 0016).
--
-- A conta é achada pelo `subject`, o `sub` do ID token, e nunca pelo e-mail: o e-mail
-- de uma conta Google pode mudar, o `sub` não. O e-mail só serve no primeiro login, para
-- ligar a identidade à conta que já tinha o mesmo endereço.
--
-- Um mesmo `sub` pertence a uma conta só. Uma conta pode ter mais de uma identidade,
-- inclusive duas do mesmo provedor. A linha entra e sai por INSERT e DELETE, nunca por
-- UPDATE; por isso só `created_at`. Sai em cascata com a conta, no expurgo.
CREATE TABLE user_identities (
    provider    VARCHAR(32) NOT NULL CHECK (provider IN ('google')),
    subject     VARCHAR(255) NOT NULL CHECK (length(subject) > 0),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (provider, subject)
);

CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);
