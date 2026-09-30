package sh.logn.app.ui.theme

import androidx.compose.ui.graphics.Color

// Porta de ios/LogNiOS/LogNiOS/DesignSystem/LognDesignSystem.swift, do LogN Design System
// v2. Não invente cores fora deste arquivo: o portão `just android/tokens-gate` barra
// `Color(0x…)` no resto do app.

object LognDark {
    val canvas = Color(0xFF0B0C0D)
    val surface = Color(0xFF111316)
    val surfaceRaised = Color(0xFF171A1E)
    val line = Color(0xFF24282D)
    val lineStrong = Color(0xFF343A41)
    val textPrimary = Color(0xFFEDEEEF)
    val textSecondary = Color(0xFF99A0A7)
    val textMuted = Color(0xFF7E858D)
    val textDim = Color(0xFF5F656C)
    val lineDim = Color(0xFF4C535B)
    val heartOff = Color(0xFF2E3238)
    val rowLine = Color(0xFF16191C)
    val buttonDisabled = Color(0xFF1B1D20)
    val accent = Color(0xFFFF7A45)
    val accentInk = Color(0xFFFF7A45)
    val onAccent = Color(0xFF160B05)
    val correct = Color(0xFF3DD68C)
    val wrong = Color(0xFFFF5C5C)
    val warn = Color(0xFFF5C451)
    val info = Color(0xFF5AA9FF)
    val correctInk = Color(0xFF3DD68C)
    val wrongInk = Color(0xFFFF5C5C)
    val warnInk = Color(0xFFF5C451)
    val infoInk = Color(0xFF5AA9FF)
    val tintOk = Color(0xFF0F2018)
    val tintErr = Color(0xFF231113)
    val tintWarn = Color(0xFF221C0C)
    val tintInfo = Color(0xFF0D1B2B)
    val synKeyword = Color(0xFFC792EA)
    val synFunction = Color(0xFF82AAFF)

    /** Acento sobre canvas: fundo de seleção (linha escolhida, tag marcada, nó ativo). */
    val accentTint = Color(0xFF241610)

    /** O brilho do balão. */
    val shine = Color(0xB8FFFFFF)
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
