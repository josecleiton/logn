# ADR 0006: Ordem explícita e validação de conteúdo na tabela `challenges`

## 1. Visão Geral

A tabela `challenges` guarda o conteúdo jogável do LogN numa coluna `JSONB` polimórfica (ver ADR 0004, *PostgreSQL JSONB Polymorphism*). Até aqui ela tinha cinco linhas de seed e dois contratos implícitos que ninguém tinha escrito em lugar nenhum:

1. **A ordem em que os desafios viram A, B, C numa partida** saía do `ORDER BY id ASC` do repositório Go, ou seja, da ordenação alfabética de uma coluna `VARCHAR`. Funcionava por acidente, porque os ids de seed eram `ch_001` a `ch_005`, todos com a mesma largura. Com largura variável, `ch_10` ordena antes de `ch_2` e a partida embaralha sozinha.

2. **As regras que tornam um desafio solúvel** — `expected_string` estar entre as `options`, `correct_options` ser subconjunto de `options`, COMPLEXITY_MATCH ter exatamente dois `correct_options`, `correct_line` cair dentro de `code_lines` — existiam só como texto em documentação. Nenhuma `CHECK CONSTRAINT` e nenhum teste as verificava. Um desafio impossível de acertar entrava no banco sem reclamação.

O que motivou esta ADR foi encontrar o defeito real. Um desafio de busca do primeiro nó tinha `correct_line` apontando para a linha errada; o bug estava duas linhas abaixo da linha marcada. A própria `explanation`, escrita na migração `0003`, descreve a linha certa. O primeiro desafio do jogo reprovava quem acertava, e o defeito atravessou intacto as constraints, o `curl` de verificação e os 39 testes do `shared_core`.

## 2. Decisões Arquiteturais

### A. Ordem de apresentação vira coluna, não convenção de nome

A tabela ganha `position_idx SMALLINT NOT NULL` com `UNIQUE (node_id, position_idx)`, e a consulta do repositório passa de `ORDER BY id ASC` para `ORDER BY node_id, position_idx`. O sufixo `_idx` segue `row_idx` e `col_idx` de `skill_nodes`, e evita o nome `position`, que em Postgres também é função.

A escolha é a mesma que `skill_nodes` já fazia com `row_idx` e `col_idx` mais `UNIQUE(row_idx, col_idx)` — ordem posicional é dado, não efeito colateral de collation. Ganho adicional: o `UNIQUE` vira guarda de verdade, porque dois desafios não conseguem reivindicar a mesma letra dentro de um nó.

O `id` permanece `VARCHAR` legível, no formato `ch_<nó><dois dígitos>` (`ch_201`, `ch_702`). Considerei trocar por UUIDv6 e descartei: o ganho de UUID ordenável por tempo é localidade de índice sob alta taxa de inserção, e aqui os desafios entram à mão, cinco por vez, numa migração revisada por uma pessoa. Em compensação, id opaco de 36 caracteres tornaria ilegível a diff da migração — que é o único lugar onde este conteúdo é revisado antes de chegar ao jogador.

### B. As invariantes estruturais moram no banco, não num teste

As quatro regras acima viram `CHECK CONSTRAINT`. Postgres expressa todas em `jsonb` sem função externa:

* `payload->'content'->'options' @> payload->'content'->'correct_options'` para o subconjunto;
* `jsonb_array_length(payload->'content'->'correct_options') = 2` para o par de complexidade;
* `(payload->'validation'->>'correct_line')::int BETWEEN 1 AND jsonb_array_length(payload->'content'->'code_lines')` para o intervalo da linha;
* pertinência de `expected_string` às `options` no FILL_IN_THE_BLANK.

Escolhi o banco em vez de teste por uma razão de tempo, não de elegância. O autor de um desafio é um agente ou uma pessoa escrevendo SQL à mão, e a constraint falha no instante em que essa pessoa ainda está com o `INSERT` na frente e o contexto na cabeça. Um teste falha depois, num passo que dá para esquecer de rodar — e de fato dava: o passo de verificação da documentação mandava rodar `cargo test`, que exercita fixtures fixas em `match_engine.rs` e **nunca vê** o conteúdo recém-escrito.

`explanation` passa a ser exigida em todos os templates. Ela não era, e a migração `0002` existe justamente para consertar a regressão de desafios sem explicação caindo no texto genérico.

Uma armadilha para quem for escrever a próxima constraint deste tipo, e que já mordeu uma vez: **`CHECK` cujo resultado é `NULL` passa.** Quando a chave testada não existe, `payload->'x'->'y'` é `NULL`, `jsonb_typeof(NULL)` é `NULL`, e a comparação inteira vira `NULL` em vez de falso — a constraint recusa o valor inválido e aceita o valor ausente, que costuma ser o caso pior. Toda condição específica de template vai dentro de `COALESCE(..., false)`, a menos que dependa só de operadores que nunca devolvem `NULL`, como `?`.

### C. O que as constraints não alcançam, e o que cobre isso

Nenhuma constraint distingue um `correct_line` válido de um `correct_line` certo. As quatro regras acima são estruturais: elas garantem que existe uma resposta possível, nunca que a resposta marcada é a verdadeira. O defeito do `ch_001` passaria por todas elas.

A cobertura desse caso é processual, e fica registrada aqui porque é a parte que alguém vai querer remover depois por parecer cara: **todo lote de desafios passa por um revisor cego antes de virar migração**. O revisor recebe apenas o bloco `content`, sem `validation` e sem `correct_options`, resolve cada desafio como jogador, e só então compara com o gabarito. Divergência bloqueia o lote. O ponto é que o revisor não pode concordar por preguiça, porque ele não viu com o que concordar.

Para DRY_RUN vale uma regra a mais: a saída esperada é obtida compilando e rodando o trecho, nunca de trace mental.

### D. Escopo de nó é contrato do banco

A coluna `description` de `skill_nodes` é mais específica que o nome do nó — cada nó tem um subconjunto de tópicos próprio, mais estreito do que o nome sugere. Quem escreve desafio cobre o que a `description` lista, e não o que o nome sugere.

## 3. Próximos Passos de Implementação

1. Migração de conserto dos seeds: `correct_line` do desafio de busca corrigido para a linha certa, e um outro desafio transposto de Kotlin para C++, que era a única linha da trilha em outra linguagem.
2. Migração de schema: as `CHECK CONSTRAINT` de B, a coluna `position_idx`, o `UNIQUE (node_id, position_idx)` e o backfill de `ch_001` a `ch_005`.
3. `ORDER BY node_id, position_idx` em `backend/internal/domain/repository.go`.
4. Vocabulário fechado de tags para TAG_THE_PATTERN, hoje inexistente — sem ele cada nó inventa o seu e o jogador vê "DP" num e "Programação dinâmica" em outro.
5. Reescrever `docs/testing/prompt-seed-desafios.md` com estas regras e com as correções factuais que a auditoria levantou.

## 4. Nota sobre o julgamento duplicado

`Event::SubmitChallengeAnswer`, em `shared_core/src/app.rs`, julga apenas SPOT_THE_BUG e FILL_IN_THE_BLANK e cai em `else { false }` para os outros três templates. O julgamento correto dos cinco está em `match_engine.rs`, via `MatchSubmit`, e é esse o caminho que as telas usam hoje — nenhuma view dispara o evento antigo, que sobrevive só no binding gerado da FFI.

Não removi o caminho morto nesta ADR porque ele está fora do escopo dela, mas registro o risco: é API exportada, e ligar uma tela nova nela produz Wrong Answer silencioso em DRY_RUN, COMPLEXITY_MATCH e TAG_THE_PATTERN.
