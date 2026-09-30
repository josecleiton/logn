package sh.logn.app.auth

import android.app.Activity
import android.content.Context
import android.net.Uri
import android.os.Handler
import android.os.Looper
import android.telephony.TelephonyManager
import android.util.Base64
import androidx.browser.customtabs.CustomTabsIntent
import androidx.core.net.toUri
import androidx.credentials.CredentialManager
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.credentials.exceptions.GetCredentialException
import androidx.credentials.exceptions.NoCredentialException
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import kotlinx.coroutines.CancellableContinuation
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withTimeoutOrNull
import sh.logn.app.BuildConfig
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Locale
import kotlin.coroutines.resume

/** Falha de login social. `Cancelled` é desistência, e não se mostra. */
sealed class SocialAuthFailure : Exception() {
    class Cancelled : SocialAuthFailure()

    class NotConfigured : SocialAuthFailure()

    class Failed : SocialAuthFailure()
}

/** 32 bytes aleatórios em base64url: o que o servidor espera do nonce, e o mínimo do PKCE. */
internal fun randomToken(): String {
    val bytes = ByteArray(TOKEN_BYTES).also { SecureRandom().nextBytes(it) }
    return base64Url(bytes)
}

internal fun base64Url(bytes: ByteArray): String = Base64.encodeToString(bytes, Base64.URL_SAFE or Base64.NO_WRAP or Base64.NO_PADDING)

internal fun sha256(text: String): ByteArray = MessageDigest.getInstance("SHA-256").digest(text.toByteArray())

internal fun sha256Hex(text: String): String = sha256(text).joinToString("") { "%02x".format(it) }

private const val TOKEN_BYTES = 32

/**
 * Login com o Google pelo Credential Manager (ADR 0023). O ID token sai com a audiência
 * do client web (`serverClientId`), que é a que o servidor aceita, e com o SHA-256 do
 * nonce; o Core recebe o token e o nonce cru, e o servidor confere os dois.
 *
 * Sem o `revoke` de iOS na exclusão da conta: o Credential Manager não entrega access
 * token, e revogar o consentimento fica com a pessoa, na conta Google dela.
 */
object GoogleAuth {
    data class Credential(
        val idToken: String,
        val nonce: String,
    )

    val isConfigured: Boolean get() = BuildConfig.GOOGLE_WEB_CLIENT_ID.endsWith(".apps.googleusercontent.com")

    private var inFlight = false

    suspend fun signIn(activity: Activity): Credential {
        if (!isConfigured) throw SocialAuthFailure.NotConfigured()
        // O toque repetido é desistência: o login que já está aberto segue.
        if (inFlight) throw SocialAuthFailure.Cancelled()
        inFlight = true
        try {
            val nonce = randomToken()
            // O botão "Continuar com o Google" do DS: a escolha de conta explícita, e não
            // a folha de login automático.
            val option =
                GetSignInWithGoogleOption
                    .Builder(BuildConfig.GOOGLE_WEB_CLIENT_ID)
                    .setNonce(sha256Hex(nonce))
                    .build()
            val request = GetCredentialRequest.Builder().addCredentialOption(option).build()
            val result =
                try {
                    CredentialManager.create(activity).getCredential(activity, request)
                } catch (_: GetCredentialCancellationException) {
                    throw SocialAuthFailure.Cancelled()
                } catch (_: NoCredentialException) {
                    // Com a escolha explícita de conta, não é desistência: é client mal
                    // cadastrado, conta ausente ou Play Services velho. Calado, o botão
                    // pareceria não fazer nada.
                    throw SocialAuthFailure.Failed()
                } catch (_: GetCredentialException) {
                    throw SocialAuthFailure.Failed()
                }
            val credential = result.credential
            if (credential !is CustomCredential ||
                credential.type != GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL
            ) {
                throw SocialAuthFailure.Failed()
            }
            val token = runCatching { GoogleIdTokenCredential.createFrom(credential.data).idToken }.getOrNull()
            if (token.isNullOrEmpty()) throw SocialAuthFailure.Failed()
            return Credential(token, nonce)
        } finally {
            inFlight = false
        }
    }
}

/**
 * Login com o GitHub (ADR 0019): OAuth com PKCE numa Custom Tab, voltando por
 * `logn://oauth/github`. Como em iOS, o código não é trocado aqui: o GitHub exige o secret
 * na troca, e o Core pede ao servidor.
 *
 * O retorno chega à `MainActivity` por `onNewIntent`, que o entrega a `complete`. A Custom
 * Tab fechada sem voltar é percebida no `onResume` seguinte, e vira desistência.
 */
object GitHubAuth {
    data class Credential(
        val code: String,
        val verifier: String,
        /** O nonce cru; o servidor recebe o SHA-256 dele na troca e o cru no login. */
        val nonce: String,
    )

    private const val AUTHORIZE = "https://github.com/login/oauth/authorize"
    private const val REDIRECT = "logn://oauth/github"

    val isConfigured: Boolean get() = BuildConfig.GITHUB_CLIENT_ID.isNotBlank()

    private class Pending(
        val state: String,
        val verifier: String,
        val nonce: String,
        val continuation: CancellableContinuation<Result<Credential>>,
    ) {
        var tabShown = false
    }

    private var pending: Pending? = null
    private val main = Handler(Looper.getMainLooper())

    suspend fun signIn(activity: Activity): Credential {
        if (!isConfigured) throw SocialAuthFailure.NotConfigured()
        if (pending != null) throw SocialAuthFailure.Cancelled()
        val verifier = randomToken()
        val state = randomToken()
        val nonce = randomToken()
        val url =
            AUTHORIZE
                .toUri()
                .buildUpon()
                .appendQueryParameter("client_id", BuildConfig.GITHUB_CLIENT_ID)
                .appendQueryParameter("redirect_uri", REDIRECT)
                // Só o id da conta, que não pede escopo, e a lista de e-mails. Nada de repositório.
                .appendQueryParameter("scope", "user:email")
                .appendQueryParameter("code_challenge", base64Url(sha256(verifier)))
                .appendQueryParameter("code_challenge_method", "S256")
                .appendQueryParameter("state", state)
                .appendQueryParameter("prompt", "select_account")
                .build()
        // Teto para o caso de nada voltar nem a Activity pausar (multijanela): sem ele, o
        // pedido ficava preso e todo toque seguinte era ignorado até o processo morrer.
        val result =
            withTimeoutOrNull(PENDING_TIMEOUT_MILLIS) {
                suspendCancellableCoroutine { cont ->
                    pending = Pending(state, verifier, nonce, cont)
                    cont.invokeOnCancellation { pending = null }
                    runCatching { CustomTabsIntent.Builder().setShowTitle(true).build().launchUrl(activity, url) }
                        .onFailure { finish(Result.failure(SocialAuthFailure.Failed())) }
                }
            } ?: Result.failure(SocialAuthFailure.Cancelled())
        return result.getOrThrow()
    }

    /**
     * O retorno `logn://oauth/github?…`. Devolve se era dele.
     *
     * Qualquer app do aparelho pode mandar esse link. Sem o `state` deste pedido, ele é
     * ignorado, sem desfazer o login em curso: nem o código entra, nem uma recusa forjada
     * cancela. O GitHub devolve o `state` também na recusa.
     */
    fun complete(uri: Uri): Boolean {
        if (uri.scheme != "logn" || uri.host != "oauth" || uri.path != "/github") return false
        val p = pending ?: return true
        if (uri.getQueryParameter("state") != p.state) return true
        // Recusa na tela do GitHub volta com `access_denied`: é desistência.
        if (uri.getQueryParameter("error") == "access_denied") {
            finish(Result.failure(SocialAuthFailure.Cancelled()))
            return true
        }
        val code = uri.getQueryParameter("code")
        if (code.isNullOrEmpty()) {
            finish(Result.failure(SocialAuthFailure.Failed()))
            return true
        }
        finish(Result.success(Credential(code, p.verifier, p.nonce)))
        return true
    }

    /** A Activity voltou à frente. Sem retorno logo depois, a aba foi fechada à mão. */
    fun onHostResumed() {
        val p = pending ?: return
        if (!p.tabShown) return
        main.postDelayed({
            if (pending === p) finish(Result.failure(SocialAuthFailure.Cancelled()))
        }, CLOSED_TAB_GRACE_MILLIS)
    }

    /** A Activity saiu da frente: é a Custom Tab abrindo. */
    fun onHostPaused() {
        pending?.tabShown = true
    }

    private fun finish(result: Result<Credential>) {
        val p = pending ?: return
        pending = null
        if (p.continuation.isActive) p.continuation.resume(result)
    }

    private const val CLOSED_TAB_GRACE_MILLIS = 600L
    private const val PENDING_TIMEOUT_MILLIS = 10 * 60 * 1000L
}

/**
 * O país da confirmação de idade: o do chip, depois o da rede, depois a região do
 * aparelho. O servidor aceita alfa-2 e grava alfa-2.
 */
object DeviceCountry {
    fun current(context: Context): String {
        val telephony = context.getSystemService(TelephonyManager::class.java)
        val candidates = listOf(telephony?.simCountryIso, telephony?.networkCountryIso, Locale.getDefault().country)
        return candidates.firstOrNull { !it.isNullOrBlank() && it.length == 2 }?.uppercase(Locale.ROOT).orEmpty()
    }
}
