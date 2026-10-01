package sh.logn.app.core

import android.content.Context
import android.util.Log
import com.posthog.PostHog
import com.posthog.PostHogOnFeatureFlags
import com.posthog.android.PostHogAndroid
import com.posthog.android.PostHogAndroidConfig
import sh.logn.app.BuildConfig
import sh.logn.core.LogN.LogLevel
import sh.logn.core.LogN.LogOperation
import sh.logn.core.LogN.MonitoringOperation
import sh.logn.core.LogN.TelemetryOperation
import sh.logn.coreshell.TelemetryPort

/**
 * PostHog, com a escolha do interruptor "Análise de uso". Espelho de Telemetry.swift e
 * dos handlers de telemetria de CoreWrapper.swift.
 *
 * A abertura e o fechamento do app são capturados pelo próprio SDK, e o Core não os
 * enxerga: por isso o interruptor chega até aqui. Desligado, o SDK sobe sem essa
 * captura, e o Core, do lado dele, para de identificar e mandar eventos de uso. Erros
 * seguem saindo, com identificador anônimo, como a política de privacidade descreve.
 *
 * Sem chave de telemetria (o padrão em desenvolvimento), nada inicia, e cada chamada vira
 * nada.
 */
class PostHogTelemetry(
    private val context: Context,
) : TelemetryPort {
    private var started = false

    fun start(analyticsEnabled: Boolean) {
        val key = BuildConfig.TELEMETRY_KEY
        val host = BuildConfig.POSTHOG_HOST
        if (key.isEmpty() || host.isEmpty()) {
            Log.i(TAG, "PostHog telemetry is disabled (no key provided)")
            return
        }
        val config =
            PostHogAndroidConfig(key, host).apply {
                captureApplicationLifecycleEvents = analyticsEnabled
                captureScreenViews = false
                captureDeepLinks = false
                sessionReplay = false
                // O crash do app vai para o Error Tracking, como em iOS e como a política
                // de privacidade descreve; independe do interruptor.
                errorTrackingConfig.autoCapture = true
                onFeatureFlags = PostHogOnFeatureFlags { android.os.Handler(android.os.Looper.getMainLooper()).post(FeatureFlags::refreshed) }
            }
        PostHogAndroid.setup(context, config)
        started = true
    }

    /** O interruptor mudou: encerra e inicia de novo, e vale na hora. */
    private fun apply(analyticsEnabled: Boolean) {
        if (started) PostHog.close()
        started = false
        start(analyticsEnabled)
    }

    override fun telemetry(operation: TelemetryOperation) {
        when (operation) {
            is TelemetryOperation.Identify -> if (started) PostHog.identify(operation.userId)
            is TelemetryOperation.Track -> if (started) PostHog.capture(operation.event, properties = operation.properties)
            is TelemetryOperation.Reset -> if (started) PostHog.reset()
            is TelemetryOperation.SetAnalyticsEnabled -> apply(operation.enabled)
        }
    }

    override fun monitoring(operation: MonitoringOperation) {
        if (!started) return
        when (operation) {
            // `$exception` em vez de um evento `error` solto: cai no Error Tracking, que
            // agrupa por mensagem. O erro nasceu no Core, e quem diz onde é a mensagem.
            is MonitoringOperation.LogError ->
                PostHog.capture(
                    EXCEPTION_EVENT,
                    properties =
                        mapOf(
                            "\$exception_type" to CORE_ERROR,
                            "\$exception_message" to operation.message,
                            "details" to operation.details,
                        ),
                )
            is MonitoringOperation.StartSpan -> PostHog.capture("span_started", properties = mapOf("span_name" to operation.name))
            is MonitoringOperation.EndSpan -> PostHog.capture("span_ended", properties = mapOf("span_name" to operation.name))
        }
    }

    // O SDK de Android não tem os logs do PostHog que o de iOS usa: o log do Core vai para
    // o logcat, só em debug (ADR 0023). Sem token, e-mail nem corpo: o Core já não os põe.
    override fun log(operation: LogOperation) {
        if (!BuildConfig.DEBUG) return
        val attributes = operation.attributes.entries.joinToString(" ") { "${it.key}=${it.value}" }
        val line = "${operation.message} $attributes".trim()
        when (operation.level) {
            LogLevel.DEBUG -> Log.d(TAG, line)
            LogLevel.INFO -> Log.i(TAG, line)
            LogLevel.WARN -> Log.w(TAG, line)
            LogLevel.ERROR -> Log.e(TAG, line)
        }
    }

    private companion object {
        const val TAG = "logn-android"
        const val EXCEPTION_EVENT = "\$exception"
        const val CORE_ERROR = "CoreError"
    }
}
