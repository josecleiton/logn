package sh.logn.app.ui.auth

import android.app.Activity
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalAutofillManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextDecoration
import kotlinx.coroutines.launch
import sh.logn.app.auth.GitHubAuth
import sh.logn.app.auth.GoogleAuth
import sh.logn.app.auth.SocialAuthFailure
import sh.logn.app.core.FeatureFlags
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.ShellState
import sh.logn.app.ui.components.BalloonMarquee
import sh.logn.app.ui.components.BrandLockup
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.FieldKind
import sh.logn.app.ui.components.GitHubIcon
import sh.logn.app.ui.components.GoogleIcon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognTextField
import sh.logn.app.ui.components.StatusLine
import sh.logn.app.ui.components.ToastCenter
import sh.logn.app.ui.components.ToastVariant
import sh.logn.app.ui.legal.LegalKind
import sh.logn.app.ui.legal.LegalLinksRow
import sh.logn.app.ui.theme.FieldMetrics
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LoginMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str
import sh.logn.app.ui.theme.Stroke as LognStroke

/**
 * A entrada. Espelho de LoginView.swift: provedores quando as flags deixam, e-mail e
 * senha, criar conta, esqueci a senha e o modo visitante.
 */
@Composable
fun LoginScreen(
    view: ViewModel,
    onRegister: () -> Unit,
    onReset: (email: String) -> Unit,
    onLegal: (LegalKind) -> Unit,
) {
    val dispatch = LocalDispatch.current

    // Sair do login sem entrar (criar conta, esqueci a senha) não oferece salvar a senha:
    // ela pode ser a errada que acabou de voltar recusada.
    val autofill = LocalAutofillManager.current
    val signedIn by rememberUpdatedState(view.hasSession)
    DisposableEffect(Unit) { onDispose { if (!signedIn) autofill?.cancel() } }

    BoxWithConstraints(
        Modifier
            .fillMaxSize()
            .background(LognDark.canvas)
            .safeDrawingPadding()
            .imePadding(),
    ) {
        val minHeight = maxHeight
        // Duas metades com `SpaceBetween` e altura mínima de uma tela: sem teclado, o
        // visitante fica no pé como no iOS; com teclado, rola. (`weight` não vale em
        // coluna que rola.)
        Column(
            Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .heightIn(min = minHeight)
                .padding(horizontal = Space.screenMargin),
            verticalArrangement = Arrangement.SpaceBetween,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            LoginForm(view, onRegister, onReset)
            LoginFooter(onGuest = { dispatch(Event.ContinueAsGuest) }, onLegal = onLegal)
        }
    }
}

@Composable
private fun LoginForm(
    view: ViewModel,
    onRegister: () -> Unit,
    onReset: (email: String) -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val scope = ShellState.scope
    var email by rememberSaveable { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    val emailFocus = remember { FocusRequester() }

    // A sessão acabou: o login vem com o e-mail dela, que pode chegar depois de a tela
    // aparecer. Só preenche o campo vazio: o que a pessoa digitou manda.
    LaunchedEffect(view.resumeEmail) {
        if (email.isEmpty() && view.resumeEmail.isNotEmpty()) email = view.resumeEmail
    }

    // As flags chegam tarde na primeira abertura e depois de sair: ler a versão faz a
    // tela recompor quando elas chegam.
    FeatureFlags.version
    val googleEnabled = GoogleAuth.isConfigured && FeatureFlags.isEnabled(FeatureFlags.GOOGLE)
    val githubEnabled = GitHubAuth.isConfigured && FeatureFlags.isEnabled(FeatureFlags.GITHUB)

    val signInLocked = view.authCooldownSeconds > 0u
    val resendLocked = view.resendCooldownSeconds > 0u
    val canSignIn = email.isNotEmpty() && password.isNotEmpty() && !signInLocked
    val busy = view.isAuthenticating || signInLocked
    val signInLabel =
        when {
            signInLocked -> Str.Status.wait_seconds(context, view.authCooldownSeconds.toInt())
            // Com a sessão aberta esta tela está saindo, e ainda aparece por um quadro.
            view.isAuthenticating || view.hasSession -> Str.Login.loading(context)
            else -> Str.Login.sign_in(context)
        }

    fun social(block: suspend (Activity) -> Unit) {
        val activity = context as? Activity ?: return
        scope.launch {
            try {
                block(activity)
            } catch (_: SocialAuthFailure.Cancelled) {
                // Fechou a janela do provedor: nada a dizer.
            } catch (_: SocialAuthFailure) {
                dispatch(Event.SocialLoginFailed)
            }
        }
    }

    fun submit() {
        if (canSignIn && !view.isAuthenticating) dispatch(Event.Login(email, password))
    }

    fun requestReset() {
        if (email.isBlank()) {
            ToastCenter.show(ToastVariant.Attention, Str.Login.email_first(context), id = "login.email_first")
            runCatching { emailFocus.requestFocus() }
            return
        }
        dispatch(Event.RequestOTP(email, "reset_password"))
        onReset(email)
    }

    Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
        BalloonMarquee(Modifier.padding(top = LoginMetrics.marqueeTop))
        Spacer(Modifier.height(LoginMetrics.marqueeToBrand))
        BrandLockup(LoginMetrics.BRAND_SIZE)
        Spacer(Modifier.height(Space.md))
        Text(
            Str.Login.slogan(context),
            style = LognFont.mono(LoginMetrics.SLOGAN_SIZE, tracking = LoginMetrics.SLOGAN_TRACKING),
            color = LognDark.textSecondary,
        )
        Spacer(Modifier.height(LoginMetrics.brandToForm))

        Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(Space.sm)) {
            if (googleEnabled) {
                ProviderButton(Str.Login.sign_in_google(context), enabled = !busy, icon = { GoogleIcon(IconMetrics.lg) }) {
                    social { activity ->
                        val credential = GoogleAuth.signIn(activity)
                        dispatch(Event.SocialLogin("google", credential.idToken, credential.nonce))
                    }
                }
            }
            if (githubEnabled) {
                ProviderButton(Str.Login.sign_in_github(context), enabled = !busy, icon = { GitHubIcon(IconMetrics.lg) }) {
                    social { activity ->
                        val credential = GitHubAuth.signIn(activity)
                        // O Core troca o código pelo bilhete no servidor e segue o login.
                        dispatch(Event.GitHubCodeReceived(credential.code, credential.verifier, credential.nonce))
                    }
                }
            }
            if (googleEnabled || githubEnabled) OrDivider(Str.Login.or_email(context))

            LognTextField(
                email,
                { email = it },
                Str.Login.email_prompt(context),
                FieldKind.Email,
                Modifier.focusRequester(emailFocus),
            )
            LognTextField(
                password,
                { password = it },
                Str.Login.password_prompt(context),
                FieldKind.Password,
                imeAction = ImeAction.Go,
                onDone = ::submit,
            )
            LognButton(
                signInLabel,
                ButtonVariant.Primary,
                Modifier.padding(top = Space.xs),
                enabled = canSignIn && !view.isAuthenticating,
                onClick = ::submit,
            )
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(top = Space.xs),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                LinkText(Str.Login.create_account(context), LognDark.textSecondary, onClick = onRegister)
                Spacer(Modifier.weight(1f))
                // Só o intervalo de reenvio trava o link. Sem e-mail ele diz o que falta.
                LinkText(
                    if (resendLocked) {
                        Str.Status.wait_seconds(context, view.resendCooldownSeconds.toInt())
                    } else {
                        Str.Login.forgot_password(context)
                    },
                    if (resendLocked) LognDark.textDim else LognDark.textSecondary,
                    enabled = !resendLocked,
                    onClick = ::requestReset,
                )
            }
        }

        StatusLine(view.status, Modifier.padding(top = Space.md))
    }
}

@Composable
private fun LoginFooter(
    onGuest: () -> Unit,
    onLegal: (LegalKind) -> Unit,
) {
    val context = LocalContext.current
    Column(
        Modifier
            .fillMaxWidth()
            .padding(top = Space.xl, bottom = Space.screenMargin),
        verticalArrangement = Arrangement.spacedBy(Space.md),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(
            Modifier
                .fillMaxWidth()
                .height(LognStroke.hairline)
                .background(LognDark.line),
        )
        GuestButton(onGuest)
        LegalLinksRow(onOpen = onLegal)
        Row(
            Modifier.padding(top = Space.xs),
            horizontalArrangement = Arrangement.spacedBy(Space.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier
                    .size(LoginMetrics.coreDot)
                    .background(LognDark.correct, CircleShape),
            )
            Text(
                Str.Login.core_ready(context),
                style = LognFont.mono(LoginMetrics.CORE_SIZE, tracking = LoginMetrics.CORE_TRACKING),
                color = LognDark.textMuted,
            )
        }
    }
}

/** "Continuar com …": superfície escura com borda, o ícone do provedor à esquerda. */
@Composable
fun ProviderButton(
    title: String,
    enabled: Boolean = true,
    icon: @Composable () -> Unit,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        Modifier
            .fillMaxWidth()
            .height(FieldMetrics.height)
            .background(LognDark.surface, shape)
            .border(LognStroke.hairline, LognDark.line, shape)
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick),
        horizontalArrangement = Arrangement.spacedBy(Space.md, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        icon()
        Text(title, style = LognFont.sans(LoginMetrics.PROVIDER_SIZE, FontWeight.Medium), color = LognDark.textPrimary)
    }
}

@Composable
private fun OrDivider(label: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(vertical = Space.sm),
        horizontalArrangement = Arrangement.spacedBy(Space.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .weight(1f)
                .height(LognStroke.hairline)
                .background(LognDark.line),
        )
        Text(
            label,
            style = LognFont.mono(LoginMetrics.DIVIDER_SIZE, tracking = LoginMetrics.DIVIDER_TRACKING),
            color = LognDark.textMuted,
        )
        Box(
            Modifier
                .weight(1f)
                .height(LognStroke.hairline)
                .background(LognDark.line),
        )
    }
}

/** Texto tocável das telas de entrada, com alvo de 44. */
@Composable
fun LinkText(
    text: String,
    color: Color,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    underline: Boolean = false,
    onClick: () -> Unit,
) {
    Box(
        modifier
            .heightIn(min = LoginMetrics.linkMinHeight)
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        val style = LognFont.sans(LoginMetrics.LINK_SIZE)
        Text(
            text,
            style = if (underline) style.copy(textDecoration = TextDecoration.Underline) else style,
            color = color,
        )
    }
}

/** Modo visitante: borda tracejada, sem fundo, o que não salva fica dito embaixo. */
@Composable
private fun GuestButton(onClick: () -> Unit) {
    val context = LocalContext.current
    Column(
        Modifier
            .fillMaxWidth()
            .height(LoginMetrics.guestHeight)
            .drawBehind {
                val dash = LoginMetrics.dash.toPx()
                drawRoundRect(
                    LognDark.line,
                    cornerRadius = CornerRadius(Radius.sm.toPx()),
                    style = Stroke(LognStroke.hairline.toPx(), pathEffect = PathEffect.dashPathEffect(floatArrayOf(dash, dash))),
                )
            }.clickable(role = Role.Button, onClick = onClick),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            Str.Login.guest_mode(context),
            style = LognFont.sans(LoginMetrics.GUEST_SIZE, FontWeight.SemiBold),
            color = LognDark.textPrimary,
        )
        Text(Str.Login.without_saving(context), style = LognFont.mono(LoginMetrics.GUEST_SUB_SIZE), color = LognDark.textMuted)
    }
}
