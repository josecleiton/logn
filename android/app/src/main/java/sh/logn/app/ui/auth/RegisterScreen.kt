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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextAlign
import sh.logn.app.auth.DeviceCountry
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.components.BalloonMarquee
import sh.logn.app.ui.components.BrandLockup
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.FieldKind
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognCheckbox
import sh.logn.app.ui.components.LognTextField
import sh.logn.app.ui.components.StatusLine
import sh.logn.app.ui.legal.LegalKind
import sh.logn.app.ui.legal.LegalLinksRow
import sh.logn.app.ui.theme.FieldMetrics
import sh.logn.app.ui.theme.LoginMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Space
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

private enum class RegisterStep { Email, Otp, Password }

/** O cadastro em três passos: e-mail e aceites, código, senha. Espelho de RegisterView.swift. */
@Composable
fun RegisterScreen(
    view: ViewModel,
    onBack: () -> Unit,
    onLegal: (LegalKind) -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    var email by rememberSaveable { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var confirm by remember { mutableStateOf("") }
    var otp by rememberSaveable { mutableStateOf("") }
    var step by rememberSaveable { mutableStateOf(RegisterStep.Email) }
    var ageConfirmed by rememberSaveable { mutableStateOf(false) }
    var termsAccepted by rememberSaveable { mutableStateOf(false) }
    val country = remember { DeviceCountry.current(context) }

    BackHandler {
        // Voltar recua um passo; do primeiro, sai do cadastro.
        step =
            when (step) {
                RegisterStep.Email -> {
                    onBack()
                    RegisterStep.Email
                }
                RegisterStep.Otp -> RegisterStep.Email
                RegisterStep.Password -> RegisterStep.Otp
            }
    }
    // O cadastro aceita a versão vigente dos documentos e declara a idade mínima do país:
    // só o servidor sabe as duas.
    LaunchedEffect(Unit) { dispatch(Event.FetchLegalVersions(country)) }
    LaunchedEffect(view.otpVerified) { if (view.otpVerified) step = RegisterStep.Password }
    // A frase da caixa mudou de idade: quem marcou antes declarou outra coisa.
    LaunchedEffect(view.minAge) { ageConfirmed = false }

    val sendLocked = view.resendCooldownSeconds > 0u
    val accountLocked = view.authCooldownSeconds > 0u
    // Sem as versões vigentes não há o que aceitar: o código seria gasto num cadastro
    // que o servidor recusa.
    val canSendCode = "@" in email && !sendLocked && ageConfirmed && termsAccepted && view.legalVersionsReady
    val passwordsMatch = confirm.isEmpty() || password == confirm
    val canRegister = password.length >= MIN_PASSWORD && password == confirm && !accountLocked

    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.canvas)
            .safeDrawingPadding()
            .imePadding()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = Space.screenMargin),
        verticalArrangement = Arrangement.spacedBy(Space.xl),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        BalloonMarquee(Modifier.padding(top = LoginMetrics.marqueeTop))
        BrandLockup(LoginMetrics.BRAND_SIZE)
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(Space.xs)) {
            Text(
                when (step) {
                    RegisterStep.Email -> Str.Register.create_account(context)
                    RegisterStep.Otp -> Str.Register.verify_email(context)
                    RegisterStep.Password -> Str.Register.set_password(context)
                },
                style = LognFont.headlineMedium,
                color = LognDark.textPrimary,
            )
            Text(
                when (step) {
                    RegisterStep.Email -> Str.Register.reason_email(context)
                    RegisterStep.Otp -> Str.Register.reason_otp(context, email)
                    RegisterStep.Password -> Str.Register.reason_password(context)
                },
                style = LognFont.mono(FieldMetrics.TEXT_SIZE),
                color = LognDark.textSecondary,
                textAlign = TextAlign.Center,
            )
        }

        when (step) {
            RegisterStep.Email ->
                Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(Space.sm)) {
                    LognTextField(email, { email = it }, Str.Register.email_prompt(context), FieldKind.Email)
                    Column(Modifier.padding(top = Space.xs, bottom = Space.sm), verticalArrangement = Arrangement.spacedBy(Space.md)) {
                        LognCheckbox(ageConfirmed, Str.Register.age_confirmation(context, view.minAge.toInt())) {
                            ageConfirmed = !ageConfirmed
                        }
                        LognCheckbox(termsAccepted, Str.Register.terms_confirmation(context)) {
                            termsAccepted = !termsAccepted
                            // A busca da abertura pode ter falhado sem rede; marcar é a hora
                            // de tentar de novo.
                            if (termsAccepted && !view.legalVersionsReady) dispatch(Event.FetchLegalVersions(country))
                        }
                        // Fora da caixa: dentro dela, o toque no link marcaria a caixa.
                        LegalLinksRow(onOpen = onLegal)
                        if (termsAccepted && !view.legalVersionsReady) {
                            Text(Str.Register.legal_pending(context), style = LognFont.sans(LoginMetrics.CHECK_SIZE), color = LognDark.textMuted)
                        }
                    }
                    LognButton(
                        when {
                            sendLocked -> Str.Status.wait_seconds(context, view.resendCooldownSeconds.toInt())
                            view.isAuthenticating -> Str.Register.sending(context)
                            else -> Str.Register.send_code(context)
                        },
                        ButtonVariant.Primary,
                        Modifier.padding(top = Space.xs),
                        enabled = canSendCode && !view.isAuthenticating,
                    ) {
                        dispatch(Event.RequestOTP(email, "verify_email"))
                        step = RegisterStep.Otp
                    }
                }
            RegisterStep.Otp -> OtpInput(view, email, "verify_email") { otp = it }
            RegisterStep.Password ->
                Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(Space.sm)) {
                    LognTextField(password, { password = it }, Str.Register.password_prompt(context), FieldKind.NewPassword)
                    LognTextField(
                        confirm,
                        { confirm = it },
                        Str.Register.confirm_prompt(context),
                        FieldKind.NewPassword,
                        error = !passwordsMatch,
                    )
                    LognButton(
                        if (accountLocked) {
                            Str.Status.wait_seconds(context, view.authCooldownSeconds.toInt())
                        } else {
                            Str.Register.create_account(context)
                        },
                        ButtonVariant.Primary,
                        Modifier.padding(top = Space.xs),
                        enabled = canRegister && !view.isAuthenticating,
                    ) {
                        // O código que o jogador digitou, não o e-mail. Quais versões e em
                        // que língua, o Core decide com o que o servidor disse.
                        dispatch(Event.Register(email, password, otp, ageConfirmed, termsAccepted))
                    }
                }
        }

        // Uma linha de status na tela inteira: a de código não repete.
        StatusLine(view.status, Modifier.padding(bottom = Space.xl))
    }
}

const val MIN_PASSWORD = 8
