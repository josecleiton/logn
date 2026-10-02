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

## A FFI numera variante por posição

O bincode não grava o nome da variante, só a posição dela no enum; o código gerado pelo `codegen` escreve esse número. Tirar ou reordenar uma variante no meio de um tipo que atravessa a FFI mudaria o protocolo em silêncio se houvesse descasamento de versão.

**Por que reordenar é seguro hoje:** os dois lados (Swift e Rust) vão no mesmo binário, gerados e compilados juntos pelo `build-ios-ffi`, então nunca discordam. O que vai para o disco para ser lido depois é JSON, e não bincode.

**Regra de estabilidade (JSON):** no JSON, a serialização se baseia no nome. Portanto, não se pode renomear variante nem campo de tipos persistidos (`OfflineSnapshot`, `SkillNode`, `Challenge`, `GameEvent` e `NodeStatus`). Se precisar de um campo novo, use `#[serde(default)]`.

**Situações que exigiriam mudar essa decisão (exigiriam "enum só cresce no fim" e travas rígidas):**
- Se algum estado passar a ser persistido em bincode.
- Se o xcframework passar a ser distribuído com versão própria e as pontas puderem desatualizar.
- Extensão (ex: widget) ou relógio trocando bincode com o app.
- Se criarmos um shell Android compilado em um pipeline separado.

## Lançamento no Android (ADR 0022)

O lançamento público passou a ser no Google Play. A conta do Play Console, de organização,
já está verificada. A landing já tem os botões do Google Play, a lista de espera do iPhone
(fechada até `LOGN_API_ORIGIN`) e `/account/delete/`. Falta, nesta ordem:

- **Backend:** feito, desligado até configurar. Ficam a lista de espera
  (`enable_waitlist`), a compra pelo Play e o job das anuladas (`play_package_name`) e o
  login com Google no Android (`google_web_client_id` + `google_android_client_ids`). Falta
  o Core mandar a compra na forma do Play e mapear `purchase_pending` e
  `store_unavailable`.
- **Documentos legais:** substituir a v4 com texto neutro de loja, Google como processador,
  reembolso pelo Google Play e a seção da lista de espera. Só depois a landing abre a
  lista.
- **Cliente Android:** a pasta inteira. Os rótulos de exclusão têm de bater com
  `/account/delete/`. Para destravar produto e `purchaseToken` reais, basta um AAB mínimo
  com billing em teste interno, e ele pode vir antes do app completo.
- **Play Console:** convidar a conta de serviço do Cloud Run, criar os produtos e preencher
  Segurança dos dados com `https://logn.sh/account/delete`.
- **iPhone:** entra quando houver 100 confirmados na lista ou receita no Android que pague a
  conta da Apple, o que vier primeiro.

## Antes de enviar para a loja

O que saiu em setembro de 2026 e deixou uma ponta aberta:

- **Conteúdo em três línguas** (ADR 0009) está no ar em português, inglês e espanhol. O
  espanhol passou por revisão de texto, mas ainda precisa de um revisor técnico nativo
  antes da loja.
- **Build de loja:** `just release-ios` empacota a trilha e os documentos a partir da
  produção e gera o `.ipa`, sem enviar. A semente (versão 3, com `track_id` e
  `requires_purchase`) e a cópia offline dos documentos foram geradas da produção em
  2026-09-30, com a v3; a v4 saiu no mesmo dia, e o `release-ios` gera as duas de novo.

## Trilhas pagas: o que falta depois do PR #2

O PR #2 entrega o ciclo inteiro: catálogo em grade, uma trilha por árvore, amostra grátis,
paywall em três pontos, compra validada pelo servidor contra a raiz fixa da Apple,
restauração, notificações de reembolso, licença offline de 30 dias com a escada de avisos,
pacote cifrado e armazenamento. Desenho e ameaças na ADR 0013; telas nos designs `LogN
Trilhas` e `LogN Validade Offline`. PRD:
[`specs/logn_trilhas_pagas_spec.md`](specs/logn_trilhas_pagas_spec.md).

Antes de vender a primeira trilha:

- **Termos e política novos.** A v4 está no ar desde 2026-09-30, não relevante: a
  política passou a citar o histórico da revogação manual (ADR 0021), e o app mostra a
  faixa de aviso. Antes dela, a v3, do mesmo dia, relevante: toda conta
  aceita de novo pelo bloqueio da ADR 0020, e `APP_PEDE_REACEITE` está ligado. A migração
  0062 do repositório de conteúdo substituiu o texto da 0061 antes de qualquer aceite,
  fechando três riscos da revisão automatizada: encerrar a conta por mau uso não revoga
  trilha comprada, uma versão nova não muda as condições do que já foi comprado, e a
  transferência internacional se apoia nos contratos de tratamento de dados, sem afirmar
  cláusulas da ANPD. Daqui em diante, com aceite registrado, correção é versão nova.
  A v3 saiu sem revisão de advogado, e ficam em aberto para ele: a base legal do aceite,
  os aceites apagados na exclusão, adolescentes, os portões de XP e a guarda dos
  registros de acesso.
  O texto promete uma coisa que o código ainda não faz: apagar `store_transactions`,
  `revoked_transactions` e `manual_revocations` 5 anos depois da transação (a primeira
  vence em 2031). Não é para agora, mas também não é para deixar de fazer: a política diz
  que apaga.
- **Revogação manual.** `just revoke` e `just appeal` (ADR 0021) revogam a licença e
  respondem à contestação pela rota interna, com a evidência gravada e o aviso por e-mail
  da seção 10.5. Nunca rodaram contra a produção. A política cita esse histórico desde a
  v4 (2026-09-30).
- **Login com GitHub.** No ar no servidor, e o login rodou de ponta a ponta num iPhone em
  2026-09-30, e o flag `sso_github_enabled` do PostHog, que mostra o botão, foi ligado no
  mesmo dia; desligá-lo esconde o botão sem build novo. Falta ver a
  exclusão de conta confirmada pelo GitHub, que revoga a autorização do app lá.
- **Notificações da App Store.** O backend está no ar com as trilhas; falta cadastrar no
  App Store Connect a URL das notificações, que é a `.run.app`, não o domínio atrás do
  Cloudflare.
- **App Store Connect.** Acordo de apps pagos, dados bancários e fiscais, e o produto não
  consumível de cada trilha, sem Compartilhamento Familiar.
- **Compra de ponta a ponta.** A compra pela loja só roda pelo Xcode, com o certificado do
  StoreKit Testing em `APPLE_XCODE_ROOT_CERT`, e ainda não foi exercitada inteira. A
  escada de validade offline também não foi vista na tela: o servidor emite a licença com a
  hora de agora, e só os testes do Core cobrem os 27 dias.

Decidido e em aberto, cada um esperando o seu momento:

- **Nivelador (nó zero).** Está no design e fica para um PR próprio, com PRD: é sistema de
  conteúdo novo, com vídeo, texto, figura, referências e o lembrete depois de dois erros.
- **Conciliação de reembolso.** O reembolso cuja notificação esgota as tentativas da Apple
  nunca chega. Falta conciliar pela App Store Server API.
- **Sandbox em produção.** Quem testa pelo TestFlight ganha a trilha de verdade. Se virar
  problema, a saída é tratar direito de Sandbox como temporário.
- **Cadeia verificada na hora de agora.** Restaurar um JWS cuja folha venceu falha fechado;
  a Apple verifica no `signedDate`.
- **Uma chave por versão da trilha.** A chave de um comprador abre o pacote daquela versão
  para qualquer um. É a ameaça que a spec deixa fora; o remédio é subir `content_version`.
- **Limite de aparelhos.** Hoje só registra. Um limite, se vier, vem depois de medir
  quantos aparelhos uma conta legítima usa.
- **Seletor no cabeçalho.** O app abre o catálogo pelo nome da trilha com chevron; o
  design tem um botão "Trilhas" à parte. O arquivo de design precisa refletir a escolha.

## Instituição de ensino no perfil

O design ("LogN Instituicao") põe a instituição no Perfil, abaixo dos números, como a
chave do placar por instituição e das inscrições em contest. Nenhum dos dois existe: a
aba Placar mostra dados de exemplo, e a aba Sede diz "UFC" para todo mundo.

Esta entrega é escolher na lista do e-MEC, no Perfil ou num passo pulável do cadastro,
ver a sigla no Perfil e provar o vínculo com um código no e-mail institucional, que não
vira login e fica guardado cifrado. Domínio de e-mail o e-MEC não traz: ele se cura a
partir de quem tenta verificar. Já diz de onde vêm os jogadores, que é o dado para
decidir onde o placar começa. A carência de troca e as métricas do card esperam o PRD
do placar, porque dependem do que ele conta; a aba Sede fica como está até lá.

Antes de codar: a tela do passo no cadastro, que o design não tem, e a chave nova
`INSTITUTION_EMAIL_KEY` no Secret Manager. A política de privacidade ganha versão nova,
de novo sem revisão jurídica.

O risco para quem joga é baixo: entra depois das trilhas pagas.

PRD: [`specs/logn_instituicao_spec.md`](specs/logn_instituicao_spec.md).

## O repositório é público

A segunda varredura (outubro de 2026) passou pela árvore, por todo blob do histórico e
pelos `refs/pull/*` atrás de segredo, conteúdo (regra 8), licença e segurança.

- **Segredo:** nenhum, nem no histórico.
- **Licença:** `NOTICE` completo (IBM Plex com `OFL.txt`, runtime Serde, boltffi, selos
  e logotipos de terceiros); o runtime do editor de design saiu do repositório.
- **Conteúdo:** a árvore ainda tinha gabarito da trilha gratuita nos testes do Core
  (`match_engine.rs`, `app.rs`); trocado por desafio inventado. O histórico tem esses
  gabaritos nas versões antigas e, em mensagem de commit, o e-mail real de uma pessoa
  e a história dela.

- **Segurança:** os limites de abuso que a revisão apontou entraram com a migração 0069
  (`/sync` com teto de eventos e vagas, falhas de OTP somadas entre reenvios, IPv6 por
  /64, tentativas de senha por e-mail e IP, envio de código por IP e teto global).

Por isso a abertura não vira a chave deste repositório: o GitHub guarda os
`refs/pull/*`, que ninguém apaga, e eles seguram os commits antigos. O histórico foi
reescrito com `git filter-repo` (mensagens e gabaritos antigos; a autoria ficou como
era) e publicado num repositório novo em 2026-10-02; o antigo fica privado, como
`logn-private`. O repositório de conteúdo lê este sem token agora. O
Workload Identity de `terraform/github_deploy.tf` é do repositório de conteúdo e não
muda; o repositório novo só precisa do environment `deployment` e do segredo
`CONTENT_DEPLOY_DISPATCH_TOKEN` outra vez.
