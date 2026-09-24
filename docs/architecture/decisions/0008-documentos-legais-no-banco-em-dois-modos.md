# ADR 0008: Documentos legais no banco, uma página em dois modos

## 1. Visão Geral

O app precisa de termos de uso e política de privacidade em três lugares: a URL que vai para a App Store, a tela do app que mostra os dois, e o cadastro, que grava qual versão a pessoa aceitou. As três línguas (pt-BR, en, es) valem desde o lançamento, e uma mudança relevante no futuro tem de pedir novo aceite no app.

O plano da loja (plano no repositório de conteúdo, `docs/planos/<nome>`, decisão 2) escolheu HTML embutido no binário Go com a versão como constante. O commit `4d68aa6` já tinha trocado isso pela tabela `legal_documents` do PRD, mas servia o fragmento cru, sem página, com o inglês como padrão.

## 2. Decisão

**O texto mora no banco.** Os seis fragmentos (`<kind>.<locale>.html`) vivem no repositório privado `logn-conteudo/legal/`. Um script de lá, `gen_documentos_legais.py`, gera a migração que os insere em `legal_documents`, com versão, data de vigência e `material` em colunas. A migração nunca é escrita à mão.

**Uma página, dois modos**, montados pelo pacote `backend/internal/legal`:

- `GET /legal/{terms,privacy}`: a página do navegador. Tem título, linha de versão e vigência gerada das colunas, tema claro e escuro e troca de língua. É a URL da loja.
- `?embed=1`: o modo do app. Usa o CSS escuro em `surfaceRaised`, sem título e sem data, porque a tela nativa mostra os dois no cabeçalho.
- `?highlight=id1,id2` marca seções como novas nos dois modos. É o que a futura folha de aceite pendente vai usar.
- A língua vem de `?lang=`, depois do `Accept-Language`, e por fim do português, que é a versão que prevalece.

O fragmento é relido por `golang.org/x/net/html`, validado contra uma lista de tags, atributos e links permitidos, e reescrito pelo parser. A linha de meta sai do fragmento, e o link para o outro documento ganha a língua. Nada do pedido volta na página, e a CSP libera só o `<style>` servido, pelo hash dele.

Versão e vigência seguem para o app nos cabeçalhos `X-LogN-Legal-Version` e `X-LogN-Legal-Effective`. A tela do iOS (`LegalDocumentView`) é só shell. É navegação com conteúdo estático, e HTML não cabe no `ViewModel`. Sem rede, ela lê a cópia que `just legal-bundle` deixa no bundle.

**O cadastro aceita a versão vigente.** `GET /api/v1/legal/current` diz quais são. O Core busca essa rota quando o cadastro abre e monta o aceite na língua do app. O servidor recusa lista vazia (400) e versão diferente da vigente (409).

**Rascunho tem três guardas.**

1. `just legal-check` valida a estrutura dos seis arquivos. Com `release`, recusa também os marcadores `[A CONFIRMAR]`, `[TO CONFIRM]` e `[POR CONFIRMAR]`.
2. `just migrate-prod` não publica migração com marcador, a não ser com `LEGAL_ALLOW_DRAFT=true`.
3. No Cloud Run, documento com marcador responde 503. Com `LEGAL_ALLOW_DRAFT=true`, sai com uma faixa de rascunho, que é o caso do TestFlight durante a revisão do advogado.

## 3. Alternativas descartadas

* **HTML embutido com `go:embed` (o Plano A).** A página nunca cairia com o banco. Mas o texto teria duas fontes, e a versão, duas verdades: a constante em Go e o banco que `pending` e `accept` consultam.
* **Copiar os fragmentos para o backend no build.** Quem clona o repositório público não compilaria sem um arquivo de mentira, e a versão continuaria precisando do banco.
* **Esconder título e data com CSS no modo embed.** Deixaria texto duplicado no DOM, lido pelo VoiceOver, e não resolveria o selo de seção nova.
* **Passar a tela pelo Core.** Não há regra de negócio nela, e passar por Core exigiria codegen e FFI só para abrir uma página.

## 4. Consequências

* A URL da loja depende do banco. Com o banco fora do ar, o app inteiro já caiu, e o risco de o banco ficar sem backup some com o uso diário.
* Uma versão publicada não se edita. `--substitui` troca o texto de uma versão que ninguém aceitou, como o rascunho em revisão, e a migração gerada falha se já houver aceite.
* Os textos que o servidor escreve em volta do documento (linha de meta, selo, faixa) ficam num mapa em Go, `internal/legal/strings.go`. É uma exceção à regra 6 do AGENTS.md: o catálogo em `i18n/` está fora do contexto de build do Docker.
* As páginas usam a fonte do sistema. O embed cita IBM Plex primeiro, e ela aparece porque o bundle já registra as fontes.
* A folha de aceite pendente, com `pending`, `accept` e o destaque das seções, fica para o ciclo das trilhas pagas. O que ela precisa (`highlight`, `onAccept` na tela, versões nos cabeçalhos) já existe.
