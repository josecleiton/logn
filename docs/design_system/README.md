# Handoff: LogN — Design System v2

## Overview
LogN é um app de micro-aprendizado de programação competitiva: sessões de ~3 minutos que
reproduzem a gramática de um contest ICPC real (problemas por letra, balões por cor, vidas,
placar que congela na última hora). Este pacote descreve o sistema visual completo — cor,
tipografia, espaço, motion, componentes, quatro templates de questão e as telas-chave.

Duas plataformas, paridade total de geometria/cor/copy: **Jetpack Compose** e **SwiftUI**.

> **Nomenclatura:** *Impecable* é a ferramenta de design em que o sistema foi produzido e
> aparece como assinatura no cabeçalho do documento. O produto — e o namespace de código —
> é **LogN**. Se existir um sistema de marca Impecable anterior que o LogN deva herdar
> (logo, paleta institucional, tom de voz corporativo), ele **não** foi fornecido e não está
> refletido aqui; tudo abaixo foi definido do zero para o LogN.

## About the Design Files
Os arquivos `.dc.html` deste bundle são **referências de design criadas em HTML** —
protótipos que mostram aparência e comportamento pretendidos. **Não são código de produção
para copiar.** A tarefa é **recriar estes designs no ambiente do codebase alvo** (Compose /
SwiftUI), usando os padrões e bibliotecas já estabelecidos lá. Se ainda não existe ambiente,
escolha a arquitetura adequada e implemente.

Os arquivos em `tokens/` são a exceção: esses **são** para entrar no codebase quase como estão.

## Fidelity
**High-fidelity.** Cores, tipografia, espaçamento, estados e durações de animação são finais.
Recrie pixel-a-pixel usando os componentes nativos de cada plataforma.

---

## Marca

O nome mora dentro da notação: `O(log n)`. A marca faz esse `O` virar balão, e a cordinha
do balão virar a curva logarítmica — sobe rápido, depois estabiliza. Duas leituras numa forma
só: para quem compete é o balão do ICPC, para qualquer um é a curva de quem melhorou.

### Símbolo
Vetor em `brand/`, viewBox `0 0 96 122`. Quatro elementos, nesta ordem de z:
1. **Corpo** — `M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z`
2. **Brilho** — `M31 21 C27 27 25 33 25 40`, traço 5.5, cap redondo, branco @75%
3. **Nó** — `M41 76 L55 76 L48 91 Z`
4. **Cauda (curva log)** — `M48 90 C49 104 56 111 68 113 C78 115 84 115 90 116`, traço 5, cap redondo

| Versão | Arquivo | Quando |
|---|---|---|
| Accent | `logn-symbol-accent.svg` | padrão, sobre canvas ou surface |
| Monocromático | `logn-symbol-currentcolor.svg` | herda `currentColor` — documento, impressão, parceiro |
| Vazado | `logn-symbol-knockout.svg` | sobre o accent, em `onAccent` |

### Lockups
Não há SVG de lockup: monte compondo o símbolo com texto vivo em IBM Plex Sans SemiBold.
- **Horizontal (uso diário — header, loja, e-mail):** símbolo + `LogN` a 44sp/600, tracking −0.035em,
  gap de 14dp, alinhados pelo centro óptico. Altura do símbolo ≈ 1,27× a altura da caixa-alta.
- **Assinatura longa (abertura, institucional, rodapé):** `(` + símbolo + `log n` + `)` em IBM Plex Mono,
  parênteses em `textSecondary`, `log n` em `textPrimary`, símbolo no lugar exato do `O`.
- **Símbolo isolado:** só onde a marca já é conhecida — ícone de app, favicon, avatar.

### Slogan
**Reduza a complexidade das suas soluções.**

Em caixa-alta, IBM Plex Mono 10sp, `letter-spacing 0.1em`, `textSecondary`, sempre abaixo do
lockup horizontal com 12dp de respiro. Aparece na tela de entrada e em peças institucionais —
nunca no header do app, onde só o lockup cabe.

Funciona em duas leituras sem piscar para o leitor: quem é da área lê complexidade de tempo
— a grandeza que O(log n) mede; quem não é lê a promessa de simplificar. **Solução** é o
objeto certo porque cobre o currículo inteiro: algoritmo, arquitetura e infra têm complexidade,
"código" deixaria as duas últimas de fora.

**Variante em inglês: "Reduce the complexity."**
Não é tradução da linha em português — é a versão que funciona no idioma. Em inglês,
*complexity* sem qualificador num contexto de engenharia já lê como time complexity, então
o objeto fica implícito e a linha pode ser curta. Em português isso não acontece: "reduza a
complexidade" pede o objeto, daí "das suas soluções". Usar cada uma no seu idioma, nunca
traduzir uma na outra.

Descartadas e por quê, para ninguém restaurar mais adiante:
- *"Mastering technology bit-by-bit"* — percorrer um item de cada vez é O(n), exatamente a
  curva que o nome promete evitar.
- *"Reduzindo a complexidade da tecnologia"* — é a frase que toda consultoria de TI usa, e o
  duplo sentido morre no clichê.
- *"Reduza a complexidade"* sem objeto — em português, evocativa demais para a tela onde
  alguém decide se baixa o app (o inglês não tem esse problema; ver acima).
- *"Crack the complexity of everything you build"* — mais energia e mais perto do ICPC, mas
  *crack* não é o verbo que anda com *complexity* em code review, e 44 caracteres estouram a
  largura no iPhone SE.

### Escala e respiro
Mínimo **16px**. Abaixo de 24px remova o brilho especular (só corpo, nó e cauda).
Área de respiro em qualquer aplicação: a largura do corpo do balão em todos os lados.
Ícone de app: símbolo a ~54% da largura do tile, centrado opticamente (o centro visual fica
acima do geométrico por causa da cauda — desloque o símbolo ~4% para cima).

### O que não fazer
- Pintar com cor de balão — as treze são endereço de problema, não a marca
- Girar — o balão sobe na vertical e a curva só lê no eixo certo
- Distorcer — a proporção do corpo é 1 : 1,14
- Escrever o `O` da assinatura com tipo — na assinatura o `O` é sempre o símbolo

---

## Design Tokens

### Superfície e linha
| Token | Dark | Light | Uso |
|---|---|---|---|
| canvas | `#0B0C0D` | `#F6F6F4` | fundo da tela |
| surface | `#111316` | `#FFFFFF` | cards, listas |
| surfaceRaised | `#171A1E` | `#F0F1EE` | bottom sheet, header de tabela |
| line | `#24282D` | `#E2E3DF` | divisor padrão |
| lineStrong | `#343A41` | `#C6C8C2` | borda de card e de botão secundário |
| lineDim | `#4C535B` | `#A9AFB5` | borda tracejada de dropzone vazia |
| rowLine | `#16191C` | `#ECEDE9` | divisor interno de lista densa |

### Conteúdo
| Token | Dark | Light | Contraste |
|---|---|---|---|
| textPrimary | `#EDEEEF` | `#14161A` | 15.8:1 / 15.4:1 |
| textSecondary | `#99A0A7` | `#555B62` | 7.1:1 / 7.0:1 |
| textMuted | `#7E858D` | `#656B72` | 5.0:1 / 4.8:1 |
| textDim | `#5F656C` | `#7C838A` | **só forma e placeholder** — 3.6:1 no light, nunca em texto lido |

Número de linha em CodeBlock usa `textMuted`, não `lineDim` — é conteúdo lido.

### Ação
| Token | Dark | Light | Uso |
|---|---|---|---|
| accent | `#FF7A45` | `#FF7A45` | **preenchimento e borda** — botão, balão ativo, seleção, aresta ativa |
| accentInk | `#FF7A45` | `#A83C0B` | **texto e ícone** em accent |
| onAccent | `#160B05` | `#160B05` | texto sobre o preenchimento accent |
| buttonDisabled | `#1B1D20` | `#E6E7E3` | botão desabilitado |

O acento governa tudo que é tocável e o progresso, e é a mesma cor nos dois temas **enquanto
é preenchimento**. Como tinta de texto ele não alcança 4.5:1 sobre branco (fica em 2.4:1), então
o light usa `accentInk` `#A83C0B`.

> **Regra de tinta — vale para accent, correct, wrong, warn e info.**
> Fundo, borda e traço de forma usam o token base (`accent`, `correct`, …).
> Qualquer glifo — label, sigla, número, eyebrow, ícone de linha — usa o token `…Ink`.
> No dark base e tinta são idênticos, então a distinção só aparece no light: escrever o token
> base num texto passa despercebido no dark e quebra o contraste no light.
> Exceção: o coração da LifeBar e o balão preenchido são **forma**, não texto — usam o base.

### Veredito (exclusivo do juiz)
Cada veredito tem três tokens: a **linha** (borda, barra lateral, ícone preenchido), a **tinta**
(sigla, número, qualquer glifo) e o **tint** de fundo. No dark linha e tinta coincidem; no light
a tinta escurece, porque a cor de linha não alcança 4.5:1 sobre o próprio tint em corpo pequeno.

| Linha | Dark | Light | | Tinta | Dark | Light | | Tint | Dark | Light |
|---|---|---|---|---|---|---|---|---|---|---|
| correct | `#3DD68C` | `#0E8F52` | | correctInk | `#3DD68C` | `#0A6B3C` | | tintOk | `#0F2018` | `#E6F5EC` |
| wrong | `#FF5C5C` | `#C93636` | | wrongInk | `#FF5C5C` | `#A82424` | | tintErr | `#231113` | `#FBEAEA` |
| warn | `#F5C451` | `#8A5B00` | | warnInk | `#F5C451` | `#6E4800` | | tintWarn | `#221C0C` | `#FAF1DC` |
| info | `#5AA9FF` | `#1660C4` | | infoInk | `#5AA9FF` | `#124F9E` | | tintInfo | `#0D1B2B` | `#E6EFFB` |

**Regra dura:** verde e vermelho aparecem *apenas* como resultado de uma resposta, célula de
placar, ou **ação destrutiva irreversível** (excluir conta, descartar progresso) — e nesse caso
só dentro do bloco de confirmação, nunca no estado de repouso da tela. Nunca em navegação, nunca em estado neutro. `warn` é timer crítico e placar congelado;
`info` é teoria, dica e submissão pós-congelamento.

### Sintaxe de código
| Token | Dark | Light |
|---|---|---|
| synKeyword | `#C792EA` | `#7A28C4` |
| synFunction | `#82AAFF` | `#0A4FA8` |
| número | usa `warn` | usa `warn` |
| texto base | `textSecondary` | `textSecondary` |
| número de linha | `textMuted` | `textMuted` |

### Balões A—M — identidade, não estado
Terceira família de cor, e a única não-semântica. **Cada letra do contest carrega a mesma cor
em toda parte**: header da questão, placar, relatório. Indica *endereço*, não estado.

| Letra | Dark | Light | | Letra | Dark | Light |
|---|---|---|---|---|---|---|
| A | `#E4572E` | `#C43F19` | | H | `#F4A261` | `#B26320` |
| B | `#F5C451` | `#A67A00` | | I | `#9BC53D` | `#5F8410` |
| C | `#3DB2FF` | `#0B6FBF` | | J | `#D64550` | `#A3202B` |
| D | `#6BCB77` | `#2E8B45` | | K | `#7C8BFF` | `#4352C9` |
| E | `#C77DFF` | `#8A3FD1` | | L | `#D8DEE4` | `#5B646D` |
| F | `#FF6FB5` | `#C2367E` | | M | `#00B894` | `#007A61` |
| G | `#4ECDC4` | `#18867E` | | | | |

Proibido: usar cor de balão para veredito, seleção ou navegação; reatribuir uma letra dentro
do mesmo contest. Acesse sempre por `Balloon.of('C')`, nunca por hex literal.

### Sombra
Único uso: bottom sheet, e **só para cima**.
| Token | Dark | Light |
|---|---|---|
| shadowSoft | `rgba(0,0,0,0.60)` — `0 -10px 40px` | `rgba(20,22,26,0.10)` |
| shadowSheet | `rgba(0,0,0,0.65)` — `0 -16px 48px` | `rgba(20,22,26,0.14)` |

Cards e botões **não** usam sombra. Hierarquia vem de 1px de linha + um degrau de luminância.

### Tipografia
IBM Plex Sans (interface) + IBM Plex Mono (código, números, rótulos de sistema).

| Estilo | Família | Tamanho/Entrelinha | Peso | Tracking |
|---|---|---|---|---|
| displayLarge | Plex Sans | 40 / 44 | 600 | −0.03em |
| headlineMedium | Plex Sans | 24 / 30 | 600 | −0.02em |
| titleMedium | Plex Sans | 19 / 25 | 600 | 0 |
| bodyLarge | Plex Sans | 17 / 26 | 400 | 0 |
| bodyMedium | Plex Sans | 15 / 22 | 400 | 0 |
| code | Plex Mono | 15 / 24 | 400 | 0 |
| label | Plex Mono | 11 / 16 | 500 | +0.14em, UPPERCASE |

Mínimos: 11sp para rótulo mono, 15sp para qualquer texto lido.
Todo número que muda em tempo real usa **tabular-nums**.

### Espaço, forma
Escala 4dp: `4 · 8 · 12 · 16 · 24 · 32 · 48`.
Margem lateral de tela **20dp**. Gap entre cards de lista **8dp**.

Raio: **2dp** blocos de código, chips e células de placar · **4dp** padrão (cards, botões,
inputs) · **8dp** apenas cantos superiores do bottom sheet.

### Alvo de toque
Piso **nativo por plataforma**: 48dp Android, 44pt iOS.
Exceção de produto: **56dp/pt em qualquer alvo durante partida** — um mis-tap custa uma vida.
Linhas de código em SPOT_THE_BUG têm 34dp visíveis + folga invisível até o piso.

---

## Motion

| Evento | Duração | Curva | Detalhe |
|---|---|---|---|
| Feedback de resposta | 120ms | linear | muda borda + tint da opção |
| Shake de erro | 240ms | ease-out | ±5dp horizontal, 4 oscilações |
| Bottom sheet enter | 280ms | emphasized decelerate | de baixo |
| Troca de questão | 180ms | standard | slide + fade |
| Balão preenche | 160ms | spring (pop 1.0→1.18→1.0) | ao receber AC |

Nenhuma animação acima de 300ms durante partida.
`reduceMotion` / `UIAccessibility.isReduceMotionEnabled`: o shake vira flash de borda.

**Haptic**
| Plataforma | Acerto | Erro |
|---|---|---|
| Compose | `HapticFeedbackType.TextHandleMove` | `HapticFeedbackType.LongPress` |
| SwiftUI | `.sensoryFeedback(.success, …)` | `.sensoryFeedback(.error, …)` |

---

## Componentes

### Button
Altura **52dp**, largura total, raio 4dp. Um primário por tela — sempre o que avança a partida.
| Variante | Fundo | Texto | Borda |
|---|---|---|---|
| primary | `accent` | `onAccent`, 15sp/600 | — |
| primary:hover | `#FF9364` | | |
| primary:pressed | translateY(1dp) | | |
| secondary | transparente | `textPrimary`, 15sp/500 | 1dp `lineStrong` |
| ghost | transparente | `textSecondary`, 15sp/500 | — |
| disabled | `buttonDisabled` | `textDim` | — |

### OptionRow
Altura mínima **56dp**, padding lateral 14dp, raio 4dp, gap 8dp entre linhas.
Prefixo: letra da alternativa em Plex Mono 12sp, largura fixa 18dp.
| Estado | Borda | Fundo | Sufixo |
|---|---|---|---|
| default | `lineStrong` | `surfaceRaised` | — |
| selected | `accent` | accentTint | — |
| correct | `correct` | `tintOk` | `AC` mono 11sp |
| wrong | `wrong` | `tintErr` | `WA` mono 11sp + shake |

Estado **nunca** depende só de cor: a sigla do juiz sempre acompanha.

### Balloon (primitiva)
**A cabeça do balão de UI é exatamente o símbolo da marca** — mesmo corpo, mesmo brilho,
mesmo nó. Só a cauda difere: no logo ela é a curva logarítmica; na UI é a cordinha ondulada.
Desenhe a partir do mesmo path, não reconstrua com formas primitivas.

ViewBox de referência `0 0 96 150` (sem cauda: `0 0 96 95`). O corpo ocupa `x 16…80`,
ou seja **66,7% da largura da viewBox** — dimensione por aí.

| Elemento | Path | Traço |
|---|---|---|
| Corpo | `M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z` | preenchido; contorno 4 quando em aberto |
| Brilho | `M31 21 C27 27 25 33 25 40` | 5.5, cap redondo, branco @72% |
| Nó | `M41 76 L55 76 L48 91 Z` | preenchido |
| Cordinha (UI) | `M48 90 C38 98 58 106 48 114 C38 122 58 130 48 138` | 4–5, cap redondo |
| Cauda log (**só** logo) | `M48 90 C49 104 56 111 68 113 C78 115 84 115 90 116` | 5, cap redondo |

Tamanhos de corpo em uso: 29 (paleta), 17 (BalloonScore), 13 (match header e relatório),
14 (header de questão), 10 (coluna do placar — **sem cordinha**).

**Estados:** preenchido = problema aceito, na cor da letra; contorno 4 em `lineStrong` com
opacidade 0.55 = em aberto. O brilho só aparece em balão preenchido e sai abaixo de 12dp de
corpo. A letra fica **sob** o balão em Plex Mono. Em fileiras densas (A—M no header e no placar) ela cai para 8—9sp, abaixo do mínimo de 11sp: ali é reforço visual, e o fallback acessível de verdade é o `contentDescription` (`"problema C, aceito"`), nunca o glifo.

Em Compose use `Path` + `PathParser` (ou um `ImageVector` gerado do SVG em `brand/`);
em SwiftUI, `Path` com os mesmos comandos, escalado por `GeometryReader`.

### LifeBar
3 corações 20sp. Cheio = `wrong`; vazio = `heartOff`. Contador mono `2 / 3` ao lado — fallback.

### VerdictChip
Altura 26dp, min-width 46dp, raio 2dp, Plex Mono 12sp/600, tracking +0.06em.
Borda na cor da linha, texto no `…Ink`, fundo no tint correspondente.
| Código | Rótulo | Tom | Nota exibida |
|---|---|---|---|
| `AC` | Accepted | correct | balão sobe |
| `WA` | Wrong Answer | wrong | +20 min de penalidade |
| `TLE` | Time Limit Exceeded | wrong | complexidade errada |
| `MLE` | Memory Limit | wrong | estrutura pesada demais |
| `RE` | Runtime Error | wrong | índice, overflow, divisão |
| `CE` | Compile Error | warn | sem penalidade |
| `PE` | Presentation Error | warn | formato da saída |
| `…` | Judging | textMuted | na fila do juiz |

A sigla é a do juiz, **sem tradução**. O texto em português fica ao lado, nunca no lugar.

### ContestClock
Plex Mono, **tabular-nums** obrigatório.
| Estado | Tamanho | Cor | Fundo |
|---|---|---|---|
| normal | 38sp/500 | `textPrimary` | `canvas`, borda `line` |
| congelado (última hora) | 38sp/500 | `warn` | `tintWarn`, borda `warn` |
| questão crítica (<15s) | 28sp/600 | `wrong` | — |

Rótulo acima em `label` (Plex Mono 10.5sp, tracking +0.16em).

### Tag
Plex Mono 12sp, padding 8×12, raio 2dp. Default: borda `lineStrong`, texto `textSecondary`.
Selecionada: borda + texto `accent`, fundo accentTint.

### DropZone
Altura 38–46dp, raio 2dp, Plex Mono 13sp.
Vazia: tracejado `lineDim`, texto `textDim`. Hover de arraste: tracejado `accent`, fundo accentTint.
Preenchida: borda sólida `lineStrong`, fundo `surfaceRaised`, texto `textPrimary`.

### CodeBlock
Fundo `canvas`, borda `line`, raio 2dp. Plex Mono 13–15sp, entrelinha 24dp.
Número de linha em `textMuted`, não selecionável, coluna fixa.
Linha realçada: fundo tint + barra lateral 2dp na cor da **linha** (não da tinta).
**Sem scroll horizontal** — linhas quebram preservando a indentação.

### MatchHeader
Altura 84dp, fixo no topo, fundo `canvas` (separa-se do conteúdo em `surface`).
Linha 1: rótulo da sessão (label) · ContestClock · LifeBar.
Linha 2: fileira de balões A—M, 11×14dp, gap 7dp, letra 8sp abaixo.

---

## Telas

### 1 · Scoreboard (telão)
O artefato mais reconhecível do ICPC e o único **não** phone-first — nasce projetado numa parede.
No app aparece entre partidas, com scroll horizontal e a linha do usuário grudada na base.

**Grid:** `52dp | minmax(190,1fr) | 56dp | 68dp | 13 × minmax(44,1fr)`, largura mínima 900dp.
- **Header do contest** (altura auto, 16×20 padding, borda inferior `line`): nome do contest em
  Plex Mono 13sp/600 tracking +0.12em · badge `CONGELADO` (borda+texto `warn`, fundo `tintWarn`,
  raio 2dp) · relógio 24sp tabular alinhado à direita.
- **Header de coluna** (40dp, fundo `surfaceRaised`): `#`, `EQUIPE`, `SLV`, `PEN` em label 10sp,
  depois A—M com balão 10×11dp acima da letra 11sp/600.
- **Linha** (52dp): rank mono 14sp/600 · nome 14sp/600 + universidade em label 10sp ·
  solved 15sp/600 · penalty 13sp `textSecondary` · 13 células.
- **Linha do usuário**: fundo accent @13% sobre canvas, borda sup/inf `accent`, nome e rank em accent.
- **Célula** (38dp de altura, margem lateral 2dp, raio 2dp, borda 1dp): topo = símbolo 12sp/600,
  base = minuto 9sp @75%.

| Estado da célula | Borda | Fundo | Topo | Base |
|---|---|---|---|---|
| aceito | `correct` | `tintOk` | `+` ou `+N` (tentativas erradas) | minuto do AC |
| tentado sem AC | `wrong` | `tintErr` | `−N` | — |
| pós-congelamento | `info` | `tintInfo` | `?` | `frz` |
| não tentado | `line` | transparente | — | — |

Legenda das quatro cores no rodapé, sempre visível. O congelamento não é enfeite: é a última
hora em que ninguém sabe o resultado — a UI mostra **dúvida**, não esconde.

### 2 · Skill tree — DAG de pré-requisitos
O currículo **é um grafo dirigido acíclico**, e a tela mostra isso. Aresta não é conector
decorativo: carrega direção, estado e grau de entrada. Um nó com duas arestas chegando precisa
dos dois pré-requisitos, e o jogador entende por que está fechado sem ler texto.

**Vértices são balões da marca.** Mesma primitiva de `Balloon`, tamanho por importância:
56 para conquistado, 70 para o ativo, 54 para bloqueado. Preenchido na cor da família com brilho branco = conquistado (o brilho passa a ser obrigatório); miolo tint accent (accentTint) e ícone em `accentInk` = ativo; silhueta inteira preenchida em `lineStrong` (`line2`), sem contorno nem brilho, ícone em `textSecondary` = bloqueado.

**Arestas são as cordinhas.** Curvas de Bézier que saem de dentro da etiqueta do nó de origem
e chegam ao topo do balão de destino.

| Estado da aresta | Traço | Cor |
|---|---|---|
| percorrida | 2.5, cap redondo, opacidade 0.8 | cor da família de origem |
| ativa | 3, cap redondo | `accent` |
| fechada | 2.5, `stroke-dasharray 4 6` | `lineDim` |

**NodeTag** — o rótulo nunca flutua solto sobre o mapa. Chip ancorado logo abaixo do balão,
centrado no eixo do nó (`translateX(-50%)`): fundo `canvas`, borda 1dp `line`, raio 2dp,
padding 4×10, texto 12.5sp/600. A aresta de saída nasce ~3dp **dentro** do chip, então a
cordinha lê como se atravessasse a etiqueta.
| Estado | Borda | Fundo | Texto |
|---|---|---|---|
| conquistado | `line` | `canvas` | `textPrimary` |
| ativo | `accent` | accentTint | `textPrimary` + 2ª linha `INFLANDO · 2/5` em **`accentInk`** mono 9.5sp |
| bloqueado | `line` | `canvas` | `textSecondary` |

**Badge de grau de entrada** — círculo 18dp no canto superior direito do balão, fundo `canvas`,
borda `lineDim`, número em mono 9sp. Só aparece quando o grau de entrada é ≥ 2.

**Header:** contagem de balões (`7 balões no ar`) 20sp/600 + XP total em accent mono.
**Rodapé:** legenda `CORDINHA TRACEJADA = ARESTA FECHADA` em label.

**Sheet do nó** (toque em qualquer vértice): balão + nome + estado; depois **Vem de** e
**Destrava** lado a lado listando a vizinhança do grafo com ícone e nome; métricas
(lições, melhor tempo, XP); CTA. Para nó bloqueado, o sheet lista as arestas de entrada uma a
uma com seu estado, mais o limiar de XP como terceiro requisito.

> **Layout:** as posições de nó são **autoradas**, não calculadas em runtime — um layout de grafo
> automático produz resultados instáveis a cada build. Guarde `x`/`y` por nó no mesmo JSON do
> currículo. Acima de ~12 nós visíveis, decida entre scroll vertical com arestas curtas (o que
> está desenhado) ou canvas com pan/zoom.

### 3 · Trap sheet (bottom sheet em partida)
Conteúdo atrás a 45% de opacidade, sem blur. Sheet: fundo `surfaceRaised`, borda superior
`lineStrong`, raio 8dp só nos cantos superiores, `shadowSheet`. Handle 36×3dp `lineStrong`.
Conteúdo: label `TRAP CLÁSSICA` em `wrong` + nome da trap em `textMuted` · título 17sp/600 ·
explicação 14sp `textSecondary` · dois botões 48dp (secundário "Ler explicação" com nota
`pausa o timer`; primário "Pular").

### 4 · Post-match report
Header: `CONTEST ENCERRADO` em label · número 40sp/600 + `/ 13 aceitos · 512 pen` em mono 15sp ·
fileira de balões A—M.
Corpo: `REVISÃO · N ERROS` em label, depois um card por erro (fundo `surface`, borda `line`,
raio 4dp, padding 14): nome do problema 15sp/600 + VerdictChip à direita · `sua resposta: X`
em mono 12sp `textMuted` · explicação 13sp `textSecondary`.
CTA "Entendi".

### 5 · Hub de perfil
Sem tab bar. Entrada: avatar circular **40dp** no canto direito da NavigationBar da Skill Tree,
com badge `warn` de 11dp quando `pending_sync_count > 0`. O perfil sobe como **sheet** sobre a
árvore (raio 8dp nos cantos superiores, `surfaceRaised`, `shadowSheet`), com a árvore visível
atrás a 18% — o jogador não perde o lugar.

**Header:** avatar 52dp (inicial em `accentInk` sobre accent @18%, borda accent) + e-mail em
Plex Mono 13sp + linha de status com ponto colorido: `correct` `TUDO SINCRONIZADO` ou
`warn` `N EVENTOS NA FILA`. Visitante troca o e-mail por um chip `MODO VISITANTE`.

**Nível é o herói:** número 44sp/600 + label `NÍVEL` em mono, ao lado de uma barra de progresso
até o próximo nível com `620 XP / 800` e `180 XP para o nível 5`.
Cálculo no core: `nivel = floor(xp / 200) + 1`. Nunca no cliente.

**Stats:** grid de três células (`XP TOTAL`, `BUGS`, `DRY RUNS`), e a soma em mono abaixo:
`47 desafios concluídos · 7 balões no ar`.

#### Divergências da spec técnica — deliberadas
Estes três pontos são restrição de implementação que vira má experiência se copiada literal.
A arquitetura (core em Rust, Keychain, `offline_events.json`) fica intacta; o que muda é o fluxo.

| Spec | O que ship |
|---|---|
| Alerta de confirmação em **todo** logout | **Sem alerta quando `pending_sync_count == 0`.** Sair está sincronizado é reversível: entrar de novo devolve tudo. Alerta ali é fricção sem prêmio e treina o usuário a confirmar sem ler — o que destrói o valor do alerta que *importa*. Mostra tela de saída com **desfazer**. |
| `"Se sair agora, você perderá esse XP permanentemente"` | **Nunca ofereça a escolha entre sair e perder progresso.** O sheet crítico traz `Sincronizar e sair` como primário, `Continuar conectado` como secundário, e `Sair e descartar 180 XP` em terceiro — texto, não botão. A perda deixa de ser o caminho padrão. |
| `Excluir conta` ao lado de `Sair` | A App Store exige que seja **encontrável**, não proeminente. Vizinho do logout, num sheet que o jogador abre para ver XP, é convite a acidente irreversível. Move para **Gerenciar conta** (um toque a mais), junto com trocar e-mail, trocar senha e **baixar meus dados** — que a mesma diretriz de privacidade recomenda. |

**Sheet crítico** (`pending_sync_count > 0`): borda superior `warn`, label
`12 EVENTOS NA FILA · 180 XP`, título que **nomeia a perda** em vez de perguntar "tem certeza".
O botão destrutivo repete verbo e número, nunca um "OK" genérico.

**Gerenciar conta:** tela empilhada (não sheet). Lista de ações neutras; o bloco de exclusão
fica no rodapé, em `tintErr` com borda `wrong`, label `IRREVERSÍVEL`, e exige senha + digitar
a palavra `EXCLUIR`. Dispara o endpoint de purga no backend Go.

**Visitante** (`is_guest == true`): oculta Sair e Gerenciar conta. No lugar, um card em accent
que nomeia o risco ("Seu progresso vive só neste aparelho") e o ganho ("Criar conta herda os
620 XP, os 7 balões e os 47 desafios"), com CTA primário para `RegisterView` e um
`Já tenho conta` secundário.

### 6 · Standings (phone)
Abas Global / Sede (aba ativa: borda inferior 2dp `accent`, 14sp/600).
Linha 44dp: rank mono 13sp largura 26dp · nome 15sp/500 + universidade em label 10sp ·
`solved · penalty` mono 14sp tabular à direita. Divisor `rowLine`.
Linha do usuário fixa na base, mesmo tratamento accent do scoreboard.
Bottom nav 3 itens (TRILHAS / ARENA / PLACAR) em label; ativo em accent com borda superior 2dp.

---

## Templates de questão
Quatro `template_type` no mesmo casco: **header fixo → corpo rolável → CTA ancorado**.
Nenhum abre teclado nativo.

Header de todos: balão colorido 14×16dp + `PROBLEM <letra>` em label, à direita ContestClock ou LifeBar.

### FILL_IN_THE_BLANK
Enunciado 19sp/600 · CodeBlock com uma lacuna inline (min-width 74dp, altura 30dp, tracejado
`accent`, raio 2dp) · label `ARRASTE O BLOCO` · 3 chips mono 13sp arrastáveis.
CTA desabilitado até preencher.

### SPOT_THE_BUG
Enunciado 19sp/600 · CodeBlock com linhas tocáveis (34dp cada + folga invisível, divisor `rowLine`).
Linha selecionada: fundo accentTint, barra lateral 2dp `accent`, número em accent.
CTA nomeia a escolha: "Confirmar linha 3".

### COMPLEXITY_MATCH
Enunciado 19sp/600 · duas linhas rotuladas `TEMPO` / `ESPAÇO` (label 11sp, coluna fixa 60dp)
com DropZone 46dp · divisor 1dp · banco de 4 chips mono 13sp.

### TAG_THE_PATTERN
Cartão de enunciado no formato da folha impressa: borda `line`, fundo `surface`, raio 2dp;
topo com `TIME LIMIT 1S` / `MEM 256MB` em mono 10sp separados por divisor; nome do problema
15sp/600; texto 15sp `textSecondary`.
Pergunta 17sp/600 · label `SELECIONE ATÉ 2` · grade de tags multi-seleção.
CTA conta a seleção: "Confirmar 2 tags".

---

## State Management
Por partida:
- `problems: List<Problem>` — cada um com `letter: Char`, `templateType`, payload, resposta correta
- `currentIndex: Int`
- `verdicts: Map<Char, Verdict>` — dirige a cor do balão e a célula do placar
- `lives: Int` (0–3) · `penaltyMinutes: Int` · `attempts: Map<Char, Int>`
- `questionTimer: Duration` (regressivo, dispara estado crítico aos 15s)
- `contestClock: Duration` · `isFrozen: Boolean` (última hora)
- `selection` por template: índice, linha, mapa de dropzone, ou set de tags
- `trapSheet: Trap?` — não-nulo pausa `questionTimer`
- `syncPending: Boolean` — exibe chip `SYNC PENDENTE` no header

Transições: submeter → veredito em 120ms → se AC, balão preenche (160ms) e avança em 180ms;
se WA, shake 240ms, `lives--`, penalidade +20min, e abre trap sheet se houver trap mapeada.

## Estados obrigatórios (todo componente)
`enabled · pressed · disabled · focused · selected · correct · wrong · locked`
- **loading**: skeleton com linha `line`, **sem spinner**
- **offline**: chip mono `SYNC PENDENTE` em `warn` no header
- **empty**: nunca uma ilustração; uma frase em `textSecondary` e o CTA que resolve

## Acessibilidade
- Piso de toque nativo (48dp / 44pt); 56 em partida
- Veredito nunca só por cor — a sigla do juiz sempre acompanha
- Balão nunca só por cor — a letra fica sob o balão e no `contentDescription`
- Célula do placar lê "problema C, aceito, 2 tentativas, 88 minutos"
- Contraste mínimo 4.5:1 em texto lido; 3:1 nas cores de balão (formas, não texto)
- Font scale até 200% (Android) / Dynamic Type até AX3 (iOS) sem truncar enunciado
- `reduceMotion`: shake vira flash de borda

## Voz e copy
- Termos técnicos em inglês, sem tradução: `binary search`, `trap`, `contest`, `freeze`
- É sempre "problema C", nunca "questão 3"
- Interface em português; rótulos de sistema em inglês monoespaçado
- Erro explica a causa, nunca julga: "Guloso falha aqui", não "Você errou feio"
- Sem exclamação, sem emoji, sem parabenização genérica

## Paridade entre plataformas
Geometria, cor e copy são **idênticas**. Só isto segue convenção nativa:
| Elemento | Compose | SwiftUI |
|---|---|---|
| Trap sheet | `ModalBottomSheet` | `.sheet` + `.presentationDetents` |
| Drag & drop | `detectDragGestures` + DragTarget | `.draggable` / `.dropDestination` |
| Haptic | `HapticFeedbackType` | `.sensoryFeedback` |
| Shake | `Animatable` + keyframes | `.phaseAnimator` / `KeyframeAnimator` |
| Escala de texto | `sp` + fontScale 200% | Dynamic Type até AX3 |
| Piso de toque | 48dp | 44pt |
| Navegação raiz | `NavigationBar` 3 itens | `TabView` `.tabBar`, mesmos 3 |

## Iconografia

Set próprio em `brand/icons/` — 12 SVGs, viewBox `0 0 24 24`, `fill="none"`,
`stroke="currentColor"`, traço 2, caps e joins redondos. Desenhados a partir da geometria de
cada técnica: two pointers converge, binary search bissecciona, sliding window emoldura uma
faixa, sorting são barras crescentes.

**Não usar SF Symbols nem Material Icons para assunto algorítmico** — não existem nas duas
plataformas e não descrevem a técnica. Ícones de sistema (voltar, fechar, compartilhar) seguem
o nativo de cada plataforma normalmente.

Cada assunto pertence a uma família, e a família define a cor — quatro cores emprestadas da
paleta de balões:

| Família | Cor | Assuntos |
|---|---|---|
| fundamentos | balão **H** `#F4A261` / light `#B26320` | adhoc, arrays, strings |
| busca e ordenação | balão **C** `#3DB2FF` / light `#0B6FBF` | two-pointers, binary-search, sliding-window, sorting |
| estruturas | balão **E** `#C77DFF` / light `#8A3FD1` | trees, graphs |
| otimização | balão **M** `#00B894` / light `#007A61` | dp, greedy |
| boss | `accent` | challenge |

Cor de família **não** é cor de problema: são usos distintos da mesma paleta. O acento fica
reservado ao nó ativo e ao boss — nenhum assunto o usa.

> **A paleta de balões é certificada para 3:1 como forma, não 4.5:1 como texto.** Use-a em
> preenchimento, traço de ícone e aresta. Em legendas e listas, a família entra como um ponto
> colorido de 9dp antes do rótulo, e o rótulo em `textSecondary` — cor vira forma, o texto
> continua legível.

## Assets
Símbolo da marca em `brand/` (3 SVGs) e 12 ícones de assunto em `brand/icons/`. Fora isso, nenhuma imagem: tudo é tipografia, forma e cor.
**Fontes:** IBM Plex Sans (Regular/Medium/SemiBold) e IBM Plex Mono (Regular/Medium) —
SIL Open Font License, baixar em <https://github.com/IBM/plex> e empacotar no app
(`res/font/` no Android, target membership + `UIAppFonts` no iOS). Não usar via CDN.

## Files
| Arquivo | O que é |
|---|---|
| `brand/logn-symbol-*.svg` | Símbolo em accent, currentColor e vazado. **Assets de produção.** |
| `brand/icons/*.svg` | 12 ícones de assunto, currentColor. **Assets de produção.** |
| `tokens/LognColor.kt` | Cores dark/light + paleta de balões. **Colar no codebase.** |
| `tokens/LognTheme.kt` | Typography, Shapes, Space, `LognTheme` composable. **Colar no codebase.** |
| `tokens/LognDesignSystem.swift` | Equivalente SwiftUI: cores, balões, fontes, raios, espaço. **Colar no codebase.** |
| `LogN Design System v2.dc.html` | **Referência visual — fonte única da verdade.** Marca, tokens, componentes, iconografia, arestas, 4 templates, telas-chave, scoreboard. Abre no navegador; tem toggle dark/light e seletor de acento. |
| `support.js` | Runtime necessário para abrir os `.dc.html`. Não é código de produção. |

Abra o v2 no navegador e mantenha ao lado durante a implementação — é a fonte da verdade
para qualquer medida não listada aqui.
