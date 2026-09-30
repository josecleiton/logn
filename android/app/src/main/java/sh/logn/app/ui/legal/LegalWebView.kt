package sh.logn.app.ui.legal

import android.annotation.SuppressLint
import android.content.Context
import android.content.Intent
import android.os.Handler
import android.os.Looper
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.net.toUri
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONObject
import sh.logn.app.BuildConfig
import sh.logn.app.ui.theme.LegalMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.coreshell.i18n.Str
import java.util.concurrent.TimeUnit

/** Termos de uso ou política de privacidade. O `path` é o da rota `/legal/<path>`. */
enum class LegalKind(
    val path: String,
) {
    Terms("terms"),
    Privacy("privacy"),
    ;

    fun title(context: Context): String =
        when (this) {
            Terms -> Str.Legal.terms_title(context)
            Privacy -> Str.Legal.privacy_title(context)
        }

    companion object {
        fun fromPath(path: String): LegalKind? = entries.firstOrNull { "/legal/${it.path}" == path }
    }
}

/** Versão, vigência e origem do documento na tela. */
data class LegalMeta(
    val version: Int,
    /** `AAAA-MM-DD`, no fuso do Brasil, como o servidor manda. */
    val effectiveAt: String,
    val draft: Boolean,
    /** Veio da cópia dos assets, e não do servidor. */
    val offline: Boolean,
)

enum class LegalLoadPhase { Loading, Loaded, Failed }

/**
 * O documento no modo do app, do servidor com `?embed=1`. Espelho de LegalWebView.swift.
 *
 * Sem rede, ou com o servidor respondendo outra coisa que 200, cai na cópia dos assets
 * (`just android/seed`). A página do servidor é buscada aqui, por OkHttp, e entregue ao
 * WebView com os cabeçalhos dela (inclusive a CSP): é o único jeito de ler a versão que
 * vem em `X-LogN-Legal-*`. Link para o outro documento troca o documento; `mailto:` e
 * `https` vão para o sistema; nada mais navega. JavaScript desligado: a página não tem
 * script, e não tem por que poder rodar um.
 */
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun LegalWebView(
    kind: LegalKind,
    locale: String,
    highlight: List<String>,
    reloadToken: Int,
    onMeta: (LegalMeta?) -> Unit,
    onPhase: (LegalLoadPhase) -> Unit,
    onSwitch: (LegalKind) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val fontScale = LocalDensity.current.fontScale
    val loader =
        remember {
            LegalLoader(context.applicationContext)
        }
    loader.callbacks = LegalLoader.Callbacks(onMeta, onPhase, onSwitch)
    AndroidView(
        modifier = modifier,
        factory = { ctx ->
            WebView(ctx).apply {
                settings.javaScriptEnabled = false
                settings.allowFileAccess = false
                settings.allowContentAccess = false
                settings.domStorageEnabled = false
                settings.setSupportZoom(false)
                // Sem isto a página pisca branco antes do CSS escuro chegar.
                setBackgroundColor(LognDark.surfaceRaised.toArgb())
                isLongClickable = false
                webViewClient = loader.client
            }
        },
        update = { web ->
            web.settings.textZoom = (fontScale * PERCENT).toInt().coerceAtMost(LegalMetrics.MAX_TEXT_ZOOM)
            loader.loadIfNeeded(web, kind, locale, highlight, reloadToken)
        },
        // Sem isto, cada documento aberto deixava um WebView preso ao contexto.
        onRelease = { web ->
            loader.callbacks = null
            web.stopLoading()
            web.destroy()
        },
    )
}

private class LegalLoader(
    private val context: Context,
) {
    data class Callbacks(
        val onMeta: (LegalMeta?) -> Unit,
        val onPhase: (LegalLoadPhase) -> Unit,
        val onSwitch: (LegalKind) -> Unit,
    )

    var callbacks: Callbacks? = null
    private val main = Handler(Looper.getMainLooper())
    private var loaded: Triple<LegalKind, Int, String>? = null
    private var kind = LegalKind.Terms
    private var locale = ""

    /** Já caiu na cópia dos assets nesta carga: uma segunda falha é falha de vez. */
    private var usingBundle = false

    /** A URL do servidor desta carga; só ela passa por OkHttp. */
    private var remoteUrl: String? = null

    fun loadIfNeeded(
        web: WebView,
        kind: LegalKind,
        locale: String,
        highlight: List<String>,
        token: Int,
    ) {
        val key = Triple(kind, token, locale)
        if (loaded == key) return
        loaded = key
        this.kind = kind
        this.locale = locale
        usingBundle = false
        report(LegalLoadPhase.Loading, null, clearMeta = true)
        val url = remoteUrl(kind, locale, highlight)
        remoteUrl = url
        if (url == null) loadBundled(web) else web.loadUrl(url)
    }

    private fun remoteUrl(
        kind: LegalKind,
        locale: String,
        highlight: List<String>,
    ): String? {
        val base = BuildConfig.API_BASE_URL.takeIf { it.isNotEmpty() } ?: return null
        val builder =
            base
                .toUri()
                .buildUpon()
                .path("/legal/${kind.path}")
                .appendQueryParameter("embed", "1")
                .appendQueryParameter("lang", locale)
        if (highlight.isNotEmpty()) builder.appendQueryParameter("highlight", highlight.joinToString(","))
        return builder.build().toString()
    }

    private fun loadBundled(web: WebView) {
        usingBundle = true
        remoteUrl = null
        val name = "legal/legal-${kind.path}.$locale.html"
        val html = runCatching { context.assets.open(name).bufferedReader().use { it.readText() } }.getOrNull()
        if (html == null) {
            report(LegalLoadPhase.Failed, null)
            return
        }
        report(LegalLoadPhase.Loading, bundledMeta())
        // Base num domínio que não existe (`.invalid`): a cópia não tem de onde carregar
        // nada, e o link para o outro documento chega ao `shouldOverrideUrlLoading` com o
        // caminho relativo resolvido.
        web.loadDataWithBaseURL(BUNDLE_BASE, html, "text/html", "utf-8", null)
    }

    private fun bundledMeta(): LegalMeta? =
        runCatching {
            val manifest =
                JSONObject(context.assets.open("legal/legal-manifest.json").bufferedReader().use { it.readText() })
            val entry = manifest.getJSONObject(kind.path).getJSONObject(locale)
            LegalMeta(entry.getInt("version"), entry.getString("effective_at"), entry.getBoolean("draft"), offline = true)
        }.getOrNull()

    private fun report(
        phase: LegalLoadPhase,
        meta: LegalMeta?,
        clearMeta: Boolean = false,
    ) {
        main.post {
            val cb = callbacks ?: return@post
            cb.onPhase(phase)
            if (clearMeta || meta != null) cb.onMeta(meta)
        }
    }

    val client =
        object : WebViewClient() {
            override fun shouldInterceptRequest(
                view: WebView,
                request: WebResourceRequest,
            ): WebResourceResponse? {
                val url = request.url.toString()
                if (!request.isForMainFrame || url != remoteUrl) return null
                return fetch(view, url)
            }

            override fun shouldOverrideUrlLoading(
                view: WebView,
                request: WebResourceRequest,
            ): Boolean {
                val uri = request.url
                LegalKind.fromPath(uri.path.orEmpty())?.let { other ->
                    main.post { callbacks?.onSwitch?.invoke(other) }
                    return true
                }
                if (uri.scheme == "mailto" || uri.scheme == "https") {
                    runCatching {
                        context.startActivity(Intent(Intent.ACTION_VIEW, uri).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                    }
                }
                return true
            }

            override fun onPageFinished(
                view: WebView,
                url: String?,
            ) {
                // A página vazia que respondeu no lugar do servidor também termina, e
                // ela não é o documento: só vale o fim da cópia.
                if (usingBundle && url?.startsWith(BUNDLE_BASE) != true) return
                report(LegalLoadPhase.Loaded, null)
            }

            override fun onReceivedError(
                view: WebView,
                request: WebResourceRequest,
                error: WebResourceError,
            ) {
                if (!request.isForMainFrame) return
                if (usingBundle) report(LegalLoadPhase.Failed, null) else main.post { loadBundled(view) }
            }
        }

    /** Busca a página do servidor e a devolve ao WebView com os cabeçalhos dela. */
    private fun fetch(
        view: WebView,
        url: String,
    ): WebResourceResponse? {
        val response =
            runCatching {
                http
                    .newCall(
                        Request
                            .Builder()
                            .url(url)
                            .build(),
                    ).execute()
            }.getOrNull()
        if (response == null || response.code != HTTP_OK) {
            // 503 de rascunho, 404, 500, sem rede: melhor a cópia do aparelho que uma
            // página de erro crua.
            response?.close()
            main.post { loadBundled(view) }
            return WebResourceResponse("text/html", "utf-8", ByteArrayEmpty.inputStream())
        }
        val header = { name: String -> response.header(name) }
        val version = header("X-LogN-Legal-Version")?.toIntOrNull()
        val effective = header("X-LogN-Legal-Effective")
        if (version != null && effective != null) {
            report(LegalLoadPhase.Loading, LegalMeta(version, effective, header("X-LogN-Legal-Draft") == "1", offline = false))
        }
        val headers = response.headers.names().associateWith { response.header(it).orEmpty() }
        val type = response.body.contentType()
        return WebResourceResponse(
            "${type?.type ?: "text"}/${type?.subtype ?: "html"}",
            type?.charset()?.name() ?: "utf-8",
            response.code,
            "OK",
            headers,
            response.body.byteStream(),
        )
    }

    private companion object {
        const val HTTP_OK = 200
        const val BUNDLE_BASE = "https://legal.invalid/"
        val ByteArrayEmpty = ByteArray(0)
        val http: OkHttpClient =
            OkHttpClient
                .Builder()
                .connectTimeout(10, TimeUnit.SECONDS)
                .callTimeout(15, TimeUnit.SECONDS)
                .build()
    }
}

private const val PERCENT = 100
