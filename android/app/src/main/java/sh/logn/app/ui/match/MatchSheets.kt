package sh.logn.app.ui.match

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import sh.logn.app.ui.components.BalloonShape
import sh.logn.app.ui.components.BalloonStyle
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.FullScreenSheet
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.MatchMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.OriginCard
import sh.logn.coreshell.i18n.Str

/**
 * Confirmação de saída. Espelho de LeaveMatchSheet.swift: não ameaça com uma perda que
 * não acontece (o XP já está na fila); diz o que acaba, a partida, e mostra os balões que
 * ficam. Quem decide se ela aparece é o Core. Fechar a folha é o mesmo que ficar.
 */
@Composable
fun LeaveMatchSheet(
    solved: Int,
    total: Int,
    balloonStates: BalloonStates,
    onStay: () -> Unit,
    onLeave: () -> Unit,
) {
    val context = LocalContext.current
    BottomSheet(onDismiss = onStay, background = LognDark.surface) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(start = MatchMetrics.sheetSide, end = MatchMetrics.sheetSide, top = MatchMetrics.sheetTop, bottom = MatchMetrics.sheetSide),
        ) {
            Text(
                Str.Leave.eyebrow(context).uppercase(),
                style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
                color = LognDark.textMuted,
            )
            Text(
                Str.Leave.solved(context, solved, total),
                style = LognFont.sans(MatchMetrics.SHEET_TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
                modifier = Modifier.padding(top = Space.md),
            )
            Text(
                Str.Leave.body(context),
                style = LognFont.sans(MatchMetrics.SHEET_BODY_SIZE).merge(TextStyle(lineHeight = MatchMetrics.SHEET_BODY_LINE.sp)),
                color = LognDark.textSecondary,
                modifier = Modifier.padding(top = MatchMetrics.dryGap),
            )
            // O que fica para trás, desenhado: perda concreta em vez de adjetivo.
            val pending = balloonStates.filter { !it.second }.map { it.first }
            if (pending.isNotEmpty()) {
                Row(Modifier.padding(top = MatchMetrics.xpTop), horizontalArrangement = Arrangement.spacedBy(Space.md)) {
                    for (letter in pending) {
                        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(Space.xs)) {
                            BalloonShape(BalloonStyle.Outline(LognDark.lineStrong, LEAVE_OUTLINE), MatchMetrics.headerBalloon)
                            Text(letter.toString(), style = LognFont.mono(MatchMetrics.LEAVE_LETTER_SIZE), color = LognDark.textMuted)
                        }
                    }
                }
            }
            // Ficar é o caminho fácil: cheio, embaixo, onde o polegar está.
            LognButton(Str.Leave.stay(context), ButtonVariant.Primary, Modifier.padding(top = MatchMetrics.leaveGap), onClick = onStay)
            LognButton(Str.Leave.confirm(context), ButtonVariant.Secondary, Modifier.padding(top = Space.md), onClick = onLeave)
        }
    }
}

/**
 * De onde o problema veio, quando não foi escrito para o LogN. O Core manda o cartão
 * pronto, na língua da trilha (ADR 0011); só o rótulo e o aviso de pausa são interface.
 */
@Composable
fun OriginSheet(
    card: OriginCard,
    clockPaused: Boolean,
    onClose: () -> Unit,
) {
    val context = LocalContext.current
    FullScreenSheet(onDismiss = onClose, background = LognDark.surface) {
        Column(Modifier.fillMaxSize()) {
            // O texto rola e o botão fica: a homenagem é longa.
            Column(
                Modifier
                    .weight(1f)
                    .verticalScroll(rememberScrollState())
                    .padding(start = MatchMetrics.sheetSide, end = MatchMetrics.sheetSide, top = MatchMetrics.sheetTop),
            ) {
                Text(
                    Str.Origin.eyebrow(context).uppercase(),
                    style = LognFont.mono(MatchMetrics.LABEL_SIZE, tracking = MatchMetrics.LABEL_TRACKING),
                    color = LognDark.textMuted,
                )
                Text(
                    card.name,
                    style = LognFont.sans(MatchMetrics.ORIGIN_NAME_SIZE, FontWeight.SemiBold),
                    color = LognDark.textPrimary,
                    modifier = Modifier.padding(top = Space.md),
                )
                Text(
                    card.role,
                    style = LognFont.mono(MatchMetrics.CARD_ANSWER_SIZE, tracking = MatchMetrics.ROLE_TRACKING),
                    color = LognDark.accent,
                    modifier = Modifier.padding(top = Space.xs),
                )
                Box(
                    Modifier
                        .padding(top = Space.screenMargin)
                        .fillMaxWidth()
                        .height(Stroke.hairline)
                        .background(LognDark.line),
                )
                Text(
                    card.body,
                    style = LognFont.sans(MatchMetrics.SHEET_BODY_SIZE).merge(TextStyle(lineHeight = MatchMetrics.SHEET_BODY_LINE.sp)),
                    color = LognDark.textSecondary,
                    modifier = Modifier.padding(top = Space.screenMargin),
                )
                if (clockPaused) PauseNotice(Modifier.padding(top = Space.xl))
            }
            LognButton(
                Str.Origin.close(context),
                ButtonVariant.Primary,
                Modifier.padding(start = MatchMetrics.sheetSide, end = MatchMetrics.sheetSide, top = Space.lg, bottom = MatchMetrics.sheetSide),
                onClick = onClose,
            )
        }
    }
}

/** O aviso da pausa é informação, não alarme: cor de informação, superfície elevada. */
@Composable
private fun PauseNotice(modifier: Modifier) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        modifier
            .fillMaxWidth()
            .background(LognDark.surfaceRaised, shape)
            .border(Stroke.hairline, LognDark.info.copy(alpha = PAUSE_BORDER_ALPHA), shape)
            .padding(MatchMetrics.cardPadding),
        horizontalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        Icon(LognIcon.PauseCircle, LognDark.info, IconMetrics.md)
        Text(Str.Origin.pause_notice(context), style = LognFont.mono(MatchMetrics.CARD_ANSWER_SIZE), color = LognDark.textSecondary)
    }
}

private const val LEAVE_OUTLINE = 4f
private const val PAUSE_BORDER_ALPHA = 0.3f
