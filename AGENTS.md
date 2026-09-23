# LogN - Agent Context & Guidelines

Este arquivo contém as diretrizes e regras arquiteturais do projeto **LogN**, servindo de contexto para agentes de IA que auxiliam no desenvolvimento.

## 📚 Documentação e Referências
- **Design System:** Localizado em `docs/design_system/`. Contém todas as especificações de UI/UX, cores, semântica e exportações de tokens (`tokens/`).
- **Decisões Arquiteturais (ADRs):** Localizadas em `docs/architecture/decisions/`. Antes de alterar paradigmas do sistema, leia as ADRs para entender o contexto das decisões passadas.
- **Teste no simulador:** `docs/testing/roteiro-simulador-ios.md`. Percorre o app inteiro tela a tela por linha de comando — geometria da janela, toque e arrasto sintéticos, atalhos de DEBUG e o que conferir em cada tela. Use antes de dar uma mudança de UI como pronta: os bugs mais caros deste projeto passaram por build, teste unitário e revisão, e só apareceram jogando.

## 🏗️ Arquitetura do Sistema
O LogN adota um padrão de **Monorepo** com separação clara de responsabilidades:
1. **Backend (Go):** Responsável pela validação do Sync, Auth e persistência. Focado em escalabilidade (concorrência em Go). Usa PostgreSQL com suporte a JSONB.
2. **Shared Core (Rust/Crux):** A "Mente" do cliente. Contém *toda* a lógica de negócios, regras de estado (Model) e o motor offline-first (Mini-Git). **Não usa UniFFI**. Usa uma ponte FFI nativa via `boltffi` e `bincode`, com tipagem gerada via `facet_typegen`.
3. **Clients (SwiftUI / Kotlin):** Camadas "burras" de renderização. Elas enviam eventos para o Core (`Event`) e recebem o modelo de visualização purificado (`ViewModel`).

## 🚨 Regras Rígidas de Implementação
1. **Zero Colisão de Nomes no Crux:** Qualquer novo tipo (Model, Event, ViewModel) adicionado em Rust deve possuir a anotação `#[derive(Facet)]` e `#[facet(fg::namespace = "LogN")]` para o *typegen* respeitar o namespace no iOS/Android.
2. **Offline-First via Cryptographic Chaining:** Todo evento do usuário de jogo deve possuir um Hash de integridade (`SHA-256(Hash(N-1) + Payload + Timestamp)`). O Go Backend deve apenas validar esse hash, nunca recalculá-lo para reescrever o histórico.
3. **Persistência de Desafios:** Desafios são armazenados no PostgreSQL em Go através de uma coluna polimórfica `JSONB`. Mutações no schema de desafios vão numa migração nova em `backend/schema/migrations/` (a `0000` é o schema de partida; nunca edite uma já aplicada), acompanhadas dos `CHECK CONSTRAINTS` de validação da estrutura JSON. Migração de schema nova precisa ser liberada pelo nome no `.gitignore`.
4. **Dependências Crux FFI:** Manter o padrão de FFI nativa deste repositório: a comunicação Rust <-> Swift é trafegada *exclusivamente* via bytes `[u8]` (Bincode) passando pelas funções exportadas em `boltffi::export`. 
5. **Toda tabela tem `created_at`. Toda tabela que sofre `UPDATE` tem `updated_at` também.**
   `TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP` nos dois. Vale para
   tabela nova e para `ALTER TABLE` que acrescente escrita a uma tabela que só era lida.

   O motivo é depuração, não burocracia: sem isso não dá para responder "quando esta
   linha apareceu" nem "isto mudou antes ou depois daquela migração", e as duas perguntas
   já apareceram investigando bug de conteúdo aqui.

   `updated_at` só vale se for mantido — ou um `trigger` de `BEFORE UPDATE`, ou a coluna
   no `SET` de todo `UPDATE`. Coluna que nasce com a linha e nunca mais anda mente pior
   que coluna nenhuma.

   **Estado atual, para quem for mexer:** a regra está quase toda por cumprir. Só
   `users`, `skill_nodes` e `refresh_tokens` têm `created_at`; **nenhuma tabela tem
   `updated_at`**, incluindo `challenges`, `users`, `user_progress` e `user_sync_state`,
   que sofrem `UPDATE` de verdade. `game_events` é append-only por desenho e só precisa
   de `created_at`; `schema_migrations` resolve o dela com `applied_at`.

## 🔄 Fluxo de Trabalho do Agente
1. Ao iniciar, revise sempre se as dependências do `Crux` e o pacote `boltffi` exigem recompilação (`cargo build --features codegen`).
2. Atualize o `codegen` e rode-o se você tocar nas definições de tipagem (`shared_core/src/bin/codegen.rs`).
3. Gere e atualize ADRs em `docs/architecture/decisions/` ao introduzir novas bibliotecas centrais (ex: Lib de Auth) ou mudar arquitetura.
