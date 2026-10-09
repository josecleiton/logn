-- Índices que faltavam para apagar sem varrer tabela.
--
-- `email_outbox.user_id` e `waitlist_id` são chaves estrangeiras com `ON DELETE CASCADE`
-- (0073) e não tinham índice: cada conta ou inscrição apagada varria a caixa de saída
-- inteira atrás das linhas dela, e a purga de inscrições pendentes apaga em lote. Parciais,
-- porque cada linha tem no máximo uma das duas referências; a igualdade da cascata já
-- implica `IS NOT NULL`, e o planejador usa o índice.
CREATE INDEX IF NOT EXISTS idx_email_outbox_user_id
    ON email_outbox (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_email_outbox_waitlist_id
    ON email_outbox (waitlist_id) WHERE waitlist_id IS NOT NULL;

-- A purga diária de contas procura as que pediram exclusão. São poucas entre todas, e
-- sem índice a busca varria `users` inteira todo dia.
CREATE INDEX IF NOT EXISTS idx_users_deletion_requested_at
    ON users (deletion_requested_at) WHERE deletion_requested_at IS NOT NULL;
