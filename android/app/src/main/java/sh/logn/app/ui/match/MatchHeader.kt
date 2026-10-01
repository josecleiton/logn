package sh.logn.app.ui.match

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import sh.logn.app.ui.components.BalloonShape
import sh.logn.app.ui.components.BalloonStyle
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.components.balloonColor
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.MatchMetrics
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.coreshell.i18n.Str
import java.util.Locale

/** (letra, aceito) de cada problema da partida. */
typealias BalloonStates = List<Pair<Char, Boolean>>

/** "mm:ss", ou "hh:mm:ss" a partir de uma hora, em dígitos tabulares. */
fun formatClock(seconds: Int): String =
    if (seconds >= HOUR) {
        String.format(Locale.ROOT, "%02d:%02d:%02d", seconds / HOUR, seconds % HOUR / MINUTE, seconds % MINUTE)
    } else {
        String.format(Locale.ROOT, "%02d:%02d", seconds / MINUTE, seconds % MINUTE)
    }

/**
 * Três corações: cheio é `wrong`, vazio `heartOff`. O contador mono é o reforço que o DS
 * pede; em partida o cabeçalho usa a forma compacta, e o rótulo carrega a informação.
 */
@Composable
fun LifeBar(
    lives: Int,
    maxLives: Int,
    heartSize: Dp,
    modifier: Modifier = Modifier,
    showsCounter: Boolean = true,
) {
    val label = Str.Profile.lives_accessibility(LocalContext.current, lives, maxLives)
    Row(
        modifier.clearAndSetSemantics { contentDescription = label },
        horizontalArrangement = Arrangement.spacedBy(Space.xs),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        for (i in 0 until maxLives) {
            Icon(if (i < lives) LognIcon.HeartFill else LognIcon.Heart, if (i < lives) LognDark.wrong else LognDark.heartOff, heartSize)
        }
        if (showsCounter) Text("$lives / $maxLives", style = LognFont.label, color = LognDark.textSecondary)
    }
}

/**
 * O cabeçalho fixo da partida (3b · Contest): problema atual, relógio, vidas e a fileira
 * de balões desta partida. Fundo `canvas`, divisor embaixo: é a moldura da tela.
 */
@Composable
fun MatchHeader(
    letter: Char,
    remainingSeconds: Int,
    isFrozen: Boolean,
    lives: Int,
    maxLives: Int,
    balloonStates: BalloonStates,
    modifier: Modifier = Modifier,
    currentIsAlive: Boolean = true,
    showsBalloonRow: Boolean = true,
    onLeave: (() -> Unit)? = null,
) {
    val context = LocalContext.current
    val clockColor =
        when {
            remainingSeconds <= CRITICAL_SECONDS -> LognDark.wrongInk
            isFrozen || remainingSeconds <= WARN_SECONDS -> LognDark.warnInk
            else -> LognDark.textSecondary
        }
    Column(
        modifier
            .fillMaxWidth()
            .background(LognDark.canvas)
            .drawBehind {
                val y = size.height - Stroke.hairline.toPx() / 2
                drawLine(LognDark.line, Offset(0f, y), Offset(size.width, y), Stroke.hairline.toPx())
            }.padding(start = MatchMetrics.headerSide, end = MatchMetrics.headerSide, top = Space.lg, bottom = MatchMetrics.headerBottom),
        verticalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Space.md)) {
            if (onLeave != null) {
                // Cinza e não acento: sair não é a ação que o app quer incentivar.
                val label = Str.Leave.accessibility(context)
                Box(
                    Modifier
                        .size(IconMetrics.touch)
                        .semantics { contentDescription = label }
                        .clickable(role = Role.Button, onClick = onLeave),
                    contentAlignment = Alignment.Center,
                ) { Icon(LognIcon.Close, LognDark.textMuted, IconMetrics.md) }
            }
            BalloonShape(
                if (currentIsAlive) BalloonStyle.Filled(balloonColor(letter)) else BalloonStyle.Outline(LognDark.lineStrong, OUTLINE_DEAD),
                MatchMetrics.headerBalloon,
                showString = true,
            )
            Text(
                Str.Match.problem_letter(context, letter.toString()),
                style = LognFont.mono(MatchMetrics.HEADER_LABEL_SIZE, tracking = MatchMetrics.HEADER_LABEL_TRACKING),
                color = LognDark.textMuted,
            )
            Spacer(Modifier.weight(1f))
            val clockLabel = Str.Match.time_remaining(context, remainingSeconds)
            Text(
                formatClock(remainingSeconds),
                style = LognFont.mono(MatchMetrics.CLOCK_SIZE, FontWeight.Medium),
                color = clockColor,
                modifier = Modifier.clearAndSetSemantics { contentDescription = clockLabel },
            )
            LifeBar(lives, maxLives, MatchMetrics.heart, Modifier.padding(start = Space.md), showsCounter = false)
        }
        if (showsBalloonRow) BalloonRow(letter, balloonStates)
    }
}

/**
 * A fileira de problemas, até a letra do último problema desta partida. A letra embaixo
 * cai para 8 (abaixo do mínimo do DS): é reforço; o rótulo de cada balão é o que conta.
 */
@Composable
private fun BalloonRow(
    current: Char,
    states: BalloonStates,
) {
    val context = LocalContext.current
    Row(horizontalArrangement = Arrangement.spacedBy(MatchMetrics.rowGap)) {
        for ((l, accepted) in states) {
            val status =
                when {
                    accepted -> Str.Verdict.accepted(context)
                    l == current -> Str.Verdict.solving(context)
                    else -> Str.Verdict.open(context)
                }
            val label = Str.Match.problem_status(context, l.toString(), status)
            Column(
                Modifier.clearAndSetSemantics { contentDescription = label },
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(Space.xxs),
            ) {
                val style =
                    when {
                        accepted -> BalloonStyle.Filled(balloonColor(l))
                        l == current -> BalloonStyle.Outline(balloonColor(l), OUTLINE_ROW)
                        else -> BalloonStyle.Outline(LognDark.lineStrong, OUTLINE_ROW)
                    }
                BalloonShape(style, MatchMetrics.rowBalloon)
                // A paleta é certificada para 3:1 como forma, não 4.5:1 como texto.
                Text(l.toString(), style = LognFont.mono(MatchMetrics.ROW_LETTER_SIZE), color = LognDark.textMuted)
            }
        }
    }
}

private const val HOUR = 3600
private const val MINUTE = 60
private const val CRITICAL_SECONDS = 15
private const val WARN_SECONDS = 45
private const val OUTLINE_DEAD = 4f
private const val OUTLINE_ROW = 5f
