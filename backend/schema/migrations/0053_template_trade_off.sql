-- TRADEOFF_MATCH: o jogador escolhe o principal benefício e a principal desvantagem de
-- uma decisão de arquitetura, entre frases. Mesma forma do COMPLEXITY_MATCH — um par
-- em `correct_options`, comparado posição a posição —, com outra ordem: [benefício,
-- desvantagem].
--
-- As opções do payload são identificadores (`isolation`, `unshared_space`), como as
-- tags do TAG_THE_PATTERN; o texto de cada uma, por língua, vai em
-- `challenge_translations.option_labels`, e o servidor troca identificador por texto
-- ao montar o desafio. A exigência de rótulo para toda opção, em toda língua, fica no
-- content-check: CHECK não olha outra tabela.
--
-- As constraints de 0000 e 0005 listam os templates de escolha por nome e não cobrem
-- este. Elas não se editam; esta soma as mesmas garantias.
--
-- O COALESCE é de propósito: campo ausente deixa a expressão nula, e CHECK nulo passa.
ALTER TABLE challenges ADD CONSTRAINT chk_tradeoff_par_beneficio_desvantagem CHECK (
    template_type <> 'TRADEOFF_MATCH' OR COALESCE(
        jsonb_typeof(payload->'content'->'options') = 'array'
        AND jsonb_array_length(payload->'content'->'options') > 2
        AND jsonb_typeof(payload->'content'->'correct_options') = 'array'
        AND jsonb_array_length(payload->'content'->'correct_options') = 2
        AND (payload->'content'->'options') @> (payload->'content'->'correct_options')
        AND payload->'content'->'correct_options'->>0 <> payload->'content'->'correct_options'->>1,
        false
    )
);
