-- A tabela `challenges` tinha dois contratos que ninguém tinha escrito.
--
-- O primeiro: a ordem em que os desafios viram A, B, C numa partida saía do
-- `ORDER BY id ASC` do repositório, ou seja, da ordenação alfabética de um VARCHAR.
-- Funcionava porque os ids de seed tinham todos a mesma largura; com largura
-- variável, ch_10 ordena antes de ch_2 e a partida embaralha sozinha.
--
-- O segundo: as regras que tornam um desafio solúvel — a resposta estar entre as
-- opções, correct_options ser subconjunto de options, o par de complexidade ter dois
-- itens, correct_line cair dentro de code_lines — só existiam como texto em
-- documentação. Um desafio impossível de acertar entrava no banco sem reclamação, e
-- só aparecia com o jogador na frente.
--
-- Contexto completo em docs/architecture/decisions/0006.

-- 1. Explicação obrigatória em todos os templates.
ALTER TABLE challenges ADD CONSTRAINT chk_explanation_presente CHECK (
    payload->'validation' ? 'explanation'
    AND length(btrim(payload->'validation'->>'explanation')) > 0
);

-- 2. SPOT_THE_BUG: correct_line existe, é número, e aponta para uma linha que existe.
--    Era o buraco que deixou um desafio apontar para uma linha que não era a do bug.
--    A forma desta constraint é corrigida na 0006 — ver o comentário de lá.
ALTER TABLE challenges ADD CONSTRAINT chk_spot_the_bug_linha_valida CHECK (
    template_type <> 'SPOT_THE_BUG' OR (
        jsonb_typeof(payload->'validation'->'correct_line') = 'number'
        AND (payload->'validation'->>'correct_line')::int
            BETWEEN 1 AND jsonb_array_length(payload->'content'->'code_lines')
    )
);

-- 3. FILL_IN_THE_BLANK: a resposta está entre os blocos arrastáveis, e o código tem
--    onde soltar.
ALTER TABLE challenges ADD CONSTRAINT chk_fill_resposta_nas_opcoes CHECK (
    template_type <> 'FILL_IN_THE_BLANK' OR (
        (payload->'content'->'options') @> (payload->'validation'->'expected_string')
        AND strpos(payload->'content'->>'code_lines', '_____') > 0
    )
);

-- 4. COMPLEXITY_MATCH e TAG_THE_PATTERN: não dá para marcar o que não está na tela.
ALTER TABLE challenges ADD CONSTRAINT chk_correct_options_subconjunto CHECK (
    template_type NOT IN ('COMPLEXITY_MATCH', 'TAG_THE_PATTERN')
    OR (payload->'content'->'options') @> (payload->'content'->'correct_options')
);

-- 5. COMPLEXITY_MATCH são dois campos, [tempo, espaço], comparados posição a posição.
--    Com menos de dois itens o motor devolve errado para qualquer resposta.
ALTER TABLE challenges ADD CONSTRAINT chk_complexity_par_tempo_espaco CHECK (
    template_type <> 'COMPLEXITY_MATCH'
    OR jsonb_array_length(payload->'content'->'correct_options') = 2
);

-- 6. Ordem de apresentação vira dado, como skill_nodes já faz com row_idx e col_idx.
ALTER TABLE challenges ADD COLUMN position_idx SMALLINT;

-- O backfill reproduz exatamente a ordem alfabética que valia até aqui: nenhuma
-- partida existente muda de letra por causa desta migração.
WITH ordem_alfabetica AS (
    SELECT id, row_number() OVER (PARTITION BY node_id ORDER BY id) AS pos
    FROM challenges
)
UPDATE challenges c
   SET position_idx = o.pos
  FROM ordem_alfabetica o
 WHERE c.id = o.id;

ALTER TABLE challenges ALTER COLUMN position_idx SET NOT NULL;

-- Dois desafios do mesmo nó não podem reivindicar a mesma letra.
ALTER TABLE challenges ADD CONSTRAINT uq_challenges_node_position UNIQUE (node_id, position_idx);
