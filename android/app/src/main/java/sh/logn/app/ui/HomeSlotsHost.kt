package sh.logn.app.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.home.HomeRoute
import sh.logn.app.ui.home.HomeSlots
import sh.logn.app.ui.match.MatchScreen
import sh.logn.app.ui.profile.ProfileHub
import sh.logn.app.ui.leaderboard.LeaderboardScreen
import sh.logn.app.ui.store.CatalogScreen
import sh.logn.app.ui.store.LockedNodeSheet
import sh.logn.app.ui.store.RestoreResultSheet
import sh.logn.app.ui.store.SampleOfferCard
import sh.logn.app.ui.store.TrackDetailScreen
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * Liga as telas das outras fases (partida, perfil, catálogo, placar) aos pontos de
 * extensão da tela do jogo. Um lugar só, para a tela do jogo não conhecer cada uma.
 */
@Composable
fun rememberHomeSlots(view: ViewModel): HomeSlots {
    val dispatch = LocalDispatch.current
    val store = LocalStore.current
    return HomeSlots(
        route = { route, onBack ->
            when (route) {
                is HomeRoute.Match ->
                    MatchScreen(view, route.nodeId, onExit = onBack) { onDismiss ->
                        // Fim da amostra de uma trilha paga: a oferta toma o lugar do "Entendi".
                        if (view.sampleOffer.active) {
                            SampleOfferCard(view, onDismiss)
                        } else {
                            LognButton(Str.Match.got_it(LocalContext.current), ButtonVariant.Primary, onClick = onDismiss)
                        }
                    }
                HomeRoute.Catalog -> CatalogRoute(view, onBack) { id -> dispatch(Event.SelectTrack(id)) }
            }
        },
        standings = { goToTrail -> LeaderboardScreen(view, goToTrail) },
        profile = { onDismiss ->
            ProfileHub(view, onDismiss, onRestore = store::restore) {
                if (view.restoreResult.active) RestoreResultSheet(view)
            }
        },
        lockedNode = { node, onDismiss -> LockedNodeSheet(node, view.currentTrack, onDismiss) },
    )
}

/** O catálogo e a página de uma trilha, empilhados. Abrir uma trilha volta à árvore. */
@Composable
private fun CatalogRoute(
    view: ViewModel,
    onBack: () -> Unit,
    select: (String) -> Unit,
) {
    var trackId by rememberSaveable { mutableStateOf<String?>(null) }
    val open: (String) -> Unit = { id ->
        select(id)
        onBack()
    }
    val current = trackId
    if (current == null) {
        CatalogScreen(view, onBack = onBack, onTrack = { trackId = it }, openTrack = open)
    } else {
        TrackDetailScreen(view, current, onBack = { trackId = null }, openTrack = open)
    }
}
