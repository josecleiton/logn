package sh.logn.coreshell

import sh.logn.core.LogN.HttpRequest
import sh.logn.core.LogN.HttpResult
import sh.logn.core.LogN.KeyValueOperation
import sh.logn.core.LogN.KeyValueResult
import sh.logn.core.LogN.LogOperation
import sh.logn.core.LogN.MonitoringOperation
import sh.logn.core.LogN.TelemetryOperation

// O que o shell faz pelo Core, um efeito por porta. A ponte (Core.kt) só despacha; quem
// sabe falar com a rede, o disco e o PostHog são as implementações em :app. As portas
// que devolvem valor são `suspend` e trocam de thread sozinhas: a ponte chama e resolve
// sempre na thread principal.
//
// As portas não lançam: falha vira o `Err` do efeito. Exceção que escapasse de uma
// derrubaria o processo e deixaria o Core esperando um resolve que nunca vem.

/** `Effect.Http`: faz o pedido e devolve a resposta, com os cabeçalhos. */
fun interface HttpPort {
    suspend fun perform(request: HttpRequest): HttpResult
}

/** `Effect.SecureStore` (crux_kv): guarda chave e valor no lugar certo para a chave. */
fun interface KeyValuePort {
    suspend fun perform(operation: KeyValueOperation): KeyValueResult
}

/**
 * `Effect.Telemetry`, `Effect.Monitoring` e `Effect.Log`. Nenhuma devolve valor; quem
 * decide se o Core espera resolve é a ponte, não a porta.
 */
interface TelemetryPort {
    fun telemetry(operation: TelemetryOperation)

    fun monitoring(operation: MonitoringOperation)

    fun log(operation: LogOperation)
}

/** O relógio de parede. Trocado nos testes. */
fun interface WallClock {
    fun nowMillis(): Long

    companion object {
        val System = WallClock { java.lang.System.currentTimeMillis() }
    }
}

/** Pedidos do Core que a interface atende, e não uma porta: precisam da Activity. */
sealed interface ShellSignal {
    /** `Effect.StoreReview`: pedir a avaliação na loja. */
    data object RequestReview : ShellSignal
}
