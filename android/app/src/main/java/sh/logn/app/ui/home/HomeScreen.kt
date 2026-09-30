package sh.logn.app.ui.home

import androidx.activity.compose.BackHandler
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.tween
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
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.HomeMetrics
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LocalReduceMotion
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.app.ui.track.TrackBalloon
import sh.logn.app.ui.track.isExpired
import sh.logn.app.ui.tree.NodeSheet
import sh.logn.app.ui.tree.SkillTree
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.NodeStatus
import sh.logn.core.LogN.OfflineState
import sh.logn.core.LogN.SkillNode
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str
import java.time.OffsetDateTime
import java.time.format.DateTimeFormatter
import java.util.Locale

enum class LognTab { Trails, Arena, Standings }

/**
 * O que está por cima da aba: a partida toma a tela inteira e a barra sai junto, como no
 * documento de gameplay. É o `NavigationStack` de cada aba no iOS.
 */
sealed interface HomeRoute {
    data class Match(
        val nodeId: String,
    ) : HomeRoute

    data object Catalog : HomeRoute

    data object Scoreboard : HomeRoute
}

/** Os pontos de extensão das fases seguintes; vazios até lá. */
data class HomeSlots(
    val route: @Composable (HomeRoute, onBack: () -> Unit) -> Unit = { _, _ -> },
    val profile: @Composable (onDismiss: () -> Unit) -> Unit = {},
    val standings: @Composable (openScoreboard: () -> Unit) -> Unit = {},
    val lockedNode: @Composable (SkillNode, onDismiss: () -> Unit) -> Unit = { _, _ -> },
)

/**
 * O app depois da entrada. Espelho de ContentView.swift: o onboarding é a tela (não uma
 * capa por cima), e a navegação mora dentro de cada aba.
 */
@Composable
fun HomeScreen(
    view: ViewModel,
    slots: HomeSlots = HomeSlots(),
) {
    val dispatch = LocalDispatch.current
    val reduceMotion = LocalReduceMotion.current
    var tab by rememberSaveable { mutableStateOf(LognTab.Trails) }
    var route by rememberSaveable(stateSaver = homeRouteSaver) { mutableStateOf<HomeRoute?>(null) }

    // A árvore assim que a sessão existe; a da semente também conta como "falta buscar",
    // senão a conta logada nunca falava com o servidor e o catálogo não chegava.
    LaunchedEffect(view.hasAccessToken) {
        if (view.hasAccessToken && (view.nodes.isEmpty() || view.trailFromBundle) && !view.isFetching) {
            dispatch(Event.FetchNodes)
        }
    }

    Crossfade(view.showOnboarding, animationSpec = tween(if (reduceMotion) 0 else FADE_MILLIS), label = "onboarding") { onboarding ->
        if (onboarding) {
            OnboardingScreen(view) { wantsCatalog ->
                tab = LognTab.Trails
                if (wantsCatalog) route = HomeRoute.Catalog
            }
            return@Crossfade
        }
        val current = route
        if (current != null) {
            BackHandler { route = null }
            slots.route(current) { route = null }
            return@Crossfade
        }
        Column(
            Modifier
                .fillMaxSize()
                .background(LognDark.canvas),
        ) {
            Box(
                Modifier
                    .weight(1f)
                    .fillMaxWidth()
                    .statusBarsPadding(),
            ) {
                when (tab) {
                    LognTab.Trails -> TrailsTab(view, slots, onRoute = { route = it })
                    LognTab.Arena -> ArenaTab()
                    LognTab.Standings -> slots.standings { route = HomeRoute.Scoreboard }
                }
            }
            BottomNav(tab, onSelect = { tab = it })
        }
    }
}

@Composable
private fun TrailsTab(
    view: ViewModel,
    slots: HomeSlots,
    onRoute: (HomeRoute) -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    var selectedId by rememberSaveable { mutableStateOf<String?>(null) }
    var paywallId by rememberSaveable { mutableStateOf<String?>(null) }
    var showsProfile by rememberSaveable { mutableStateOf(false) }
    val track = view.currentTrack

    Column(Modifier.fillMaxSize()) {
        TrailsHeader(view, onCatalog = { onRoute(HomeRoute.Catalog) }, onProfile = { showsProfile = true })
        if (view.isGuest) {
            WarnBar(LognIcon.ExclamationTriangleFill, Str.Dashboard.sync_guest_warning(context))
        } else if (view.isOfflineSession) {
            WarnBar(LognIcon.WifiSlash, Str.Dashboard.offline_session(context))
        }
        // A tarja acima diz que o progresso não sobe; esta, que o conteúdo pode estar velho.
        // As duas podem valer juntas. Esta é a mais quieta: não é aviso, é ressalva.
        if (view.trailFromBundle) BundledTrailNote(view.trailGeneratedAt)
        when {
            // Só a trilha vencida fecha, numa tela própria; a grátis e as amostras seguem.
            track.isExpired && !track.isFree -> ExpiredTrackScreen(view, track)
            view.nodes.isEmpty() -> EmptyState(view.isFetching) { dispatch(Event.FetchNodes) }
            else ->
                SkillTree(view.nodes, onNode = { node ->
                    if (node.status == NodeStatus.PAYWALLLOCKED) paywallId = node.id else selectedId = node.id
                })
        }
    }

    view.nodes.firstOrNull { it.id == selectedId }?.let { node ->
        NodeSheet(
            node = node,
            incoming = view.nodes.filter { it.id in node.prerequisites },
            unlocks = view.nodes.filter { node.id in it.prerequisites },
            // Quantos problemas o nó tem: sem isto, a folha prometia partida em nó vazio.
            problemCount = view.challenges.count { it.nodeId == node.id },
            onDismiss = { selectedId = null },
            onStartMatch = { onRoute(HomeRoute.Match(node.id)) },
        )
    }
    view.nodes.firstOrNull { it.id == paywallId }?.let { node -> slots.lockedNode(node) { paywallId = null } }
    if (showsProfile) slots.profile { showsProfile = false }
}

/**
 * A trilha na tela e o botão que abre o catálogo, com o selo de validade. Sem catálogo
 * ainda (primeira abertura sem rede), o contador de balões.
 */
@Composable
private fun TrailsHeader(
    view: ViewModel,
    onCatalog: () -> Unit,
    onProfile: () -> Unit,
) {
    val context = LocalContext.current
    val track = view.currentTrack
    Column {
        Row(
            Modifier
                .fillMaxWidth()
                .padding(start = Space.screenMargin, end = Space.screenMargin, top = HomeMetrics.headerTop, bottom = HomeMetrics.headerBottom),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(Space.md),
        ) {
            Box(Modifier.weight(1f)) {
                if (track.name.isEmpty()) {
                    // "N balões no ar": o contador é de nós conquistados, não de problemas.
                    val balloons = view.nodes.count { it.status == NodeStatus.COMPLETED }
                    Text(
                        if (view.isFetching) Str.Dashboard.map_updating(context) else Str.Dashboard.balloons_up(context, balloons),
                        style = LognFont.sans(HomeMetrics.HEADER_SIZE, FontWeight.SemiBold, HomeMetrics.HEADER_TRACKING),
                        color = LognDark.textPrimary,
                    )
                } else {
                    TrackSwitcher(view, onCatalog)
                }
            }
            val label = Str.Profile.guest_mode(context)
            // O círculo tem 40; o alvo cresce para os 44 mínimos, invisível.
            Box(
                Modifier
                    .size(IconMetrics.touch)
                    .semantics { contentDescription = label }
                    .clickable(role = Role.Button, onClick = onProfile),
                contentAlignment = Alignment.Center,
            ) {
                ProfileAvatar(
                    initial = if (view.isGuest) "?" else view.accountEmail,
                    hasPending = view.pendingSyncCount > 0u,
                )
            }
        }
        Divider()
    }
}

/** O nome da trilha é o próprio seletor; o selo de validade fica colado ao chevron. */
@Composable
private fun TrackSwitcher(
    view: ViewModel,
    onCatalog: () -> Unit,
) {
    val context = LocalContext.current
    val track = view.currentTrack
    val badge = view.catalogBadge
    val badgeText =
        when (badge.state) {
            OfflineState.SOON -> Str.Catalog.days(context, badge.daysLeft.toInt())
            OfflineState.TODAY -> Str.Catalog.today(context)
            OfflineState.EXPIRED -> Str.Catalog.connect(context)
            else -> null
        }
    val progress = Str.Catalog.progress(context, track.nodesDone.toInt(), track.nodeCount.toInt(), track.trackXp)
    val meta = if (view.isOfflineSession) progress + " · " + Str.Catalog.offline(context) else progress
    val label = listOfNotNull(Str.Catalog.button(context), track.name, badgeText).joinToString(", ")
    Row(
        Modifier
            .heightIn(min = IconMetrics.touch)
            .clearAndSetSemantics { contentDescription = label }
            .clickable(role = Role.Button, onClick = onCatalog),
        horizontalArrangement = Arrangement.spacedBy(Space.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        TrackBalloon(track, HomeMetrics.trackBalloon)
        Column(verticalArrangement = Arrangement.spacedBy(Space.xxs)) {
            Row(horizontalArrangement = Arrangement.spacedBy(Space.sm), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    track.name,
                    style = LognFont.sans(HomeMetrics.TRACK_NAME_SIZE, FontWeight.SemiBold),
                    color = LognDark.textPrimary,
                    maxLines = 1,
                    overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f, fill = false),
                )
                Icon(LognIcon.ChevronDown, LognDark.textSecondary, IconMetrics.sm)
                if (badgeText != null) {
                    Text(
                        badgeText,
                        style = LognFont.mono(HomeMetrics.BADGE_SIZE, FontWeight.Medium, HomeMetrics.BADGE_TRACKING),
                        color = LognDark.canvas,
                        maxLines = 1,
                        modifier =
                            Modifier
                                .background(if (badge.state == OfflineState.EXPIRED) LognDark.wrong else LognDark.warn, RoundedCornerShape(Radius.xs))
                                .padding(horizontal = HomeMetrics.badgePaddingH, vertical = HomeMetrics.badgePaddingV),
                    )
                }
            }
            Text(meta, style = LognFont.mono(HomeMetrics.META_SIZE), color = LognDark.textMuted, maxLines = 1)
        }
    }
}

/**
 * A entrada do perfil: círculo com a inicial em `accentInk` sobre acento a 18%, e o selo
 * `warn` quando há evento na fila.
 */
@Composable
fun ProfileAvatar(
    initial: String,
    hasPending: Boolean,
    modifier: Modifier = Modifier,
    size: androidx.compose.ui.unit.Dp = HomeMetrics.avatar,
) {
    val letter = initial.firstOrNull()?.uppercaseChar()?.toString() ?: "?"
    Box(modifier.size(size)) {
        Box(
            Modifier
                .size(size)
                .background(LognDark.accent.copy(alpha = HomeMetrics.AVATAR_FILL_ALPHA), CircleShape)
                .border(Stroke.hairline, LognDark.accent, CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            Text(letter, style = LognFont.sans(size.value * HomeMetrics.AVATAR_TEXT_RATIO, FontWeight.SemiBold), color = LognDark.accentInk)
        }
        if (hasPending) {
            Box(
                Modifier
                    .align(Alignment.TopEnd)
                    .offset(x = Stroke.hairline, y = -Stroke.hairline)
                    .size(HomeMetrics.avatarBadge)
                    .background(LognDark.canvas, CircleShape)
                    .padding(HomeMetrics.avatarBadgeStroke)
                    .background(LognDark.warn, CircleShape),
            )
        }
    }
}

/** A tarja `warn`: visitante (não sobe) ou sessão sem servidor (sobe depois). */
@Composable
private fun WarnBar(
    icon: LognIcon,
    text: String,
) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(LognDark.warn)
            .padding(vertical = Space.sm),
        horizontalArrangement = Arrangement.spacedBy(Space.sm, Alignment.CenterHorizontally),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, LognDark.onAccent, HomeMetrics.warnIcon, knockout = LognDark.warn)
        Text(text, style = LognFont.label, color = LognDark.onAccent)
    }
}

/** A trilha na tela é a que veio dentro do app, congelada no dia do build. */
@Composable
private fun BundledTrailNote(generatedAt: String) {
    val context = LocalContext.current
    val date =
        runCatching {
            val locale = Locale.forLanguageTag(sh.logn.app.AppLocale.current(context))
            val pattern = android.text.format.DateFormat.getBestDateTimePattern(locale, "d MMMM")
            OffsetDateTime.parse(generatedAt).format(DateTimeFormatter.ofPattern(pattern, locale))
        }.getOrDefault(generatedAt)
    Column {
        Row(
            Modifier
                .fillMaxWidth()
                .background(LognDark.surface)
                .padding(horizontal = Space.lg, vertical = Space.sm),
            horizontalArrangement = Arrangement.spacedBy(Space.sm, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.Top,
        ) {
            Icon(LognIcon.Box, LognDark.textMuted, HomeMetrics.warnIcon, Modifier.padding(top = Stroke.hairline))
            Text(Str.Status.trail_from_bundle(context, date), style = LognFont.label, color = LognDark.textMuted, textAlign = TextAlign.Center)
        }
        Divider()
    }
}

/** Carregando é esqueleto, não spinner; vazio é uma frase e o CTA que resolve. */
@Composable
private fun EmptyState(
    isFetching: Boolean,
    onRetry: () -> Unit,
) {
    val context = LocalContext.current
    Column(
        Modifier
            .fillMaxSize()
            .padding(horizontal = Space.screenMargin),
        verticalArrangement = Arrangement.spacedBy(Space.lg, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        if (isFetching) {
            repeat(SKELETON_ROWS) {
                Box(
                    Modifier
                        .fillMaxWidth()
                        .height(HomeMetrics.skeletonRow)
                        .background(LognDark.surface, RoundedCornerShape(Radius.sm))
                        .border(Stroke.hairline, LognDark.line, RoundedCornerShape(Radius.sm)),
                )
            }
        } else {
            Text(Str.Dashboard.map_failed(context), style = LognFont.bodyMedium, color = LognDark.textSecondary, textAlign = TextAlign.Center)
            LognButton(Str.Dashboard.try_again(context), ButtonVariant.Secondary, onClick = onRetry)
        }
    }
}

@Composable
private fun ArenaTab() {
    val context = LocalContext.current
    Column(
        Modifier
            .fillMaxSize()
            .padding(horizontal = Space.screenMargin),
        verticalArrangement = Arrangement.spacedBy(Space.md, Alignment.CenterVertically),
    ) {
        Text(Str.Tabs.arena(context), style = LognFont.label, color = LognDark.textMuted)
        Text(
            Str.Dashboard.arena_desc(context),
            style = LognFont.sans(HomeMetrics.ARENA_TITLE_SIZE, FontWeight.SemiBold, HomeMetrics.HEADER_TRACKING),
            color = LognDark.textPrimary,
        )
        Text(Str.Dashboard.arena_sub(context), style = LognFont.bodyMedium, color = LognDark.textSecondary)
    }
}

/**
 * A barra de baixo, desenhada à mão como no iOS: rótulos em mono sobre `surface`,
 * divisor em `line`, e o item ativo em acento com uma borda de 2 que cobre o divisor.
 */
@Composable
fun BottomNav(
    selection: LognTab,
    onSelect: (LognTab) -> Unit,
) {
    val context = LocalContext.current
    Row(
        Modifier
            .fillMaxWidth()
            .background(LognDark.surface)
            .drawBehind {
                drawLine(LognDark.line, Offset(0f, 0f), Offset(size.width, 0f), Stroke.hairline.toPx())
            }.navigationBarsPadding(),
    ) {
        for (item in LognTab.entries) {
            val active = item == selection
            val title =
                when (item) {
                    LognTab.Trails -> Str.Tabs.trails(context)
                    LognTab.Arena -> Str.Tabs.arena(context)
                    LognTab.Standings -> Str.Tabs.standings(context)
                }
            Box(
                Modifier
                    .weight(1f)
                    .semantics { selected = active }
                    .clickable(role = Role.Tab) { onSelect(item) }
                    .drawBehind {
                        if (active) {
                            val h = HomeMetrics.navIndicator.toPx()
                            drawRect(LognDark.accent, Offset(0f, -h / 2), androidx.compose.ui.geometry.Size(size.width, h))
                        }
                    }.padding(vertical = HomeMetrics.navPaddingV),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    title,
                    style = LognFont.mono(HomeMetrics.NAV_SIZE, tracking = HomeMetrics.NAV_TRACKING),
                    color = if (active) LognDark.accentInk else LognDark.textMuted,
                )
            }
        }
    }
}

@Composable
fun Divider(modifier: Modifier = Modifier) {
    Box(
        modifier
            .fillMaxWidth()
            .height(Stroke.hairline)
            .background(LognDark.line),
    )
}

/** A rota sobrevive à recriação da Activity como texto: "match:<id>", "catalog", "scoreboard". */
private val homeRouteSaver =
    androidx.compose.runtime.saveable.Saver<HomeRoute?, String>(
        save = { route ->
            when (route) {
                is HomeRoute.Match -> "match:${route.nodeId}"
                HomeRoute.Catalog -> "catalog"
                HomeRoute.Scoreboard -> "scoreboard"
                null -> ""
            }
        },
        restore = { saved ->
            when {
                saved.startsWith("match:") -> HomeRoute.Match(saved.removePrefix("match:"))
                saved == "catalog" -> HomeRoute.Catalog
                saved == "scoreboard" -> HomeRoute.Scoreboard
                else -> null
            }
        },
    )

private const val FADE_MILLIS = 250
private const val SKELETON_ROWS = 3
