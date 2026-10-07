package sh.logn.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import sh.logn.app.auth.GitHubAuth
import sh.logn.app.ui.LognRoot
import sh.logn.app.ui.theme.LognTheme
import sh.logn.core.LogN.Event

class MainActivity : ComponentActivity() {
    private val core get() = (application as LognApplication).core

    override fun onCreate(savedInstanceState: Bundle?) {
        // A splash do sistema só cobre o primeiro quadro: a de verdade, com o log da
        // abertura, é a SplashScreen do Compose, que começa no mesmo balão.
        installSplashScreen()
        super.onCreate(savedInstanceState)
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
            navigationBarStyle = SystemBarStyle.dark(android.graphics.Color.TRANSPARENT),
        )
        setContent {
            LognTheme {
                LognRoot(core, (application as LognApplication).store)
            }
        }
        if (savedInstanceState == null) handleIntent(intent)
        if (BuildConfig.DEBUG) handleDebugExtras(intent)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        // Sem isto, reabrir pelos recentes entrega de novo o link antigo, com um OTP já
        // consumido.
        setIntent(intent)
        handleIntent(intent)
    }

    override fun onResume() {
        super.onResume()
        GitHubAuth.onHostResumed()
    }

    override fun onPause() {
        super.onPause()
        GitHubAuth.onHostPaused()
    }

    /**
     * O botão do e-mail (`https://logn.sh/app/verify`, `/app/reset-password`),
     * `logn://verify`, `logn://reset-password` e o retorno do GitHub, como
     * `handleIncomingURL` de iOS.
     */
    private fun handleIntent(intent: Intent?) {
        val uri: Uri = intent?.data ?: return
        val (action, params) = when (uri.scheme) {
            "logn" -> {
                if (GitHubAuth.complete(uri)) return
                uri.host to uri
            }
            // App Link (ADR 0028): os caminhos do manifest e nenhum outro, já que um
            // intent explícito não passa pelo filtro. Os parâmetros vêm no fragmento.
            "https" -> {
                val action = APP_LINKS[uri.path] ?: return
                if (uri.host != "logn.sh") return
                action to Uri.Builder().encodedQuery(uri.encodedFragment).build()
            }
            else -> return
        }
        val code = params.getQueryParameter("code").orEmpty()
        val email = params.getQueryParameter("email").orEmpty()
        if (code.isEmpty() || email.isEmpty()) return
        val purpose = when (action) {
            "verify" -> params.getQueryParameter("purpose") ?: "verify_email"
            "reset-password" -> "reset_password"
            else -> return
        }
        // Quem decide se o link vale é o Core: só com pedido aberto neste app para o mesmo
        // e-mail e propósito. Aceito, ele volta em `otpLink`, e a tela reage.
        core.update(Event.OpenOTPLink(email, code, purpose))
    }

    // Atalho de inspeção visual, o `-LogNStartAsGuest` de iOS:
    // `adb shell am start -n sh.logn.app/.MainActivity --ez logn.start_as_guest true`.
    private fun handleDebugExtras(intent: Intent?) {
        if (intent?.getBooleanExtra(EXTRA_START_AS_GUEST, false) != true) return
        val view = core.view.value
        if (!view.isGuest && !view.hasSession) core.update(Event.ContinueAsGuest)
    }

    private companion object {
        const val EXTRA_START_AS_GUEST = "logn.start_as_guest"

        /** Caminho do App Link → ação, nas três línguas da landing, como no manifest. */
        val APP_LINKS = listOf("", "/en", "/es").flatMap { prefix ->
            listOf("verify", "reset-password").map { "$prefix/app/$it" to it }
        }.toMap()
    }
}
