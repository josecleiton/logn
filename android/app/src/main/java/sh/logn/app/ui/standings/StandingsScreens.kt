package sh.logn.app.ui.standings

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import sh.logn.app.ui.components.BALLOON_LETTERS
import sh.logn.app.ui.components.BalloonShape
import sh.logn.app.ui.components.BalloonStyle
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.components.balloonColor
import sh.logn.app.ui.match.formatClock
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.StandingsMetrics
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.ScoreCell
import sh.logn.core.LogN.ScoreCellState
import sh.logn.core.LogN.ScoreboardRow
import sh.logn.core.LogN.StandingRow
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/** O fio de baixo de uma linha: acento na do jogador, `rowLine` nas outras. */
private fun Modifier.rowLine(isUser: Boolean): Modifier =
    drawBehind {
        val h = Stroke.hairline.toPx()
        if (isUser) drawRect(LognDark.accent, Offset(0f, 0f), size.copy(height = h))
        drawRect(if (isUser) LognDark.accent else LognDark.rowLine, Offset(0f, size.height - h), size.copy(height = h))
    }

/** Tarja de dados de exemplo: o placar ainda não tem servidor. Em `info`, não em `warn`. */
@Composable
fun SampleDataNotice() {
    val context = LocalContext.current
    Row(
        Modifier
            .fillMaxWidth()
            .background(LognDark.tintInfo)
            .drawBehind {
                val h = Stroke.hairline.toPx()
                drawRect(LognDark.info, Offset(0f, size.height - h), size.copy(height = h))
            }.padding(horizontal = Space.screenMargin, vertical = Space.md),
        horizontalArrangement = Arrangement.spacedBy(Space.sm),
    ) {
        Icon(LognIcon.TestTube, LognDark.infoInk, IconMetrics.sm, Modifier.padding(top = Stroke.hairline))
        Text(Str.Status.sample_standings(context), style = LognFont.sans(StandingsMetrics.NOTICE_SIZE), color = LognDark.infoInk)
    }
}

/**
 * O ranking de celular (tela 6 do DS). Espelho de StandingsView.swift: abas Global e
 * Sede, linhas densas, e a linha do jogador grudada no pé com o tratamento do telão.
 */
@Composable
fun StandingsScreen(
    view: ViewModel,
    onScoreboard: () -> Unit,
) {
    val context = LocalContext.current
    var home by rememberSaveable { mutableStateOf(false) }
    val rows = if (home) view.standingsHome else view.standingsGlobal
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.canvas),
    ) {
        Column(Modifier.padding(start = Space.screenMargin, end = Space.screenMargin, top = StandingsMetrics.headerTop)) {
            Text(
                Str.Standings.title(context),
                style = LognFont.sans(StandingsMetrics.TITLE_SIZE, FontWeight.SemiBold, StandingsMetrics.TITLE_TRACKING),
                color = LognDark.textPrimary,
                modifier = Modifier.padding(bottom = StandingsMetrics.titleBottom),
            )
            Row(
                Modifier
                    .fillMaxWidth()
                    .drawBehind {
                        val h = Stroke.hairline.toPx()
                        drawRect(LognDark.line, Offset(0f, size.height - h), size.copy(height = h))
                    },
                horizontalArrangement = Arrangement.spacedBy(Space.screenMargin),
            ) {
                TabButton(Str.Standings.global_tab(context), !home) { home = false }
                TabButton(view.userStanding.university, home) { home = true }
            }
        }
        if (view.standingsAreSample) SampleDataNotice()
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState()),
        ) {
            for (row in rows) StandingRowView(row)
            val shape = RoundedCornerShape(Radius.sm)
            Box(
                Modifier
                    .padding(horizontal = Space.screenMargin, vertical = Space.lg)
                    .fillMaxWidth()
                    .background(LognDark.surface, shape)
                    .border(Stroke.hairline, LognDark.line, shape)
                    .clickable(role = Role.Button, onClick = onScoreboard)
                    .padding(vertical = Space.md),
                contentAlignment = Alignment.Center,
            ) { Text(Str.Standings.full_screen(context), style = LognFont.label, color = LognDark.accentInk) }
        }
        // A linha do jogador não rola com a lista: fica sempre à vista.
        StandingRowView(view.userStanding)
    }
}

/** Aba ativa: borda de 2 em acento, 14/600. */
@Composable
private fun TabButton(
    title: String,
    active: Boolean,
    onClick: () -> Unit,
) {
    Box(
        Modifier
            .semantics { selected = active }
            .clickable(role = Role.Tab, onClick = onClick)
            .drawBehind {
                if (active) {
                    val h = StandingsMetrics.tabIndicator.toPx()
                    drawRect(LognDark.accent, Offset(0f, size.height - h / 2), size.copy(height = h))
                }
            }.padding(vertical = Space.md, horizontal = Space.xxs),
    ) {
        Text(
            title,
            style = LognFont.sans(StandingsMetrics.TAB_SIZE, if (active) FontWeight.SemiBold else FontWeight.Normal),
            color = if (active) LognDark.textPrimary else LognDark.textMuted,
        )
    }
}

/** Linha: rank mono, handle e universidade, `resolvidos · penalidade` à direita. */
@Composable
fun StandingRowView(row: StandingRow) {
    val context = LocalContext.current
    val label =
        if (row.isUser) {
            Str.Standings.row_accessibility_user(context, row.rank, row.handle, row.university, row.solved, row.penalty)
        } else {
            Str.Standings.row_accessibility(context, row.rank, row.handle, row.university, row.solved, row.penalty)
        }
    Row(
        Modifier
            .fillMaxWidth()
            .background(if (row.isUser) LognDark.accentTint else Color.Transparent)
            .rowLine(row.isUser)
            .clearAndSetSemantics { contentDescription = label }
            .padding(horizontal = Space.screenMargin, vertical = if (row.isUser) Space.lg else StandingsMetrics.rowPaddingV),
        horizontalArrangement = Arrangement.spacedBy(StandingsMetrics.rowGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            "${row.rank}",
            style = LognFont.mono(StandingsMetrics.RANK_SIZE),
            color = if (row.isUser) LognDark.accentInk else LognDark.textMuted,
            modifier = Modifier.width(StandingsMetrics.rankWidth),
        )
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(Space.xxs)) {
            Text(
                row.handle,
                style = LognFont.sans(StandingsMetrics.HANDLE_SIZE, if (row.isUser) FontWeight.SemiBold else FontWeight.Medium),
                color = LognDark.textPrimary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                if (row.note.isEmpty()) row.university else "${row.university} · ${row.note}",
                style = LognFont.mono(StandingsMetrics.SUB_SIZE),
                color = if (row.isUser) LognDark.accentInk else LognDark.textMuted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        Text(
            "${row.solved} · ${row.penalty}",
            style = LognFont.mono(StandingsMetrics.SCORE_SIZE, if (row.isUser) FontWeight.SemiBold else FontWeight.Normal),
            color = LognDark.textPrimary,
        )
    }
}

private fun cellBorder(state: ScoreCellState): Color =
    when (state) {
        ScoreCellState.ACCEPTED -> LognDark.correct
        ScoreCellState.FAILED -> LognDark.wrong
        ScoreCellState.FROZEN -> LognDark.info
        ScoreCellState.UNTRIED -> LognDark.line
    }

private fun cellBackground(state: ScoreCellState): Color =
    when (state) {
        ScoreCellState.ACCEPTED -> LognDark.tintOk
        ScoreCellState.FAILED -> LognDark.tintErr
        ScoreCellState.FROZEN -> LognDark.tintInfo
        ScoreCellState.UNTRIED -> Color.Transparent
    }

private fun cellInk(state: ScoreCellState): Color =
    when (state) {
        ScoreCellState.ACCEPTED -> LognDark.correctInk
        ScoreCellState.FAILED -> LognDark.wrongInk
        ScoreCellState.FROZEN -> LognDark.infoInk
        ScoreCellState.UNTRIED -> LognDark.textMuted
    }

private fun cellName(
    context: android.content.Context,
    state: ScoreCellState,
): String =
    when (state) {
        ScoreCellState.ACCEPTED -> Str.Verdict.accepted(context)
        ScoreCellState.FAILED -> Str.Verdict.failed(context)
        ScoreCellState.FROZEN -> Str.Verdict.frozen(context)
        ScoreCellState.UNTRIED -> Str.Verdict.untried(context)
    }

/**
 * O telão do ginásio (tela 1 do DS), o único que não é phone-first. Espelho de
 * ScoreboardView.swift: o grid é mais largo que o aparelho, então rank e time ficam
 * congelados à esquerda enquanto A–M rolam.
 */
@Composable
fun ScoreboardScreen(
    view: ViewModel,
    onClose: () -> Unit,
) {
    val context = LocalContext.current
    val rows = view.scoreboard
    val frozen = view.matchView.isFrozen
    val horizontal = rememberScrollState()
    BackHandler(onBack = onClose)
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.canvas)
            .statusBarsPadding(),
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .drawBehind {
                    val h = Stroke.hairline.toPx()
                    drawRect(LognDark.line, Offset(0f, size.height - h), size.copy(height = h))
                }.padding(start = Space.sm, end = Space.screenMargin, top = Space.sm, bottom = Space.sm),
            horizontalArrangement = Arrangement.spacedBy(StandingsMetrics.barGap),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val close = Str.Scoreboard.close(context)
            Box(
                Modifier
                    .size(IconMetrics.touch)
                    .semantics { contentDescription = close }
                    .clickable(role = Role.Button, onClick = onClose),
                contentAlignment = Alignment.Center,
            ) { Icon(LognIcon.Close, LognDark.textMuted, IconMetrics.md) }
            // Nome e selo juntos ocupam o que sobra; o nome só cede quando não cabe.
            Row(
                Modifier.weight(1f),
                horizontalArrangement = Arrangement.spacedBy(StandingsMetrics.barGap),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    view.contestName,
                    style = LognFont.mono(StandingsMetrics.CONTEST_SIZE, FontWeight.SemiBold, StandingsMetrics.CONTEST_TRACKING),
                    color = LognDark.textPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f, fill = false),
                )
                if (frozen) {
                    Text(
                        Str.Scoreboard.frozen(context),
                        style = LognFont.mono(StandingsMetrics.FROZEN_SIZE, tracking = StandingsMetrics.FROZEN_TRACKING),
                        color = LognDark.warnInk,
                        modifier =
                            Modifier
                                .background(LognDark.tintWarn, RoundedCornerShape(Radius.xs))
                                .border(Stroke.hairline, LognDark.warn, RoundedCornerShape(Radius.xs))
                                .padding(horizontal = Space.md, vertical = StandingsMetrics.frozenPaddingV),
                    )
                }
            }
            // Fora de partida não há relógio: 00:00 o jogador leria como "acabou".
            if (view.matchView.contestSeconds > 0) {
                Text(
                    formatClock(view.matchView.contestSeconds),
                    style = LognFont.mono(StandingsMetrics.CLOCK_SIZE, FontWeight.Medium, StandingsMetrics.CLOCK_TRACKING),
                    color = LognDark.textPrimary,
                )
            }
        }
        if (view.standingsAreSample) SampleDataNotice()
        Row(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState()),
        ) {
            // Identidade: fixa.
            Column(
                Modifier
                    .width(StandingsMetrics.rank + StandingsMetrics.team)
                    .drawBehind {
                        val w = Stroke.hairline.toPx()
                        drawRect(LognDark.line, Offset(size.width - w, 0f), size.copy(width = w))
                    },
            ) {
                IdentityHeader()
                for (row in rows) IdentityCell(row)
            }
            // Números e as 13 células: rolam na horizontal, juntas.
            Column(Modifier.horizontalScroll(horizontal)) {
                ProblemHeader()
                for (row in rows) ScoreCells(row)
            }
        }
        Legend()
    }
}

@Composable
private fun headerModifier(): Modifier =
    Modifier
        .height(StandingsMetrics.headerHeight)
        .background(LognDark.surfaceRaised)
        .drawBehind {
            val h = Stroke.hairline.toPx()
            drawRect(LognDark.line, Offset(0f, size.height - h), size.copy(height = h))
        }

@Composable
private fun IdentityHeader() {
    val context = LocalContext.current
    val style = LognFont.mono(StandingsMetrics.HEADER_SIZE, tracking = StandingsMetrics.HEADER_TRACKING)
    Row(headerModifier().fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(
            Str.Scoreboard.rank(context),
            style = style,
            color = LognDark.textMuted,
            modifier =
                Modifier
                    .width(StandingsMetrics.rank)
                    .padding(start = Space.screenMargin),
        )
        Text(Str.Scoreboard.team(context), style = style, color = LognDark.textMuted, modifier = Modifier.width(StandingsMetrics.team))
    }
}

@Composable
private fun ProblemHeader() {
    val context = LocalContext.current
    val style = LognFont.mono(StandingsMetrics.HEADER_SIZE, tracking = StandingsMetrics.HEADER_TRACKING)
    Row(headerModifier(), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.width(StandingsMetrics.solved), contentAlignment = Alignment.Center) {
            Text(Str.Scoreboard.slv(context), style = style, color = LognDark.textMuted)
        }
        Box(Modifier.width(StandingsMetrics.penalty), contentAlignment = Alignment.Center) {
            Text(Str.Scoreboard.pen(context), style = style, color = LognDark.textMuted)
        }
        for (letter in BALLOON_LETTERS) {
            Column(
                Modifier.width(StandingsMetrics.cell),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(StandingsMetrics.gap3),
            ) {
                BalloonShape(BalloonStyle.Filled(balloonColor(letter)), StandingsMetrics.headerBalloon)
                Text(letter.toString(), style = LognFont.mono(StandingsMetrics.LETTER_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
            }
        }
    }
}

@Composable
private fun IdentityCell(row: ScoreboardRow) {
    Row(
        Modifier
            .fillMaxWidth()
            .height(StandingsMetrics.rowHeight)
            .background(if (row.isUser) LognDark.accentTint else LognDark.surface)
            .bottomLine(row.isUser),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            "${row.rank}",
            style = LognFont.mono(StandingsMetrics.RANK_CELL_SIZE, FontWeight.SemiBold),
            color = if (row.isUser) LognDark.accentInk else LognDark.textPrimary,
            modifier =
                Modifier
                    .width(StandingsMetrics.rank)
                    .padding(start = Space.screenMargin),
        )
        Column(
            Modifier
                .width(StandingsMetrics.team)
                .padding(end = Space.md),
            verticalArrangement = Arrangement.spacedBy(Space.xxs),
        ) {
            Text(
                row.team,
                style = LognFont.sans(StandingsMetrics.TEAM_SIZE, FontWeight.SemiBold),
                color = if (row.isUser) LognDark.accentInk else LognDark.textPrimary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                row.university,
                style = LognFont.mono(StandingsMetrics.HEADER_SIZE, tracking = StandingsMetrics.UNIVERSITY_TRACKING),
                color = LognDark.textMuted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

private fun Modifier.bottomLine(isUser: Boolean): Modifier =
    drawBehind {
        val h = Stroke.hairline.toPx()
        drawRect(if (isUser) LognDark.accent else LognDark.rowLine, Offset(0f, size.height - h), size.copy(height = h))
    }

@Composable
private fun ScoreCells(row: ScoreboardRow) {
    val context = LocalContext.current
    Row(
        Modifier
            .height(StandingsMetrics.rowHeight)
            .background(if (row.isUser) LognDark.accentTint else LognDark.surface)
            .bottomLine(row.isUser),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.width(StandingsMetrics.solved), contentAlignment = Alignment.Center) {
            Text("${row.solved}", style = LognFont.mono(StandingsMetrics.SOLVED_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
        }
        Box(Modifier.width(StandingsMetrics.penalty), contentAlignment = Alignment.Center) {
            Text("${row.penalty}", style = LognFont.mono(StandingsMetrics.PENALTY_SIZE), color = LognDark.textSecondary)
        }
        for ((index, cell) in row.cells.withIndex()) {
            val letter = BALLOON_LETTERS[index.coerceAtMost(BALLOON_LETTERS.lastIndex)]
            ScoreCellView(cell, Str.Scoreboard.cell_accessibility(context, letter.toString(), cellName(context, cell.state)))
        }
    }
}

/** Célula de 38, margem de 2, raio 2. Topo é o símbolo; base, o minuto a 75%. */
@Composable
private fun ScoreCellView(
    cell: ScoreCell,
    label: String,
) {
    val shape = RoundedCornerShape(Radius.xs)
    Column(
        Modifier
            .width(StandingsMetrics.cell)
            .padding(horizontal = Space.xxs)
            .height(StandingsMetrics.cellHeight)
            .background(cellBackground(cell.state), shape)
            .border(Stroke.hairline, cellBorder(cell.state), shape)
            .clearAndSetSemantics { contentDescription = label },
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        val ink = cellInk(cell.state)
        if (cell.top.isNotEmpty()) Text(cell.top, style = LognFont.mono(StandingsMetrics.CELL_TOP_SIZE, FontWeight.SemiBold), color = ink)
        if (cell.bottom.isNotEmpty()) {
            Text(cell.bottom, style = LognFont.mono(StandingsMetrics.CELL_BOTTOM_SIZE), color = ink.copy(alpha = StandingsMetrics.CELL_BOTTOM_ALPHA))
        }
    }
}

/** A legenda das quatro cores, sempre visível: quebra em duas linhas em vez de rolar. */
@Composable
private fun Legend() {
    val context = LocalContext.current
    Column(
        Modifier
            .fillMaxWidth()
            .background(LognDark.surface)
            .drawBehind { drawRect(LognDark.line, Offset.Zero, size.copy(height = Stroke.hairline.toPx())) }
            .navigationBarsPadding()
            .padding(horizontal = Space.screenMargin, vertical = StandingsMetrics.legendPaddingV),
        verticalArrangement = Arrangement.spacedBy(Space.sm),
    ) {
        // Como no iOS: cada item na largura dele, o espaço sobrando no meio. O da
        // esquerda é que encolhe (e quebra) quando os dois não cabem.
        val left = Modifier.padding(end = StandingsMetrics.legendGap)
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            LegendItem(ScoreCellState.ACCEPTED, Str.Scoreboard.legend_accepted(context), left.weight(1f, fill = false))
            LegendItem(ScoreCellState.FAILED, Str.Scoreboard.legend_failed(context), Modifier)
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            LegendItem(ScoreCellState.FROZEN, Str.Scoreboard.legend_frozen(context), left.weight(1f, fill = false))
            LegendItem(ScoreCellState.UNTRIED, Str.Scoreboard.legend_untried(context), Modifier)
        }
    }
}

@Composable
private fun LegendItem(
    state: ScoreCellState,
    label: String,
    modifier: Modifier,
) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(Space.sm), verticalAlignment = Alignment.CenterVertically) {
        val shape = RoundedCornerShape(Radius.xs)
        Box(
            Modifier
                .size(StandingsMetrics.swatchWidth, StandingsMetrics.swatchHeight)
                .background(cellBackground(state), shape)
                .border(Stroke.hairline, cellBorder(state), shape),
        )
        Text(label, style = LognFont.mono(StandingsMetrics.HEADER_SIZE), color = LognDark.textMuted)
    }
}
