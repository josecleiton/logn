-- 1. Tabelas Independentes
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255), -- Nullable for OAuth users
    global_xp INT NOT NULL DEFAULT 0,
    bugs_found INT NOT NULL DEFAULT 0,
    dry_runs_completed INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS skill_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    row_idx INT NOT NULL,
    col_idx INT NOT NULL,
    required_xp INT NOT NULL DEFAULT 100,
    prerequisites JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(row_idx, col_idx)
);

CREATE TABLE IF NOT EXISTS otps (
    email VARCHAR(255) NOT NULL,
    otp_code VARCHAR(6) NOT NULL,
    purpose VARCHAR(50) NOT NULL, -- 'verify_email', 'reset_password'
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (email, purpose)
);

-- 2. Tabelas Dependentes
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    device_id VARCHAR(255),
    revoked BOOLEAN DEFAULT FALSE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_progress (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    node_id UUID REFERENCES skill_nodes(id) ON DELETE CASCADE,
    current_xp INT NOT NULL DEFAULT 0,
    unlocked BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMP WITH TIME ZONE,
    PRIMARY KEY (user_id, node_id)
);

CREATE TABLE challenges (
    id VARCHAR(50) PRIMARY KEY,
    node_id UUID REFERENCES skill_nodes(id) NOT NULL,
    template_type VARCHAR(50) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    
    -- Garantia de Consistência JSONB
    CONSTRAINT chk_payload_structure CHECK (
        payload ? 'content' AND
        payload ? 'validation' AND
        (
            (template_type = 'SPOT_THE_BUG' AND payload->'content' ? 'code_lines') OR
            (template_type != 'SPOT_THE_BUG')
        ) AND
        -- DRY_RUN precisa do código a ser traçado, do painel de watch e da saída esperada.
        (
            (template_type = 'DRY_RUN'
                AND payload->'content' ? 'code_lines'
                AND payload->'content' ? 'watch_variables'
                AND jsonb_typeof(payload->'content'->'watch_variables') = 'array'
                AND payload->'validation' ? 'expected_string'
                AND payload->'validation'->>'type' = 'OUTPUT_MATCH'
            ) OR
            (template_type != 'DRY_RUN')
        ) AND
        -- FILL_IN_THE_BLANK é arrastar e soltar: sem `options` a tela abre com a lacuna
        -- e nenhum bloco para arrastar, e o problema fica sem resposta possível.
        (
            (template_type = 'FILL_IN_THE_BLANK'
                AND payload->'content' ? 'code_lines'
                AND payload->'content' ? 'options'
                AND jsonb_typeof(payload->'content'->'options') = 'array'
                AND jsonb_array_length(payload->'content'->'options') > 1
                AND payload->'validation' ? 'expected_string'
            ) OR
            (template_type != 'FILL_IN_THE_BLANK')
        )
    ),

    -- Mesma regra para os templates de escolha: sem opções não há o que responder, e
    -- sem gabarito o motor reprova qualquer resposta.
    CONSTRAINT chk_choice_templates_have_options CHECK (
        template_type NOT IN ('COMPLEXITY_MATCH', 'TAG_THE_PATTERN') OR (
            payload->'content' ? 'options'
            AND jsonb_typeof(payload->'content'->'options') = 'array'
            AND jsonb_array_length(payload->'content'->'options') > 1
            AND payload->'content' ? 'correct_options'
            AND jsonb_typeof(payload->'content'->'correct_options') = 'array'
            AND jsonb_array_length(payload->'content'->'correct_options') > 0
        )
    )
);

-- Tabela simplificada para guardar o Sync (Mini-Git)
CREATE TABLE user_sync_state (
    user_id VARCHAR(50) PRIMARY KEY,
    last_hash VARCHAR(64) NOT NULL
);

CREATE TABLE game_events (
    id VARCHAR(50) PRIMARY KEY,
    user_id VARCHAR(50) REFERENCES user_sync_state(user_id),
    event_type VARCHAR(50) NOT NULL,
    payload_json JSONB NOT NULL,
    timestamp BIGINT NOT NULL,
    previous_hash VARCHAR(64) NOT NULL,
    current_hash VARCHAR(64) NOT NULL
);

