# ADR 0004: Polimorfismo de Desafios via JSONB no PostgreSQL

## Status
Aceito

## Contexto
O LogN possui diversos modos de micro-jogos (`SPOT_THE_BUG`, `FILL_IN_THE_BLANK`, etc). Tentar modelar as validações, posições e strings corretas de cada modo em colunas relacionais do SQL geraria dezenas de tabelas de junção e campos anuláveis inúteis, tornando a API engessada para novos modos de jogo (ex: se quisermos adicionar Drag n Drop).

## Decisão
Usar **PostgreSQL com colunas JSONB** para o catálogo de desafios:
- Tabela única `challenges`.
- Atributos base (ID, Chapter, Versão e Tipo) em colunas relacionais rígidas.
- Todo o dado de apresentação e validação reside no `payload` JSONB.
- A consistência do JSONB é garantida pelo banco via `CONSTRAINT chk_payload_structure CHECK (...)`, blindando o banco de lixo, porém flexível o suficiente.

## Consequências
- Novos tipos de template não requerem Migration DDL (CREATE TABLE), a não ser atualização na Constraint caso queiramos travar garantias explícitas no banco.
- O Go precisará mapear isso num `json.RawMessage` no struct do repositório para repassar facilmente via API.
