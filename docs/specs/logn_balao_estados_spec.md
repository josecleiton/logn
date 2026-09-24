# LogN - Spec: estados do balão na árvore (bloqueado 1b, ativo 3b)

Aplica no iOS dois estados do nó que o design fechou: o **bloqueado 1b**, um balão cinza
inteiro, e o **ativo 3b**, o contorno accent com miolo tint. A mudança é só no shell. O Core
não muda e não entra texto novo.

Fonte: o design `LogN Balão Bloqueado.dc.html` (bloco `text/x-dc`) e o handoff do design
system (`README.md`, `tokens/LognDesignSystem.swift`, `LogN Design System v2.dc.html`),
exportados do Claude Design.

## 0. O que já existe

1. **O handoff já registra 1b e 3b.** A única diferença entre `docs/design_system/README.md` e o README do handoff é o parágrafo dos vértices (repo, linhas 385–386). No handoff ele descreve o ativo com "miolo tint accent (accent @14% sobre canvas) e ícone em `accentInk`", o bloqueado como "silhueta inteira preenchida em `line2`, sem contorno nem brilho, ícone em `textSecondary`", e diz que o brilho passa a ser obrigatório no conquistado. O `.dc.html` v2 do handoff difere do repo só nas linhas 740–753: os balões ativo e bloqueado do exemplo. O arquivo de tokens é igual nos dois lados.
2. **`line2` não é token do README.** No `.dc.html` v2, `line2` vale `#343A41` no dark e `#C6C8C2` no light, os mesmos valores de `lineStrong`. No app o token é `LognDark.lineStrong`. O arquivo do balão traz `line2` light `#CDD0CA` (e também `lineDim` e `text2` light levemente diferentes). Fica valendo o DS.
3. **O tint do ativo já existe no app, só no dark.** `LognDark.accentTint = #241610` (`LognDesignSystem.swift:49-51`) é o mesmo literal que o design usa no 3b. Isso dá cerca de 10% de accent sobre o canvas, e o README fala em 14% (cerca de `#2D1B15`). Vale o `#241610` (decisão 1).
4. **O app é só dark.** O comentário em `ContentView.swift:27-29` diz "`LognLight` existe nos tokens mas nenhum mock desenha o app em claro" e o código força `.preferredColorScheme(.dark)`. O painel light do design não tem onde aparecer hoje.
5. **O Core não muda.** `NodeStatus` é `Locked | Active | Completed` (`shared_core/src/domain.rs:271-276`). O rótulo "INFLANDO · n/m" já sai de `node.problemsSolved` com `Str.Solved.inflating` (`SkillTreeView.swift:357-359`), então o dado já está no ViewModel. Não entra chave de i18n nova.
6. **O badge "2" já existe.** Ele mostra o grau de entrada: o número de pré-requisitos, e só aparece quando é 2 ou mais (`SkillTreeView.swift:298-310`), como pede o DS. No design ele está fixo no nó bloqueado só como exemplo. Não muda nada nele.
7. **Tamanhos, arestas e tags já batem com o DS.** Os tamanhos são 56/70/54 (`SkillTreeView.swift:178-184`). A aresta fechada é tracejada `lineDim` 2.5 `[4,6]` (236). A ativa é `accent` com traço 3 (243). As tags (349–395) já seguem a tabela NodeTag. A mudança é só no balão.

## 1. Estado atual no app vs design

| Estado | App hoje | Design (fechado) |
|---|---|---|
| Bloqueado | `.deflated`: corpo menor `bodyDeflated`, traço 4 tracejado `5 4` em `lineDim`, nó `knotDeflated` `lineDim` (`BalloonShape.swift:36-37, 74-80`), ícone `textMuted` (`SkillTreeView.swift:325`), centro do ícone em 0.50 (336) | **1b**: corpo inteiro `body`, preenchimento `lineStrong` (`line2`), sem traço nem brilho, nó `lineStrong`, ícone `textSecondary` |
| Ativo | `.active`: traço 5 `accent`, miolo vazio, brilho `accent` @0.40, nó `accent` (69–72), ícone `LognDark.accent` (324) | **3b**: mesmo traço 5 `accent`, preenchimento `accentTint`, brilho `rgba(255,122,69,0.45)`, nó `accent`, ícone `accentInk` |
| Conquistado | `.filled(topic.color)` com brilho branco @0.72 (60–63) | Sem mudança, mas o brilho passa a ser obrigatório |
| Tag ativa / bloqueada | Borda `accent`, fundo `accentTint`, segunda linha em `accentInk` / texto `textSecondary`, borda `line` | Igual |
| Arestas | Descritas no item 7 da seção 0 | Igual (DS) |

**Brilho do conquistado.** Hoje ele sempre aparece nas duas telas. `drawShine` só não desenha quando `showHighlight == false` ou quando o corpo tem menos de 12dp (`BalloonShape.swift:102-103`). A árvore usa largura 56 (corpo de cerca de 37dp) e não passa `showHighlight`. O cabeçalho do sheet também usa 56 (`NodeSheetView.swift:82`). O único `showHighlight: false` está nos balões de problema do sheet (206), que não são nós. Não há nada a consertar. Só fica a regra escrita no código, para ninguém desligar o brilho num nó.

## 2. Mudanças de código

### 2.1 `ios/LogNiOS/LogNiOS/Components/BalloonShape.swift`
- **Linhas 16–19:** atualizar a doc de `.active` para "contorno accent traço 5 com miolo `accentTint`". Renomear `.deflated` para `.locked` (decisão 2), com a doc "balão cinza: corpo inteiro em `lineStrong`, sem traço nem brilho".
- **Linhas 35–37 e 55–57:** remover `bodyDeflated`, `knotDeflated` e o `isDeflated`. Todo estado passa a usar `D.body` e `D.knot`.
- **Linhas 69–72 (`.active`):** primeiro `context.fill(bodyPath, with: .color(LognDark.accentTint))`, depois o traço 5 `accent` que já existe. Brilho `LognDark.accent.opacity(0.45)` e nó `accent`. Preencher antes de traçar mantém o traço inteiro por cima.
- **Linhas 74–80 (`.locked`):** `context.fill(bodyPath, .color(LognDark.lineStrong))` e `context.fill(knotPath, .color(LognDark.lineStrong))`, sem traço e sem `drawShine`.
- **Linha 116 (`stringColor`):** `.locked` → `LognDark.lineStrong`.
- **Linhas 12–13:** acrescentar à doc de `.filled` que em nó o brilho é obrigatório, porque é o que separa conquistado de bloqueado, e que `showHighlight: false` só serve para balão de problema.

### 2.2 `ios/LogNiOS/LogNiOS/Views/SkillTreeView.swift`
- **Linha 317:** `.locked` → `.locked`, seguindo o novo nome.
- **Linhas 321–327 (`iconColor`):** ativo → `LognDark.accentInk` (regra de tinta: glifo usa `…Ink`; no dark o hex é o mesmo), bloqueado → `LognDark.textSecondary`.
- **Linhas 333–337 (`iconCenterRatio`):** 0.42 para todos, e apagar o comentário da silhueta murcha. No `.dc.html` v2 o ícone do bloqueado subiu 5px num balão de 54 (top 21 → 16). Isso dá cerca de 0.09 de razão, e 0.50 − 0.09 fica perto do 0.42 dos outros estados.
- **Linhas 329–331:** manter 0.34 para o bloqueado (18/54 no DS).
- **Linha 343:** fica em 2.0 (decisão 4).

### 2.3 `ios/LogNiOS/LogNiOS/Views/NodeSheetView.swift`
- **Linha 105:** `.deflated` → `.locked`.
- **Linhas 109–115:** ativo → `accentInk`, bloqueado → `textSecondary`, igual à árvore. A posição do ícone já é `56 * 0.42` para todos (84). Com o corpo inteiro, o ícone do bloqueado deixa de ficar fora do centro.
- **Linhas 127–133 (`stateColor`):** sem mudança.

### 2.4 Tokens e documentação
- `ios/.../DesignSystem/LognDesignSystem.swift`: acrescentar `LognLight.accentTint = #FDE9DF` (valor do arquivo do balão), sem uso por enquanto, só para os dois temas terem os mesmos tokens.
- `docs/design_system/README.md:383-386` e `docs/design_system/LogN Design System v2.dc.html`: copiar os dois arquivos do handoff, como no commit `8ae2759` ("sync the design system from the live source").
- `docs/design_system/tokens/LognDesignSystem.swift`: acrescentar `accentTint` (dark `#241610`, light `#FDE9DF`) para bater com o app. Pedir ao design que leve o token para a tabela do handoff e troque o "14%" do README pelo valor real.
- `docs/specs/logn_skill_tree_spec.md:14-18`: descreve um MVP antigo (pulse, SF Symbols, bloqueado em `surfaceRaised` com borda `lineDim`). Trocar só os bullets de UI de §2 por uma referência à seção "Skill tree" do README do DS (decisão 5).

## 3. Onde o balão aparece

| Superfície | Arquivo | Muda? |
|---|---|---|
| Árvore | `SkillTreeView.swift:284` | Sim |
| Cabeçalho do sheet do nó | `NodeSheetView.swift:82` | Sim |
| Problemas do sheet | `NodeSheetView.swift:201` (`.filled` / `.outline`) | Não |
| Partida: cabeçalho e fileira | `LognComponents.swift:281, 336` | Não |
| Veredito, relatório | `MatchView.swift:517, 714` | Não |
| Placar, sair da partida | `ScoreboardView.swift:137`, `LeaveMatchSheet.swift:48` | Não |
| Fileira das telas de entrada | `BalloonShape.swift:198-208` (`BalloonMarquee`) | Não |
| Splash, ícone | Imagens em `Assets.xcassets` | Não |

`.active` e `.deflated` só aparecem nas duas primeiras linhas. O perfil não desenha nó. Não há cliente Android no repositório.

## 4. Acessibilidade

- **Estado não depende só de cor.** O ativo tem traço, tamanho 70 e tag de duas linhas. O bloqueado não tem traço e o texto da tag fica em `textSecondary`. O conquistado se separa do bloqueado pelo brilho e pela cor do ícone.
- **Rótulos.** Já saem do catálogo: `Str.Tree.node_completed`, `node_active`, `node_locked` e `node_locked_prereq` (`SkillTreeView.swift:404-420`). Não entra texto novo.
- **Contraste.** Ícone `textSecondary` sobre `lineStrong`: cerca de 4.3:1 (passa o 3:1 de forma). Corpo `lineStrong` contra o canvas: cerca de 1.7:1, abaixo do 2.5:1 do tracejado `lineDim` de hoje. Foi escolha do design ("sem a cor do problema"). A identificação fica com o ícone (cerca de 7:1 contra o canvas), a tag e o rótulo. Vai para os riscos, não para bloqueio.
- **Fora do escopo, só registro.** O nó bloqueado abre o sheet ao toque (linhas 41–43), mas não tem o trait `.isButton` (277).

## 5. Testes

O iOS não tem target de teste nem `#Preview`. A verificação é o roteiro do simulador, que o `AGENTS.md` exige antes de dar UI como pronta. `just build-ios-ffi` não é necessário, porque o Rust não muda.

**Passos para rodar e depois gravar no roteiro:**

1. Build e instalação como na seção 0 do roteiro (`xcodebuild … build`, `simctl install`).
2. Estado limpo: `xcrun simctl launch booted sh.logn.LogNiOS -LogNStartAsGuest 1`, depois `xcrun simctl io booted screenshot "$SP/08-arvore-estados.png"`. Conferir o nó 1 ativo com miolo cor de tijolo escuro e traço laranja, os outros como balões cinza inteiros sem traço, ícone cinza-claro centrado, badge de grau de entrada legível na borda do corpo cinza e aresta tracejada saindo do ativo.
3. Sheet do bloqueado: tocar a tag de um nó bloqueado (confirmar a coordenada no screenshot) e tirar o screenshot `09-no-bloqueado.png`. O balão do cabeçalho deve estar cinza e inteiro, com o ícone no centro.
4. Conquistado: jogar 4.4 a 4.8 e voltar à árvore. Se o XP já passou o portão de um filho do nó 1 (o ROADMAP explica como os portões dos primeiros nós se relacionam), o nó 1 aparece na cor da família com brilho. Screenshot `10-arvore-conquistado.png`, conferindo que conquistado e bloqueado não se confundem.
5. VoiceOver ou Accessibility Inspector: os rótulos dos três estados dizem "conquistado", "em curso" e "bloqueado".

**Mudança no roteiro:** acrescentar ao **Conferir** de 4.2 (hoje nas linhas 249–253) a frase "ativo com miolo tint e traço accent; bloqueado como balão cinza inteiro, sem traço nem brilho; conquistado sempre com brilho". Em 4.3, conferir o balão do cabeçalho.

## 6. Ordem

1. `BalloonShape.swift` (2.1), junto com os dois pontos de uso (2.2 e 2.3), porque o renome quebra a compilação.
2. Build e passos 2 a 5 do roteiro.
3. Sincronizar os docs do DS (2.4) e atualizar o roteiro.
4. Um commit `feat(ios): …` para o código e outro `docs: …` para o DS e o roteiro, como nos commits recentes.

## 7. Riscos

- **Bloqueado some no canvas** (1.7:1). Se no screenshot o corpo não se destacar, levar ao design antes de mexer em cor. O design proíbe inventar cor fora dos tokens.
- **Badge sobre o corpo cheio.** Antes ele ficava sobre o vazio da silhueta menor. Agora encosta cerca de 2dp no corpo cinza. Ele tem fundo `canvas` e borda, então deve continuar legível. Conferir no passo 2.
- **Vários ativos ao mesmo tempo.** A `view()` do Core destrava por XP global (`app.rs:2361-2384`), e irmãos podem estar ativos juntos. Com miolo preenchido o ativo pesa mais, então vale conferir uma linha com dois ativos.

## 8. Decisões

1. **Tint do ativo: `#241610`.** É o token que o app já tem e o literal do 3b no design. O "14%" do README é que precisa ser corrigido, no handoff.
2. **`.deflated` vira `.locked`.** São dois pontos de uso, e o nome antigo descreve um desenho que deixou de existir.
3. **Aresta ativa com traço 3,** como na tabela do DS. O 2.5 do mock é ilustração.
4. **Traço do ícone do bloqueado fica em 2.0** nesta entrega. O DS desenha 2.2; decide-se olhando o screenshot, fora da decisão 1b.
5. **`logn_skill_tree_spec.md`:** troco só os bullets de UI de §2 por uma referência ao README do DS. Reescrever o spec fica fora do escopo.
6. **Fora do escopo:** `LognComponents.swift:357-359` monta `"problema \(l), aceito"` como literal, o que viola a regra 6 do `AGENTS.md`. Vai num item separado.

Não precisa de ADR: não entra biblioteca nem muda arquitetura, é aplicação do DS.
