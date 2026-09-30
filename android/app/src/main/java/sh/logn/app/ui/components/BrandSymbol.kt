package sh.logn.app.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import sh.logn.app.ui.theme.LognDark

/** Os caminhos do símbolo (brand/logn-symbol, viewBox 96×122), os mesmos de iOS. */
private object BrandPaths {
    const val VIEW_WIDTH = 96f
    const val VIEW_HEIGHT = 122f
    const val BODY =
        "M48 4 C66 4 80 20 80 41 C80 60 66 74 53 79 L48 83 L43 79 C30 74 16 60 16 41 C16 20 30 4 48 4 Z"
    const val SHINE = "M31 21 C27 27 25 33 25 40"
    const val KNOT = "M41 76 L55 76 L48 91 Z"

    /** Cauda: sobe rápido, depois estabiliza. */
    const val TAIL = "M48 90 C49 104 56 111 68 113 C78 115 84 115 90 116"
    const val SHINE_WIDTH = 5.5f
    const val TAIL_WIDTH = 5f

    /** Abaixo disto o brilho vira ruído (iOS: largura menor que 24 pt). */
    const val SHINE_MIN_WIDTH_PX = 24f
}

/**
 * O símbolo do LogN: balão, nó e cauda, desenhados a partir dos caminhos do DS. A largura
 * sai da proporção da viewBox.
 */
@Composable
fun BrandSymbol(
    height: Dp,
    modifier: Modifier = Modifier,
    color: Color = LognDark.accent,
) {
    val width = height * (BrandPaths.VIEW_WIDTH / BrandPaths.VIEW_HEIGHT)
    val body = remember { PathParser().parsePathString(BrandPaths.BODY).toPath() }
    val shine = remember { PathParser().parsePathString(BrandPaths.SHINE).toPath() }
    val knot = remember { PathParser().parsePathString(BrandPaths.KNOT).toPath() }
    val tail = remember { PathParser().parsePathString(BrandPaths.TAIL).toPath() }
    Canvas(modifier.size(width, height)) {
        val s = size.height / BrandPaths.VIEW_HEIGHT
        scale(s, s, pivot = androidx.compose.ui.geometry.Offset.Zero) {
            drawPath(body, color)
            if (size.width * s >= BrandPaths.SHINE_MIN_WIDTH_PX) {
                drawPath(shine, LognDark.shine, style = Stroke(BrandPaths.SHINE_WIDTH, cap = StrokeCap.Round))
            }
            drawPath(knot, color)
            drawPath(tail, color, style = Stroke(BrandPaths.TAIL_WIDTH, cap = StrokeCap.Round))
        }
    }
}
