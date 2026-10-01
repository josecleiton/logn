package sh.logn.app.ui.components

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import sh.logn.app.ui.copy.copy
import sh.logn.app.ui.copy.isInProgress
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.core.LogN.StatusKey

/**
 * A linha de status das telas de autenticação, num lugar só. Em andamento sai em
 * `textSecondary`; problema, em `wrongInk`. Sucesso não aparece: a prova é a tela avançar.
 */
@Composable
fun StatusLine(
    status: StatusKey,
    modifier: Modifier = Modifier,
) {
    val copy = status.copy(LocalContext.current) ?: return
    Text(
        copy,
        style = LognFont.label,
        color = if (status.isInProgress) LognDark.textSecondary else LognDark.wrongInk,
        textAlign = TextAlign.Center,
        // O leitor de tela anuncia o problema sem o foco ir até ele.
        modifier = modifier.semantics { liveRegion = LiveRegionMode.Polite },
    )
}
