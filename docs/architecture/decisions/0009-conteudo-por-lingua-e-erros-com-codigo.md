# ADR 0009: Conteúdo da trilha por língua, e erros da API com código

## 1. Visão Geral

A interface do app já saía em pt-BR, en e es pelo catálogo de `i18n/`, mas o conteúdo da trilha não: nome dos nós, enunciado, explicação e rótulo das opções moravam dentro do `payload` JSONB de `challenges` e nas colunas `name` e `description` de `skill_nodes`, só em português. O app em espanhol mostrava a tela em espanhol e o desafio em português.

Três coisas vieram junto, porque tocavam o mesmo caminho:

- o commit `94ee7b3` tinha trocado `ChallengeContent.description` por um enum `TrapKey`, e o Core deixou de desserializar qualquer desafio real;
- os erros da API eram frase solta em inglês (`http.Error(w, "Email already registered", 409)`), e o Core decidia por prefixo de texto;
- a cor e o ícone de cada nó saíam de palavras procuradas no nome em português, que em espanhol erram.

O plano está no repositório de conteúdo, `docs/planos/<nome>`, com as 15 decisões tomadas em 2026-09-24.

## 2. Decisão

**O texto sai do payload e vai para tabelas de tradução.** A migração `0043_conteudo_por_lingua.sql`, num release só:

- cria `skill_node_translations (node_id, locale, name, description)` e `challenge_translations (challenge_id, locale, title, description, explanation, watch_note, option_labels)`, com `created_at`, `updated_at` e o trigger `set_updated_at`;
- copia o português que existia;
- tira `title`, `description`, `watch_note` e `explanation` do payload, e a CHECK `chk_payload_sem_texto` impede que voltem;
- apaga `skill_nodes.name` e `description`;
- acrescenta `skill_nodes.topic`, o assunto neutro do nó (`adhoc`, `graphs`), que decide cor e ícone.

**A API monta o payload de sempre, na língua pedida.** `GET /api/v1/nodes` e `/api/v1/challenges` escolhem a língua por `?lang=`, depois `Accept-Language` e por fim pt-BR (`internal/locale`, o mesmo que os documentos legais usam). `domain.AssemblePayload` devolve o texto ao payload neutro e troca cada opção de TAG (`queue`) pelo rótulo da língua (`Fila`), no `options` e no `correct_options` juntos, para o motor de partida continuar comparando dentro do mesmo payload. A resposta leva `Content-Language` e `Vary: Accept-Language`.

**Publicação por língua.** Um nó só aparece numa língua quando ele e todos os desafios dele têm tradução nela (o CTE `publishedNodes`). Nó sem desafio aparece. Pré-requisito que aponta para nó escondido some da lista.

**O Core guarda a língua do que tem.** `Model.content_locale` vem do `Content-Language`. A semente do bundle passa à versão 2, uma trilha por língua (`TrailSeed.locales`), e cai em português quando a língua do app não veio. O retrato offline grava a língua. Retrato em outra língua devolve o XP e os desafios pagos, e o texto fica o da semente na língua certa até a próxima busca. Retrato sem o campo conta como português. Trocar a língua com conteúdo na tela aplica a semente e busca de novo.

**A fonte do conteúdo são arquivos.** `logn-conteudo/trilha/` tem um arquivo de estrutura por nó e por desafio e um de texto por língua, e `glossario/tags.json` tem o rótulo de cada opção de TAG. `exporta_trilha.py` fez a virada do banco para os arquivos. `gen_conteudo.py` gera, dos arquivos, uma migração idempotente com o estado inteiro. `just content-check` confere os arquivos antes, e `just content-check release` recusa também o que falta traduzir ou reescrever.

**Erro da API é `{"code", "message"}`.** `writeError` responde com um código estável (`email_taken`, `password_too_short`, `otp_resend_too_soon`, `rate_limited`…) e o `StatusText` como mensagem, que é para log. O Core decide pelo código (`api_code`) e ainda aceita o texto antigo onde já aceitava. Quatro `StatusKey` novos dizem ao jogador o que corrigir no cadastro. O 404 de usuário inexistente no token virou 401 `unauthenticated`.

**O Core não manda frase.** `TrapInfo` carrega `TrapKind` e o texto cru do conteúdo; "resposta errada", "tempo esgotado" e "linha N" saem do catálogo, no shell.

## 3. Alternativas descartadas

* **Texto por língua dentro do payload** (`title: {"pt-BR": …, "en": …}`). Cada app instalado leria um formato novo, e as CHECKs de estrutura ficariam presas ao número de línguas.
* **Expand e contract em dois releases.** Nenhum app antigo em produção depende das colunas apagadas, e um release só evita um período com duas fontes de texto.
* **Fonte do conteúdo no banco, editada por migração à mão.** Revisar tradução em SQL é ler aspas escapadas, e não dá para conferir o conjunto antes de aplicar. Os arquivos dão diff por desafio e por língua.
* **Cair em português por desafio quando falta tradução.** A partida misturaria línguas no meio de um nó. Publicar o nó inteiro ou nada deixa a lacuna visível.
* **O app traduz os rótulos de TAG pelo catálogo.** Opção de TAG é conteúdo e cresce com a trilha; o catálogo de `i18n/` é da interface e só muda com release do app.

## 4. Consequências

* Conteúdo novo exige a tradução pt-BR no mesmo arquivo de migração: o payload sozinho não aparece em língua nenhuma.
* A semente tem de ser gerada com a API de pé, uma língua por vez. `TestTrailSeedMatchesTheDatabase` compara cada língua da semente com o que a API serviria, e reprova língua publicada que não viaja no app. `just seed-bundle --release` recusa trilha menor em alguma língua.
* `topic` nasce nulo, e só a migração de conteúdo gerada pelos arquivos preenche. Até lá o app cai no nome, como antes.
* Os códigos de erro são contrato com apps já instalados: renomear um é quebrar o app antigo.
* A lista de palavras em português do `content-check` foi tirada dos identificadores da trilha antes da reescrita; identificador novo em português passa até alguém acrescentar a palavra.
