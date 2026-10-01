package sh.logn.app.ui.store

import androidx.activity.compose.BackHandler
import androidx.activity.compose.LocalActivity
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
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import sh.logn.app.core.StorePrice
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.LocalStore
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.HomeMetrics
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.StoreMetrics
import sh.logn.app.ui.theme.Stroke
import sh.logn.app.ui.track.DiscountLine
import sh.logn.app.ui.track.OfflineDaysBar
import sh.logn.app.ui.track.TrackBalloon
import sh.logn.app.ui.track.TrackChip
import sh.logn.app.ui.track.TrackChipStyle
import sh.logn.app.ui.track.TrackDates
import sh.logn.app.ui.track.activeNodeIndex
import sh.logn.app.ui.track.isExpired
import sh.logn.app.ui.track.languagesLabel
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.OfflineState
import sh.logn.core.LogN.TrackView
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str
import java.util.Locale

/**
 * O catálogo em grade, um balão por trilha. Espelho de CatalogView.swift: a principal no
 * topo, as pagas em duas colunas. `openTrack` leva a árvore à trilha e fecha o catálogo.
 */
@Composable
fun CatalogScreen(
    view: ViewModel,
    onBack: () -> Unit,
    onTrack: (String) -> Unit,
    openTrack: (String) -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val store = LocalStore.current
    val free = view.tracks.firstOrNull { it.isFree }
    val paid = view.tracks.filter { !it.isFree }
    LaunchedEffect(Unit) { dispatch(Event.CatalogOpened) }
    LaunchedEffect(paid.map { it.productId }) { store.loadPrices(paid.map { it.productId }) }
    BackHandler(onBack = onBack)
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.surface)
            .statusBarsPadding(),
    ) {
        BackBar(onBack)
        Column(
            Modifier
                .verticalScroll(rememberScrollState())
                .navigationBarsPadding()
                .padding(start = Space.lg, end = Space.lg, bottom = Space.xl),
            verticalArrangement = Arrangement.spacedBy(Space.lg),
        ) {
            Row(Modifier.padding(top = Space.sm), verticalAlignment = Alignment.CenterVertically) {
                Text(Str.Catalog.title(context), style = LognFont.headlineMedium, color = LognDark.textPrimary, modifier = Modifier.weight(1f))
                if (view.isOfflineSession) {
                    Text(
                        Str.Catalog.offline(context),
                        style = LognFont.mono(StoreMetrics.SMALL_SIZE, tracking = StoreMetrics.SMALL_TRACKING),
                        color = LognDark.textMuted,
                    )
                }
            }
            if (free != null) PrincipalCard(free) { openTrack(free.id) }
            for (pair in paid.chunked(2)) {
                Row(Modifier.height(IntrinsicSize.Min), horizontalArrangement = Arrangement.spacedBy(StoreMetrics.gridGap)) {
                    for (track in pair) TrackCard(track, store.prices[track.productId], Modifier.weight(1f)) { onTrack(track.id) }
                    if (pair.size == 1) Spacer(Modifier.weight(1f))
                }
            }
        }
    }
}

/** Voltar, no topo das telas empilhadas da trilha. */
@Composable
fun BackBar(onBack: () -> Unit) {
    val context = LocalContext.current
    val label = Str.Logout.back(context)
    Box(
        Modifier
            .padding(start = Space.xs)
            .clickable(role = Role.Button, onClick = onBack)
            .clearAndSetSemantics { contentDescription = label }
            .padding(Space.md),
    ) { Icon(LognIcon.ChevronLeft, LognDark.textSecondary, IconMetrics.md) }
}

@Composable
private fun PrincipalCard(
    track: TrackView,
    onClick: () -> Unit,
) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        Modifier
            .fillMaxWidth()
            .background(if (track.selected) LognDark.accentTint else LognDark.canvas, shape)
            .border(Stroke.hairline, if (track.selected) LognDark.accent else LognDark.line, shape)
            .clickable(role = Role.Button, onClick = onClick)
            .padding(StoreMetrics.cardPadding),
        horizontalArrangement = Arrangement.spacedBy(Space.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TrackBalloon(track, StoreMetrics.principalBalloon)
        Column(verticalArrangement = Arrangement.spacedBy(StoreMetrics.gap3)) {
            Text(
                Str.Catalog.principal(context),
                style = LognFont.mono(StoreMetrics.SMALL_SIZE, tracking = StoreMetrics.EYEBROW_TRACKING),
                color = LognDark.accentInk,
            )
            Text(track.name, style = LognFont.sans(StoreMetrics.PRINCIPAL_NAME_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
            Text(
                Str.Catalog.progress(context, track.nodesDone.toInt(), track.nodeCount.toInt(), track.trackXp),
                style = LognFont.mono(StoreMetrics.META_SIZE),
                color = LognDark.textSecondary,
            )
        }
    }
}

@Composable
private fun TrackCard(
    track: TrackView,
    price: StorePrice?,
    modifier: Modifier,
    onClick: () -> Unit,
) {
    val context = LocalContext.current
    val chip = TrackChipStyle.of(context, track, price)
    val warn = track.owned && (track.offline == OfflineState.SOON || track.offline == OfflineState.TODAY)
    val dimmed = track.isExpired || track.revoked
    val label = Str.Catalog.open_accessibility(context, track.name, chip.spoken)
    val shape = RoundedCornerShape(Radius.sm)
    Column(
        modifier
            .fillMaxHeight()
            .heightIn(min = StoreMetrics.cardMinHeight)
            .background(LognDark.canvas, shape)
            .border(Stroke.hairline, if (warn) LognDark.warn else LognDark.line, shape)
            .clearAndSetSemantics { contentDescription = label }
            .clickable(role = Role.Button, onClick = onClick)
            .padding(Space.md),
        verticalArrangement = Arrangement.spacedBy(Space.sm),
    ) {
        Row(verticalAlignment = Alignment.Top) {
            TrackBalloon(track, StoreMetrics.cardBalloon, dimmed = dimmed)
            Spacer(Modifier.weight(1f))
            // Na comprada é progresso ("5/9"), não contagem.
            Text(
                if (track.owned) "${track.nodesDone}/${track.nodeCount}" else Str.Catalog.nodes(context, track.nodeCount.toInt()),
                style = LognFont.mono(StoreMetrics.SMALL_SIZE),
                color = LognDark.textMuted,
            )
        }
        Text(
            track.name,
            style = LognFont.sans(StoreMetrics.CARD_NAME_SIZE, FontWeight.SemiBold),
            color = LognDark.textPrimary,
            modifier = Modifier.alpha(if (dimmed) StoreMetrics.DIMMED_ALPHA else 1f),
        )
        Spacer(Modifier.weight(1f))
        TrackChip(chip)
    }
}

/**
 * A página da trilha: ementa, nós e preço antes de entrar, e o aviso de validade offline
 * de quem comprou. Espelho de TrackDetailView.swift.
 */
@Composable
fun TrackDetailScreen(
    view: ViewModel,
    trackId: String,
    onBack: () -> Unit,
    openTrack: (String) -> Unit,
) {
    val track = view.tracks.firstOrNull { it.id == trackId } ?: return
    val store = LocalStore.current
    LaunchedEffect(track.productId) { store.loadPrices(listOf(track.productId)) }
    BackHandler(onBack = onBack)
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.surface)
            .statusBarsPadding(),
    ) {
        BackBar(onBack)
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(start = StoreMetrics.side, end = StoreMetrics.side, bottom = Space.xl),
            verticalArrangement = Arrangement.spacedBy(StoreMetrics.cardPadding),
        ) {
            Heading(track)
            if (!track.owned && track.description.isNotEmpty()) {
                Text(
                    track.description,
                    style = LognFont.sans(StoreMetrics.DESC_SIZE).merge(TextStyle(lineHeight = StoreMetrics.DESC_LINE.sp)),
                    color = LognDark.textSecondary,
                )
            }
            Notices(track)
            NodeList(track)
        }
        Footer(track, store.prices[track.productId], openTrack)
    }
}

@Composable
private fun Heading(track: TrackView) {
    val context = LocalContext.current
    Row(
        Modifier.padding(top = Space.sm),
        horizontalArrangement = Arrangement.spacedBy(StoreMetrics.cardPadding),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TrackBalloon(track, HomeMetrics.expiredBalloon, dimmed = track.revoked)
        Column(verticalArrangement = Arrangement.spacedBy(StoreMetrics.gap3)) {
            Text(
                track.name,
                style = LognFont.sans(StoreMetrics.TITLE_SIZE, FontWeight.SemiBold, StoreMetrics.TITLE_TRACKING),
                color = LognDark.textPrimary,
            )
            Text(
                if (track.owned) {
                    Str.Track.owned_meta(context, track.nodesDone.toInt(), track.nodeCount.toInt())
                } else {
                    Str.Track.meta(context, track.nodeCount.toInt(), track.author.uppercase(), track.languagesLabel)
                },
                style = LognFont.mono(StoreMetrics.META_SIZE),
                color = LognDark.textMuted,
            )
        }
    }
}

@Composable
private fun Notices(track: TrackView) {
    val context = LocalContext.current
    val locale = appLocale(context)
    when {
        track.revoked ->
            Note(
                if (track.revokedReason == "refund") {
                    Str.Track.revoked_refund_play(context, track.name, track.trackXp)
                } else {
                    Str.Track.revoked_other(context, track.name, track.trackXp)
                },
                LognDark.surfaceRaised,
                LognDark.lineStrong,
            )
        track.owned && (track.offline == OfflineState.SOON || track.offline == OfflineState.TODAY) -> OfflineBlock(track)
        track.owned && track.discontinued -> Note(Str.Track.discontinued_note(context), LognDark.surfaceRaised, LognDark.lineStrong)
        track.owned && track.validUntil > 0 ->
            Text(
                Str.Track.valid_until(context, TrackDates.short(track.validUntil, locale)),
                style = LognFont.mono(StoreMetrics.META_SIZE),
                color = LognDark.textMuted,
            )
    }
}

@Composable
private fun Note(
    text: String,
    background: Color,
    line: Color,
) {
    val shape = RoundedCornerShape(Radius.sm)
    Text(
        text,
        style = LognFont.sans(StoreMetrics.NOTE_SIZE).merge(TextStyle(lineHeight = StoreMetrics.NOTE_LINE.sp)),
        color = LognDark.textPrimary,
        modifier =
            Modifier
                .fillMaxWidth()
                .background(background, shape)
                .border(Stroke.hairline, line, shape)
                .padding(StoreMetrics.cardPadding),
    )
}

/** O aviso se explica por inteiro: prazo, os dias desenhados, o que acontece se vencer. */
@Composable
private fun OfflineBlock(track: TrackView) {
    val context = LocalContext.current
    val locale = appLocale(context)
    val lastDay = track.offline == OfflineState.TODAY
    val shape = RoundedCornerShape(Radius.sm)
    Column(
        Modifier
            .fillMaxWidth()
            .background(LognDark.tintWarn, shape)
            .border(Stroke.hairline, LognDark.warn, shape)
            .padding(StoreMetrics.cardPadding),
        verticalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        Row(Modifier.fillMaxWidth()) {
            val style = LognFont.mono(StoreMetrics.META_SIZE, tracking = StoreMetrics.SMALL_TRACKING)
            Text(
                if (lastDay) Str.Track.offline_last_day_tag(context) else Str.Track.offline_days_tag(context, track.daysSinceContact.toInt()),
                style = style,
                color = LognDark.warnInk,
                modifier = Modifier.weight(1f),
            )
            Text(Str.Track.valid_until(context, TrackDates.short(track.validUntil, locale)), style = style, color = LognDark.warnInk)
        }
        Text(
            if (lastDay) {
                Str.Track.offline_today_body(context, track.name)
            } else {
                Str.Track.offline_soon_body(context, TrackDates.weekday(track.validUntil, locale), track.name)
            },
            style = LognFont.sans(StoreMetrics.BODY_SIZE),
            color = LognDark.textPrimary,
        )
        OfflineDaysBar(track.daysSinceContact.toInt().coerceAtMost(OFFLINE_DAYS))
        Text(Str.Track.offline_rule(context), style = LognFont.sans(StoreMetrics.RULE_SIZE), color = LognDark.textSecondary)
    }
}

@Composable
private fun NodeList(track: TrackView) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(Radius.sm)
    Column(
        Modifier
            .fillMaxWidth()
            .border(Stroke.hairline, LognDark.line, shape),
    ) {
        for ((index, row) in track.nodes.withIndex()) {
            val sample = !track.owned && row.free
            Row(
                Modifier
                    .fillMaxWidth()
                    .background(if (sample) LognDark.tintOk else Color.Transparent)
                    .drawBehind {
                        if (index > 0) drawLine(LognDark.line, Offset(0f, 0f), Offset(size.width, 0f), Stroke.hairline.toPx())
                    }.padding(horizontal = Space.md, vertical = StoreMetrics.rowPaddingV),
                horizontalArrangement = Arrangement.spacedBy(Space.md),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    String.format(Locale.ROOT, "%02d", index + 1),
                    style = LognFont.mono(StoreMetrics.META_SIZE),
                    color = if (sample) LognDark.correctInk else LognDark.textMuted,
                    modifier = Modifier.width(StoreMetrics.indexWidth),
                )
                Text(
                    row.name,
                    style = LognFont.sans(StoreMetrics.NOTE_SIZE),
                    color = if (sample || row.active) LognDark.textPrimary else LognDark.textSecondary,
                    modifier = Modifier.weight(1f),
                )
                when {
                    sample ->
                        Text(
                            Str.Track.free_tag(context),
                            style = LognFont.mono(StoreMetrics.SMALL_SIZE, tracking = StoreMetrics.FREE_TRACKING),
                            color = LognDark.correctInk,
                        )
                    track.owned && row.done -> Icon(LognIcon.Check, LognDark.correctInk, IconMetrics.sm)
                    track.owned && row.active -> Icon(LognIcon.ArrowRight, LognDark.accentInk, IconMetrics.sm)
                }
            }
        }
    }
}

@Composable
private fun Footer(
    track: TrackView,
    price: StorePrice?,
    openTrack: (String) -> Unit,
) {
    val context = LocalContext.current
    val store = LocalStore.current
    val activity = LocalActivity.current
    Column(
        Modifier
            .fillMaxWidth()
            .drawBehind { drawLine(LognDark.line, Offset(0f, 0f), Offset(size.width, 0f), Stroke.hairline.toPx()) }
            .navigationBarsPadding()
            .padding(start = StoreMetrics.side, end = StoreMetrics.side, top = Space.md, bottom = StoreMetrics.side),
        verticalArrangement = Arrangement.spacedBy(Space.sm),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        store.problem?.let { Text(sh.logn.app.core.PlayStore.copy(context, it), style = LognFont.bodyMedium, color = LognDark.textSecondary) }
        // Só onde o rodapé vende: quem tem a trilha vê "Continuar".
        if (price != null && (track.revoked || !track.owned)) DiscountLine(price)
        when {
            track.revoked ->
                LognButton(
                    price?.let { Str.Track.buy_again(context, it.formatted) } ?: Str.Track.buy_again_no_price(context),
                    ButtonVariant.Primary,
                    loading = store.isPurchasing,
                ) { activity?.let { store.buy(it, track.productId) } }
            track.owned ->
                LognButton(
                    track.activeNodeIndex?.let { Str.Track.continue_node(context, it) } ?: Str.Track.continue_track(context),
                    ButtonVariant.Primary,
                ) { openTrack(track.id) }
            else -> {
                Row(horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
                    LognButton(Str.Track.play_sample(context), ButtonVariant.Secondary, Modifier.weight(1f)) { openTrack(track.id) }
                    LognButton(
                        price?.let { Str.Track.buy(context, it.formatted) } ?: Str.Track.buy_no_price(context),
                        ButtonVariant.Primary,
                        Modifier.weight(1f),
                        enabled = track.productId.isNotEmpty(),
                        loading = store.isPurchasing,
                    ) { activity?.let { store.buy(it, track.productId) } }
                }
                Text(Str.Track.buy_note(context), style = LognFont.mono(StoreMetrics.SMALL_SIZE), color = LognDark.textMuted)
            }
        }
    }
}

/** A língua do app, para as datas da trilha. */
fun appLocale(context: android.content.Context): Locale = Locale.forLanguageTag(sh.logn.app.AppLocale.current(context))

private const val OFFLINE_DAYS = 30
