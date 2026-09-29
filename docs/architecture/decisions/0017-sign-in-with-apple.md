# ADR 0017: Sign in with Apple

## 1. Visão Geral

A diretriz 4.8 da App Store pede que app com login de terceiro (o Google, ADR 0016) ofereça também o Sign in with Apple. E a Apple exige de quem usa o Sign in with Apple que a exclusão de conta revogue o acesso pela REST API dela. Esta ADR segue o desenho da 0016 e registra o que a Apple muda.

## 2. Decisão

**Mesmo fluxo do Google, com `provider = "apple"`.** O iOS usa `ASAuthorizationAppleIDProvider` (`AppleAuth.swift`), com o SHA-256 do nonce no pedido e só o escopo de e-mail: o app não guarda nome. O Core manda o ID token e o nonce cru a `POST /api/v1/auth/social`. A regra de vínculo é a da 0016: `sub` primeiro, e na primeira vez o e-mail verificado liga à conta existente. Com o "Ocultar meu e-mail", o endereço é um relay (`privaterelay.appleid.com`), que não bate com conta nenhuma: sai uma conta nova.

**O servidor confere o token com as chaves públicas da Apple** (`appleid.apple.com/auth/keys`), sem lib nova: `golang-jwt`, que o projeto já usa, com RS256 fixo, `iss` igual a `https://appleid.apple.com`, `aud` igual ao bundle (`APPLE_BUNDLE_ID`), `exp` obrigatório, `iat` com um minuto de folga e o nonce em tempo constante. As chaves ficam em cache por uma hora; `kid` desconhecido força uma busca nova, no máximo uma por minuto, para token forjado não virar enxurrada de pedidos à Apple. A busca roda fora do lock e uma por vez, e quem chega durante ela espera por ela. Com a Apple fora do ar, a chave vencida vale por até 24 horas, e depois não.

**A exclusão revoga o acesso na Apple sem guardar token nenhum.** A exclusão já pede um login novo no provedor (0016). O da Apple traz um `authorization_code` de uso único, válido por cinco minutos, que o app manda junto. Depois de desativar a conta, o servidor troca o código pelo refresh token em `/auth/token` e o revoga em `/auth/revoke`. O `client_secret` é um JWT ES256 assinado com a chave `.p8` do Sign in with Apple, com cinco minutos de validade. Sem o código, a exclusão pela Apple é recusada (`400`): a conta sairia ainda autorizada. O código tem de ser do `sub` que provou a posse: o `id_token` que a troca devolve diz de quem ele é, e se não bater nada é revogado. A revogação roda num contexto que não morre com o pedido, porque o código é de uso único.

**A Apple fora do ar não segura a exclusão.** A revogação roda depois de a conta estar desativada; se falhar, vai para o log, para revogar à mão.

**Conta com Apple vinculada exclui pela Apple.** Pela senha ou pelo Google não há código para revogar, então o servidor responde `409 provider_reauth_required`, e a tela oferece "Confirmar com a Apple". Conta sem Apple continua saindo pela senha ou pelo Google. A lista de quem exige revogação é fixa no código, não o que estiver ligado: com a Apple desligada depois de haver contas nela, a exclusão sai pela senha, para ninguém ficar sem saída, e o log avisa que falta revogar à mão.

**Desligado até existir a conta paga.** A chave `.p8` e a capability só existem no Apple Developer pago.
- Servidor: a Apple liga com `APPLE_SIGNIN_TEAM_ID`, `APPLE_SIGNIN_KEY_ID` e `APPLE_SIGNIN_PRIVATE_KEY` juntas. Sem nenhuma, responde `provider_disabled`; com uma parte só, aborta em produção, porque é deploy pela metade. A chave vem do segredo `logn-apple-signin-key`, nos três lugares da regra 9, e o Cloud Run só a referencia com `enable_apple_signin = true`.
- App: `LOGN_CODE_SIGN_ENTITLEMENTS` no `.env` aponta para `LogNiOS/LogNiOS.entitlements`. Vazio, o build sai sem a capability, que o time gratuito não assina, e sem o botão. O botão também espera a flag `sso_apple_enabled` do PostHog.

**Google, na mesma leva.** O escopo do Google passa a ser só `openid email` (nome e foto não servem ao servidor). E a exclusão confirmada pelo Google também tira o LogN dos apps com acesso à conta: o aparelho guarda o access token do login da exclusão só na memória e chama `oauth2.googleapis.com/revoke` quando a exclusão dá certo. O Google não exige; é para a conta apagada não seguir listada lá. No login comum não se revoga, porque apagaria o consentimento e o Google pediria de novo a cada entrada.

## 3. Alternativas descartadas

- **Guardar o refresh token da Apple no cadastro, para revogar depois.** Seria mais um segredo por conta no banco, a troca do código já no login e uma chave para cifrá-los. O login novo na exclusão já traz um código fresco.
- **Revogar antes de desativar e recusar a exclusão se a Apple falhar.** Seguraria o que o jogador pediu por causa de uma falha fora do nosso controle, e a conta sai de qualquer jeito em 30 dias.
- **Botão `SignInWithAppleButton` do sistema.** A diretriz aceita botão próprio que siga as regras de cor e texto; o design system já tem o branco com o logo.

## 4. Consequências e risco conhecido

- **Sem teste de ponta a ponta até a conta paga.** O verificador e a revogação têm teste com chaves e servidor falsos; o fluxo real (folha da Apple, entitlement, `/auth/token` com `.p8` de verdade) só roda com o time pago.
- **Revogação que falhou fica só no log.** Não há fila para tentar de novo; quem olha o log revoga à mão, com o código já vencido. Se virar frequente, entra uma fila.
- **Notificações servidor a servidor da Apple** (quem revoga o acesso pelos Ajustes do iPhone, ou apaga o Apple ID) não estão ligadas. A Apple recomenda, não exige; a conta continua existindo e entra por senha ou por outro login.
- **Um bundle por vez.** O `aud` é o `APPLE_BUNDLE_ID`, o mesmo da StoreKit (`sh.logn.app`). Build de desenvolvimento com outro bundle não entra pela Apple contra este servidor.
