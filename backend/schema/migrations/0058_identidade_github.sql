-- Login com GitHub (ADR 0019): `github` entra na lista de provedores de
-- `user_identities`. O `subject` é o id numérico da conta no GitHub, em texto; o login
-- (`octocat`) muda quando a pessoa quer, e por isso não serve.
ALTER TABLE user_identities
    DROP CONSTRAINT user_identities_provider_check,
    ADD CONSTRAINT user_identities_provider_check CHECK (provider IN ('google', 'apple', 'github'));
