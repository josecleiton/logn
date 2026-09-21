# ADR 0001: Arquitetura de Telemetria com PostHog

**Status:** Aceito
**Data:** 21 de Setembro de 2026

## Contexto
Precisamos rastrear o engajamento dos usuários (e.g., desafios respondidos) e capturar erros no aplicativo para melhorar a experiência e solucionar bugs. No entanto, queremos identificar o usuário **apenas pelo seu ID**, respeitando sua privacidade. Além disso, as chamadas de telemetria devem ser transparentes para as regras de negócio em Rust.

## Decisão
1. Adotamos o **PostHog** como ferramenta unificada de Product Analytics e Error Tracking.
2. Criamos uma Capability (`TelemetryOperation`) no **Crux Core (Rust)**. Toda requisição analítica (`Identify`, `Track`, `LogError`) é despachada como um efeito (`Effect::Telemetry`) para a camada nativa.
3. A camada nativa (`LogNiOSApp.swift` no iOS) implementa o SDK do PostHog via Swift Package Manager e resolve o efeito.
4. **Gate de Ambiente**: A inicialização do PostHog depende de uma variável de ambiente `TELEMETRY_KEY` injetada via `Local.xcconfig`. Se estiver vazia (comportamento padrão de desenvolvimento local), o SDK não inicia, impedindo que testes locais sujem os dados de produção.

## Consequências
* **Positivo:** A lógica de negócio no Core permanece pura e testável, sem dependência direta de redes ou bibliotecas fechadas de telemetria.
* **Positivo:** Respeita a privacidade exigida (`identify` usando apenas o `user_id`, não e-mails nem nomes).
* **Positivo:** Evita ruídos em produção gerados por compilações em dev.
