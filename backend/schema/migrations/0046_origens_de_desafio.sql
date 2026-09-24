-- O cartão de origem vira conteúdo (ADR 0011), não mais texto fixo no catálogo de
-- interface: nome, papel e corpo agora moram no banco, traduzidos, como o resto da
-- trilha.
--
-- Duas tabelas, no padrão de `skill_node_translations`/`challenge_translations`
-- (0043): uma linha por origem, e uma por origem e língua. `challenges.origin` já
-- existe desde a 0010 e continua um VARCHAR solto por enquanto — a FK entra só na 0048,
-- depois que a 0047 (privada) inserir a origem que já está em uso hoje (`FARIAS`). Três
-- migrações para produção não quebrar no meio: esta cria as tabelas vazias, a 0047 põe
-- o conteúdo, a 0048 fecha a referência.

CREATE TABLE challenge_origins (
    id         TEXT PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER trg_challenge_origins_updated_at
    BEFORE UPDATE ON challenge_origins
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE challenge_origin_translations (
    origin_id  TEXT        NOT NULL REFERENCES challenge_origins(id) ON DELETE CASCADE,
    locale     TEXT        NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    name       TEXT        NOT NULL CHECK (length(btrim(name)) > 0),
    role       TEXT        NOT NULL CHECK (length(btrim(role)) > 0),
    body       TEXT        NOT NULL CHECK (length(btrim(body)) > 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (origin_id, locale)
);
CREATE TRIGGER trg_challenge_origin_translations_updated_at
    BEFORE UPDATE ON challenge_origin_translations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
