# ADR 0027: SMTP com go-mail, prazo pelo ctx e TLS obrigatório em produção

**Status:** Aceito
**Data:** 7 de outubro de 2026
**Substitui:** ADR 0002

## 1. Contexto

O backend mandava e-mail com `gopkg.in/gomail.v2`, escolhido na ADR 0002 por montar
`multipart/related` com imagens inline por CID. As imagens saíram dos templates depois, e
a razão da escolha saiu com elas. Ficaram os problemas:

- **Sem prazo depois de conectar.** gomail só limita o tempo da conexão. Ler e escrever
  não têm prazo, e não há como passar `context`. A ADR 0026 contornou com uma goroutine
  abandonada depois de 20 s: o envio continuava rodando por baixo, e podia chegar depois
  de contado como falha.
- **STARTTLS oportunista.** Fora da 465, gomail só cifra se o servidor oferecer. Quem
  está no meio tira a oferta, e o OTP passa em claro. Se o servidor anunciar `AUTH LOGIN`
  sem `PLAIN`, a senha também. Produção usa a 465 desde o começo, mas o default do
  Terraform era 587 até o commit que o trocou.
- **Projeto parado.** Sem release desde 2016. Não há CVE aberto, mas se aparecer um, não
  sai correção.

O aviso de licença (ADR 0021) mandava e-mail síncrono, sem prazo nenhum.

## 2. Decisão

**Biblioteca:** `github.com/wneessen/go-mail` v0.8.1, MIT. Mantida, com release recente,
OpenSSF Scorecard 8,7 e um advisory já corrigido (GO-2025-3988, na 0.7.1). Depende só de
`golang.org/x/crypto` e `golang.org/x/text`, que o backend já usava nas mesmas versões: a
troca não traz dependência transitiva nova ao binário.

**Prazo pelo ctx.** Os quatro envios (`SendOTP`, `SendWelcome`,
`SendWaitlistConfirmation`, `SendLicenseNotice`) recebem `context`. A biblioteca só olha o
ctx para conectar e renova o prazo de cada passo, então o Mailer abre a conexão TCP ele
mesmo (`WithDialContextFunc`) e a fecha quando o ctx acaba. Um `DATA` sem resposta cai na
hora. A caixa de saída e o aviso de licença usam o mesmo `smtpSendTimeout`, 20 s, e o erro
leva `context.DeadlineExceeded` na cadeia.

**TLS pela porta.** Na 465, TLS implícito, com o handshake feito no dial. Em qualquer
outra porta, STARTTLS: obrigatório em produção (`K_SERVICE` setado), oportunista fora
dela, porque o Mailpit do desenvolvimento não fala TLS. Nos dois casos o certificado é
conferido contra as raízes do sistema, com o nome de `SMTP_HOST`. O login é `PLAIN` fixo,
sem a escolha automática da biblioteca. É uma segunda trava: a biblioteca recusa PLAIN em
conexão sem TLS, exceto com localhost, mas em produção a política acima já cifrou a
conexão antes do login. Resend anuncia `AUTH PLAIN LOGIN` na 465.

O aviso de licença usa o prazo sem o cancelamento do pedido
(`context.WithoutCancel`): a mudança já está gravada quando o e-mail sai, e quem chama
desconectar não deve derrubar o aviso no meio.

**Configuração.** Em produção, faltar `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER` ou `SMTP_PASS`
aborta o boot. Antes caía em `localhost:1025` e o OTP sumia sem alarme. `SMTP_PORT` que
não é número é erro em qualquer ambiente.

**Cabeçalhos.** `Date` e `Message-ID` explícitos, este no domínio do remetente. gomail não
mandava `Message-ID`: quem gerava era o servidor. `User-Agent` e `X-Mailer` desligados,
para o e-mail não dizer qual biblioteca e qual versão o mandou. HELO fixo em `localhost`,
como era.

**Erro sem endereço.** O Mailer passa todo erro por `RedactAddresses` antes de devolver.
O SMTP repete o destinatário na recusa, e go-mail também (`SendError` lista os
destinatários afetados). O log do aviso de licença gravava o erro cru.

## 3. Alternativa recusada

**API HTTP de Resend**, com `net/http`. Seria a única que acaba com o e-mail duplicado,
pela chave de idempotência, que SMTP não tem. Ficou de fora para manter um caminho só
entre desenvolvimento (Mailpit) e produção, e para não amarrar o envio a um provedor.
Se a duplicação virar problema, é ela que resolve.

## 4. Consequências

- O e-mail ainda pode sair duas vezes: se o servidor aceitar a mensagem e o prazo acabar
  antes da resposta, ou se o processo morrer antes de marcar a linha (ADR 0026). A janela
  ficou menor, porque o envio não segue rodando depois do prazo.
- go-mail ainda é v0.x. Subir de versão pode quebrar a API.
- Testes em `smtp_test.go`, contra um servidor SMTP falso com certificado gerado no
  teste: produção recusa servidor sem STARTTLS antes de `AUTH` e `DATA`; na 465, um
  certificado sem raiz conhecida derruba a conexão antes do `EHLO`, e um reconhecido leva
  o login cifrado sem STARTTLS; fora da 465, o login vem depois do STARTTLS; o ctx derruba
  um envio parado depois do `DATA`; o erro sai sem endereço; produção sem variável SMTP
  não sobe.
