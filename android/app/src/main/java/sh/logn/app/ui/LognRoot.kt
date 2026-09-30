package sh.logn.app.ui

import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import sh.logn.app.ui.splash.SplashScreen
import sh.logn.app.ui.theme.LocalReduceMotion
import sh.logn.app.ui.theme.LognDark
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.Core

/** A tela que o `ViewModel` pede, na ordem de LogNiOSApp.swift. Sem NavHost. */
private enum class RootScreen { Splash, TermsUpdate, App, DeletionNotice, LogoutNotice, Login }

private fun ViewModel.rootScreen(): RootScreen =
    when {
        boot.inProgress -> RootScreen.Splash
        termsUpdate != null -> RootScreen.TermsUpdate
        hasSession || isGuest -> RootScreen.App
        deletionPurgeAfter > 0 -> RootScreen.DeletionNotice
        justLoggedOut -> RootScreen.LogoutNotice
        else -> RootScreen.Login
    }

@Composable
fun LognRoot(core: Core) {
    val view by core.view.collectAsStateWithLifecycle()
    val reduceMotion = LocalReduceMotion.current

    // A splash sai em fade: sem duração mínima, uma abertura rápida sem ele vira um piscar.
    Crossfade(
        targetState = view.rootScreen(),
        animationSpec = tween(if (reduceMotion) 0 else FADE_MILLIS),
        label = "root",
    ) { screen ->
        when (screen) {
            RootScreen.Splash -> SplashScreen(view.boot, core::update)
            // As outras telas chegam uma por fase, na ordem de iOS.
            else -> Box(Modifier.fillMaxSize().background(LognDark.canvas))
        }
    }
}

private const val FADE_MILLIS = 250
