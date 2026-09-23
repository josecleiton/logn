# ADR 0007: `crux_time` como relógio do Core

## 1. Visão Geral

O backend passou a responder 429 nas rotas de autenticação: limite por IP, e intervalo mínimo entre dois envios de código para o mesmo e-mail. O app precisa travar o botão pelo tempo do `Retry-After` e mostrar a contagem.

Contar tempo exige relógio, e o Core não tinha um. A única hora que ele via era o `Event::Tick { now }`, que o shell manda na abertura do app. Essa hora envelhece: num 429 que chega uma hora depois de abrir o app, `now + 60` já está no passado, e o botão nem chegaria a travar.

## 2. Decisão

O Core ganha a capability `crux_time` (0.18, a da mesma geração do `crux_core` 0.20), com `facet_typegen`, e um efeito novo, `Effect::Time(TimeRequest)`.

Num 429, o Core guarda quantos segundos o servidor pediu e pede a hora com `Time::now()`. Quando a hora volta, ele fixa o fim do bloqueio e agenda `Time::notify_after(1s)`. A cada segundo pede a hora de novo e recalcula quanto falta. O `ViewModel` expõe só os segundos restantes (`auth_cooldown_seconds`, `resend_cooldown_seconds`), e as telas desenham o que recebem.

Pedir a hora a cada segundo, em vez de descontar um a cada disparo, é de propósito. O iOS congela o app em segundo plano e os disparos param junto. Quem volta depois de um minuto tem de achar o botão liberado, não com 59 s pela frente.

## 3. Alternativas descartadas

* **Contagem no shell, com `Timer` no SwiftUI.** Funcionaria, mas põe regra no cliente: o que trava, por quanto tempo e quando libera. O Android teria de reescrever tudo isso, e é exatamente o que o Core existe para evitar (ADR 0003).
* **Âncora pelo cabeçalho `Date` da resposta.** Dispensaria a capability, mas compararia o relógio do servidor com o do aparelho. Uma diferença de minutos entre os dois trava o botão tempo demais ou tempo nenhum.
* **O shell mandar `Tick` a cada segundo enquanto houver bloqueio.** Divide a decisão entre as duas camadas: o shell teria de saber quando começar e quando parar.

## 4. Consequências

* Os tipos de tempo saem no typegen em dois módulos Swift. `TimeRequest`, `TimerId` e `Instant` vão para `LogN`, porque o `Effect` os referencia. `TimeResponse` só existe em `App`, porque nenhum tipo de `LogN` o usa. O shell recebe o pedido com os tipos de `LogN` e responde com os de `App`. O bincode é o mesmo; o id passa de um para o outro pelo valor.
* O nome `Duration` gerado colide com o `Duration` do Swift. O shell sempre qualifica: `LogN.Duration`.
* O `Tick` continua existindo para o que já fazia, que é decidir se a sessão guardada vale sem rede. Migrar esse uso para o `crux_time` fica para quando alguém mexer nele.
* O shell passou a repassar os cabeçalhos da resposta HTTP. Antes ia uma lista vazia, e o Core não tinha como ler o `Retry-After`.
