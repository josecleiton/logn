package sh.logn.coreshell

import android.os.Looper
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import sh.logn.core.LogN.Effect
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.core.Requests
import sh.logn.ffi.CoreFFI

/**
 * A ponte com o Core Rust (ADR 0023): manda `Event`, recebe pedidos de efeito, resolve
 * cada um e publica o `ViewModel`. Espelho de `CoreWrapper.swift`.
 *
 * Tudo que toca o `CoreFFI` roda na thread principal: o Core não é reentrante, e a
 * ordem dos resolves é a ordem em que chegam. As portas trocam de thread sozinhas.
 *
 * O FFI devolve bytes vazios quando o Core entra em pânico (o `catch_unwind` da ponte)
 * ou quando os tipos gerados não batem com o `.so`. Isso vai para `onBridgeFailure`, e
 * nada é decodificado.
 */
class Core(
    private val scope: CoroutineScope,
    private val http: HttpPort,
    private val store: KeyValuePort,
    private val telemetry: TelemetryPort,
    clock: WallClock = WallClock.System,
    private val onBridgeFailure: (operation: String) -> Unit,
) : AutoCloseable {
    private val ffi = CoreFFI()
    private val timers = TimerRegistry(scope, clock) { id, response -> resolve(id, response.bincodeSerialize()) }

    private val viewState = MutableStateFlow(readView() ?: error("o Core não devolveu o ViewModel inicial"))
    val view: StateFlow<ViewModel> = viewState.asStateFlow()

    private val signalFlow = MutableSharedFlow<ShellSignal>(extraBufferCapacity = 8)
    val signals: SharedFlow<ShellSignal> = signalFlow.asSharedFlow()

    fun update(event: Event) {
        assertMainThread()
        process("update", ffi.update(event.bincodeSerialize()))
    }

    private fun resolve(
        requestId: UInt,
        payload: ByteArray,
    ) {
        assertMainThread()
        process("resolve", ffi.resolve(requestId, payload))
    }

    private fun process(
        operation: String,
        bytes: ByteArray,
    ) {
        if (bytes.isEmpty()) {
            onBridgeFailure(operation)
            return
        }
        for (request in Requests.bincodeDeserialize(bytes).value) dispatch(request.id, request.effect)
    }

    // Exaustivo e sem `else`: efeito novo no Core quebra este build, em vez de travar
    // o Core à espera de um resolve que nunca vem.
    private fun dispatch(
        requestId: UInt,
        effect: Effect,
    ) {
        when (effect) {
            is Effect.Render -> readView()?.let { viewState.value = it }
            is Effect.Http ->
                scope.launch { resolve(requestId, http.perform(effect.value).bincodeSerialize()) }
            is Effect.SecureStore ->
                scope.launch { resolve(requestId, store.perform(effect.value).bincodeSerialize()) }
            // request_from_shell: o Core espera o resolve, com payload vazio, senão trava.
            // Vai pela fila, como em iOS. `Dispatchers.Main`, e não o `immediate` do escopo:
            // o `immediate` rodaria o resolve aqui dentro, antes do resto do lote.
            is Effect.Telemetry -> {
                telemetry.telemetry(effect.value)
                scope.launch(Dispatchers.Main) { resolve(requestId, ByteArray(0)) }
            }
            // notify_shell: nunca resolve.
            is Effect.Monitoring -> telemetry.monitoring(effect.value)
            is Effect.Log -> telemetry.log(effect.value)
            is Effect.StoreReview -> signalFlow.tryEmit(ShellSignal.RequestReview)
            is Effect.Time -> timers.handle(requestId, effect.value)
        }
    }

    private fun readView(): ViewModel? {
        val bytes = ffi.view()
        if (bytes.isEmpty()) {
            onBridgeFailure("view")
            return null
        }
        return ViewModel.bincodeDeserialize(bytes)
    }

    private fun assertMainThread() {
        check(Looper.myLooper() == Looper.getMainLooper()) { "o Core só é chamado na thread principal" }
    }

    override fun close() = ffi.close()
}
