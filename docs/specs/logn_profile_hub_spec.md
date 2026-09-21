# LogN - Especificação do Hub de Perfil (Profile & Logout)

Este documento descreve a arquitetura, as regras de negócio e a interface de usuário (UI/UX) para a tela de Perfil do Jogador, devendo ser seguido pelo Agente de Design e pela implementação técnica.

## 1. Ponto de Acesso e Navegação
- **Localização:** A tela não utilizará uma `TabView` fixa no rodapé. O acesso será feito através de um botão circular (Avatar ou ícone de usuário padrão) posicionado no canto superior direito do cabecalho (NavigationBar) da tela de Skill Tree.
- **Apresentação:** Ao tocar no botão, a tela de Perfil deve subir como um **Modal/Sheet** sobrepondo a árvore, mantendo a imersão.

## 2. Apresentação de Dados (Stats)
O cabeçalho do Perfil focará na identidade e no prestígio do jogador. O Core em Rust exportará as seguintes informações que devem ser desenhadas com destaque tipográfico:
- **Identidade:** O E-mail do usuário logado (ou a tag "Modo Visitante" se for *Guest*).
- **XP Total:** Somatório de pontos de experiência do jogador.
- **Nível Atual (Level):** Um número inteiro calculado dinamicamente pelo Rust com base no XP Total (ex: Nível 1 a cada 200 XP).
- **Desafios Concluídos:** Contagem total de bugs resolvidos e *Dry Runs* concluídos.

## 3. Comportamento das Ações e Segurança

### 3.1 Usuários Autenticados
Na base do modal de Perfil, existirão dois botões críticos:
1. **Sair da Conta (Logout):**
   - *UI Normal:* Botão de peso secundário. Ao tocar, um Alerta nativo de confirmação ("Tem certeza que deseja sair?") previne toques acidentais.
   - *Double Confirmation (Prevenção de Perda de XP):* Se a variável `pending_sync_count` for maior que 0 (ou seja, há eventos não sincronizados com o servidor), o alerta deve mudar para um estilo crítico alertando: *"Você possui progresso offline não salvo. Se sair agora, você perderá esse XP permanentemente. Deseja mesmo sair?"*.
   - *Under the hood:* A ação disparada limpa o Token no *Keychain* do iOS, apaga o arquivo local `offline_events.json` e reseta o Estado/RAM do Crux para os padrões de fábrica, arremessando o usuário de volta para a tela inicial de Login.
2. **Excluir Conta (Delete Account):**
   - *UI:* Botão de estilo destrutivo (vermelho / `LognDark.wrong`).
   - *Regra Apple:* Obrigatório para passar na App Store. Exige dupla confirmação e acionará um novo endpoint no backend Golang para purgar os dados do usuário do PostgreSQL.

### 3.2 Usuários em Modo Visitante (Guest)
- Caso o campo `is_guest` no estado do Crux seja verdadeiro, a tela oculta os botões "Sair" e "Excluir Conta".
- No lugar, exibe um CTA (Call To Action) principal: **"Criar Conta / Salvar Progresso"**.
- Ao clicar, o fluxo joga o visitante para a tela de `RegisterView`, permitindo que o XP e os nós já acumulados no `offline_events.json` sejam herdados pela nova conta que será criada no Go Backend.
