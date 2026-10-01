package sh.logn.app.ui.notice

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.sp
import sh.logn.app.AppLocale
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.NoticeMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Locale

private fun body(line: Float = NoticeMetrics.BODY_LINE) = LognFont.sans(NoticeMetrics.BODY_SIZE).merge(TextStyle(lineHeight = line.sp))

/** O molde das telas de aviso: ícone, título e texto no meio, ações no pé. */
@Composable
private fun NoticeScaffold(
    icon: LognIcon,
    title: String,
    text: String,
    actions: @Composable () -> Unit,
) {
    Column(Modifier.fillMaxSize().background(LognDark.canvas).safeDrawingPadding()) {
        Column(
            Modifier.weight(1f).fillMaxWidth().padding(horizontal = NoticeMetrics.sideMargin),
            verticalArrangement = Arrangement.spacedBy(Space.lg, Alignment.CenterVertically),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Icon(icon, LognDark.textMuted, IconMetrics.xl, strokeWidth = LIGHT_STROKE)
            Text(
                title,
                style = LognFont.sans(NoticeMetrics.TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
                textAlign = TextAlign.Center,
            )
            Text(text, style = body(), color = LognDark.textSecondary, textAlign = TextAlign.Center)
        }
        Column(Modifier.padding(start = Space.screenMargin, end = Space.screenMargin, bottom = NoticeMetrics.bottom)) { actions() }
    }
}

/**
 * Saída direta, sem nada na fila: despedida com desfazer, no lugar de um alerta de
 * confirmação. Espelho de LogoutNoticeView.
 */
@Composable
fun LogoutNoticeScreen(view: ViewModel) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    NoticeScaffold(
        LognIcon.Logout,
        if (view.displayName.isEmpty()) Str.Logout.see_you(context) else Str.Logout.see_you_name(context, view.displayName),
        Str.Logout.server_safe(context, view.globalXp),
    ) {
        LognButton(Str.Logout.sign_in_again(context), ButtonVariant.Primary) { dispatch(Event.DismissLogoutNotice) }
        Row(
            Modifier
                .fillMaxWidth()
                .padding(top = Space.xs)
                .heightIn(min = IconMetrics.touch)
                .clickable(role = Role.Button) { dispatch(Event.UndoLogout) },
            horizontalArrangement = Arrangement.spacedBy(Space.sm, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(LognIcon.ArrowCounterclockwise, LognDark.textMuted, IconMetrics.sm)
            Text(Str.Logout.undo_logout(context), style = LognFont.mono(NoticeMetrics.UNDO_SIZE), color = LognDark.textSecondary)
        }
    }
}

/**
 * Depois de pedir a exclusão: a conta está desativada, e diz até quando entrar ainda a
 * traz de volta. Espelho de DeletionNoticeView.
 */
@Composable
fun DeletionNoticeScreen(view: ViewModel) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val date = longDate(view.deletionPurgeAfter, AppLocale.current(context))
    NoticeScaffold(LognIcon.ClockBack, Str.Logout.deleted_title(context), Str.Logout.deleted_body(context, date)) {
        LognButton(Str.Logout.notice_ok(context), ButtonVariant.Secondary) { dispatch(Event.DismissDeletionNotice) }
    }
}

/** Entrou dentro da carência: a exclusão foi cancelada. Aparece uma vez, por cima do jogo. */
@Composable
fun AccountRestoredCard(modifier: Modifier = Modifier) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val shape = RoundedCornerShape(Radius.md)
    Box(modifier.fillMaxWidth().padding(start = Space.screenMargin, end = Space.screenMargin, bottom = Space.xl)) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(LognDark.surfaceRaised, shape)
                .border(Stroke.hairline, LognDark.lineStrong, shape)
                .padding(NoticeMetrics.cardPadding),
        ) {
            Text(
                Str.Logout.restored_title(context),
                style = LognFont.sans(NoticeMetrics.CARD_TITLE_SIZE, FontWeight.SemiBold),
                color = LognDark.textPrimary,
            )
            Text(
                Str.Logout.restored_body(context),
                style = LognFont.sans(NoticeMetrics.CARD_BODY_SIZE).merge(TextStyle(lineHeight = NoticeMetrics.CARD_BODY_LINE.sp)),
                color = LognDark.textSecondary,
                modifier = Modifier.padding(top = Space.sm),
            )
            LognButton(Str.Logout.notice_ok(context), ButtonVariant.Secondary, Modifier.padding(top = Space.md)) {
                dispatch(Event.DismissAccountRestoredNotice)
            }
        }
    }
}

/** Um instante do Core (segundos) por extenso, na língua do app, no fuso do aparelho. */
fun longDate(
    epochSeconds: Long,
    locale: String,
): String =
    DateTimeFormatter
        .ofLocalizedDate(FormatStyle.LONG)
        .withLocale(Locale.forLanguageTag(locale))
        .format(Instant.ofEpochSecond(epochSeconds).atZone(ZoneId.systemDefault()))

private const val LIGHT_STROKE = 1.4f
