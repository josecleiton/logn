# ADR 0019: Login com GitHub

## 1. Visão Geral

A ADR 0005 previa GitHub, Google, Apple e e-mail. A 0016 fechou o Google e a 0017 a Apple; o botão do GitHub está na tela de login desde então, atrás de uma flag e sem ação. Esta ADR fecha o GitHub e registra o que ele muda em relação aos outros dois.

O que muda é o protocolo. O GitHub fala OAuth 2.0, não OpenID Connect: não há ID token, nem nonce, nem `email_verified` num token assinado. A identidade sai da API (`/user` e `/user/emails`) com um access token. E a troca do código pelo token **exige o `client_secret`**, mesmo com PKCE: um client de iOS não troca o código sozinho, como faz no Google.

## 2. Decisão

**O aparelho obtém o código; o servidor troca.** O iOS abre `github.com/login/oauth/authorize` numa `ASWebAuthenticationSession`, com PKCE (S256), `state` e escopo só `user:email`, e intercepta o retorno em `logn://oauth/github` (`GitHubAuth.swift`). O Core manda o código e o verifier a `POST /api/v1/auth/github/exchange`, junto com o SHA-256 de um nonce que o aparelho gerou. O servidor, com o secret:

1. troca o código em `github.com/login/oauth/access_token`, mandando o mesmo `redirect_uri` fixo;
2. lê `GET /user` (o id numérico) e `GET /user/emails` (o e-mail marcado `primary`, e se está `verified`), com 5 s de limite em cada chamada;
3. apaga o access token no GitHub (`DELETE /applications/{client_id}/token`), para não sobrar token vivo em lugar nenhum;
4. devolve um **bilhete**: um JWT HS256 assinado pelo servidor, com `sub` (o id do GitHub), `email`, `email_verified`, `nonce` (o hash recebido), `iat` e `exp` de 10 minutos.

**O bilhete faz o papel do ID token.** Dali em diante o GitHub entra pelo caminho da 0016 sem mudança nas rotas: o Core chama `POST /api/v1/auth/social` com `provider = "github"`, `id_token` = bilhete e o nonce cru. O verificador do GitHub (`internal/infrastructure/socialauth/github.go`) confere o bilhete como os outros conferem o token do provedor: tamanho, HS256 fixo, assinatura, `aud`, `iss`, `exp`, `iat` sem futuro e nonce em tempo constante. O `signup_required` volta com o mesmo bilhete, como volta com o mesmo ID token: o código do GitHub já foi gasto na troca, e é por isso que existe bilhete e não "manda o código de novo".

**A chave do bilhete não é a da sessão.** Ela é derivada de `JWT_SECRET` por HMAC-SHA256 com um rótulo próprio (`logn/github-ticket/v1`). Um bilhete não passa como token de sessão, e um token de sessão não passa como bilhete: as chaves são outras, além de `aud` e `iss` diferentes.

**A conta é achada pelo id numérico, nunca pelo login nem pelo e-mail.** `user_identities` passa a aceitar `github` (migração 0058). O login do GitHub (`octocat`) muda quando a pessoa quer; o id não.

**Vínculo e cadastro seguem a 0016.** Na primeira vez, o e-mail principal verificado liga à conta que já existe com ele; sem conta, o cadastro pede idade e termos. E-mail principal não verificado responde `social_email_unverified`. Só o principal conta: o GitHub guarda vários endereços por conta, e ligar por um secundário daria a quem adicionou um endereço a entrada numa conta que não é dela.

**Excluir conta pede login novo no GitHub, e revoga o acesso.** A troca aceita `purpose = "delete"`, e só com sessão: sem ela, a troca de exclusão só serviria para tirar do GitHub um token vivo. Nesse caso o servidor **não** apaga o access token no passo 3 e o devolve junto do bilhete. O app o guarda só na memória e o manda como `authorization_code` em `/users/me/delete`, que já tem esse campo desde a 0017. O bilhete prova a pessoa na frente da tela: o código do GitHub vale 10 minutos e é de uso único, e o bilhete tem de ter no máximo 5 minutos (`deleteReauthMaxAge`). Depois de a conta estar desativada, o `GitHubRevoker` confere o token em `POST /applications/{client_id}/token`, que diz de que app e de que usuário ele é, e só se o usuário for o `sub` que provou a posse chama `DELETE /applications/{client_id}/grant`. Isso tira o LogN da lista de apps autorizados da pessoa. Revogação que falha vai para o log, como na Apple.

**Exclusão recusada apaga o token.** Se a exclusão para depois de o token sair do GitHub e antes de revogar (bilhete de outra conta, bilhete de mais de 5 minutos, conta que também tem Apple e cai no `provider_reauth_required`), o servidor apaga aquele access token no GitHub (`DELETE /applications/{client_id}/token`), sem mexer na autorização. O token não expira sozinho, e o caso da Apple é determinístico, não raro.

**A revogação é garantia de esforço, não trava.** O que prova a pessoa na frente da tela é o bilhete fresco; o `authorization_code` só alimenta a revogação. Um token que não confere (de outro usuário, de outro app, vazio de propósito) faz a revogação falhar no log, e a conta sai do mesmo jeito, como sai quando o GitHub está fora do ar.

**O GitHub não entra em `revocationRequired`.** A lista é de quem *exige* revogar, e o GitHub não exige. Uma conta com GitHub ligado que exclui pela senha sai sem revogar, como a do Google.

**Configuração.** `GITHUB_CLIENT_ID` é variável comum (Cloud Run pela `github_client_id` do `terraform.tfvars`; app pelo `.env` → `Local.xcconfig` → `LogNGitHubClientID`). O secret vem no segredo agrupado (abaixo). Sem os dois, o GitHub fica desligado e a rota responde `provider_disabled`. Com um só, o servidor aborta em produção, como a Apple com parte das variáveis. O botão aparece só com o Client ID no app **e** a flag `sso_github_enabled` do PostHog, pelo mesmo motivo da 0016.

**Rate limit e corpo.** A troca passa pelo `authLimiter` e pelo `authBodyLimit`, como as outras rotas de login. O código e o verifier têm tamanho conferido antes de qualquer chamada ao GitHub, e um erro do GitHub vira `social_token_invalid` para o app, com o motivo só no log.

### Os segredos do servidor num segredo só

O secret do GitHub seria a 7ª versão ativa do Secret Manager, uma além do que o free tier cobre. As chaves que o servidor gera e que ninguém mais lê passam a morar num segredo JSON, `logn-server-keys`, exposto ao Cloud Run como `SERVER_KEYS`:

```json
{ "jwt_secret": "…", "track_key_secret": "…", "github_client_secret": "…" }
```

- `logn-jwt-secret` e `logn-track-key-secret` saem, e a conta fica em 5.
- `logn-origin-shared-secret` **não entra**: ele é gerado pelo Terraform (`random_password`) e vai também para a Cloudflare. Juntá-lo às chaves manuais obrigaria o Terraform a ver as outras.
- O servidor lê cada chave de `SERVER_KEYS` e, na falta dele, da variável solta de sempre (`JWT_SECRET`, `TRACK_KEY_SECRET`, `GITHUB_CLIENT_SECRET`). As duas fontes para a mesma chave, com valores diferentes, abortam, e campo desconhecido no JSON também. A trava de produção continua por chave: sem `jwt_secret` ou `track_key_secret`, de onde for, o servidor não sobe no Cloud Run. O `jwt_secret` ganha a trava que a track key já tinha: em produção, menos de 32 bytes, a chave de desenvolvimento ou o `…` do exemplo acima abortam. Com o código público, uma chave fraca deixa qualquer um forjar sessão e bilhete.
- O segredo novo entra nos três lugares da regra 9: `google_secret_manager_secret`, o mapa `runtime_secrets` e a referência no `cloud_run.tf`.
- O `just sync-env` deixa de copiar `*_SECRET` para o `Local.xcconfig`: config de build aparece em log do `xcodebuild`, e um `${...}` no `Info.plist` embarcaria o valor no app.

A troca, uma vez, na ordem:

1. deploy do código que lê as duas fontes;
2. `terraform apply -target=google_secret_manager_secret.server_keys -target='google_secret_manager_secret_iam_member.runtime_reads["server_keys"]'`: cria só o segredo e o acesso a ele. O apply inteiro já trocaria as variáveis do Cloud Run para um segredo sem versão, e a revisão nova não subiria;
3. subir a versão de `logn-server-keys` com os valores **atuais** de JWT e track key (trocar o JWT derruba todas as sessões; trocar a track key tranca as trilhas compradas) e o secret do GitHub. `github_client_secret` só entra junto de `github_client_id` no `tfvars`: um sem o outro, o servidor não sobe;
4. `terraform plan` e `apply` inteiro, que troca as variáveis do Cloud Run pelo `SERVER_KEYS`;
5. com a revisão nova servindo, destruir as versões dos dois segredos antigos (`gcloud secrets versions destroy`) e tirá-los do Terraform, junto com as entradas deles em `runtime_secrets`, que continuam até aqui porque a revisão antiga os lê a cada instância que sobe.

Entre o 3 e o 5 são 7 versões ativas por alguns minutos.

## 3. Alternativas descartadas

- **Trocar o código no aparelho, sem secret.** O GitHub não aceita a troca sem `client_secret`, nem com PKCE.
- **Pôr o secret no app.** Segredo em binário distribuído não é segredo; qualquer um extrairia e se passaria pelo LogN no GitHub.
- **Mandar o código direto a `/auth/social`.** O código é de uso único: o `signup_required` gastaria o código na primeira tentativa, e a segunda, com idade e termos, não teria o que trocar.
- **Devolver o access token ao app no login e conferi-lo a cada pedido.** O token do GitHub não expira por padrão; ele ficaria vivo no aparelho sem motivo. O bilhete vence em 10 minutos e não abre nada fora do LogN.
- **Device flow do GitHub.** Evita o redirect, mas faz a pessoa copiar um código de oito letras para outra tela. É para TV, não para celular.
- **Ligar pela lista inteira de e-mails verificados.** Ver o vínculo e o cadastro acima.
- **Um segredo por chave, pagando a 7ª versão.** Custo pequeno, mas o requisito é o free tier, e a 8ª (a `.p8` da Apple) viria logo depois.

## 4. Consequências e risco conhecido

- **O bilhete não é de uso único.** Dentro dos 10 minutos, quem tiver o bilhete e o nonce cru entra, como com o par (ID token, nonce) da 0016. Os dois só existem na memória do app e no corpo do pedido, por TLS.
- **O access token da exclusão passa pelo aparelho.** Ele é da própria pessoa, vive só na memória, e o servidor o revoga junto com a autorização, ou o apaga se a exclusão for recusada. Sobra vivo só se o pedido de exclusão nunca chegar ao servidor (app fechado entre a troca e a exclusão), até a pessoa sair do LogN nas configurações do GitHub.
- **Uma troca do GitHub por vez no Core.** Login e exclusão dividem o lugar do nonce; uma troca que chega com outra em voo é ignorada.
- **Domínio próprio que expira** vale para o GitHub como para o Google (0016, seção 4).
- **E-mail privado do GitHub.** Esconder o e-mail no perfil não o tira de `/user/emails`, que o escopo `user:email` lê; o principal verificado continua sendo o que conta.
- **A troca de segredo derruba tudo se a cópia sair errada.** Um JWT diferente encerra as sessões de todos os aparelhos; uma track key diferente faz as trilhas compradas pararem de abrir até corrigir. O passo 2 copia os valores das versões atuais com `gcloud secrets versions access`, nunca à mão.
- **Sem teste de ponta a ponta automatizado.** O verificador, a troca e a revogação têm teste contra um GitHub falso; o fluxo real passa pelo roteiro do simulador.
