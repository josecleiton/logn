package sh.logn.app.ui.legal

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import sh.logn.app.AppLocale
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.FullScreenSheet
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LegalMetrics
import sh.logn.app.ui.theme.LoginMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.coreshell.i18n.Str
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Locale

/**
 * Termos de uso ou política de privacidade, em tela cheia. Espelho de
 * LegalDocumentView.swift: cabeçalho nativo com versão e vigência, o texto é a página do
 * servidor. O rodapé "Aceitar" só existe quando quem abre pede aceite.
 */
@Composable
fun LegalDocumentScreen(
    initialKind: LegalKind,
    onClose: () -> Unit,
    highlight: List<String> = emptyList(),
    onAccept: (() -> Unit)? = null,
) {
    FullScreenSheet(onDismiss = onClose) {
        LegalDocumentContent(initialKind, onClose, highlight, onAccept)
    }
}

@Composable
private fun LegalDocumentContent(
    initialKind: LegalKind,
    onClose: () -> Unit,
    highlight: List<String>,
    onAccept: (() -> Unit)?,
) {
    val context = LocalContext.current
    var kind by rememberSaveable { mutableStateOf(initialKind) }
    var meta by remember { mutableStateOf<LegalMeta?>(null) }
    var phase by remember { mutableStateOf(LegalLoadPhase.Loading) }
    var reloadToken by remember { mutableIntStateOf(0) }
    val locale = remember { AppLocale.current(context) }
    BackHandler(onBack = onClose)

    Column(Modifier.fillMaxSize().background(LognDark.surfaceRaised)) {
        Header(kind.title(context), meta, locale, onClose)
        Box(Modifier.weight(1f).fillMaxWidth()) {
            LegalWebView(
                kind = kind,
                locale = locale,
                highlight = if (kind == initialKind) highlight else emptyList(),
                reloadToken = reloadToken,
                onMeta = { meta = it },
                onPhase = { phase = it },
                onSwitch = { kind = it },
                modifier = Modifier.fillMaxSize().alpha(if (phase == LegalLoadPhase.Loaded) 1f else 0f),
            )
            when (phase) {
                LegalLoadPhase.Loading -> Skeleton()
                LegalLoadPhase.Failed -> Failure { reloadToken++ }
                LegalLoadPhase.Loaded -> Unit
            }
        }
        if (onAccept != null) {
            // Só se aceita o que está na tela: nem rascunho, nem cópia offline que pode
            // estar velha.
            val canAccept = phase == LegalLoadPhase.Loaded && meta?.let { !it.draft && !it.offline } == true
            Column(Modifier.fillMaxWidth().background(LognDark.surfaceRaised)) {
                Box(Modifier.fillMaxWidth().height(Stroke.hairline).background(LognDark.line))
                LognButton(
                    Str.Legal.accept(context),
                    ButtonVariant.Primary,
                    Modifier.padding(start = Space.screenMargin, end = Space.screenMargin, top = Space.lg, bottom = Space.md),
                    enabled = canAccept,
                    onClick = onAccept,
                )
            }
        }
    }
}

@Composable
private fun Header(
    title: String,
    meta: LegalMeta?,
    locale: String,
    onClose: () -> Unit,
) {
    val context = LocalContext.current
    Column(Modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().padding(start = Space.xs, end = Space.screenMargin, top = Space.sm, bottom = Space.md),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(Space.xs),
        ) {
            val back = Str.Logout.back(context)
            Box(
                Modifier
                    .size(IconMetrics.touch)
                    .semantics { contentDescription = back }
                    .clickable(role = Role.Button, onClick = onClose),
                contentAlignment = Alignment.Center,
            ) { Icon(LognIcon.ChevronLeft, LognDark.textSecondary, IconMetrics.md) }
            Column(verticalArrangement = Arrangement.spacedBy(Space.xs)) {
                Text(title, style = LognFont.sans(LegalMetrics.TITLE_SIZE, FontWeight.SemiBold), color = LognDark.textPrimary)
                MetaLine(meta, locale)
            }
        }
        Box(Modifier.fillMaxWidth().height(Stroke.hairline).background(LognDark.line))
    }
}

@Composable
private fun MetaLine(
    meta: LegalMeta?,
    locale: String,
) {
    val context = LocalContext.current
    if (meta == null) {
        // Esqueleto do tamanho da linha: o título não pula quando ela chega.
        Box(
            Modifier
                .padding(vertical = Space.xs)
                .width(LegalMetrics.metaSkeletonWidth)
                .height(LegalMetrics.metaSkeletonHeight)
                .background(LognDark.line, RoundedCornerShape(Radius.xs))
                .clearAndSetSemantics { },
        )
        return
    }
    val style = LognFont.mono(LegalMetrics.META_SIZE)
    // Uma linha cada: lado a lado, a data era cortada no meio em espanhol.
    Text(Str.Legal.version_effective(context, meta.version, formattedDate(meta.effectiveAt, locale)), style = style, color = LognDark.textMuted)
    if (meta.offline) Text(Str.Legal.offline_copy(context), style = style, color = LognDark.textMuted)
}

/** A vigência por extenso, na língua do app. A data vem pura, sem hora nem fuso. */
fun formattedDate(
    iso: String,
    locale: String,
): String =
    runCatching {
        LocalDate.parse(iso).format(DateTimeFormatter.ofLocalizedDate(FormatStyle.LONG).withLocale(Locale.forLanguageTag(locale)))
    }.getOrDefault(iso)

/** Carregando: linhas no lugar do texto, sem spinner, como pede o DS. */
@Composable
private fun Skeleton() {
    val label = Str.Legal.loading_accessibility(LocalContext.current)
    Column(
        Modifier
            .fillMaxSize()
            .background(LognDark.surfaceRaised)
            .padding(Space.screenMargin)
            .clearAndSetSemantics { contentDescription = label },
        verticalArrangement = Arrangement.spacedBy(Space.md),
    ) {
        for (i in 0 until SKELETON_LINES) {
            val title = i % SKELETON_GROUP == 0
            val short = i % SKELETON_GROUP == SKELETON_GROUP - 1
            if (title && i > 0) Spacer(Modifier.height(Space.lg))
            Box(
                Modifier
                    .then(if (short) Modifier.width(LegalMetrics.skeletonShort) else Modifier.fillMaxWidth())
                    .height(if (title) LegalMetrics.skeletonTitle else LegalMetrics.skeletonLine)
                    .background(LognDark.line, RoundedCornerShape(Radius.xs)),
            )
        }
    }
}

@Composable
private fun Failure(onRetry: () -> Unit) {
    val context = LocalContext.current
    Column(
        Modifier.fillMaxSize().background(LognDark.surfaceRaised).padding(horizontal = Space.xxl),
        verticalArrangement = Arrangement.spacedBy(Space.lg, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            Str.Legal.unavailable(context),
            style = LognFont.bodyMedium,
            color = LognDark.textSecondary,
            textAlign = TextAlign.Center,
        )
        LognButton(
            Str.Dashboard.try_again(context),
            ButtonVariant.Secondary,
            Modifier.widthIn(min = LegalMetrics.failureButtonWidth),
            onClick = onRetry,
        )
    }
}

/** "Termos de uso · Política de privacidade": dois links que abrem o documento. */
@Composable
fun LegalLinksRow(
    onOpen: (LegalKind) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    Row(
        modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        for ((index, kind) in LegalKind.entries.withIndex()) {
            if (index > 0) {
                Text(
                    DOT,
                    style = LognFont.sans(LegalMetrics.LINK_SIZE),
                    color = LognDark.textDim,
                    modifier = Modifier.clearAndSetSemantics { },
                )
            }
            Box(
                Modifier
                    .heightIn(min = LoginMetrics.linkMinHeight)
                    .clickable(role = Role.Button) { onOpen(kind) }
                    .padding(horizontal = Space.sm),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    kind.title(context),
                    style = LognFont.sans(LegalMetrics.LINK_SIZE).copy(textDecoration = TextDecoration.Underline),
                    color = LognDark.textSecondary,
                )
            }
        }
    }
}

private const val DOT = "·"
private const val SKELETON_LINES = 9
private const val SKELETON_GROUP = 4
