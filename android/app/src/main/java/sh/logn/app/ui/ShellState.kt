package sh.logn.app.ui

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import kotlinx.coroutines.MainScope

/** O link de redefinição que chegou, como um dado só: a tela nasce dele. */
data class ResetLink(
    val email: String,
    val code: String,
)

/**
 * O que é do shell, e não do Core: o que chega de fora da árvore de telas (o link do
 * e-mail) e as escolhas de navegação que o `ViewModel` não descreve. Os `@State` de
 * LogNiOSApp.swift e o `wantsRegistration` de CoreWrapper.swift.
 */
object ShellState {
    /**
     * Escopo do processo para o login social: a janela do provedor pode recriar a
     * Activity (troca de tema, de fonte), e um escopo de tela levaria o pedido junto.
     */
    val scope = MainScope()

    var resetLink by mutableStateOf<ResetLink?>(null)

    /** O visitante escolheu salvar o progresso: cai direto no cadastro. */
    var wantsRegistration by mutableStateOf(false)
}
