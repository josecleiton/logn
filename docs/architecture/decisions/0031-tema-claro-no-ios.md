# ADR 0031: Tema claro no iOS, com as regras do Android

Estende a ADR 0030 ao iOS. Substitui, nela, a consequência "O iOS segue escuro".

## 1. Contexto

A ADR 0030 deu o claro ao Android e deixou o iOS escuro, com quatro pendências conhecidas:
cinco telas forçando `.preferredColorScheme(.dark)`, o botão "Entrar com Apple" branco, o
símbolo do GitHub com a cor gravada no SVG e as sombras pretas do escuro. O dono pediu o iOS
logo depois de aprovar o claro do Android no aparelho.

As telas do iOS liam `LognDark` direto, em 705 pontos. A raiz não forçava tema, então login,
cadastro e abertura já recebiam do sistema o teclado e a barra de status claros, sobre telas
pintadas de escuro.

## 2. Decisão

**As regras são as da ADR 0030.** Segue o sistema, sem seletor; o claro é o `LognLight`; AA
em cada par que aparece na tela; texto e glifo na tinta; tinta também onde o traço é o único
sinal, e o laranja da marca onde número, visto ou posição já dizem o estado; balão de corpo
vivo com contorno no traço de 3:1; `textFaint` e `onStatus`. O escuro não muda.

**Mecanismo.** `LognColors`, no `LognDesignSystem.swift`: cada token é um `UIColor` com
provedor dinâmico, que escolhe o `LognLight` ou o `LognDark` pelo `userInterfaceStyle` do
traço em vigor. Como quem resolve é o traço, a mesma cor vale numa `View`, dentro de `Canvas`
e passada ao UIKit (o fundo do WebView do documento legal), e a troca de tema repinta sem a
tela perguntar. As telas leem `LognColors`, nunca `LognDark`, `LognLight` ou hex literal, e
`just ios-tokens-gate` barra. `Balloon.fill(_:)` e `Balloon.edge(_:)` dão o corpo e o traço
da letra no tema em vigor; `BalloonColor.filled(_:)` monta o balão cheio, como o
`filledBalloon` do Android.

**Decisões por tema na tela**, onde um token só não basta, lendo `colorScheme`:

- o contorno do balão de letra, só no claro; o nó e a cordinha vão no traço nos dois temas,
  e no escuro traço e corpo são a mesma cor. O balão do boss é o acento inteiro e não ganha
  contorno, como no Android;
- a sombra do aviso do topo, só no escuro;
- o exemplo do campo de apelido, `lineDim` no escuro e `textFaint` no claro;
- o visto da caixa marcada (`checkmark.square.fill`): vazado no canvas no escuro, como era;
  `onAccent` no claro, onde o canvas daria 2.4:1 sobre o acento;
- o exemplo dos campos que só têm título (cadastro, redefinição de senha, exclusão de
  conta): no escuro, o cinza do sistema, como sempre; no claro, `textFaint`, porque o cinza do
  sistema fica em 1.7:1 sobre o branco;
- o botão "Entrar com Apple": branco no escuro e preto no claro, as variantes que a
  diretriz da Apple pede para cada fundo. Ele só aparece com o login da Apple configurado;
- o "0 XP" do palco do erro, que cai na sombra do painel do veredito: `textMuted` no escuro e
  `textSecondary` no claro, onde a sombra escurece o fundo e o `textMuted` desce a 4.4:1. O
  Android não tem essa sombra sobre o palco.

**Sistema.** As cinco telas deixam de forçar o escuro. A cor de abertura (`LaunchBackground`)
ganha o canvas claro; o ícone do app fica escuro, como no Android. O símbolo do GitHub vira
molde, pintado com `textPrimary`, que no escuro é o hex que estava gravado. A sombra do painel
do veredito usa o `shadowSheet` do DS, que já tinha o valor claro. O tom global, que pinta o
cursor, os links e a seta de voltar, passa a `accentInk`. O interruptor é o do sistema: a
bolinha é sempre branca e o trilho ligado fica no `accent`; a posição diz o estado.

**Documento legal.** Sem o escuro forçado, o WebView passa o tema do app ao
`prefers-color-scheme`, e o `embed.css` já tem o bloco claro (ADR 0008). A cópia que viaja no
bundle se gera de novo (`just legal-bundle`) com esse CSS no ar.

**O claro não mudou o escuro.** Cada tela foi capturada no simulador antes e depois da
mudança, pelo mesmo roteiro: das 78, 55 saem iguais pixel a pixel, e as outras 23 diferem só
onde duas rodadas do mesmo build já diferem: os dados da conta nova, o relógio da partida e o
antisserrilhado de um ícone de tópico.

**Depois da revisão, o dono decidiu corrigir nos dois temas** defeitos que o claro mostrou e
que o escuro já tinha e escondia:

- a sombra do painel do veredito vira um bloco só (`.compositingGroup()`). Sem isso, cada
  filho projetava a sua, e o botão ganhava um halo dentro do painel. O raio passa ao do DS: o
  do SwiftUI é o desvio da gaussiana, metade do borrão do CSS, e `0 -16 48` dá 24, não 48;
- o tom global sobe para a raiz. Em `ContentView`, login, cadastro, redefinição de senha e
  termos ficavam com o cursor azul do sistema;
- as folhas de exclusão de conta e de compra como visitante pintam a folha inteira. Antes,
  sobrava em volta o fundo do sistema, e na de compra o botão da tela de trás vazava pelo
  vidro;
- os botões desligados da exclusão ficam em `textDim`, sem o esmaecimento do sistema por
  cima;
- bordas e rótulos seguem o Android:
  - a saída dos termos ganha a borda do botão secundário (`lineStrong`);
  - os campos de redefinição de senha passam a `line`;
  - a linha do armazenamento é `line`, no lugar do `Divider()` do sistema;
  - "Sair mesmo assim" passa a `textPrimary`;
- a célula "XP TOTAL" ocupa a altura da linha no perfil.

Conferido de novo por captura: no escuro, só as telas dessas correções mudaram, e as outras
diferem só nos dados da conta, no relógio e no antisserrilhado.

## 3. Alternativas descartadas

- **Cores no catálogo de assets (`.colorset` com aparência clara e escura).** É o idioma do
  Xcode, mas tiraria os valores do `LognDesignSystem.swift`, o arquivo gerado do DS, e
  repetiria cada hex em JSON, fora do alcance do portão. Só a cor de abertura fica no
  catálogo, porque o `Info.plist` a pede pelo nome.
- **Cada tela ler `@Environment(\.colorScheme)` e escolher a paleta.** Espalharia a pergunta
  pelas telas; com o provedor dinâmico, só os casos por tela acima leem o esquema.
- **`Color.resolve(in:)`.** É do iOS 17, e o app ainda roda no 16.
- **Exemplo próprio em todo campo, nos dois temas.** Uniformizaria o exemplo em `textFaint`,
  mas mudaria o escuro, onde o cinza do sistema e o `textDim` não são o mesmo pixel.
- **Interruptor desenhado à mão, com a bolinha `onAccent` do Android.** O controle do sistema
  é o que quem usa iOS reconhece, e o estado já está na posição da bolinha.

## 4. Consequências

- Toda tela do iOS se confere nos dois temas (`docs/testing/roteiro-simulador-ios.md`,
  "Tema"). No Xcode 26 o toque sintético do roteiro não achou os elementos do app; o roteiro
  descreve como dirigir o app instalado por XCUITest.
- iOS e Android leem a mesma paleta, em duas cópias: mudança de token vale nas duas.
- A cor da trilha que vem do servidor continua uma só para os dois temas (ADR 0030).
