# ADR 0021: Revogação manual de licença e quem assina as rotas internas

## 1. Visão Geral

A seção 10.5 dos termos (v3) tem dois casos de revogação feitos por nós: redistribuição do conteúdo e compartilhamento da conta. Para os dois, o texto promete o aviso por e-mail logo depois de revogar, com o motivo e o caminho da contestação, resposta em até 5 dias, e, no compartilhamento, a licença de volta enquanto a contestação é analisada. A spec das trilhas pagas deixou a revogação manual para "um runbook com SQL", que nunca foi escrito. O que existia era `RevokeTransaction`, a função da notificação de reembolso da Apple, e ela não resolvia:

- **Não havia como devolver a licença.** `revoked_transactions` só aceita desfazer reembolso (`CHECK (reversed_at_ms IS NULL OR reason = 'refund')`), e a devolução durante a análise é promessa do texto.
- **Os dois bloqueios dividiam uma linha.** A revogação que já vale não muda de motivo. Um reembolso que chegasse com a licença revogada à mão não deixava rastro, e devolver a licença depois da contestação a devolvia a quem a Apple já tinha reembolsado.
- **Não havia aviso.** Nada mandava e-mail, e nada dizia em que língua.

Havia também uma brecha na única rota interna. A purga conferia assinatura, emissor e audiência do token OIDC, mas não quem o assinou. Qualquer conta de serviço de qualquer projeto no Google Cloud emite um token do Google com a audiência que pedir, e a audiência é a URL `.run.app` do serviço, que não é segredo. O estrago era pequeno (a purga só apaga contas com exclusão vencida), mas a defesa era mais fraca do que a regra 9 do AGENTS.md pede.

## 2. Decisão

**Dois bloqueios, cada um com o seu caminho de saída.** A revogação manual ganha `manual_revocations`, chaveada pela transação como `revoked_transactions` e sem chave para `users`, para sobreviver à exclusão da conta e barrar a restauração numa conta nova. `revoked_transactions` fica só com os motivos da loja (`refund`, `store_revoke`, `fraud`). A licença vale quando nenhum dos dois bloqueia.

**A contestação é um estado do bloqueio, não uma revogação nova.** O bloqueio manual está `revoked` ou, só no compartilhamento de conta, `review`: a licença volta enquanto analisamos, e o bloqueio fica aberto até a decisão. O ciclo inteiro, pela seção 10.5:

| Passo | Quando | Licença | Bloqueio | Aviso |
|---|---|---|---|---|
| `revoke` | licença ativa, sem bloqueio aberto | sai | `revoked` | revogação, com motivo e como contestar |
| `received` | redistribuição, `revoked` | segue fora | `revoked` | contestação recebida, resposta em 5 dias |
| `review` | compartilhamento, `revoked` | volta | `review` | licença de volta enquanto analisamos |
| `accepted` | `revoked` ou `review` | volta (ou fica) | apagado | contestação aceita |
| `rejected` | redistribuição `revoked`; compartilhamento `review`, ou `revoked` com reembolso | sai (ou segue fora) | `revoked` | revogação mantida |

- O motivo é o gravado na revogação; a contestação não o troca.
- **Com a loja revogando também (um reembolso), a loja manda na licença.** `review` é recusado (`license_store_revoked`), porque a licença não pode voltar para a análise. A contestação continua tendo resposta: `rejected` sai direto do `revoked`, e `accepted` apaga o bloqueio, deixa a licença fora pelo motivo da loja e manda um aviso que diz isso. Se a Apple reverter o reembolso depois, a licença volta só se não houver bloqueio nosso.
- `ReinstateRefund` não reativa licença com bloqueio `revoked`: reverter o reembolso não desfaz a revogação nossa, e o motivo da licença volta a ser o da revogação, para uma contestação aceita depois ainda devolvê-la.
- `GrantEntitlement` confere os dois bloqueios; o `review` não bloqueia, porque a licença voltou.
- `RevokeTransaction` recusa motivo manual.

**Histórico com evidência e autor.** Cada passo vira uma linha de `license_actions` (só INSERT): motivo, resultado, evidência em texto (até 2000 caracteres) e a conta de serviço que chamou.

**Aviso por e-mail, na língua do último aceite.** A conta não guarda língua. A do último aceite dos termos é a que a pessoa usava no app, e o nome da trilha sai nela, com o português quando faltar tradução. O texto mora no Go, como o dos outros e-mails (`email/license_copy.go`), com um template (`license.html`) e um aviso por passo. Sem e-mail configurado, a rota recusa antes de mudar qualquer coisa. A mudança é gravada antes do envio; se o SMTP falha, ela fica, e a resposta traz `email_sent: false` para quem opera avisar por outro caminho.

**Rotas internas, chamadas por uma pessoa.** `POST /api/v1/internal/licenses/revoke` e `/appeal`, com limite de 10 por minuto e corpo de até 32 KB. Quem chama emite o token em nome de `logn-admin`, uma conta de serviço sem papel nenhum no projeto e sem chave, com a própria conta Google:

```
gcloud auth print-identity-token --impersonate-service-account=<logn-admin> \
  --audiences=<URL .run.app> --include-email
```

Só pode fazer isso quem está em `admin_members` no Terraform, com `roles/iam.serviceAccountOpenIdTokenCreator` sobre essa conta, nunca no projeto. Esse papel só emite token de identidade; o `TokenCreator` daria também token de acesso e assinatura em nome dela. `just revoke` e `just appeal` (`tools/license_admin.py`) leem a audiência e a conta do próprio serviço no Cloud Run e chamam a URL `.run.app` direto.

**Toda rota interna confere quem assinou.** O validador devolve o e-mail verificado do token, e cada rota aceita uma conta só, comparada exatamente: a purga, a `logn-scheduler` (`CLOUD_SCHEDULER_SERVICE_ACCOUNT`); as de licença, a `logn-admin` (`ADMIN_SERVICE_ACCOUNT`). O Scheduler deixa de assinar com a conta do Cloud Run: com ela, o próprio servidor, ou quem o fizesse buscar uma URL, emitia token de purga pelo servidor de metadados. Variável faltando fecha a rota (403), como já fazia a audiência. O autor gravado no histórico sai do token, nunca do corpo, que recusa campo desconhecido.

## 3. Alternativas descartadas

- **Comando local contra o banco de produção.** Não abre rota nova, mas põe a senha do banco e a do SMTP na máquina de quem opera, e a identidade de quem revogou vira o que a pessoa digitar.
- **Uma tabela só, com `reversed_at_ms` liberado para os motivos manuais.** Continua sem distinguir o reembolso que chega no meio da contestação.
- **Apagar a linha de `revoked_transactions` para devolver.** Mesmo problema, e some o rastro.
- **Token estático compartilhado.** Recusado pela regra 9.

## 4. Consequências

- A primeira revogação manual deixa de depender de SQL à mão, e o aviso da seção 10.5 passa a existir.
- Deploy em ordem: o `terraform apply` (as duas contas, a permissão, o job do Scheduler e as duas variáveis) antes do código novo. Sem `CLOUD_SCHEDULER_SERVICE_ACCOUNT`, a purga responde 403 e pula o dia.
- A evidência é texto livre. É registro do que foi visto, não cópia de dado pessoal; o `just revoke` diz isso.
- **Limite aceito: o histórico sai com a conta, e o bloqueio não.** A evidência some no expurgo, como a licença e o registro de aparelhos (política, seção 9), e o bloqueio fica, para a compra não voltar numa conta nova. Uma contestação aceita depois da exclusão não tem rota que tire o bloqueio: as duas partem da licença de uma conta que existe. Se acontecer, é SQL à mão.
- **O bloqueio é da transação, a rota parte da licença da conta.** Quem, com a licença revogada, compra de novo com outro Apple ID ganha uma transação nova, que os termos permitem, e a contestação da antiga deixa de ter rota. Quem, com a contestação em análise, exclui a conta e restaura numa nova passa, porque `review` não bloqueia, e a decisão tem de mirar a conta nova. São raros; se acontecerem, é SQL à mão com o histórico de `license_actions` na frente.
- A trava de `GrantEntitlement` usa o provedor fixo da Apple, e a da revogação manual, o provedor gravado na compra. Hoje são o mesmo; um segundo provedor precisa alinhar as duas.
- **A política cita o histórico desde a v4** (2026-09-30, não relevante): a seção 3.3 descreve a anotação e quem a escreveu, e as seções 9 e 10 dizem que ela vive e morre com a conta.
- Fica de fora: tela de administração (spec, fora do escopo), e revogação manual por fraude, que a seção 10.5 não põe entre os casos com aviso.
