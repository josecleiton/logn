-- Novo aceite dos termos (ADR 0020): o "o que mudou" de cada versão, e o registro do
-- aceite com o que prova o consentimento.

-- Uma linha por mudança de uma versão, por língua. O texto vem de
-- logn-conteudo/legal/mudancas/v<N>.toml, por gen_documentos_legais.py; nunca à mão.
-- Só de inserção, como `legal_documents`: corrigir é publicar a próxima versão.
CREATE TABLE legal_document_changes (
    kind        VARCHAR(16) NOT NULL,
    version     INT NOT NULL,
    locale      VARCHAR(8)  NOT NULL,
    -- A ordem em que a mudança aparece na tela, dentro da versão e do documento.
    position    INT NOT NULL CHECK (position >= 0),
    change      VARCHAR(8)  NOT NULL CHECK (change IN ('added', 'changed', 'removed')),
    -- O id da `<section>` no fragmento: é o que o `?highlight=` marca.
    section_id  VARCHAR(64) NOT NULL CHECK (section_id ~ '^[a-z0-9-]+$'),
    summary     TEXT NOT NULL CHECK (length(summary) BETWEEN 1 AND 400),
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (kind, version, locale, position),
    FOREIGN KEY (kind, locale, version) REFERENCES legal_documents (kind, locale, version)
);

-- O aceite passa a guardar o texto que estava na tela (o hash do corpo servido), de
-- que versão a pessoa vinha, quais mudanças ela viu, de que app e por onde. As linhas
-- de antes ficam nulas; as novas trazem `source`, e com ele o resto.
ALTER TABLE legal_acceptances
    ADD COLUMN body_sha256   CHAR(64),
    ADD COLUMN from_version  INT,
    ADD COLUMN shown_changes JSONB,
    ADD COLUMN app_version   VARCHAR(32),
    ADD COLUMN platform      VARCHAR(16),
    ADD COLUMN source        VARCHAR(16),
    ADD CONSTRAINT chk_legal_acceptances_source
        CHECK (source IS NULL OR source IN ('signup', 'reaccept', 'notice')),
    ADD CONSTRAINT chk_legal_acceptances_sha
        CHECK (body_sha256 IS NULL OR body_sha256 ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT chk_legal_acceptances_platform
        CHECK (platform IS NULL OR platform IN ('ios', 'android')),
    ADD CONSTRAINT chk_legal_acceptances_shown_changes
        CHECK (shown_changes IS NULL OR (
            jsonb_typeof(shown_changes) = 'array'
            AND NOT jsonb_path_exists(shown_changes, '$[*] ? (@.type() != "string")')
        )),
    -- A versão de origem é uma que a pessoa aceitou antes: menor que esta (0 é nunca).
    ADD CONSTRAINT chk_legal_acceptances_from_version
        CHECK (from_version IS NULL OR (from_version >= 0 AND from_version < version)),
    ADD CONSTRAINT chk_legal_acceptances_app_version
        CHECK (app_version IS NULL OR app_version ~ '^[0-9A-Za-z.+-]{1,32}$'),
    -- Aceite novo prova o texto: com `source`, o hash é obrigatório. O reaceite diz
    -- também de onde veio e o que foi mostrado.
    ADD CONSTRAINT chk_legal_acceptances_new_rows
        CHECK (source IS NULL OR body_sha256 IS NOT NULL),
    ADD CONSTRAINT chk_legal_acceptances_reaccept
        CHECK (source IS DISTINCT FROM 'reaccept'
               OR (from_version IS NOT NULL AND shown_changes IS NOT NULL AND platform IS NOT NULL));
