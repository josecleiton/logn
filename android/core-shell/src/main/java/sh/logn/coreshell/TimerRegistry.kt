package sh.logn.coreshell

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import sh.logn.core.LogN.TimeRequest
import sh.logn.core.TimeResponse

/**
 * `Effect.Time` (crux_time): a hora, e o aviso de que um prazo passou.
 *
 * O Core não tem relógio. A contagem de um 429 pede a hora a cada segundo em vez de
 * descontar um por tique, porque o sistema congela o app em segundo plano: quem volta
 * depois de um minuto tem de achar o botão liberado.
 *
 * O pedido chega com os tipos de `sh.logn.core.LogN`, mas a resposta só existe em
 * `sh.logn.core`, porque nenhum tipo de `LogN` a referencia. O bincode dos dois é o
 * mesmo, então o id passa de um para o outro pelo valor, como em iOS.
 */
class TimerRegistry(
    private val scope: CoroutineScope,
    private val clock: WallClock,
    private val resolve: (requestId: UInt, response: TimeResponse) -> Unit,
) {
    private val timers = mutableMapOf<ULong, Job>()

    fun handle(
        requestId: UInt,
        request: TimeRequest,
    ) {
        when (request) {
            is TimeRequest.Now -> resolve(requestId, TimeResponse.Now(now()))
            is TimeRequest.NotifyAt -> {
                val atMillis =
                    request.instant.seconds.toLong() * 1000 + request.instant.nanos.toLong() / 1_000_000
                schedule(requestId, request.id.value, atMillis - clock.nowMillis()) {
                    TimeResponse.InstantArrived(sh.logn.core.TimerId(request.id.value))
                }
            }
            is TimeRequest.NotifyAfter ->
                schedule(requestId, request.id.value, (request.duration.nanos / 1_000_000u).toLong()) {
                    TimeResponse.DurationElapsed(sh.logn.core.TimerId(request.id.value))
                }
            is TimeRequest.Clear -> {
                timers.remove(request.id.value)?.cancel()
                resolve(requestId, TimeResponse.Cleared(sh.logn.core.TimerId(request.id.value)))
            }
        }
    }

    private fun schedule(
        requestId: UInt,
        timer: ULong,
        afterMillis: Long,
        response: () -> TimeResponse,
    ) {
        timers.remove(timer)?.cancel()
        timers[timer] =
            scope.launch {
                delay(afterMillis.coerceAtLeast(0))
                timers.remove(timer)
                resolve(requestId, response())
            }
    }

    private fun now(): sh.logn.core.Instant {
        val millis = clock.nowMillis()
        return sh.logn.core.Instant(
            seconds = (millis / 1000).toULong(),
            nanos = ((millis % 1000) * 1_000_000).toUInt(),
        )
    }
}
