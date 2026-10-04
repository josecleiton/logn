# ADR 0026: caixa de saída de e-mail com Cloud Tasks

## 1. Contexto

Quatro e-mails saíam numa goroutine disparada depois da resposta: o código de
verificação (`request-otp`), as boas-vindas do cadastro por senha e do cadastro social, e
a confirmação da lista de espera. O Cloud Run roda com `cpu_idle = true`, e com isso corta
a CPU da instância quando a resposta termina. A goroutine podia congelar no meio do SMTP
ou morrer com a instância, e ninguém ficava sabendo: o código não chegava, não havia log
nem nova tentativa, e a pessoa só podia pedir outro depois do intervalo de reenvio.

Trocar `cpu_idle` por CPU sempre ligada resolveria o congelamento, mas custa instância
cobrada o tempo todo e não resolve a instância que cai. Mandar o e-mail dentro do pedido
deixaria o cliente esperando o SMTP e ainda sem nova tentativa.

## 2. Decisão

**Outbox transacional.** O e-mail vira uma linha de `email_outbox`
(`0073_caixa_de_saida_de_email.sql`), gravada na mesma transação da mudança que o pede:
`SaveOTP`, `JoinWaitlist`, `CreateUser` e `CreateSocialUser`. `SaveOTP` e `JoinWaitlist`,
que eram comandos soltos no pool, passaram a abrir transação. Sem o commit, não há
e-mail; com ele, o e-mail existe mesmo que tudo o que vem depois falhe.

**A linha guarda referência, não endereço, sempre que dá.** As boas-vindas apontam para
`users`, a confirmação para `waitlist_entries`, e as duas somem em cascata com elas. O
código de verificação não tem para onde apontar, porque `otps` só guarda o HMAC: ele vai
cifrado com AES-256-GCM, com o endereço e o propósito como dado associado, numa chave
derivada por HKDF de `TRACK_KEY_SECRET` com o rótulo `logn/email-outbox/v1`. Nenhum
segredo novo no Secret Manager, que está no limite do free tier. Um `CHECK` amarra o texto
cifrado ao estado: ele existe só enquanto a linha de código está `pending`, e sai no
envio, no descarte e na desistência.

**Quem põe na fila é um middleware, não o handler.** O domínio, depois do commit, anota o
id da linha no `OutboxTracker` do pedido (`domain.TrackOutbox`, no `context`). O
middleware `withOutbox`, em volta de todas as rotas, cria o tracker e, quando o handler
termina, põe na fila o que foi anotado, antes de a resposta acabar de sair: ainda dentro
do pedido, com CPU. Anotar só depois do commit impede que uma transação que voltou ponha
na fila uma linha que não existe. Sem tracker no `context` (teste, rotina fora de
pedido), a anotação não faz nada, e a varredura leva a linha. Nenhum handler precisa
lembrar de despachar.

**A fila é o Cloud Tasks.** A tarefa leva só o id da linha. A fila (`email`, em
`cloud_tasks.tf`) chama `POST /api/v1/internal/email/send` na URL `.run.app`, com token
OIDC da conta `logn-tasks`, a única que a rota aceita (ADR 0021). A rota trava a linha
(`FOR UPDATE`), confere se o e-mail ainda faz sentido, envia e marca. Código vencido,
trocado por um mais novo (o `code_hash` de `otps` já é outro) ou usado, conta em
exclusão e inscrição já confirmada viram `skipped` sem envio. Falha do SMTP responde 503,
e a fila tenta de novo; na quinta tentativa (`X-CloudTasks-TaskRetryCount`), a linha vira
`failed`, e a inscrição da lista de espera volta a aceitar envio, como fazia a goroutine.

- **Nome da tarefa fixo**, `outbox-<id>`: pôr a mesma linha na fila de novo recebe 409,
  que conta como sucesso.
- **Ritmo**: dois por segundo, o teto do SMTP do Resend no plano free; cinco tentativas
  com esperas de 30 s, 1, 2 e 4 min (7,5 min ao todo), dentro dos quinze minutos do
  código.
- **Prazo do SMTP**: o envio roda com a linha travada e uma conexão do pool presa, e o
  gomail só tem prazo para conectar. A rota desiste dele em 20 s e conta como falha; o
  envio que ficou para trás ainda pode chegar, e o e-mail sai duas vezes. Sem o prazo, um
  SMTP que segurasse a conexão esgotava o pool da instância única.
- **Pub/Sub ficou de fora.** Ele é para evento com vários consumidores; aqui é um
  comando com um destino, e o Cloud Tasks traz o que ele não tem: teto de vazão por fila,
  deduplicação pelo nome e entrega agendada.

**Varredura de cinco em cinco minutos.** O Cloud Scheduler chama
`POST /api/v1/internal/email/sweep`, com a conta da purga, e ela põe na fila as linhas
pendentes há mais de um minuto que nunca viraram tarefa (`enqueued_at` vazio): a fila
estava fora do ar, ou a instância caiu entre o commit e o fim do pedido. Para na
primeira recusa da fila, que cria uma tarefa por chamada (não há criação em lote na API
v2): insistir com a fila fora do ar passaria do prazo do pedido. Antes de pôr na fila,
ela faz duas coisas:

- descarta os códigos que passaram do prazo, para o texto cifrado não esperar a poda;
- dá por perdidas (`failed`) as linhas que viraram tarefa há mais de uma hora sem
  desfecho. A fila desiste sem a rota gravar nada quando as tentativas acabam em erro de
  banco, 429 ou timeout do Cloud Run, e a linha ficava pendente até a poda. Na lista de
  espera isso deixava a pessoa sem o link e sem como pedir outro. Pôr de novo na fila não
  serve: o Cloud Tasks guarda por um tempo o nome da tarefa executada, e recusaria a nova.

É o terceiro job do Scheduler, o último do free tier; cada execução é um pedido curto ao
Cloud Run, 8.640 por mês, menos de 1% da cota gratuita de pedidos e de CPU.

**Poda.** A purga diária apaga as linhas com mais de uma semana. Elas respondem "esse
e-mail saiu? quando? em qual tentativa?", e `last_error` guarda o erro do SMTP sem
endereço, ou o motivo do descarte quando o código não abre mais com a chave atual. As de
código saem em 24 h: guardam o endereço em claro, e qualquer um pede código para
qualquer endereço, inclusive sem conta. A exclusão de conta apaga as linhas de código do
endereço, que não têm chave para `users`.

**Desenvolvimento sem fila.** Sem `CLOUD_TASKS_QUEUE` e `CLOUD_TASKS_SERVICE_ACCOUNT`, o
middleware envia na hora, como última tentativa, e o Mailpit recebe como antes. No Cloud
Run, sem as duas, ou com só uma, o servidor não sobe.

**Cliente REST à mão.** O cliente gerado `google.golang.org/api/cloudtasks/v2` puxa
`github.com/google/uuid`, que o servidor não tinha. A chamada é um `POST` com JSON, e o
token sai de `golang.org/x/oauth2/google`, que já estava no grafo (passou de indireta a
direta no `go.mod`, na mesma versão). Nenhum módulo novo, nenhuma linha nova no `go.sum`.

**O aviso de licença fica de fora.** Ele sai dentro da rota de administração, e quem roda
`just revoke` vê na resposta se o e-mail saiu (`email_sent`). Pela fila, ganharia nova
tentativa e perderia essa resposta.

## 3. Consequências

- **A lista de espera volta a dizer, pelo tempo de resposta, quem é inscrição nova.** A
  goroutine existia para o `POST /api/v1/waitlist` responder no mesmo tempo para endereço
  novo e repetido. Agora a inscrição nova espera a fila aceitar a tarefa, algumas dezenas
  de milissegundos que a repetida não gasta. Quem medir consegue saber se um endereço já
  estava na lista. A alternativa era deixar a confirmação só para a varredura, chegando
  em um a seis minutos com o tempo de resposta igual; a troca foi decisão do dono do
  projeto, por ser lista de espera de um app sem usuários ainda, e fica registrada aqui
  porque afrouxa uma defesa que existia (regra 9 do AGENTS.md).
- O pedido que grava e-mail fica mais lento pelo tempo de criar a tarefa. Se a fila
  demorar mais de 5 s, o pedido desiste e a varredura leva a linha.
- Um e-mail pode sair duas vezes, se o processo morrer entre o SMTP aceitar e a linha
  ser marcada como enviada. Nunca sai nenhuma vez sem deixar rastro.
- `OutboxMaxAttempts` (backend) e `max_attempts` da fila (Terraform) andam juntos.
- Deploy: primeiro `terraform apply` (API ligada, fila, conta, permissões, job e as
  variáveis do Cloud Run), depois o backend. Na ordem inversa, a revisão nova não sobe.
