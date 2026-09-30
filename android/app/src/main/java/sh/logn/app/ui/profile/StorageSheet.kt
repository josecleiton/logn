package sh.logn.app.ui.profile

import android.text.format.Formatter
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.components.FullScreenSheet
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.home.Divider
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.TrackView
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * O que as trilhas pagas ocupam no aparelho, e o apagar (spec, seção 9). Espelho de
 * StorageManagementView: apagar tira a chave e o pacote daqui; a compra continua na conta.
 */
@Composable
fun StorageSheet(
    view: ViewModel,
    onClose: () -> Unit,
) {
    val context = LocalContext.current
    // Fora da árvore de telas: filtro dentro dela é o que o AGENTS.md (regra 11) evita.
    val owned = view.tracks.filter { it.owned || it.downloaded }
    FullScreenSheet(onDismiss = onClose) {
        Column(Modifier.fillMaxSize()) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(start = Space.xl, end = Space.md, top = Space.xl, bottom = Space.lg),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(Str.Storage.title(context), style = LognFont.titleMedium, color = LognDark.textPrimary, modifier = Modifier.weight(1f))
                val close = Str.Storage.close(context)
                Box(
                    Modifier
                        .size(IconMetrics.touch)
                        .semantics { contentDescription = close }
                        .clickable(role = Role.Button, onClick = onClose),
                    contentAlignment = Alignment.Center,
                ) { Icon(LognIcon.Close, LognDark.textSecondary, IconMetrics.md) }
            }
            Divider()
            Column(
                Modifier
                    .verticalScroll(rememberScrollState())
                    .padding(horizontal = Space.xl),
                verticalArrangement = Arrangement.spacedBy(Space.md),
            ) {
                Text(Str.Storage.desc(context), style = LognFont.bodyMedium, color = LognDark.textSecondary, modifier = Modifier.padding(top = Space.lg))
                if (owned.isEmpty()) {
                    Text(
                        Str.Storage.empty(context),
                        style = LognFont.bodyMedium,
                        color = LognDark.textSecondary,
                        modifier = Modifier.padding(top = Space.xl),
                    )
                }
                for (track in owned) StorageRow(track)
            }
        }
    }
}

@Composable
private fun StorageRow(track: TrackView) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val detail =
        if (track.downloaded) {
            Str.Storage.size(context, Formatter.formatShortFileSize(context, track.downloadBytes.toLong()))
        } else {
            Str.Storage.not_downloaded(context)
        }
    val shape = RoundedCornerShape(Radius.md)
    Row(
        Modifier
            .fillMaxWidth()
            .background(LognDark.canvas, shape)
            .border(Stroke.hairline, LognDark.line, shape)
            .padding(Space.lg),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(Space.xs)) {
            Text(track.name, style = LognFont.bodyLarge, color = LognDark.textPrimary)
            Text(detail, style = LognFont.label, color = LognDark.textSecondary)
        }
        if (track.downloaded) {
            val label = Str.Storage.delete_accessibility(context, track.name)
            Box(
                Modifier
                    .size(IconMetrics.touch)
                    .semantics { contentDescription = label }
                    .clickable(role = Role.Button) { dispatch(Event.DeleteTrackDownload(track.id)) },
                contentAlignment = Alignment.Center,
            ) { Icon(LognIcon.Trash, LognDark.wrongInk, IconMetrics.md) }
        } else {
            Box(
                Modifier
                    .heightIn(min = IconMetrics.touch)
                    .clickable(role = Role.Button) { dispatch(Event.FetchLicense(track.id)) },
                contentAlignment = Alignment.Center,
            ) { Text(Str.Storage.download(context), style = LognFont.label, color = LognDark.accent) }
        }
    }
}
