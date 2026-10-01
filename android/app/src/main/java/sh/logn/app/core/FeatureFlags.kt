package sh.logn.app.core

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.setValue
import com.posthog.PostHog

/**
 * As chaves de cada provedor de login, do PostHog: se o login quebrar do lado do Google
 * ou do GitHub, a flag some com o botão sem versão nova, e quem entra por e-mail segue.
 * Sem PostHog (debug sem chave), tudo desligado, como em iOS.
 */
object FeatureFlags {
    /** Muda quando as flags chegam: na primeira abertura e depois de sair, chegam tarde. */
    var version by mutableIntStateOf(0)
        private set

    fun refreshed() {
        version++
    }

    fun isEnabled(key: String): Boolean = runCatching { PostHog.isFeatureEnabled(key) }.getOrDefault(false)

    const val GOOGLE = "sso_google_enabled"
    const val GITHUB = "sso_github_enabled"
}
