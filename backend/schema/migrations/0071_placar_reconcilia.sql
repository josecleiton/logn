-- Placar geral de XP, segunda de duas (a primeira é a 0070).
--
-- Entre as duas, a revisão anterior do servidor pode ter cadastrado conta sem número e
-- somado XP sem `free_xp`. Esta recalcula o placar de todo mundo, sorteia o número de
-- quem ficou sem, confere de novo e só então fecha `anon_number`.

-- Preenchimento não é mudança da conta: o trigger de `updated_at` fica desligado até o
-- fim (regra 5, como na 0070).
ALTER TABLE users DISABLE TRIGGER trg_users_updated_at;

-- O mesmo cálculo da 0070, agora para toda conta: quem não tem desafio pago da trilha
-- gratuita fica com zero e sem hora. `ProcessEventXP` grava `free_xp_reached_at` com o
-- mesmo CURRENT_TIMESTAMP da linha de `user_paid_challenges`, então a hora recalculada
-- é a que já estava lá.
UPDATE users u
SET (free_xp, free_xp_reached_at) = (
    SELECT 50 * count(*), max(up.created_at)
    FROM user_paid_challenges up
    JOIN challenges c ON c.id = up.challenge_id
    JOIN skill_nodes sn ON sn.id = c.node_id
    JOIN tracks t ON t.id = sn.track_id
    WHERE up.user_id = u.id AND t.kind = 'free'
);

DO $guard$
DECLARE
    bad int;
BEGIN
    SELECT count(*) INTO bad
    FROM users u
    LEFT JOIN (
        SELECT up.user_id, count(*) FILTER (WHERE t.kind = 'paid') AS paid
        FROM user_paid_challenges up
        JOIN challenges c ON c.id = up.challenge_id
        JOIN skill_nodes sn ON sn.id = c.node_id
        JOIN tracks t ON t.id = sn.track_id
        GROUP BY up.user_id
    ) p ON p.user_id = u.id
    WHERE u.global_xp - 50 * COALESCE(p.paid, 0) <> u.free_xp;
    IF bad > 0 THEN
        RAISE EXCEPTION 'placar: % contas com global_xp que não bate com os desafios pagos', bad;
    END IF;
END
$guard$;

-- O número de quem nasceu sem, do mesmo baralho da 0070, sem os números já dados.
WITH n AS (
    SELECT count(*) AS c FROM users
),
pool AS (
    SELECT v, row_number() OVER (ORDER BY gen_random_uuid()) AS rn
    FROM n, generate_series(1000, CASE WHEN n.c <= 4500 THEN 9999 ELSE 99999 END) AS v
    WHERE NOT EXISTS (SELECT 1 FROM users taken WHERE taken.anon_number = v)
),
who AS (
    SELECT id, row_number() OVER (ORDER BY gen_random_uuid()) AS rn
    FROM users
    WHERE anon_number IS NULL
)
UPDATE users u
SET anon_number = pool.v
FROM who
JOIN pool USING (rn)
WHERE u.id = who.id;

ALTER TABLE users ENABLE TRIGGER trg_users_updated_at;

-- Se o baralho acabasse (mais de 99 mil contas sem número), sobraria NULL e este
-- ALTER falharia, desfazendo a migração: nenhuma conta fica sem número em silêncio.
ALTER TABLE users ALTER COLUMN anon_number SET NOT NULL;
