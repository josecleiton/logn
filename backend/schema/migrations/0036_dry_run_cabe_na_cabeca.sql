-- DRY_RUN tem de caber na cabeça: no máximo quatro células de estado.
--
-- O jogo é de celular, e quem joga no ônibus não tem papel. Sem papel, o limite de um
-- traço não é o relógio, é a memória de trabalho — umas quatro coisas ao mesmo tempo.
-- Dar mais tempo a quem não tem onde anotar só adia o momento em que ele perde o fio.
-- Quatro dos sete DRY_RUN da trilha pediam de 5 a 24 células (a tabela de troco com
-- sete posições, a BFS com seis distâncias e a fila, o crivo com 21 posições) e foram
-- reescritos na migração de conteúdo anterior a esta.
--
-- Quem escreve o desafio declara o pico em validation.trace_cells. A regra de contagem,
-- que está no guia de escrita de desafios:
--
--   * cada variável escalar cujo valor muda durante o trecho conta 1;
--   * cada posição de vetor escrita durante o trecho conta 1;
--   * fila, pilha ou heap contam pelos elementos que têm dentro;
--   * não contam índices de laço for, nem o que está no código ou no watch e não muda.
--
-- Vale o pico, o maior número de células vivas ao mesmo tempo, não a soma.
--
-- O número mora em validation, e não em content, de propósito: o revisor cego recebe só
-- content, conta as células sem ver o que o autor declarou, e divergência bloqueia o
-- lote. A constraint garante que o número existe e está dentro do teto; se ele é
-- verdade, só a revisão cega diz, como com qualquer gabarito (ADR 0006, seção C).
--
-- COALESCE pelo motivo de sempre: sem a chave, a expressão é NULL, e CHECK que avalia
-- para NULL passa.

ALTER TABLE challenges ADD CONSTRAINT chk_dry_run_cabe_na_cabeca CHECK (
    template_type <> 'DRY_RUN' OR COALESCE(
        jsonb_typeof(payload->'validation'->'trace_cells') = 'number'
        AND (payload->'validation'->>'trace_cells')::int BETWEEN 1 AND 4
    , false)
);
