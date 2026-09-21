# ADR 0005: Authentication & Security Architecture

## 1. Visão Geral
Sistema de autenticação baseado em **Access Token (JWT)** e **Refresh Token (Opaque/UUID)**, orquestrado 100% pela mente em Rust (Crux) com chaves protegidas no Vault nativo do dispositivo.

## 2. Decisões Arquiteturais

### A. Fluxo de Tokens (Access & Refresh)
*   **Access Token (JWT):** Curta duração (ex: 15 min). Fica armazenado apenas em **memória no Model do Rust**. É anexado em todas as requisições HTTP (`Authorization: Bearer <JWT>`).
*   **Refresh Token (UUID):** Longa duração. Fica armazenado no **Keychain/Keystore do dispositivo**. O Rust requisitará a leitura/escrita desse token via uma nova Capability do Crux chamada `SecureStore`.

### B. Orquestração do Retry (401 Unauthorized)
A máquina de estados do Crux controla o retry:
1. O Rust dispara um `HttpRequest` (ex: `SyncNow`).
2. O Servidor retorna `401 Unauthorized` porque o JWT expirou.
3. O Rust intercepta no `SyncCompleted` (status 401), pausa a fila, dispara um Request via `SecureStore` para pegar o Refresh Token no iOS.
4. O Rust chama `POST /api/v1/auth/refresh`. Se der sucesso (novo JWT), ele salva na memória e re-dispara o evento de `SyncNow` automaticamente.

### C. Backend (Go & PostgreSQL)
*   **Providers:** Suporte a GitHub, Google, Apple e Email/Senha.
*   **Hashing:** Senhas tradicionais serão hasheadas usando **Argon2id**.
*   **Gestão de Sessão:** A tabela `refresh_tokens` guardará os hashes ou UUIDs dos Refresh Tokens ativos junto com o `user_id`, `device_id` e `revoked`. Isso permite revogar sessões remotamente (banir cheaters).
*   **OAuth Mobile:** O App iOS usará bibliotecas nativas (`AuthenticationServices` para Apple, SDK para Google) para obter o **Provider Token**, e mandará via `POST /api/v1/auth/social` para o Go validar a assinatura com a Apple/Google e emitir o JWT nativo do LogN.

## 3. Próximos Passos de Implementação
1. Adicionar Capability `SecureStore` no `crux_core`.
2. Criar a tabela `users` e `refresh_tokens` no `init.sql`.
3. Criar os endpoints `/auth/register`, `/auth/login` (Argon2) e `/auth/refresh` no Go.
4. Mapear o State Machine de Retry no Rust.
