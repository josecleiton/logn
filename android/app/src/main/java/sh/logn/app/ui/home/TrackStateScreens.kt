package sh.logn.app.ui.home

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.auth.LinkText
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
import sh.logn.app.ui.theme.Stroke
import sh.logn.app.ui.theme.TermsMetrics
import sh.logn.app.ui.track.TrackBalloon
import sh.logn.app.ui.track.freeTrack
import sh.logn.app.ui.tree.StatCell
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.TrackView
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

/**
 * "Por onde começar?", uma tela só, na primeira abertura. Espelho de OnboardingView.
 * Nenhum preço aqui: quem é novo cai na grátis; quem já sabe vai ao catálogo.
 */
@Composable
fun OnboardingScreen(
    view: ViewModel,
    onFinish: (wantsCatalog: Boolean) -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    var wantsCatalog by rememberSaveable { mutableStateOf(false) }
    val free = view.tracks.freeTrack()
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.surface)
            .safeDrawingPadding()
            .padding(start = HomeMetrics.onboardingSide, end = HomeMetrics.onboardingSide, bottom = HomeMetrics.onboardingBottom),
    ) {
        Column(Modifier.padding(top = HomeMetrics.onboardingTop), verticalArrangement = Arrangement.spacedBy(Space.md)) {
            Text(
                Str.Onboarding.title(context),
                style = LognFont.sans(HomeMetrics.ONBOARDING_TITLE_SIZE, FontWeight.SemiBold, HomeMetrics.HEADER_TRACKING),
                color = LognDark.textPrimary,
            )
            Text(Str.Onboarding.body(context), style = LognFont.sans(HomeMetrics.ONBOARDING_BODY_SIZE), color = LognDark.textSecondary)
        }
        Column(Modifier.padding(top = TermsMetrics.sideMargin), verticalArrangement = Arrangement.spacedBy(Space.md)) {
            Option(!wantsCatalog, free?.name.orEmpty(), Str.Onboarding.free_body(context)) { wantsCatalog = false }
            Option(wantsCatalog, Str.Onboarding.know_title(context), Str.Onboarding.know_body(context)) { wantsCatalog = true }
        }
        Spacer(Modifier.weight(1f))
        LognButton(Str.Onboarding.start(context), ButtonVariant.Primary) {
            dispatch(Event.CompleteOnboarding)
            onFinish(wantsCatalog)
        }
    }
}

@Composable
private fun Option(
    selected: Boolean,
    title: String,
    body: String,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        Modifier
            .fillMaxWidth()
            .background(if (selected) LognDark.accentTint else Color.Transparent, shape)
            .border(if (selected) HomeMetrics.optionStroke else Stroke.hairline, if (selected) LognDark.accent else LognDark.line, shape)
            .semantics { this.selected = selected }
            .clickable(role = Role.RadioButton, onClick = onClick)
            .padding(HomeMetrics.optionPadding),
        horizontalArrangement = Arrangement.spacedBy(Space.md),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(TermsMetrics.cellGap)) {
            Text(title, style = LognFont.sans(HomeMetrics.OPTION_TITLE_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
            Text(body, style = LognFont.sans(TermsMetrics.BODY_SIZE), color = LognDark.textSecondary)
        }
        if (selected) Icon(LognIcon.Check, LognDark.accentInk, IconMetrics.md)
    }
}

/**
 * A trilha comprada cuja licença offline venceu. Estado vazio dentro da trilha, não um
 * modal: a saída secundária leva ao que ainda abre. Espelho de ExpiredTrackView.
 */
@Composable
fun ExpiredTrackScreen(
    view: ViewModel,
    track: TrackView,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val free = view.tracks.freeTrack()
    Column(
        Modifier
            .fillMaxSize()
            .padding(start = HomeMetrics.onboardingSide, end = HomeMetrics.onboardingSide, top = HomeMetrics.expiredTop, bottom = Space.lg),
        verticalArrangement = Arrangement.spacedBy(TermsMetrics.footerTop),
    ) {
        TrackBalloon(track, HomeMetrics.expiredBalloon, dimmed = true)
        Text(
            Str.Expired.eyebrow(context, track.daysSinceContact.toInt()),
            style = LognFont.mono(TermsMetrics.CELL_DATE_SIZE, tracking = TermsMetrics.LIST_LABEL_TRACKING),
            color = LognDark.wrongInk,
        )
        Text(
            Str.Expired.title(context, track.name),
            style = LognFont.sans(HomeMetrics.EXPIRED_TITLE_SIZE, FontWeight.SemiBold, HomeMetrics.HEADER_TRACKING),
            color = LognDark.textPrimary,
        )
        Text(Str.Expired.body(context), style = LognFont.sans(TermsMetrics.BODY_SIZE + 1), color = LognDark.textSecondary)
        val shape = RoundedCornerShape(Radius.sm)
        Row(
            Modifier
                .fillMaxWidth()
                .height(IntrinsicSize.Min)
                .background(LognDark.line, shape)
                .border(Stroke.hairline, LognDark.line, shape),
            horizontalArrangement = Arrangement.spacedBy(Stroke.hairline),
        ) {
            StatCell(
                Str.Expired.progress(context),
                "${track.nodesDone}/${track.nodeCount}",
                Modifier.weight(1f),
                HomeMetrics.STAT_VALUE_SIZE,
                FontWeight.SemiBold,
            )
            StatCell(Str.Expired.xp(context), "${track.trackXp}", Modifier.weight(1f), HomeMetrics.STAT_VALUE_SIZE, FontWeight.SemiBold)
        }
        Spacer(Modifier.weight(1f))
        LognButton(Str.Expired.retry(context), ButtonVariant.Primary) { dispatch(Event.FetchLicense(track.id)) }
        if (free != null) {
            LinkText(Str.Expired.back(context, free.name), LognDark.textSecondary, Modifier.fillMaxWidth()) {
                dispatch(Event.SelectTrack(free.id))
            }
        }
    }
}
