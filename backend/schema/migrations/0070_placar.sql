-- Placar geral de XP (docs/specs/logn_placar_spec.md), primeira de duas.
--
-- Esta é aditiva: a revisão anterior do servidor ainda atende enquanto a nova sobe, e
-- ela cadastra conta sem número e soma XP sem `free_xp`. Por isso `anon_number` nasce
-- sem NOT NULL; a 0071 recalcula `free_xp` de todo mundo, sorteia o número de quem
-- nasceu na janela do deploy, e só então fecha a coluna.

ALTER TABLE users
    ADD COLUMN free_xp            INT NOT NULL DEFAULT 0 CHECK (free_xp >= 0),
    ADD COLUMN free_xp_reached_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN anon_number        INT CHECK (anon_number > 0),
    ADD COLUMN nickname           VARCHAR(20) CHECK (nickname ~ '^[a-z0-9_]{3,20}$'),
    ADD COLUMN nickname_burned_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN leaderboard_hidden BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE users
    ADD CONSTRAINT users_nickname_key UNIQUE (nickname),
    ADD CONSTRAINT users_anon_number_key UNIQUE (anon_number);

-- Os preenchimentos abaixo não são mudança da conta: com o trigger ligado, todo
-- `updated_at` viraria a hora desta migração, e a pergunta "isto mudou antes ou depois
-- da migração X" (regra 5) perderia a resposta. Desliga só aqui; a transação da
-- migração religa no fim, ou desfaz tudo.
ALTER TABLE users DISABLE TRIGGER trg_users_updated_at;

-- O XP da trilha gratuita, contado dos desafios que já pagaram. O desafio não tem
-- trilha: quem tem é o nó. O 50 é `XPPerAcceptedAnswer`; um teste do Go trava a
-- constante nesse valor, para esta conta não mentir.
--
-- A hora em que chegou ao valor é a do último desafio pago, que `ProcessEventXP` grava
-- com o mesmo CURRENT_TIMESTAMP de `free_xp_reached_at`.
WITH earned AS (
    SELECT up.user_id, count(*) AS n, max(up.created_at) AS last_at
    FROM user_paid_challenges up
    JOIN challenges c ON c.id = up.challenge_id
    JOIN skill_nodes sn ON sn.id = c.node_id
    JOIN tracks t ON t.id = sn.track_id
    WHERE t.kind = 'free'
    GROUP BY up.user_id
)
UPDATE users u
SET free_xp = e.n * 50, free_xp_reached_at = e.last_at
FROM earned e
WHERE u.id = e.user_id;

-- Desde a 0032, `ProcessEventXP` é o único que escreve `global_xp`, sempre 50 por linha
-- nova em `user_paid_challenges`. Uma conta em que o global menos o pago não dá o
-- gratuito tem desafio que sumiu do banco, ou XP escrito por fora: é bug de XP, e não
-- pode virar placar. Para aqui, sem consertar. A mensagem só conta, sem id nem e-mail.
--
-- A revisão anterior continua atendendo: um acerto gravado entre o preenchimento e
-- esta guarda também a dispara. Nesse caso a migração desfaz tudo e basta subir de
-- novo. Antes do deploy, a mesma conta roda como SELECT em produção.
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

ALTER TABLE users
    ADD CONSTRAINT users_free_xp_reached CHECK ((free_xp = 0) = (free_xp_reached_at IS NULL));

-- O número de quem já tem conta. Sorteado de um baralho embaralhado por
-- gen_random_uuid() (pg_strong_random): nada sai do id nem da ordem do cadastro, que
-- diria quando a conta nasceu. Quatro dígitos enquanto o baralho fica no máximo meio
-- cheio; cinco depois. O Go sorteia o de quem se cadastrar daqui em diante.
WITH n AS (
    SELECT count(*) AS c FROM users
),
pool AS (
    SELECT v, row_number() OVER (ORDER BY gen_random_uuid()) AS rn
    FROM n, generate_series(1000, CASE WHEN n.c <= 4500 THEN 9999 ELSE 99999 END) AS v
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

-- A ordem do placar: XP, quem chegou primeiro, id. Só quem tem XP aparece.
CREATE INDEX users_leaderboard_idx ON users (free_xp DESC, free_xp_reached_at, id) WHERE free_xp > 0;

-- O histórico da moderação e das objeções, com o motivo e quem fez (ADR 0021, como
-- `license_actions`). Só recebe INSERT. Sai com a conta.
CREATE TABLE leaderboard_actions (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    action     VARCHAR(16)  NOT NULL CHECK (action IN ('anonymize', 'hide', 'unhide')),
    -- O apelido no momento da ação; nulo quando a conta não tinha.
    nickname   VARCHAR(20),
    reason     TEXT         NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 2000),
    -- A conta de serviço que chamou a rota, tirada do token (ADR 0021).
    actor      VARCHAR(320) NOT NULL CHECK (char_length(actor) > 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX leaderboard_actions_user_idx ON leaderboard_actions (user_id, created_at);

-- O apelido removido pela moderação, que ninguém pode escolher de novo. Sem chave para
-- `users`, como `manual_revocations`: a cascata de `leaderboard_actions` levaria o
-- bloqueio junto com a conta do dono, e o apelido voltaria a ficar livre depois da
-- purga. Só recebe INSERT.
CREATE TABLE blocked_nicknames (
    nickname   VARCHAR(20) PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
