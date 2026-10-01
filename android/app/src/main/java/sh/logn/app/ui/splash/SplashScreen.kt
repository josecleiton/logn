package sh.logn.app.ui.splash

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.IntOffset
import sh.logn.app.ui.components.BrandSymbol
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.StatusGlyph
import sh.logn.app.ui.copy.checkLabel
import sh.logn.app.ui.copy.detailLabel
import sh.logn.app.ui.copy.glyph
import sh.logn.app.ui.copy.tone
import sh.logn.app.ui.theme.LocalReduceMotion
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.SplashMetrics
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.BootViewModel
import sh.logn.core.LogN.Event
import sh.logn.coreshell.i18n.Str

/**
 * A abertura do app: o juiz rodando os testes. Espelho de SplashView.swift.
 *
 * Enquanto o balão sobe, o Core confere a sessão e manda a fila, e cada verificação
 * imprime o seu veredito numa linha. Não há duração mínima: a splash some quando a última
 * linha fecha. Sem rede e com a sessão dentro do prazo, ela para e pergunta.
 */
@Composable
fun SplashScreen(
    boot: BootViewModel,
    dispatch: (Event) -> Unit,
) {
    val context = LocalContext.current
    val reduceMotion = LocalReduceMotion.current
    var risen by remember { mutableStateOf(reduceMotion) }
    LaunchedEffect(Unit) { risen = true }
    val rise by animateDpAsState(
        targetValue = if (risen) SplashMetrics.symbolRest else SplashMetrics.symbolRise,
        animationSpec = tween(if (reduceMotion) 0 else SplashMetrics.RISE_MILLIS),
        label = "rise",
    )

    Box(Modifier.fillMaxSize().background(LognDark.canvas)) {
        Column(
            Modifier
                .fillMaxSize()
                .safeDrawingPadding()
                .padding(horizontal = Space.screenMargin)
                .padding(bottom = Space.xl),
        ) {
            Spacer(Modifier.weight(1f))
            Column(
                Modifier
                    .fillMaxWidth()
                    .clearAndSetSemantics { contentDescription = Str.Boot.opening_accessibility(context) },
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(Space.md),
            ) {
                BrandSymbol(height = SplashMetrics.symbolHeight, modifier = Modifier.offset { IntOffset(0, rise.roundToPx()) })
                Text(
                    Str.App.name(context),
                    style = LognFont.sans(SplashMetrics.TITLE_SIZE, FontWeight.SemiBold, SplashMetrics.TITLE_TRACKING),
                    color = LognDark.textPrimary,
                )
            }
            Spacer(Modifier.weight(1f))

            BootLog(boot)

            AnimatedVisibility(
                visible = boot.awaitingOfflineChoice,
                enter = fadeIn(tween(if (reduceMotion) 0 else SplashMetrics.CHOICE_MILLIS)),
                exit = fadeOut(tween(if (reduceMotion) 0 else SplashMetrics.CHOICE_MILLIS)),
            ) {
                OfflineChoice(dispatch, Modifier.padding(top = Space.lg))
            }
            if (!boot.awaitingOfflineChoice) {
                ProgressBar(boot.progress.toInt(), reduceMotion, Modifier.padding(top = Space.md))
            }
        }
    }
}

@Composable
private fun BootLog(boot: BootViewModel) {
    val context = LocalContext.current
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(SplashMetrics.lineGap)) {
        for (line in boot.lines) {
            val check = line.checkLabel(context)
            val detail = line.detailLabel(context)
            Row(
                Modifier
                    .fillMaxWidth()
                    .clearAndSetSemantics { contentDescription = Str.Boot.line_accessibility(context, check, detail) },
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(Space.sm),
            ) {
                StatusGlyph(line.glyph, line.tone, SplashMetrics.lineIcon)
                Text(check, style = LognFont.mono(SplashMetrics.LINE_TEXT_SIZE), color = LognDark.textPrimary)
                Spacer(Modifier.weight(1f))
                Text(
                    detail,
                    style = LognFont.mono(SplashMetrics.LINE_TEXT_SIZE),
                    color = LognDark.textMuted,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}

@Composable
private fun ProgressBar(
    progress: Int,
    reduceMotion: Boolean,
    modifier: Modifier = Modifier,
) {
    val fraction by animateFloatAsState(
        targetValue = progress.coerceIn(0, 100) / 100f,
        animationSpec = tween(if (reduceMotion) 0 else SplashMetrics.PROGRESS_MILLIS),
        label = "progress",
    )
    Box(modifier.fillMaxWidth().height(SplashMetrics.progressHeight).background(LognDark.line)) {
        Box(Modifier.fillMaxHeight().fillMaxWidth(fraction).background(LognDark.accent))
    }
}

@Composable
private fun OfflineChoice(
    dispatch: (Event) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(Radius.sm)
    Column(modifier, verticalArrangement = Arrangement.spacedBy(Space.md)) {
        Text(
            Str.Boot.offline_notice(context),
            style = LognFont.sans(SplashMetrics.NOTICE_TEXT_SIZE),
            color = LognDark.textPrimary,
            modifier =
                Modifier
                    .fillMaxWidth()
                    .background(LognDark.tintWarn, shape)
                    .border(Stroke.hairline, LognDark.warn, shape)
                    .padding(Space.md),
        )
        Row(horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
            LognButton(Str.Boot.retry(context), ButtonVariant.Secondary, Modifier.weight(1f)) {
                dispatch(Event.RetryBoot)
            }
            LognButton(Str.Boot.continue_offline(context), ButtonVariant.Primary, Modifier.weight(1f)) {
                dispatch(Event.ContinueOffline)
            }
        }
    }
}
