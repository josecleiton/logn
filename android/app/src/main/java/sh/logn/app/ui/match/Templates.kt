package sh.logn.app.ui.match

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGesturesAfterLongPress
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.boundsInRoot
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.IntOffset
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.MatchMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.core.LogN.WatchVariable
import sh.logn.coreshell.i18n.Str
import kotlin.math.roundToInt
import sh.logn.app.ui.theme.Stroke as LognStroke

/**
 * O arraste de um chip do banco até uma casa. Um por partida: as casas se registram com
 * a própria área, o chip arrastado desenha por cima de tudo, e soltar dentro de uma casa
 * a preenche.
 *
 * Arrastar é o gesto do DS, mas não o único: tocar num chip o põe na próxima casa vazia.
 * É o caminho do TalkBack, que não arrasta, e o de quem acha arrastar difícil.
 */
class ChipDragState {
    var dragging by mutableStateOf<String?>(null)
        private set
    var pointer by mutableStateOf(Offset.Zero)
        private set
    var hovered by mutableStateOf<Any?>(null)
        private set
    private val targets = mutableStateMapOf<Any, Pair<Rect, (String) -> Unit>>()

    fun register(
        key: Any,
        bounds: Rect,
        onDrop: (String) -> Unit,
    ) {
        targets[key] = bounds to onDrop
    }

    fun unregister(key: Any) {
        targets.remove(key)
    }

    fun start(
        text: String,
        at: Offset,
    ) {
        dragging = text
        pointer = at
    }

    fun move(to: Offset) {
        pointer = to
        hovered = targets.entries.firstOrNull { it.value.first.contains(to) }?.key
    }

    fun end() {
        val text = dragging
        val target = hovered?.let { targets[it] }
        dragging = null
        hovered = null
        if (text != null && target != null) target.second(text)
    }

    fun cancel() {
        dragging = null
        hovered = null
    }
}

val LocalChipDrag = staticCompositionLocalOf { ChipDragState() }

/** O chip que segue o dedo, desenhado por quem hospeda a partida, por cima de tudo. */
@Composable
fun DragGhost(state: ChipDragState) {
    val text = state.dragging ?: return
    Box(Modifier.offset { IntOffset(state.pointer.x.roundToInt(), state.pointer.y.roundToInt()) }) {
        ChipFace(text, Modifier.offset(x = -MatchMetrics.ghostOffset, y = -MatchMetrics.ghostOffset), raised = true)
    }
}

@Composable
private fun ChipFace(
    text: String,
    modifier: Modifier = Modifier,
    raised: Boolean = false,
) {
    val shape = RoundedCornerShape(Radius.xs)
    Box(
        modifier
            .heightIn(min = Space.matchTouch)
            .background(LognDark.surfaceRaised, shape)
            .border(LognStroke.hairline, if (raised) LognDark.accent else LognDark.lineStrong, shape)
            .padding(horizontal = Space.md, vertical = MatchMetrics.chipPaddingV),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, style = LognFont.mono(MatchMetrics.CHIP_SIZE), color = LognDark.textPrimary)
    }
}

/** Um lugar onde o chip pode cair, para o leitor de tela. */
class ChipTarget(
    val name: String,
    val place: (String) -> Unit,
)

/**
 * O banco de opções: chips da largura do texto, quebrando linha.
 *
 * O toque põe o chip na próxima casa vazia. Para o TalkBack, cada casa também vira uma
 * ação nomeada no chip ("Colocar em TEMPO"), que não aparece na tela: com duas casas, o
 * toque sozinho não deixa escolher qual.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ChipBank(
    options: List<String>,
    targets: List<ChipTarget>,
    onTap: (String) -> Unit,
) {
    val drag = LocalChipDrag.current
    FlowRow(horizontalArrangement = Arrangement.spacedBy(Space.sm), verticalArrangement = Arrangement.spacedBy(Space.sm)) {
        for (option in options) {
            var origin by remember { mutableStateOf(Offset.Zero) }
            ChipFace(
                option,
                Modifier
                    .onGloballyPositioned { origin = it.boundsInRoot().topLeft }
                    .semantics {
                        customActions =
                            targets.map { target ->
                                CustomAccessibilityAction(target.name) {
                                    target.place(option)
                                    true
                                }
                            }
                    }.clickable(role = Role.Button) { onTap(option) }
                    .pointerInput(option) {
                        detectDragGesturesAfterLongPress(
                            onDragStart = { drag.start(option, origin + it) },
                            onDragEnd = { drag.end() },
                            onDragCancel = { drag.cancel() },
                        ) { change, amount ->
                            change.consume()
                            drag.move(drag.pointer + amount)
                        }
                    },
            )
        }
    }
}

/**
 * A casa do arraste: altura 46, raio 2. Vazia, tracejado `lineDim`; com o chip por cima,
 * tracejado acento sobre `accentTint`; cheia, borda sólida. Tocar na cheia a esvazia.
 */
@Composable
fun DropZone(
    value: String,
    onDrop: (String) -> Unit,
    onRemove: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val drag = LocalChipDrag.current
    val key = remember { Any() }
    val filled = value.isNotEmpty()
    val targeted = drag.hovered === key
    val fill =
        when {
            targeted -> LognDark.accentTint
            filled -> LognDark.surfaceRaised
            else -> LognDark.canvas
        }
    val stroke =
        when {
            targeted -> LognDark.accent
            filled -> LognDark.lineStrong
            else -> LognDark.lineDim
        }
    val label = if (filled) Str.Arena.filled_accessibility(context, value) else Str.Arena.drop_here_accessibility(context)
    Box(
        modifier
            .fillMaxWidth()
            .height(MatchMetrics.dropHeight)
            .onGloballyPositioned { drag.register(key, it.boundsInRoot(), onDrop) }
            .clearAndSetSemantics { contentDescription = label }
            .clickable(enabled = filled, role = Role.Button, onClick = onRemove)
            .drawBehind {
                val r = CornerRadius(Radius.xs.toPx())
                drawRoundRect(fill, cornerRadius = r)
                val dash = MatchMetrics.dropDash.toPx()
                drawRoundRect(
                    stroke,
                    cornerRadius = r,
                    style = Stroke(LognStroke.hairline.toPx(), pathEffect = if (filled) null else PathEffect.dashPathEffect(floatArrayOf(dash, dash))),
                )
            },
        contentAlignment = Alignment.Center,
    ) {
        Text(
            if (filled) value else Str.Arena.drop_here(context),
            style = LognFont.mono(if (filled) MatchMetrics.DROP_FILLED_SIZE else MatchMetrics.CHIP_SIZE),
            color = if (filled) LognDark.textPrimary else LognDark.textDim,
        )
    }
    DisposableEffect(key) { onDispose { drag.unregister(key) } }
}

/** A casa com rótulo à esquerda (TEMPO, ESPAÇO) do COMPLEXITY_MATCH. */
@Composable
fun LabelledDrop(
    label: String,
    value: String,
    onDrop: (String) -> Unit,
    onRemove: () -> Unit,
) {
    Row(horizontalArrangement = Arrangement.spacedBy(Space.md), verticalAlignment = Alignment.CenterVertically) {
        Text(
            label,
            style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
            color = LognDark.textMuted,
            modifier = Modifier.width(MatchMetrics.axisLabel),
        )
        DropZone(value, onDrop, onRemove, Modifier.weight(1f))
    }
}

/** Tag de seleção múltipla: Plex Mono 13, piso de 56 (um toque errado custa vida). */
@Composable
fun TagChip(
    text: String,
    selected: Boolean,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(Radius.xs)
    Box(
        Modifier
            .heightIn(min = Space.matchTouch)
            .background(if (selected) LognDark.accentTint else Color.Transparent, shape)
            .border(LognStroke.hairline, if (selected) LognDark.accent else LognDark.lineStrong, shape)
            .semantics { this.selected = selected }
            .clickable(role = Role.Checkbox, onClick = onClick)
            .padding(horizontal = MatchMetrics.tagPaddingH, vertical = MatchMetrics.tagPaddingV),
        contentAlignment = Alignment.Center,
    ) {
        Text(text, style = LognFont.mono(MatchMetrics.CHIP_SIZE), color = if (selected) LognDark.accentInk else LognDark.textSecondary)
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun TagBank(
    options: List<String>,
    selected: List<String>,
    onToggle: (String) -> Unit,
) {
    FlowRow(horizontalArrangement = Arrangement.spacedBy(Space.sm), verticalArrangement = Arrangement.spacedBy(Space.sm)) {
        for (option in options) TagChip(option, option in selected) { onToggle(option) }
    }
}

/**
 * TRADEOFF_MATCH: duas casas no topo e as opções como linhas inteiras, porque são frases.
 * Toque, não arraste: a opção vai para a próxima casa vazia, e tocar na casa a esvazia.
 */
@Composable
fun TradeoffPanel(
    codeLines: List<String>,
    options: List<String>,
    benefit: String,
    drawback: String,
    onPick: (String) -> Unit,
    onClearBenefit: () -> Unit,
    onClearDrawback: () -> Unit,
) {
    val context = LocalContext.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.md)) {
        if (codeLines.isNotEmpty()) CodeBlock(codeLines)
        TradeoffSlot(Str.Arena.benefit_slot(context), benefit, isNext = benefit.isEmpty(), onClear = onClearBenefit)
        TradeoffSlot(Str.Arena.drawback_slot(context), drawback, isNext = benefit.isNotEmpty() && drawback.isEmpty(), onClear = onClearDrawback)
        Box(
            Modifier
                .fillMaxWidth()
                .padding(vertical = Space.xs)
                .height(LognStroke.hairline)
                .background(LognDark.line),
        )
        for ((index, option) in options.withIndex()) {
            val used = option == benefit || option == drawback
            OptionRow(('A' + index % ALPHABET).toString(), option, used) { onPick(option) }
        }
    }
}

@Composable
private fun TradeoffSlot(
    label: String,
    value: String,
    isNext: Boolean,
    onClear: () -> Unit,
) {
    val context = LocalContext.current
    val filled = value.isNotEmpty()
    val stroke =
        when {
            filled -> LognDark.lineStrong
            isNext -> LognDark.accent
            else -> LognDark.lineDim
        }
    val a11y = if (filled) Str.Arena.slot_filled_accessibility(context, label, value) else Str.Arena.slot_empty_accessibility(context, label)
    Column(
        Modifier
            .fillMaxWidth()
            .heightIn(min = Space.matchTouch)
            .clearAndSetSemantics { contentDescription = a11y }
            .clickable(enabled = filled, role = Role.Button, onClick = onClear)
            .drawBehind {
                val r = CornerRadius(Radius.sm.toPx())
                if (filled) drawRoundRect(LognDark.surfaceRaised, cornerRadius = r)
                val dash = MatchMetrics.dropDash.toPx()
                drawRoundRect(
                    stroke,
                    cornerRadius = r,
                    style =
                        Stroke(
                            LognStroke.hairline.toPx(),
                            pathEffect = if (filled) null else PathEffect.dashPathEffect(floatArrayOf(dash, dash * DASH_GAP_RATIO)),
                        ),
                )
            }.padding(horizontal = MatchMetrics.slotPaddingH, vertical = Space.sm),
        verticalArrangement = Arrangement.spacedBy(Space.xxs),
    ) {
        Text(
            label,
            style = LognFont.mono(MatchMetrics.LABEL_SIZE, FontWeight.Medium, MatchMetrics.SLOT_TRACKING),
            color = if (isNext) LognDark.accentInk else LognDark.textSecondary,
        )
        Text(
            if (filled) value else Str.Arena.tap_an_option(context),
            style = LognFont.sans(MatchMetrics.OPTION_SIZE),
            color = if (filled) LognDark.textPrimary else LognDark.textMuted,
        )
    }
}

@Composable
private fun OptionRow(
    letter: String,
    text: String,
    used: Boolean,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = Space.matchTouch)
            .background(if (used) Color.Transparent else LognDark.surfaceRaised, shape)
            .border(LognStroke.hairline, if (used) LognDark.line else LognDark.lineStrong, shape)
            .clickable(enabled = !used, role = Role.Button, onClick = onClick)
            .padding(horizontal = MatchMetrics.slotPaddingH, vertical = Space.md),
        horizontalArrangement = Arrangement.spacedBy(Space.sm),
    ) {
        Text(letter, style = LognFont.mono(MatchMetrics.LETTER_SIZE), color = LognDark.textMuted, modifier = Modifier.width(MatchMetrics.optionLetter))
        Text(text, style = LognFont.sans(MatchMetrics.OPTION_SIZE), color = if (used) LognDark.textDim else LognDark.textPrimary)
    }
}

/**
 * DRY_RUN (2b · Watch): o código não tem bug, o desafio é o trace. O painel de watch
 * mostra as variáveis no estado inicial; a saída prevista usa o acento (verde é do juiz).
 */
@Composable
fun DryRunPanel(
    codeLines: List<String>,
    watch: List<WatchVariable>,
    watchNote: String,
    predicted: String,
    onChange: (String) -> Unit,
) {
    val context = LocalContext.current
    Column(verticalArrangement = Arrangement.spacedBy(MatchMetrics.dryGap)) {
        CodeBlock(codeLines, fontSize = MatchMetrics.DRY_CODE_SIZE, lineHeight = MatchMetrics.dryLine)
        if (watch.isNotEmpty()) WatchPanel(watch, watchNote)
        var focused by remember { mutableStateOf(false) }
        val shape = RoundedCornerShape(Radius.xs)
        val outputLabel = Str.Dry_run.output_accessibility(context)
        Column(
            Modifier
                .fillMaxWidth()
                .background(LognDark.canvas, shape)
                .border(LognStroke.hairline, if (focused || predicted.isNotEmpty()) LognDark.accent else LognDark.line, shape)
                .padding(horizontal = MatchMetrics.slotPaddingH, vertical = MatchMetrics.outputPaddingV),
            verticalArrangement = Arrangement.spacedBy(Space.sm),
        ) {
            Row(Modifier.fillMaxWidth()) {
                Text(
                    Str.Dry_run.expected_output(context),
                    style = LognFont.mono(MatchMetrics.WATCH_LABEL_SIZE, tracking = MatchMetrics.WATCH_TRACKING),
                    color = LognDark.textMuted,
                    modifier = Modifier.weight(1f),
                )
                // A limpeza é visível: o jogador sabe que espaço não conta.
                if (predicted.isNotEmpty()) {
                    Text(Str.Dry_run.ignored_spaces(context), style = LognFont.mono(MatchMetrics.WATCH_LABEL_SIZE), color = LognDark.infoInk)
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Space.sm), verticalAlignment = Alignment.CenterVertically) {
                Text(PROMPT, style = LognFont.mono(MatchMetrics.OUTPUT_SIZE), color = LognDark.accentInk)
                val size = if (predicted.isEmpty()) MatchMetrics.OUTPUT_SIZE else MatchMetrics.OUTPUT_FILLED_SIZE
                BasicTextField(
                    value = predicted,
                    onValueChange = onChange,
                    singleLine = true,
                    textStyle = LognFont.mono(size).copy(color = LognDark.textPrimary),
                    cursorBrush = SolidColor(LognDark.accent),
                    keyboardOptions =
                        KeyboardOptions(
                            capitalization = KeyboardCapitalization.None,
                            autoCorrectEnabled = false,
                            keyboardType = KeyboardType.Ascii,
                        ),
                    modifier =
                        Modifier
                            .weight(1f)
                            .semantics { contentDescription = outputLabel }
                            .onFocusChanged { focused = it.isFocused },
                    decorationBox = { inner ->
                        Box {
                            if (predicted.isEmpty()) Text(Str.Dry_run.tap_to_type(context), style = LognFont.mono(size), color = LognDark.textDim)
                            inner()
                        }
                    },
                )
            }
        }
    }
}

@Composable
private fun WatchPanel(
    watch: List<WatchVariable>,
    note: String,
) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(Radius.xs)
    Column(
        Modifier
            .fillMaxWidth()
            .background(LognDark.surfaceRaised, shape)
            .border(LognStroke.hairline, LognDark.line, shape),
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .drawBehind {
                    val y = size.height - LognStroke.hairline.toPx() / 2
                    drawLine(LognDark.line, Offset(0f, y), Offset(size.width, y), LognStroke.hairline.toPx())
                }.padding(horizontal = Space.md, vertical = MatchMetrics.watchHeaderV),
        ) {
            Text(
                Str.Dry_run.watch(context),
                style = LognFont.mono(MatchMetrics.WATCH_LABEL_SIZE, tracking = MatchMetrics.WATCH_TRACKING),
                color = LognDark.textMuted,
                modifier = Modifier.weight(1f),
            )
            if (note.isNotEmpty()) Text(note, style = LognFont.mono(MatchMetrics.WATCH_LABEL_SIZE), color = LognDark.textMuted)
        }
        Row(Modifier.height(IntrinsicSize.Min)) {
            for ((index, variable) in watch.withIndex()) {
                val label = Str.Dry_run.watch_accessibility(context, variable.name, variable.value)
                Column(
                    Modifier
                        .weight(1f)
                        .clearAndSetSemantics { contentDescription = label }
                        .padding(horizontal = Space.md, vertical = Space.md),
                    verticalArrangement = Arrangement.spacedBy(Space.xxs),
                ) {
                    Text(variable.name, style = LognFont.mono(MatchMetrics.LABEL_SIZE), color = LognDark.textMuted)
                    Text(variable.value, style = LognFont.mono(MatchMetrics.WATCH_VALUE_SIZE), color = LognDark.textPrimary)
                }
                if (index < watch.lastIndex) {
                    Box(
                        Modifier
                            .width(LognStroke.hairline)
                            .fillMaxHeight()
                            .background(LognDark.line),
                    )
                }
            }
        }
    }
}

private const val PROMPT = ">"
private const val ALPHABET = 26
private const val DASH_GAP_RATIO = 0.75f
