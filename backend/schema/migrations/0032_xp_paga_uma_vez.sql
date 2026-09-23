-- XP paga uma vez por desafio.
--
-- Cada resposta aceita somava 50, e rejogar um nó pagava de novo: os portões da trilha
-- viravam moagem, dava para abrir o nó 7 repetindo o nó 1. Esta tabela guarda quais
-- desafios já renderam para cada jogador, e o processador de XP só credita o que entra
-- nela pela primeira vez. Os contadores de bugs e dry runs seguem a mesma regra.
--
-- `challenge_id` não tem chave estrangeira de propósito: corrigir um desafio no
-- repositório de conteúdo não pode apagar o registro de quem já o resolveu. O id é a
-- identidade do desafio; conteúdo novo que mereça XP de novo ganha id novo.
--
-- A linha nasce e nunca muda, então só tem `created_at`.
CREATE TABLE user_paid_challenges (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    challenge_id VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, challenge_id)
);

-- O progresso de hoje foi moído rejogando nós, e só existem contas de teste. Zera para
-- os portões poderem ser testados com a regra nova — sem isto as contas já passaram de
-- todos. Os eventos ficam: a cadeia de hash não é reescrita.
UPDATE users SET global_xp = 0, bugs_found = 0, dry_runs_completed = 0;
DELETE FROM user_progress;
