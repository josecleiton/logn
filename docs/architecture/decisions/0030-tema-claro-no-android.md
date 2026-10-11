# ADR 0030: Tema claro no Android, derivado dos tokens e seguindo o sistema

Substitui, na ADR 0023, o descarte de "Tema claro e paisagem" na parte do tema claro. A
paisagem continua fora.

## 1. Contexto

Os dois apps eram só escuros. O iOS força `.preferredColorScheme(.dark)`, o Android só
tinha `darkColorScheme`, e as telas liam `LognDark` direto. A paleta clara (`LognLight`)
existia nos tokens do iOS desde o Design System v2, sem uso. A ADR 0023 descartou o claro
porque o iOS não o tinha, e o iOS é a referência de cada tela.

O motivo de abrir agora é leitura e acessibilidade: quem joga no sol, ou prefere claro,
lê pior um app escuro. O lançamento é Android (ADR 0022), e não há conta Apple Developer,
então o claro nasce no Android e o iOS vem depois.

## 2. Decisão

**Segue o sistema, sem seletor.** `LognTheme` escolhe pelo `isSystemInDarkTheme()`. Um
seletor (Sistema, Claro, Escuro) pediria texto novo, um lugar na interface e persistência,
e cabe por cima disto quando fizer falta.

**Derivado, não redesenhado.** O claro é o `LognLight` do `LognDesignSystem.swift`: a
escala de superfície inverte, o acento é o mesmo hex, e veredito, sintaxe e balões
escurecem para manter contraste. Onde um mock do design system tem interruptor de tema
(Validade Offline, Trilhas, Splash), ele manda naquela tela.

**WCAG AA no claro, medido em cada par que aparece na tela.** 4.5:1 para texto; 3:1 para
o que não é texto e carrega estado sozinho. Hairline decorativa, borda de campo com rótulo
e controle desligado ficam como o design system definiu, nos dois temas. Onde um token do
DS não passa, a tela troca de token; o único valor do DS que mudou foi a paleta clara dos
balões, abaixo.

- **Glifo e texto em tinta.** A regra do DS (a base preenche e contorna, a `…Ink` escreve)
  não aparecia no escuro, onde base e tinta são o mesmo hex. O `_____` da lacuna, o sinal
  do diff de termos, a contagem do nó conquistado, o rótulo de origem, o ícone de pausa e
  o sinal das linhas da abertura escreviam com a base e passaram à tinta. A chave cheia de
  "Redefinir senha" fez o caminho inverso: é forma, e passou da tinta à base.
- **Tinta só onde o traço é o único sinal.** Aba ativa, opção marcada (tags, cartão do
  onboarding, trilha aberta no catálogo), foco do OTP e da saída prevista, cursor e a barra
  lateral da linha escolhida no código ficam em `accentInk`: a base do acento sobre o canvas
  claro dá 2.4:1, e a tinta, 5.9:1. Onde número, visto ou texto já dizem o estado, fica a
  base, com o laranja da marca: as barras de progresso (abertura, placar fechado, nível no
  perfil), o interruptor ligado e as caixas marcadas do cadastro e dos termos, as duas
  últimas com a bolinha e o visto em `onAccent`, como o texto do botão primário (7.5:1). A
  primeira versão pôs todo estado em tinta, e o claro perdeu a cor da marca; o dono pediu o
  laranja de volta. A linha do jogador no placar fica em tinta porque o fio de baixo encosta
  na aba ativa, e dois laranjas na mesma aresta destoavam. O nó ativo do mapa fica no
  acento: o estado dele está escrito na etiqueta ("INFLANDO · n/m"). No escuro base e tinta
  são o mesmo hex, e nada disso muda.
- **Balões: corpo vivo, traço a 3:1.** Cada letra tem duas cores no claro. O corpo é a cor
  do escuro. O traço (contorno de 1 dp do balão cheio, balão em aberto, nó, cordinha,
  aresta do mapa, ícone de traço) é ela com o mesmo matiz e croma em OKLCH, escurecida até
  3:1 sobre a folha do nó, o fundo mais escuro onde um balão aparece. Escurecer o corpo
  inteiro transformava laranja em marrom, amarelo em mostarda e lima em oliva: a paleta do
  handoff (3.6–6.9:1) e depois uma paleta viva a 3:1 sobre o canvas foram reprovadas pelo
  dono no aparelho. A letra escura dentro do corpo vai de 6.5 a 14:1; J clareia um fio para
  chegar a 4.5:1. No escuro corpo e traço são a mesma cor, e não há contorno. É a única
  mudança de valor do DS; a tabela do README e as cópias dos tokens foram junto.
- **Dois papéis na paleta**, para a tela não perguntar o tema:
  - `textFaint`: o texto mais apagado que ainda se lê (exemplo dentro do campo, versão do
    app). `textDim` no escuro; `textMuted` no claro, onde `textDim` fica em 3.6:1 e o DS o
    reserva a forma.
  - `onStatus`: a tinta sobre um preenchimento cheio que escurece no claro (`warn`,
    `wrong`): tarja sem rede e botão de excluir. `onAccent` no escuro; `canvas` no claro,
    como o mock de Validade Offline escreve, porque o texto escuro ali cai a 3.3:1.

  Três casos não cabem num papel e decidem pelo tema na própria tela: o exemplo do campo de
  apelido, que no escuro escreve em `lineDim` e no claro em `textFaint`; a sombra do aviso
  do topo, que no claro vira halo e sai (o DS só põe sombra em folha); e o visto vazado da
  caixa do cadastro, que no escuro mostra o canvas e no claro, onde o canvas daria 2.4:1
  sobre o acento, é `onAccent`.

**Mecanismo.** A paleta é um `LognPalette`, e `LognTheme` fornece a do tema por um
`CompositionLocal`. As telas leem `LognColors`, com os mesmos nomes de token, e nunca
`LognDark` ou `LognLight`. Cor lida dentro de `drawBehind`, `Canvas` ou `remember` é lida
antes da lambda. `just android/tokens-gate` barra `Color(0x…)` fora de `ui/theme`.

**Sistema.** Janela e splash do sistema saem de `values` e `values-night`; o fundo do ícone
fica escuro nos dois, porque é a marca. As barras do sistema têm ícones escuros no claro.
Trocar o tema do sistema recria a Activity, como antes; o estado do jogo mora no Core.

As folhas têm janela própria e não herdam a aparência das barras da Activity, então cada
uma pede os ícones pelo tema: a folha cheia (`Dialog`) pelo controlador da janela, a de
baixo pelas propriedades do Material3, que sem elas decidia pela cor do conteúdo e saía
com ícones escuros também no escuro. A folha de baixo aberta de dentro de um diálogo (a
exclusão, em "Gerenciar conta") herdava o escurecimento dele e somava 73% de preto ao
fundo; ela larga esse escurecimento e fica com o scrim de 32% das outras.

**Documento legal.** O `?embed=1` passa a seguir o `prefers-color-scheme` (ADR 0008). O
WebView do Android passa o tema do app à consulta pelo pai do tema
(`Theme.Material.Light` em `values`), e o iOS, que força o escuro, continua escuro.

**O escuro não mudou**, com duas exceções decididas pelo dono depois da revisão, as duas das
folhas acima: os ícones da barra de status sobre uma folha de baixo eram pretos sobre o
scrim quase preto (1.02:1) e ficaram claros; e o fundo da folha de exclusão vai de `030303`
a `070809`. O resto foi conferido tela a tela, com captura antes e depois no emulador.

## 3. Alternativas descartadas

- **Seletor no app, com padrão Sistema ou Escuro.** Mais texto, mais persistência, e o
  padrão escuro esconderia o claro de quem precisa dele.
- **3:1 em toda borda e hairline.** Escureceria as linhas do claro, e o claro teria outra
  linguagem visual que o escuro, que também não passaria nesses pares.
- **Ajustar valores do DS para fazer passar o que não passa.** O iOS herdaria valores que
  ninguém desenhou; trocar de token resolve sem isso. A exceção é a paleta dos balões, que
  não reprovava: estava escura demais e apagava os balões.
- **Corpo do balão escurecido até 3:1, sem contorno.** Foi a primeira versão viva: passa
  sobre o canvas, mas fica abaixo de 3:1 na folha do nó, e laranja, amarelo e lima viram
  marrom, mostarda e oliva.
- **Corpo vivo sem contorno.** Amarelo e prata ficam a 1.5 e 1.25:1 do fundo e somem no
  branco.
- **Todo estado em tinta.** Passa em tudo, mas tira o laranja da marca de barras, caixas e
  interruptor que já dizem o estado por número, visto ou posição.
- **Paleta global mutável em vez de `CompositionLocal`.** Funcionaria dentro de
  `drawBehind` sem ler a cor antes, mas esconderia onde a cor é lida, e não é o idioma do
  Compose.
- **Não recriar a Activity na troca de tema (`configChanges="uiMode"`).** Guardaria o
  estado local da tela, mas a janela, as barras e o WebView teriam de ser atualizados à
  mão.
- **iOS junto.** Sem conta Apple Developer e com o lançamento no Android, o iOS claro não
  chegaria a ninguém tão cedo.

## 4. Consequências

- Toda tela nova ou mexida no Android se confere nos dois temas
  (`docs/testing/roteiro-android.md`). Base no lugar de tinta só aparece no claro.
- A cor da trilha vem do servidor numa só, para os dois temas. Hoje toda trilha usa o
  padrão `#FF7A45`, que é o acento; uma trilha com cor própria vai precisar do par claro.
- A cópia do documento legal que viaja no build da loja sai da produção: o `.aab` gerado
  antes do deploy deste `embed.css` leva a cópia offline só escura.
- O iOS segue escuro, com pendências conhecidas para quando ganhar o claro:
  - cinco telas forçam `.preferredColorScheme(.dark)`, e a raiz não: a interface do
    sistema já segue o aparelho nas telas de entrada;
  - o botão "Entrar com Apple" é branco, e as diretrizes da Apple pedem o preto ou o
    contornado sobre fundo claro;
  - o ícone do GitHub tem `fill="#EDEEEF"` gravado no asset;
  - as sombras são `.black` com opacidade do escuro.
- A referência de cada tela continua o iOS no escuro; no claro, enquanto o iOS não tiver,
  é o Android contra o design system.
