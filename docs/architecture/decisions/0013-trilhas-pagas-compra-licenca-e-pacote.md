# ADR 0013: Trilha paga — compra, licença e pacote cifrado

> A compra pelo Google Play, ao lado desta, está na ADR 0022. O que vale aqui para a App Store continua valendo.

## 1. Visão Geral

A spec `docs/specs/logn_trilhas_pagas_spec.md` define a trilha paga: compra avulsa pela App Store, direito na conta, conteúdo baixado e cifrado, 30 dias sem rede. A primeira implementação passou por build e teste e tinha o paywall contornável de três jeitos independentes:

- A cadeia `x5c` do JWS da Apple era aceita comparando o **nome** da raiz com `"Apple Root CA - G3"`. Qualquer um gera uma raiz autoassinada com esse nome: a compra forjada virava direito de acesso, e a notificação forjada revogava a compra de outra pessoa.
- A revogação vivia só em `entitlements`, e o upsert da compra reativava o direito. Um JWS assinado antes do reembolso não traz `revocationDate`: bastava reenviá-lo.
- `GET /api/v1/challenges` devolvia a trilha paga inteira, em claro, sem conta. O pacote cifrado não protegia nada.

Esta ADR registra o desenho que substitui aquele, e a dependência nova que ele traz.

## 2. Decisão

**Confiança por raiz fixa.** `internal/storekit` embute a Apple Root CA - G3 (`AppleRootCA-G3.pem`, SHA-256 `63:34:3A:BF:…:3E:91:79`, conferido contra a página da Apple) e verifica a folha com `x509.Verify` contra esse conjunto e a intermediária do cabeçalho. A raiz que vem no token é ignorada. Folha e intermediária precisam das extensões da Apple (`1.2.840.113635.100.6.11.1` e `…6.2.1`), o algoritmo é fixo em ES256 e a chave é P-256. A transação precisa ser `Non-Consumable` e `PURCHASED`: sem Compartilhamento Familiar (spec, seção 2). O ambiente Xcode só existe fora do Cloud Run, e só com a raiz local exportada do Xcode em `APPLE_XCODE_ROOT_CERT`; não há mais atalho que pule a cadeia.

**Produção aceita Production e Sandbox.** A App Review compra em Sandbox contra o build de produção, e recusar isso é rejeição. O custo, aceito: quem testa pelo TestFlight ganha a trilha de verdade. O ambiente fica gravado em `store_transactions`.

**A compra é da conta que comprou.** O app compra com `appAccountToken` igual ao id da conta. `POST /api/v1/purchases` recusa transação de outro token (`purchase_account_mismatch`). `POST /api/v1/purchases/restore` aceita a de uma conta que **não existe mais** e recusa a de outra conta que existe (`purchase_owned_by_other_account`). Conta com exclusão pedida ainda segura a compra: a exclusão se desfaz com uma troca de senha, e soltar a compra ali deixava uma compra só rodar entre contas (pede exclusão, o próximo restaura, desfaz, repete). O critério 6 da spec vale depois do expurgo.

**Revogação fora da conta.** `revoked_transactions` guarda toda transação revogada, automática ou manual, por `(provider, original_transaction_id)`, sem chave para `users`: excluir a conta não apaga a revogação. `GrantEntitlement` consulta a tabela antes de tudo e responde `purchase_revoked`. `store_transactions` também sobrevive à conta (`ON DELETE SET NULL`), é única por transação e guarda o JWS como chegou.

**Uma transação por vez.** Conceder, revogar e reverter a mesma transação passam por `pg_advisory_xact_lock` da transação da loja. Sem isso, uma restauração lia "não revogada", o REFUND gravava a revogação sem ver a linha ainda não confirmada, e a compra reembolsada terminava ativa.

**Notificações.** `POST /api/v1/appstore/notifications`, o caminho da spec, passa por teto de corpo (256 KB, folgado até haver medida) e de ritmo. Fica fora da verificação de origem e é cadastrada no App Store Connect na URL `.run.app`, não no domínio atrás do Cloudflare: o Bot Fight Mode não aceita exceção e desafiaria a Apple com JS, como já acontecia com o Cloud Scheduler. A defesa dela é a assinatura contra a raiz fixa. Erro de banco responde 500 para a Apple mandar de novo; a versão anterior respondia 200 e perdia o reembolso. A Apple reenvia e não garante ordem, então a ordem sai da hora assinada (`signedDate`): a revogação guarda `revoked_at_ms`, a reversão guarda `reversed_at_ms`, e um REFUND mais velho que a reversão dele é ignorado, chegue antes ou depois. Só reembolso se reverte. `REVOKE` tem motivo próprio (`store_revoke`), diferente do que a spec dizia, justamente para um `REFUND_REVERSED` não desfazê-lo; a revogação manual também não se desfaz por notificação.

**Catálogo.** `GET /api/v1/tracks` devolve todas as trilhas, a principal primeiro, com tipo, cor do balão (`tracks.color`), línguas publicadas, número de nós e de problemas, se a conta tem direito e o motivo da revogação quando houver. O Core guarda a trilha escolhida no aparelho e entrega a árvore filtrada por ela; os selos de validade, a oferta do fim da amostra e o passo a passo da compra saem do `view`, e o shell só desenha.

**Conteúdo fechado só no pacote.** A regra da amostra mora num lugar só, `openNode` em `repository.go`: nó da trilha gratuita ou `row_idx = 0`. Ela filtra `GET /api/v1/challenges`, decide o `requires_purchase` de cada nó em `GET /api/v1/nodes` e decide o XP no sync. O Core não adivinha pela linha nem por um id de trilha fixo.

**XP.** O sync só paga desafio que existe no banco (antes um id inventado virava XP), com o nó do banco, não o do evento, e só paga trilha paga fora da amostra com direito ativo; o evento segue na cadeia (ADR 0002). `GET /api/v1/progress` ganha `paid_track_xp`. O portão de um nó conta o XP da trilha dele; o da gratuita é o global menos a soma das pagas, que é todo o XP de antes das trilhas.

**Licença e aparelhos.** `GET /api/v1/tracks/{id}/license` exige `X-Device-ID` (UUID, o `identifierForVendor`) e registra o aparelho **sem limite**: o registro é evidência para a revogação por compartilhamento (spec, seção 7), não trava. Um limite, se vier, vem depois de medir quantos aparelhos uma conta legítima usa. A licença leva a chave de conteúdo, a versão, `issued_at` e `valid_until` (30 dias).

**Chave em repouso.** `track_keys.wrapped_key` é o nonce seguido da chave cifrada em AES-256-GCM com `TRACK_KEY_SECRET` (32 bytes, base64), com a trilha e a versão como dado associado. A chave nasce na primeira licença ou pacote pedido da versão. O servidor aborta em produção sem o segredo, como com `JWT_SECRET`. A chave fixa de zeros que saía quando faltava a linha acabou.

**Pacote.** `GET /api/v1/tracks/{id}/package` devolve o nonce de 12 bytes seguido do AES-256-GCM do JSON `{track_id, content_version, challenges: {língua: [...]}}`, com `logn-track:{id}:{versão}` como dado associado: pacote de outra trilha ou versão não abre.

**No aparelho.** A licença vai para o Keychain, só deste aparelho (`track_key:<conta>:<trilha>`, prefixo que o `CoreWrapper` manda para o Keychain). O pacote vai para um arquivo em Application Support (`track_package:`), com o nome derivado por hash, nunca da chave crua. O retrato offline, em `UserDefaults`, não leva licença nem desafio fechado: o conteúdo fechado só existe aberto na memória. O Core confere a validade com a hora do shell, nunca abaixo da maior hora já vista (pelo relógio ou pela emissão de uma licença), guardada no Keychain junto das licenças: atrasar o relógio depois de vencer não ressuscita a licença. Também recusa relógio antes de `issued_at` (menos 5 minutos de folga). Congelar o relógio sem nunca deixar o app ver a hora certa segue esticando o prazo; sem fonte de hora confiável offline, isso fica dentro do que a spec aceita como fora do alcance. Chave por conta, e resposta de licença ou pacote pedida por outra conta é descartada: quem entra depois no mesmo aparelho não abre a trilha de outra pessoa.

**Compra no app.** A transação verificada vai ao Core (`SubmitPurchase`) e ao servidor; o shell só finaliza na loja o que volta em `purchases_to_finish`. Recusa definitiva (403, 409) também finaliza, senão a loja entregaria a transação a cada abertura para sempre; erro do servidor e falta de rede ficam abertos e voltam por `Transaction.unfinished`.

## 3. Dependência nova

**`aes-gcm` 0.10 (RustCrypto) no `shared_core`,** com o `Cargo.lock` versionado. Abre o pacote no Core, que é onde mora a regra de quando ele abre. É Rust puro, sem código nativo, e compila nos três alvos do iOS. No Go, a cifra sai da biblioteca padrão (`crypto/aes`, `crypto/cipher`); nenhuma dependência nova no backend. `tracks.rs` tem um vetor cifrado em Go que o Rust abre, para provar que os dois lados concordam no formato.

## 4. Alternativas descartadas

* **Decifrar no shell com CryptoKit.** Tiraria a dependência do Rust, mas poria no Swift a decisão de quando o pacote abre, e o Android teria de repeti-la.
* **Licença assinada pelo servidor, com a chave embrulhada por aparelho.** Protege mais contra quem edita o Keychain, mas a spec fixa a ameaça em cópia casual e deixa jailbreak fora. Keychain só deste aparelho, fora do backup, já impede a cópia casual da licença e do prazo.
* **Limite de três aparelhos.** Contraria a spec (seção 2 e 10) e pune troca de celular. O header forjado furava o limite de qualquer jeito.
* **Aceitar só Production.** Mais estrito, e rejeição quase certa na App Review.

## 5. Consequências

* `TRACK_KEY_SECRET` e `APPLE_BUNDLE_ID` passam a ser obrigatórios no Cloud Run. Trocar `TRACK_KEY_SECRET` invalida as chaves guardadas: é rotação com migração de dados, não troca de variável.
* A revogação manual do runbook passa por `RevokeTransaction` (ou pelo mesmo par de `INSERT`/`UPDATE`), para valer também contra a restauração numa conta nova.
* Nó novo de trilha paga não precisa de nada no app: o servidor diz `requires_purchase`.
* A semente empacotada leva `track_id` e `requires_purchase` nos nós, com `default` no Core; a versão dela não muda.
* **Risco aceito: uma chave por versão, igual para todo comprador.** A chave de um comprador abre o pacote daquela versão para qualquer um, e quem baixa e depois reembolsa pode guardar o conteúdo com um cliente modificado. É a ameaça que a spec deixa fora (extração); o remédio é subir `content_version`, que gera chave nova.
* **Licença, pacote e compra têm balde de ritmo próprio**, separado do login, porque cada abertura revalida as trilhas da conta.
* **Pendências conhecidas.** (1) Reembolso cuja notificação esgota as tentativas da Apple nunca chega: falta conciliar pela App Store Server API (histórico de reembolso). (2) A cadeia é verificada na hora de agora, não no `signedDate`; restaurar um JWS cuja folha venceu vai falhar fechado. (3) Sandbox em produção dá a trilha de verdade a quem testa pelo TestFlight; se isso virar problema, a saída é tratar direito de Sandbox como temporário.
