# LogN - PRD: Conteúdo em várias línguas

Primeiro de três PRDs em sequência: este, depois o jurídico (`logn_legal_spec.md`), depois
as trilhas pagas (`logn_trilhas_pagas_spec.md`). Os outros dois dependem deste: os
documentos legais saem por língua, e uma trilha paga vendida fora do Brasil precisa estar
na língua de quem compra.

## 1. Por quê

O app vai para as lojas do mundo todo, menos União Europeia, Reino Unido e China (ver o PRD
jurídico). A interface já é bilíngue, com o catálogo em `i18n/locales/`. A base de
conhecimento só existe em português: enunciados, explicações, nomes dos nós e o próprio
código dos desafios.

## 2. Decisões

| Tema | Decisão |
|---|---|
| Línguas | Português (`pt-BR`), inglês (`en`) e espanhol (`es`) no conteúdo, na interface e nos documentos legais. |
| Código dos desafios | Uma versão só, em inglês, para as três línguas. Só o texto é traduzido. |
| Armazenamento | Tabela de traduções por desafio e língua, e outra por nó e língua. O `payload` guarda só a estrutura neutra. |
| Sem tradução | Um nó só aparece numa língua quando todos os desafios dele estão traduzidos nela. |
| Lançamento | Nenhum país abre nas lojas antes de a trilha principal estar completa nas três línguas, nem o Brasil. |
| Língua pedida | `Accept-Language` na requisição. A semente do bundle leva as três línguas. |
| Erros do backend | Código estável no corpo, e o app traduz pelo próprio catálogo. O servidor nunca devolve frase traduzida. |
| Tradução | A IA faz o rascunho no fluxo de revisão cega que o conteúdo já usa, e o autor aprova antes de publicar. |

## 3. Código dos desafios em inglês

Os identificadores e comentários hoje estão em português (por exemplo `maiorValor`, `resultado`).
Traduzir o código mudaria as linhas que o gabarito aponta: `correct_line` do SPOT_THE_BUG
e `expected_string` do DRY_RUN e do FILL_IN_THE_BLANK. Com uma versão só, em inglês, o
gabarito continua um só nas três línguas.

- Os desafios atuais são reescritos com identificadores e comentários em inglês, no
  repositório de conteúdo, **com os mesmos ids**. O progresso de quem já jogou continua
  apontando para o desafio certo.
- Saída de DRY_RUN e opção de FILL_IN_THE_BLANK não podem depender de língua: nada de
  string impressa em português, nem opção que seja palavra. Desafio que hoje dependa disso
  é reescrito.
- A revisão cega confere, a cada reescrita, que a linha do bug, a saída esperada e as
  opções continuam valendo.

## 4. Modelo de dados

Migração nova de schema em `backend/schema/migrations/`, liberada pelo nome no
`.gitignore`. O texto das traduções entra por migrações do repositório de conteúdo, como já
acontece com os desafios.

```sql
CREATE TABLE challenge_translations (
    challenge_id VARCHAR(50) NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    locale       VARCHAR(8)  NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    title        TEXT NOT NULL,
    description  TEXT NOT NULL,
    explanation  TEXT,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (challenge_id, locale)
);

CREATE TABLE skill_node_translations (
    node_id     UUID NOT NULL REFERENCES skill_nodes(id) ON DELETE CASCADE,
    locale      VARCHAR(8) NOT NULL CHECK (locale IN ('pt-BR', 'en', 'es')),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (node_id, locale)
);
```

Os dois `updated_at` são mantidos por `trigger` `BEFORE UPDATE`, com a mesma
`set_updated_at()` da migração `0013` (regra 5 do AGENTS.md).

A migração copia o texto atual para `pt-BR`: `content.title`, `content.description` e
`validation.explanation` de cada `payload`, e `name` e `description` de cada nó. Depois tira
esses três campos do `payload`. As `CHECK CONSTRAINTS` de `challenges` validam estrutura
(`code_lines`, `options`, `validation`), não texto, e não mudam.

**Publicação.** O nó entra no catálogo de uma língua quando o próprio nó e todos os
desafios dele têm tradução nela. É uma condição da consulta, não uma coluna: não existe
flag de "publicado" para esquecer de atualizar.

## 5. API

- `GET /api/v1/nodes` e `GET /api/v1/challenges` leem o `Accept-Language` e resolvem a
  língua em três passos: igualdade exata (`pt-BR`), depois a língua sem região (`es-MX`
  vira `es`), e por último `pt-BR`. A resposta leva a língua resolvida no cabeçalho
  `Content-Language`.
- **O formato da resposta não muda.** `title`, `description` e `explanation` voltam
  achatados nos mesmos lugares de hoje, já na língua resolvida. O modelo do Core não
  precisa mudar para ler conteúdo.
- O shell manda o `Accept-Language` de forma explícita, com as línguas preferidas do
  aparelho, e não depende do cabeçalho que o `URLSession` põe sozinho.

### Erros com código

Todo erro de API passa a responder JSON:

```json
{ "code": "otp_resend_too_soon", "message": "otp requested too soon" }
```

`code` é estável e é o que o app lê. `message` é inglês, para log e depuração, e nunca vai
para a tela. Códigos iniciais, um por motivo que hoje existe no backend:

| Código | Status | Onde |
|---|---|---|
| `invalid_request` | 400 | corpo ilegível |
| `invalid_email` | 400 | cadastro, troca de senha, pedido de código |
| `password_too_short`, `password_too_long` | 400 | cadastro, troca de senha |
| `invalid_credentials` | 401 | login |
| `otp_invalid` | 401 | código errado, expirado ou esgotado |
| `session_invalid` | 401 | refresh inválido, vencido ou reutilizado |
| `unauthenticated` | 401 | token ausente ou inválido |
| `email_taken` | 409 | cadastro |
| `rebase_required` | 409 | sync (o corpo continua levando `server_top`) |
| `sync_rejected` | 403 | cadeia de hash inválida |
| `otp_resend_too_soon` | 429 | pedido de código antes do intervalo |
| `rate_limited` | 429 | limite por IP |
| `internal` | 500 | qualquer falha interna |

Com os dois 429 separados por código, o Core deixa de decidir o escopo do bloqueio pela
rota: `otp_resend_too_soon` trava só o envio, `rate_limited` trava todas as ações de conta.

## 6. Core e cliente

- **Texto que hoje está escrito direto no Core** vira chave, no mesmo desenho do
  `StatusKey`: a explicação genérica de resposta errada (`match_engine.rs:39`) e o título
  "O relógio da questão zerou" (`match_engine.rs:439`). A cópia vai para
  `i18n/locales/`.
- **Espanhol no catálogo da interface**: `i18n/locales/es.toml`, com as mesmas chaves de
  `pt-BR.toml` e `en.toml`, gerado pelo `just i18n`.
- **Semente do bundle**: passa a levar as três línguas. O formato muda e o
  `TRAIL_SEED_VERSION` sobe. Com 36 desafios o arquivo continua pequeno.
- **Língua no Core**: o shell informa a língua na abertura, junto do `Tick`, e de novo
  quando o sistema avisa que ela mudou. O Core usa essa língua para escolher a semente e
  para descartar o retrato offline de outra língua.
- **Retrato offline**: guarda a língua em que foi baixado. Se a língua do aparelho mudou,
  o retrato vale até o próximo fetch com rede, e é substituído.

## 7. Fora do escopo

- Línguas além das três.
- Escrita da direita para a esquerda.
- Tradução dos documentos legais: ela está no PRD jurídico, mas usa as três línguas
  decididas aqui.
- Mudar a língua pelo app. Vale a língua do sistema.

## 8. Critérios de aceite

1. Com o aparelho em inglês, a trilha principal inteira abre em inglês, online e sem rede,
   numa instalação nova.
2. Com o aparelho em alemão, que não é uma das três línguas, o conteúdo sai em português,
   e o `Content-Language` diz `pt-BR`.
3. Um nó com um único desafio sem tradução em espanhol não aparece para quem está em
   espanhol, e aparece inteiro para quem está em português.
4. Nenhum desafio tem identificador, comentário ou saída esperada em português.
5. O progresso de uma conta criada antes da migração continua apontando para os mesmos
   desafios.
6. Cada erro de API da tabela acima responde JSON com o `code` correspondente, e o app não
   mostra `message` em lugar nenhum.
7. `rg '"[A-ZÁÉ][a-zçãõáéíóú]+ ' shared_core/src/match_engine.rs` não acha frase de
   interface fora dos testes.

## 9. Riscos

- **Tradução técnica errada.** "Cota apertada" e "espaço auxiliar" têm tradução certa e
  tradução que ensina errado. A revisão precisa de um glossário por língua, escrito antes
  da primeira leva.
- **O lançamento espera a língua mais lenta.** Com "tudo junto", o Brasil só abre quando o
  espanhol estiver pronto.
