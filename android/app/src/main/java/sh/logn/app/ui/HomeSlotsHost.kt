package sh.logn.app.ui

import androidx.compose.runtime.Composable
import sh.logn.app.ui.home.HomeSlots
import sh.logn.core.LogN.ViewModel

/**
 * Liga as telas das outras fases (partida, perfil, catálogo, placar) aos pontos de
 * extensão da tela do jogo. Um lugar só, para a tela do jogo não conhecer cada uma.
 */
@Composable
fun rememberHomeSlots(view: ViewModel): HomeSlots = HomeSlots()
