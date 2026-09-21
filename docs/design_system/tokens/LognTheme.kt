package app.logn.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.runtime.*
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

val PlexSans = FontFamily(
    Font(R.font.ibm_plex_sans_regular, FontWeight.Normal),
    Font(R.font.ibm_plex_sans_medium, FontWeight.Medium),
    Font(R.font.ibm_plex_sans_semibold, FontWeight.SemiBold)
)
val PlexMono = FontFamily(
    Font(R.font.ibm_plex_mono_regular, FontWeight.Normal),
    Font(R.font.ibm_plex_mono_medium, FontWeight.Medium)
)

val LognTypography = Typography(
    displayLarge   = TextStyle(PlexSans, 40.sp, FontWeight.SemiBold, lineHeight = 44.sp, letterSpacing = (-1.2).sp),
    headlineMedium = TextStyle(PlexSans, 24.sp, FontWeight.SemiBold, lineHeight = 30.sp, letterSpacing = (-0.5).sp),
    titleMedium    = TextStyle(PlexSans, 19.sp, FontWeight.SemiBold, lineHeight = 25.sp),
    bodyLarge      = TextStyle(PlexSans, 17.sp, FontWeight.Normal,   lineHeight = 26.sp),
    bodyMedium     = TextStyle(PlexSans, 15.sp, FontWeight.Normal,   lineHeight = 22.sp)
)

// fora do Typography do M3, porque o M3 não tem slot para código
val CodeStyle  = TextStyle(PlexMono, 15.sp, FontWeight.Normal, lineHeight = 24.sp)
val LabelStyle = TextStyle(PlexMono, 11.sp, FontWeight.Medium, lineHeight = 16.sp, letterSpacing = 1.54.sp)

val LognShapes = Shapes(
    extraSmall = androidx.compose.foundation.shape.RoundedCornerShape(2.dp),
    small      = androidx.compose.foundation.shape.RoundedCornerShape(4.dp),
    medium     = androidx.compose.foundation.shape.RoundedCornerShape(8.dp)
)

object Space {
    val xs = 4.dp;  val sm = 8.dp;  val md = 12.dp
    val lg = 16.dp; val xl = 24.dp; val xxl = 32.dp; val xxxl = 48.dp
    val screenMargin = 20.dp
    val listGap = 8.dp
    val minTouch = 48.dp      // piso Android
    val matchTouch = 56.dp    // alvos em partida
}

@Composable
fun LognTheme(dark: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    val c = if (dark) LognDark else LognLight
    MaterialTheme(
        colorScheme = darkColorScheme(
            background = c.canvas, surface = c.surface, surfaceVariant = c.surfaceRaised,
            onBackground = c.textPrimary, onSurface = c.textPrimary,
            primary = c.accent, onPrimary = c.onAccent,
            error = c.wrong, outline = c.line
        ),
        typography = LognTypography,
        shapes = LognShapes,
        content = content
    )
}
