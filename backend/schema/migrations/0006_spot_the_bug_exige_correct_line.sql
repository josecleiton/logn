-- A constraint chk_spot_the_bug_linha_valida, escrita na 0005, deixava passar um
-- SPOT_THE_BUG sem correct_line nenhum.
--
-- Lógica de três valores: com a chave ausente, payload->'validation'->'correct_line'
-- é NULL, jsonb_typeof(NULL) é NULL, NULL = 'number' é NULL, e um CHECK cujo
-- resultado é NULL é considerado satisfeito. A constraint recusava linha fora do
-- intervalo e aceitava linha nenhuma — que é o caso pior, porque o motor compara com
-- None e reprova qualquer resposta.
--
-- COALESCE fecha o buraco: ausência de chave passa a ser falso, não desconhecido.
--
-- As outras constraints da 0005 não têm o mesmo problema, e por acidente feliz: as
-- chaves de que elas dependem já são exigidas por chk_payload_structure e por
-- chk_choice_templates_have_options. correct_line era a única que ninguém exigia.

ALTER TABLE challenges DROP CONSTRAINT chk_spot_the_bug_linha_valida;

ALTER TABLE challenges ADD CONSTRAINT chk_spot_the_bug_linha_valida CHECK (
    template_type <> 'SPOT_THE_BUG' OR COALESCE(
        jsonb_typeof(payload->'validation'->'correct_line') = 'number'
        AND (payload->'validation'->>'correct_line')::int
            BETWEEN 1 AND jsonb_array_length(payload->'content'->'code_lines')
    , false)
);
