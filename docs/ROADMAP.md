# Roadmap

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

## Antes de enviar para a loja

O que saiu em setembro de 2026 e deixou uma ponta aberta:

- **Conteúdo em três línguas** (ADR 0009) está no ar em português, inglês e espanhol. O
  espanhol passou por revisão de texto, mas ainda precisa de um revisor técnico nativo
  antes da loja.
- **Termos e política** (ADR 0008) estão na versão 1, sem marcador de rascunho, com
  aceite no cadastro e exclusão de conta de verdade. O PRD pede revisão jurídica do texto
  final; confirmar se ela aconteceu antes de publicar.
- **Build de loja:** `just release-ios` empacota a trilha e os documentos a partir da
  produção e gera o `.ipa`, sem enviar. Antes dele, `just seed-bundle --release`: os nós
  ganharam `track_id` e `requires_purchase`, e uma semente de antes disso não serve.

## Trilhas pagas: o que falta depois do PR #2

O PR #2 entrega o ciclo inteiro: catálogo em grade, uma trilha por árvore, amostra grátis,
paywall em três pontos, compra validada pelo servidor contra a raiz fixa da Apple,
restauração, notificações de reembolso, licença offline de 30 dias com a escada de avisos,
pacote cifrado e armazenamento. Desenho e ameaças na ADR 0012; telas nos designs `LogN
Trilhas` e `LogN Validade Offline`. PRD:
[`specs/logn_trilhas_pagas_spec.md`](specs/logn_trilhas_pagas_spec.md).

Antes de vender a primeira trilha:

- **Termos e política novos.** A spec pede, como pré-requisito, uma versão dos termos
  marcada como relevante, com as hipóteses de revogação da seção 7, e uma versão nova da
  política de privacidade, que passa a coletar o registro de aparelhos
  (`identifierForVendor`) e as compras. Nenhuma das duas foi escrita. Junto vem a folha de
  aceite pendente, que avisa o jogador quando uma versão nova entra em vigor (ADR 0008): o
  servidor já tem `pending` e `accept`, e a página já marca as seções novas.
- **Deploy.** O token do Cloudflare precisa de Zone · WAF · Edit para o `terraform apply`
  criar o rate limit da borda; o valor de `logn-track-key-secret` sobe pelo `gcloud` antes
  do deploy, senão a revisão nova não sobe; a URL das notificações da App Store é a
  `.run.app`, não o domínio atrás do Cloudflare.
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

## O repositório ainda é privado

Ele está preparado para ser público — Apache 2.0, `TRADEMARKS.md`, `NOTICE`, conteúdo
separado, gitleaks limpo. Antes de virar a chave, uma limpeza pela regra 8 do AGENTS.md:
testes ainda usam nomes reais de nó da trilha — em `backend/internal/content/validate_test.go`
e nos testes de `shared_core/src/app.rs` — e precisam trocar por nós de manual inventados.
Como já estão no histórico, sair da árvore não basta: é `git filter-repo`.
