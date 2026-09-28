CREATE TABLE tracks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        VARCHAR(64) UNIQUE NOT NULL,
    kind        VARCHAR(8)  NOT NULL CHECK (kind IN ('free', 'paid')),
    status      VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'discontinued')),
    author      VARCHAR(255) NOT NULL,
    app_store_product_id VARCHAR(255) UNIQUE,  -- nulo na trilha gratuita
    content_version INT NOT NULL DEFAULT 1,
    -- A cor do balão da trilha no catálogo, como os problemas A–M (LogN Trilhas, 1b).
    color       VARCHAR(7) NOT NULL DEFAULT '#FF7A45' CHECK (color ~ '^#[0-9A-F]{6}$'),
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((kind = 'paid') = (app_store_product_id IS NOT NULL))
);

CREATE TRIGGER trg_tracks_updated_at
BEFORE UPDATE ON tracks
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE track_translations (
    track_id    UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    locale      VARCHAR(8) NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (track_id, locale)
);

CREATE TRIGGER trg_track_translations_updated_at
BEFORE UPDATE ON track_translations
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE skill_nodes ADD COLUMN track_id UUID REFERENCES tracks(id);

INSERT INTO tracks (id, slug, kind, status, author)
VALUES ('00000000-0000-0000-0000-000000000000', 'free', 'free', 'active', 'LogN');

-- A principal aparece no catálogo como as outras, com nome nas três línguas.
INSERT INTO track_translations (track_id, locale, name) VALUES
    ('00000000-0000-0000-0000-000000000000', 'pt-BR', 'Problem Solving'),
    ('00000000-0000-0000-0000-000000000000', 'en', 'Problem Solving'),
    ('00000000-0000-0000-0000-000000000000', 'es', 'Problem Solving');

UPDATE skill_nodes SET track_id = '00000000-0000-0000-0000-000000000000';

ALTER TABLE skill_nodes ALTER COLUMN track_id SET NOT NULL;

ALTER TABLE skill_nodes DROP CONSTRAINT skill_nodes_row_idx_col_idx_key,
                        ADD UNIQUE (track_id, row_idx, col_idx);

CREATE TABLE entitlements (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id     UUID NOT NULL REFERENCES tracks(id),
    original_transaction_id VARCHAR(64) NOT NULL,
    status       VARCHAR(16) NOT NULL CHECK (status IN ('active', 'revoked')),
    revoked_reason VARCHAR(32) CHECK (revoked_reason IN ('refund', 'store_revoke', 'fraud', 'redistribution', 'account_sharing')),
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, track_id),
    -- Revogado sempre diz por quê, e ativo nunca carrega motivo de uma revogação antiga.
    CHECK ((status = 'revoked') = (revoked_reason IS NOT NULL))
);

-- Uma transação liga-se a uma conta ativa por vez.
CREATE UNIQUE INDEX entitlements_one_active_owner
    ON entitlements (original_transaction_id) WHERE status = 'active';

CREATE TRIGGER trg_entitlements_updated_at
BEFORE UPDATE ON entitlements
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Registro de aparelhos, sem limite: é evidência de compartilhamento, não trava (spec, seção 2).
CREATE TABLE entitlement_devices (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id    UUID NOT NULL REFERENCES tracks(id),
    device_id   VARCHAR(64) NOT NULL,   -- identifierForVendor, nunca identificador de publicidade
    last_seen_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, track_id, device_id)
);

CREATE TRIGGER trg_entitlement_devices_updated_at
BEFORE UPDATE ON entitlement_devices
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A chave de conteúdo de cada versão da trilha. Nunca em claro: `wrapped_key` é o nonce
-- seguido da chave cifrada em AES-256-GCM com TRACK_KEY_SECRET, que só o servidor tem.
-- Nasce na primeira licença ou pacote pedido para a versão, e não muda mais.
CREATE TABLE track_keys (
    track_id    UUID NOT NULL REFERENCES tracks(id),
    content_version INT NOT NULL,
    wrapped_key BYTEA NOT NULL CHECK (octet_length(wrapped_key) = 12 + 32 + 16),
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (track_id, content_version)
);
