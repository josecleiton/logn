package sh.logn.app

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
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
                LognRoot(core)
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

    /** `logn://verify` e `logn://reset-password`, como `handleIncomingURL` de iOS. */
    private fun handleIntent(intent: Intent?) {
        val uri: Uri = intent?.data ?: return
        if (uri.scheme != "logn") return
        val code = uri.getQueryParameter("code").orEmpty()
        val email = uri.getQueryParameter("email").orEmpty()
        if (code.isEmpty() || email.isEmpty()) return
        when (uri.host) {
            "verify" -> {
                val purpose = uri.getQueryParameter("purpose") ?: "verify_email"
                core.update(Event.VerifyOTP(email, code, purpose))
            }
            // A tela de nova senha chega com as telas de login (fase 2).
            "reset-password" -> Unit
        }
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
    }
}
