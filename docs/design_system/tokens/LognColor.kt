package app.logn.ui.theme

import androidx.compose.ui.graphics.Color

// Gerado a partir do LogN Design System v2. Não invente cores fora deste arquivo.

object LognDark {
    val canvas          = Color(0xFF0B0C0D)
    val surface         = Color(0xFF111316)
    val surfaceRaised   = Color(0xFF171A1E)
    val line            = Color(0xFF24282D)
    val lineStrong      = Color(0xFF343A41)
    val textPrimary     = Color(0xFFEDEEEF)
    val textSecondary   = Color(0xFF99A0A7)
    val textMuted       = Color(0xFF7E858D)
    val textDim         = Color(0xFF5F656C)
    val lineDim         = Color(0xFF4C535B)
    val heartOff        = Color(0xFF2E3238)
    val rowLine         = Color(0xFF16191C)
    val buttonDisabled  = Color(0xFF1B1D20)
    val accent          = Color(0xFFFF7A45)
    val accentInk       = Color(0xFFFF7A45)
    val onAccent        = Color(0xFF160B05)
    val correct         = Color(0xFF3DD68C)
    val wrong           = Color(0xFFFF5C5C)
    val warn            = Color(0xFFF5C451)
    val info            = Color(0xFF5AA9FF)
    val correctInk      = Color(0xFF3DD68C)
    val wrongInk        = Color(0xFFFF5C5C)
    val warnInk         = Color(0xFFF5C451)
    val infoInk         = Color(0xFF5AA9FF)
    val tintOk          = Color(0xFF0F2018)
    val tintErr         = Color(0xFF231113)
    val tintWarn        = Color(0xFF221C0C)
    val tintInfo        = Color(0xFF0D1B2B)
    val synKeyword      = Color(0xFFC792EA)
    val synFunction     = Color(0xFF82AAFF)
}

object LognLight {
    val canvas          = Color(0xFFF6F6F4)
    val surface         = Color(0xFFFFFFFF)
    val surfaceRaised   = Color(0xFFF0F1EE)
    val line            = Color(0xFFE2E3DF)
    val lineStrong      = Color(0xFFC6C8C2)
    val textPrimary     = Color(0xFF14161A)
    val textSecondary   = Color(0xFF555B62)
    val textMuted       = Color(0xFF656B72)
    val textDim         = Color(0xFF7C838A)
    val lineDim         = Color(0xFFA9AFB5)
    val heartOff        = Color(0xFFD6D8D3)
    val rowLine         = Color(0xFFECEDE9)
    val buttonDisabled  = Color(0xFFE6E7E3)
    val accent          = Color(0xFFFF7A45)
    val accentInk       = Color(0xFFA83C0B)
    val onAccent        = Color(0xFF160B05)
    val correct         = Color(0xFF0E8F52)
    val wrong           = Color(0xFFC93636)
    val warn            = Color(0xFF8A5B00)
    val info            = Color(0xFF1660C4)
    val correctInk      = Color(0xFF0A6B3C)
    val wrongInk        = Color(0xFFA82424)
    val warnInk         = Color(0xFF6E4800)
    val infoInk         = Color(0xFF124F9E)
    val tintOk          = Color(0xFFE6F5EC)
    val tintErr         = Color(0xFFFBEAEA)
    val tintWarn        = Color(0xFFFAF1DC)
    val tintInfo        = Color(0xFFE6EFFB)
    val synKeyword      = Color(0xFF7A28C4)
    val synFunction     = Color(0xFF0A4FA8)
}

/**
 * Balões A—M. Identidade categórica do problema, NUNCA estado.
 * A cor da letra é fixa durante todo o contest.
 */
object Balloon {
    val dark = listOf(
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
    val light = listOf(
        Color(0xFFC43F19), // A · vermelho
        Color(0xFFA67A00), // B · amarelo
        Color(0xFF0B6FBF), // C · azul
        Color(0xFF2E8B45), // D · verde
        Color(0xFF8A3FD1), // E · violeta
        Color(0xFFC2367E), // F · rosa
        Color(0xFF18867E), // G · turquesa
        Color(0xFFB26320), // H · âmbar
        Color(0xFF5F8410), // I · lima
        Color(0xFFA3202B), // J · carmim
        Color(0xFF4352C9), // K · índigo
        Color(0xFF5B646D), // L · prata
        Color(0xFF007A61), // M · esmeralda
    )
    fun of(letter: Char, isLight: Boolean = false): Color =
        (if (isLight) light else dark)[letter.uppercaseChar() - 'A']
}
