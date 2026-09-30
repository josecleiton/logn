package sh.logn.app.ui.account

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.auth.LinkText
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.DestructiveButton
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.NoticeMetrics
import sh.logn.app.ui.theme.ProfileMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * Saída com fila pendente. Espelho de CriticalLogoutSheet: não pergunta "tem certeza";
 * nomeia a perda em números, e põe a saída sem perda como primário. O destrutivo fica
 * em terceiro, como texto.
 */
@Composable
fun CriticalLogoutSheet(
    view: ViewModel,
    onStay: () -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val pending = view.pendingSyncCount.toInt()
    val xpAtRisk = view.xpIntoLevel
    BottomSheet(onDismiss = onStay, dismissible = !view.isSyncing) {
        Column(
            Modifier
                .fillMaxWidth()
                .drawBehind { drawLine(LognDark.warn, Offset(0f, 0f), Offset(size.width, 0f), Stroke.hairline.toPx()) }
                .padding(start = Space.screenMargin, end = Space.screenMargin, top = Space.screenMargin, bottom = Space.xl),
        ) {
            Text(
                Str.Logout.risk_summary(context, Str.Profile.queued_events(context, pending), xpAtRisk),
                style = LognFont.mono(ProfileMetrics.META_SIZE, tracking = ProfileMetrics.TAG_TRACKING),
                color = LognDark.warnInk,
            )
            Text(
                Str.Logout.save_before(context),
                style = LognFont.sans(NoticeMetrics.TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
                modifier = Modifier.padding(top = ProfileMetrics.gap9),
            )
            Text(
                Str.Logout.save_desc(context),
                style = LognFont.sans(NoticeMetrics.BODY_SIZE).merge(TextStyle(lineHeight = NoticeMetrics.BODY_LINE.sp)),
                color = LognDark.textSecondary,
                modifier = Modifier.padding(top = ProfileMetrics.gap9),
            )
            // Um evento só: quem decide sair é o Core, depois da fila subir.
            LognButton(
                Str.Logout.sync_and_leave(context),
                ButtonVariant.Primary,
                Modifier.padding(top = ProfileMetrics.gap18),
                loading = view.isSyncing,
            ) { dispatch(Event.SyncAndLogout) }
            LognButton(Str.Logout.stay_connected(context), ButtonVariant.Secondary, Modifier.padding(top = Space.sm), onClick = onStay)
            // Texto, não botão: a perda deixa de ser o caminho padrão. Descartar com o envio
            // no ar apagaria a fila que está subindo.
            LinkText(
                Str.Logout.leave_and_drop(context, xpAtRisk),
                if (view.isSyncing) LognDark.textDim else LognDark.wrongInk,
                Modifier
                    .fillMaxWidth()
                    .padding(top = Space.xs),
                enabled = !view.isSyncing,
            ) { dispatch(Event.Logout) }
        }
    }
}

/**
 * Gerenciar conta, uma tela empilhada. A exclusão vive aqui, um toque longe do logout:
 * encontrável, não proeminente. Vermelho só no bloco de confirmação.
 */
@Composable
fun ManageAccountScreen(
    view: ViewModel,
    onBack: () -> Unit,
) {
    val context = LocalContext.current
    var showsDelete by rememberSaveable { mutableStateOf(false) }
    BackHandler(onBack = onBack)
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.canvas),
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .drawBehind {
                    val y = size.height - Stroke.hairline.toPx() / 2
                    drawLine(LognDark.line, Offset(0f, y), Offset(size.width, y), Stroke.hairline.toPx())
                }.padding(start = Space.xs, end = Space.screenMargin, top = Space.md, bottom = Space.md),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val back = Str.Logout.back(context)
            Box(
                Modifier
                    .size(IconMetrics.touch)
                    .semantics { contentDescription = back }
                    .clickable(role = Role.Button, onClick = onBack),
                contentAlignment = Alignment.Center,
            ) { Icon(LognIcon.ChevronLeft, LognDark.textSecondary, IconMetrics.md) }
            Text(Str.Account.manage(context), style = LognFont.sans(ProfileMetrics.HEADER_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
        }
        Spacer(Modifier.weight(1f))
        val shape = RoundedCornerShape(Radius.sm)
        Column(
            Modifier
                .padding(Space.screenMargin)
                .fillMaxWidth()
                .background(LognDark.tintErr, shape)
                .border(Stroke.hairline, LognDark.wrong, shape)
                .padding(Space.lg),
        ) {
            Text(
                Str.Logout.grace_period(context),
                style = LognFont.mono(ProfileMetrics.META_SIZE, tracking = ProfileMetrics.TAG_TRACKING),
                color = LognDark.wrongInk,
            )
            Text(
                Str.Logout.delete_account(context),
                style = LognFont.sans(NoticeMetrics.CARD_TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
                modifier = Modifier.padding(top = Space.sm),
            )
            Text(
                Str.Logout.delete_desc(context, view.globalXp),
                style = LognFont.sans(ProfileMetrics.DESC_SIZE).merge(TextStyle(lineHeight = ProfileMetrics.DESC_LINE.sp)),
                color = LognDark.textSecondary,
                modifier = Modifier.padding(top = ProfileMetrics.gap7),
            )
            DestructiveButton(Str.Logout.delete_button(context), Modifier.padding(top = NoticeMetrics.cardPadding)) { showsDelete = true }
            Text(
                Str.Logout.delete_prompt(context),
                style = LognFont.mono(ProfileMetrics.META_SIZE),
                color = LognDark.textMuted,
                modifier =
                    Modifier
                        .align(Alignment.CenterHorizontally)
                        .padding(top = ProfileMetrics.gap9),
            )
        }
    }
    if (showsDelete) DeleteAccountSheet(view, onDismiss = { showsDelete = false })
}
