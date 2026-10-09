package sh.logn.app

import android.app.Application
import android.content.Context
import android.util.Log
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import sh.logn.app.core.DeviceKeyValueStore
import sh.logn.app.core.OkHttpPort
import sh.logn.app.core.PlayStore
import sh.logn.app.core.PostHogTelemetry
import sh.logn.core.LogN.Event
import sh.logn.coreshell.Core
import java.io.File
import java.util.TimeZone
import java.util.UUID

/**
 * Dono do Core: um por processo, vivo enquanto o app vive, como o `@StateObject` de
 * LogNiOSApp.swift. A Activity só o observa; girar a tela ou recriá-la não reinicia a
 * sessão.
 */
class LognApplication : Application() {
    lateinit var core: Core
        private set

    /** A loja: começa com o app, não com a tela de compra (spec, seção 5). */
    lateinit var store: PlayStore
        private set

    override fun onCreate() {
        super.onCreate()
        val telemetry = PostHogTelemetry(this)
        // Com a escolha do interruptor "Análise de uso" guardada no aparelho.
        telemetry.start(analyticsEnabled = analyticsEnabled())

        val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
        // Cache HTTP no diretório de cache: o sistema pode limpar, e o app só baixa de novo.
        val httpClient = OkHttpPort.defaultClient(File(cacheDir, "http"))
        core =
            Core(
                scope = scope,
                http = OkHttpPort(BuildConfig.API_BASE_URL, httpClient),
                // A conta saindo leva o cache HTTP: o conteúdo dela não fica no disco.
                store = DeviceKeyValueStore(this) { httpClient.cache?.evictAll() },
                telemetry = telemetry,
                onBridgeFailure = { Log.e(TAG, "Core bridge returned no bytes on $it") },
            )
        startCore()
        // A compra que ficou sem confirmar volta na abertura, antes de qualquer tela.
        store = PlayStore(this, scope).also { it.attach(core) }
    }

    /** A mesma sequência de abertura de CoreWrapper.swift, na mesma ordem. */
    private fun startCore() {
        // A língua antes de qualquer pedido: é o `Accept-Language` de cada um e a língua
        // do aceite no cadastro.
        core.update(Event.SetLocale(AppLocale.current(this)))
        // Versão e plataforma vão no registro do aceite dos termos (ADR 0020).
        core.update(Event.SetClientInfo(BuildConfig.VERSION_NAME, PLATFORM))
        core.update(Event.RestoreAnalyticsPreference)
        core.update(Event.SetUtcOffset(TimeZone.getDefault().getOffset(System.currentTimeMillis()) / MILLIS_PER_SECOND))
        core.update(Event.RestorePreferences)
        // O `X-Device-ID` da licença da trilha paga. Aleatório por instalação, nunca o
        // id de publicidade nem o ANDROID_ID (política de privacidade, seção do aparelho).
        core.update(Event.SetDeviceId(deviceId()))
        loadBundledTrail()
        // O Core não tem relógio: sem a hora, ele não decide se a sessão guardada ainda
        // vale quando não há rede.
        core.update(Event.Tick(System.currentTimeMillis() / MILLIS_PER_SECOND))
        core.update(Event.StartBoot)
    }

    /**
     * `trail-seed.json` dos assets, gerado por `just android/seed` a partir da própria
     * API. Fora do git (regra 8); sem ele o app segue buscando pela rede.
     */
    private fun loadBundledTrail() {
        val json =
            runCatching { assets.open(SEED_ASSET).bufferedReader().use { it.readText() } }
                .getOrElse {
                    Log.w(TAG, "$SEED_ASSET is not in the assets; run `just android/seed`")
                    return
                }
        core.update(Event.BundledTrailLoaded(json))
    }

    private fun deviceId(): String {
        val file = File(filesDir, DEVICE_ID_FILE)
        val stored = if (file.exists()) file.readText().trim() else ""
        if (stored.isNotEmpty()) return stored
        val id = UUID.randomUUID().toString().uppercase()
        file.writeText(id)
        return id
    }

    // A mesma chave que o Core grava pelo `SecureStore`: "1" é desligado.
    private fun analyticsEnabled(): Boolean {
        val stored =
            getSharedPreferences(DeviceKeyValueStore.PLAIN_FILE, Context.MODE_PRIVATE)
                .getString(ANALYTICS_DISABLED_KEY, null)
                ?.let(DeviceKeyValueStore::decode)
        return stored?.decodeToString() != "1"
    }

    private companion object {
        const val TAG = "logn-android"
        const val PLATFORM = "android"
        const val SEED_ASSET = "trail-seed.json"
        const val DEVICE_ID_FILE = "device_id"
        const val ANALYTICS_DISABLED_KEY = "analytics_disabled"
        const val MILLIS_PER_SECOND = 1000
    }
}

/** A língua do app, do recurso que cada `values-*` declara (tools_i18n). */
object AppLocale {
    fun current(context: Context): String = context.getString(sh.logn.coreshell.R.string.i18n_locale)
}
