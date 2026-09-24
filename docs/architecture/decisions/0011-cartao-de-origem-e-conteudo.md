# ADR 0011: O cartão de origem vira conteúdo

## 1. Visão Geral

O selo de origem (a coluna `challenges.origin`, desde a 0010) abre um cartão de homenagem quando um desafio não nasceu no LogN. O texto desse cartão — nome, papel e corpo — vivia em `i18n/keys.toml` e nas três `i18n/locales/*.toml`, como `origin_name`, `origin_role` e `origin_body`, e `OriginSheetView` escolhia entre eles com um `if origin == "FARIAS"`.

Isso contradizia a regra 6 do `AGENTS.md`: o catálogo de i18n é da **interface**, e texto de **conteúdo** — nome de nó, enunciado, explicação, rótulo de opção de TAG — vem do servidor, das tabelas de tradução (ADR 0009). O texto de origem é conteúdo pela mesma razão que o nome de um nó é: descreve algo do currículo, não algo da tela, e crescer para uma segunda origem exigiria uma chave nova por campo e um release do app, em vez de uma linha na trilha.

## 2. Decisão

**O cartão de origem sai do catálogo e vai para tabelas de tradução, como os nós e os desafios.** Três migrações, nesta ordem, para produção não quebrar no meio:

- `0046_origens_de_desafio.sql` (pública) cria `challenge_origins(id, created_at, updated_at)` e `challenge_origin_translations(origin_id, locale, name, role, body, created_at, updated_at)`, com o trigger `set_updated_at` nas duas. Não mexe em `challenges.origin`.
- `0047_origens.sql` (privada, `logn-conteudo`) insere a origem já em uso hoje — `FARIAS` — e o texto nas três línguas, gerada por `gen_conteudo.py` a partir de `trilha/origens.json` e `trilha/origens.<locale>.json`.
- `0048_origem_referencia_cartao.sql` (pública) fecha a referência: `ALTER TABLE challenges ADD CONSTRAINT fk_challenges_origin FOREIGN KEY (origin) REFERENCES challenge_origins(id)`.

A ordem existe porque `challenges.origin` já tem `FARIAS` em produção sem linha pai nenhuma: aplicar a FK antes do conteúdo quebraria a migração no banco de produção. Criar a tabela vazia primeiro (0046), preencher (0047) e só então fechar a referência (0048) deixa cada passo aplicável sozinho, na ordem certa.

**O cartão viaja dentro de `GET /api/v1/nodes`.** O corpo passa de uma lista solta para `{"nodes": [...], "origins": [...]}`: o cartão de origem é conteúdo da trilha, não uma rota à parte para servir um selo que só aparece dentro dela. `repository.GetOriginCards` lê `challenge_origins` e `challenge_origin_translations` na língua pedida, no mesmo padrão de `GetSkillNodes` — origem sem tradução na língua fica de fora, e o cliente decide o que fazer com um id sem cartão.

**`internal/content/validate.go` cobra o cartão nas três línguas.** Todo desafio com `origin` não vazio precisa de um cartão em `trilha/origens.json` (pt-BR) e `trilha/origens.en.json`/`origens.es.json`, e o erro nomeia o desafio e a língua que falta — não só o id da origem, porque é o desafio que quem lê o achado está editando.

**O Core resolve o id para o cartão, com um fallback que nunca trava a partida.** `Model.origins: Vec<OriginCard>` chega pelo `/nodes` e pela semente empacotada (`TRAIL_SEED_VERSION` sobe de 2 para 3). `Model.origin_sheet` continua a ser só o id — é estado de navegação, não texto — e quem monta o `MatchViewModel` (`origin_sheet: Option<OriginCard>`) procura o id em `Model.origins`; sem achar, abre um cartão com o próprio id no lugar do nome e o resto vazio, em vez de recusar abrir. `origins_seen` continua chaveado pelo id: a cortesia de pausar o relógio na primeira leitura não muda com o texto, só com a identidade da origem.

**O iOS só mostra o que chegou.** `OriginSheetView` lê `name`, `role` e `body` do `OriginCard` do ViewModel; o `if origin == "FARIAS"` some. `origin_name`, `origin_role` e `origin_body` saem de `i18n/keys.toml` e das três locales — o grupo `origin` fica com `eyebrow`, `pause_notice` e `close`, que são interface de verdade: o rótulo da tarja e o aviso de pausa não descrevem uma origem específica.

## 3. Alternativas descartadas

* **Manter o texto no catálogo, com uma chave por origem.** Escala mal: origem nova vira chave nova em quatro arquivos e um release do app, exatamente o problema que a regra 6 existe para evitar em nome de nó e enunciado.
* **FK direto na 0046, sem esperar o conteúdo.** Quebra a migração em produção: `challenges.origin` já tem `FARIAS` sem `challenge_origins` correspondente.
* **Rota própria para os cartões de origem** (`GET /api/v1/origins`). Mais uma chamada de rede para um dado que só faz sentido junto da trilha, e mais uma coisa para a semente empacotada replicar à parte.
* **Cair em branco quando o id não tem cartão.** Um id sem cartão — conteúdo atrasado em relação ao app, ou id que mudou — não pode impedir o jogador de ver o selo que ele tocou; o fallback para o próprio id é pior cosmeticamente, nunca funcionalmente.

## 4. Consequências

* Origem nova é conteúdo: entra em `logn-conteudo/trilha/origens.json` e nas duas locales, passa por `just content-check` e vira migração por `gen_conteudo.py`, como um nó ou um desafio.
* `TRAIL_SEED_VERSION` subiu de 2 para 3; a semente empacotada e `tools/seed_bundle.py` precisam ser gerados de novo, e um app antigo que leia uma semente da versão 3 a ignora, como já fazia com qualquer versão desconhecida.
* `TestTrailSeedMatchesTheDatabase` compara também os cartões de origem de cada língua da semente com o que a API serviria.
* O `NOTICE` do repositório público perde nome e instituição: fica só "Parte dos problemas tem origem em exercícios de Farias." — o texto completo, com nome e contexto, mora em `logn-conteudo`, que é privado.
