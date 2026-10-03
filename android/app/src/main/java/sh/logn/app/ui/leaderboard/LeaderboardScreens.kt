package sh.logn.app.ui.leaderboard

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
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Text
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.sp
import sh.logn.app.AppLocale
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.ShellState
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.LeaderboardMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.LeaderboardAgeUnit
import sh.logn.core.LogN.LeaderboardMeStatus
import sh.logn.core.LogN.LeaderboardRow
import sh.logn.core.LogN.LeaderboardState
import sh.logn.core.LogN.LeaderboardView
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str
import java.text.NumberFormat
import java.util.Locale
import sh.logn.app.ui.theme.Stroke as LognStroke

private const val ME_KEY = "me"

/** O nome do placar: o apelido, ou "jogador #N" do catálogo. */
fun leaderboardName(
    context: android.content.Context,
    anonNumber: Int,
    nickname: String?,
): String = nickname ?: Str.Leaderboard.anon_name(context, anonNumber.toString())

/** XP com o separador de milhar da língua do app ("4.850"). */
private fun formatXp(
    context: android.content.Context,
    xp: Int,
): String = NumberFormat.getIntegerInstance(Locale.forLanguageTag(AppLocale.current(context))).format(xp)

private fun ageText(
    context: android.content.Context,
    view: LeaderboardView,
): String {
    val n = view.ageValue.toInt()
    return when (view.ageUnit) {
        LeaderboardAgeUnit.JUSTNOW -> Str.Leaderboard.age_now(context)
        LeaderboardAgeUnit.MINUTES -> Str.Leaderboard.age_minutes(context, n)
        LeaderboardAgeUnit.HOURS -> Str.Leaderboard.age_hours(context, n)
        LeaderboardAgeUnit.DAYS -> Str.Leaderboard.age_days(context, n)
    }
}

private fun Modifier.bottomLine(color: Color): Modifier =
    drawBehind {
        val h = LognStroke.hairline.toPx()
        drawRect(color, Offset(0f, size.height - h), size.copy(height = h))
    }

/**
 * O placar geral de XP (docs/specs/logn_placar_spec.md), no canvas "LogN — Placar geral de
 * XP": a Global, sem abas, com a linha do jogador fixa embaixo quando ela sai de vista.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LeaderboardScreen(
    view: ViewModel,
    onGoToTrail: () -> Unit,
) {
    val dispatch = LocalDispatch.current
    val board = view.leaderboard
    LaunchedEffect(Unit) { dispatch(Event.LeaderboardOpened) }
    Column(Modifier.fillMaxSize().background(LognDark.canvas)) {
        Header()
        when (board.state) {
            LeaderboardState.SIGNEDOUT -> GuestState()
            LeaderboardState.LOADING -> Skeleton()
            LeaderboardState.UNAVAILABLE -> UnavailableState()
            LeaderboardState.CLOSED -> {
                if (board.offline) OfflineBanner(board)
                ClosedState(board, Modifier.weight(1f))
                PinnedRow(board)
            }
            LeaderboardState.OPEN -> {
                if (board.offline) OfflineBanner(board)
                PullToRefreshBox(
                    isRefreshing = board.refreshing,
                    onRefresh = { dispatch(Event.LeaderboardRefresh) },
                    modifier = Modifier.weight(1f),
                ) { OpenList(board) }
                when (board.meStatus) {
                    LeaderboardMeStatus.ZEROXP -> ZeroXpFooter(board, onGoToTrail)
                    else -> Unit
                }
            }
        }
    }
}

@Composable
private fun Header() {
    val context = LocalContext.current
    Column(
        Modifier
            .fillMaxWidth()
            .bottomLine(LognDark.line)
            .padding(start = Space.screenMargin, end = Space.screenMargin, top = LeaderboardMetrics.headerTop),
        verticalArrangement = Arrangement.spacedBy(LeaderboardMetrics.headerGap),
    ) {
        Text(
            Str.Leaderboard.title(context),
            style = LognFont.sans(LeaderboardMetrics.TITLE_SIZE, FontWeight.SemiBold, LeaderboardMetrics.TITLE_TRACKING),
            color = LognDark.textPrimary,
            modifier = Modifier.semantics { heading() },
        )
        Text(
            Str.Leaderboard.subtitle(context),
            style = LognFont.mono(LeaderboardMetrics.LABEL_SIZE, tracking = LeaderboardMetrics.LABEL_TRACKING),
            color = LognDark.textMuted,
            modifier = Modifier.padding(bottom = LeaderboardMetrics.headerBottom),
        )
    }
}

@Composable
private fun OfflineBanner(board: LeaderboardView) {
    val context = LocalContext.current
    Row(
        Modifier
            .fillMaxWidth()
            .background(LognDark.tintInfo)
            .bottomLine(LognDark.info)
            .padding(horizontal = Space.screenMargin, vertical = LeaderboardMetrics.bannerPaddingV),
        horizontalArrangement = Arrangement.spacedBy(Space.sm),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(LognIcon.WifiSlash, LognDark.infoInk, LeaderboardMetrics.bannerIcon)
        Text(
            Str.Leaderboard.offline(context, ageText(context, board)),
            style = LognFont.sans(LeaderboardMetrics.BANNER_SIZE),
            color = LognDark.infoInk,
        )
    }
}

@Composable
private fun OpenList(board: LeaderboardView) {
    val state = rememberLazyListState()
    // A linha fixa aparece quando a do jogador não está na lista ou saiu de vista.
    val meVisible by remember(board) {
        derivedStateOf { state.layoutInfo.visibleItemsInfo.any { it.key == ME_KEY } }
    }
    val showsPinned = board.meStatus != LeaderboardMeStatus.ZEROXP && (!board.meInRows || !meVisible)
    Column(Modifier.fillMaxSize()) {
        LazyColumn(Modifier.weight(1f), state = state) {
            // Na lista, a linha do jogador já tem o tratamento da fixa: é por ela que ele
            // se acha no meio dos outros.
            items(board.rows, key = { if (it.isMe) ME_KEY else "r${it.rank}" }) { row ->
                if (row.isMe) PinnedRow(board) else ListRow(row)
            }
        }
        if (showsPinned) PinnedRow(board)
    }
}

/** Uma linha: posição, nome e XP. O anônimo em mono e cinza, para não parecer erro. */
@Composable
private fun ListRow(row: LeaderboardRow) {
    val context = LocalContext.current
    val name = leaderboardName(context, row.anonNumber, row.nickname)
    val xp = formatXp(context, row.xp)
    val label =
        if (row.isMe) {
            Str.Leaderboard.row_accessibility_me(context, row.rank, name, xp)
        } else {
            Str.Leaderboard.row_accessibility(context, row.rank, name, xp)
        }
    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = LeaderboardMetrics.rowMinHeight)
            .bottomLine(LognDark.rowLine)
            .clearAndSetSemantics { contentDescription = label }
            .padding(horizontal = Space.screenMargin, vertical = LeaderboardMetrics.rowPaddingV),
        horizontalArrangement = Arrangement.spacedBy(LeaderboardMetrics.rowGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            "${row.rank}",
            style = LognFont.mono(LeaderboardMetrics.RANK_SIZE),
            color = LognDark.textMuted,
            modifier = Modifier.width(LeaderboardMetrics.rankWidth),
        )
        Text(
            name,
            style = nameStyle(row.nickname, FontWeight.Medium),
            color = if (row.nickname != null) LognDark.textPrimary else LognDark.textSecondary,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        Text(
            Str.Leaderboard.xp(context, xp),
            style = LognFont.mono(LeaderboardMetrics.XP_SIZE),
            color = LognDark.textPrimary,
        )
    }
}

private fun nameStyle(
    nickname: String?,
    weight: FontWeight,
): TextStyle =
    if (nickname != null) {
        LognFont.sans(LeaderboardMetrics.NICKNAME_SIZE, weight)
    } else {
        LognFont.mono(LeaderboardMetrics.ANON_SIZE, if (weight == FontWeight.Medium) FontWeight.Normal else weight)
    }

/** A linha do jogador, fixa embaixo: fundo em acento, fio de acento em cima e embaixo. */
@Composable
private fun PinnedRow(board: LeaderboardView) {
    val context = LocalContext.current
    val me = board.me
    val name = leaderboardName(context, me.anonNumber, me.nickname)
    val xp = formatXp(context, me.xp)
    val ranked = board.meStatus == LeaderboardMeStatus.RANKED && me.rank > 0
    val label =
        if (ranked) {
            Str.Leaderboard.row_accessibility_me(context, me.rank, name, xp)
        } else {
            Str.Leaderboard.me_unranked_accessibility(context, name, xp)
        }
    Row(
        Modifier
            .fillMaxWidth()
            .background(LognDark.accentTint)
            .drawBehind {
                val h = LognStroke.hairline.toPx()
                drawRect(LognDark.accent, Offset.Zero, size.copy(height = h))
                drawRect(LognDark.accent, Offset(0f, size.height - h), size.copy(height = h))
            }.clearAndSetSemantics { contentDescription = label }
            .padding(horizontal = Space.screenMargin, vertical = Space.lg),
        horizontalArrangement = Arrangement.spacedBy(LeaderboardMetrics.rowGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            if (ranked) "${me.rank}" else Str.Leaderboard.rank_none(context),
            style = LognFont.mono(LeaderboardMetrics.RANK_SIZE),
            color = LognDark.accentInk,
            modifier = Modifier.width(LeaderboardMetrics.rankWidth),
        )
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(LeaderboardMetrics.meNameGap)) {
            Text(
                name,
                style = nameStyle(me.nickname, FontWeight.SemiBold),
                color = LognDark.textPrimary,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                if (board.meStatus == LeaderboardMeStatus.HIDDEN) Str.Leaderboard.hidden_label(context) else Str.Leaderboard.you(context),
                style = LognFont.mono(LeaderboardMetrics.LABEL_SIZE, tracking = LeaderboardMetrics.LABEL_TRACKING),
                color = LognDark.accentInk,
            )
        }
        Text(
            Str.Leaderboard.xp(context, xp),
            style = LognFont.mono(LeaderboardMetrics.XP_SIZE, FontWeight.SemiBold),
            color = LognDark.textPrimary,
        )
    }
}

/** Ainda não há jogadores: "7 / 10", a barra de dez e quantos faltam. */
@Composable
private fun ClosedState(
    board: LeaderboardView,
    modifier: Modifier,
) {
    val context = LocalContext.current
    val threshold = board.threshold.toInt()
    val players = (threshold - board.missing.toInt()).coerceAtLeast(0)
    Column(
        modifier
            .fillMaxWidth()
            .padding(horizontal = LeaderboardMetrics.emptyPaddingH),
        verticalArrangement = Arrangement.spacedBy(LeaderboardMetrics.closedGap, Alignment.CenterVertically),
    ) {
        Text(
            Str.Leaderboard.closed_eyebrow(context),
            style = LognFont.mono(LeaderboardMetrics.LABEL_SIZE, tracking = LeaderboardMetrics.LABEL_TRACKING),
            color = LognDark.textMuted,
        )
        Row(
            Modifier.clearAndSetSemantics { contentDescription = Str.Leaderboard.closed_accessibility(context, players, threshold) },
            horizontalArrangement = Arrangement.spacedBy(LeaderboardMetrics.closedCountGap),
            verticalAlignment = Alignment.Bottom,
        ) {
            Text(
                "$players",
                style = LognFont.mono(LeaderboardMetrics.COUNT_SIZE, FontWeight.SemiBold).copy(lineHeight = LeaderboardMetrics.COUNT_SIZE.sp),
                color = LognDark.textPrimary,
                modifier = Modifier.alignByBaseline(),
            )
            Text(
                Str.Leaderboard.closed_of(context, threshold),
                style = LognFont.mono(LeaderboardMetrics.COUNT_OF_SIZE),
                color = LognDark.textMuted,
                modifier = Modifier.alignByBaseline(),
            )
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(LeaderboardMetrics.barGap)) {
            for (i in 0 until threshold) {
                Box(
                    Modifier
                        .weight(1f)
                        .height(LeaderboardMetrics.barHeight)
                        .background(if (i < players) LognDark.accent else LognDark.line, RoundedCornerShape(LeaderboardMetrics.barRadius)),
                )
            }
        }
        Column(verticalArrangement = Arrangement.spacedBy(LeaderboardMetrics.textGap)) {
            Text(
                Str.Leaderboard.closed_missing(context, board.missing.toInt()),
                style = LognFont.sans(LeaderboardMetrics.CLOSED_TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
            )
            val body =
                if (board.meStatus == LeaderboardMeStatus.WAITING) {
                    Str.Leaderboard.closed_body(context) + " " + Str.Leaderboard.closed_counted(context)
                } else {
                    Str.Leaderboard.closed_body(context)
                }
            Text(body, style = bodyStyle(), color = LognDark.textSecondary)
        }
    }
}

private fun bodyStyle(): TextStyle = LognFont.sans(LeaderboardMetrics.BODY_SIZE).copy(lineHeight = LeaderboardMetrics.BODY_LINE.sp)

/** Sem XP: a lista, e embaixo o convite para o primeiro desafio. */
@Composable
private fun ZeroXpFooter(
    board: LeaderboardView,
    onGoToTrail: () -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    Column(
        Modifier
            .fillMaxWidth()
            .background(LognDark.surface)
            .drawBehind { drawRect(LognDark.line, Offset.Zero, size.copy(height = LognStroke.hairline.toPx())) }
            .padding(start = Space.screenMargin, end = Space.screenMargin, top = Space.lg, bottom = LeaderboardMetrics.zeroBottom),
        verticalArrangement = Arrangement.spacedBy(LeaderboardMetrics.zeroGap),
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(Space.sm + Space.xxs), verticalAlignment = Alignment.CenterVertically) {
            Text(
                leaderboardName(context, board.me.anonNumber, board.me.nickname),
                style = nameStyle(board.me.nickname, FontWeight.SemiBold),
                color = LognDark.textPrimary,
            )
            Text(
                Str.Leaderboard.you(context) + " · " + Str.Leaderboard.xp(context, "0"),
                style = LognFont.mono(LeaderboardMetrics.LABEL_SIZE, tracking = LeaderboardMetrics.LABEL_TRACKING),
                color = LognDark.accentInk,
            )
        }
        Text(Str.Leaderboard.zero_xp(context), style = bodyStyle(), color = LognDark.textSecondary)
        OutlineButton(Str.Leaderboard.zero_xp_action(context), LognDark.accent, LognDark.accentInk, FontWeight.SemiBold, LeaderboardMetrics.smallButton) {
            dispatch(Event.SelectTrack(""))
            onGoToTrail()
        }
    }
}

/** Visitante: a chamada para criar conta. */
@Composable
private fun GuestState() {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    CenteredState {
        Box(
            Modifier
                .size(LeaderboardMetrics.guestAvatar)
                .background(LognDark.surface, CircleShape)
                .drawBehind {
                    val dash = LeaderboardMetrics.guestDash.toPx()
                    drawCircle(
                        LognDark.lineDim,
                        style = Stroke(LognStroke.hairline.toPx(), pathEffect = PathEffect.dashPathEffect(floatArrayOf(dash, dash))),
                    )
                },
            contentAlignment = Alignment.Center,
        ) { Icon(LognIcon.Person, LognDark.textMuted, LeaderboardMetrics.guestIcon) }
        StateText(Str.Leaderboard.guest_title(context), Str.Leaderboard.guest_body(context))
        Column(verticalArrangement = Arrangement.spacedBy(LeaderboardMetrics.buttonGap)) {
            FilledButton(Str.Leaderboard.guest_create(context)) {
                ShellState.wantsRegistration = true
                dispatch(Event.Logout)
            }
            OutlineButton(Str.Leaderboard.guest_sign_in(context), LognDark.lineStrong, LognDark.textPrimary, FontWeight.Medium, LeaderboardMetrics.button) {
                dispatch(Event.Logout)
            }
        }
    }
}

/** Sem rede e nada guardado. */
@Composable
private fun UnavailableState() {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    CenteredState {
        Icon(LognIcon.WifiSlash, LognDark.textMuted, LeaderboardMetrics.stateIcon)
        StateText(Str.Leaderboard.unavailable_title(context), Str.Leaderboard.unavailable_body(context))
        OutlineButton(Str.Dashboard.try_again(context), LognDark.lineStrong, LognDark.textPrimary, FontWeight.Medium, LeaderboardMetrics.smallButton) {
            dispatch(Event.LeaderboardRefresh)
        }
    }
}

/** Carregando: linhas vazias no formato da lista, sem brilho correndo. */
@Composable
private fun Skeleton() {
    val context = LocalContext.current
    val label = Str.Leaderboard.loading_accessibility(context)
    Column(Modifier.fillMaxWidth().clearAndSetSemantics { contentDescription = label }) {
        for (width in LeaderboardMetrics.skeletonWidths) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = LeaderboardMetrics.rowMinHeight)
                    .bottomLine(LognDark.rowLine)
                    .padding(horizontal = Space.screenMargin, vertical = LeaderboardMetrics.rowPaddingV),
                horizontalArrangement = Arrangement.spacedBy(LeaderboardMetrics.rowGap),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(Modifier.size(LeaderboardMetrics.skeletonRank, LeaderboardMetrics.skeletonRankHeight).background(LognDark.rowLine, RoundedCornerShape(Radius.xs)))
                Box(Modifier.size(width, LeaderboardMetrics.skeletonText).background(LognDark.surface, RoundedCornerShape(Radius.xs)))
                Box(Modifier.weight(1f))
                Box(Modifier.size(LeaderboardMetrics.skeletonXp, LeaderboardMetrics.skeletonText).background(LognDark.surface, RoundedCornerShape(Radius.xs)))
            }
        }
    }
}

@Composable
private fun CenteredState(content: @Composable () -> Unit) {
    Column(
        Modifier
            .fillMaxSize()
            .padding(horizontal = LeaderboardMetrics.emptyPaddingH),
        verticalArrangement = Arrangement.spacedBy(LeaderboardMetrics.stateGap, Alignment.CenterVertically),
    ) { content() }
}

@Composable
private fun StateText(
    title: String,
    body: String,
) {
    Column(verticalArrangement = Arrangement.spacedBy(LeaderboardMetrics.textGap)) {
        Text(title, style = LognFont.sans(LeaderboardMetrics.STATE_TITLE_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
        Text(body, style = bodyStyle(), color = LognDark.textSecondary)
    }
}

@Composable
private fun FilledButton(
    title: String,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(Radius.sm)
    Box(
        Modifier
            .fillMaxWidth()
            .height(LeaderboardMetrics.button)
            .background(LognDark.accent, shape)
            .clickable(role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Text(title, style = LognFont.sans(LeaderboardMetrics.BUTTON_SIZE, FontWeight.SemiBold), color = LognDark.onAccent)
    }
}

@Composable
private fun OutlineButton(
    title: String,
    border: Color,
    ink: Color,
    weight: FontWeight,
    height: Dp,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(Radius.sm)
    val size = if (height == LeaderboardMetrics.button) LeaderboardMetrics.BUTTON_SIZE else LeaderboardMetrics.SMALL_BUTTON_SIZE
    Box(
        Modifier
            .fillMaxWidth()
            .height(height)
            .border(LognStroke.hairline, border, shape)
            .clickable(role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Text(title, style = LognFont.sans(size, weight), color = ink)
    }
}
