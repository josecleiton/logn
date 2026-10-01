package sh.logn.app.ui.auth

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.FieldKind
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.components.LognTextField
import sh.logn.app.ui.components.StatusLine
import sh.logn.app.ui.theme.FieldMetrics
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Space
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

private const val CODE_LENGTH = 6

/**
 * Nova senha. Espelho de ResetPasswordView.swift. Com o código do link do e-mail, só as
 * senhas; pelo "Esqueci a senha", o campo do código também, num campo só: aqui quem
 * consome o código é a própria troca, e não a conferência das seis caixas.
 */
@Composable
fun ResetPasswordScreen(
    view: ViewModel,
    email: String,
    linkCode: String,
    onClose: () -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val asksForCode = linkCode.isEmpty()
    var typed by rememberSaveable { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var confirm by remember { mutableStateOf("") }
    val code = if (asksForCode) typed else linkCode
    val resetLocked = view.authCooldownSeconds > 0u
    val resendLocked = view.resendCooldownSeconds > 0u
    val passwordsMatch = confirm.isEmpty() || password == confirm
    val canSubmit = code.length == CODE_LENGTH && password.length >= MIN_PASSWORD && password == confirm && !resetLocked
    BackHandler(onBack = onClose)

    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.canvas)
            .safeDrawingPadding()
            .imePadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = Space.screenMargin, vertical = Space.xl),
        verticalArrangement = Arrangement.spacedBy(Space.xl),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        // Azul é informação; o que a tela pede é ação, e ação é o acento.
        Icon(LognIcon.KeyFill, LognDark.accentInk, IconMetrics.hero)
        Text(Str.Reset.title(context), style = LognFont.headlineMedium, color = LognDark.textPrimary)
        Text(
            if (asksForCode) Str.Reset.subtitle_with_code(context, email) else Str.Reset.subtitle(context, email),
            style = LognFont.bodyLarge,
            color = LognDark.textSecondary,
            textAlign = TextAlign.Center,
        )
        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(Space.md)) {
            if (asksForCode) {
                LognTextField(
                    typed,
                    { typed = it.filter(Char::isDigit).take(CODE_LENGTH) },
                    Str.Reset.code_prompt(context),
                    FieldKind.Number,
                    textStyle = LognFont.mono(FieldMetrics.CODE_TEXT_SIZE, FontWeight.SemiBold),
                )
                LinkText(
                    if (resendLocked) Str.Status.wait_seconds(context, view.resendCooldownSeconds.toInt()) else Str.Otp.resend_code(context),
                    if (resendLocked) LognDark.textDim else LognDark.textSecondary,
                    Modifier.align(Alignment.CenterHorizontally),
                    enabled = !resendLocked,
                    underline = !resendLocked,
                ) { dispatch(Event.RequestOTP(email, "reset_password")) }
            }
            LognTextField(password, { password = it }, Str.Reset.password_prompt(context), FieldKind.NewPassword)
            LognTextField(
                confirm,
                { confirm = it },
                Str.Reset.confirm_prompt(context),
                FieldKind.NewPassword,
                error = !passwordsMatch,
                errorColor = LognDark.warn,
            )
            // Verde é veredito do juiz, não confirmação de formulário.
            LognButton(
                if (resetLocked) Str.Status.wait_seconds(context, view.authCooldownSeconds.toInt()) else Str.Reset.save_password(context),
                ButtonVariant.Primary,
                enabled = canSubmit && !view.isAuthenticating,
            ) { dispatch(Event.ResetPassword(email, password, code)) }
        }
        StatusLine(view.status)
    }
}
