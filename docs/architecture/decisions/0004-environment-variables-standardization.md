# ADR 0004: Padronização de Variáveis de Ambiente

**Status:** Aceito
**Data:** 21 de Setembro de 2026

## Contexto
Tanto o backend em Go quanto os shells móveis precisam conhecer hosts de banco de dados, chaves de telemetria e o host da API. Havia URLs em formato *hardcoded* (`localhost:8080`) diretamente na lógica de negócio do Core Crux (Rust), o que impossibilitaria deploys flexíveis ou execuções em aparelhos físicos.

## Decisão
Adotamos injeção de propriedades no momento do build e interceptação via shell.
1. **Padrão `.env` Local:** Um `.env` gitignorado dita os apontamentos no backend.
2. **`Local.xcconfig` no iOS:** Arquivo de Build Settings lido pelo `XcodeGen` durante a compilação, cujas chaves se tornam acessíveis em *runtime* pela leitura do `Bundle.main.infoDictionary`.
3. **Rust Core Agnóstico:** O core Crux não tem conhecimento de hosts ou `API_BASE_URL`. Ele despacha endpoints relativos (ex: `/api/v1/sync`). O interceptor no shell iOS lê a variável de ambiente, constrói a URL qualificada baseada nela, e executa o disparo, devolvendo ao Core a resposta.

## Consequências
* **Positivo:** Mudanças de servidor de API não tocam a lógica de Rust nem exigem recompilações da ponte FFI.
* **Positivo:** Gatekeeper de telemetria via `TELEMETRY_KEY` garante que chaves publicadas nunca manchem relatórios com dados de build local.
