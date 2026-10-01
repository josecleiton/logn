package sh.logn.app.ui

import androidx.compose.runtime.staticCompositionLocalOf
import sh.logn.app.core.PlayStore
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel

/**
 * O `dispatch` do Core para qualquer tela, sem passar de mão em mão: o `environmentObject`
 * de iOS. A leitura do `ViewModel` continua descendo por parâmetro, que é o que o Compose
 * sabe recompor.
 */
val LocalDispatch = staticCompositionLocalOf<(Event) -> Unit> { error("LocalDispatch sem Core") }

/**
 * O `ViewModel` de agora, lido na hora, e não o da última recomposição: o `update` do Core
 * é síncrono, e o que vem logo depois de um evento (o veredito do submit) já está nele.
 * É o `core.viewModel` que o iOS lê depois de `dispatch`.
 */
val LocalReadView = staticCompositionLocalOf<() -> ViewModel> { error("LocalReadView sem Core") }

/** A loja do aparelho (Google Play), para as telas de compra. */
val LocalStore = staticCompositionLocalOf<PlayStore> { error("LocalStore sem loja") }
