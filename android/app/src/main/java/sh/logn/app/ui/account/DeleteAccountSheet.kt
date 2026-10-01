package sh.logn.app.ui.account

import android.app.Activity
import androidx.activity.compose.LocalActivity
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import kotlinx.coroutines.launch
import sh.logn.app.auth.GitHubAuth
import sh.logn.app.auth.GoogleAuth
import sh.logn.app.auth.SocialAuthFailure
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.ShellState
import sh.logn.app.ui.auth.LinkText
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.components.DestructiveButton
import sh.logn.app.ui.components.FieldKind
import sh.logn.app.ui.components.LognTextField
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.NoticeMetrics
import sh.logn.app.ui.theme.Space
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * A palavra que confirma a exclusão. É a mesma em toda língua, de propósito: o texto de
 * cada uma pede "a palavra EXCLUIR".
 */
private const val CONFIRM_WORD = "EXCLUIR"

/**
 * Excluir a conta: senha e a palavra, ou a confirmação no provedor, para conta que
 * nasceu dele e não tem senha. Espelho de DeleteAccountSheet (LogoutViews.swift). O
 * servidor só aceita login de provedor de poucos minutos atrás.
 */
@Composable
fun DeleteAccountSheet(
    view: ViewModel,
    onDismiss: () -> Unit,
) {
    val context = LocalContext.current
    val activity = LocalActivity.current
    val dispatch = LocalDispatch.current
    val scope = ShellState.scope
    var password by remember { mutableStateOf("") }
    var confirmation by remember { mutableStateOf("") }
    var providerBusy by remember { mutableStateOf(false) }
    val confirmed = confirmation == CONFIRM_WORD
    val canDelete = password.isNotEmpty() && confirmed

    fun withProvider(block: suspend (Activity) -> Unit) {
        val host = activity ?: return
        providerBusy = true
        scope.launch {
            try {
                block(host)
                onDismiss()
            } catch (_: SocialAuthFailure.Cancelled) {
                // Desistiu na janela do provedor: a conta fica.
            } catch (_: SocialAuthFailure) {
                dispatch(Event.SocialLoginFailed)
            } finally {
                providerBusy = false
            }
        }
    }

    BottomSheet(onDismiss = onDismiss, dismissible = !providerBusy) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(Space.screenMargin),
            verticalArrangement = Arrangement.spacedBy(Space.screenMargin),
        ) {
            Text(
                Str.Logout.delete_account(context),
                style = LognFont.sans(NoticeMetrics.TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
            )
            Text(Str.Logout.delete_desc(context, view.globalXp), style = LognFont.sans(NoticeMetrics.BODY_SIZE), color = LognDark.textSecondary)
            Text(
                Str.Logout.grace_period(context),
                style = LognFont.sans(NoticeMetrics.BODY_SIZE, FontWeight.SemiBold),
                color = LognDark.wrongInk,
            )
            LognTextField(password, { password = it }, Str.Login.password_prompt(context), FieldKind.Password)
            LognTextField(confirmation, { confirmation = it }, Str.Logout.delete_confirm_placeholder(context), FieldKind.Text)
            DestructiveButton(Str.Logout.delete_button(context), filled = true, enabled = canDelete) {
                dispatch(Event.DeleteAccount(password))
                onDismiss()
            }
            if (GoogleAuth.isConfigured) {
                ProviderLink(Str.Logout.delete_with_google(context), confirmed && !providerBusy) {
                    withProvider { activity ->
                        val credential = GoogleAuth.signIn(activity)
                        dispatch(Event.DeleteAccountWithProvider("google", credential.idToken, credential.nonce, ""))
                    }
                }
            }
            if (GitHubAuth.isConfigured) {
                ProviderLink(Str.Logout.delete_with_github(context), confirmed && !providerBusy) {
                    withProvider { activity ->
                        val credential = GitHubAuth.signIn(activity)
                        // O servidor troca o código e revoga no GitHub; o aparelho não faz nada.
                        dispatch(Event.DeleteAccountWithGitHub(credential.code, credential.verifier, credential.nonce))
                    }
                }
            }
        }
    }
}

@Composable
private fun ProviderLink(
    title: String,
    enabled: Boolean,
    onClick: () -> Unit,
) {
    LinkText(
        title,
        if (enabled) LognDark.textSecondary else LognDark.textDim,
        Modifier.fillMaxWidth(),
        enabled = enabled,
        onClick = onClick,
    )
}
