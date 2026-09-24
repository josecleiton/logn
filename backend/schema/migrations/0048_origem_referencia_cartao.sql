-- Fecha a referência que a 0046 deixou aberta de propósito: só depois que a 0047
-- (privada, repositório de conteúdo) inserir a linha de `challenge_origins` para toda
-- origem já em uso — hoje, `FARIAS` — é que a FK pode entrar sem quebrar produção.

ALTER TABLE challenges
    ADD CONSTRAINT fk_challenges_origin FOREIGN KEY (origin) REFERENCES challenge_origins(id);
