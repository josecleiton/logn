# ADR 0022: Android primeiro no lançamento público, e iPhone por lista de espera

## 1. Visão Geral

O plano até aqui era lançar no iPhone e trazer o Android depois (ADR 0001, spec das trilhas pagas). A spec pôs "Android e Play Billing" fora do escopo, e a compra, a landing e os documentos legais foram escritos só para App Store.

Esse plano pressupunha a conta paga do Apple Developer Program, que custa US$ 99 por ano, e essa conta nunca foi criada. Sem ela não há TestFlight, App Store, Sign in with Apple nem APNs. Compra só existe no `.storekit` local, e a assinatura do Xcode vence a cada 7 dias. Documentos que dizem que o app "estava no TestFlight" (ADRs 0008, 0013 e 0020) descrevem uma distribuição que não aconteceu: o app só rodou no aparelho do dono, pelo Xcode.

A conta do Google Play Console já existe e é de organização, verificada no CNPJ que aparece nos termos. Por ser de organização, não se aplica a regra de teste fechado com 12 testadores por 14 dias, que vale para conta pessoal nova. A taxa é única, de US$ 25.

## 2. Decisão

**O lançamento público é no Google Play.** O motivo é custo: não pagar US$ 99 por ano para validar a ideia. O iOS continua no repositório e roda no aparelho do dono. O código de Apple (StoreKit, Sign in with Apple, a validação do JWS) fica como está, sem uso em produção, e nada dele é apagado.

**A conta da Apple entra quando um de dois gatilhos disparar**, o que vier primeiro:

- 100 inscrições **confirmadas** na lista de espera do iPhone;
- receita do Android que cubra US$ 99 por ano.

**A landing** (`landing/README.md`):

- Os botões apontam para o Google Play (`LOGN_PLAY_STORE_URL`). A App Store é opcional (`LOGN_APP_STORE_URL`) e está desligada. Cada selo só entra quando a loja tem link.
- O texto diz "Android · grátis · iPhone a caminho".
- **Lista de espera só do iPhone**, porque o Android vai sair antes. É um `<form>` sem JS que posta em `POST /api/v1/waitlist`, e o backend responde com `303` para `/waitlist/{thanks,confirmed,left,error}/`. O `form-action` da CSP aceita o domínio da API e nenhum outro. O formulário só sai com `LOGN_API_ORIGIN`, e isso só se define depois que a rota existir e a política cobrir a lista.
- `/account/delete/` é a URL de exclusão que a ficha do Google Play exige em "Segurança dos dados". Uma URL serve para todas as línguas, e a página tem o seletor. Ela repete a seção 10 da política: pelo app, na hora; sem o app, por `contact@logn.sh` a partir do e-mail cadastrado.

**A lista de espera, no backend:**

- Tabela `waitlist_entries` (`email`, `locale`, `confirmed_at`, `created_at`, `updated_at` com trigger), no Postgres. Não fica no worker nem com terceiro, para o dado pessoal ficar no mesmo lugar e na mesma purga.
- Confirmação dupla: link com token HMAC, e o não confirmado é purgado 7 dias depois da criação. A resposta é a mesma para e-mail novo, pendente e confirmado, para a rota não denunciar quem está na lista.
- **A pendente recebe um e-mail só.** Pedir de novo não reenvia, a não ser que o SMTP tenha falhado. Com reenvio, quem mandasse o formulário todo dia faria chegar um e-mail por dia a um endereço que não é dele, e a linha nunca venceria. Quem volta depois de a pendente vencer se inscreve de novo, então o pior caso é um e-mail a cada 7 dias por endereço.
- Contra abuso:
  - Só vale formulário da própria landing: `Origin` igual à landing, ou `Sec-Fetch-Site: same-site`. A landing sai com `no-referrer`, e com isso o navegador manda `Origin: null`. Sem essa checagem, qualquer página postaria o formulário pelo navegador de cada visitante, um IP por visitante.
  - Rate limit por IP, `limitBody`, e um teto global de 100 e-mails de confirmação por hora, contado no banco.
  - O campo-isca `website`, que robô preenche e gente não vê.
  - Sem Turnstile nem captcha: seria script de terceiro, e só entra se o spam aparecer, com ADR própria.
- **O link do e-mail não muda nada no GET.** Filtro de e-mail corporativo abre todo link que recebe. Um GET que confirmasse inscreveria quem não pediu, e um que tirasse da lista tiraria quem pediu. `GET /api/v1/waitlist/{confirm,leave}?t=` mostra uma página com um botão, com CSP por hash e `Referrer-Policy: no-referrer`, porque o token está na URL. O botão faz o `POST` na mesma URL, que responde com `303` para a landing.
- **O token é `id.HMAC(id, ação)`**, com a chave do JWT e um prefixo próprio. O e-mail nunca vai no link, porque a URL passa pelos registros da hospedagem, e eles não guardam e-mail. O token não vence sozinho: morre com a linha. Cada ação tem a sua assinatura, então o link de confirmar não tira ninguém da lista.
- Todo e-mail leva o link de saída, também no `List-Unsubscribe`, e a saída apaga a linha.
- Não mandamos `List-Unsubscribe-Post` (RFC 8058): o clique de saída num toque é um POST do servidor do provedor de e-mail, e o Bot Fight Mode da borda desafia servidor com JS, sem exceção (ADR 0013). A saída falharia calada. O cabeçalho volta com um caminho que não passe pelo desafio, e com DKIM assinando os dois cabeçalhos.
- O e-mail de confirmação sai fora do pedido, como o do OTP, para o tempo de resposta não dizer quem já está na lista. Se o SMTP falha, a goroutine libera o reenvio. O erro vai para o log sem endereço de e-mail, porque o SMTP costuma repetir o destinatário na recusa.
- `WAITLIST_LANDING_ORIGIN` e `WAITLIST_API_ORIGIN` ligam as rotas (`enable_waitlist` no Terraform). Sem as duas, as rotas não existem. Uma só, ou fora do formato `https://domínio`, e o servidor não sobe.
- O e-mail confirmado fica até o lançamento no iPhone, sem prazo fixo. A política diz isso.
- Um único aviso, quando o iPhone sair. O envio desse aviso fica para quando o gatilho disparar.

**A compra pelo Google Play**, ao lado da App Store:

- **Um id de produto nas duas lojas.** Os ids atuais são válidos no Play. Uma migração nova renomeia `tracks.app_store_product_id` para `store_product_id`, e `logn-conteudo` acompanha.
- **`entitlements` ganha `provider`**, e o índice de dono ativo passa a `(provider, original_transaction_id)`. O CHECK de `environment` em `store_transactions` passa a aceitar os ambientes do Play. As outras tabelas de compra e de revogação já aceitam `google_play` (migrações 0050 e 0063).
- **`original_transaction_id` do Play é o SHA-256 hex do `purchaseToken`.** O token não tem tamanho máximo documentado, e o `orderId` não vem em compra de testador de licença. O token cru vai para `raw_payload` e para a chamada à API.
- **Verificação no servidor, pela Google Play Developer API**:
  - `purchaseState` tem de ser comprado; pendente não libera.
  - `obfuscatedAccountId` tem de ser o `user_id`, com a mesma regra do `appAccountToken` na ADR 0013 (na restauração, só de conta que não existe mais).
  - O servidor faz o acknowledge logo depois de gravar a licença. Sem acknowledge em 3 dias, o Play estorna sozinho.
- **Compra de testador de licença libera a trilha**, como a de Sandbox na ADR 0013. A diferença é que só as contas que cadastramos no Play Console conseguem fazê-la.
- **Credencial sem chave.** A conta de serviço do Cloud Run é convidada no Play Console, com permissão só de ver dados financeiros e gerenciar pedidos. O token vem do servidor de metadados, com o escopo `androidpublisher`, e a chamada é REST por `net/http`, sem SDK, como na ADR 0016. Não entra segredo novo nem dependência nova.
- **Reembolso por consulta diária.** Um job do Cloud Scheduler chama `POST /api/v1/internal/play/voided`, que lê a Voided Purchases API e revoga pelo mesmo caminho de `revoked_transactions`. A rota confere emissor, audiência e a conta que assinou, e essa conta é só dela (ADR 0021).
- O provedor deixa de ser constante em `GrantEntitlement` e na notificação, e fecha a pendência da ADR 0021: as duas travas passam a usar o provedor da compra.

**Login no Android:**

- Google pelo Credential Manager. O ID token vem com a audiência do *client id web*, então o backend passa a aceitar uma lista de audiências (a do iOS e a web).
- GitHub funciona como está.
- Sem "Entrar com Apple": no Android ele exige um Services ID, que exige a conta paga. O segredo `logn-apple-signin-key` continua no Terraform, reservado.

**Documentos legais:** a v4 é substituída no lugar (`gen_documentos_legais.py --substitui`), porque só o dono a aceitou. O texto fica neutro de loja, com Google como processador do pagamento, o reembolso pelo Google Play e a seção da lista de espera.

## 3. Alternativas descartadas

- **Pagar a conta Apple e lançar nas duas lojas.** Recusado pelo custo antes de a ideia se provar.
- **Lista de espera das duas plataformas.** O Android sai antes, e a lista seria do dia do lançamento.
- **Lista no worker (D1/KV) ou em serviço de terceiro.** Tiraria dado pessoal do Postgres, da purga e da política atual.
- **`fetch` com JS no formulário.** Abriria o `connect-src` e manteria um script por uma coisa que o HTML resolve.
- **Notificações em tempo real do Play (RTDN) por Pub/Sub.** Seria uma rota pública nova, com tópico e assinatura, para ganhar menos de 24 h numa revogação de trilha.
- **Chave JSON de conta de serviço.** Seria um segredo a mais, no limite do plano gratuito, quando a conta do Cloud Run já basta.
- **Coluna `play_product_id` separada.** Os dois catálogos são nossos, e um id só dispensa o mapa.

## 4. Consequências

- **Sem AAB, sem teste ponta a ponta.** O Play Console só cria produto depois de receber um AAB com a permissão de billing, e `purchaseToken` só sai de um cliente real. Até lá, a verificação do Play é testada contra respostas montadas a partir da documentação.
- A página `/account/delete/` e a seção 10 da política dizem a mesma coisa. Quem mudar uma muda a outra. O app Android precisa usar os mesmos rótulos, "Gerenciar conta → Excluir minha conta".
- `LOGN_API_ORIGIN` na landing só depois da rota e da política no ar, nessa ordem.
- **Limites aceitos da lista:**
  - O token está na URL, então passa pelos registros da borda e do Cloud Run. Ele não traz dado pessoal e só confirma ou tira da lista aquela inscrição.
  - Trocar a chave do JWT mata todo link já enviado, inclusive o de saída. Quem estiver confirmado sai por `contact@logn.sh`.
  - Com `cpu_idle`, a goroutine do envio pode ficar sem CPU depois da resposta, como a do OTP. Se o envio morrer ali, a pendente fica sem e-mail até vencer.
  - Navegador sem `Sec-Fetch-Site`, que ainda manda `Origin: null`, não consegue se inscrever.
  - Antes de ligar, confira o formulário e os links do e-mail passando pela borda de verdade, com o Bot Fight Mode ligado.
- **A lista pode ficar anos parada.** O prazo "até o lançamento" foi escolhido sabendo disso. Se o gatilho não disparar, apagar a lista é decisão a revisitar aqui.
- O cliente Android, o push (FCM no Android, APNs no iOS quando houver conta), o `assetlinks.json` e o envio do aviso de lançamento ficam fora desta decisão.
- ADR 0001 e a spec das trilhas pagas continuam valendo no que descrevem da arquitetura. A ordem de lançamento passa a ser esta.
