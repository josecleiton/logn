# ADR 0018: Organização do backend Go

## 1. Visão Geral

A ADR 0001 separa o monorepo em Go, Rust/Crux e clientes, mas nenhuma ADR dizia como o backend se organiza por dentro. O layout cresceu rota a rota. Os handlers e o middleware viviam no `package main`, em 24 arquivos na raiz de `backend/`, e `main.go` misturava leitura de ambiente, montagem das dependências, roteamento e os handlers de sync e de desafios.

Esta ADR fixa o desenho: a camada HTTP sai para `internal/httpapi`, e o `main` fica só com a composição.

## 2. Decisão

**Camadas simples, no layout `cmd/` + `internal/`, com a dependência só descendo.**

```
backend/
├── main.go, config.go    raiz de composição (package main)
├── cmd/                  binários auxiliares (contentcheck, legalcheck)
├── internal/
│   ├── httpapi/          rotas, handlers e middleware
│   ├── domain/           regra de negócio e acesso ao Postgres
│   ├── infrastructure/   serviços de fora: email, socialauth, cloudauth
│   ├── storekit/         verificação do JWS da App Store (ADR 0013)
│   ├── legal/            documentos legais e allowlist de HTML (ADR 0008)
│   ├── locale/           negociação de língua por lista fechada
│   └── content/          validação de conteúdo
└── schema/               migrações embutidas
```

**O `main` é a raiz de composição.** `main.go` lê o ambiente, aplica os fallbacks de desenvolvimento, aborta em produção quando falta segredo (AGENTS.md, regra 9), conecta o banco, roda migração e expurgo e sobe o `http.Server` com os timeouts. `config.go` guarda os leitores de ambiente que montam dependência (`trackKeySecretFromEnv`, `storeKitValidatorFromEnv`, `socialVerifiersFromEnv`). Handler não mora aqui.

**`internal/httpapi` é a camada HTTP.** Ela expõe duas coisas: `Deps`, com as dependências prontas, e `New(Deps) (http.Handler, error)`. O `Server` e os handlers ficam dentro do pacote.

- `routes.go` monta todas as rotas num `http.ServeMux` com padrão de método (`"POST /api/v1/sync"`), com os limitadores por rota, a verificação de origem e o gzip por fora. Rota não é registrada em nenhum outro lugar.
- `server.go` tem o `Server`, `authenticate`, `optionalAccount`, as sondas, sync e desafios.
- `*_handlers.go`, um por área (auth, otp, progression, purchases, track, social, legal, internal). O handler decodifica o pedido, tira o `user_id` do token, chama o domínio e traduz o erro para `writeError`. Regra de negócio não mora aqui.
- Middleware e transporte: `limits.go` (`limitBody`, `rateLimiter`), `gzip.go`, `origin_verification.go`, `api_errors.go` (os códigos, que são contrato com o app instalado).
- `New` devolve erro em vez de abortar quando a verificação de origem é obrigatória e está mal configurada, e o `main` aborta com ele. O que depende do pedido continua lendo o ambiente na hora (`CLOUD_SCHEDULER_AUDIENCE`, `K_SERVICE` para o `X-Forwarded-For`).

O nome é `httpapi`, não `http`: todo arquivo do pacote importa `net/http`, e o nome igual obrigaria um apelido em cada um.

**Em Go a unidade é o pacote, não o arquivo.** Todo `.go` de uma pasta compila junto e se enxerga sem import, então dividir `httpapi` em arquivos por área é só para a leitura. Arquivo com `main` próprio na raiz fica fora do build com `//go:build ignore` (`test_match_sync.go`), para não colidir com o `main` do servidor.

**`internal/` é fechado pelo compilador.** Nenhum módulo de fora do `backend` importa esses pacotes. Num repositório público, isso deixa a forma deles mudar sem quebrar ninguém.

**`internal/domain` guarda a regra e o SQL juntos.** `domain.Repository` é uma struct concreta sobre `*pgxpool.Pool`, sem interface na frente. As validações que não dependem de banco (`ValidateSync`, credenciais, hash de senha, JWT) são funções do mesmo pacote. O SQL segue a regra 9: só parâmetros posicionais (`$1`).

**Interface só na borda, onde há mais de uma implementação ou um serviço de fora a trocar no teste.** `socialauth.Verifier` (Google e Apple), `socialauth.Revoker` (a Apple) e `cloudauth.Validator` (OIDC do Cloud Scheduler) são interfaces, e o `Server` guarda um `Verifier` por provedor ligado. O banco não ganha interface enquanto só existir o Postgres.

**Ordem de import.** `main` importa `httpapi` e o resto. `httpapi` importa `domain`, `infrastructure`, `storekit`, `legal` e `locale`. `infrastructure` pode importar `domain` (`email` usa tipos do domínio para montar a mensagem), e o contrário não. `legal` e `email` dependem de `locale`. Nada em `internal/` importa `httpapi`, e nada em `domain` importa `infrastructure`.

**Teste fica ao lado do código (`*_test.go`), e teste de repositório fala com Postgres de verdade.** `setupTestDB` em `internal/domain/repository_test.go` e em `internal/httpapi/server_test.go` abre um `pgxpool` real. O que o SQL e os `CHECK CONSTRAINT` garantem só se prova contra o banco. Os testes de handler montam o `Server` direto. `routes_test.go` passa por `New` e prova que teto de corpo, limitador compartilhado e verificação de origem estão ligados às rotas; mudança na montagem vem com caso nele.

**Como encaixar uma coisa nova:**

- Rota nova: handler no `*_handlers.go` da área (ou um arquivo novo, se a área for nova), registro em `routes.go`, `limitBody` e o `rateLimiter` que couber, erro por `writeError` com código novo em `api_errors.go`.
- Dependência nova do servidor: campo em `Deps`, montada em `main.go` ou `config.go`.
- Regra ou tabela nova: função ou método em `internal/domain`, migração nova em `schema/migrations/`.
- Serviço externo novo: pacote em `internal/infrastructure/<nome>`, com interface se houver mais de um provedor ou se o teste precisar trocá-lo.
- Binário novo: `cmd/<nome>/main.go`.

## 3. Alternativas descartadas

- **Hexagonal, com porta para o repositório.** Não é idiomático em Go. A interface ficaria do lado de quem implementa, espelhando cada método do `Repository`, quando Go pede interface pequena declarada por quem consome (*accept interfaces, return structs*). Toda porta teria o Postgres como único adaptador, e a camada de serviço repassaria a chamada sem regra própria. O pedaço que se paga, interface onde há mais de uma implementação, já existe na borda.
- **Manter os handlers no `package main`.** Era o estado anterior. `main.go` misturava composição e handlers, e não havia fronteira entre o que monta o servidor e o que atende o pedido.
- **`main.go` em `cmd/server/`, como os outros binários.** Alinharia o layout, mas mexe no `Dockerfile` e no `justfile` sem ganho para a separação que motivou a mudança. Fica na raiz.

## 4. Consequências

- `main.go` caiu de 437 para cerca de 150 linhas e não tem mais handler.
- Testar regra que passa pelo `Repository` exige Postgres rodando. É mais lento que um mock, e em troca o teste pega o erro de SQL e de constraint que um mock esconderia.
- O domínio tem estado global de configuração: `domain.JwtSecretKey` e `domain.TrackKeySecret` são variáveis de pacote preenchidas pelo `main` na subida. Teste que depende delas precisa preenchê-las, e dois testes com segredos diferentes não podem rodar em paralelo. Passar esses valores por `Deps` e pelo `Repository` resolve, e fica para quando incomodar.
- A verificação de origem ainda lê o ambiente dentro de `New`. Ela poderia vir montada em `Deps`, como as outras dependências. Ficou onde estava para a mudança de pacote não mexer em defesa.
- `internal/domain` tende a crescer. Se um conjunto de arquivos passar a ter tabela e regra só suas, sem tocar `users` nem sessão, ele pode sair para um pacote próprio em `internal/`, seguindo a ordem de import acima.
