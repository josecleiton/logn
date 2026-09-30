package sh.logn.app.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.unit.Dp

/** Os sinais do log de juiz, que iOS desenha com SF Symbols. */
enum class Glyph { Ellipsis, Check, Exclamation, Cross, Minus }

/**
 * Um sinal pequeno e grosso, desenhado em vez de caractere: a Plex Mono não tem o ✓, e
 * o sistema trocaria por um sinal parecido com √ (o mesmo motivo de iOS).
 */
@Composable
fun StatusGlyph(
    glyph: Glyph,
    color: Color,
    size: Dp,
    modifier: Modifier = Modifier,
) {
    Canvas(modifier.size(size)) {
        val w = this.size.width
        val stroke = w * STROKE_RATIO

        fun line(
            x0: Float,
            y0: Float,
            x1: Float,
            y1: Float,
        ) = drawLine(color, Offset(w * x0, w * y0), Offset(w * x1, w * y1), stroke, StrokeCap.Round)
        when (glyph) {
            Glyph.Check -> {
                line(0.15f, 0.55f, 0.4f, 0.8f)
                line(0.4f, 0.8f, 0.85f, 0.25f)
            }
            Glyph.Cross -> {
                line(0.2f, 0.2f, 0.8f, 0.8f)
                line(0.8f, 0.2f, 0.2f, 0.8f)
            }
            Glyph.Minus -> line(0.2f, 0.5f, 0.8f, 0.5f)
            Glyph.Exclamation -> {
                line(0.5f, 0.12f, 0.5f, 0.6f)
                drawCircle(color, stroke * DOT_RATIO, Offset(w * 0.5f, w * 0.86f))
            }
            Glyph.Ellipsis ->
                listOf(0.18f, 0.5f, 0.82f).forEach { drawCircle(color, stroke * DOT_RATIO, Offset(w * it, w * 0.5f)) }
        }
    }
}

private const val STROKE_RATIO = 0.18f
private const val DOT_RATIO = 0.7f
