package sh.logn.app.ui.match

import android.view.HapticFeedbackConstants
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.delay
import sh.logn.app.core.FeatureFlags
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.LocalReadView
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.theme.LocalReduceMotion
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.MatchMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.app.ui.tree.XP_PER_ACCEPTED
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.MatchViewModel
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * A partida (3b · Contest). Espelho de MatchView.swift: o placar sempre à vista, os seis
 * formatos, e o veredito que toma a tela inteira.
 *
 * O Core manda na partida; o shell só congela o veredito no instante do submit (o Core já
 * avançou quando a tela aparece), bate o relógio a cada segundo e volta à trilha quando o
 * Core diz que a saída foi do jogador. `reportBottom` é o pé do relatório: o "Entendi", ou
 * a oferta do fim da amostra.
 */
@Composable
fun MatchScreen(
    view: ViewModel,
    nodeId: String,
    onExit: () -> Unit,
    reportBottom: @Composable (onDismiss: () -> Unit) -> Unit,
) {
    val dispatch = LocalDispatch.current
    val readView = LocalReadView.current
    val hostView = LocalView.current
    val reduceMotion = LocalReduceMotion.current
    val mv = view.matchView
    var verdict by rememberSaveable(stateSaver = verdictSaver) { mutableStateOf<VerdictSnapshot?>(null) }
    val drag = remember { ChipDragState() }
    val currentLetter = mv.currentLetter.firstOrNull() ?: 'A'
    val states = mv.balloonStates.mapNotNull { s -> s.letter.firstOrNull()?.let { it to s.isAccepted } }
    // A história da origem só abre com a flag; o selo é atribuição e aparece sempre.
    FeatureFlags.version
    val originStory = FeatureFlags.isEnabled(ORIGIN_FLAG)

    // A letra em jogo. No TLE o Core já avançou para o próximo problema quando a armadilha
    // chega, e o veredito tem de ser do problema que estourou, não do seguinte.
    var playingLetter by rememberSaveable { mutableStateOf(currentLetter) }
    SideEffect { if (!mv.hasTrap && verdict == null) playingLetter = currentLetter }

    // A Activity recriada (tema, fonte) volta a esta tela com a partida viva no Core:
    // começar de novo zerava vidas e relógio. Depois da morte do processo o Core está
    // vazio, e aí começa.
    var started by rememberSaveable { mutableStateOf(false) }
    LaunchedEffect(nodeId) {
        if (!started || !readView().matchView.isActive) dispatch(Event.StartMatch(nodeId))
        started = true
    }
    // Quem decide que a partida acabou é o Core; desempilhar é do shell. Lido na hora: o
    // `matchLeft` desta composição ainda é o da partida anterior, que o `StartMatch` acima
    // acabou de limpar.
    LaunchedEffect(view.matchLeft) { if (readView().matchLeft) onExit() }
    // O TLE não vem de um toque: o relógio zera e o Core submete sozinho.
    LaunchedEffect(mv.hasTrap) {
        if (mv.hasTrap && verdict == null) verdict = VerdictSnapshot(playingLetter, VerdictCode.of(mv.lastVerdict), mv.lives)
    }
    // O relógio da questão, que para no veredito, na armadilha e com o app fora da frente:
    // o Core não tem relógio, e o iOS suspende o Timer em segundo plano. Sem isto, trocar de
    // app no meio da questão custava vidas.
    val verdictOpen by rememberUpdatedState(verdict != null)
    val lifecycle = LocalLifecycleOwner.current
    LaunchedEffect(lifecycle) {
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            while (true) {
                delay(TICK_MILLIS)
                val now = readView().matchView
                if (now.isActive && !now.hasTrap && !verdictOpen) dispatch(Event.MatchTimerTick)
            }
        }
    }

    fun submit() {
        val letter = currentLetter
        dispatch(Event.MatchSubmit(System.currentTimeMillis() / MILLIS))
        // Lido na hora: o `mv` desta recomposição ainda é o de antes do submit.
        val result = readView().matchView
        val code = VerdictCode.of(result.lastVerdict)
        val haptic =
            when {
                android.os.Build.VERSION.SDK_INT < android.os.Build.VERSION_CODES.R -> HapticFeedbackConstants.KEYBOARD_TAP
                code == VerdictCode.AC -> HapticFeedbackConstants.CONFIRM
                else -> HapticFeedbackConstants.REJECT
            }
        hostView.performHapticFeedback(haptic)
        verdict = VerdictSnapshot(letter, code, result.lives)
    }

    fun dismissVerdict() {
        if (readView().matchView.hasTrap) dispatch(Event.MatchDismissTrap)
        verdict = null
    }

    BackHandler {
        when {
            verdict != null -> dismissVerdict()
            mv.isActive -> dispatch(Event.LeaveMatch)
            else -> {
                dispatch(Event.MatchReportClosed)
                onExit()
            }
        }
    }

    CompositionLocalProvider(LocalChipDrag provides drag) {
        Box(
            Modifier
                .fillMaxSize()
                .background(LognDark.canvas),
        ) {
            Box(
                Modifier
                    .fillMaxSize()
                    .statusBarsPadding(),
            ) {
                val snapshot = verdict
                when {
                    snapshot != null -> {
                        // Aceito de desafio que já pagou não paga de novo.
                        val paid = mv.balloonStates.firstOrNull { it.letter == snapshot.letter.toString() }?.alreadyPaid == true
                        MatchVerdictScreen(snapshot, reduceMotion, mv, states, if (paid) 0 else XP_PER_ACCEPTED, ::dismissVerdict)
                    }
                    // Relatório é para quem jogou até o fim; quem saiu pelo X está voltando.
                    !mv.isActive && !view.matchLeft ->
                        MatchReport(mv, states) {
                            reportBottom {
                                dispatch(Event.MatchReportClosed)
                                onExit()
                            }
                        }
                    else -> PlayScreen(mv, currentLetter, states, originStory, ::submit)
                }
            }
            DragGhost(drag)
        }
    }

    mv.originSheet?.let { card -> OriginSheet(card, mv.originSheetPaused) { dispatch(Event.CloseOriginSheet) } }
    if (mv.leavePending) {
        LeaveMatchSheet(
            solved = mv.solvedSoFar,
            total = mv.totalProblems,
            balloonStates = states,
            onStay = { dispatch(Event.CancelLeaveMatch) },
            onLeave = { dispatch(Event.ConfirmLeaveMatch) },
        )
    }
}

@Composable
private fun PlayScreen(
    mv: MatchViewModel,
    letter: Char,
    states: BalloonStates,
    originStory: Boolean,
    onSubmit: () -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    Column(
        Modifier
            .fillMaxSize()
            .imePadding(),
    ) {
        MatchHeader(letter, mv.questionSeconds, mv.isFrozen, mv.lives, mv.maxLives, states, onLeave = { dispatch(Event.LeaveMatch) })
        // SPOT_THE_BUG estica o bloco de código até o botão, e rola quando não cabe.
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState()),
        ) {
            ProblemStatement(mv, originStory)
            Box(Modifier.padding(start = MatchMetrics.bodySide, end = MatchMetrics.bodySide, top = Space.lg, bottom = Space.xl)) {
                TemplateBody(mv)
            }
        }
        // O botão nomeia a escolha, para que um toque errado seja reversível.
        LognButton(
            submitTitle(context, mv),
            ButtonVariant.Primary,
            Modifier
                .background(LognDark.canvas)
                .navigationBarsPadding()
                .padding(start = MatchMetrics.bodySide, end = MatchMetrics.bodySide, top = Space.lg, bottom = MatchMetrics.panelBottom),
            enabled = canSubmit(mv),
            onClick = onSubmit,
        )
    }
}

@Composable
private fun ProblemStatement(
    mv: MatchViewModel,
    originStory: Boolean,
) {
    val dispatch = LocalDispatch.current
    Column(
        Modifier
            .fillMaxWidth()
            .padding(start = MatchMetrics.headerSide, end = MatchMetrics.headerSide, top = MatchMetrics.statementTop),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(Space.sm), verticalAlignment = Alignment.CenterVertically) {
            Text(
                mv.currentTitle.uppercase(),
                style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
                color = LognDark.textMuted,
            )
            // A origem na linha do título: atribuição que ninguém vê não é atribuição.
            if (mv.currentOrigin.isNotEmpty()) {
                val shape = RoundedCornerShape(Radius.sm)
                Text(
                    mv.currentOrigin.uppercase(),
                    style = LognFont.mono(MatchMetrics.ORIGIN_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
                    color = if (originStory) LognDark.accent else LognDark.textSecondary,
                    modifier =
                        Modifier
                            .border(Stroke.hairline, if (originStory) LognDark.accent.copy(alpha = ORIGIN_ALPHA) else LognDark.lineStrong, shape)
                            .then(if (originStory) Modifier.clickable(role = Role.Button) { dispatch(Event.OpenOriginSheet) } else Modifier)
                            .padding(horizontal = MatchMetrics.originPaddingH, vertical = Space.xxs),
                )
            }
        }
        Text(
            mv.currentDescription,
            style =
                LognFont
                    .sans(MatchMetrics.STATEMENT_SIZE, FontWeight.SemiBold)
                    .merge(TextStyle(lineHeight = MatchMetrics.STATEMENT_LINE.sp)),
            color = LognDark.textPrimary,
            modifier = Modifier.padding(top = Space.md),
        )
    }
}

@Composable
private fun TemplateBody(mv: MatchViewModel) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    when (mv.currentTemplateType) {
        SPOT_THE_BUG ->
            CodeBlock(
                mv.currentCodeLines,
                selectedLine = mv.selectedLine.takeIf { it >= 0 },
                onSelectLine = { dispatch(Event.MatchSelectLine(it)) },
            )
        DRY_RUN ->
            DryRunPanel(mv.currentCodeLines, mv.watchVariables, mv.watchNote, mv.predictedOutput) { dispatch(Event.MatchSetOutput(it)) }
        FILL_IN_THE_BLANK ->
            Column(verticalArrangement = Arrangement.spacedBy(Space.lg)) {
                CodeBlock(mv.currentCodeLines)
                Text(
                    Str.Arena.drag_block(context),
                    style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
                    color = LognDark.textMuted,
                )
                DropZone(mv.answerString, { dispatch(Event.MatchSetAnswer(it)) }, { dispatch(Event.MatchSetAnswer("")) })
                ChipBank(mv.currentOptions.filter { it != mv.answerString }) { dispatch(Event.MatchSetAnswer(it)) }
            }
        COMPLEXITY_MATCH ->
            Column(verticalArrangement = Arrangement.spacedBy(Space.md)) {
                // Mostrar vale mais que descrever: o tipo da variável é onde a complexidade se decide.
                if (mv.currentCodeLines.isNotEmpty()) CodeBlock(mv.currentCodeLines)
                LabelledDrop(Str.Arena.time_axis(context), mv.dropTime, { dispatch(Event.MatchSetDropTime(it)) }) {
                    dispatch(Event.MatchSetDropTime(""))
                }
                LabelledDrop(Str.Arena.space_axis(context), mv.dropSpace, { dispatch(Event.MatchSetDropSpace(it)) }) {
                    dispatch(Event.MatchSetDropSpace(""))
                }
                Box(
                    Modifier
                        .padding(vertical = Space.sm)
                        .fillMaxWidth()
                        .height(Stroke.hairline)
                        .background(LognDark.line),
                )
                // Tocar põe o chip na próxima casa vazia: primeiro tempo, depois espaço.
                ChipBank(mv.currentOptions.filter { it != mv.dropTime && it != mv.dropSpace }) { option ->
                    dispatch(if (mv.dropTime.isEmpty()) Event.MatchSetDropTime(option) else Event.MatchSetDropSpace(option))
                }
            }
        TRADEOFF_MATCH ->
            TradeoffPanel(
                mv.currentCodeLines,
                mv.currentOptions,
                mv.tradeoffBenefit,
                mv.tradeoffDrawback,
                onPick = { dispatch(Event.MatchPickTradeoff(it)) },
                onClearBenefit = { dispatch(Event.MatchClearBenefit) },
                onClearDrawback = { dispatch(Event.MatchClearDrawback) },
            )
        TAG_THE_PATTERN ->
            Column(verticalArrangement = Arrangement.spacedBy(MatchMetrics.xpTop)) {
                if (mv.currentCodeLines.isNotEmpty()) CodeBlock(mv.currentCodeLines)
                Text(
                    Str.Arena.select_up_to(context, mv.maxSelections),
                    style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
                    color = LognDark.textMuted,
                )
                TagBank(mv.currentOptions, mv.selectedTags) { dispatch(Event.MatchToggleTag(it)) }
            }
    }
}

private fun canSubmit(mv: MatchViewModel): Boolean =
    when (mv.currentTemplateType) {
        SPOT_THE_BUG -> mv.selectedLine >= 0
        FILL_IN_THE_BLANK -> mv.answerString.isNotEmpty()
        TAG_THE_PATTERN -> mv.selectedTags.size == mv.maxSelections
        COMPLEXITY_MATCH -> mv.dropTime.isNotEmpty() && mv.dropSpace.isNotEmpty()
        TRADEOFF_MATCH -> mv.tradeoffBenefit.isNotEmpty() && mv.tradeoffDrawback.isNotEmpty()
        DRY_RUN -> mv.predictedOutput.isNotBlank()
        else -> false
    }

/** O botão sempre nomeia a escolha: é ele que torna um toque errado reversível. */
private fun submitTitle(
    context: android.content.Context,
    mv: MatchViewModel,
): String =
    when (mv.currentTemplateType) {
        SPOT_THE_BUG -> if (mv.selectedLine >= 0) Str.Arena.confirm_line(context, mv.selectedLine + 1) else Str.Arena.confirm(context)
        FILL_IN_THE_BLANK -> Str.Arena.confirm_answer(context)
        TAG_THE_PATTERN -> if (mv.selectedTags.isEmpty()) Str.Arena.confirm(context) else Str.Arena.confirm_tags(context, mv.selectedTags.size)
        COMPLEXITY_MATCH -> Str.Arena.confirm_complexity(context)
        TRADEOFF_MATCH -> Str.Arena.confirm_tradeoff(context)
        DRY_RUN -> Str.Arena.confirm_output(context)
        else -> Str.Arena.confirm(context)
    }

/** O veredito sobrevive à recriação como "letra|sigla|vidas". */
private val verdictSaver =
    androidx.compose.runtime.saveable.Saver<VerdictSnapshot?, String>(
        save = { it?.let { v -> "${v.letter}|${v.code.name}|${v.livesLeft}" } ?: "" },
        restore = { saved ->
            val parts = saved.split('|')
            if (parts.size != 3) {
                null
            } else {
                runCatching { VerdictSnapshot(parts[0].first(), VerdictCode.valueOf(parts[1]), parts[2].toInt()) }.getOrNull()
            }
        },
    )

private const val SPOT_THE_BUG = "SPOT_THE_BUG"
private const val DRY_RUN = "DRY_RUN"
private const val FILL_IN_THE_BLANK = "FILL_IN_THE_BLANK"
private const val COMPLEXITY_MATCH = "COMPLEXITY_MATCH"
private const val TRADEOFF_MATCH = "TRADEOFF_MATCH"
private const val TAG_THE_PATTERN = "TAG_THE_PATTERN"
private const val ORIGIN_FLAG = "origin_story_enabled"
private const val ORIGIN_ALPHA = 0.5f
private const val TICK_MILLIS = 1_000L
private const val MILLIS = 1000L
