-- Trilha fora da vitrine (ADR 0014).
--
-- `available = false` tira a trilha do catálogo, dos nós e da amostra para quase todo
-- mundo: continua vendo quem está em `track_previewers` daquela trilha e quem já tem
-- direito ativo a ela. A compra vale mais que a flag; esconder uma trilha à venda não
-- pode sumir com o que alguém pagou.
--
-- A gratuita não sai da vitrine: sem ela o app abre sem trilha nenhuma.
ALTER TABLE tracks
    ADD COLUMN available BOOLEAN NOT NULL DEFAULT true,
    ADD CONSTRAINT chk_tracks_free_available CHECK (kind = 'paid' OR available);

-- Quem vê a trilha indisponível antes de ela abrir. Entra e sai por SQL, à mão: não há
-- rota que escreva aqui. Linha não sofre UPDATE, só INSERT e DELETE.
CREATE TABLE track_previewers (
    track_id    UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (track_id, user_id)
);
