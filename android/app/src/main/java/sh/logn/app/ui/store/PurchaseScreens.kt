package sh.logn.app.ui.store

import androidx.activity.compose.LocalActivity
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import sh.logn.app.core.PlayStore
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.LocalStore
import sh.logn.app.ui.ShellState
import sh.logn.app.ui.auth.LinkText
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.FullScreenSheet
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.copy.copy
import sh.logn.app.ui.theme.HomeMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.StoreMetrics
import sh.logn.app.ui.theme.Stroke
import sh.logn.app.ui.track.DiscountLine
import sh.logn.app.ui.track.TrackBalloon
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.PurchaseFlowView
import sh.logn.core.LogN.PurchaseStage
import sh.logn.core.LogN.SkillNode
import sh.logn.core.LogN.TrackView
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * O passo a passo da compra (F3): o app só libera depois de o servidor confirmar, e o log
 * mostra cada passo como saída de juiz. Espelho de PurchaseProgressView. A tela nunca
 * prende: enquanto espera dá para sair, e a compra termina sozinha.
 */
@Composable
fun PurchaseProgressScreen(view: ViewModel) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val flow = view.purchaseFlow
    val track = view.tracks.firstOrNull { it.id == flow.trackId }
    val name = track?.name.orEmpty()
    val failed = flow.stage == PurchaseStage.FAILED
    val waiting = !failed && flow.stage != PurchaseStage.READY
    FullScreenSheet(onDismiss = { dispatch(Event.ClosePurchaseFlow) }, background = LognDark.surface) {
        Column(
            Modifier
                .fillMaxSize()
                .padding(start = StoreMetrics.side, end = StoreMetrics.side, top = HomeMetrics.onboardingTop, bottom = HomeMetrics.onboardingBottom),
            verticalArrangement = Arrangement.spacedBy(StoreMetrics.cardPadding),
        ) {
            if (track != null) TrackBalloon(track, HomeMetrics.expiredBalloon)
            Text(
                stageLabel(context, flow.stage),
                style = LognFont.mono(StoreMetrics.META_SIZE, tracking = StoreMetrics.EYEBROW_TRACKING),
                color = if (failed) LognDark.wrongInk else LognDark.textMuted,
            )
            Text(
                stageTitle(context, flow.stage, name),
                style = LognFont.sans(HomeMetrics.EXPIRED_TITLE_SIZE, FontWeight.SemiBold, HomeMetrics.HEADER_TRACKING),
                color = LognDark.textPrimary,
            )
            Text(
                stageBody(context, flow),
                style = LognFont.sans(StoreMetrics.BODY_SIZE).merge(TextStyle(lineHeight = StoreMetrics.BODY_LINE.sp)),
                color = LognDark.textSecondary,
            )
            StepLog(flow.stage)
            Spacer(Modifier.weight(1f))
            LognButton(
                when {
                    failed -> Str.Purchase.close(context)
                    flow.stage == PurchaseStage.READY -> Str.Purchase.open_track(context, name)
                    else -> Str.Purchase.wait(context)
                },
                if (failed) ButtonVariant.Secondary else ButtonVariant.Primary,
                enabled = !waiting,
            ) { dispatch(Event.ClosePurchaseFlow) }
            if (waiting) {
                LinkText(Str.Purchase.background(context), LognDark.textSecondary, Modifier.fillMaxWidth()) { dispatch(Event.ClosePurchaseFlow) }
            }
        }
    }
}

private fun stageRank(stage: PurchaseStage): Int =
    when (stage) {
        PurchaseStage.IDLE, PurchaseStage.VALIDATING -> 1
        PurchaseStage.LICENSING -> 2
        PurchaseStage.DOWNLOADING -> 3
        PurchaseStage.READY -> 4
        PurchaseStage.FAILED -> 0
    }

private fun stageLabel(
    context: android.content.Context,
    stage: PurchaseStage,
): String =
    when (stage) {
        PurchaseStage.READY -> Str.Purchase.ready_label(context)
        PurchaseStage.DOWNLOADING, PurchaseStage.LICENSING -> Str.Purchase.downloading_label(context)
        PurchaseStage.FAILED -> Str.Purchase.failed_label(context)
        else -> Str.Purchase.validating_label(context)
    }

private fun stageTitle(
    context: android.content.Context,
    stage: PurchaseStage,
    track: String,
): String =
    when (stage) {
        PurchaseStage.READY -> Str.Purchase.ready_title(context)
        PurchaseStage.DOWNLOADING, PurchaseStage.LICENSING -> Str.Purchase.downloading_title(context, track)
        PurchaseStage.FAILED -> Str.Purchase.failed_title(context)
        else -> Str.Purchase.validating_title(context)
    }

private fun stageBody(
    context: android.content.Context,
    flow: PurchaseFlowView,
): String =
    when (flow.stage) {
        PurchaseStage.READY -> Str.Purchase.ready_body(context)
        PurchaseStage.DOWNLOADING, PurchaseStage.LICENSING -> Str.Purchase.downloading_body(context)
        PurchaseStage.FAILED -> flow.failure.copy(context) ?: Str.Status.purchase_failed(context)
        else -> Str.Purchase.validating_body_play(context)
    }

/** Os quatro passos: loja, transação, licença, pacote. Feito, em andamento, a vir. */
@Composable
private fun StepLog(stage: PurchaseStage) {
    val context = LocalContext.current
    val failed = stage == PurchaseStage.FAILED
    val rank = stageRank(stage)
    val steps =
        listOf(
            Str.Purchase.log_store_play(context) to Str.Purchase.log_store_ok(context),
            Str.Purchase.log_transaction(context) to Str.Purchase.log_transaction_ok(context),
            Str.Purchase.log_license(context) to Str.Purchase.log_license_ok(context),
            Str.Purchase.log_package(context) to Str.Purchase.log_package_ok(context),
        )
    val shape = RoundedCornerShape(Radius.sm)
    Column(
        Modifier
            .fillMaxWidth()
            .background(LognDark.canvas, shape)
            .border(Stroke.hairline, LognDark.line, shape)
            .padding(StoreMetrics.cardPadding),
    ) {
        for ((i, step) in steps.withIndex()) {
            // A loja já aprovou quando esta tela abre; cada passo depende do anterior.
            val done = i == 0 || i < rank
            val current = !failed && i == rank
            val (mark, ink) =
                when {
                    done -> MARK_DONE to LognDark.correctInk
                    current -> MARK_CURRENT to LognDark.warnInk
                    else -> MARK_LATER to LognDark.textMuted
                }
            Row(Modifier.heightIn(min = StoreMetrics.logRow), verticalAlignment = Alignment.CenterVertically) {
                val style = LognFont.mono(StoreMetrics.LOG_SIZE)
                Text(mark, style = style, color = ink, modifier = Modifier.width(StoreMetrics.logMark))
                Text(step.first, style = style, color = if (done) LognDark.textPrimary else LognDark.textMuted, modifier = Modifier.weight(1f))
                Text(if (done) step.second else "", style = style, color = LognDark.textMuted)
            }
        }
    }
}

/** O visitante tocou em comprar (F2): a compra pede conta, e volta sozinha depois. */
@Composable
fun GuestPurchaseSheet(
    view: ViewModel,
    productId: String,
) {
    val context = LocalContext.current
    val store = LocalStore.current
    val track = view.tracks.firstOrNull { it.productId == productId }
    BottomSheet(onDismiss = { store.guestPrompt = null }, background = LognDark.surface) {
        Column(
            Modifier.padding(start = StoreMetrics.side, end = StoreMetrics.side, top = StoreMetrics.side, bottom = Space.md),
            verticalArrangement = Arrangement.spacedBy(Space.md),
        ) {
            if (track != null) {
                val price = store.prices[track.productId]
                Row(horizontalArrangement = Arrangement.spacedBy(Space.md), verticalAlignment = Alignment.CenterVertically) {
                    TrackBalloon(track, HomeMetrics.trackBalloon)
                    Text(
                        track.name.uppercase() + (price?.let { " · ${it.formatted}" } ?: ""),
                        style = LognFont.mono(StoreMetrics.META_SIZE, tracking = StoreMetrics.EYEBROW_TRACKING),
                        color = LognDark.textMuted,
                    )
                }
                price?.let { DiscountLine(it) }
            }
            Text(
                Str.Guest_purchase.title(context),
                style = LognFont.sans(StoreMetrics.SHEET_TITLE_SIZE, FontWeight.SemiBold, StoreMetrics.TITLE_TRACKING),
                color = LognDark.textPrimary,
            )
            Text(
                Str.Guest_purchase.body_play(context),
                style = LognFont.sans(StoreMetrics.NOTE_SIZE).merge(TextStyle(lineHeight = StoreMetrics.BODY_LINE.sp)),
                color = LognDark.textSecondary,
            )
            LognButton(Str.Guest_purchase.create(context), ButtonVariant.Primary) {
                store.continueAfterSignUp(productId, register = true) { ShellState.wantsRegistration = it }
            }
            LognButton(Str.Guest_purchase.login(context), ButtonVariant.Secondary) {
                store.continueAfterSignUp(productId, register = false) { ShellState.wantsRegistration = it }
            }
            Text(
                Str.Guest_purchase.note(context),
                style = LognFont.mono(StoreMetrics.SMALL_SIZE),
                color = LognDark.textMuted,
                modifier = Modifier.align(Alignment.CenterHorizontally),
            )
        }
    }
}

/** O nó fechado tocado na árvore (1e): o preço aparece quando a pessoa quer seguir. */
@Composable
fun LockedNodeSheet(
    node: SkillNode,
    track: TrackView,
    onDismiss: () -> Unit,
) {
    val context = LocalContext.current
    val store = LocalStore.current
    val activity = LocalActivity.current
    val index = track.nodes.indexOfFirst { it.id == node.id }.let { if (it < 0) 1 else it + 1 }
    LaunchedEffect(track.productId) { store.loadPrices(listOf(track.productId)) }
    val price = store.prices[track.productId]
    BottomSheet(onDismiss = onDismiss, background = LognDark.surface) {
        Column(
            Modifier.padding(start = StoreMetrics.side, end = StoreMetrics.side, top = StoreMetrics.side, bottom = Space.md),
            verticalArrangement = Arrangement.spacedBy(Space.md),
        ) {
            Text(
                Str.Locked.eyebrow(context, index, node.name.uppercase()),
                style = LognFont.mono(StoreMetrics.SMALL_SIZE, tracking = StoreMetrics.EYEBROW_TRACKING),
                color = LognDark.textMuted,
            )
            Text(
                Str.Locked.title(context),
                style = LognFont.sans(StoreMetrics.SHEET_TITLE_SIZE, FontWeight.SemiBold, StoreMetrics.TITLE_TRACKING),
                color = LognDark.textPrimary,
            )
            // "Você terminou a amostra" só para quem terminou. Cada frase é uma chave inteira.
            Column(verticalArrangement = Arrangement.spacedBy(Space.xxs)) {
                val style = LognFont.sans(StoreMetrics.NOTE_SIZE).merge(TextStyle(lineHeight = StoreMetrics.BODY_LINE.sp))
                if (track.sampleDone) Text(Str.Locked.sample_done(context, track.sampleBalloons.toInt()), style = style, color = LognDark.textSecondary)
                Text(Str.Locked.rest(context, track.closedNodeCount.toInt()), style = style, color = LognDark.textSecondary)
            }
            store.problem?.let { Text(PlayStore.copy(context, it), style = LognFont.bodyMedium, color = LognDark.textSecondary) }
            price?.let { DiscountLine(it) }
            // A folha fecha antes da compra: a tela do passo a passo abre da raiz.
            LognButton(
                price?.let { Str.Locked.unlock(context, track.name, it.formatted) } ?: Str.Locked.unlock_no_price(context, track.name),
                ButtonVariant.Primary,
                enabled = track.productId.isNotEmpty(),
                loading = store.isPurchasing,
            ) {
                onDismiss()
                activity?.let { store.buy(it, track.productId) }
            }
            LinkText(Str.Locked.not_now(context), LognDark.textSecondary, Modifier.fillMaxWidth(), onClick = onDismiss)
        }
    }
}

/** A oferta do fim da amostra, no lugar do "Entendi" do relatório (1f). */
@Composable
fun SampleOfferCard(
    view: ViewModel,
    onDismiss: () -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val store = LocalStore.current
    val activity = LocalActivity.current
    val offer = view.sampleOffer
    val track = view.tracks.firstOrNull { it.id == offer.trackId }
    val free = view.tracks.firstOrNull { it.isFree }
    LaunchedEffect(track?.productId) { track?.let { store.loadPrices(listOf(it.productId)) } }
    val name = track?.name.orEmpty()
    val price = track?.let { store.prices[it.productId] }
    val shape = RoundedCornerShape(Radius.sm)
    Column(verticalArrangement = Arrangement.spacedBy(Space.md)) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(LognDark.canvas, shape)
                .border(Stroke.hairline, LognDark.lineStrong, shape)
                .padding(Space.lg),
            verticalArrangement = Arrangement.spacedBy(Space.md),
        ) {
            Text(
                Str.Offer.eyebrow(context, offer.nextNodeIndex.toInt(), track?.nodeCount?.toInt() ?: 0),
                style = LognFont.mono(StoreMetrics.SMALL_SIZE, tracking = StoreMetrics.EYEBROW_TRACKING),
                color = LognDark.textMuted,
            )
            Text(offer.nextNodeName, style = LognFont.sans(StoreMetrics.PRINCIPAL_NAME_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
            Row(horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
                val style = LognFont.mono(StoreMetrics.LOG_SIZE)
                Text(Str.Offer.nodes(context, offer.remainingNodes.toInt()), style = style, color = LognDark.textSecondary)
                Text(Str.Offer.problems(context, offer.remainingProblems.toInt()), style = style, color = LognDark.textSecondary)
            }
            Text(Str.Offer.tail(context), style = LognFont.sans(StoreMetrics.NOTE_SIZE), color = LognDark.textSecondary)
        }
        store.problem?.let { Text(PlayStore.copy(context, it), style = LognFont.bodyMedium, color = LognDark.textSecondary) }
        price?.let { DiscountLine(it) }
        LognButton(
            price?.let { Str.Offer.continue_buy(context, name, it.formatted) } ?: Str.Offer.continue_buy_no_price(context, name),
            ButtonVariant.Primary,
            enabled = track != null,
            loading = store.isPurchasing,
        ) { if (track != null && activity != null) store.buy(activity, track.productId) }
        LinkText(Str.Offer.back(context, free?.name.orEmpty()), LognDark.textSecondary, Modifier.fillMaxWidth()) {
            dispatch(Event.DismissSampleOffer(offer.trackId))
            if (free != null) dispatch(Event.SelectTrack(free.id))
            onDismiss()
        }
    }
}

/** O resultado de "Restaurar compras". */
@Composable
fun RestoreResultSheet(view: ViewModel) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val result = view.restoreResult
    val message =
        when {
            !result.finished -> Str.Restore.running_play(context)
            result.total == 0u || result.restoredNames.isEmpty() -> Str.Restore.nothing_play(context)
            else -> Str.Restore.names(context, result.restoredNames.joinToString(", "))
        }
    BottomSheet(onDismiss = { dispatch(Event.DismissRestoreResult) }, dismissible = result.finished, background = LognDark.surface) {
        Column(Modifier.padding(StoreMetrics.side), verticalArrangement = Arrangement.spacedBy(Space.md)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    Str.Restore.title(context),
                    style = LognFont.mono(StoreMetrics.META_SIZE, tracking = StoreMetrics.EYEBROW_TRACKING),
                    color = LognDark.textMuted,
                    modifier = Modifier.weight(1f),
                )
                if (result.finished && result.total > 0u) {
                    val any = result.restored > 0u
                    Text(
                        Str.Restore.count(context, result.restored.toInt(), result.total.toInt()),
                        style = LognFont.mono(StoreMetrics.META_SIZE, FontWeight.Medium),
                        color = if (any) LognDark.correctInk else LognDark.textSecondary,
                        modifier =
                            Modifier
                                .border(Stroke.hairline, if (any) LognDark.correct else LognDark.lineStrong, RoundedCornerShape(Radius.xs))
                                .padding(horizontal = StoreMetrics.chipPaddingH, vertical = StoreMetrics.gap3),
                    )
                }
            }
            Text(message, style = LognFont.sans(StoreMetrics.BODY_SIZE), color = LognDark.textPrimary)
            if (result.otherAccount > 0u) {
                Text(Str.Restore.other(context, result.otherAccount.toInt()), style = LognFont.sans(StoreMetrics.NOTE_SIZE), color = LognDark.warnInk)
            }
            LognButton(Str.Restore.ok(context), ButtonVariant.Secondary, enabled = result.finished) { dispatch(Event.DismissRestoreResult) }
        }
    }
}

private const val MARK_DONE = "✓"
private const val MARK_CURRENT = "…"
private const val MARK_LATER = "·"
