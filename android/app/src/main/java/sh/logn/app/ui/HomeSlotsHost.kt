package sh.logn.app.ui

import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.home.HomeRoute
import sh.logn.app.ui.home.HomeSlots
import sh.logn.app.ui.match.MatchScreen
import sh.logn.app.ui.profile.ProfileHub
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * Liga as telas das outras fases (partida, perfil, catálogo, placar) aos pontos de
 * extensão da tela do jogo. Um lugar só, para a tela do jogo não conhecer cada uma.
 */
@Composable
fun rememberHomeSlots(view: ViewModel): HomeSlots =
    HomeSlots(
        route = { route, onBack ->
            when (route) {
                is HomeRoute.Match ->
                    MatchScreen(view, route.nodeId, onExit = onBack) { onDismiss ->
                        LognButton(Str.Match.got_it(LocalContext.current), ButtonVariant.Primary, onClick = onDismiss)
                    }
                HomeRoute.Catalog, HomeRoute.Scoreboard -> Unit
            }
        },
        profile = { onDismiss -> ProfileHub(view, onDismiss, onRestore = {}) },
    )
