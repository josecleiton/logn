# ADR 0001: Arquitetura Monorepo com Go e Rust (Crux)

## Status
Aceito. A ordem de lançamento ("iOS agora, Android no futuro") foi revista na ADR 0022: o lançamento público é no Android.

## Contexto
O LogN requer um backend escalável e um client altamente consistente, capaz de operar em plataformas iOS e Android futuramente, compartilhando exatamente as mesmas lógicas de progresso, repetição e offline-first. Manter códigos de lógica de negócios isolados em Swift e Kotlin resulta em desvios de regra de negócio, retrabalho e bugs inconsistentes.

## Decisão
Adotaremos a arquitetura do framework **Crux**:
1. O **Core (Rust)** congrega toda a lógica de estado, redução e eventos. 
2. As **Shells (iOS/SwiftUI)** apenas renderizam a UI e repassam os *Side-Effects* (Capabilities) solicitados pelo Core (como salvar localmente, tocar som ou requisição HTTP).
3. O **Backend (Go)** manipula as validações criptográficas de sincronização e o armazenamento relacional em alta concorrência.
Tudo residirá em um único Monorepo.

## Consequências
- Acelera o desenvolvimento cross-platform (iOS agora, Android no futuro).
- Aumenta a complexidade do setup inicial e gestão da ponte FFI.
- O time precisa entender o paradigma *Behavioral UI* do Crux (Event -> Update -> Render -> View).
