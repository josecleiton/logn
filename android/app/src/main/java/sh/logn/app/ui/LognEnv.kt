package sh.logn.app.ui

import androidx.compose.runtime.staticCompositionLocalOf
import sh.logn.core.LogN.Event

/**
 * O `dispatch` do Core para qualquer tela, sem passar de mão em mão: o `environmentObject`
 * de iOS. A leitura do `ViewModel` continua descendo por parâmetro, que é o que o Compose
 * sabe recompor.
 */
val LocalDispatch = staticCompositionLocalOf<(Event) -> Unit> { error("LocalDispatch sem Core") }
