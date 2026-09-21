# ADR 0002: Offline-First Sync via Cryptographic Chaining (Mini-Git)

## Status
Aceito

## Contexto
O modelo de micro-aprendizagem do LogN exige que o usuário não seja interrompido por oscilações de rede. Ele deve conseguir completar dezenas de desafios de código no metrô e o aplicativo deve apenas despachar isso para o servidor quando estiver online, mas assegurando 100% de integridade transacional dos pontos e XP.

## Decisão
A sincronização será inspirada no Git:
- Cada ação gerará um `GameEvent` no dispositivo contendo seu payload e Timestamp.
- O Client (Rust) fará o hash do evento em cadeia: `CurrentHash = SHA256(PreviousHash + ID + EventType + Payload + Timestamp)`.
- O servidor manterá registro da "Top Hash" (HEAD) do usuário.
- Durante o POST `/api/v1/sync`, o Go recalcula a cadeia. Se o `PreviousHash` do primeiro elemento diferir da Top Hash do server, retorna `409 Conflict` provocando um Rebase do lado do client.

## Consequências
- Fim de problemas de duplicidade ou *replay attacks* em requisições offline atrasadas.
- Qualquer alteração manual no JSON no SQLite do celular invalidará o hash, blindando a gamificação contra cheaters simples.
