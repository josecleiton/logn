package sh.logn.app.ui.match

import android.content.Context
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import sh.logn.app.ui.components.BalloonShape
import sh.logn.app.ui.components.BalloonStyle
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.balloonColor
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.MatchMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.MatchError
import sh.logn.core.LogN.MatchViewModel
import sh.logn.core.LogN.TrapKind
import sh.logn.coreshell.i18n.Str
import kotlin.math.PI
import kotlin.math.roundToInt
import kotlin.math.sin

/**
 * A sigla do juiz, sem tradução. O significado fica ao lado, nunca no lugar, e em inglês:
 * é o vocabulário do contest, como a própria sigla.
 */
enum class VerdictCode(
    val raw: String,
    val meaning: String,
) {
    AC("AC", "Accepted"),
    WA("WA", "Wrong Answer"),
    TLE("TLE", "Time Limit Exceeded"),
    MLE("MLE", "Memory Limit"),
    RE("RE", "Runtime Error"),
    CE("CE", "Compile Error"),
    PE("PE", "Presentation Error"),
    JUDGING("…", "Judging"),
    ;

    /** Tinta: a sigla e qualquer glifo. */
    val ink: Color
        get() =
            when (this) {
                AC -> LognDark.correctInk
                WA, TLE, MLE, RE -> LognDark.wrongInk
                CE, PE -> LognDark.warnInk
                JUDGING -> LognDark.textMuted
            }

    companion object {
        fun of(code: String): VerdictCode = entries.firstOrNull { it.raw == code.uppercase() } ?: JUDGING
    }
}

/** O veredito congelado no submit: o Core já avançou quando esta tela aparece. */
data class VerdictSnapshot(
    val letter: Char,
    val code: VerdictCode,
    val livesLeft: Int,
) {
    val isAccepted: Boolean get() = code == VerdictCode.AC
}

/**
 * O texto do cartão de armadilha a partir do que o Core manda: o tipo, o título e a
 * explicação crua. As frases em volta saem do catálogo.
 */
object TrapCopy {
    fun category(
        context: Context,
        kind: TrapKind,
    ): String = if (kind == TrapKind.TIMELIMIT) Str.Match.tle_category(context) else Str.Match.trap_classic(context)

    fun title(
        context: Context,
        kind: TrapKind,
        problemTitle: String,
    ): String = if (kind == TrapKind.TIMELIMIT) Str.Match.tle_title(context) else problemTitle

    fun explanation(
        context: Context,
        kind: TrapKind,
        own: String,
    ): String {
        val text = own.trim()
        return when (kind) {
            TrapKind.TIMELIMIT -> if (text.isEmpty()) Str.Match.tle_context(context) else "${Str.Match.tle_context(context)} $text"
            TrapKind.WRONGANSWER -> text.ifEmpty { Str.Match.generic_wrong_answer(context) }
        }
    }
}

/**
 * O veredito em tela cheia (3b · Contest). A sigla é o herói; a punição aparece em
 * números, não em adjetivos. No erro, o cabeçalho treme (ou pisca, com animação reduzida).
 */
@Composable
fun MatchVerdictScreen(
    snapshot: VerdictSnapshot,
    reduceMotion: Boolean,
    mv: MatchViewModel,
    balloonStates: BalloonStates,
    xpAward: Int,
    onContinue: () -> Unit,
) {
    val context = LocalContext.current
    val shake = remember { Animatable(0f) }
    var flash by remember { mutableStateOf(false) }
    LaunchedEffect(snapshot) {
        if (snapshot.isAccepted) return@LaunchedEffect
        if (reduceMotion) {
            flash = true
            delay(SHAKE_MILLIS.toLong() * 2)
            flash = false
        } else {
            shake.animateTo(1f, tween(SHAKE_MILLIS))
        }
    }
    Column(Modifier.fillMaxSize()) {
        MatchHeader(
            snapshot.letter,
            mv.questionSeconds,
            isFrozen = false,
            lives = mv.lives,
            maxLives = mv.maxLives,
            balloonStates = balloonStates,
            modifier =
                Modifier
                    .offset {
                        val dx = MatchMetrics.shakeAmount.toPx() * sin(shake.value * PI.toFloat() * SHAKES)
                        androidx.compose.ui.unit.IntOffset(dx.roundToInt(), 0)
                    }
                    .drawBehind {
                        if (flash) {
                            val h = MatchMetrics.flash.toPx()
                            drawRect(LognDark.wrong, Offset(0f, size.height - h), size.copy(height = h))
                        }
                    },
            currentIsAlive = snapshot.isAccepted,
            showsBalloonRow = snapshot.isAccepted,
        )
        Box(
            Modifier
                .weight(1f)
                .fillMaxWidth(),
        ) {
            if (snapshot.isAccepted) HitStage(snapshot, xpAward) else MissStage(snapshot)
        }
        ExplanationPanel(context, snapshot, mv, balloonStates, onContinue)
    }
}

@Composable
private fun HitStage(
    snapshot: VerdictSnapshot,
    xpAward: Int,
) {
    val context = LocalContext.current
    val letter = snapshot.letter.toString()
    val label =
        if (xpAward > 0) Str.Match.accepted_accessibility(context, letter, xpAward) else Str.Solved.verdict_accessibility(context, letter)
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.tintOk)
            .clearAndSetSemantics { contentDescription = label },
        verticalArrangement = Arrangement.spacedBy(Space.sm, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        BalloonShape(BalloonStyle.Filled(balloonColor(snapshot.letter)), MatchMetrics.hitBalloon, showString = true)
        Text(
            VerdictCode.AC.raw,
            style = LognFont.mono(MatchMetrics.HIT_SIZE, FontWeight.SemiBold, MatchMetrics.HERO_TRACKING),
            color = LognDark.correctInk,
            modifier = Modifier.padding(top = Space.md),
        )
        Text(
            Str.Match.balloon_up(context, letter),
            style = LognFont.mono(MatchMetrics.STAGE_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
            color = LognDark.correctInk,
        )
        if (xpAward > 0) {
            Text(
                Str.Match.xp_earned(context, xpAward),
                style = LognFont.mono(MatchMetrics.XP_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
                modifier = Modifier.padding(top = MatchMetrics.xpTop),
            )
        } else {
            Text(
                Str.Solved.verdict(context),
                style = LognFont.mono(MatchMetrics.STAGE_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
                color = LognDark.textMuted,
                modifier = Modifier.padding(top = MatchMetrics.xpTop),
            )
        }
    }
}

@Composable
private fun MissStage(snapshot: VerdictSnapshot) {
    val context = LocalContext.current
    val label = Str.Match.failed_accessibility(context, snapshot.code.raw, snapshot.code.meaning)
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.tintErr)
            .clearAndSetSemantics { contentDescription = label }
            .padding(vertical = Space.lg),
        verticalArrangement = Arrangement.spacedBy(Space.sm, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            snapshot.code.raw,
            style = LognFont.mono(MatchMetrics.MISS_SIZE, FontWeight.SemiBold, MatchMetrics.HERO_TRACKING),
            color = LognDark.wrongInk,
        )
        Text(
            snapshot.code.meaning.uppercase(),
            style = LognFont.mono(MatchMetrics.MEANING_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
            color = LognDark.wrongInk,
        )
        Row(Modifier.padding(top = MatchMetrics.xpTop), horizontalArrangement = Arrangement.spacedBy(Space.screenMargin)) {
            val style = LognFont.mono(MatchMetrics.PENALTY_SIZE)
            Text(Str.Match.penalty_minutes(context), style = style, color = LognDark.textPrimary)
            Text(Str.Match.lost_life(context), style = style, color = LognDark.wrongInk)
            Text(Str.Match.zero_xp(context), style = style, color = LognDark.textMuted)
        }
    }
}

@Composable
private fun ExplanationPanel(
    context: Context,
    snapshot: VerdictSnapshot,
    mv: MatchViewModel,
    balloonStates: BalloonStates,
    onContinue: () -> Unit,
) {
    val matchOver = !mv.isActive
    Column(
        Modifier
            .fillMaxWidth()
            .shadow(MatchMetrics.panelShadow)
            .background(LognDark.surfaceRaised)
            .drawBehind { drawLine(LognDark.lineStrong, Offset(0f, 0f), Offset(size.width, 0f), Stroke.hairline.toPx()) }
            .navigationBarsPadding()
            .padding(start = MatchMetrics.headerSide, end = MatchMetrics.headerSide, top = Space.screenMargin, bottom = MatchMetrics.panelBottom),
    ) {
        if (!snapshot.isAccepted) {
            // Explicação longa empurrava o botão para fora: só ela rola.
            Column(
                Modifier
                    .heightIn(max = MatchMetrics.explanationMax)
                    .verticalScroll(rememberScrollState()),
            ) {
                Text(
                    TrapCopy.category(context, mv.trapKind).uppercase(),
                    style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.TRAP_TRACKING),
                    color = LognDark.wrongInk,
                )
                val title = TrapCopy.title(context, mv.trapKind, mv.trapTitle)
                if (title.isNotEmpty()) {
                    Text(
                        title,
                        style = LognFont.sans(MatchMetrics.TRAP_TITLE_SIZE).merge(TextStyle(lineHeight = MatchMetrics.TRAP_TITLE_LINE.sp)),
                        color = LognDark.textPrimary,
                        modifier = Modifier.padding(top = Space.md),
                    )
                }
                val explanation = TrapCopy.explanation(context, mv.trapKind, mv.trapExplanation)
                if (explanation.isNotEmpty()) {
                    Text(
                        explanation,
                        style = LognFont.sans(MatchMetrics.TRAP_BODY_SIZE).merge(TextStyle(lineHeight = MatchMetrics.TRAP_BODY_LINE.sp)),
                        color = LognDark.textSecondary,
                        modifier = Modifier.padding(top = Space.md),
                    )
                }
            }
        }
        // Acabou a partida, não há próximo problema: o botão não promete um.
        LognButton(
            when {
                matchOver -> Str.Match.view_report(context)
                snapshot.isAccepted -> Str.Match.next_problem(context)
                else -> Str.Match.keep_going(context)
            },
            ButtonVariant.Primary,
            Modifier.padding(top = MatchMetrics.xpTop),
            onClick = onContinue,
        )
        if (matchOver) {
            // Quem subiu todos os balões fechou o nó; "em aberto" para quem zerou é mentira.
            val all = balloonStates.isNotEmpty() && balloonStates.all { it.second }
            Text(
                if (all) Str.Arena.back_to_trail(context) else Str.Arena.back_to_trail_open(context),
                style = LognFont.mono(MatchMetrics.LABEL_SIZE),
                color = LognDark.textMuted,
                modifier =
                    Modifier
                        .align(Alignment.CenterHorizontally)
                        .padding(top = Space.md),
            )
        }
    }
}

/**
 * O relatório (tela 4 do DS): `CONTEST ENCERRADO`, os aceitos, a fileira desta partida e a
 * revisão dos erros. `bottom` é o que fica no pé: o "Entendi", ou a oferta da amostra.
 */
@Composable
fun MatchReport(
    mv: MatchViewModel,
    balloonStates: BalloonStates,
    bottom: @Composable () -> Unit,
) {
    val context = LocalContext.current
    Column(Modifier.fillMaxSize()) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(LognDark.canvas)
                .drawBehind {
                    val y = size.height - Stroke.hairline.toPx() / 2
                    drawLine(LognDark.line, Offset(0f, y), Offset(size.width, y), Stroke.hairline.toPx())
                }.padding(start = Space.screenMargin, end = Space.screenMargin, top = MatchMetrics.reportTop, bottom = MatchMetrics.headerBottom),
        ) {
            Text(
                Str.Match.contest_over(context),
                style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.TRAP_TRACKING),
                color = LognDark.textMuted,
            )
            Row(Modifier.padding(top = Space.sm), horizontalArrangement = Arrangement.spacedBy(Space.md), verticalAlignment = Alignment.Bottom) {
                Text(
                    "${mv.solvedCount}",
                    style = LognFont.sans(MatchMetrics.REPORT_COUNT_SIZE, FontWeight.SemiBold, MatchMetrics.HERO_TRACKING),
                    color = LognDark.textPrimary,
                )
                Text(
                    Str.Match.report_stats(context, mv.totalProblems, mv.penaltyMinutes),
                    style = LognFont.mono(MatchMetrics.REPORT_STATS_SIZE),
                    color = LognDark.textMuted,
                    modifier = Modifier.padding(bottom = Space.sm),
                )
            }
            // A fileira grande: o troféu da sessão, só com os problemas desta partida.
            Row(Modifier.padding(top = MatchMetrics.dryGap), horizontalArrangement = Arrangement.spacedBy(MatchMetrics.rowGap)) {
                for ((letter, accepted) in balloonStates) {
                    val label =
                        Str.Match.problem_status(
                            context,
                            letter.toString(),
                            if (accepted) Str.Verdict.accepted(context) else Str.Verdict.untried(context),
                        )
                    Column(
                        Modifier
                            .alpha(if (accepted) 1f else MatchMetrics.UNSOLVED_ALPHA)
                            .clearAndSetSemantics { contentDescription = label },
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(Space.xs),
                    ) {
                        BalloonShape(
                            if (accepted) BalloonStyle.Filled(balloonColor(letter)) else BalloonStyle.Outline(LognDark.lineStrong, REPORT_OUTLINE),
                            MatchMetrics.reportBalloon,
                            showString = true,
                        )
                        Text(letter.toString(), style = LognFont.mono(MatchMetrics.ROW_LETTER_SIZE), color = LognDark.textMuted)
                    }
                }
            }
            // O XP, e por que ele não é aceitos × 50 quando não é: aceito repetido não paga.
            val repeated = mv.balloonStates.count { it.isAccepted && it.alreadyPaid }
            var xp = Str.Solved.report_xp(context, mv.xpEarned)
            if (repeated > 0) xp += " · " + Str.Solved.report_repeated(context, repeated)
            Text(
                xp,
                style = LognFont.mono(MatchMetrics.LABEL_SIZE),
                color = if (mv.xpEarned > 0) LognDark.textSecondary else LognDark.textMuted,
                modifier = Modifier.padding(top = Space.md),
            )
        }
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = Space.screenMargin, vertical = MatchMetrics.headerBottom),
            verticalArrangement = Arrangement.spacedBy(Space.md),
        ) {
            Text(
                if (mv.errors.isEmpty()) Str.Match.review(context) else Str.Match.review_errors(context, mv.errors.size),
                style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
                color = LognDark.textMuted,
            )
            if (mv.errors.isEmpty()) {
                Text(Str.Match.no_errors(context), style = LognFont.bodyMedium, color = LognDark.textSecondary)
            } else {
                for (error in mv.errors) ErrorReviewCard(error)
            }
        }
        Box(
            Modifier
                .background(LognDark.canvas)
                .navigationBarsPadding()
                .padding(start = Space.screenMargin, end = Space.screenMargin, top = Space.md, bottom = Space.xl),
        ) { bottom() }
    }
}

/** Um card por erro: a sigla em mono na tinta do tom, a resposta dada e a explicação. */
@Composable
private fun ErrorReviewCard(error: MatchError) {
    val context = LocalContext.current
    val verdict = VerdictCode.of(error.verdict)
    // A linha do SPOT_THE_BUG vem como número, e "linha N" sai na língua do app.
    val given = if (error.givenLine >= 0) Str.Match.line_number(context, error.givenLine) else error.givenAnswer
    val explanation =
        TrapCopy.explanation(context, if (error.verdict == VerdictCode.TLE.raw) TrapKind.TIMELIMIT else TrapKind.WRONGANSWER, error.explanation)
    val label = Str.Match.error_accessibility(context, error.letter, error.title, verdict.raw, verdict.meaning)
    val shape = RoundedCornerShape(Radius.sm)
    Column(
        Modifier
            .fillMaxWidth()
            .background(LognDark.surface, shape)
            .border(Stroke.hairline, LognDark.line, shape)
            .clearAndSetSemantics { contentDescription = "$label. $explanation" }
            .padding(MatchMetrics.cardPadding),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            Text(
                error.title,
                style = LognFont.sans(MatchMetrics.CARD_TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
                modifier = Modifier.weight(1f),
            )
            Text(verdict.raw, style = LognFont.mono(MatchMetrics.LABEL_SIZE), color = verdict.ink)
        }
        if (given.isNotEmpty()) {
            Text(
                Str.Match.your_answer(context, given),
                style = LognFont.mono(MatchMetrics.CARD_ANSWER_SIZE),
                color = LognDark.textMuted,
                modifier = Modifier.padding(top = Space.sm),
            )
        }
        if (explanation.isNotEmpty()) {
            Text(
                explanation,
                style = LognFont.sans(MatchMetrics.CARD_BODY_SIZE).merge(TextStyle(lineHeight = MatchMetrics.CARD_BODY_LINE.sp)),
                color = LognDark.textSecondary,
                modifier = Modifier.padding(top = Space.sm),
            )
        }
    }
}

private const val SHAKE_MILLIS = 240
private const val SHAKES = 4
private const val REPORT_OUTLINE = 4.5f
