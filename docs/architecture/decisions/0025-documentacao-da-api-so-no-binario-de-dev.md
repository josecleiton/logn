# ADR 0025: documentação da API só no binário de desenvolvimento

## 1. Contexto

O backend não tinha descrição nenhuma das rotas fora do código. Para testar uma rota à
mão era preciso ler o handler, montar o `curl` e lembrar o código de erro de cabeça.

A imagem de produção tem 35 MB, é `distroless` e não tem nada além do servidor. A
documentação não podia acrescentar um byte a ela nem abrir rota nova no serviço de
verdade, que é público e lido por quem ataca (regra 9).

## 2. Decisão

**Spec por anotação, com swaggo.** Cada handler leva um bloco `@Summary`, `@Param`,
`@Success`, `@Failure` e `@Router` no comentário, e o cabeçalho do spec mora acima do
`main`. `just api-docs` roda `swag init` e grava `internal/httpapi/apidocs/swagger.json`
(Swagger 2.0), que fica versionado: o binário de desenvolvimento sobe sem precisar
gerar nada.

A dependência é só a CLI: `github.com/swaggo/swag` v1.16.6, declarada como `tool` no
`go.mod`, com versão e checksum no `go.sum`, e chamada por `go tool swag`. Nenhum pacote
de swaggo é importado pelo servidor, então ela não entra em binário nenhum. A v2 ficou
de fora por ainda estar em RC.

**Só com `-tags dev`.** `apidocs_dev.go` (`//go:build dev`) registra `GET /docs`,
`GET /docs/init.js` e `GET /docs/swagger.json`. `apidocs_off.go` (`//go:build !dev`)
define a mesma função vazia, e o `Dockerfile` compila sem tag. `just run-backend` passa
a rodar `go run -tags dev .`. O binário de desenvolvimento **aborta** se subir com
`K_SERVICE` setado (`errDevBuildInProduction`), como os outros guardas de produção.

**Interface pela jsDelivr, com versão e SRI fixos.** A página carrega
`swagger-ui-dist@5.33.1` com `integrity` sha384 nos dois arquivos; o script de
inicialização sai de `/docs/init.js`, para a CSP da página não precisar de
`'unsafe-inline'`. Nenhum JS de terceiro entra no repositório. Vendorizar os ~1,7 MB
funcionaria offline, mas deixaria código de terceiro versionado num repositório público
para atualizar à mão.

**Anotação esquecida quebra teste.** `TestAPIDocsCoverEveryRoute`, sem tag, lê do
código-fonte o padrão de todo `HandleFunc` do pacote e compara com o `swagger.json` nas
duas direções. Um `HandleFunc` com padrão que o teste não sabe ler também falha.

## 3. Consequências

- O binário de produção não ganha código. Compilado com o comando do `Dockerfile`
  antes e depois da mudança, tem o mesmo tamanho, os mesmos 29.778 símbolos com os
  mesmos tamanhos, e a desmontagem de `httpapi` e `main` é a mesma instrução por
  instrução. O hash muda, e mudaria com qualquer comentário: a anotação desloca as
  linhas, a `.gopclntab` (a tabela de linhas do stack trace) cresce alguns bytes e
  empurra o endereço do que vem depois dela.
- Mudou rota, corpo ou código de erro, roda-se `just api-docs`. O teste pega rota
  nova ou removida; anotação desatualizada de uma rota que continua existindo, não.
- A página `/docs` precisa de internet para carregar a interface.
- `just test-backend` roda os testes duas vezes, com e sem a tag, porque cada binário
  tem um teste que só existe nele.
