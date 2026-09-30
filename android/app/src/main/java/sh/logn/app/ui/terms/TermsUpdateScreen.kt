package sh.logn.app.ui.terms

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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.size
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.account.DeleteAccountSheet
import sh.logn.app.ui.auth.LinkText
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.DestructiveButton
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.legal.LegalDocumentScreen
import sh.logn.app.ui.legal.LegalKind
import sh.logn.app.ui.theme.FieldMetrics
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.NoticeMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.app.ui.theme.TermsMetrics
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.TermsChange
import sh.logn.core.LogN.TermsChangeKind
import sh.logn.core.LogN.TermsUpdateViewModel
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * O que a tela mostra, como identidade: versão e mudanças. Mudou, a tela nasce de novo,
 * com a caixa desmarcada (a caixa marcada para uma versão não vale para a que veio depois
 * de um 409).
 */
fun TermsUpdateViewModel.identity(): String =
    buildString {
        append(fromVersion).append('|').append(toVersion)
        for (c in changes) append('|').append(c.kind).append(':').append(c.version).append(':').append(c.section)
    }

/**
 * A tela que cobre o app quando há versão relevante dos termos para aceitar (ADR 0020).
 * Espelho de TermsUpdateView.swift. O Core decide quando aparece e o que lista; a caixa é
 * daqui, nunca vem marcada, e o botão só aceita com ela marcada.
 */
@Composable
fun TermsUpdateScreen(
    view: ViewModel,
    update: TermsUpdateViewModel,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    var checked by rememberSaveable(update.identity()) { mutableStateOf(false) }
    var showsDocuments by rememberSaveable { mutableStateOf(false) }
    var showsDecline by rememberSaveable { mutableStateOf(false) }
    val canAccept = checked && !update.accepting
    val headline =
        when {
            update.fromVersion == 0u -> Str.Reaccept.headline_first(context)
            update.versionsSkipped > 1u -> Str.Reaccept.headline_times(context, update.versionsSkipped.toInt())
            else -> Str.Reaccept.headline(context)
        }

    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.canvas)
            .safeDrawingPadding(),
    ) {
        Column(
            Modifier
                .weight(1f)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = TermsMetrics.sideMargin)
                .padding(bottom = Space.lg),
            verticalArrangement = Arrangement.spacedBy(Space.lg),
        ) {
            Text(
                Str.Reaccept.eyebrow(context),
                style = LognFont.mono(TermsMetrics.EYEBROW_SIZE, tracking = TermsMetrics.EYEBROW_TRACKING),
                color = LognDark.infoInk,
                modifier = Modifier.padding(top = Space.sm),
            )
            Text(headline, style = LognFont.sans(TermsMetrics.HEADLINE_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
            Versions(update)
            Changes(update.changes)
            LinkText(
                Str.Reaccept.read_documents(context),
                LognDark.textSecondary,
                underline = true,
            ) { showsDocuments = true }
        }
        Footer(update, checked, canAccept, onToggle = { checked = !checked }, onDecline = { showsDecline = true }) {
            dispatch(Event.AcceptTerms)
        }
    }

    // Abre nos termos, com as seções novas marcadas; a troca para a política fica no
    // cabeçalho. Os dois documentos têm seções de mesmo id, e as da política não vão juntas.
    if (showsDocuments) LegalDocumentScreen(LegalKind.Terms, onClose = { showsDocuments = false }, highlight = update.termsSections)
    if (showsDecline) TermsDeclineSheet(view, onDismiss = { showsDecline = false })
}

@Composable
private fun Versions(update: TermsUpdateViewModel) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        Modifier
            .fillMaxWidth()
            .height(IntrinsicSize.Min)
            .background(LognDark.surface, shape)
            .border(Stroke.hairline, LognDark.line, shape),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        VersionCell(
            Str.Reaccept.you_accepted(context),
            if (update.fromVersion == 0u) {
                Str.Reaccept.never_accepted(context)
            } else {
                Str.Reaccept.version(context, update.fromVersion.toInt())
            },
            update.fromDate,
            LognDark.textMuted,
            LognDark.textSecondary,
            Modifier.weight(1f),
        )
        Box(Modifier.width(TermsMetrics.arrowSlot), contentAlignment = Alignment.Center) {
            Icon(LognIcon.ArrowRight, LognDark.textMuted, IconMetrics.sm)
        }
        Box(
            Modifier
                .width(Stroke.hairline)
                .fillMaxHeight()
                .background(LognDark.line),
        )
        VersionCell(
            Str.Reaccept.current(context),
            Str.Reaccept.version(context, update.toVersion.toInt()),
            update.toDate,
            LognDark.infoInk,
            LognDark.textPrimary,
            Modifier.weight(1f),
        )
    }
}

@Composable
private fun VersionCell(
    label: String,
    version: String,
    date: String,
    labelColor: Color,
    valueColor: Color,
    modifier: Modifier,
) {
    Column(
        modifier
            .padding(horizontal = Space.md, vertical = TermsMetrics.cellPaddingV)
            .clearAndSetSemantics { contentDescription = "$label, $version, $date" },
        verticalArrangement = Arrangement.spacedBy(TermsMetrics.cellGap),
    ) {
        Text(label, style = LognFont.mono(TermsMetrics.CELL_LABEL_SIZE, tracking = TermsMetrics.CELL_LABEL_TRACKING), color = labelColor)
        Text(version, style = LognFont.mono(TermsMetrics.CELL_VALUE_SIZE), color = valueColor)
        Text(date, style = LognFont.mono(TermsMetrics.CELL_DATE_SIZE), color = LognDark.textMuted)
    }
}

@Composable
private fun Changes(changes: List<TermsChange>) {
    val context = LocalContext.current
    Column(verticalArrangement = Arrangement.spacedBy(Space.sm)) {
        Row(Modifier.fillMaxWidth()) {
            val style = LognFont.mono(TermsMetrics.LIST_LABEL_SIZE, tracking = TermsMetrics.LIST_LABEL_TRACKING)
            Text(Str.Reaccept.what_changed(context), style = style, color = LognDark.textMuted)
            Spacer(Modifier.weight(1f))
            Text(Str.Reaccept.changes_count(context, changes.size), style = style, color = LognDark.textMuted)
        }
        for (change in changes) ChangeRow(change)
    }
}

/** Uma mudança do diff: o sinal numa faixa tingida, de onde veio e a frase. */
@Composable
private fun ChangeRow(change: TermsChange) {
    val context = LocalContext.current
    val (sign, name, ink, tint) =
        when (change.change) {
            TermsChangeKind.ADDED -> ChangeLook("+", Str.Reaccept.sign_added(context), LognDark.correct, LognDark.tintOk)
            TermsChangeKind.CHANGED -> ChangeLook("~", Str.Reaccept.sign_changed(context), LognDark.warn, LognDark.tintWarn)
            TermsChangeKind.REMOVED -> ChangeLook("−", Str.Reaccept.sign_removed(context), LognDark.wrong, LognDark.tintErr)
        }
    val document = if (change.kind == "privacy") Str.Reaccept.doc_privacy(context) else Str.Reaccept.doc_terms(context)
    val place = Str.Reaccept.change_where(context, document, change.version.toInt())
    val label = Str.Reaccept.change_accessibility(context, name, place, change.summary)
    val shape = RoundedCornerShape(TermsMetrics.rowRadius)
    Row(
        Modifier
            .fillMaxWidth()
            .height(IntrinsicSize.Min)
            .background(LognDark.surface, shape)
            .border(Stroke.hairline, LognDark.line, shape)
            .clearAndSetSemantics { contentDescription = label },
    ) {
        Row(
            Modifier
                .fillMaxHeight()
                .background(tint),
        ) {
            Text(
                sign,
                style = LognFont.mono(TermsMetrics.SIGN_SIZE),
                color = ink,
                modifier =
                    Modifier
                        .width(TermsMetrics.signSlot)
                        .padding(top = Space.sm),
                textAlign = TextAlign.Center,
            )
            Box(
                Modifier
                    .width(TermsMetrics.signBar)
                    .fillMaxHeight()
                    .background(ink),
            )
        }
        Column(
            Modifier.padding(horizontal = TermsMetrics.rowPaddingH, vertical = Space.sm),
            verticalArrangement = Arrangement.spacedBy(TermsMetrics.cellGap),
        ) {
            Text(place, style = LognFont.mono(TermsMetrics.CELL_LABEL_SIZE, tracking = TermsMetrics.PLACE_TRACKING), color = LognDark.textMuted)
            Text(change.summary, style = LognFont.sans(TermsMetrics.BODY_SIZE), color = LognDark.textPrimary)
        }
    }
}

private data class ChangeLook(
    val sign: String,
    val name: String,
    val ink: Color,
    val tint: Color,
)

@Composable
private fun Footer(
    update: TermsUpdateViewModel,
    checked: Boolean,
    canAccept: Boolean,
    onToggle: () -> Unit,
    onDecline: () -> Unit,
    onAccept: () -> Unit,
) {
    val context = LocalContext.current
    Column(
        Modifier
            .fillMaxWidth()
            .background(LognDark.surface),
    ) {
        Box(
            Modifier
                .fillMaxWidth()
                .height(Stroke.hairline)
                .background(LognDark.line),
        )
        Column(
            Modifier.padding(start = TermsMetrics.sideMargin, end = TermsMetrics.sideMargin, top = TermsMetrics.footerTop, bottom = Space.md),
            verticalArrangement = Arrangement.spacedBy(Space.md),
        ) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = Space.minTouch)
                    .semantics { selected = checked }
                    .clickable(role = Role.Checkbox, onClick = onToggle),
                horizontalArrangement = Arrangement.spacedBy(FieldMetrics.checkboxGap),
                verticalAlignment = Alignment.Top,
            ) {
                AcceptBox(checked)
                Text(
                    Str.Reaccept.accept_check(context, update.toVersion.toInt()),
                    style = LognFont.sans(TermsMetrics.BODY_SIZE),
                    color = LognDark.textSecondary,
                )
            }
            LognButton(
                if (update.accepting) Str.Reaccept.accepting(context) else Str.Reaccept.accept_button(context),
                ButtonVariant.Primary,
                enabled = canAccept,
                onClick = onAccept,
            )
            LinkText(
                Str.Reaccept.decline(context),
                LognDark.textSecondary,
                Modifier.fillMaxWidth(),
                enabled = !update.accepting,
                onClick = onDecline,
            )
        }
    }
}

/** A caixa do aceite: 20, raio 3, traço 1.5; cheia de acento quando marcada. */
@Composable
private fun AcceptBox(checked: Boolean) {
    val shape = RoundedCornerShape(TermsMetrics.boxRadius)
    Box(
        Modifier
            .size(TermsMetrics.box)
            .background(if (checked) LognDark.accent else Color.Transparent, shape)
            .border(TermsMetrics.boxStroke, if (checked) LognDark.accent else LognDark.lineDim, shape),
        contentAlignment = Alignment.Center,
    ) {
        if (checked) Icon(LognIcon.Check, LognDark.onAccent, IconMetrics.sm, strokeWidth = TermsMetrics.CHECK_STROKE)
    }
}

/**
 * "Não concordo": quem está bloqueado não alcança o Perfil, então a saída e a exclusão
 * ficam aqui.
 */
@Composable
fun TermsDeclineSheet(
    view: ViewModel,
    onDismiss: () -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    var showsDelete by rememberSaveable { mutableStateOf(false) }
    BottomSheet(onDismiss = onDismiss) {
        Column(
            Modifier.padding(start = Space.screenMargin, end = Space.screenMargin, top = TermsMetrics.declineTop, bottom = Space.lg),
            verticalArrangement = Arrangement.spacedBy(Space.lg),
        ) {
            Text(
                Str.Reaccept.decline_title(context),
                style = LognFont.sans(NoticeMetrics.TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
            )
            Text(Str.Reaccept.decline_body(context), style = LognFont.sans(NoticeMetrics.BODY_SIZE), color = LognDark.textSecondary)
            LognButton(Str.Reaccept.sign_out(context), ButtonVariant.Secondary) {
                onDismiss()
                dispatch(Event.Logout)
            }
            DestructiveButton(Str.Reaccept.delete_account(context)) { showsDelete = true }
            LinkText(Str.Reaccept.back(context), LognDark.textSecondary, Modifier.fillMaxWidth(), onClick = onDismiss)
        }
    }
    if (showsDelete) DeleteAccountSheet(view, onDismiss = { showsDelete = false })
}

/** A faixa das mudanças não relevantes, uma vez. O aceite já foi gravado; fechar só a tira. */
@Composable
fun TermsNoticeBanner(modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    var showsDocument by rememberSaveable { mutableStateOf(false) }
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        modifier
            .padding(horizontal = Space.screenMargin)
            .fillMaxWidth()
            .background(LognDark.tintInfo, shape)
            .border(Stroke.hairline, LognDark.info, shape)
            .padding(start = TermsMetrics.bannerPaddingStart),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        Text(
            Str.Reaccept.notice(context),
            style = LognFont.sans(TermsMetrics.BODY_SIZE),
            color = LognDark.textPrimary,
            modifier =
                Modifier
                    .weight(1f)
                    .padding(vertical = Space.md),
        )
        Box(
            Modifier
                .heightIn(min = Space.minTouch)
                .clickable(role = Role.Button) { showsDocument = true },
            contentAlignment = Alignment.Center,
        ) {
            Text(
                Str.Reaccept.notice_open(context),
                style = LognFont.sans(TermsMetrics.BODY_SIZE, FontWeight.Medium),
                color = LognDark.infoInk,
            )
        }
        val close = Str.Reaccept.notice_close(context)
        Box(
            Modifier
                .size(Space.minTouch)
                .semantics { contentDescription = close }
                .clickable(role = Role.Button) { dispatch(Event.DismissTermsNotice) },
            contentAlignment = Alignment.Center,
        ) { Icon(LognIcon.Close, LognDark.textMuted, IconMetrics.sm) }
    }
    if (showsDocument) LegalDocumentScreen(LegalKind.Terms, onClose = { showsDocument = false })
}
