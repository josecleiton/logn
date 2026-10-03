# Roadmap

[English](ROADMAP.md) · **Português**

O que está aberto no app, com o que já se sabe sobre cada coisa. Currículo não entra aqui:
enunciados, gabaritos e a auditoria deles vivem no repositório de conteúdo.

Ordem é por risco para quem joga, não por esforço.

## Os portões não obrigam caminho nenhum

A trilha tem teto e os portões estão calibrados no repositório de conteúdo. A `view()`
do Core destrava nó **só por XP**, o da trilha do nó (o da gratuita é o global menos o das
pagas); os pré-requisitos entram apenas para marcar o pai como conquistado — hoje eles
não travam de verdade.

Ou os pré-requisitos passam a travar de verdade, ou os números dos portões mudam. Entra
junto com a calibração do relógio: as duas coisas pedem jogar a trilha inteira.

## O relógio da questão nunca foi calibrado jogando

`seconds_for_template` em `match_engine.rs` dá 150s ao DRY_RUN, 90s ao SPOT_THE_BUG, 75s ao
COMPLEXITY_MATCH e 60s ao resto, e um desafio sobrepõe com `content.seconds`.

As réguas foram estimadas, não medidas com gente. A revisão cega mediu dois extremos no nó
7: um DRY_RUN com 109 segundos sobrando, e um COMPLEXITY_MATCH em que a leitura sozinha
consome quase todo o orçamento para um leitor devagar. Os números saíram de modelo de
palavras por minuto, não de jogador — por isso isto está aqui e não foi mexido.

A revisão por língua (setembro de 2026) reforçou o sinal: pela fórmula do guia de
conteúdo, sete itens passam de metade do relógio só lendo, nas três línguas, quase todos
COMPLEXITY_MATCH, em que as opções são varridas duas vezes. O espanhol sai mais longo e
fica no limite em mais casos. Enxugar o enunciado já foi feito onde dava sem mudar o
sentido; o resto pede cortar opção ou código, ou dar mais tempo ao template. A lista dos
itens está no repositório de conteúdo.

Calibrar exige jogar as sete trilhas e anotar onde sobra e onde falta.

## Ninguém lê o fim da partida

Toda partida com ao menos uma resposta fecha com `MATCH_END`: a jogada até o fim com
`solved`, a abandonada com `solved` e `abandoned: true`. Quem entra e sai sem responder
não gera evento. Mas nada consome esse evento ainda — o backend o ignora em
`ProcessEventXP`, e não há tela nem rota de histórico. O relatório pós-partida lê o
estado em memória, não o evento.

Quando o histórico existir, é ele quem decide o que a partida abandonada vale: derrota,
neutra ou escondida. O flag já está gravado para as três leituras.

## Trilhas pagas: o que falta

O PR #2 entregou o ciclo inteiro no iOS, e o cliente Android levou para o Google Play:
catálogo em grade, uma trilha por árvore, amostra grátis, paywall em três pontos, compra
validada pelo servidor, restauração, reembolso (notificações da App Store; job das
anuladas do Play), licença offline de 30 dias com a escada de avisos, pacote cifrado e
armazenamento. Desenho e ameaças na ADR 0013; telas nos designs `LogN Trilhas` e `LogN
Validade Offline`. PRD: [`specs/logn_trilhas_pagas_spec.md`](specs/logn_trilhas_pagas_spec.md).

As duas trilhas estão à venda no Play, cada uma com oferta de primeira compra. Falta:

- **Escada de validade offline na tela.** Compra e reembolso já rodaram de ponta a ponta
  no Play, mas a escada não foi vista: o servidor emite a licença com a hora de agora,
  então só os testes do Core cobrem os 27 dias.
- **Termos e política.** A v4 está no ar (2026-09-30), com texto neutro de loja: Google
  como processador, reembolso pelo Google Play e a seção da lista de espera. Saiu sem
  revisão de advogado, e ficam em aberto para ele: a base legal do aceite, os aceites
  apagados na exclusão, adolescentes, os portões de XP e a guarda dos registros de acesso.
  Com aceite registrado, correção é versão nova.
  O texto promete uma coisa que o código ainda não faz: apagar `store_transactions`,
  `revoked_transactions` e `manual_revocations` 5 anos depois da transação (a primeira
  vence em 2031). Não é para agora, mas também não é para deixar de fazer: a política diz
  que apaga.
- **Revogação manual.** `just revoke` e `just appeal` (ADR 0021) revogam a licença e
  respondem à contestação pela rota interna, com a evidência gravada e o aviso por e-mail
  da seção 10.5. Nunca rodaram contra a produção.
- **Login com GitHub.** No ar, com o flag `sso_github_enabled` do PostHog ligado desde
  2026-09-30. Falta ver a exclusão de conta confirmada pelo GitHub, que revoga a
  autorização do app lá.

Decidido e em aberto, cada um esperando o seu momento:

- **Nivelador (nó zero).** Está no design e fica para um PR próprio, com PRD: é sistema de
  conteúdo novo, com vídeo, texto, figura, referências e o lembrete depois de dois erros.
- **Uma chave por versão da trilha.** A chave de um comprador abre o pacote daquela versão
  para qualquer um. É a ameaça que a spec deixa fora; o remédio é subir `content_version`.
- **Limite de aparelhos.** Hoje só registra. Um limite, se vier, vem depois de medir
  quantos aparelhos uma conta legítima usa.
- **Seletor no cabeçalho.** O app abre o catálogo pelo nome da trilha com chevron; o
  design tem um botão "Trilhas" à parte. O arquivo de design precisa refletir a escolha.
- **Espanhol.** O conteúdo em três línguas (ADR 0009) está no ar em português, inglês e
  espanhol. O espanhol passou por revisão de texto, mas ainda precisa de um revisor
  técnico nativo.

## Instituição de ensino no perfil

O design ("LogN Instituicao") põe a instituição no Perfil, abaixo dos números, como a
chave do placar por instituição e das inscrições em contest. É o primeiro passo das duas
seções abaixo, e nada dele existe ainda: a aba Sede diz "UFC" para todo mundo.

Esta entrega é escolher na lista do e-MEC, no Perfil ou num passo pulável do cadastro,
ver a sigla no Perfil e provar o vínculo com um código no e-mail institucional, que não
vira login e fica guardado cifrado. Domínio de e-mail o e-MEC não traz: ele se cura a
partir de quem tenta verificar. Já diz de onde vêm os jogadores, que é o dado para
decidir onde o placar começa. A carência de troca e as métricas do card esperam o PRD
do placar, porque dependem do que ele conta; a aba Sede fica como está até lá.

Antes de codar: a tela do passo no cadastro, que o design não tem, e a chave nova
`INSTITUTION_EMAIL_KEY` no Secret Manager. A política de privacidade ganha versão nova,
de novo sem revisão jurídica.

PRD: [`specs/logn_instituicao_spec.md`](specs/logn_instituicao_spec.md).

## O placar é dado de exemplo

A aba Placar existe nos dois clientes, com as abas Global e Sede, a linha do jogador
fixa embaixo, e o telão completo: posição e time congelados, resolvidos, penalidade e as
colunas A–M, com a legenda dos balões. Tudo sai de `mock_data.rs`; o Core manda
`standings_are_sample: true`, e a tela mostra a tarja de dados de exemplo no topo. Não há
tabela, rota nem job no servidor.

O placar geral tem PRD: XP da trilha gratuita, desde sempre, aberto com 10 jogadores.
Todo mundo aparece como "jogador #N" até escolher um apelido definitivo no Perfil, e a
moderação é manual, por rota interna. O telão e a aba Sede saem da tela. Antes de codar
faltam aprovar as telas (canvas "LogN — Placar geral de XP") e a v5 da política.

PRD: [`specs/logn_placar_spec.md`](specs/logn_placar_spec.md).

O placar por instituição continua sem PRD: quem conta (aluno, professor, ex-aluno), a
carência de troca de instituição e as métricas do card do Perfil ("na instituição") vêm
com ele. Lá só conta vínculo verificado.

## Contest ainda não existe

A partida já fala a língua de um contest: problema por letra, balão, relógio,
`CONTEST ENCERRADO` no relatório, e `isFrozen` para a última hora no design system. Mas
cada partida é jogada sozinha, offline. Não há contest com outras pessoas: nem agenda,
nem inscrição, nem conjunto de problemas compartilhado, nem telão ao vivo.

A spec da instituição já fixa duas regras para ele: só vínculo verificado conta na
inscrição, e trocar de instituição nunca vale com contest em andamento. O resto pede PRD
próprio, depois do PRD do placar, porque o telão do contest é a mesma tela com dado de
verdade. Contest é a primeira função que precisa estar online numa hora marcada, o que vai
contra o desenho offline-first (ADR 0002): o PRD tem de dizer quanto vale uma resposta
dada offline durante um contest.

## O iPhone

O lançamento público passou a ser no Google Play (ADR 0022), no ar desde 2026-10-01. O
iPhone entra quando houver 100 confirmados na lista de espera ou receita no Android que
pague a conta de Apple Developer, o que vier primeiro. A lista de espera da landing está
aberta.

Até lá o iOS só roda pelo Xcode, e tudo que precisa da conta da Apple espera:

- **Build de loja.** `just release-ios` empacota a trilha e os documentos a partir da
  produção e gera o `.ipa`, sem enviar; gera de novo a semente e os documentos na hora.
- **App Store Connect.** Acordo de apps pagos, dados bancários e fiscais, e o produto não
  consumível de cada trilha, sem Compartilhamento Familiar.
- **Notificações da App Store.** O backend está no ar; falta cadastrar a URL das
  notificações, que é a `.run.app`, não o domínio atrás do Cloudflare.
- **Sign in with Apple.** Apagado até a conta paga (ADR 0017).
- **Compra de ponta a ponta.** A compra pela loja só roda pelo Xcode, com o certificado do
  StoreKit Testing em `APPLE_XCODE_ROOT_CERT`, e ainda não foi exercitada inteira.
- **Conciliação de reembolso.** O reembolso cuja notificação esgota as tentativas da Apple
  nunca chega. Falta conciliar pela App Store Server API.
- **Sandbox em produção.** Quem testa pelo TestFlight ganha a trilha de verdade. Se virar
  problema, a saída é tratar direito de Sandbox como temporário.
- **Cadeia verificada na hora de agora.** Restaurar um JWS cuja folha venceu falha fechado;
  a Apple verifica no `signedDate`.

## A FFI numera variante por posição

O bincode não grava o nome da variante, só a posição dela no enum; o código gerado pelo `codegen` escreve esse número. Tirar ou reordenar uma variante no meio de um tipo que atravessa a FFI mudaria o protocolo em silêncio se houvesse descasamento de versão.

**Por que reordenar é seguro hoje:** em cada plataforma os dois lados vão no mesmo binário. O iOS gera e compila os dois juntos pelo `build-ios-ffi`; o Android pelo `just android/generate`, e o `verifyGenerated` do Gradle recusa o build se o Core mudou depois do último generate (ADR 0023). O que vai para o disco para ser lido depois é JSON, e não bincode.

**Regra de estabilidade (JSON):** no JSON, a serialização se baseia no nome. Portanto, não se pode renomear variante nem campo de tipos persistidos (`OfflineSnapshot`, `SkillNode`, `Challenge`, `GameEvent` e `NodeStatus`). Se precisar de um campo novo, use `#[serde(default)]`.

**Situações que exigiriam mudar essa decisão (exigiriam "enum só cresce no fim" e travas rígidas):**
- Se algum estado passar a ser persistido em bincode.
- Se o Core passar a ser distribuído com versão própria e as pontas puderem desatualizar.
- Extensão (ex: widget) ou relógio trocando bincode com o app.
