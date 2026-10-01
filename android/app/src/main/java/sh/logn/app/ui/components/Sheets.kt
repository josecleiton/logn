package sh.logn.app.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.rememberModalBottomSheetState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.nestedscroll.NestedScrollConnection
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.unit.Velocity
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.Radius

/**
 * A folha grande de iOS (`.sheet` sem detente): documento legal, cadastro social. Tela
 * cheia, fundo `surfaceRaised`, voltar do sistema fecha. `dismissible = false` segura o
 * voltar enquanto um pedido está no ar.
 */
@Composable
fun FullScreenSheet(
    onDismiss: () -> Unit,
    dismissible: Boolean = true,
    background: Color = LognDark.surfaceRaised,
    content: @Composable () -> Unit,
) {
    Dialog(
        onDismissRequest = { if (dismissible) onDismiss() },
        properties =
            DialogProperties(
                usePlatformDefaultWidth = false,
                decorFitsSystemWindows = false,
                dismissOnBackPress = dismissible,
                dismissOnClickOutside = false,
            ),
    ) {
        Box(Modifier.fillMaxSize().background(background).safeDrawingPadding()) { content() }
    }
}

/**
 * O que sobra de um arremesso para cima depois de o conteúdo rolar morre aqui, antes de
 * chegar à folha. No Material3 1.4 a folha aberta, com conteúdo perto da altura da tela
 * (o perfil), recebia essa sobra e não parava mais de quicar: a animação de assentar
 * brigava com a medida da folha, sobe e desce sem fim. Para baixo nada muda: arrastar e
 * arremessar para fechar continuam chegando à folha.
 */
private object SwallowUpwardFling : NestedScrollConnection {
    override suspend fun onPostFling(
        consumed: Velocity,
        available: Velocity,
    ): Velocity = if (available.y < 0f) Velocity(0f, available.y) else Velocity.Zero
}

/**
 * A folha de baixo, com a altura do conteúdo: confirmações e escolhas curtas. É o
 * `presentationDetents` de iOS.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BottomSheet(
    onDismiss: () -> Unit,
    dismissible: Boolean = true,
    background: Color = LognDark.surfaceRaised,
    content: @Composable () -> Unit,
) {
    val state =
        rememberModalBottomSheetState(
            skipPartiallyExpanded = true,
            confirmValueChange = { dismissible },
        )
    ModalBottomSheet(
        onDismissRequest = { if (dismissible) onDismiss() },
        sheetState = state,
        containerColor = background,
        shape = androidx.compose.foundation.shape.RoundedCornerShape(topStart = Radius.md, topEnd = Radius.md),
        dragHandle = null,
    ) {
        Box(
            Modifier
                .fillMaxWidth()
                .nestedScroll(SwallowUpwardFling)
                .verticalScroll(rememberScrollState())
                .navigationBarsPadding()
                .padding(bottom = Radius.md),
        ) {
            content()
        }
    }
}
