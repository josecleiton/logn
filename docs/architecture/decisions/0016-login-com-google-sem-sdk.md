# ADR 0016: Login com Google, sem SDK

## 1. Visão Geral

A ADR 0005 previa GitHub, Google, Apple e e-mail, com o app obtendo o token do provedor e o Go validando. Até aqui só o e-mail existia: os três botões da tela de login tinham `action: {}`. Esta ADR fecha o Google: rota, tabela, fluxo no Core e no iOS, e o que fazer quando o e-mail do Google já tem conta com senha.

## 2. Decisão

**O iOS fala OAuth 2.0 direto, sem o SDK GoogleSignIn.** Código de autorização com PKCE (S256), `state` e `nonce`, numa `ASWebAuthenticationSession`, e a troca do código pelo ID token no próprio aparelho: client de iOS não tem secret (`ios/LogNiOS/LogNiOS/Core/GoogleAuth.swift`). O retorno é o esquema do Client ID ao contrário (`com.googleusercontent.apps.<id>:/oauthredirect`), que a sessão intercepta sem entrada no `Info.plist`. Os endereços vêm do discovery document (`accounts.google.com/.well-known/openid-configuration`), lido uma vez por execução do app. Cada um só é aceito em HTTPS e no host de hoje (`accounts.google.com` para a autorização, `oauth2.googleapis.com` para o token): o discovery pode mudar o caminho sem versão nova, mas um documento adulterado não manda o código e o verifier a outro servidor. Sem resposta, valem os endereços conhecidos. Um login por vez: toque repetido enquanto um está aberto é ignorado. O escopo é só `openid email`, desde a ADR 0017, que também faz a exclusão revogar o acesso no Google.

**O shell obtém o token; o servidor decide.** O shell manda ao Core `SocialLogin { provider, id_token, nonce }`, como a StoreKit manda o JWS da compra. O Core chama `POST /api/v1/auth/social` e trata a resposta. O Go (`internal/infrastructure/socialauth`) confere, nesta ordem:

- tamanho do token e do nonce (nonce de 32 a 128 caracteres);
- algoritmo **RS256 fixo**, lido do cabeçalho antes da validação: `idtoken.Validate` também aceita ES256 contra as chaves do IAP, que não emitem login;
- assinatura contra as chaves públicas do Google, `aud` igual ao Client ID e `exp`, por `idtoken.Validate`, com 5 s de limite para buscar as chaves;
- `iss` em `accounts.google.com` ou `https://accounts.google.com`, `sub` presente, `azp` igual ao Client ID quando vier, `iat` não mais que um minuto no futuro;
- `nonce` do token igual ao SHA-256 (hex) do nonce cru que o app manda, em tempo constante. O pedido ao Google leva o hash; o cru nunca sai do aparelho a não ser nesse pedido, por TLS.

Audiência vazia é recusada na construção do verificador: `idtoken.Validate` pula a checagem de `aud` quando ela vem vazia.

**A conta é achada pelo `sub`, nunca pelo e-mail.** `user_identities (provider, subject, user_id)`, chave primária em `(provider, subject)` (migração 0055). O e-mail do Google pode mudar; o `sub` não.

**Na primeira vez, e-mail verificado liga à conta que já existe.** Sem identidade ligada, o servidor exige `email_verified` e procura conta com o mesmo e-mail normalizado. Achando, liga a identidade e entra. Todo cadastro por e-mail passa por OTP, então toda conta existente teve o e-mail provado: não há como alguém criar antes, com senha, a conta de um e-mail que não é dele. Dois primeiros logins simultâneos do mesmo `sub` correm para o mesmo INSERT; o que perde lê o dono gravado pelo outro e entra nele.

**Sem conta, o cadastro pede o mesmo que o por e-mail, menos OTP e senha.** A primeira resposta é `409 signup_required`. O app mostra a tela de idade e termos, e o Core reenvia o mesmo token com `age_confirmed`, `country` e `legal_acceptances`, conferidos pelas mesmas regras do `/register`. A conta nasce sem senha (`password_hash` NULL) e com a identidade, numa transação.

**Conta sem senha exclui com login novo no Google.** `/users/me/delete` aceita `provider`, `id_token` e `nonce` no lugar da senha. O `sub` tem de ser uma identidade desta conta, e o token tem de ter sido emitido há no máximo 5 minutos: excluir pede a pessoa na frente da tela, não um token que sobrou. Conta sem senha pode ganhar uma pelo "Esqueci a senha": o código do e-mail prova o mesmo que o Google provou.

**Configuração.** O Client ID chega ao servidor por `GOOGLE_IOS_CLIENT_ID` (Cloud Run, pela variável `google_ios_client_id` do `terraform.tfvars`) e ao app pelo `.env` → `Local.xcconfig` → `LogNGoogleClientID` no `Info.plist`. Não é segredo, mas identifica o projeto e fica fora do git como o resto. Sem ele, o servidor sobe com o Google desligado e responde `503 provider_disabled`; o app esconde o botão.

**Desligar sem versão nova.** O botão só aparece com o Client ID configurado **e** a flag `sso_google_enabled` do PostHog ligada. A tela de login vem depois do `reset` do PostHog no logout, então a flag não pode depender de id de pessoa: o filtro, se houver, é por propriedade do aparelho ou do app (versão, build). Se o login quebrar do lado do Google, desligar a flag tira o botão na hora, e o e-mail segue funcionando enquanto a correção passa pela revisão da Apple. O que decide se um token vale mora no servidor, que muda em minutos.

**Códigos novos** (`api_errors.go`): `provider_disabled`, `social_token_invalid`, `social_email_unverified`, `signup_required`. A resposta de sessão ganha `email`, que o Core guarda como e-mail da conta: no login pelo Google o app não digitou nenhum.

## 3. Alternativas descartadas

- **SDK GoogleSignIn (10.0.0).** Menos código nosso, mas cinco pacotes a mais na cadeia (AppAuth, GTMAppAuth, GTMSessionFetcher, AppCheck, GoogleUtilities) num repositório público, atualização a cada iOS e Xcode, e endereços fixos no binário. Não poupa hotfix: se o Google mudar o protocolo, a correção também é versão nova do app. O SDK fala o mesmo OAuth com PKCE que este código.
- **Vincular por e-mail só depois de um OTP.** Fecha o caso do domínio próprio que expirou e foi comprado por outro. Descartado por decisão de produto: custa um passo a quem já tinha conta, e com Gmail o risco não existe.
- **Recusar e-mail que já tem conta.** Quem já tinha conta nunca entraria com o Google.
- **Abortar o servidor sem `GOOGLE_IOS_CLIENT_ID`.** O Client ID não é segredo, e derrubar o deploy por ele tiraria do ar também o login por e-mail.

## 4. Consequências e risco conhecido

- **O nonce não é de uso único no servidor.** Um par (token, nonce cru) vale até o `exp` do token, uma hora. Usar exige o nonce cru, que só existe na memória do app e no corpo do pedido por TLS: quem o tem já controla o aparelho. Fechar isso pede uma tabela de nonces usados com expurgo; fica para quando houver motivo.
- **Domínio próprio que expira.** Com o vínculo automático, quem comprar um domínio expirado e criar no Google uma conta com o endereço antigo entra na conta do LogN daquele endereço. Com Gmail não acontece.
- **Pontos no Gmail.** O Gmail ignora pontos no nome, e o Google devolve a forma canônica. Um endereço cadastrado com senha na forma com ponto não bate com o que o token traz: sai uma segunda conta, sem senha. Não é invasão, é conta duplicada.
- **Um Client ID por vez.** Hoje o client de teste é do bundle de desenvolvimento, `sh.logn.LogNiOS`. Quando `sh.logn.app` entrar, sai outro client de iOS e troca-se o valor nos dois lados; o código não muda.
- A tela de exclusão mostra "Confirmar com o Google" a toda conta quando o app tem Client ID. Para conta com senha e sem Google ligado, o servidor responde credencial inválida.
