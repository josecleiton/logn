package sh.logn.app.ui.track

import android.content.Context
import android.text.format.DateFormat
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import sh.logn.app.ui.components.BalloonShape
import sh.logn.app.ui.components.BalloonStyle
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Stroke
import sh.logn.app.ui.theme.TrackMetrics
import sh.logn.core.LogN.OfflineState
import sh.logn.core.LogN.TrackView
import sh.logn.coreshell.i18n.Str
import java.util.Date
import java.util.Locale

// Peças comuns às telas de trilha. Espelho de TrackComponents.swift.

/** A cor do balão da trilha, que chega do servidor como `#RRGGBB`. */
val TrackView.tint: Color
    get() {
        val hex = color.removePrefix("#")
        val value = hex.takeIf { it.length == HEX_LENGTH }?.toLongOrNull(HEX_RADIX) ?: return LognDark.accent
        return Color(OPAQUE or value)
    }

/** "PT EN ES", na ordem do produto, só as línguas publicadas. */
val TrackView.languagesLabel: String
    get() = listOf("pt-BR" to "PT", "en" to "EN", "es" to "ES").filter { it.first in languages }.joinToString(" ") { it.second }

/** O próximo nó a jogar, a partir de 1, para "Continuar · Nó N". */
val TrackView.activeNodeIndex: Int?
    get() = nodes.indexOfFirst { it.active }.takeIf { it >= 0 }?.plus(1)

/** Nenhuma licença que valha agora, mas a trilha é da conta. */
val TrackView.isExpired: Boolean get() = owned && offline == OfflineState.EXPIRED

/** A trilha gratuita do catálogo, se houver. */
fun List<TrackView>.freeTrack(): TrackView? = firstOrNull { it.isFree }

/** O balão de uma trilha, na cor dela. */
@Composable
fun TrackBalloon(
    track: TrackView,
    width: Dp,
    modifier: Modifier = Modifier,
    dimmed: Boolean = false,
) {
    BalloonShape(
        BalloonStyle.Filled(track.tint),
        width,
        modifier.alpha(if (dimmed) TrackMetrics.DIMMED_ALPHA else 1f),
        showString = true,
        showHighlight = width >= TrackMetrics.balloonHighlightMin,
    )
}

/** O selo de estado de uma trilha: preço, comprada, dias sem rede, revogada. */
data class TrackChipStyle(
    val text: String,
    val ink: Color,
    val line: Color,
) {
    companion object {
        fun of(
            context: Context,
            track: TrackView,
            price: String?,
        ): TrackChipStyle {
            if (track.revoked) return TrackChipStyle(Str.Catalog.no_access(context), LognDark.textSecondary, LognDark.lineStrong)
            if (track.owned) {
                when (track.offline) {
                    OfflineState.SOON ->
                        return TrackChipStyle(Str.Catalog.days(context, track.offlineDaysLeft.toInt()), LognDark.warnInk, LognDark.warn)
                    OfflineState.TODAY -> return TrackChipStyle(Str.Catalog.today(context), LognDark.warnInk, LognDark.warn)
                    OfflineState.EXPIRED -> return TrackChipStyle(Str.Catalog.connect(context), LognDark.wrongInk, LognDark.wrong)
                    else -> Unit
                }
                if (track.discontinued) return TrackChipStyle(Str.Catalog.discontinued(context), LognDark.textSecondary, LognDark.lineStrong)
                return TrackChipStyle(Str.Catalog.owned(context), LognDark.correctInk, LognDark.correct)
            }
            return TrackChipStyle(price ?: Str.Catalog.see(context), LognDark.textPrimary, LognDark.lineStrong)
        }
    }
}

@Composable
fun TrackChip(
    style: TrackChipStyle,
    modifier: Modifier = Modifier,
) {
    Text(
        style.text,
        style = LognFont.mono(TrackMetrics.CHIP_SIZE, FontWeight.Medium),
        color = style.ink,
        modifier =
            modifier
                .border(Stroke.hairline, style.line, RoundedCornerShape(Radius.xs))
                .padding(horizontal = TrackMetrics.chipPaddingH, vertical = TrackMetrics.chipPaddingV),
    )
}

/** Os 30 dias da licença offline, um traço por dia: usados em cinza, os três últimos em âmbar. */
@Composable
fun OfflineDaysBar(
    usedDays: Int,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier
            .fillMaxWidth()
            .clearAndSetSemantics { },
        horizontalArrangement = Arrangement.spacedBy(TrackMetrics.dayGap),
    ) {
        for (i in 0 until OFFLINE_DAYS) {
            val color =
                when {
                    i >= usedDays -> LognDark.line
                    i >= OFFLINE_DAYS - WARN_DAYS -> LognDark.warn
                    else -> LognDark.lineDim
                }
            Box(
                Modifier
                    .weight(1f)
                    .height(TrackMetrics.dayHeight)
                    .background(color, RoundedCornerShape(TrackMetrics.dayRadius)),
            )
        }
    }
}

object TrackDates {
    /** "26 OUT": o dia até quando a licença vale, na língua do app. */
    fun short(
        unix: Long,
        locale: Locale,
    ): String = format("d MMM", unix, locale).uppercase(locale)

    /** "terça": o último dia em que conectar ainda salva a licença. */
    fun weekday(
        unix: Long,
        locale: Locale,
    ): String = format("EEEE", unix - 1, locale)

    private fun format(
        skeleton: String,
        unix: Long,
        locale: Locale,
    ): String {
        val pattern = DateFormat.getBestDateTimePattern(locale, skeleton)
        return java.text.SimpleDateFormat(pattern, locale).format(Date(unix * MILLIS))
    }

    private const val MILLIS = 1000L
}

private const val HEX_LENGTH = 6
private const val HEX_RADIX = 16
private const val OPAQUE = 0xFF000000
private const val OFFLINE_DAYS = 30
private const val WARN_DAYS = 4
