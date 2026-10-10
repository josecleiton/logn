package sh.logn.app.ui.theme

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

// Porta de ios/LogNiOS/LogNiOS/DesignSystem/LognDesignSystem.swift, do LogN Design System
// v2. Não invente cores fora deste arquivo: o portão `just android/tokens-gate` barra
// `Color(0x…)` no resto do app. As telas leem `LognColors`, a paleta do tema em vigor,
// nunca `LognDark` direto.

/** As cores de um tema. Cinco têm par base/tinta: a base preenche e contorna; a `…Ink` escreve. */
@Immutable
class LognPalette(
    val canvas: Color,
    val surface: Color,
    val surfaceRaised: Color,
    val line: Color,
    val lineStrong: Color,
    val textPrimary: Color,
    val textSecondary: Color,
    val textMuted: Color,
    val textDim: Color,
    val lineDim: Color,
    val heartOff: Color,
    val rowLine: Color,
    val buttonDisabled: Color,
    val accent: Color,
    val accentInk: Color,
    val onAccent: Color,
    val correct: Color,
    val wrong: Color,
    val warn: Color,
    val info: Color,
    val correctInk: Color,
    val wrongInk: Color,
    val warnInk: Color,
    val infoInk: Color,
    val tintOk: Color,
    val tintErr: Color,
    val tintWarn: Color,
    val tintInfo: Color,
    val synKeyword: Color,
    val synFunction: Color,
    /** Acento sobre canvas: fundo de seleção (linha escolhida, tag marcada, nó ativo). */
    val accentTint: Color,
    /** O brilho do balão, branco nos dois temas. */
    val shine: Color,
)

val LognDark =
    LognPalette(
        canvas = Color(0xFF0B0C0D),
        surface = Color(0xFF111316),
        surfaceRaised = Color(0xFF171A1E),
        line = Color(0xFF24282D),
        lineStrong = Color(0xFF343A41),
        textPrimary = Color(0xFFEDEEEF),
        textSecondary = Color(0xFF99A0A7),
        textMuted = Color(0xFF7E858D),
        textDim = Color(0xFF5F656C),
        lineDim = Color(0xFF4C535B),
        heartOff = Color(0xFF2E3238),
        rowLine = Color(0xFF16191C),
        buttonDisabled = Color(0xFF1B1D20),
        accent = Color(0xFFFF7A45),
        accentInk = Color(0xFFFF7A45),
        onAccent = Color(0xFF160B05),
        correct = Color(0xFF3DD68C),
        wrong = Color(0xFFFF5C5C),
        warn = Color(0xFFF5C451),
        info = Color(0xFF5AA9FF),
        correctInk = Color(0xFF3DD68C),
        wrongInk = Color(0xFFFF5C5C),
        warnInk = Color(0xFFF5C451),
        infoInk = Color(0xFF5AA9FF),
        tintOk = Color(0xFF0F2018),
        tintErr = Color(0xFF231113),
        tintWarn = Color(0xFF221C0C),
        tintInfo = Color(0xFF0D1B2B),
        synKeyword = Color(0xFFC792EA),
        synFunction = Color(0xFF82AAFF),
        accentTint = Color(0xFF241610),
        shine = Color(0xB8FFFFFF),
    )

/** O tema em vigor, que `LognTheme` fornece. */
val LocalLognPalette = staticCompositionLocalOf { LognDark }

/** As cores do tema em vigor. Fora de composição (num `drawBehind`), leia antes e passe o valor. */
object LognColors {
    val palette: LognPalette
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current
    val canvas: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.canvas
    val surface: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.surface
    val surfaceRaised: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.surfaceRaised
    val line: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.line
    val lineStrong: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.lineStrong
    val textPrimary: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.textPrimary
    val textSecondary: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.textSecondary
    val textMuted: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.textMuted
    val textDim: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.textDim
    val lineDim: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.lineDim
    val heartOff: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.heartOff
    val rowLine: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.rowLine
    val buttonDisabled: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.buttonDisabled
    val accent: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.accent
    val accentInk: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.accentInk
    val onAccent: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.onAccent
    val correct: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.correct
    val wrong: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.wrong
    val warn: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.warn
    val info: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.info
    val correctInk: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.correctInk
    val wrongInk: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.wrongInk
    val warnInk: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.warnInk
    val infoInk: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.infoInk
    val tintOk: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.tintOk
    val tintErr: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.tintErr
    val tintWarn: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.tintWarn
    val tintInfo: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.tintInfo
    val synKeyword: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.synKeyword
    val synFunction: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.synFunction
    val accentTint: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.accentTint
    val shine: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.shine
}

/** As cores de marca dos provedores de login, das diretrizes de cada um. */
object ProviderColor {
    val googleBlue = Color(0xFF4285F4)
    val googleGreen = Color(0xFF34A853)
    val googleYellow = Color(0xFFFBBC05)
    val googleRed = Color(0xFFEA4335)
}

/** Balões A–M. Identidade categórica do problema, nunca estado. */
object Balloon {
    private val dark =
        listOf(
            Color(0xFFE4572E), // A · vermelho
            Color(0xFFF5C451), // B · amarelo
            Color(0xFF3DB2FF), // C · azul
            Color(0xFF6BCB77), // D · verde
            Color(0xFFC77DFF), // E · violeta
            Color(0xFFFF6FB5), // F · rosa
            Color(0xFF4ECDC4), // G · turquesa
            Color(0xFFF4A261), // H · âmbar
            Color(0xFF9BC53D), // I · lima
            Color(0xFFD64550), // J · carmim
            Color(0xFF7C8BFF), // K · índigo
            Color(0xFFD8DEE4), // L · prata
            Color(0xFF00B894), // M · esmeralda
        )

    fun of(letter: Char): Color = dark[(letter.uppercaseChar() - 'A').coerceIn(0, dark.lastIndex)]
}
