package sh.logn.app.ui.auth

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import sh.logn.app.auth.DeviceCountry
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognCheckbox
import sh.logn.app.ui.components.StatusLine
import sh.logn.app.ui.legal.LegalKind
import sh.logn.app.ui.legal.LegalLinksRow
import sh.logn.app.ui.theme.LoginMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.NoticeMetrics
import sh.logn.app.ui.theme.Space
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * Primeiro login por um provedor: a conta ainda não existe (ADR 0016). Pede o que o
 * cadastro por e-mail pede, menos e-mail, código e senha: a idade mínima do país e o
 * aceite dos documentos vigentes. Espelho de SocialSignupSheet.
 */
@Composable
fun SocialSignupSheet(
    view: ViewModel,
    onLegal: (LegalKind) -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    var ageConfirmed by remember { mutableStateOf(false) }
    var termsAccepted by remember { mutableStateOf(false) }
    val country = remember { DeviceCountry.current(context) }
    val locked = view.authCooldownSeconds > 0u
    val canContinue = ageConfirmed && termsAccepted && view.legalVersionsReady && !locked

    LaunchedEffect(Unit) { dispatch(Event.FetchLegalVersions(country)) }
    LaunchedEffect(view.minAge) { ageConfirmed = false }

    // Fechar pela pessoa é o mesmo que "Agora não"; com o pedido no ar, não fecha.
    BottomSheet(onDismiss = { dispatch(Event.CancelSocialSignup) }, dismissible = !view.isAuthenticating) {
        Column(Modifier.fillMaxWidth().padding(Space.screenMargin), verticalArrangement = Arrangement.spacedBy(Space.lg)) {
            Text(
                Str.Login.google_signup_title(context),
                style = LognFont.sans(NoticeMetrics.TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
            )
            Text(Str.Login.google_signup_reason(context), style = LognFont.sans(NoticeMetrics.BODY_SIZE), color = LognDark.textSecondary)
            LognCheckbox(ageConfirmed, Str.Register.age_confirmation(context, view.minAge.toInt())) { ageConfirmed = !ageConfirmed }
            LognCheckbox(termsAccepted, Str.Register.terms_confirmation(context)) {
                termsAccepted = !termsAccepted
                if (termsAccepted && !view.legalVersionsReady && country.isNotEmpty()) dispatch(Event.FetchLegalVersions(country))
            }
            LegalLinksRow(onOpen = onLegal)
            if (termsAccepted && !view.legalVersionsReady) {
                Text(Str.Register.legal_pending(context), style = LognFont.sans(LoginMetrics.CHECK_SIZE), color = LognDark.textMuted)
            }
            StatusLine(view.status)
            val continueLabel =
                if (locked) {
                    Str.Status.wait_seconds(context, view.authCooldownSeconds.toInt())
                } else {
                    Str.Login.google_signup_continue(context)
                }
            LognButton(
                continueLabel,
                ButtonVariant.Primary,
                enabled = canContinue && !view.isAuthenticating,
            ) { dispatch(Event.CompleteSocialSignup(ageConfirmed, termsAccepted)) }
            LinkText(
                Str.Login.google_signup_cancel(context),
                LognDark.textSecondary,
                Modifier.fillMaxWidth(),
            ) { dispatch(Event.CancelSocialSignup) }
        }
    }
}
