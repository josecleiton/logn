package sh.logn.app.ui.theme

import android.provider.Settings
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.platform.LocalContext

/** Animação reduzida no sistema (escala de animação 0): o `reduceMotion` de iOS. */
val LocalReduceMotion = staticCompositionLocalOf { false }

private fun LognPalette.materialScheme(): ColorScheme =
    if (isLight) {
        lightColorScheme(
            primary = accent,
            onPrimary = onAccent,
            background = canvas,
            onBackground = textPrimary,
            surface = surface,
            onSurface = textPrimary,
            surfaceVariant = surfaceRaised,
            onSurfaceVariant = textSecondary,
            outline = line,
            outlineVariant = lineStrong,
            error = wrong,
        )
    } else {
        darkColorScheme(
            primary = accent,
            onPrimary = onAccent,
            background = canvas,
            onBackground = textPrimary,
            surface = surface,
            onSurface = textPrimary,
            surfaceVariant = surfaceRaised,
            onSurfaceVariant = textSecondary,
            outline = line,
            outlineVariant = lineStrong,
            error = wrong,
        )
    }

private val DarkScheme = LognDark.materialScheme()
private val LightScheme = LognLight.materialScheme()

/**
 * O tema do app: claro ou escuro, como o sistema (ADR 0030). Trocar o tema do sistema recria
 * a Activity, que resolve de novo a janela, a splash e as barras (`values-night`). As telas
 * leem `LognColors` e `LognFont`; o esquema de Material existe para os componentes do
 * sistema (folhas, seleção de texto) não saírem com as cores padrão.
 */
@Composable
fun LognTheme(
    dark: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    val context = LocalContext.current
    val reduceMotion =
        Settings.Global.getFloat(context.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f
    CompositionLocalProvider(LocalReduceMotion provides reduceMotion, LocalLognPalette provides if (dark) LognDark else LognLight) {
        MaterialTheme(colorScheme = if (dark) DarkScheme else LightScheme, content = content)
    }
}
