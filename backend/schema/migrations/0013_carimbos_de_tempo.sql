-- Toda tabela passa a ter created_at, e toda tabela que sofre UPDATE ganha updated_at
-- mantido por trigger. É a regra 5 do AGENTS.md.
--
-- O motivo é depuração, não burocracia. Investigando conteúdo apareceram duas
-- perguntas que o schema não sabia responder: quando esta linha apareceu, e isto mudou
-- antes ou depois daquela migração.
--
-- updated_at por trigger, e não por disciplina no SET de cada UPDATE: coluna que
-- depende de alguém lembrar de atualizar acaba mentindo, e coluna que mente é pior que
-- coluna nenhuma — ela dá a impressão de que o dado está fresco.
--
-- Duas tabelas ficam de fora de propósito:
--
--   game_events      é append-only por desenho (ADR 0002): a cadeia criptográfica
--                    depende de o histórico não ser reescrito, e um updated_at ali
--                    seria convite para alguém reescrevê-lo. Ganha só created_at.
--   schema_migrations já resolve com applied_at, e quem escreve nela é o migrate.go.
--
-- As linhas que já existem nascem com o created_at desta migração, não com a hora real
-- em que foram criadas. Não há como recuperar isso, e é melhor um carimbo aproximado e
-- declarado do que coluna nula.

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

ALTER TABLE challenges
    ADD COLUMN created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
CREATE TRIGGER trg_challenges_updated_at BEFORE UPDATE ON challenges
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE users
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE skill_nodes
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
CREATE TRIGGER trg_skill_nodes_updated_at BEFORE UPDATE ON skill_nodes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE refresh_tokens
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
CREATE TRIGGER trg_refresh_tokens_updated_at BEFORE UPDATE ON refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE user_progress
    ADD COLUMN created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
CREATE TRIGGER trg_user_progress_updated_at BEFORE UPDATE ON user_progress
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE user_sync_state
    ADD COLUMN created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
CREATE TRIGGER trg_user_sync_state_updated_at BEFORE UPDATE ON user_sync_state
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE otps
    ADD COLUMN created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
CREATE TRIGGER trg_otps_updated_at BEFORE UPDATE ON otps
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- game_events: append-only. Carimbo de nascimento e nada mais.
ALTER TABLE game_events
    ADD COLUMN created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
