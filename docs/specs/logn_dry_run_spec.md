# LogN - Especificação do Jogo: Dry Run (Preveja a Saída)

Este documento detalha a arquitetura, regras de negócio e UI/UX para o novo modo de jogo "Dry Run". Deve ser utilizado pelo Agente de Design para desenhar a interface e pelos desenvolvedores para expansão do Rust Core e do Go Backend.

## 1. Visão Geral
No modo **Dry Run**, o código apresentado *não possui bugs*. O objetivo do jogador é realizar o teste de mesa mental (*mental trace*) e prever exatamente qual será o retorno da função ou a saída no console, treinando sua leitura algorítmica e capacidade de acompanhar a mutação de estados.

## 2. Estrutura de Domínio (Modelo de Dados)
O modelo `Challenge` existente no Rust Core será estendido para suportar este modo sem quebrar retrocompatibilidade:
- **`template_type`:** Será definido como `"DRY_RUN"`.
- **`content.code_lines`:** O código completo. Para evitar atrito visual, **as variáveis de entrada (inputs) já virão declaradas** nas primeiras linhas do código (ex: `let arr = [1, 2, 3];`). Isso permite reaproveitar 100% do componente de Scroll Horizontal com *Syntax Highlighting* construído no MVP.
- **`validation.validation_type`:** `"CONSOLE_OUTPUT"`.
- **`validation.expected_string`:** A saída exata esperada (ex: `"[1, 3, 2]"` ou `"42"`).

## 3. UI / UX da Tela de Jogo

### 3.1 Apresentação do Código
- A mecânica de toque nas linhas (utilizada no *Spot The Bug*) é **desativada**. O código se torna inerte/somente leitura.
- O componente visual de código continua em um `ScrollView` horizontal, mantendo a indentação e o *Syntax Highlighting* neon das palavras-chave.

### 3.2 Componente de Resposta (Terminal)
- Imediatamente abaixo do bloco de código, deve haver uma área com estética de **Console/Terminal** (ex: fundo muito escuro, talvez com a fonte verde tradicional ou cor secundária, e um cursor de digitação estilo `>_`).
- Neste `TextField`, o usuário possui liberdade para digitar qual é a saída do algoritmo.

## 4. Regras de Validação e Sanitização
Para evitar frustração com erros mínimos de digitação (ex: o usuário acertou a lógica mas errou um espaço):
- O App/Core fará uma **sanitização em tempo de execução** antes de comparar a resposta do usuário com o `expected_string`.
- **Regra:** Espaços em branco serão unificados ou removidos, e não haverá diferenciação rígida de formatação. Exemplo: Se o usuário digitar `[1,2,3]` e o esperado for `[1, 2, 3]`, o sistema acusará acerto.

## 5. Ciclo de Feedback e Punição

Ao clicar no botão "Confirmar", a mesma lógica de *Bottom Sheet* do MVP é acionada:

- **Caso Acerto:** O Bottom Sheet fica verde (`LognDark.correct`), avisa que o *trace* foi perfeito e concede +50 XP, destravando o avanço na *Skill Tree*.
- **Caso Erro:** O Bottom Sheet fica vermelho (`LognDark.wrong`). Em vez de apenas dizer "Errado", o campo de explicação trará um **Trace Explanation** focado em didática — um texto revelando em qual iteração o estado se comportou de forma inesperada (ex: *"Incorreto. A saída esperada era 15. Note que na terceira iteração do `while`, o ponteiro `j` não retrocede, violando o invariante do laço."*). A punição é de 0 XP, e o usuário é retornado ao mapa (ou permitido tentar novamente, de acordo com o fluxo do app).
