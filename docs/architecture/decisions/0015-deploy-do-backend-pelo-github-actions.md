# ADR 0015: Deploy do backend pelo GitHub Actions

## 1. Visão Geral

O deploy do backend era `just deploy-backend` na máquina do dono: `gcloud run deploy --source ./backend`, com a sessão do `gcloud` dele. A imagem embute `backend/schema/migrations/*.sql`, e as migrações de conteúdo (a trilha, pela regra 8) não estão no repositório público: ficam no `.gitignore` e só existem no disco de quem as copiou do repositório de conteúdo. Um deploy feito de um clone limpo subiria sem elas.

## 2. Decisão

**O workflow de deploy mora no repositório de conteúdo, que é privado.** Ele copia as migrações de conteúdo para `backend/schema/migrations/`, recusa documento legal em rascunho (a mesma trava do `just migrate-prod`), confere que o upload leva todas as migrações e roda o mesmo `gcloud run deploy --source`. A API aplica as migrações no boot, como antes. Um nome de migração que exista nos dois repositórios reprova o deploy.

**Publicar é confiança total, e só a main do app é publicada.** O código publicado roda com a conta de runtime e aplica SQL no boot com a role de migração. Por isso o workflow resolve o head da main do app no começo e publica aquele SHA, sem aceitar commit ou branch escolhido por quem dispara. O que chega à main vai para produção: ver a seção 4, sobre a falta de branch protection.

**Código do app não roda perto do token.** O `content-check` executa código do app, então roda num job sem `id-token`. O job de deploy, com `id-token`, só copia arquivos e chama o `gcloud`, e exige o environment `production`. A saída do check vai para o log só como veredito, sem o texto dos achados, porque o log é legível por quem tem o token de dispatch.

**Dispara num push na main de qualquer um dos dois, e à mão no repositório de conteúdo.** No app, `.github/workflows/deploy-backend.yml` só chama o workflow do conteúdo, quando o push na main mexe em `backend/`. O token dele é fine-grained, com `Actions: read and write` só no repositório de conteúdo, e mora num environment do app restrito à main. Ele dispara, cancela e reexecuta workflow e lê o log de lá; não lê nem escreve o repositório. Reexecutar um run antigo não faz downgrade: o run resolve de novo o head da main.

**O Google aceita o GitHub por OIDC, sem chave.** Workload Identity Federation (`terraform/github_deploy.tf`). A condição do provider exige o id numérico do repositório de conteúdo (não se recria apagando o nome), `refs/heads/main`, o arquivo exato do workflow e o environment `production`.

**Duas contas de serviço.** `logn-deployer`, que o GitHub assume, tem `roles/run.sourceDeveloper` (sobe código, cria build e revisão; não lê o Secret Manager direto nem muda IAM) e age como a conta de runtime e como `logn-builder`. `logn-builder` roda o Cloud Build: escreve no repositório `cloud-run-source-deploy`, lê o bucket `run-sources-…` e grava log, cada permissão no recurso e não no projeto. O build rodava com a conta padrão do Compute, que tem `roles/editor`; agir como ela daria ao CI o projeto inteiro.

**Actions fixadas por SHA**, com a versão em comentário: tag pode ser movida por quem controla a action.

## 3. Alternativas descartadas

- **Workflow no repositório do app, lendo o conteúdo com deploy key.** Disparo mais simples, mas o repositório público passaria a guardar uma credencial que lê a trilha inteira, e qualquer mudança no workflow viraria caminho para ela.
- **Chave JSON de conta de serviço em segredo do GitHub.** Não expira, não tem condição de repositório e vaza inteira se vazar.
- **Commitar as migrações de conteúdo no app.** É exatamente o que a regra 8 proíbe.
- **`roles/run.builder` para a conta de build.** Tem as mesmas permissões, mas no projeto inteiro.

## 4. Consequências e risco conhecido

- **As mains não têm branch protection.** Os dois repositórios são privados numa conta Free, e o GitHub só oferece ruleset e proteção de branch nesse caso com o Pro. Qualquer push na main de um dos dois vai para produção sem revisão, e force push na main não é barrado. O que continua de pé sem isso: o Google só aceita o token do job de deploy da main do repositório de conteúdo, no environment `production`; e o token de disparo do app mora no environment `deployment`, que só aceita a main. Quando o app ficar público, ruleset nele passa a ser gratuito e entra aqui; o repositório de conteúdo continua sem, a menos que a conta mude de plano.
- **Não está confirmado que a restrição de branch dos environments é aplicada em repositório privado no plano Free.** O GitHub aceitou configurá-la. A condição do Google não depende dela: exige `refs/heads/main` e o arquivo do workflow.

- **O deployer vale o que a conta de runtime vale.** Agir como ela é poder rodar código com as permissões dela. Hoje ela tem `secretmanager.secretAccessor` no projeto (todos os segredos, não só os do serviço), `storage.editor` e `firebase.editor`. O CI não piora o que um deploy à mão já podia, mas o alcance de um workflow comprometido é esse. A correção é uma conta de runtime dedicada, com `secretAccessor` só nos segredos do `logn`; é mudança no serviço no ar e fica para uma decisão à parte.
- **`roles/run.sourceDeveloper` inclui criar serviço e job** (`run.services.create`, `run.jobs.create`) e SSH na revisão. Com o `actAs` acima, um workflow comprometido poderia criar outro serviço rodando como a conta de runtime.
- **A conta de build pode sobrescrever imagem** no repositório `cloud-run-source-deploy`, inclusive a de produção. É o que o build precisa fazer.
- O state do Terraform continua local: o CI faz deploy, não `terraform apply`.
- `just deploy-backend` segue funcionando da máquina do dono, para emergência.
- Enquanto o app for privado, o workflow do conteúdo precisa de `LOGN_APP_READ_TOKEN` (fine-grained, `Contents: read` no app). Quando o app abrir, o segredo sai.
- O workflow do app retorna verde assim que dispara: o resultado do deploy aparece no repositório de conteúdo.
- As migrações de conteúdo que sobem são as do repositório de conteúdo; a cópia no disco local deixa de importar.

## 5. Conferência manual depois do primeiro apply

- Um `workflow_dispatch` do conteúdo a partir de outra branch tem de falhar na troca de token com o Google.
- Um job sem `environment: production` tem de falhar do mesmo jeito.
- O passo "Confere o que sobe" tem de mostrar a mesma contagem de migrações do disco local.
