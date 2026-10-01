package sh.logn.app.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.Dp
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.ProviderColor

// Os mesmos SVG do Assets.xcassets de iOS (GoogleIcon, GitHubIcon), com as bandeiras dos
// arcos separadas por espaço para o parser.

private val googleParts =
    listOf(
        ProviderColor.googleBlue to
            "M22 12.2 c0 -0.7 -0.06 -1.4 -0.18 -2 H12 v3.9 h5.6 a4.8 4.8 0 0 1 -2.08 3.14 v2.6 h3.36 C20.84 18 22 15.4 22 12.2 z",
        ProviderColor.googleGreen to
            "M12 22 c2.7 0 4.97 -0.9 6.63 -2.43 l-3.36 -2.6 c-0.93 0.63 -2.12 1 -3.27 1 c-2.6 0 -4.8 -1.75 -5.6 -4.1 H2.94 v2.58 A10 10 0 0 0 12 22 z",
        ProviderColor.googleYellow to "M6.4 13.87 a6 6 0 0 1 0 -3.74 V7.55 H2.94 a10 10 0 0 0 0 8.9 l3.46 -2.58 z",
        ProviderColor.googleRed to
            "M12 5.98 c1.47 0 2.79 0.5 3.83 1.5 l2.87 -2.87 C16.96 2.99 14.7 2 12 2 a10 10 0 0 0 -9.06 5.55 L6.4 10.13 c0.8 -2.35 3 -4.15 5.6 -4.15 z",
    )

private const val GITHUB =
    "M12 2 a10 10 0 0 0 -3.16 19.49 c0.5 0.09 0.68 -0.22 0.68 -0.48 l-0.01 -1.7 c-2.78 0.6 -3.37 -1.34 -3.37 -1.34 " +
        "c-0.45 -1.16 -1.11 -1.47 -1.11 -1.47 c-0.91 -0.62 0.07 -0.6 0.07 -0.6 c1 0.07 1.53 1.03 1.53 1.03 " +
        "c0.9 1.53 2.34 1.09 2.91 0.83 c0.09 -0.65 0.35 -1.09 0.63 -1.34 c-2.22 -0.25 -4.56 -1.11 -4.56 -4.94 " +
        "c0 -1.09 0.39 -1.98 1.03 -2.68 c-0.1 -0.25 -0.45 -1.27 0.1 -2.64 c0 0 0.84 -0.27 2.75 1.02 a9.5 9.5 0 0 1 5 0 " +
        "c1.91 -1.29 2.75 -1.02 2.75 -1.02 c0.55 1.37 0.2 2.39 0.1 2.64 c0.64 0.7 1.03 1.59 1.03 2.68 " +
        "c0 3.84 -2.34 4.69 -4.57 4.94 c0.36 0.31 0.68 0.92 0.68 1.85 l-0.01 2.75 c0 0.27 0.18 0.58 0.69 0.48 A10 10 0 0 0 12 2 z"

@Composable
private fun FilledIcon(
    parts: List<Pair<Color, String>>,
    size: Dp,
    modifier: Modifier,
) {
    val paths = remember(parts) { parts.map { (color, d) -> color to PathParser().parsePathString(d).toPath() } }
    Canvas(modifier.size(size).clearAndSetSemantics { }) {
        val s = this.size.width / GRID
        scale(s, s, pivot = Offset.Zero) { for ((color, path) in paths) drawPath(path, color) }
    }
}

@Composable
fun GoogleIcon(
    size: Dp,
    modifier: Modifier = Modifier,
) = FilledIcon(googleParts, size, modifier)

@Composable
fun GitHubIcon(
    size: Dp,
    modifier: Modifier = Modifier,
) = FilledIcon(remember { listOf(LognDark.textPrimary to GITHUB) }, size, modifier)

private const val GRID = 24f
