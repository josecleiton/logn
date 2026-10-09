-- O refresh procura o token pelo hash duas vezes a cada renovação: na leitura e no
-- `UPDATE` da rotação. Só havia índice por `user_id` (0031), então as duas buscas
-- varriam a tabela inteira, que ganha uma linha a cada refresh e nunca perdia nenhuma.
--
-- Único, além de rápido: dois tokens com o mesmo hash seriam o mesmo token, e a rotação
-- revogaria os dois de uma vez.
CREATE UNIQUE INDEX IF NOT EXISTS refresh_tokens_token_hash_key ON refresh_tokens (token_hash);

-- A purga diária apaga os vencidos por `expires_at`.
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens (expires_at);
