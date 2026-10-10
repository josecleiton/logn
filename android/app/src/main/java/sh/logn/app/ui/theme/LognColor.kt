package sh.logn.app.ui.theme

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

// Porta de ios/LogNiOS/LogNiOS/DesignSystem/LognDesignSystem.swift, do LogN Design System
// v2. Não invente cores fora deste arquivo: o portão `just android/tokens-gate` barra
// `Color(0x…)` no resto do app. As telas leem `LognColors`, a paleta do tema em vigor,
// nunca `LognDark` ou `LognLight` direto.
//
// O claro é derivado, não redesenhado (ADR 0030): a escala de superfície inverte, o acento
// é o mesmo hex, e veredito e sintaxe escurecem para manter 4.5:1. No claro vale o WCAG AA
// em cada par que aparece na tela; onde o token do DS não passa, a tela troca de token,
// e os dois papéis do fim da paleta dizem qual.

/** As cores de um tema. Cinco têm par base/tinta: a base preenche e contorna; a `…Ink` escreve. */
@Immutable
class LognPalette(
    val isLight: Boolean,
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
    /**
     * O texto mais apagado que ainda se lê: o exemplo dentro do campo, a versão do app.
     * `textDim` no escuro, como o DS desenhou; `textMuted` no claro, onde `textDim` fica em
     * 3.6:1 e o DS o reserva a forma.
     */
    val textFaint: Color,
    /**
     * A tinta sobre um preenchimento cheio que escurece no claro (`warn`, `wrong`): a tarja
     * sem rede e o botão de excluir. `onAccent` no escuro; `canvas` no claro, onde o texto
     * escuro cairia a 3.3:1 (o mock de Validade Offline já escreve em `canvas`).
     */
    val onStatus: Color,
)

val LognDark =
    LognPalette(
        isLight = false,
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
        textFaint = Color(0xFF5F656C),
        onStatus = Color(0xFF160B05),
    )

val LognLight =
    LognPalette(
        isLight = true,
        canvas = Color(0xFFF6F6F4),
        surface = Color(0xFFFFFFFF),
        surfaceRaised = Color(0xFFF0F1EE),
        line = Color(0xFFE2E3DF),
        lineStrong = Color(0xFFC6C8C2),
        textPrimary = Color(0xFF14161A),
        textSecondary = Color(0xFF555B62),
        textMuted = Color(0xFF656B72),
        textDim = Color(0xFF7C838A),
        lineDim = Color(0xFFA9AFB5),
        heartOff = Color(0xFFD6D8D3),
        rowLine = Color(0xFFECEDE9),
        buttonDisabled = Color(0xFFE6E7E3),
        accent = Color(0xFFFF7A45),
        accentInk = Color(0xFFA83C0B),
        onAccent = Color(0xFF160B05),
        correct = Color(0xFF0E8F52),
        wrong = Color(0xFFC93636),
        warn = Color(0xFF8A5B00),
        info = Color(0xFF1660C4),
        correctInk = Color(0xFF0A6B3C),
        wrongInk = Color(0xFFA82424),
        warnInk = Color(0xFF6E4800),
        infoInk = Color(0xFF124F9E),
        tintOk = Color(0xFFE6F5EC),
        tintErr = Color(0xFFFBEAEA),
        tintWarn = Color(0xFFFAF1DC),
        tintInfo = Color(0xFFE6EFFB),
        synKeyword = Color(0xFF7A28C4),
        synFunction = Color(0xFF0A4FA8),
        accentTint = Color(0xFFFDE9DF),
        shine = Color(0xB8FFFFFF),
        textFaint = Color(0xFF656B72),
        onStatus = Color(0xFFF6F6F4),
    )

/** O tema em vigor, que `LognTheme` escolhe pelo sistema. */
val LocalLognPalette = staticCompositionLocalOf { LognDark }

/** As cores do tema em vigor. Fora de composição (num `drawBehind`), leia antes e passe o valor. */
object LognColors {
    val palette: LognPalette
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current
    val isLight: Boolean
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.isLight
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
    val textFaint: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.textFaint
    val onStatus: Color
        @Composable @ReadOnlyComposable
        get() = LocalLognPalette.current.onStatus
}

/** As cores de marca dos provedores de login, das diretrizes de cada um. */
object ProviderColor {
    val googleBlue = Color(0xFF4285F4)
    val googleGreen = Color(0xFF34A853)
    val googleYellow = Color(0xFFFBBC05)
    val googleRed = Color(0xFFEA4335)
}

/**
 * Balões A–M. Identidade categórica do problema, nunca estado. Cada letra tem duas cores:
 * o corpo (`fill`) e o traço (`edge`), que é o contorno do balão cheio, o balão em aberto,
 * o nó, a cordinha, a aresta do mapa e o ícone de traço.
 *
 * No escuro as duas são a mesma. No claro o corpo continua a cor viva do escuro, e o traço
 * é ela com o mesmo matiz e croma em OKLCH, escurecida até 3:1 sobre a folha do nó, o fundo
 * mais escuro onde um balão aparece: o contorno segura a borda do corpo claro sobre o
 * branco. Escurecer o corpo inteiro até 3:1 virava laranja em marrom e amarelo em mostarda.
 * J clareia um fio no claro para a letra escura dentro dele chegar a 4.5:1.
 */
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

    private val lightFill = dark.mapIndexed { i, color -> if (i == 'J' - 'A') Color(0xFFD84751) else color }

    private val lightEdge =
        listOf(
            Color(0xFFE4572E), // A · vermelho
            Color(0xFFB28300), // B · amarelo
            Color(0xFF0092DD), // C · azul
            Color(0xFF3C9D4C), // D · verde
            Color(0xFFB369EA), // E · violeta
            Color(0xFFE4569D), // F · rosa
            Color(0xFF009B93), // G · turquesa
            Color(0xFFC77836), // H · âmbar
            Color(0xFF709700), // I · lima
            Color(0xFFD84751), // J · carmim
            Color(0xFF7280F3), // K · índigo
            Color(0xFF868C91), // L · prata
            Color(0xFF009D7B), // M · esmeralda
        )

    fun fill(
        letter: Char,
        isLight: Boolean,
    ): Color = pick(if (isLight) lightFill else dark, letter)

    fun edge(
        letter: Char,
        isLight: Boolean,
    ): Color = pick(if (isLight) lightEdge else dark, letter)

    @Composable
    @ReadOnlyComposable
    fun fill(letter: Char): Color = fill(letter, LocalLognPalette.current.isLight)

    @Composable
    @ReadOnlyComposable
    fun edge(letter: Char): Color = edge(letter, LocalLognPalette.current.isLight)

    private fun pick(
        colors: List<Color>,
        letter: Char,
    ): Color = colors[(letter.uppercaseChar() - 'A').coerceIn(0, colors.lastIndex)]
}
