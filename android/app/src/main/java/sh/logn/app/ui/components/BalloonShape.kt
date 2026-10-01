package sh.logn.app.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.Dp
import sh.logn.app.ui.theme.Balloon
import sh.logn.app.ui.theme.BalloonMetrics
import sh.logn.app.ui.theme.LognDark

/** Os caminhos do balão do DS, os mesmos de BalloonShape.swift. */
internal object BalloonPaths {
    const val BODY =
        "M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z"
    const val KNOT = "M41 76 L55 76 L48 91 Z"
    const val SHINE = "M31 21 C27 27 25 33 25 40"
    const val STRING = "M48 90 C38 98 58 106 48 114 C38 122 58 130 48 138"
    const val SHINE_WIDTH = 5.5f
    const val STRING_WIDTH = 5f
    const val ACTIVE_STROKE = 5f
    const val VIEW_WIDTH = 96f
    const val VIEW_HEIGHT_SHORT = 95f
    const val VIEW_HEIGHT_TALL = 150f

    /** O brilho some abaixo de 12 de corpo; o corpo é 2/3 da viewBox. */
    const val SHINE_MIN_BODY = 12f
    const val BODY_RATIO = 64f / 96f
    const val SHINE_ALPHA = 0.72f
    const val ACTIVE_SHINE_ALPHA = 0.45f
}

/** Como o balão aparece. */
@Immutable
sealed interface BalloonStyle {
    /** Aceito ou conquistado: corpo cheio na cor. */
    data class Filled(
        val color: Color,
    ) : BalloonStyle

    /** Em aberto: contorno na cor, com a espessura em unidades de viewBox. */
    data class Outline(
        val color: Color,
        val lineWidth: Float,
    ) : BalloonStyle

    /** Nó ativo: contorno accent de traço 5 e miolo `accentTint`. */
    data object Active : BalloonStyle

    /** Bloqueado: corpo inteiro em `lineStrong`, sem traço nem brilho. */
    data object Locked : BalloonStyle
}

/**
 * A primitiva do balão. `width` é a largura da viewBox, não do corpo, como o design
 * especifica cada uso; a altura sai da proporção.
 */
@Composable
fun BalloonShape(
    style: BalloonStyle,
    width: Dp,
    modifier: Modifier = Modifier,
    showString: Boolean = false,
    showHighlight: Boolean = true,
) {
    val viewHeight = if (showString) BalloonPaths.VIEW_HEIGHT_TALL else BalloonPaths.VIEW_HEIGHT_SHORT
    val height = width * (viewHeight / BalloonPaths.VIEW_WIDTH)
    val body = remember { PathParser().parsePathString(BalloonPaths.BODY).toPath() }
    val knot = remember { PathParser().parsePathString(BalloonPaths.KNOT).toPath() }
    val shine = remember { PathParser().parsePathString(BalloonPaths.SHINE).toPath() }
    val string = remember { PathParser().parsePathString(BalloonPaths.STRING).toPath() }
    Canvas(modifier.size(width, height).clearAndSetSemantics { }) {
        val s = size.width / BalloonPaths.VIEW_WIDTH
        // Em dp, como os pontos de iOS.
        val bodyWidth = width.value * BalloonPaths.BODY_RATIO
        val shineOn = showHighlight && bodyWidth >= BalloonPaths.SHINE_MIN_BODY
        scale(s, s, pivot = Offset.Zero) {
            val stringColor: Color
            when (style) {
                is BalloonStyle.Filled -> {
                    drawPath(body, style.color)
                    if (shineOn) drawPath(shine, Color.White.copy(alpha = BalloonPaths.SHINE_ALPHA), style = shineStroke)
                    drawPath(knot, style.color)
                    stringColor = style.color
                }
                is BalloonStyle.Outline -> {
                    drawPath(body, style.color, style = Stroke(style.lineWidth))
                    drawPath(knot, style.color)
                    stringColor = style.color
                }
                BalloonStyle.Active -> {
                    drawPath(body, LognDark.accentTint)
                    drawPath(body, LognDark.accent, style = Stroke(BalloonPaths.ACTIVE_STROKE))
                    if (shineOn) {
                        drawPath(shine, LognDark.accent.copy(alpha = BalloonPaths.ACTIVE_SHINE_ALPHA), style = shineStroke)
                    }
                    drawPath(knot, LognDark.accent)
                    stringColor = LognDark.accent
                }
                BalloonStyle.Locked -> {
                    drawPath(body, LognDark.lineStrong)
                    drawPath(knot, LognDark.lineStrong)
                    stringColor = LognDark.lineStrong
                }
            }
            if (showString) {
                drawPath(string, stringColor, style = Stroke(BalloonPaths.STRING_WIDTH, cap = StrokeCap.Round))
            }
        }
    }
}

private val shineStroke = Stroke(BalloonPaths.SHINE_WIDTH, cap = StrokeCap.Round)

/** A cor da letra do problema: endereço, nunca estado. Fora de A–M, `textMuted`. */
fun balloonColor(letter: Char): Color = if (letter.uppercaseChar() in 'A'..'M') Balloon.of(letter) else LognDark.textMuted

val BALLOON_LETTERS: List<Char> = ('A'..'M').toList()

/** A fileira decorativa A–M das telas de entrada: a paleta inteira se apresentando. */
@Composable
fun BalloonMarquee(modifier: Modifier = Modifier) {
    Row(
        modifier.alpha(BalloonMetrics.MARQUEE_ALPHA).clearAndSetSemantics { },
        horizontalArrangement = Arrangement.spacedBy(BalloonMetrics.marqueeGap),
    ) {
        for (letter in BALLOON_LETTERS) {
            BalloonShape(BalloonStyle.Filled(balloonColor(letter)), BalloonMetrics.marqueeWidth)
        }
    }
}
