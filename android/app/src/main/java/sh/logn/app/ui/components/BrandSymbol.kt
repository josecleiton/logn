package sh.logn.app.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import sh.logn.app.ui.theme.BrandMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.coreshell.i18n.Str

/** Os caminhos do símbolo (brand/logn-symbol, viewBox 96×122), os mesmos de iOS. */
private object BrandPaths {
    const val VIEW_WIDTH = 96f
    const val VIEW_HEIGHT = 122f

    /** Cauda: sobe rápido, depois estabiliza. */
    const val TAIL = "M48 90 C49 104 56 111 68 113 C78 115 84 115 90 116"
    const val TAIL_WIDTH = 5f
}

/**
 * O símbolo do LogN: o balão de UI com a cauda logarítmica no lugar da cordinha. A largura
 * sai da proporção da viewBox.
 */
@Composable
fun BrandSymbol(
    height: Dp,
    modifier: Modifier = Modifier,
    color: Color = LognDark.accent,
) {
    val width = height * (BrandPaths.VIEW_WIDTH / BrandPaths.VIEW_HEIGHT)
    val body = remember { PathParser().parsePathString(BalloonPaths.BODY).toPath() }
    val shine = remember { PathParser().parsePathString(BalloonPaths.SHINE).toPath() }
    val knot = remember { PathParser().parsePathString(BalloonPaths.KNOT).toPath() }
    val tail = remember { PathParser().parsePathString(BrandPaths.TAIL).toPath() }
    Canvas(modifier.size(width, height)) {
        // Medido antes do `scale`: dentro dele, `size` ainda é o da tela.
        val showShine = size.width >= BrandMetrics.shineMinWidth.toPx()
        val s = size.height / BrandPaths.VIEW_HEIGHT
        scale(s, s, pivot = Offset.Zero) {
            drawPath(body, color)
            if (showShine) {
                drawPath(shine, LognDark.shine, style = Stroke(BalloonPaths.SHINE_WIDTH, cap = StrokeCap.Round))
            }
            drawPath(knot, color)
            drawPath(tail, color, style = Stroke(BrandPaths.TAIL_WIDTH, cap = StrokeCap.Round))
        }
    }
}

/** Lockup horizontal: símbolo e "LogN" em Plex Sans SemiBold, gap 14. */
@Composable
fun BrandLockup(
    fontSize: Float,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val name = Str.App.name(context)
    Row(
        modifier.clearAndSetSemantics { contentDescription = name },
        horizontalArrangement = Arrangement.spacedBy(BrandMetrics.lockupGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        BrandSymbol(height = (fontSize * BrandMetrics.LOCKUP_SYMBOL_RATIO).dp)
        Text(
            name,
            style = LognFont.sans(fontSize, FontWeight.SemiBold, BrandMetrics.LOCKUP_TRACKING),
            color = LognDark.textPrimary,
        )
    }
}
