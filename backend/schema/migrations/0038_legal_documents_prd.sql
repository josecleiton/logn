CREATE TABLE legal_documents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind         VARCHAR(16) NOT NULL CHECK (kind IN ('terms', 'privacy')),
    locale       VARCHAR(8)  NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    version      INT NOT NULL,
    effective_at TIMESTAMP WITH TIME ZONE NOT NULL,
    material     BOOLEAN NOT NULL,
    body_html    TEXT NOT NULL,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (kind, locale, version)
);

CREATE TABLE legal_acceptances (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        VARCHAR(16) NOT NULL,
    version     INT NOT NULL,
    locale      VARCHAR(8) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, kind, version)
);

ALTER TABLE users DROP COLUMN IF EXISTS legal_acceptances;
