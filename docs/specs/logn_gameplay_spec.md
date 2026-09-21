# Logn - Especificação do Core de Gameplay (Spot The Bug)

Este documento concentra as decisões arquiteturais, de fluxo e de UI/UX referentes à camada principal do app (Gameplay). Ele deve ser usado como base para guiar o Agente de Design e a futura implementação.

## 1. Modelos de Interação com o Código

A infraestrutura atual em Rust envia um objeto `Challenge` que possui dois tipos de validação (`LINE_MATCH` e `EXACT_MATCH`). As mecânicas de interação foram definidas da seguinte forma:

### 1.1 `LINE_MATCH` (Ache a linha do Bug)
- **Ação do Usuário:** O usuário visualiza o bloco de código e toca em uma linha específica.
- **Feedback Visual (UI):** A linha tocada recebe um *highlight* (destaque de fundo e cor da fonte alterada, seguindo os tokens de seleção do `LognDesignSystem`).
- **Submissão:** Ao selecionar uma linha, um botão fixo no rodapé ("Confirmar" / "Submit") se torna habilitado (`disabled = false`). O usuário precisa clicar nele para confirmar a escolha, evitando *missclicks*.

### 1.2 `EXACT_MATCH` (Corrija o Bug)
- **Ação do Usuário:** O usuário toca em um pedaço do código (ou linha específica onde o erro se encontra).
- **Mutação da UI:** A linha/trecho selecionado se transmuta em um campo de texto embutido (`TextField` nativo do SwiftUI).
- **Correção:** O usuário digita a string correta usando o teclado padrão do celular (ex: alterando `i < n` para `i <= n`).
- **Submissão:** O botão "Confirmar" é pressionado para engatilhar a validação no estado central (Crux).

## 2. Renderização do Snippet de Código

Devido às restrições de telas *mobile*:
- **Layout Fixo Horizontal:** O código nunca sofrerá *word-wrap* forçado. Ele será inserido dentro de um `ScrollView` de eixo horizontal (`.horizontal`). Isso preserva 100% da indentação e da estrutura reta visual típica de uma IDE.
- **Tipografia:** Uso estrito de fonte monoespaçada (`LognFont.mono` ou sistema `.monospaced()`).
- **Syntax Highlighting Obrigatório:** Diferente de uma abordagem monocromática, a renderização deve obrigatoriamente possuir *Syntax Highlighting* básico. Como os desafios envolvem lógica de programação padrão (onde as linguagens usam inglês para *keywords*), devem ser aplicadas cores específicas (baseadas no `LognDark`) para distinguir palavras-chave, strings e números. O componente visual deve ser capaz de *parsear* ou aplicar formatação em texto simples.

## 3. Ciclo de Feedback e Punição

Ao pressionar "Confirmar", o evento chega à máquina de estados do Rust, que devolve o veredito instantâneo:

- **Feedback Visual (Bottom Sheet):** Um painel animado desliza da parte inferior da tela.
- **Caso Erro (Miss):**
  - O painel assume a cor de erro (ex: `LognDark.warn` ou vermelho de erro do Design System).
  - É renderizada uma *explicação técnica detalhada* do porquê o código estava errado.
  - **Punição:** O usuário ganha 0 XP pela tentativa errada. O botão no bottom sheet exibe "Continuar", que ao ser clicado ejeta o jogador de volta para a Árvore de Trilha (Mapa), deixando o nó não concluído.
- **Caso Acerto (Hit):**
  - O painel exibe a cor de sucesso (`LognDark.success`).
  - Animação de XP (+50 XP). O nó na Skill Tree passa a ser visualmente contabilizado como "Completo".

## 4. Arquitetura do Motor Offline (O Sync Engine)

Como o LogN é *Offline-First* com restrições criptográficas para impedir cheating:

- **Persistência Imediata (SQLite):** Todo evento de `ChallengeAnswered` não pode ficar apenas na RAM (evitando perda em *crash* ou fechamento abrupto de app). Ele será obrigatoriamente gravado de forma imediata em uma tabela ou arquivo local persistente nativo. 
- **Decisão Arquitetural iOS:** Utilizaremos o **SQLite** nativo (ou equivalente seguro como CoreData rápido) como motor de cache local de eventos.
- **Geração de Hash:** Antes de salvar na tabela, a biblioteca em Rust encadeia o evento (`Hash(N-1) + Payload + Timestamp`).
- **Batches Assíncronos:** O `SyncNow` consome a fila do SQLite localmente e efetua as requisições `POST /api/v1/sync` quando a conexão de rede for reestabelecida.
