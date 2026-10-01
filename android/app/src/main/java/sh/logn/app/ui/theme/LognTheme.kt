package sh.logn.app.ui.theme

import android.provider.Settings
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.platform.LocalContext

/** Animação reduzida no sistema (escala de animação 0): o `reduceMotion` de iOS. */
val LocalReduceMotion = staticCompositionLocalOf { false }

private val LognColorScheme =
    darkColorScheme(
        primary = LognDark.accent,
        onPrimary = LognDark.onAccent,
        background = LognDark.canvas,
        onBackground = LognDark.textPrimary,
        surface = LognDark.surface,
        onSurface = LognDark.textPrimary,
        surfaceVariant = LognDark.surfaceRaised,
        onSurfaceVariant = LognDark.textSecondary,
        outline = LognDark.line,
        outlineVariant = LognDark.lineStrong,
        error = LognDark.wrong,
    )

/**
 * O tema do app. Escuro sempre, como iOS com `.preferredColorScheme(.dark)`. As telas
 * leem `LognDark` e `LognFont` direto; o esquema de Material existe para os componentes
 * do sistema (folhas, seleção de texto) não saírem com as cores padrão.
 */
@Composable
fun LognTheme(content: @Composable () -> Unit) {
    val context = LocalContext.current
    val reduceMotion =
        Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f
    CompositionLocalProvider(LocalReduceMotion provides reduceMotion) {
        MaterialTheme(colorScheme = LognColorScheme, content = content)
    }
}
