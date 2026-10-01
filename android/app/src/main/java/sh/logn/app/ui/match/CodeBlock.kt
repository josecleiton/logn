package sh.logn.app.ui.match

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.MatchMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.coreshell.i18n.Str
import sh.logn.app.ui.theme.Stroke as LognStroke

/**
 * Bloco de código com número de linha, realce e seleção de linha. Espelho de
 * CodeBlock.swift: sem rolagem horizontal, as linhas longas quebram.
 *
 * Tocável (SPOT_THE_BUG): Plex Mono 12.5 em linha de 33, divisor entre linhas, raio 4.
 * Só de leitura: 13 em 26, raio 2, sem divisor.
 */
@Composable
fun CodeBlock(
    lines: List<String>,
    modifier: Modifier = Modifier,
    selectedLine: Int? = null,
    highlightColor: Color = LognDark.accent,
    highlightInk: Color = LognDark.accentInk,
    fontSize: Float? = null,
    lineHeight: Dp? = null,
    fillsHeight: Boolean = false,
    onSelectLine: ((Int) -> Unit)? = null,
) {
    val selectable = onSelectLine != null
    val size = fontSize ?: if (selectable) MatchMetrics.CODE_SELECTABLE_SIZE else MatchMetrics.CODE_SIZE
    val height = lineHeight ?: if (selectable) MatchMetrics.codeSelectableLine else MatchMetrics.codeLine
    val shape = RoundedCornerShape(if (selectable) Radius.sm else Radius.xs)
    Column(
        modifier
            .fillMaxWidth()
            .then(if (fillsHeight) Modifier.fillMaxHeight() else Modifier)
            .background(LognDark.surface, shape)
            .border(LognStroke.hairline, LognDark.line, shape)
            .padding(vertical = if (selectable) Space.none else Space.sm),
    ) {
        for ((index, line) in lines.withIndex()) {
            CodeLine(
                number = index + 1,
                code = line,
                selected = index == selectedLine,
                showsDivider = selectable && index < lines.lastIndex,
                highlightColor = highlightColor,
                highlightInk = highlightInk,
                fontSize = size,
                lineHeight = height,
                onTap = onSelectLine?.let { { it(index) } },
            )
        }
        if (fillsHeight) Spacer(Modifier.weight(1f))
    }
}

@Composable
private fun CodeLine(
    number: Int,
    code: String,
    selected: Boolean,
    showsDivider: Boolean,
    highlightColor: Color,
    highlightInk: Color,
    fontSize: Float,
    lineHeight: Dp,
    onTap: (() -> Unit)?,
) {
    val context = LocalContext.current
    val blank = code.indexOf(BLANK)
    Row(
        Modifier
            .fillMaxWidth()
            .height(IntrinsicSize.Min)
            .heightIn(min = lineHeight)
            .background(if (selected) highlightColor.copy(alpha = MatchMetrics.SELECTED_ALPHA) else Color.Transparent)
            .semantics { this.selected = selected }
            .then(if (onTap != null) Modifier.clickable(role = Role.Button, onClick = onTap) else Modifier)
            .drawBehind {
                // Barra lateral de 2: o estado nunca depende só do fundo.
                if (selected) drawRect(highlightColor, size = size.copy(width = MatchMetrics.sideBar.toPx()))
                if (showsDivider) {
                    val y = size.height - LognStroke.hairline.toPx()
                    drawRect(LognDark.rowLine, topLeft = androidx.compose.ui.geometry.Offset(0f, y), size = size.copy(height = LognStroke.hairline.toPx()))
                }
            }.padding(horizontal = Space.md),
        horizontalArrangement = Arrangement.spacedBy(Space.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            "$number",
            style = LognFont.mono(fontSize),
            color = if (selected) highlightInk else LognDark.textMuted,
            maxLines = 1,
            textAlign = TextAlign.End,
            modifier = Modifier.width(MatchMetrics.lineNumber),
        )
        if (blank >= 0) {
            // A lacuna do FILL_IN_THE_BLANK é uma caixa tracejada, não texto sublinhado.
            Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically) {
                Text(highlight(code.substring(0, blank)), style = LognFont.mono(fontSize))
                val gap = Str.Arena.gap_to_fill(context)
                Box(
                    Modifier
                        .padding(horizontal = Space.xxs)
                        .width(MatchMetrics.blankWidth)
                        .height(MatchMetrics.blankHeight)
                        .clearAndSetSemantics { contentDescription = gap }
                        .drawBehind {
                            val dash = MatchMetrics.blankDash.toPx()
                            drawRoundRect(
                                LognDark.accent,
                                cornerRadius = CornerRadius(Radius.xs.toPx()),
                                style = Stroke(LognStroke.hairline.toPx(), pathEffect = PathEffect.dashPathEffect(floatArrayOf(dash, dash))),
                            )
                        },
                )
                Text(highlight(code.substring(blank + BLANK.length)), style = LognFont.mono(fontSize))
            }
        } else {
            Text(highlight(code, emphasised = selected), style = LognFont.mono(fontSize), modifier = Modifier.weight(1f))
        }
    }
}

private const val BLANK = "_____"

/** C/C++, e o Kotlin das telas do documento de gameplay. */
private val keywords =
    (
        "int void return if else for while do break continue bool true false string char long double float auto const " +
            "struct class public private static vector map set pair queue stack using namespace std include define " +
            "fun val var in is when null"
    ).split(' ').toSet()

/**
 * Realce básico do pseudocódigo (sabor C/C++/Kotlin), como o `SyntaxHighlighter` de iOS:
 * palavra-chave em `synKeyword`, chamada em `synFunction`, número em `warnInk`.
 */
fun highlight(
    code: String,
    emphasised: Boolean = false,
): AnnotatedString =
    buildAnnotatedString {
        val base = if (emphasised) LognDark.textPrimary else LognDark.textSecondary
        var i = 0

        fun isWord(c: Char) = c.isLetterOrDigit() || c == '_' || c == '#'
        while (i < code.length) {
            if (code.startsWith(BLANK, i)) {
                withStyle(SpanStyle(color = LognDark.accent)) { append(BLANK) }
                i += BLANK.length
                continue
            }
            if (isWord(code[i])) {
                val start = i
                while (i < code.length && isWord(code[i])) i++
                val word = code.substring(start, i)
                val color =
                    when {
                        word in keywords -> LognDark.synKeyword
                        word.all(Char::isDigit) -> LognDark.warnInk
                        i < code.length && code[i] == '(' -> LognDark.synFunction
                        else -> base
                    }
                withStyle(SpanStyle(color = color)) { append(word) }
                continue
            }
            val start = i
            while (i < code.length && !isWord(code[i])) i++
            if (i == start) i++
            withStyle(SpanStyle(color = base)) { append(code.substring(start, i)) }
        }
    }
