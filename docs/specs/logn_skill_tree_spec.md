# LogN - Especificação da Árvore de Trilha (Visual Skill Tree)

Este documento detalha a arquitetura e UI/UX da "Skill Tree" (o mapa principal do jogo), devendo ser utilizado pelo Agente de Design e desenvolvedores para evoluir o atual ContentView.

## 1. Topologia e Navegação
Diferente da visualização padrão de lista (List nativa), o mapa adotará a estética de um **Path/Snake**:
- **Scroll Top-to-Bottom:** O primeiro nó da jornada (ex: Ad-Hoc / Introdução) reside no topo absoluto da tela. O usuário fará o scroll natural para baixo conforme progride para revelar nós mais complexos (Grafos, PD, etc.).
- **Zig-Zag:** Os nós serão desenhados com offsets horizontais alternados (esquerda, centro, direita, centro, esquerda...) para dar a sensação orgânica de uma trilha.
- **Conectores:** Linhas contínuas ou tracejadas conectarão o centro de um nó ao outro. O caminho percorrido será colorido (Neon), e o caminho futuro será cinza escuro (`LognDark.lineDim`).

## 2. Estados Visuais dos Nós
A progressão é baseada no `global_xp` do usuário cruzado com o `required_xp` do nó. Em uma abordagem matemática de MVP:
- **Nó Ativo (Active):** O nó cujo `required_xp <= global_xp` E que ainda não foi superado pelo XP do próximo nó.
  - *UI:* Exibe a cor do balão (ex: Azul Neon), preenchido, e possui uma animação de "Pulse" (ScaleUp/Down) contínua para chamar a atenção de que é ali que ele deve clicar.
- **Nó Completado (Completed):** Nós anteriores ao nó ativo.
  - *UI:* Preenchimento sólido, com ícone brilhante ou um "Checkmark" (SFSymbol `checkmark`). Clicável para Replay.
- **Nó Bloqueado (Locked):** Nós cujo `required_xp > global_xp`.
  - *UI:* Cinza opaco/desaturado (`LognDark.surfaceRaised` com borda `lineDim`), não interativo (disabled). O ícone interno aparece escurecido.

## 3. Iconografia
- **SFSymbols:** No lugar de texto seco, a face do nó exibirá um SFSymbol representativo de seu assunto. Exemplo: `brain.head.profile` para Ad-Hoc, `network` para Grafos, `bolt.fill` para Otimização.
- **Cores Categóricas:** A paleta `Balloon.dark` (já exportada no Design System) ditará a cor base do nó. Ex: Nó 1 usa a cor A (Vermelho), Nó 2 usa a cor B (Amarelo), gerando um arco-íris progressivo pela tela de fundo `canvas`.

## 4. Integração de Dados
- O Swift lerá a lista ordenada `core.viewModel.nodes`.
- A lógica do `ContentView` calculará o índice do "Nó Ativo" em tempo real usando o XP total fornecido pelo `core.viewModel.userXP` (que precisará ser exposto pela FFI).
