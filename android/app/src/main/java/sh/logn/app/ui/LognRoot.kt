package sh.logn.app.ui

import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import sh.logn.app.ui.auth.LoginScreen
import sh.logn.app.ui.auth.RegisterScreen
import sh.logn.app.ui.auth.ResetPasswordScreen
import sh.logn.app.ui.auth.SocialSignupSheet
import sh.logn.app.ui.components.FullScreenSheet
import sh.logn.app.ui.components.ToastHost
import sh.logn.app.ui.home.HomeScreen
import sh.logn.app.ui.legal.LegalDocumentScreen
import sh.logn.app.ui.legal.LegalKind
import sh.logn.app.ui.notice.AccountRestoredCard
import sh.logn.app.ui.notice.DeletionNoticeScreen
import sh.logn.app.ui.notice.LogoutNoticeScreen
import sh.logn.app.ui.splash.SplashScreen
import sh.logn.app.ui.terms.TermsNoticeBanner
import sh.logn.app.ui.terms.TermsUpdateScreen
import sh.logn.app.ui.terms.identity
import sh.logn.app.ui.theme.LocalReduceMotion
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.RootMetrics
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.Core

/** A tela que o `ViewModel` pede, na ordem de LogNiOSApp.swift. Sem NavHost. */
private enum class RootScreen { Splash, TermsUpdate, App, DeletionNotice, Registration, LogoutNotice, Login }

private fun ViewModel.rootScreen(wantsRegistration: Boolean): RootScreen =
    when {
        boot.inProgress -> RootScreen.Splash
        termsUpdate != null -> RootScreen.TermsUpdate
        // Sessão, não credencial: sem rede e com a sessão no prazo, entra no jogo.
        hasSession || isGuest -> RootScreen.App
        deletionPurgeAfter > 0 -> RootScreen.DeletionNotice
        wantsRegistration -> RootScreen.Registration
        justLoggedOut -> RootScreen.LogoutNotice
        else -> RootScreen.Login
    }

@Composable
fun LognRoot(core: Core) {
    val view by core.view.collectAsStateWithLifecycle()
    val reduceMotion = LocalReduceMotion.current
    var legal by rememberSaveable { mutableStateOf<LegalKind?>(null) }
    val openLegal: (LegalKind) -> Unit = { legal = it }

    // Quem já estava logado não vê token chegar: o Core avisa que a senha trocou.
    LaunchedEffect(view.hasAccessToken) { if (view.hasAccessToken) ShellState.resetLink = null }
    LaunchedEffect(view.passwordResetDone) {
        if (view.passwordResetDone) {
            ShellState.resetLink = null
            core.update(Event.DismissPasswordReset)
        }
    }

    CompositionLocalProvider(LocalDispatch provides core::update) {
        Box(Modifier.fillMaxSize().background(LognDark.canvas)) {
            // A splash sai em fade: sem duração mínima, uma abertura rápida vira um piscar.
            Crossfade(
                targetState = view.rootScreen(ShellState.wantsRegistration),
                animationSpec = tween(if (reduceMotion) 0 else FADE_MILLIS),
                label = "root",
            ) { screen ->
                when (screen) {
                    RootScreen.Splash -> SplashScreen(view.boot, core::update)
                    RootScreen.TermsUpdate ->
                        view.termsUpdate?.let { update ->
                            // Conteúdo novo é tela nova: a caixa marcada para uma versão não
                            // vale para a que o servidor trouxe depois de um 409.
                            key(update.identity()) { TermsUpdateScreen(view, update) }
                        }
                    RootScreen.App -> AppHost(view)
                    RootScreen.DeletionNotice -> DeletionNoticeScreen(view)
                    RootScreen.Registration ->
                        RegisterScreen(view, onBack = { ShellState.wantsRegistration = false }, onLegal = openLegal)
                    RootScreen.LogoutNotice -> LogoutNoticeScreen(view)
                    RootScreen.Login -> AuthFlow(view, openLegal)
                }
            }

            ShellState.resetLink?.let { link ->
                FullScreenSheet(onDismiss = { ShellState.resetLink = null }, background = LognDark.canvas) {
                    ResetPasswordScreen(view, link.email, link.code, onClose = { ShellState.resetLink = null })
                }
            }
            if (view.socialSignupRequired) SocialSignupSheet(view, openLegal)
            legal?.let { kind -> LegalDocumentScreen(kind, onClose = { legal = null }) }
            // Por cima de qualquer tela, e não de cada uma: um aviso por vez no app.
            ToastHost(Modifier.align(Alignment.TopCenter))
        }
    }
}

/** O jogo, com os avisos que ficam por cima dele. */
@Composable
private fun AppHost(view: ViewModel) {
    Box(Modifier.fillMaxSize().background(LognDark.canvas)) {
        HomeScreen(view, rememberHomeSlots(view))
        // A exclusão pedida foi cancelada por este login.
        if (view.accountRestoredNotice) AccountRestoredCard(Modifier.align(Alignment.BottomCenter))
        // Só mudanças não relevantes: a faixa, uma vez, acima da barra de abas.
        if (view.termsNotice) {
            TermsNoticeBanner(
                Modifier
                    .align(Alignment.BottomCenter)
                    .navigationBarsPadding()
                    .padding(bottom = RootMetrics.bannerAboveTabs),
            )
        }
    }
}

private enum class AuthRoute { Login, Register, Reset }

/**
 * Login, cadastro e "Esqueci a senha", empilhados como o `NavigationStack` de LoginView.
 * Voltar do sistema desempilha.
 */
@Composable
private fun AuthFlow(
    view: ViewModel,
    openLegal: (LegalKind) -> Unit,
) {
    var route by rememberSaveable { mutableStateOf(AuthRoute.Login) }
    var resetEmail by rememberSaveable { mutableStateOf("") }
    when (route) {
        AuthRoute.Login ->
            LoginScreen(
                view,
                onRegister = { route = AuthRoute.Register },
                onReset = {
                    resetEmail = it
                    route = AuthRoute.Reset
                },
                onLegal = openLegal,
            )
        AuthRoute.Register -> RegisterScreen(view, onBack = { route = AuthRoute.Login }, onLegal = openLegal)
        AuthRoute.Reset -> ResetPasswordScreen(view, resetEmail, linkCode = "", onClose = { route = AuthRoute.Login })
    }
}

private const val FADE_MILLIS = 250
