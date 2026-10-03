# LogN - PRD: Placar geral de XP

A aba Placar existe nos dois clientes e mostra dado de exemplo: `mock_data.rs`, com
`standings_are_sample: true` e a tarja no topo. Este PRD troca a aba Global por um placar
de verdade, ordenado por XP da trilha gratuita, e tira da tela o que não tem dado: o telão
ICPC e a aba Sede.

O telão volta com o contest, e a Sede com a instituição (`logn_instituicao_spec.md`). Os
dois ficam fora daqui (seção 10).

As telas estão no canvas "LogN — Placar geral de XP" no claude.ai. Elas cobrem os estados
da seção 7, e o design é a fonte de como eles ficam. A seção 11 guarda o pedido que deu
origem às telas.

## 1. Conceito

- **O que ordena:** o XP da trilha gratuita, desde sempre. Todo mundo tem o mesmo teto,
  e comprar trilha não sobe ninguém no placar.
- **Quem aparece:** toda conta com XP gratuito acima de zero. Ninguém escolhe aparecer, e
  ninguém sai sozinho.
- **Como aparece:** como "jogador #4821", um número sorteado, até escolher um apelido no
  Perfil. O apelido é uma escolha só, e não troca.
- **Quando abre:** com 10 jogadores na lista. Antes disso, o jogador vê a própria linha e
  quantos faltam.

## 2. Decisões

| Tema | Decisão |
|---|---|
| Métrica | XP da trilha gratuita. As pagas ficam de fora: o placar não premia quem comprou. |
| Janela | Desde sempre. Semanal ou por temporada entra quando houver gente para encher uma semana. |
| Desempate | Quem chegou primeiro ao XP (`free_xp_reached_at`), depois `id`. |
| Abas | Só a Global. O telão e a aba Sede saem da tela; tipos e mock ficam no Core para o contest. |
| Abertura | Com 10 contas visíveis com XP > 0. Antes disso, "faltam K", com a linha do jogador. |
| Visibilidade | Toda conta com XP > 0. Sem interruptor de sair. |
| Objeção | Por e-mail, pelo contato da política; a remoção é a ação `hide` da rota interna (seção 8). |
| Nome padrão | "jogador #N". `N` é sorteado e guardado uma vez, nunca derivado do `user_id`. O servidor manda o número; o shell compõe a frase com o catálogo. |
| Apelido | Opcional, no Perfil, para qualquer conta, com XP ou sem. O convidado não escolhe. |
| Formato | Único, de 3 a 20 caracteres, `a-z0-9_`. A entrada é passada para minúsculas antes de validar. |
| Nomes reservados | Lista fixa no Go (seção 5.3). Barra o nome exato e ele seguido só de `_` ou dígitos. |
| Troca | Não há. O apelido é definitivo, e não existe botão de voltar ao anônimo. |
| Moderação | Sem filtro de palavrão. Apelido ofensivo volta ao número pela ação `anonymize` da rota interna, e isso queima a chance de escolher outro. |
| Perfil | O cabeçalho mostra o apelido, ou "jogador #N", com o e-mail na linha de baixo. |
| Convidado | O Placar mostra "entre para aparecer no placar". |
| Exclusão pedida | A conta some do placar na hora do pedido, não na purga. |
| Posição fora do Placar | Em lugar nenhum: nem Perfil, nem notificação. |
| Offline | O app guarda a última lista no disco e mostra "atualizado há X". Sair da conta apaga. |
| Atualização | Ao abrir a aba e ao puxar para atualizar. O `/sync` não traz o placar. |
| Política | v5 sem `material`, com faixa de aviso (seção 3). |
| Cadeia de hash | Não entra. Apelido não é evento de jogo. |
| Telemetria | Nenhum evento novo. |

## 3. Pré-requisitos

- **Telas:** o canvas cobre todos os estados da seção 7 menos o "oculto", que reaproveita
  a linha fixa sem posição. Precisa da sua aprovação antes de codar o cliente.
- **Política de privacidade, v5:**
  - Entram como dados tratados:
    - o número sorteado;
    - o apelido, quando houver;
    - a exibição de XP, posição e nome para outros jogadores.
  - Base: legítimo interesse.
  - O dado é pseudônimo: o número não diz quem é a pessoa, e o apelido é escolha dela.
  - A objeção sai por e-mail, e a remoção é manual. O apelido e o número vão embora na
    exclusão da conta, menos o apelido removido por moderação, que fica guardado para
    ninguém reusá-lo.
  - O texto vive em `logn-conteudo/legal/` e sai por `gen_documentos_legais.py`, sem
    `material`.
  - **Não passa por revisão jurídica.** O ponto "sem opção de sair do placar" entra na
    lista de pendências para o advogado no `docs/ROADMAP.md`.
- **Migração:** a próxima livre depois da 0069, liberada pelo nome no `.gitignore`.

## 4. Modelo de dados

### 4.1 Colunas novas em `users`

| Coluna | Tipo | Regra |
|---|---|---|
| `free_xp` | `INT NOT NULL DEFAULT 0` | `CHECK (free_xp >= 0)`. Anda junto com `global_xp` quando o desafio pago é da trilha gratuita. |
| `free_xp_reached_at` | `TIMESTAMPTZ` | Hora em que `free_xp` chegou ao valor atual. Nulo enquanto for zero. |
| `anon_number` | `INT NOT NULL UNIQUE` | Sorteado no cadastro. `CHECK (anon_number > 0)`. |
| `nickname` | `VARCHAR(20) UNIQUE` | Nulo até escolher. `CHECK (nickname ~ '^[a-z0-9_]{3,20}$')`. |
| `nickname_burned_at` | `TIMESTAMPTZ` | Preenchido pela ação `anonymize`. Com ele, a conta não escolhe mais. |
| `leaderboard_hidden` | `BOOLEAN NOT NULL DEFAULT false` | Ligado pela ação `hide`. |

- `updated_at` de `users` já é mantido por `trg_users_updated_at`.
- A busca do placar ganha um índice em `(free_xp DESC, free_xp_reached_at, id)`, parcial em
  `free_xp > 0`.

### 4.2 Sorteio do número

- É feito no Go, com `crypto/rand`, entre 1000 e 9999.
- Em conflito com `UNIQUE`, sorteia de novo. Depois de algumas colisões seguidas, a faixa
  sobe um dígito.
- O número não sai do `user_id` nem da ordem do cadastro, para não dizer quando a conta
  nasceu.

### 4.3 Migração das contas existentes

- `free_xp` = soma dos desafios em `user_paid_challenges` cujo nó é de trilha gratuita,
  vezes `XPPerAcceptedAnswer`.
- `free_xp_reached_at` = o maior `created_at` desses desafios.
- `anon_number` é sorteado para cada conta. A coluna nasce nula, recebe valor, e só
  depois ganha `NOT NULL` e `UNIQUE`.
- A migração confere que `global_xp` menos o XP das pagas bate com o `free_xp` calculado.
  Se não bater, para. Uma diferença ali é bug de XP, e não pode virar placar.

### 4.4 `leaderboard_actions`

Só recebe `INSERT`, no molde de `license_actions` (0063), com `created_at` apenas.

| Coluna | Regra |
|---|---|
| `user_id` | `REFERENCES users (id) ON DELETE CASCADE` |
| `action` | `anonymize`, `hide` ou `unhide` |
| `nickname` | O apelido no momento da ação, para saber o que foi moderado. |
| `reason` | Texto curto, de 1 a 2000 caracteres. |
| `actor` | A conta de serviço tirada do token (ADR 0021). |

### 4.5 `blocked_nicknames`

O apelido moderado não volta para a fila, nem depois que o dono exclui a conta. A ação
`anonymize` grava o apelido aqui, e o `PUT` recusa qualquer valor desta tabela.

| Coluna | Regra |
|---|---|
| `nickname` | `VARCHAR(20) PRIMARY KEY`. |
| `created_at` | Só ela: a linha nasce e não muda. |

Não tem FK para `users`, de propósito, no molde de `manual_revocations`: a cascata de
`leaderboard_actions` levaria o bloqueio junto com a conta.

## 5. Backend

### 5.1 XP

Em `ProcessEventXP`, o bloco que soma `XPPerAcceptedAnswer` a `global_xp` passa a fazer
mais uma coisa quando o desafio é de trilha gratuita: somar a `free_xp` e gravar
`free_xp_reached_at = now()`. As duas escritas ficam na mesma transação, depois do mesmo
`INSERT ... ON CONFLICT DO NOTHING` em `user_paid_challenges`. Replay de evento não anda o
placar, pelo mesmo motivo que não paga XP duas vezes.

### 5.2 `GET /api/v1/leaderboard`

- Autenticada. Quem joga como convidado não chama a rota.
- `rateLimiter` por IP, na faixa da rota de trilhas. Sem corpo.
- `Cache-Control: private, no-store`, porque a resposta traz a linha do jogador.

| Campo | Conteúdo |
|---|---|
| `open` | `true` com 10 ou mais contas visíveis. |
| `missing` | Quantas faltam para abrir, quando `open` é `false`. |
| `rows` | Top 100: `rank`, `anon_number`, `nickname` (ou nulo), `free_xp`, `is_me`. Vazio quando `open` é `false`. |
| `me` | A linha do jogador, com `rank` nulo se ele ainda tem XP 0 ou está oculto, e `hidden`. |
| `generated_at` | A hora do servidor, que vira o "atualizado há X". |

- O filtro de visível é `free_xp > 0`, `deletion_requested_at IS NULL` e
  `NOT leaderboard_hidden`.
- A posição do jogador é a contagem de visíveis à frente dele, mais um. Quem está oculto
  vê a própria linha sem posição.
- O `user_id` sai do token, nunca da query.
- A resposta não traz `user_id` de ninguém, nem o e-mail.

### 5.3 `PUT /api/v1/profile/nickname`

- Autenticada, com `limitBody` e `rateLimiter` por usuário.
- Corpo: `{"nickname": "..."}`.
- Validação, nesta ordem:
  1. Conta que já escolheu, ou perdeu a chance, ouve `nickname_locked` antes de tudo.
     Senão ela poderia sondar quais apelidos a moderação bloqueou.
  2. Passa a entrada para minúsculas (só ASCII) e tira espaço das pontas.
  3. Confere o formato.
  4. Confere a lista de reservados.
- Gravação: um `UPDATE` com `nickname IS NULL AND nickname_burned_at IS NULL` e o apelido
  fora de `blocked_nicknames` no `WHERE`. O bloqueio entra no próprio `UPDATE` para uma
  moderação no meio do pedido não escapar, e a unicidade é garantida pelo índice, não por
  uma leitura antes.

| Código | Quando |
|---|---|
| `nickname_invalid` | Formato fora da regra. |
| `nickname_reserved` | Lista reservada, ou apelido moderado antes. |
| `nickname_taken` | Violação de `UNIQUE`. |
| `nickname_locked` | A conta já tem apelido ou teve a chance queimada. |

- Os quatro entram em `api_errors.go` e no `api_code` do Core, e ganham `StatusKey`
  próprio, porque o jogador pode corrigir os três primeiros.
- **Nomes reservados:** `logn`, `admin`, `administrador`, `root`, `gm`, `mod`,
  `moderador`, `moderator`, `staff`, `equipe`, `team`, `suporte`, `support`, `oficial`,
  `official`, `sistema`, `system`, `jogador`, `player`, `jugador`.
  - Um apelido é recusado quando começa por um desses nomes e o resto é vazio ou só `_` e
    dígitos. Com isso caem `gm`, `gm_1`, `logn2` e `jogador_4821`.
  - Não é recusado quando o nome aparece no meio de outro, como `modesto` ou `teamaker`.

### 5.4 Rota interna

- `POST /api/v1/internal/leaderboard/actions`, com o mesmo `internal()` das rotas de
  licença: OIDC, emissor, audiência e a conta `ADMIN_SERVICE_ACCOUNT`.
- Corpo: `action`, `reason` e **exatamente um** identificador da conta: `user_id`,
  `nickname`, `anon_number` ou `email`. A objeção chega por e-mail e o que se vê no placar
  é o apelido ou o número, então quem opera não precisa consultar o banco de produção. O
  e-mail nunca vai para o log.
  - `anonymize` zera `nickname`, grava `nickname_burned_at` e põe o apelido em
    `blocked_nicknames`.
  - `hide` liga `leaderboard_hidden`.
  - `unhide` desliga `leaderboard_hidden`.
- Cada chamada grava uma linha em `leaderboard_actions` na mesma transação.
- Três receitas no `Justfile`, ao lado de `revoke`: `leaderboard-anonymize`,
  `leaderboard-hide` e `leaderboard-unhide`.

### 5.5 Exclusão de conta

O apelido e o número estão em `users`, e `leaderboard_actions` cai em cascata. O pedido
de exclusão já tira a conta do placar pelo filtro da seção 5.2. Só `blocked_nicknames`
fica: guarda o texto do apelido moderado, sem ligação com a conta.

## 6. Core

- Saem do `ViewModel`:
  - `standings_global`, `standings_home`, `user_standing` e `standings_are_sample`;
  - `scoreboard` e `contest_name` da tela.
- Os tipos e o mock do telão ficam em `domain.rs` e `mock_data.rs`, sem uso, para o
  contest.
- Entra `leaderboard: LeaderboardView`, com `#[derive(Facet)]` e o namespace `LogN`:

| Campo | Conteúdo |
|---|---|
| `state` | `SignedOut`, `Loading`, `Closed { missing }`, `Open`, `Unavailable` (offline sem lista guardada). |
| `rows` | `rank`, `anon_number`, `nickname: Option<String>`, `xp`, `is_me`. |
| `me` | A linha do jogador, com `rank: Option<i32>` e `hidden`. |
| `updated_at` | `generated_at` da última resposta, para o "atualizado há X". |
| `refreshing` | Pedido em andamento com lista na tela. |

- `Event::LeaderboardOpened` e `Event::LeaderboardRefresh` disparam o `GET`.
- A resposta vai para o disco em JSON, pelo `KeyValuePort`, numa chave da conta, e sai
  junto com o resto em `SignOut`. Por ser persistida em JSON, a regra de estabilidade do
  roadmap vale para ela.
- O Perfil ganha `profile_name`:
  - `Nickname(String)` ou `Anonymous(i32)`;
  - `can_choose_nickname`, quando a conta não tem apelido nem chance queimada.
- `Event::NicknameSubmitted(String)` chama o `PUT`. O erro volta como `StatusKey`, e o
  sucesso troca o `profile_name` sem esperar o próximo `GET`.
- O número e o apelido entram no retrato offline do Perfil.

## 7. Telas e estados

| Tela | Estado | O que mostra |
|---|---|---|
| Placar | Convidado | Chamada para criar conta. |
| Placar | Carregando, sem lista guardada | Esqueleto da lista. |
| Placar | Fechado | A linha do jogador e "placar abre com 10 jogadores · faltam K". |
| Placar | Aberto | Até 100 linhas (posição, nome, XP), a linha do jogador destacada e fixa embaixo quando ele está fora da janela visível ou fora do top 100. |
| Placar | Jogador com XP 0 | A lista, e no rodapé "acerte um desafio para entrar". |
| Placar | Oculto | A lista, e no rodapé a própria linha sem posição. |
| Placar | Offline com lista | A última lista, com "atualizado há X". |
| Placar | Offline sem lista | "Conecte para ver o placar". |
| Perfil | Sem apelido | "jogador #N", e-mail embaixo, botão "escolher apelido". |
| Perfil | Com apelido | O apelido, e-mail embaixo, sem botão. |
| Perfil | Chance queimada | "jogador #N", sem botão. |
| Escolher apelido | Digitação | Campo com a regra visível (3 a 20, letras, números e `_`). |
| Escolher apelido | Erro | Formato, em uso, reservado, cada um com o seu texto. O formato aparece no "Continuar"; em uso e reservado só no servidor, depois de "Confirmar", e a folha volta ao campo com o erro. Não há rota de checagem antes. |
| Escolher apelido | Confirmação | "Não dá para trocar depois", com o apelido em destaque, e confirmar ou voltar. |

Todo texto sai do catálogo (`i18n/keys.toml`, `pt-BR.toml`, `en.toml`). "jogador #N" é
uma chave com placeholder, e o número é formatado sem separador de milhar.

## 8. Moderação e objeção

- **Apelido ofensivo:** chega por e-mail ou é visto no placar. A ação é
  `just leaderboard-anonymize <user> <motivo>`. Ela volta o apelido ao número, queima a
  chance de escolher outro, e o apelido fica recusado para todo mundo.
- **Objeção ao placar:** chega por e-mail. A ação é `just leaderboard-hide <user> <motivo>`.
  `unhide` desfaz, se a pessoa pedir.
- Nenhuma das duas manda e-mail ao jogador nesta entrega (seção 12).

## 9. Segurança e testes

- Toda rota nova passa por `limitBody` e pelo `rateLimiter` que couber.
- Os erros saem por `writeError` com código.
- Query só com parâmetros posicionais.
- Testes do caso negativo, que entram no mesmo commit:
  - `PUT` do apelido sem token: 401.
  - Segundo `PUT` na mesma conta: `nickname_locked`.
  - Corrida de dois `PUT` com o mesmo apelido: um grava, o outro recebe `nickname_taken`.
  - Reservado exato, com `_` e dígitos depois, e o caso que passa (`modesto`).
  - Apelido moderado tentado por outra conta: `nickname_reserved`.
  - Rota interna com audiência certa e conta de serviço errada: 403.
  - Conta oculta, conta com exclusão pedida e conta com XP 0 fora de `rows`.
  - Replay de `MATCH_ANSWER` não anda `free_xp`.
  - Desafio de trilha paga anda `global_xp` e não anda `free_xp`.
  - Corpo acima do limite: 413.
- A migração, `ProcessEventXP` e a rota interna passam pela revisão cega de um segundo
  agente antes do commit (regra 10).

## 10. Fora do escopo

- Aba Sede e placar por instituição, que dependem de `logn_instituicao_spec.md`.
- Telão ICPC e contest.
- Placar por trilha paga, janela semanal ou por temporada.
- Filtro de palavrão. Nenhum pacote em Go cobre português; a moderação é manual.
- Interruptor de sair do placar.

## 11. Pedido ao designer

O pedido que deu origem ao canvas:

> No LogN, a aba Placar deixa de usar dado de exemplo e vira um placar geral de XP da
> trilha gratuita. Trabalhe sobre a tela Placar que já existe no projeto, com o design
> system do LogN.
>
> **O que sai:** o telão (colunas A–M, penalidade, legenda dos balões) e a aba Sede. Fica
> só a Global, sem abas. A tarja de "dados de exemplo" some.
>
> **Linha do placar:** posição, nome e XP. O nome é um apelido (`a-z0-9_`, até 20
> caracteres) ou, para quem não escolheu, "jogador #4821". Os dois formatos precisam
> conviver na mesma lista sem que o anônimo pareça erro. A linha do jogador é destacada
> e fica fixa embaixo quando ele está fora da área visível ou fora do top 100.
>
> **Estados do Placar:**
> 1. Aberto, com lista longa e o jogador no meio.
> 2. Fechado: ainda não há 10 jogadores. Mostra a linha do jogador e "placar abre com 10
>    jogadores · faltam 3".
> 3. Jogador com XP 0: a lista, e no rodapé um convite para acertar o primeiro desafio.
> 4. Convidado: chamada para criar conta e aparecer no placar.
> 5. Offline com a última lista: "atualizado há 2 h", discreto.
> 6. Offline sem lista: "conecte para ver o placar".
> 7. Carregando.
>
> **Perfil:** o cabeçalho, hoje com o e-mail, passa a mostrar o nome do placar (apelido
> ou "jogador #4821") em cima e o e-mail embaixo. Sem apelido, aparece um botão
> "escolher apelido". Com apelido, o botão some.
>
> **Escolher apelido (folha ou tela, como couber):**
> - Campo com a regra visível: 3 a 20 caracteres, letras, números e `_`.
> - Três erros: formato inválido, apelido em uso, apelido reservado.
> - Confirmação final, deixando claro que o apelido **não pode ser trocado depois**,
>   com o apelido em destaque e as opções confirmar e voltar.
>
> Verde fica só para resultado de resposta, como manda o design system. Entregar em
> português, para iOS e Android.

## 12. Em aberto

- **Aviso por e-mail na moderação:** a revogação de licença avisa o jogador (seção 10.5
  dos termos), mas `anonymize` e `hide` não avisam. Se a política v5 prometer aviso, a
  rota interna ganha o e-mail, no molde de `just revoke`.
