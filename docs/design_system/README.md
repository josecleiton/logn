# Handoff: LogN — Design System v2

**English** · [Português](README.pt-BR.md)

## Overview
LogN is a micro-learning app for competitive programming: ~3-minute sessions that
reproduce the grammar of a real ICPC contest (problems by letter, balloons by color, lives,
a scoreboard that freezes in the last hour). This package describes the complete visual system: color,
typography, space, motion, components, four question templates and the key screens.

Two platforms, full parity of geometry/color/copy: **Jetpack Compose** and **SwiftUI**.

> **Naming:** *Impecable* is the design tool the system was produced in, and it
> appears as a signature in the document header. The product, and the code namespace,
> is **LogN**. If there is an earlier Impecable brand system that LogN should inherit
> (logo, corporate palette, corporate tone of voice), it was **not** provided and is not
> reflected here; everything below was defined from scratch for LogN.

## About the Design Files
The handoff `.dc.html` files live in the design project, outside this repository: they
depend on an editor runtime that does not belong to this project and is not licensed to be
redistributed here. They are **design references built in HTML**:
prototypes that show the intended look and behavior. **They are not production code
to copy.** The task is to **recreate these designs in the target codebase's environment** (Compose /
SwiftUI), using the patterns and libraries already established there. If no environment exists yet,
choose the appropriate architecture and implement it.

The files in `tokens/` are the exception: those **are** meant to go into the codebase nearly as they are.

## Fidelity
**High-fidelity.** Colors, typography, spacing, states and animation durations are final.
Recreate them pixel for pixel using each platform's native components.

---

## Brand

The name lives inside the notation: `O(log n)`. The mark turns that `O` into a balloon, and the
balloon's string into the logarithmic curve: it rises fast, then levels off. Two readings in one
shape: for competitors it is the ICPC balloon, for anyone else it is the curve of someone who improved.

### Symbol
Vector in `brand/`, viewBox `0 0 96 122`. Four elements, in this z-order:
1. **Body** — `M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z`
2. **Highlight** — `M31 21 C27 27 25 33 25 40`, stroke 5.5, round cap, white @75%
3. **Knot** — `M41 76 L55 76 L48 91 Z`
4. **Tail (log curve)** — `M48 90 C49 104 56 111 68 113 C78 115 84 115 90 116`, stroke 5, round cap

| Version | File | When |
|---|---|---|
| Accent | `logn-symbol-accent.svg` | default, on canvas or surface |
| Monochrome | `logn-symbol-currentcolor.svg` | inherits `currentColor`: documents, print, partners |
| Knockout | `logn-symbol-knockout.svg` | on accent, in `onAccent` |

### Lockups
There is no lockup SVG: build it by composing the symbol with live text in IBM Plex Sans SemiBold.
- **Horizontal (everyday use: header, store, email):** symbol + `LogN` at 44sp/600, tracking −0.035em,
  14dp gap, aligned on the optical center. Symbol height ≈ 1.27× the cap height.
- **Long signature (splash, corporate, footer):** `(` + symbol + `log n` + `)` in IBM Plex Mono,
  parentheses in `textSecondary`, `log n` in `textPrimary`, symbol in the exact place of the `O`.
- **Symbol alone:** only where the brand is already known: app icon, favicon, avatar.

### Tagline
**Reduza a complexidade das suas soluções.** ("Reduce the complexity of your solutions.")

Uppercase, IBM Plex Mono 10sp, `letter-spacing 0.1em`, `textSecondary`, always below the
horizontal lockup with 12dp of clear space. It appears on the welcome screen and in corporate pieces,
never in the app header, where only the lockup fits.

It works on two readings without winking at the reader: people in the field read time complexity,
the quantity O(log n) measures; everyone else reads a promise to simplify. **Solution** is the
right object because it covers the whole curriculum: algorithms, architecture and infra all have complexity,
while "code" would leave the last two out.

**English variant: "Reduce the complexity."**
It is not a translation of the Portuguese line; it is the version that works in that language. In English,
*complexity* with no qualifier in an engineering context already reads as time complexity, so
the object stays implicit and the line can be short. In Portuguese that does not happen: "reduza a
complexidade" asks for an object, hence "das suas soluções". Use each one in its own language, never
translate one into the other.

Rejected options and why, so nobody brings them back later:
- *"Mastering technology bit-by-bit"*: going through one item at a time is O(n), exactly the
  curve the name promises to avoid.
- *"Reduzindo a complexidade da tecnologia"* ("Reducing the complexity of technology"): it is the line every IT consultancy uses, and the
  double meaning dies in the cliché.
- *"Reduza a complexidade"* with no object: in Portuguese, too evocative for the screen where
  someone decides whether to download the app (English does not have this problem; see above).
- *"Crack the complexity of everything you build"*: more energy and closer to ICPC, but
  *crack* is not the verb that goes with *complexity* in code review, and 44 characters overflow the
  width on the iPhone SE.

### Scale and clear space
Minimum **16px**. Below 24px, remove the specular highlight (body, knot and tail only).
Clear space in any use: the width of the balloon body on every side.
App icon: symbol at ~54% of the tile width, optically centered (the visual center sits
above the geometric one because of the tail; shift the symbol ~4% up).

### Don'ts
- Don't paint it in a balloon color: the thirteen are problem addresses, not the brand
- Don't rotate it: the balloon rises vertically and the curve only reads on the right axis
- Don't distort it: the body ratio is 1 : 1.14
- Don't typeset the signature's `O`: in the signature the `O` is always the symbol

---

## Design Tokens

### Surface and line
| Token | Dark | Light | Use |
|---|---|---|---|
| canvas | `#0B0C0D` | `#F6F6F4` | screen background |
| surface | `#111316` | `#FFFFFF` | cards, lists |
| surfaceRaised | `#171A1E` | `#F0F1EE` | bottom sheet, table header |
| line | `#24282D` | `#E2E3DF` | default divider |
| lineStrong | `#343A41` | `#C6C8C2` | card and secondary button border |
| lineDim | `#4C535B` | `#A9AFB5` | dashed border of an empty dropzone |
| rowLine | `#16191C` | `#ECEDE9` | inner divider in dense lists |

### Content
| Token | Dark | Light | Contrast |
|---|---|---|---|
| textPrimary | `#EDEEEF` | `#14161A` | 15.8:1 / 15.4:1 |
| textSecondary | `#99A0A7` | `#555B62` | 7.1:1 / 7.0:1 |
| textMuted | `#7E858D` | `#656B72` | 5.0:1 / 4.8:1 |
| textDim | `#5F656C` | `#7C838A` | **shape and placeholder only**: 3.6:1 in light, never on text that is read |

CodeBlock line numbers use `textMuted`, not `lineDim`: they are content that gets read.

### Action
| Token | Dark | Light | Use |
|---|---|---|---|
| accent | `#FF7A45` | `#FF7A45` | **fill and border**: button, active balloon, selection, active edge |
| accentInk | `#FF7A45` | `#A83C0B` | **text and icon** in accent |
| onAccent | `#160B05` | `#160B05` | text on the accent fill |
| buttonDisabled | `#1B1D20` | `#E6E7E3` | disabled button |

The accent governs everything tappable and all progress, and it is the same color in both themes **as long as
it is a fill**. As text ink it does not reach 4.5:1 on white (it lands at 2.4:1), so
light uses `accentInk` `#A83C0B`.

> **Ink rule: applies to accent, correct, wrong, warn and info.**
> Background, border and shape strokes use the base token (`accent`, `correct`, …).
> Any glyph (label, verdict code, number, eyebrow, line icon) uses the `…Ink` token.
> In dark, base and ink are identical, so the distinction only shows in light: writing the base
> token on text goes unnoticed in dark and breaks contrast in light.
> Exception: the LifeBar heart and the filled balloon are **shape**, not text, so they use the base.

### Verdict (judge only)
Each verdict has three tokens: the **line** (border, side bar, filled icon), the **ink**
(verdict code, number, any glyph) and the background **tint**. In dark, line and ink match; in light
the ink gets darker, because the line color does not reach 4.5:1 on its own tint at small sizes.

| Line | Dark | Light | | Ink | Dark | Light | | Tint | Dark | Light |
|---|---|---|---|---|---|---|---|---|---|---|
| correct | `#3DD68C` | `#0E8F52` | | correctInk | `#3DD68C` | `#0A6B3C` | | tintOk | `#0F2018` | `#E6F5EC` |
| wrong | `#FF5C5C` | `#C93636` | | wrongInk | `#FF5C5C` | `#A82424` | | tintErr | `#231113` | `#FBEAEA` |
| warn | `#F5C451` | `#8A5B00` | | warnInk | `#F5C451` | `#6E4800` | | tintWarn | `#221C0C` | `#FAF1DC` |
| info | `#5AA9FF` | `#1660C4` | | infoInk | `#5AA9FF` | `#124F9E` | | tintInfo | `#0D1B2B` | `#E6EFFB` |

**Hard rule:** green and red appear *only* as the result of an answer, a scoreboard
cell, or an **irreversible destructive action** (delete account, discard progress), and in that case
only inside the confirmation block, never in the screen's resting state. Never in navigation, never in a neutral state. `warn` is the critical timer and the frozen scoreboard;
`info` is theory, hints and post-freeze submissions.

### Code syntax
| Token | Dark | Light |
|---|---|---|
| synKeyword | `#C792EA` | `#7A28C4` |
| synFunction | `#82AAFF` | `#0A4FA8` |
| number | uses `warn` | uses `warn` |
| base text | `textSecondary` | `textSecondary` |
| line number | `textMuted` | `textMuted` |

### Balloons A—M: identity, not state
The third color family, and the only non-semantic one. **Each contest letter carries the same color
everywhere**: question header, scoreboard, report. It indicates an *address*, not a state.

| Letter | Dark | Light | | Letter | Dark | Light |
|---|---|---|---|---|---|---|
| A | `#E4572E` | `#C43F19` | | H | `#F4A261` | `#B26320` |
| B | `#F5C451` | `#A67A00` | | I | `#9BC53D` | `#5F8410` |
| C | `#3DB2FF` | `#0B6FBF` | | J | `#D64550` | `#A3202B` |
| D | `#6BCB77` | `#2E8B45` | | K | `#7C8BFF` | `#4352C9` |
| E | `#C77DFF` | `#8A3FD1` | | L | `#D8DEE4` | `#5B646D` |
| F | `#FF6FB5` | `#C2367E` | | M | `#00B894` | `#007A61` |
| G | `#4ECDC4` | `#18867E` | | | | |

Forbidden: using a balloon color for a verdict, selection or navigation; reassigning a letter within
the same contest. Always access it through `Balloon.of('C')`, never through a hex literal.

### Shadow
Only use: the bottom sheet, and **only upward**.
| Token | Dark | Light |
|---|---|---|
| shadowSoft | `rgba(0,0,0,0.60)` — `0 -10px 40px` | `rgba(20,22,26,0.10)` |
| shadowSheet | `rgba(0,0,0,0.65)` — `0 -16px 48px` | `rgba(20,22,26,0.14)` |

Cards and buttons do **not** use shadow. Hierarchy comes from a 1px line plus one step of luminance.

### Typography
IBM Plex Sans (interface) + IBM Plex Mono (code, numbers, system labels).

| Style | Family | Size/Line height | Weight | Tracking |
|---|---|---|---|---|
| displayLarge | Plex Sans | 40 / 44 | 600 | −0.03em |
| headlineMedium | Plex Sans | 24 / 30 | 600 | −0.02em |
| titleMedium | Plex Sans | 19 / 25 | 600 | 0 |
| bodyLarge | Plex Sans | 17 / 26 | 400 | 0 |
| bodyMedium | Plex Sans | 15 / 22 | 400 | 0 |
| code | Plex Mono | 15 / 24 | 400 | 0 |
| label | Plex Mono | 11 / 16 | 500 | +0.14em, UPPERCASE |

Minimums: 11sp for mono labels, 15sp for any text that is read.
Every number that changes in real time uses **tabular-nums**.

### Space, shape
4dp scale: `4 · 8 · 12 · 16 · 24 · 32 · 48`.
Screen side margin **20dp**. Gap between list cards **8dp**.

Radius: **2dp** code blocks, chips and scoreboard cells · **4dp** default (cards, buttons,
inputs) · **8dp** only on the top corners of the bottom sheet.

### Touch target
**Native per-platform** floor: 48dp Android, 44pt iOS.
Product exception: **56dp/pt for any target during a match**, since a mis-tap costs a life.
Code lines in SPOT_THE_BUG are 34dp visible + invisible padding up to the floor.

---

## Motion

| Event | Duration | Curve | Detail |
|---|---|---|---|
| Answer feedback | 120ms | linear | changes the option's border + tint |
| Error shake | 240ms | ease-out | ±5dp horizontal, 4 oscillations |
| Bottom sheet enter | 280ms | emphasized decelerate | from the bottom |
| Question change | 180ms | standard | slide + fade |
| Balloon fills | 160ms | spring (pop 1.0→1.18→1.0) | on receiving AC |

No animation longer than 300ms during a match.
`reduceMotion` / `UIAccessibility.isReduceMotionEnabled`: the shake becomes a border flash.

**Haptic**
| Platform | Correct | Wrong |
|---|---|---|
| Compose | `HapticFeedbackType.TextHandleMove` | `HapticFeedbackType.LongPress` |
| SwiftUI | `.sensoryFeedback(.success, …)` | `.sensoryFeedback(.error, …)` |

---

## Components

### Button
Height **52dp**, full width, radius 4dp. One primary per screen, always the one that moves the match forward.
| Variant | Background | Text | Border |
|---|---|---|---|
| primary | `accent` | `onAccent`, 15sp/600 | — |
| primary:hover | `#FF9364` | | |
| primary:pressed | translateY(1dp) | | |
| secondary | transparent | `textPrimary`, 15sp/500 | 1dp `lineStrong` |
| ghost | transparent | `textSecondary`, 15sp/500 | — |
| disabled | `buttonDisabled` | `textDim` | — |

### OptionRow
Minimum height **56dp**, side padding 14dp, radius 4dp, 8dp gap between rows.
Prefix: the option letter in Plex Mono 12sp, fixed width 18dp.
| State | Border | Background | Suffix |
|---|---|---|---|
| default | `lineStrong` | `surfaceRaised` | — |
| selected | `accent` | accentTint | — |
| correct | `correct` | `tintOk` | `AC` mono 11sp |
| wrong | `wrong` | `tintErr` | `WA` mono 11sp + shake |

State **never** depends on color alone: the judge's verdict code always goes with it.

### Balloon (primitive)
**The head of the UI balloon is exactly the brand symbol**: same body, same highlight,
same knot. Only the tail differs: in the logo it is the logarithmic curve; in the UI it is the wavy string.
Draw it from the same path; do not rebuild it from primitive shapes.

Reference viewBox `0 0 96 150` (without tail: `0 0 96 95`). The body spans `x 16…80`,
that is **66.7% of the viewBox width**; size it from there.

| Element | Path | Stroke |
|---|---|---|
| Body | `M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z` | filled; outline 4 when open |
| Highlight | `M31 21 C27 27 25 33 25 40` | 5.5, round cap, white @72% |
| Knot | `M41 76 L55 76 L48 91 Z` | filled |
| String (UI) | `M48 90 C38 98 58 106 48 114 C38 122 58 130 48 138` | 4–5, round cap |
| Log tail (logo **only**) | `M48 90 C49 104 56 111 68 113 C78 115 84 115 90 116` | 5, round cap |

Body sizes in use: 29 (palette), 17 (BalloonScore), 13 (match header and report),
14 (question header), 10 (scoreboard column, **no string**).

**States:** filled = problem accepted, in the letter's color; outline 4 in `lineStrong` at
opacity 0.55 = open. The highlight only appears on a filled balloon and is dropped below a 12dp
body. The letter sits **below** the balloon in Plex Mono. In dense rows (A—M in the header and on the scoreboard) it drops to 8—9sp, below the 11sp minimum: there it is visual reinforcement, and the real accessible fallback is the `contentDescription` (`"problema C, aceito"`, "problem C, accepted"), never the glyph.

In Compose use `Path` + `PathParser` (or an `ImageVector` generated from the SVG in `brand/`);
in SwiftUI, `Path` with the same commands, scaled with `GeometryReader`.

### LifeBar
3 hearts at 20sp. Full = `wrong`; empty = `heartOff`. Mono counter `2 / 3` beside them as the fallback.

### VerdictChip
Height 26dp, min-width 46dp, radius 2dp, Plex Mono 12sp/600, tracking +0.06em.
Border in the line color, text in `…Ink`, background in the matching tint.
| Code | Label | Tone | Note shown |
|---|---|---|---|
| `AC` | Accepted | correct | balloon goes up |
| `WA` | Wrong Answer | wrong | +20 min penalty |
| `TLE` | Time Limit Exceeded | wrong | wrong complexity |
| `MLE` | Memory Limit | wrong | data structure too heavy |
| `RE` | Runtime Error | wrong | index, overflow, division |
| `CE` | Compile Error | warn | no penalty |
| `PE` | Presentation Error | warn | output format |
| `…` | Judging | textMuted | in the judge queue |

The code is the judge's, **untranslated**. The Portuguese text goes beside it, never in its place.

### ContestClock
Plex Mono, **tabular-nums** required.
| State | Size | Color | Background |
|---|---|---|---|
| normal | 38sp/500 | `textPrimary` | `canvas`, border `line` |
| frozen (last hour) | 38sp/500 | `warn` | `tintWarn`, border `warn` |
| critical question (<15s) | 28sp/600 | `wrong` | — |

Label above in `label` (Plex Mono 10.5sp, tracking +0.16em).

### Tag
Plex Mono 12sp, padding 8×12, radius 2dp. Default: border `lineStrong`, text `textSecondary`.
Selected: border + text `accent`, background accentTint.

### DropZone
Height 38–46dp, radius 2dp, Plex Mono 13sp.
Empty: dashed `lineDim`, text `textDim`. Drag hover: dashed `accent`, background accentTint.
Filled: solid border `lineStrong`, background `surfaceRaised`, text `textPrimary`.

### CodeBlock
Background `canvas`, border `line`, radius 2dp. Plex Mono 13–15sp, line height 24dp.
Line numbers in `textMuted`, not selectable, fixed column.
Highlighted line: tint background + 2dp side bar in the **line** color (not the ink).
**No horizontal scroll**: lines wrap and keep their indentation.

### MatchHeader
Height 84dp, pinned to the top, background `canvas` (it separates from the content on `surface`).
Row 1: session label (label) · ContestClock · LifeBar.
Row 2: A—M balloon row, 11×14dp, 7dp gap, 8sp letter below.

---

## Screens

### 1 · Scoreboard (big screen)
The most recognizable ICPC artifact and the only one that is **not** phone-first: it is born projected on a wall.
In the app it shows up between matches, with horizontal scroll and the user's row stuck to the bottom.

**Grid:** `52dp | minmax(190,1fr) | 56dp | 68dp | 13 × minmax(44,1fr)`, minimum width 900dp.
- **Contest header** (auto height, 16×20 padding, bottom border `line`): contest name in
  Plex Mono 13sp/600 tracking +0.12em · `CONGELADO` (FROZEN) badge (border+text `warn`, background `tintWarn`,
  radius 2dp) · 24sp tabular clock aligned right.
- **Column header** (40dp, background `surfaceRaised`): `#`, `EQUIPE` (TEAM), `SLV`, `PEN` in 10sp label,
  then A—M with a 10×11dp balloon above an 11sp/600 letter.
- **Row** (52dp): rank mono 14sp/600 · name 14sp/600 + university in 10sp label ·
  solved 15sp/600 · penalty 13sp `textSecondary` · 13 cells.
- **User's row**: accent @13% background over canvas, top/bottom border `accent`, name and rank in accent.
- **Cell** (38dp tall, 2dp side margin, radius 2dp, 1dp border): top = 12sp/600 symbol,
  bottom = 9sp minute @75%.

| Cell state | Border | Background | Top | Bottom |
|---|---|---|---|---|
| accepted | `correct` | `tintOk` | `+` or `+N` (wrong attempts) | minute of the AC |
| tried, no AC | `wrong` | `tintErr` | `−N` | — |
| post-freeze | `info` | `tintInfo` | `?` | `frz` |
| not tried | `line` | transparent | — | — |

Legend for the four colors in the footer, always visible. The freeze is not decoration: it is the last
hour in which nobody knows the result, and the UI shows **doubt** instead of hiding it.

### 2 · Skill tree: prerequisite DAG
The curriculum **is a directed acyclic graph**, and the screen shows that. An edge is not a decorative
connector: it carries direction, state and in-degree. A node with two incoming edges needs
both prerequisites, and the player understands why it is locked without reading any text.

**Vertices are brand balloons.** Same `Balloon` primitive, sized by importance:
56 for conquered, 70 for the active one, 54 for locked. Filled in the family color with a white highlight = conquered (the highlight becomes mandatory); accent tint center (accentTint) and icon in `accentInk` = active; whole silhouette filled in `lineStrong` (`line2`), with no outline or highlight, icon in `textSecondary` = locked.

**Edges are the strings.** Bézier curves that leave from inside the source node's tag
and arrive at the top of the target balloon.

| Edge state | Stroke | Color |
|---|---|---|
| traversed | 2.5, round cap, opacity 0.8 | source family color |
| active | 3, round cap | `accent` |
| locked | 2.5, `stroke-dasharray 4 6` | `lineDim` |

**NodeTag**: the label never floats loose over the map. A chip anchored just below the balloon,
centered on the node's axis (`translateX(-50%)`): background `canvas`, 1dp border `line`, radius 2dp,
padding 4×10, text 12.5sp/600. The outgoing edge starts ~3dp **inside** the chip, so the
string reads as if it ran through the tag.
| State | Border | Background | Text |
|---|---|---|---|
| conquered | `line` | `canvas` | `textPrimary` |
| active | `accent` | accentTint | `textPrimary` + 2nd line `INFLANDO · 2/5` (INFLATING) in **`accentInk`** mono 9.5sp |
| locked | `line` | `canvas` | `textSecondary` |

**In-degree badge**: an 18dp circle at the balloon's top-right corner, background `canvas`,
border `lineDim`, number in mono 9sp. It only appears when the in-degree is ≥ 2.

**Header:** balloon count (`7 balões no ar`, "7 balloons up") 20sp/600 + total XP in accent mono.
**Footer:** legend `CORDINHA TRACEJADA = ARESTA FECHADA` (DASHED STRING = LOCKED EDGE) in label.

**Node sheet** (tap on any vertex): balloon + name + state; then **Vem de** (Comes from) and
**Destrava** (Unlocks) side by side, listing the node's graph neighborhood with icon and name; metrics
(lessons, best time, XP); CTA. For a locked node, the sheet lists the incoming edges one
by one with their state, plus the XP threshold as a third requirement.

> **Layout:** node positions are **authored**, not computed at runtime: an automatic graph
> layout produces unstable results on every build. Store `x`/`y` per node in the same JSON as the
> curriculum. Above ~12 visible nodes, choose between vertical scroll with short edges (what
> is drawn here) or a canvas with pan/zoom.

### 3 · Trap sheet (bottom sheet during a match)
Content behind at 45% opacity, no blur. Sheet: background `surfaceRaised`, top border
`lineStrong`, 8dp radius on the top corners only, `shadowSheet`. Handle 36×3dp `lineStrong`.
Content: label `TRAP CLÁSSICA` (CLASSIC TRAP) in `wrong` + trap name in `textMuted` · title 17sp/600 ·
explanation 14sp `textSecondary` · two 48dp buttons (secondary "Ler explicação" (Read explanation) with the note
`pausa o timer` (pauses the timer); primary "Pular" (Skip)).

### 4 · Post-match report
Header: `CONTEST ENCERRADO` (CONTEST OVER) in label · number 40sp/600 + `/ 13 aceitos · 512 pen` in mono 15sp ·
A—M balloon row.
Body: `REVISÃO · N ERROS` (REVIEW · N ERRORS) in label, then one card per error (background `surface`, border `line`,
radius 4dp, padding 14): problem name 15sp/600 + VerdictChip on the right · `sua resposta: X` (your answer: X)
in mono 12sp `textMuted` · explanation 13sp `textSecondary`.
CTA "Entendi" (Got it).

### 5 · Profile hub
No tab bar. Entry point: a **40dp** circular avatar at the right corner of the Skill Tree's NavigationBar,
with an 11dp `warn` badge when `pending_sync_count > 0`. The profile rises as a **sheet** over the
tree (8dp radius on the top corners, `surfaceRaised`, `shadowSheet`), with the tree visible
behind at 18%, so the player does not lose their place.

**Header:** 52dp avatar (initial in `accentInk` on accent @18%, accent border) + email in
Plex Mono 13sp + a status line with a colored dot: `correct` `TUDO SINCRONIZADO` (ALL SYNCED) or
`warn` `N EVENTOS NA FILA` (N EVENTS QUEUED). A guest gets a `MODO VISITANTE` (GUEST MODE) chip in place of the email.

**Level is the hero:** number 44sp/600 + `NÍVEL` (LEVEL) label in mono, next to a progress bar
to the next level with `620 XP / 800` and `180 XP para o nível 5` (180 XP to level 5).
Computed in the core: `nivel = floor(xp / 200) + 1`. Never in the client.

**Stats:** a three-cell grid (`XP TOTAL`, `BUGS`, `DRY RUNS`), with the totals in mono below:
`47 desafios concluídos · 7 balões no ar` (47 challenges completed · 7 balloons up).

#### Deviations from the technical spec: deliberate
These three points are implementation constraints that turn into bad UX if copied literally.
The architecture (Rust core, Keychain, `offline_events.json`) stays intact; what changes is the flow.

| Spec | What ships |
|---|---|
| Confirmation alert on **every** logout | **No alert when `pending_sync_count == 0`.** Signing out while synced is reversible: signing back in restores everything. An alert there is friction with no payoff and trains the user to confirm without reading, which destroys the value of the alert that *matters*. Show a signed-out screen with **undo**. |
| `"Se sair agora, você perderá esse XP permanentemente"` ("If you sign out now, you will lose this XP permanently") | **Never offer the choice between signing out and losing progress.** The critical sheet has `Sincronizar e sair` (Sync and sign out) as primary, `Continuar conectado` (Stay signed in) as secondary, and `Sair e descartar 180 XP` (Sign out and discard 180 XP) third, as text, not a button. Loss stops being the default path. |
| `Excluir conta` (Delete account) next to `Sair` (Sign out) | The App Store requires it to be **findable**, not prominent. Next to logout, in a sheet the player opens to check XP, it invites an irreversible accident. Move it to **Gerenciar conta** (Manage account), one more tap away, together with change email, change password and **download my data**, which the same privacy guideline recommends. |

**Critical sheet** (`pending_sync_count > 0`): top border `warn`, label
`12 EVENTOS NA FILA · 180 XP` (12 EVENTS QUEUED · 180 XP), a title that **names the loss** instead of asking "are you sure".
The destructive button repeats the verb and the number, never a generic "OK".

**Manage account:** a pushed screen (not a sheet). A list of neutral actions; the deletion block
sits at the bottom, in `tintErr` with a `wrong` border, label `IRREVERSÍVEL` (IRREVERSIBLE), and requires the password + typing
the word `EXCLUIR` (DELETE). It calls the purge endpoint on the Go backend.

**Guest** (`is_guest == true`): hides Sign out and Manage account. In their place, an accent card
that names the risk ("Seu progresso vive só neste aparelho", "Your progress lives only on this device") and the gain ("Criar conta herda os
620 XP, os 7 balões e os 47 desafios", "Creating an account keeps the 620 XP, the 7 balloons and the 47 challenges"), with a primary CTA to `RegisterView` and a
secondary `Já tenho conta` (I already have an account).

### 6 · Standings (phone)
Global / Sede (Site) tabs (active tab: 2dp bottom border `accent`, 14sp/600).
44dp row: rank mono 13sp width 26dp · name 15sp/500 + university in 10sp label ·
`solved · penalty` mono 14sp tabular on the right. Divider `rowLine`.
The user's row is pinned to the bottom, with the same accent treatment as the scoreboard.
Bottom nav with 3 items (TRILHAS / ARENA / PLACAR, i.e. TRACKS / ARENA / SCOREBOARD) in label; active one in accent with a 2dp top border.

---

## Question templates
Every `template_type` uses the same shell: **fixed header → scrollable body → anchored CTA**.
None of them opens the native keyboard.

Header for all: colored 14×16dp balloon + `PROBLEM <letra>` in label, with ContestClock or LifeBar on the right.

### FILL_IN_THE_BLANK
Prompt 19sp/600 · CodeBlock with one inline blank (min-width 74dp, height 30dp, dashed
`accent`, radius 2dp) · label `ARRASTE O BLOCO` (DRAG THE BLOCK) · 3 draggable mono 13sp chips.
CTA disabled until the blank is filled.

### SPOT_THE_BUG
Prompt 19sp/600 · CodeBlock with tappable lines (34dp each + invisible padding, divider `rowLine`).
Selected line: background accentTint, 2dp side bar `accent`, number in accent.
The CTA names the choice: "Confirmar linha 3" (Confirm line 3).

### COMPLEXITY_MATCH
Prompt 19sp/600 · two rows labeled `TEMPO` / `ESPAÇO` (TIME / SPACE) (11sp label, fixed 60dp column)
with a 46dp DropZone · 1dp divider · bank of 4 mono 13sp chips.

### TRADEOFF_MATCH
Design: canvas `LogN Trade-off Match`, proposal A. Prompt 19sp/600 · optional CodeBlock ·
two slots labeled `BENEFÍCIO` / `DESVANTAGEM` (BENEFIT / DRAWBACK) (11sp label, above the text), full width,
min 56dp · 1dp divider · options as full-width rows (min 56dp, mono 12sp letter as a
prefix, 15sp text that wraps).
The options are sentences, not `O(n)`, so they do not use the COMPLEXITY_MATCH mono chip.
Tap, don't drag: the option goes to the next empty slot, benefit before drawback, and the
current slot has its label in `accentInk` and its dashes in `accent`. An option already used is
dimmed (`textDim`, border `line`) and does not respond. Tapping a filled slot empties it.
CTA "Confirmar trade-off" (Confirm trade-off), disabled until both slots are filled.

### TAG_THE_PATTERN
Prompt card in the format of the printed problem sheet: border `line`, background `surface`, radius 2dp;
top with `TIME LIMIT 1S` / `MEM 256MB` in mono 10sp separated by a divider; problem name
15sp/600; text 15sp `textSecondary`.
Question 17sp/600 · label `SELECIONE ATÉ 2` (SELECT UP TO 2) · multi-select tag grid.
The CTA counts the selection: "Confirmar 2 tags" (Confirm 2 tags).

---

## State Management
Per match:
- `problems: List<Problem>`: each with `letter: Char`, `templateType`, payload, correct answer
- `currentIndex: Int`
- `verdicts: Map<Char, Verdict>`: drives the balloon color and the scoreboard cell
- `lives: Int` (0–3) · `penaltyMinutes: Int` · `attempts: Map<Char, Int>`
- `questionTimer: Duration` (counts down, triggers the critical state at 15s)
- `contestClock: Duration` · `isFrozen: Boolean` (last hour)
- `selection` per template: index, line, dropzone map, or tag set
- `trapSheet: Trap?`: non-null pauses `questionTimer`
- `syncPending: Boolean`: shows the `SYNC PENDENTE` (SYNC PENDING) chip in the header

Transitions: submit → verdict in 120ms → on AC, the balloon fills (160ms) and advances in 180ms;
on WA, 240ms shake, `lives--`, +20min penalty, and the trap sheet opens if a trap is mapped.

## Required states (every component)
`enabled · pressed · disabled · focused · selected · correct · wrong · locked`
- **loading**: skeleton with `line` rows, **no spinner**
- **offline**: mono `SYNC PENDENTE` chip in `warn` in the header
- **empty**: never an illustration; one sentence in `textSecondary` and the CTA that resolves it

## Accessibility
- Native touch floor (48dp / 44pt); 56 during a match
- A verdict is never conveyed by color alone: the judge's code always goes with it
- A balloon is never conveyed by color alone: the letter sits below the balloon and in the `contentDescription`
- A scoreboard cell reads "problema C, aceito, 2 tentativas, 88 minutos" ("problem C, accepted, 2 attempts, 88 minutes")
- Minimum contrast 4.5:1 on text that is read; 3:1 for balloon colors (shapes, not text)
- Font scale up to 200% (Android) / Dynamic Type up to AX3 (iOS) without truncating the prompt
- `reduceMotion`: the shake becomes a border flash

## Voice and copy
- Technical terms stay in English, untranslated: `binary search`, `trap`, `contest`, `freeze`
- It is always "problema C" (problem C), never "questão 3" (question 3)
- Interface in Portuguese; system labels in monospaced English
- An error explains the cause and never judges: "Guloso falha aqui" ("Greedy fails here"), not "Você errou feio" ("You really blew it")
- No exclamation marks, no emoji, no generic congratulations

## Cross-platform parity
Geometry, color and copy are **identical**. Only these follow native conventions:
| Element | Compose | SwiftUI |
|---|---|---|
| Trap sheet | `ModalBottomSheet` | `.sheet` + `.presentationDetents` |
| Drag & drop | `detectDragGestures` + DragTarget | `.draggable` / `.dropDestination` |
| Haptic | `HapticFeedbackType` | `.sensoryFeedback` |
| Shake | `Animatable` + keyframes | `.phaseAnimator` / `KeyframeAnimator` |
| Text scaling | `sp` + fontScale 200% | Dynamic Type up to AX3 |
| Touch floor | 48dp | 44pt |
| Root navigation | `NavigationBar` 3 items | `TabView` `.tabBar`, same 3 |

## Iconography

Custom set in `brand/icons/`: 12 SVGs, viewBox `0 0 24 24`, `fill="none"`,
`stroke="currentColor"`, stroke 2, round caps and joins. Drawn from the geometry of
each technique: two pointers converge, binary search bisects, sliding window frames a
band, sorting is ascending bars.

**Do not use SF Symbols or Material Icons for algorithm topics**: they do not exist on both
platforms and they do not describe the technique. System icons (back, close, share) follow
each platform's native set as usual.

Each topic belongs to a family, and the family sets the color: four colors borrowed from the
balloon palette:

| Family | Color | Topics |
|---|---|---|
| fundamentals | balloon **H** `#F4A261` / light `#B26320` | adhoc, arrays, strings |
| search and sorting | balloon **C** `#3DB2FF` / light `#0B6FBF` | two-pointers, binary-search, sliding-window, sorting |
| structures | balloon **E** `#C77DFF` / light `#8A3FD1` | trees, graphs |
| optimization | balloon **M** `#00B894` / light `#007A61` | dp, greedy |
| boss | `accent` | challenge |

A family color is **not** a problem color: they are distinct uses of the same palette. The accent stays
reserved for the active node and the boss; no topic uses it.

> **The balloon palette is certified for 3:1 as shape, not 4.5:1 as text.** Use it for
> fills, icon strokes and edges. In legends and lists, the family appears as a 9dp colored
> dot before the label, with the label in `textSecondary`: the color becomes shape and the text
> stays legible.

## Assets
Brand symbol in `brand/` (3 SVGs) and 12 topic icons in `brand/icons/`. Beyond that, no images: everything is typography, shape and color.
**Fonts:** IBM Plex Sans (Regular/Medium/SemiBold) and IBM Plex Mono (Regular/Medium),
SIL Open Font License, download from <https://github.com/IBM/plex> and bundle them in the app
(`res/font/` on Android, target membership + `UIAppFonts` on iOS). Do not load them from a CDN.

## Files
| File | What it is |
|---|---|
| `brand/logn-symbol-*.svg` | Symbol in accent, currentColor and knockout. **Production assets.** |
| `brand/icons/*.svg` | 12 topic icons, currentColor. **Production assets.** |
| `tokens/LognColor.kt` | Dark/light colors + balloon palette. **Paste into the codebase.** |
| `tokens/LognTheme.kt` | Typography, Shapes, Space, `LognTheme` composable. **Paste into the codebase.** |
| `tokens/LognDesignSystem.swift` | SwiftUI equivalent: colors, balloons, fonts, radii, space. **Paste into the codebase.** |
| `LogN Design System v2.dc.html` | **Visual reference, the single source of truth.** Brand, tokens, components, iconography, edges, 4 templates, key screens, scoreboard. Opens in the browser; has a dark/light toggle and an accent picker. |
| `support.js` | Runtime needed to open the `.dc.html` files. Not production code. |

Open v2 in the browser and keep it open beside you during implementation: it is the source of truth
for any measurement not listed here.
