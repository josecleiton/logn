-- Sign in with Apple (ADR 0017): `apple` entra na lista de provedores de
-- `user_identities`. A 0055 já foi aplicada, então a restrição é trocada aqui.
ALTER TABLE user_identities
    DROP CONSTRAINT user_identities_provider_check,
    ADD CONSTRAINT user_identities_provider_check CHECK (provider IN ('google', 'apple'));
