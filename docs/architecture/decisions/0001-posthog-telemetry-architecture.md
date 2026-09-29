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

## Adendo (28 de setembro de 2026): logs e erros de verdade

O `LogError` saiu da telemetria para a capability `MonitoringOperation`, e em setembro
de 2026 o Core ainda não o emitia em lugar nenhum: nenhum erro chegava ao PostHog. Três
mudanças:

1. **Capability de log** (`LogOperation { level, message, attributes }`, `Effect::Log`),
   fire-and-forget pelo `notify_shell`. O provedor é o shell: no iOS, `captureLog` do
   PostHog (produto Logs), com `serviceName = "logn-ios"`. Trocar de provedor mexe só no
   `CoreWrapper`.
2. **Erro vira `$exception`.** O shell trata `MonitoringOperation::LogError` com
   `captureException`, que agrupa por mensagem em issues do Error Tracking. O stack
   trace é o do shell; o erro nasceu no Core, e a mensagem diz onde.
3. **As respostas HTTP são observadas num lugar só**, na entrada do `update`
   (`observe_http`), em vez de handler por handler:
   - sem rede: log `Info`, porque é o normal de um app offline-first;
   - timeout: `Warn`;
   - 5xx: log `Error` e `LogError`, que abre issue;
   - 4xx e sucesso: nada, porque é erro de quem usa e a tela já trata.

   A rota vai com `{id}` no lugar do valor, e a frase do erro de rede fica de fora,
   porque pode trazer o endereço inteiro. Mensagem e atributos seguem a regra 9: sem
   token, senha, OTP, e-mail nem corpo.

Logs e erros saem mesmo com "Análise de uso" desligada: são diagnóstico, não uso, como a
política já diz dos erros. Session replay continua desligado.
