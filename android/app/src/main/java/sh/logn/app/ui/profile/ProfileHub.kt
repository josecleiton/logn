package sh.logn.app.ui.profile

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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.sp
import sh.logn.app.BuildConfig
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.ShellState
import sh.logn.app.ui.account.CriticalLogoutSheet
import sh.logn.app.ui.account.ManageAccountScreen
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.FullScreenSheet
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.home.ProfileAvatar
import sh.logn.app.ui.legal.LegalDocumentScreen
import sh.logn.app.ui.legal.LegalKind
import sh.logn.app.ui.legal.LegalLinksRow
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.NodeSheetMetrics
import sh.logn.app.ui.theme.ProfileMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.tree.StatCell
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str
import sh.logn.app.ui.theme.Stroke as LognStroke

/**
 * O hub de perfil (tela 5 do DS), numa folha sobre a árvore, que fica à vista atrás.
 * Espelho de ProfileHubView.swift: sincronizado, com fila offline, ou visitante.
 * `onRestore` é o "Restaurar compras" da loja do aparelho.
 */
@Composable
fun ProfileHub(
    view: ViewModel,
    onDismiss: () -> Unit,
    onRestore: () -> Unit,
    restoreSheet: @Composable () -> Unit = {},
) {
    var showsCritical by rememberSaveable { mutableStateOf(false) }
    var showsAccount by rememberSaveable { mutableStateOf(false) }
    var showsStorage by rememberSaveable { mutableStateOf(false) }
    var legal by rememberSaveable { mutableStateOf<LegalKind?>(null) }
    BottomSheet(onDismiss = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(start = Space.screenMargin, end = Space.screenMargin, top = Space.lg, bottom = ProfileMetrics.bottom),
        ) {
            Box(
                Modifier
                    .align(Alignment.CenterHorizontally)
                    .padding(bottom = ProfileMetrics.grabberBottom)
                    .size(NodeSheetMetrics.handleWidth, NodeSheetMetrics.handleHeight)
                    .background(LognDark.lineStrong, RoundedCornerShape(NodeSheetMetrics.handleHeight)),
            )
            Identity(view)
            if (view.pendingSyncCount > 0u && !view.isGuest) QueueCard(view)
            LevelBlock(view)
            Stats(view)
            Summary(view)
            Footer(
                view,
                onStorage = { showsStorage = true },
                onRestore = onRestore,
                onLogout = { if (view.pendingSyncCount > 0u) showsCritical = true },
                onAccount = { showsAccount = true },
                onLegal = { legal = it },
            )
        }
    }
    if (showsCritical) {
        CriticalLogoutSheet(view, onStay = { showsCritical = false })
    }
    if (showsAccount) {
        FullScreenSheet(onDismiss = { showsAccount = false }, background = LognDark.canvas) {
            ManageAccountScreen(view, onBack = { showsAccount = false })
        }
    }
    if (showsStorage) StorageSheet(view) { showsStorage = false }
    legal?.let { LegalDocumentScreen(it, onClose = { legal = null }) }
    restoreSheet()
}

@Composable
private fun Identity(view: ViewModel) {
    val context = LocalContext.current
    val pending = view.pendingSyncCount.toInt()
    Row(horizontalArrangement = Arrangement.spacedBy(NodeSheetMetrics.headerGap), verticalAlignment = Alignment.CenterVertically) {
        if (view.isGuest) {
            // Visitante não tem inicial: ícone genérico e borda tracejada.
            Box(
                Modifier
                    .size(ProfileMetrics.avatar)
                    .background(LognDark.surface, CircleShape)
                    .drawBehind {
                        val dash = ProfileMetrics.dash.toPx()
                        drawCircle(
                            LognDark.lineDim,
                            style = Stroke(LognStroke.hairline.toPx(), pathEffect = PathEffect.dashPathEffect(floatArrayOf(dash, dash))),
                        )
                    },
                contentAlignment = Alignment.Center,
            ) { Icon(LognIcon.Person, LognDark.textMuted, ProfileMetrics.personIcon) }
        } else {
            ProfileAvatar(view.displayName.ifEmpty { view.accountEmail }, hasPending = false, size = ProfileMetrics.avatar)
        }
        Column(Modifier.weight(1f)) {
            if (view.isGuest) {
                Text(
                    Str.Profile.guest_mode(context),
                    style = LognFont.mono(ProfileMetrics.TAG_SIZE, tracking = ProfileMetrics.TAG_TRACKING),
                    color = LognDark.textSecondary,
                    modifier =
                        Modifier
                            .border(LognStroke.hairline, LognDark.lineStrong, RoundedCornerShape(Radius.xs))
                            .padding(horizontal = ProfileMetrics.tagPaddingH, vertical = Space.xs),
                )
                Text(
                    Str.Profile.guest_sub(context),
                    style = LognFont.mono(ProfileMetrics.SMALL_SIZE, tracking = ProfileMetrics.SMALL_TRACKING),
                    color = LognDark.textMuted,
                    modifier = Modifier.padding(top = ProfileMetrics.gap6),
                )
            } else {
                Text(
                    view.accountEmail,
                    style = LognFont.mono(ProfileMetrics.EMAIL_SIZE),
                    color = LognDark.textPrimary,
                    maxLines = 1,
                    overflow = TextOverflow.MiddleEllipsis,
                )
                Row(
                    Modifier.padding(top = ProfileMetrics.gap6),
                    horizontalArrangement = Arrangement.spacedBy(ProfileMetrics.gap6),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Box(
                        Modifier
                            .size(ProfileMetrics.dot)
                            .background(if (pending > 0) LognDark.warn else LognDark.correct, CircleShape),
                    )
                    Text(
                        if (pending > 0) Str.Profile.queued_events(context, pending) else Str.Profile.all_synced(context),
                        style = LognFont.mono(ProfileMetrics.SMALL_SIZE, tracking = ProfileMetrics.SMALL_TRACKING),
                        color = if (pending > 0) LognDark.warnInk else LognDark.textMuted,
                    )
                }
            }
        }
    }
}

@Composable
private fun QueueCard(view: ViewModel) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        Modifier
            .padding(top = Space.lg)
            .fillMaxWidth()
            .background(LognDark.tintWarn, shape)
            .border(LognStroke.hairline, LognDark.warn, shape)
            .padding(horizontal = ProfileMetrics.cardPaddingH, vertical = Space.md),
        horizontalArrangement = Arrangement.spacedBy(ProfileMetrics.gap11),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                Str.Profile.local_xp_only(context, view.globalXp),
                style = LognFont.sans(ProfileMetrics.BODY_SIZE).merge(TextStyle(lineHeight = ProfileMetrics.BODY_LINE.sp)),
                color = LognDark.textPrimary,
            )
            Text(
                Str.Profile.pending_events(context, view.pendingSyncCount.toInt()),
                style = LognFont.mono(ProfileMetrics.META_SIZE),
                color = LognDark.textSecondary,
                modifier = Modifier.padding(top = Space.xs),
            )
        }
        Box(
            Modifier
                .height(ProfileMetrics.retryHeight)
                .border(LognStroke.hairline, LognDark.warn, RoundedCornerShape(ProfileMetrics.retryRadius))
                .clickable(role = Role.Button) { dispatch(Event.SyncNow) }
                .padding(horizontal = ProfileMetrics.cardPaddingH),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                if (view.isSyncing) ELLIPSIS else Str.Profile.retry(context),
                style = LognFont.mono(ProfileMetrics.RETRY_SIZE, FontWeight.Medium),
                color = LognDark.warnInk,
            )
        }
    }
}

/** O nível é o herói da tela. */
@Composable
private fun LevelBlock(view: ViewModel) {
    val context = LocalContext.current
    val label = Str.Profile.level_accessibility(context, view.level, view.globalXp, view.xpToNextLevel, view.level + 1)
    val progress = view.xpIntoLevel.toFloat() / view.xpForLevel.coerceAtLeast(1)
    Row(
        Modifier
            .padding(top = Space.screenMargin)
            .fillMaxWidth()
            .clearAndSetSemantics { contentDescription = label },
        horizontalArrangement = Arrangement.spacedBy(NodeSheetMetrics.headerGap),
        verticalAlignment = Alignment.Bottom,
    ) {
        Row(horizontalArrangement = Arrangement.spacedBy(Space.sm), verticalAlignment = Alignment.Bottom) {
            Text(
                "${view.level}",
                style = LognFont.sans(ProfileMetrics.LEVEL_SIZE, FontWeight.SemiBold, ProfileMetrics.LEVEL_TRACKING),
                color = LognDark.textPrimary,
            )
            Text(
                Str.Profile.level(context),
                style = LognFont.mono(ProfileMetrics.TAG_SIZE, tracking = ProfileMetrics.LEVEL_LABEL_TRACKING),
                color = LognDark.textMuted,
                modifier = Modifier.padding(bottom = Space.sm),
            )
        }
        Column(
            Modifier
                .weight(1f)
                .padding(bottom = ProfileMetrics.gap3),
        ) {
            Row(Modifier.fillMaxWidth()) {
                val style = LognFont.mono(ProfileMetrics.META_SIZE)
                Text(Str.Profile.xp(context, view.globalXp), style = style, color = LognDark.textMuted)
                Spacer(Modifier.weight(1f))
                Text("${view.level * view.xpForLevel}", style = style, color = LognDark.textMuted)
            }
            Box(
                Modifier
                    .padding(top = ProfileMetrics.gap6)
                    .fillMaxWidth()
                    .height(ProfileMetrics.barHeight)
                    .background(LognDark.line, CircleShape),
            ) {
                Box(
                    Modifier
                        .fillMaxWidth(progress.coerceIn(0f, 1f))
                        .height(ProfileMetrics.barHeight)
                        .background(LognDark.accent, CircleShape),
                )
            }
            Text(
                Str.Profile.xp_to_next(context, view.xpToNextLevel, view.level + 1),
                style = LognFont.mono(ProfileMetrics.TINY_SIZE),
                color = LognDark.textMuted,
                modifier = Modifier.padding(top = ProfileMetrics.gap5),
            )
        }
    }
}

@Composable
private fun Stats(view: ViewModel) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        Modifier
            .padding(top = Space.screenMargin)
            .fillMaxWidth()
            .height(IntrinsicSize.Min)
            .background(LognDark.line, shape)
            .border(LognStroke.hairline, LognDark.line, shape),
        horizontalArrangement = Arrangement.spacedBy(LognStroke.hairline),
    ) {
        ProfileStat(Str.Profile.total_xp(context), "${view.globalXp}", null, Modifier.weight(1f))
        // Os nomes dos formatos, como no iOS: é vocabulário do jogo, igual em toda língua.
        ProfileStat(Str.Profile.bugs(context), "${view.bugsFound}", SPOT_THE_BUG, Modifier.weight(1f))
        ProfileStat(Str.Profile.dry_runs(context), "${view.dryRunsCompleted}", TRACE, Modifier.weight(1f))
    }
}

@Composable
private fun ProfileStat(
    label: String,
    value: String,
    sub: String?,
    modifier: Modifier,
) {
    Column(
        modifier
            .fillMaxHeight()
            .background(LognDark.surface)
            .padding(horizontal = ProfileMetrics.cardPaddingH, vertical = Space.md),
    ) {
        Text(label, style = LognFont.mono(ProfileMetrics.SMALL_SIZE, tracking = ProfileMetrics.SMALL_TRACKING), color = LognDark.textMuted)
        Text(
            value,
            style = LognFont.mono(ProfileMetrics.STAT_SIZE, FontWeight.SemiBold),
            color = LognDark.textPrimary,
            modifier = Modifier.padding(top = Space.xs),
        )
        if (sub != null) {
            Text(
                sub,
                style = LognFont.mono(ProfileMetrics.TINY_SIZE),
                color = LognDark.textMuted,
                modifier = Modifier.padding(top = Space.xxs),
            )
        }
    }
}

@Composable
private fun Summary(view: ViewModel) {
    val context = LocalContext.current
    Text(
        Str.Profile.stats_summary(
            context,
            Str.Profile.challenges_completed(context, view.challengesCompleted),
            Str.Dashboard.balloons_up(context, view.balloonsUp),
        ),
        style = LognFont.mono(ProfileMetrics.META_SIZE),
        color = LognDark.textMuted,
        modifier = Modifier.padding(top = Space.md),
    )
}

@Composable
private fun Footer(
    view: ViewModel,
    onStorage: () -> Unit,
    onRestore: () -> Unit,
    onLogout: () -> Unit,
    onAccount: () -> Unit,
    onLegal: (LegalKind) -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val pending = view.pendingSyncCount > 0u
    if (view.isGuest) {
        ConversionCard(view)
        FooterLink(Str.Profile.has_account(context), Modifier.padding(top = ProfileMetrics.gap6)) { dispatch(Event.Logout) }
    } else {
        FooterRow(LognIcon.Drive, Str.Profile.storage(context), Modifier.padding(top = Space.lg), onClick = onStorage)
        FooterRow(LognIcon.ArrowClockwise, Str.Profile.restore_purchases(context), onClick = onRestore)
        Box(
            Modifier
                .padding(top = if (pending) Space.screenMargin else ProfileMetrics.gap22, bottom = Space.lg)
                .fillMaxWidth()
                .height(LognStroke.hairline)
                .background(LognDark.line),
        )
        // Sem alerta quando não há o que perder: sair sincronizado é reversível, e a tela
        // de saída traz o desfazer.
        val shape = RoundedCornerShape(Radius.sm)
        Row(
            Modifier
                .fillMaxWidth()
                .heightIn(min = ProfileMetrics.logoutHeight)
                .border(LognStroke.hairline, LognDark.lineStrong, shape)
                .clickable(role = Role.Button) { if (pending) onLogout() else dispatch(Event.Logout) },
            horizontalArrangement = Arrangement.spacedBy(ProfileMetrics.gap9, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (pending) {
                Box(
                    Modifier
                        .size(ProfileMetrics.logoutDot)
                        .background(LognDark.warn, CircleShape),
                )
            }
            Text(Str.Profile.sign_out(context), style = LognFont.sans(ProfileMetrics.LOGOUT_SIZE, FontWeight.Medium), color = LognDark.textPrimary)
        }
        Row(
            Modifier
                .padding(top = ProfileMetrics.gap6)
                .fillMaxWidth()
                .heightIn(min = ProfileMetrics.linkHeight)
                .clickable(role = Role.Button, onClick = onAccount),
            horizontalArrangement = Arrangement.spacedBy(ProfileMetrics.gap7, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(LognIcon.Lock, LognDark.textMuted, IconMetrics.sm)
            Text(Str.Profile.manage_account(context), style = LognFont.sans(ProfileMetrics.BODY_SIZE), color = LognDark.textSecondary)
        }
    }
    // Visitante e conta: a política promete o interruptor para os dois.
    AnalyticsToggle(view, Modifier.padding(top = NodeSheetMetrics.headerGap))
    LegalLinksRow(onOpen = onLegal, modifier = Modifier.padding(top = Space.xs))
    // Para o jogador dizer qual versão tem quando reporta um problema.
    Text(
        Str.Profile.app_version(context, BuildConfig.VERSION_NAME, BuildConfig.VERSION_CODE.toString()),
        style = LognFont.mono(ProfileMetrics.VERSION_SIZE),
        color = LognDark.textDim,
        textAlign = TextAlign.Center,
        modifier = Modifier.fillMaxWidth().padding(top = Space.xxs),
    )
}

@Composable
private fun FooterRow(
    icon: LognIcon,
    title: String,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    Row(
        modifier
            .fillMaxWidth()
            .heightIn(min = ProfileMetrics.rowHeight)
            .clickable(role = Role.Button, onClick = onClick),
        horizontalArrangement = Arrangement.spacedBy(ProfileMetrics.gap9),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, LognDark.textSecondary, IconMetrics.md)
        Text(title, style = LognFont.sans(ProfileMetrics.ROW_SIZE), color = LognDark.textSecondary)
    }
}

@Composable
private fun FooterLink(
    title: String,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    Box(
        modifier
            .fillMaxWidth()
            .heightIn(min = ProfileMetrics.rowHeight)
            .clickable(role = Role.Button, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) { Text(title, style = LognFont.sans(ProfileMetrics.ROW_SIZE), color = LognDark.textSecondary) }
}

/** O interruptor da telemetria de uso. Quem guarda a escolha e para de mandar é o Core. */
@Composable
private fun AnalyticsToggle(
    view: ViewModel,
    modifier: Modifier,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        modifier
            .fillMaxWidth()
            .background(LognDark.surface, shape)
            .border(LognStroke.hairline, LognDark.line, shape)
            .padding(horizontal = NodeSheetMetrics.headerGap, vertical = Space.md),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(ProfileMetrics.gap3)) {
            Text(Str.Profile.analytics_title(context), style = LognFont.sans(ProfileMetrics.TOGGLE_TITLE_SIZE, FontWeight.Medium), color = LognDark.textPrimary)
            Text(Str.Profile.analytics_desc(context), style = LognFont.sans(ProfileMetrics.TOGGLE_DESC_SIZE), color = LognDark.textMuted)
        }
        Switch(
            checked = view.analyticsEnabled,
            onCheckedChange = { dispatch(Event.SetAnalyticsEnabled(it)) },
            colors =
                SwitchDefaults.colors(
                    checkedThumbColor = LognDark.onAccent,
                    checkedTrackColor = LognDark.accent,
                    uncheckedThumbColor = LognDark.textMuted,
                    uncheckedTrackColor = LognDark.line,
                    uncheckedBorderColor = LognDark.lineStrong,
                ),
        )
    }
}

/** Nomeia o risco e o ganho em números reais, nunca um "crie sua conta" genérico. */
@Composable
private fun ConversionCard(view: ViewModel) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val shape = RoundedCornerShape(Radius.sm)
    Column(
        Modifier
            .padding(top = ProfileMetrics.gap22)
            .fillMaxWidth()
            .background(LognDark.accent.copy(alpha = ProfileMetrics.CONVERSION_ALPHA), shape)
            .border(LognStroke.hairline, LognDark.accent, shape)
            .padding(start = Space.lg, end = Space.lg, top = Space.lg, bottom = ProfileMetrics.gap18),
    ) {
        Text(
            Str.Profile.guest_risk(context),
            style = LognFont.sans(ProfileMetrics.RISK_SIZE, FontWeight.SemiBold),
            color = LognDark.textPrimary,
        )
        Text(
            Str.Profile.guest_risk_desc(context, view.globalXp, view.balloonsUp, view.challengesCompleted),
            style = LognFont.sans(ProfileMetrics.BODY_SIZE).merge(TextStyle(lineHeight = ProfileMetrics.RISK_LINE.sp)),
            color = LognDark.textSecondary,
            modifier = Modifier.padding(top = Space.sm),
        )
        LognButton(Str.Profile.create_account(context), ButtonVariant.Primary, Modifier.padding(top = Space.lg)) {
            ShellState.wantsRegistration = true
            dispatch(Event.Logout)
        }
    }
}

private const val ELLIPSIS = "…"
private const val SPOT_THE_BUG = "spot the bug"
private const val TRACE = "trace"
