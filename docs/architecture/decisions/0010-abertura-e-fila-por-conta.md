# ADR 0010: A abertura como verificação, e uma fila offline por conta

## 1. Visão Geral

O app não tinha splash. Enquanto o refresh e a busca do progresso estavam no ar, `has_session` era falso, e quem tinha sessão via a tela de login piscar. Com rede ruim, o timeout padrão do `URLSession` deixava a pessoa até um minuto nela. O sync não rodava na abertura: a fila guardada só subia depois da próxima resposta, ou pelo botão do Perfil.

A fila offline era uma chave só, `offline_events`, sem dono. O login não a limpava e `GameEvent` não tem `user_id`. Se a sessão de A expirasse e B entrasse no mesmo aparelho, o sync mandava as partidas de A como se fossem de B, e o rebase do 409 ainda as encaixava na cadeia de B.

O design da splash (`LogN Splash.dc.html`, no projeto de design) tem quatro telas: 01 recuperando, 02 termos mudaram, 03 sem rede, 04 sessão expirada. O plano e as decisões de 2026-09-24 estão em `tmp/claude-plan-20260924-141450.md`. Esta v0 faz 01, 03 e 04. A 02 fica para quando o app souber conferir os termos.

## 2. Decisão

**A abertura é um fluxo do Core.** O shell despacha `StartBoot` no lugar de `AttemptRefresh` e `RestoreOfflineQueue`. O Core mantém `Model.boot` e manda `ViewModel.boot` (`BootViewModel`): as linhas do log (`BootLine { check, verdict, detail, count }`), o progresso e `awaiting_offline_choice`. A frase de cada linha é do catálogo (`Str.Boot.*`). A ordem é sessão e, depois, sync da fila de quem é a sessão. A splash some quando a última linha fecha, sem duração mínima. O shell faz o fade e usa a cor do canvas na tela de lançamento, para a passagem não piscar.

**A espera tem teto de 5 s por verificação** (`BOOT_WAIT_SECS`, `crux_time::notify_after`):

- estourou na sessão: a abertura segue pelo caminho sem rede;
- estourou no sync: a splash sai e o envio termina por trás.

A resposta tardia do refresh nunca é descartada. O servidor já rodou o token, e reapresentar o antigo é reuso, que derruba todas as sessões. Pelo mesmo motivo, "Tentar de novo" com um refresh no ar (`refresh_in_flight`) espera a resposta dele em vez de mandar o token de novo. O relógio de cada tentativa leva o número dela (`attempt`), para o de uma tentativa anterior não cortar a atual.

**Sem rede, com a sessão dentro do prazo e com fila, a splash para** (tela 03), com "Tentar de novo" (`RetryBoot`) e "Continuar" (`ContinueOffline`). Sem fila, não há o que esperar da rede: o app entra direto, e a tarja de sem rede avisa. Sessão recusada, ou prazo vencido sem rede, fecha a linha da sessão em `Fail` e vai ao login. O login vem com o e-mail da sessão (`resume_email`) e o texto genérico de `SessionExpired`. O texto não afirma "30 dias": o mesmo 401 sai de reuso de token, de troca de senha e de exclusão pedida.

**Uma fila por dono.** A chave passa a ser `offline_events:<dono>`, e o dono é o id da conta ou `guest`. `Model.queue_owner` diz de quem é a fila na memória. `claim_queue` troca de dono: limpa a memória, que já está no disco sob o dono anterior, lê a fila do dono novo e, na ordem, as que ele adota. A adoção põe a fila adotada no fim, reencadeada pelo `GameEvent::rebase`. Primeiro grava sob o dono novo, depois apaga a chave velha (`QueueAdopted`): fechar o app no meio duplica a fila, mas nunca a perde. Resposta de leitura que chega depois de o dono mudar é descartada.

- Login, cadastro e troca de senha (`SessionExpiryStored`) adotam a fila do visitante e a chave antiga. Quem jogou como visitante e entra leva o que jogou, e a fila sobe logo.
- A abertura com sessão adota só a chave antiga, que é a migração das instalações de antes desta ADR.
- Sessão que acaba, saída, exclusão e "Jogar como visitante" passam a fila para `guest`. A fila da conta fica no disco até ela voltar.

**O id da conta fica no aparelho** (`account_user_id`, em `UserDefaults`, como `account_email`: identifica a conta, mas não autentica ninguém). Ele é gravado no login e em cada refresh, e apagado na saída e na exclusão. Sem rede, é ele que diz de quem é a fila. Aparelho que ainda não o tem segue com a chave antiga até o servidor responder.

**Desfazer a saída regrava a sessão inteira no aparelho**: token, `account_email`, `account_user_id`, `session_expires_at`, a fila e o retrato. Antes, só o token voltava ao disco. Fechar o app depois do desfazer perdia a fila, e a abertura seguinte sem rede caía no login por falta de prazo.

## 3. Consequências

- A primeira abertura depois desta versão, com sessão e rede, move a fila antiga para a chave da conta. Sem sessão, ela vai para `guest` e é adotada pela primeira conta que entrar, que é o comportamento de antes para esse caso. Uma fila antiga de A, deixada por uma sessão que já tinha expirado antes da atualização, ainda pode ir para B nessa migração. É uma vez só e não há como saber o dono.
- A fila de uma conta que nunca volta fica no `UserDefaults` até o app ser desinstalado.
- **Termos não são conferidos na abertura.** A seção de mudanças dos termos promete pedir o aceite de versão relevante antes de a pessoa continuar. Até o app fazer isso, `gen_documentos_legais.py` (em `logn-conteudo`) recusa `--material` com versão acima de 1 (`APP_PEDE_REACEITE`).
- A splash cobre só a abertura a frio. Voltar do segundo plano não passa por ela, porque não há `scenePhase` nem `Tick` na volta.
- O sync só tira da fila o que foi no envio (`sync_sent_ids`). Antes, a resposta 200 limpava a fila inteira, e a resposta dada com o sync no ar se perdia. O que sobrou sobe em seguida. Se a fila trocou de dono no meio do envio, os enviados saem também da fila do dono anterior, no disco (`SyncedQueueRead`).
