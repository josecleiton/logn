# ADR 0028: Botão dos e-mails de código por App Link em logn.sh/app

**Status:** Aceito
**Data:** 7 de outubro de 2026
**Substitui:** itens 2 e 3 da ADR 0003

## 1. Contexto

A ADR 0003 pôs no botão dos e-mails de código o esquema próprio do app:
`logn://verify?code=…&email=…&purpose=…` e `logn://reset-password?…`. Três problemas:

- **O Gmail tira link de esquema próprio.** O botão chega sem destino, e a pessoa só tem
  o código digitado. Era o cliente de e-mail de quase todo mundo.
- **Qualquer app pode registrar `logn://`.** No Android, o sistema pergunta qual abrir, ou
  entrega ao outro, e o OTP vai junto.
- **No computador o link não faz nada.** Quem lê o e-mail no navegador do desktop clica e
  nada acontece, sem dizer por quê.

O lançamento é no Android (ADR 0022), e `logn.sh/.well-known/assetlinks.json` já liga o
domínio ao pacote assinado pelo Play App Signing.

## 2. Decisão

**O botão aponta para a landing.** `https://logn.sh/app/verify` e
`https://logn.sh/app/reset-password`, com o prefixo de língua da landing (`/en/`, `/es/`;
pt-BR na raiz). O link sai pronto do mailer (`appLink`), e o template só o escreve.

**Código e e-mail no fragmento, não na query.**
`/app/verify#code=…&email=…&purpose=…`. O navegador não manda o fragmento ao servidor:
nem a borda da Cloudflare, nem o log do Worker, nem o Web Analytics da landing veem o
OTP quando o link abre no navegador.

**Android: App Link verificado.** Um `intent-filter` com `android:autoVerify="true"`,
`https`, host `logn.sh` e os seis caminhos (duas ações em três línguas). Com a
verificação feita, o sistema entrega o link só a este pacote, sem perguntar. A Activity
tira a ação do último trecho do caminho e os parâmetros do fragmento.

**Navegador: página de passagem.** Onde o App Link não abre o app (computador, iPhone,
build de debug, verificação que falhou), a landing serve `app/<ação>.html` em cada língua:
"Abra no celular" e um botão "Abrir no LogN", que o `app-link.js` monta para
`logn://<ação>?code=…&email=…&purpose=…` só com esses três parâmetros, e só se o
fragmento os trouxer. O script tira o fragmento da barra de endereço e do histórico.

**O Core decide se o link vale.** Os shells não verificam nem abrem tela com o link: mandam
`OpenOTPLink` ao Core. Ele aceita só com pedido de código aberto neste app (`RequestOTP`)
para o mesmo e-mail, sem diferença de caixa e espaço, e o mesmo propósito. Sem isso,
qualquer página com um link do código de outra conta punha o app nela: com App Link
verificado, o Android abre sem perguntar. Link recusado vira `StatusKey::CodeLinkIgnored`,
e a pessoa digita o código do e-mail. Aceito, o link fica em `ViewModel.otp_link` até o
pedido fechar ou ser refeito. No cadastro, o Core verifica o código, e a tela usa o código do
link no `Register`. Antes ela mandava o que tinha sido digitado, nada, e o cadastro pelo
link não fechava. Na redefinição, a tela de senha nova sobe com o código, como antes.

O pedido vive na memória do Core. Se o sistema matar o app entre pedir o código e tocar no
link, o link é recusado e a pessoa digita o código.

**`logn://` fica.** O Android e o iOS continuam aceitando `logn://verify` e
`logn://reset-password`: é para onde a página de passagem leva. O iOS não tem Universal
Link enquanto não houver conta Apple Developer, e no iPhone o caminho é sempre esse.

**O retorno do GitHub não muda.** Continua `logn://oauth/github` (ADR 0019). O OAuth App
do GitHub aceita uma URL de callback, e a `ASWebAuthenticationSession` do iOS só aceita
callback `https` com Associated Domains.

## 3. Consequências

- O botão volta a funcionar no Gmail, e no Android abre o app direto.
- O host está escrito em três lugares: `appLinkOrigin` no mailer, o `AndroidManifest.xml`
  e o `assetlinks.json`. Mudar o domínio é mudar os três.
- O build de debug é assinado com outra chave e não passa na verificação: o link abre a
  página de passagem, e o botão leva ao app pelo `logn://`.
- A página de passagem depende de JS. Sem ele, fica o texto e o código do e-mail.
- Quando houver conta Apple Developer, o mesmo caminho vira Universal Link com um
  `apple-app-site-association` ao lado do `assetlinks.json` e o iOS lendo o fragmento como
  o Android já lê.
