# LogN - Agent Context & Guidelines

Este arquivo contém as diretrizes e regras arquiteturais do projeto **LogN**, servindo de contexto para agentes de IA que auxiliam no desenvolvimento.

## 📚 Documentação e Referências
- **Design System:** Localizado em `docs/design_system/`. Contém todas as especificações de UI/UX, cores, semântica e exportações de tokens (`tokens/`).
- **Decisões Arquiteturais (ADRs):** Localizadas em `docs/architecture/decisions/`. Antes de alterar paradigmas do sistema, leia as ADRs para entender o contexto das decisões passadas.
- **Teste no simulador:** `docs/testing/roteiro-simulador-ios.md`. Percorre o app inteiro tela a tela por linha de comando — geometria da janela, toque e arrasto sintéticos, atalhos de DEBUG e o que conferir em cada tela. Use antes de dar uma mudança de UI como pronta: os bugs mais caros deste projeto passaram por build, teste unitário e revisão, e só apareceram jogando.

## 🏗️ Arquitetura do Sistema
O LogN adota um padrão de **Monorepo** com separação clara de responsabilidades:
1. **Backend (Go):** Responsável pela validação do Sync, Auth e persistência. Focado em escalabilidade (concorrência em Go). Usa PostgreSQL com suporte a JSONB.
2. **Shared Core (Rust/Crux):** A "Mente" do cliente. Contém *toda* a lógica de negócios, regras de estado (Model) e o motor offline-first (Mini-Git). **Não usa UniFFI**. Usa uma ponte FFI nativa via `boltffi` e `bincode`, com tipagem gerada via `facet_typegen`.
3. **Clients (SwiftUI / Kotlin):** Camadas "burras" de renderização. Elas enviam eventos para o Core (`Event`) e recebem o modelo de visualização purificado (`ViewModel`).

## 🚨 Regras Rígidas de Implementação
1. **Zero Colisão de Nomes no Crux:** Qualquer novo tipo (Model, Event, ViewModel) adicionado em Rust deve possuir a anotação `#[derive(Facet)]` e `#[facet(fg::namespace = "LogN")]` para o *typegen* respeitar o namespace no iOS/Android.
2. **Offline-First via Cryptographic Chaining:** Todo evento do usuário de jogo deve possuir um Hash de integridade (`SHA-256(Hash(N-1) + Payload + Timestamp)`). O Go Backend deve apenas validar esse hash, nunca recalculá-lo para reescrever o histórico.
3. **Persistência de Desafios:** Desafios são armazenados no PostgreSQL em Go através de uma coluna polimórfica `JSONB`. Mutações no schema de desafios vão numa migração nova em `backend/schema/migrations/` (a `0000` é o schema de partida; nunca edite uma já aplicada), acompanhadas dos `CHECK CONSTRAINTS` de validação da estrutura JSON. Migração de schema nova precisa ser liberada pelo nome no `.gitignore`.
4. **Dependências Crux FFI:** Manter o padrão de FFI nativa deste repositório: a comunicação Rust <-> Swift é trafegada *exclusivamente* via bytes `[u8]` (Bincode) passando pelas funções exportadas em `boltffi::export`. 
5. **Toda tabela tem `created_at`. Toda tabela que sofre `UPDATE` tem `updated_at` também.**
   `TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP` nos dois. Vale para
   tabela nova e para `ALTER TABLE` que acrescente escrita a uma tabela que só era lida.

   O motivo é depuração, não burocracia: sem isso não dá para responder "quando esta
   linha apareceu" nem "isto mudou antes ou depois daquela migração", e as duas perguntas
   já apareceram investigando bug de conteúdo aqui.

   `updated_at` só vale se for mantido — ou um `trigger` de `BEFORE UPDATE`, ou a coluna
   no `SET` de todo `UPDATE`. Coluna que nasce com a linha e nunca mais anda mente pior
   que coluna nenhuma.

   **Estado atual, para quem for mexer:** a regra está cumprida desde a `0013_carimbos_de_tempo.sql`. Toda tabela tem `created_at` (agora com `NOT NULL` aplicado pela 0034). `users`, `skill_nodes`, `refresh_tokens`, `otps`, `user_progress`, `challenges`, `user_sync_state`, `skill_node_translations`, `challenge_translations` (0043), `challenge_origins` e `challenge_origin_translations` (0046), `tracks`, `track_translations`, `entitlements` e `entitlement_devices` (0049) têm `updated_at` mantido pelo trigger `trg_<tabela>_updated_at`, que chama `set_updated_at()` — a função já existe, tabela nova só cria o trigger dela. Ficam só com `created_at` as que nunca sofrem `UPDATE`: `game_events` (append-only por desenho, ADR 0002), `user_paid_challenges` (0032), `track_keys` (a chave de uma versão não muda), `store_transactions` e `revoked_transactions` (0050; a revogação sai por `DELETE`, nunca por `UPDATE`). `schema_migrations` resolve com `applied_at`. Em `otps` o `updated_at` anda a cada tentativa errada; por isso o intervalo entre envios usa `sent_at` (0031), não ele.

6. **Todo texto que o jogador lê ou ouve sai do catálogo de i18n, nunca de literal no código.**
   Vale para rótulo, botão, legenda, veredito e `accessibilityLabel`, em qualquer cliente.
   A chave vai em `i18n/keys.toml` (com `placeholders` tipados e `plural = true` quando o
   número muda a frase), o texto em `i18n/locales/pt-BR.toml` **e** `en.toml`, e
   `just i18n` gera os acessores — no iOS, `Str.<Grupo>.<chave>`. O Core manda chave
   (`StatusKey`), nunca frase: ele não sabe em que língua o app está.

   Nome de chave não pode ser palavra reservada do Swift ou do Kotlin (`continue`,
   `default`, `in`…): vira identificador no código gerado.

   O catálogo é da **interface**. Texto de **conteúdo** — nome de nó, enunciado,
   explicação, rótulo de opção de TAG — vem do servidor já na língua pedida, das
   tabelas de tradução (ADR 0009), e o cliente mostra como chegou. O Core também não
   escreve frase em volta do conteúdo: manda o tipo (`TrapKind`) e o texto cru, e o
   shell compõe com o catálogo. Conteúdo novo se escreve em `logn-conteudo/trilha/`,
   passa por `just content-check` e vira migração por `gen_conteudo.py`.

7. **Erro de rota que o app lê é `writeError(w, status, code)`, nunca `http.Error`
   com frase.** Ficam fora a rota interna de purga, o `/ready` e as páginas HTML de
   `/legal`, que ninguém do app lê. O corpo é `{"code", "message"}`; o código está em `backend/api_errors.go` e é
   contrato com app instalado — acrescente, não renomeie. O Core decide pelo código
   (`api_code` em `app.rs`), e erro que o jogador pode corrigir ganha `StatusKey`
   próprio.

8. **Este repositório vai ser público. Currículo, dado pessoal e material de terceiros
   não entram nele, em arquivo nenhum, em commit nenhum.** Engenharia é pública; a
   trilha vive em `logn-conteudo` (privado) e só de lá. Uma vez commitado, só
   `git filter-repo` tira; `git rm` não resolve. Por isso a regra é na entrada.

   **É currículo, e fica fora:** enunciado, código do desafio, `correct_line`,
   `expected_string`, opções e `correct_options` reais, explicação de erro, `watch_note`,
   nome e descrição reais dos nós, portões de XP (`required_xp`) e arestas do grafo
   real, ordem e contagem dos desafios por nó, ids `ch_NNN` ao lado de qualquer texto de
   conteúdo, texto do cartão de origem, o prompt que ensina a escrever desafio, e a
   trilha empacotada (`trail-seed.json`, já ignorada).

   **É dado pessoal ou de terceiro, e fica fora:** CNPJ, razão social, endereço e
   deliberação sobre natureza jurídica (`logn-conteudo/legal/entidade.md`); e-mail real
   de qualquer pessoa em teste, doc ou mock (use `@example.com`); nome completo,
   instituição e história de quem cedeu problema — a origem `FARIAS` é identificador e
   a atribuição pública é "exercícios de Farias", sem explicação; nome, pacote ou texto
   de qualquer outro produto ou empregador, em código, teste, doc, comentário ou
   mensagem de commit.

   **O que é permitido, e como:** schema, `CHECK CONSTRAINT`, motor de julgamento,
   telas, ADRs e specs sobre mecânica. Fixture e mock usam desafio de manual inventado
   (`ch_t01` soma de dois números, "Nó A" com portão 10), nunca um real com nome
   trocado. Doc que precisa de exemplo descreve a forma ("um desafio de busca tinha a
   linha errada"), não o caso. Plano de trabalho e dump de sessão vão para
   `logn-conteudo/docs/planos/`; `tmp/` é ignorado e continua assim.

   **Antes de todo commit**, `git grep -n -i -E` na árvore com os marcadores da última
   varredura (nomes reais de nó, `expected_string: Some(`, `correct_line:` com valor,
   `ch_[0-9]{3}` perto de texto, e-mails reais, CNPJ, nome de outro produto) tem de
   voltar vazio. Mensagem de commit entra na conta: ela também vai para o público.
   Na dúvida se algo é conteúdo ou engenharia, é conteúdo, e vai para `logn-conteudo`.

9. **Segurança: o código é público, o atacante lê tudo. Nenhuma mudança pode afrouxar
   uma defesa que existe, e toda rota nova nasce com as mesmas.** O que está abaixo é o
   que o backend faz hoje; mudar qualquer item é decisão com ADR, não ajuste de
   passagem.

   **Segredos e configuração.** Nenhum segredo, chave, URL de produção ou credencial
   em código, teste, doc, fixture, `.xcconfig` versionado ou mensagem de commit; o
   lugar é variável de ambiente (`.env`, ignorado) e Secret Manager. Fallback fixo só
   em desenvolvimento, e o servidor **aborta** em produção (`K_SERVICE` setado) se
   `JWT_SECRET`, `TRACK_KEY_SECRET` ou `APPLE_BUNDLE_ID` faltarem, ou se
   `APPLE_XCODE_ROOT_CERT` estiver setado; não crie outro fallback nem enfraqueça esse. Chave de
   telemetria e URL da API chegam ao iOS por `Local.xcconfig` (ignorado) e
   `Info.plist`, nunca em literal Swift. Placeholder em exemplo é visivelmente falso
   (`phc_SuaChaveAqui`). Não logue token, senha, OTP, e-mail completo nem corpo de
   requisição.

   **Autenticação e sessão.** Senha só com Argon2id nos parâmetros de
   `credentials.go`, comparação em tempo constante, e login contra `DummyHash` quando
   a conta não existe, para o tempo não denunciar e-mail. Erro de credencial, de OTP e
   de reset devolve o **mesmo código** para conta inexistente e senha errada; nada de
   "e-mail não cadastrado". OTP guardado como HMAC, nunca em claro, com
   `OTPMaxAttempts`, `OTPResendCooldown` e consumo atômico; refresh token guardado como
   SHA-256, rotacionado a cada uso, e reuso de token já rotacionado derruba todas as
   sessões da conta. JWT com `exp` verificado e algoritmo fixo; nunca aceite `none`
   nem leia o `alg` do token. No iOS, credencial vai ao Keychain (`keychainKeys` e o
   prefixo `track_key:` em `CoreWrapper.swift`), nunca a `UserDefaults`.

   **Autorização.** Toda rota autenticada tira o `user_id` do token, nunca do corpo:
   `/sync` sobrescreve `payload.UserID`, e é assim que toda rota nova se comporta.
   Exclusão de conta, aceite de termos, progresso e qualquer leitura por id só do
   próprio usuário. Rota interna (`/api/v1/internal/*`) só com OIDC do Cloud Scheduler
   validando emissor e `CLOUD_SCHEDULER_AUDIENCE`, e falha fechada (403) se a variável
   faltar; token estático compartilhado não é opção.

   **Integridade do jogo.** XP é constante do servidor (`XPPerAcceptedAnswer`); o
   cliente manda `is_correct`, `challenge_id`, `node_id`, nunca quantia. Pagamento é
   idempotente por `user_paid_challenges`; replay de evento não dá XP duas vezes. A
   cadeia de hash do `/sync` é validada, nunca recalculada para "consertar" histórico
   (ADR 0002), e a corrida em `user_sync_state` continua guardada por `ErrStaleChain`.
   Gabarito não sai para o cliente além do que o formato exige para julgar localmente.
   Só paga desafio que existe no banco, e trilha paga fora da amostra só com direito
   ativo (ADR 0013).

   **Trilha paga (ADR 0013).** O JWS da App Store só vale verificado contra a raiz fixa
   embutida (`internal/storekit`), nunca pelo nome da raiz que vem no token; algoritmo
   ES256 fixo. Transação revogada fica em `revoked_transactions`, fora da conta, e não
   volta por compra nem restauração. Conteúdo fechado só sai no pacote cifrado, e a
   regra do que é aberto mora em `openNode` (`repository.go`), num lugar só. A compra
   é do `appAccountToken` de quem comprou.

   **Entrada e transporte.** Query só com parâmetros posicionais do `pgx` (`$1`), sem
   concatenar string em SQL, inclusive em JSONB e traduções. Corpo limitado por rota
   (`authBodyLimit`, `syncBodyLimit`) e `http.Server` com timeouts; rota nova passa
   por `limitBody` e pelo `rateLimiter` que couber. `X-Forwarded-For` só conta atrás do
   Cloud Run (`K_SERVICE`), pegando a última entrada. Rota que o app lê responde
   `writeError` com código, nunca mensagem interna, stack ou SQL. Locale e qualquer
   valor que vire caminho ou chave vem de lista fechada (`locale.Negotiate`), nunca do
   pedido cru.

   **Saída HTML e e-mail.** E-mail só por `html/template` (auto-escape); páginas de
   `/legal` passam pela allowlist de tags de `internal/legal` e saem com CSP com hash,
   `X-Content-Type-Options: nosniff` e `Referrer-Policy: no-referrer`. Nada de
   `template.HTML` com texto de usuário nem de `innerHTML` com conteúdo do servidor no
   cliente.

   **Dependências e imagem.** Dependência nova é ADR, com `go.sum` e `Cargo.lock`
   versionados e sem `replace` apontando para fora. Imagem continua `distroless`,
   não-root, multi-stage, sem shell e sem segredo em camada. Migração aplicada não se
   edita (só comentário), e `schema_migrations` é a única fonte do que rodou.

   **Ao tocar em auth, sync, legal, internal ou limites**, o commit traz teste que
   prova o caso negativo (token de outro usuário, OTP na sexta tentativa, refresh
   reutilizado, corpo acima do limite, audiência errada). Um PR que remove ou
   relaxa um desses testes é recusado até vir com a ADR que justifica.

10. **Revisão cega por um segundo agente.** Ao terminar lógica complexa — validação de
    segurança, regra financeira no backend, mudança de banco, máquina de estados do Core
    —, não siga para o próximo passo nem comite. Mande o código e a spec a um subagente
    isolado, no papel de revisor ou de especialista em segurança, para uma análise
    independente de segurança, desempenho e aderência. Só avance depois de incorporar o
    que ele achar de crítico.

11. **Timeout do type-checker do SwiftUI.** "Unable to type-check this expression in
    reasonable time" derruba a compilação do iOS quando um `body` tem lógica densa ou usa
    token de design que não existe (`Radius.max` no lugar de `.cornerRadius`). Nunca
    encadeie `.filter{}.map{}` dentro da árvore de views: resolva num laço `for` ou numa
    variável computada antes. Sub-árvore complexa vira `@ViewBuilder` separado.


## 🔄 Fluxo de Trabalho do Agente
1. Ao iniciar, revise sempre se as dependências do `Crux` e o pacote `boltffi` exigem recompilação (`cargo build --features codegen`).
2. Atualize o `codegen` e rode-o se você tocar nas definições de tipagem (`shared_core/src/bin/codegen.rs`).
3. Gere e atualize ADRs em `docs/architecture/decisions/` ao introduzir novas bibliotecas centrais (ex: Lib de Auth) ou mudar arquitetura.

