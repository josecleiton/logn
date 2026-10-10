package sh.logn.app.ui.profile

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.sp
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.components.BottomSheet
import sh.logn.app.ui.copy.copy
import sh.logn.app.ui.leaderboard.leaderboardName
import sh.logn.app.ui.theme.LognColors
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.NicknameMetrics
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.NicknameStep
import sh.logn.core.LogN.StatusKey
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str
import sh.logn.app.ui.theme.Stroke as LognStroke

private const val MAX_NICKNAME = 20

/**
 * Escolher o apelido do placar (canvas "LogN — Placar geral de XP"): o campo, a
 * confirmação de que não dá para trocar depois, e o fim. Formato errado aparece no
 * "Continuar"; em uso e reservado, só depois de "Confirmar", e a folha volta ao campo.
 */
@Composable
fun NicknameSheet(
    view: ViewModel,
    onDismiss: () -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    val flow = view.nicknameFlow
    var text by rememberSaveable { mutableStateOf("") }
    LaunchedEffect(Unit) { dispatch(Event.NicknameStarted) }
    val close = {
        dispatch(Event.NicknameFlowClosed)
        onDismiss()
    }
    val anon = Str.Leaderboard.anon_name(context, view.profileAnonNumber.toString())
    BottomSheet(onDismiss = close, dismissible = !flow.submitting, background = LognColors.surface) {
        Column(
            Modifier
                .fillMaxWidth()
                .padding(
                    start = Space.screenMargin,
                    end = Space.screenMargin,
                    top = NicknameMetrics.paddingTop,
                    bottom = NicknameMetrics.paddingBottom,
                ),
            verticalArrangement = Arrangement.spacedBy(NicknameMetrics.gap),
        ) {
            Box(
                Modifier
                    .align(Alignment.CenterHorizontally)
                    .size(NicknameMetrics.handleWidth, NicknameMetrics.handleHeight)
                    .background(LognColors.lineStrong, RoundedCornerShape(NicknameMetrics.handleHeight)),
            )
            when (flow.step) {
                NicknameStep.INPUT -> InputStep(text, flow.error, anon, onChange = { text = it.take(MAX_NICKNAME) }) {
                    dispatch(Event.NicknameChecked(text))
                }
                NicknameStep.CONFIRM -> ConfirmStep(flow.draft, anon, flow.submitting)
                NicknameStep.DONE -> DoneStep(flow.draft, close)
            }
        }
    }
}

@Composable
private fun InputStep(
    text: String,
    error: StatusKey,
    anon: String,
    onChange: (String) -> Unit,
    onContinue: () -> Unit,
) {
    val context = LocalContext.current
    val hasError = error != StatusKey.SILENT
    val count = text.trim().length
    Column(verticalArrangement = Arrangement.spacedBy(NicknameMetrics.textGap)) {
        Text(Str.Nickname.title(context), style = titleStyle(), color = LognColors.textPrimary)
        Text(Str.Nickname.body(context, anon), style = bodyStyle(), color = LognColors.textSecondary)
    }
    Column(verticalArrangement = Arrangement.spacedBy(NicknameMetrics.textGap)) {
        Text(Str.Nickname.label(context), style = labelStyle(), color = LognColors.textMuted)
        val fieldLabel = Str.Nickname.field_accessibility(context)
        BasicTextField(
            value = text,
            onValueChange = onChange,
            singleLine = true,
            textStyle = LognFont.mono(NicknameMetrics.FIELD_SIZE).copy(color = LognColors.textPrimary),
            cursorBrush = SolidColor(LognColors.accentInk),
            keyboardOptions =
                KeyboardOptions(
                    capitalization = KeyboardCapitalization.None,
                    autoCorrectEnabled = false,
                    keyboardType = KeyboardType.Ascii,
                    imeAction = ImeAction.Done,
                ),
            modifier = Modifier.fillMaxWidth().semantics { contentDescription = fieldLabel },
            decorationBox = { inner ->
                val shape = RoundedCornerShape(Radius.sm)
                Box(
                    Modifier
                        .fillMaxWidth()
                        .height(NicknameMetrics.field)
                        .background(LognColors.canvas, shape)
                        .border(LognStroke.hairline, if (hasError) LognColors.warn else LognColors.lineStrong, shape)
                        .padding(horizontal = NicknameMetrics.fieldPaddingH),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    if (text.isEmpty()) {
                        // O exemplo dos outros campos é `textFaint`; este escrevia em `lineDim`,
                        // que no claro cai a 2:1. O escuro fica como estava (ADR 0030, escuro
                        // intacto).
                        val placeholder = if (LognColors.isLight) LognColors.textFaint else LognColors.lineDim
                        Text(Str.Nickname.placeholder(context), style = LognFont.mono(NicknameMetrics.FIELD_SIZE), color = placeholder)
                    }
                    inner()
                }
            },
        )
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
            Row(
                Modifier.weight(1f).padding(end = Space.md),
                horizontalArrangement = Arrangement.spacedBy(NicknameMetrics.errorIconGap),
            ) {
                if (hasError) {
                    sh.logn.app.ui.components.Icon(
                        sh.logn.app.ui.components.LognIcon.ExclamationCircle,
                        LognColors.warnInk,
                        NicknameMetrics.errorIcon,
                        Modifier.padding(top = NicknameMetrics.errorIconTop),
                    )
                }
                Text(
                    if (hasError) error.copy(context) ?: Str.Nickname.helper(context) else Str.Nickname.helper(context),
                    style = LognFont.sans(NicknameMetrics.HELPER_SIZE),
                    color = if (hasError) LognColors.warnInk else LognColors.textMuted,
                )
            }
            Text(
                Str.Nickname.counter(context, count, MAX_NICKNAME),
                style = LognFont.mono(NicknameMetrics.COUNTER_SIZE),
                color = LognColors.textMuted,
            )
        }
    }
    SheetButton(Str.Nickname.continue_action(context), filled = true, onClick = onContinue)
}

@Composable
private fun ConfirmStep(
    draft: String,
    anon: String,
    submitting: Boolean,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    Text(Str.Nickname.confirm_eyebrow(context), style = labelStyle(), color = LognColors.accentInk)
    val shape = RoundedCornerShape(Radius.sm)
    Column(
        Modifier
            .fillMaxWidth()
            .background(LognColors.accentTint, shape)
            .border(LognStroke.hairline, LognColors.accent, shape)
            .padding(horizontal = NicknameMetrics.cardPaddingH, vertical = NicknameMetrics.cardPaddingV),
        verticalArrangement = Arrangement.spacedBy(NicknameMetrics.cardGap),
    ) {
        Text(draft, style = LognFont.sans(NicknameMetrics.CARD_NAME_SIZE, FontWeight.SemiBold), color = LognColors.textPrimary)
        Text(Str.Nickname.confirm_sub(context, anon), style = LognFont.mono(NicknameMetrics.CARD_SUB_SIZE), color = LognColors.textSecondary)
    }
    Text(Str.Nickname.confirm_body(context), style = bodyStyle(), color = LognColors.textSecondary)
    Column(verticalArrangement = Arrangement.spacedBy(NicknameMetrics.buttonGap)) {
        SheetButton(Str.Nickname.confirm(context, draft), filled = true, enabled = !submitting) {
            dispatch(Event.NicknameSubmitted(draft))
        }
        SheetButton(Str.Nickname.back(context), filled = false, enabled = !submitting) { dispatch(Event.NicknameBack) }
    }
}

@Composable
private fun DoneStep(
    nickname: String,
    onClose: () -> Unit,
) {
    val context = LocalContext.current
    Column(verticalArrangement = Arrangement.spacedBy(NicknameMetrics.textGap)) {
        Text(Str.Nickname.success(context, nickname), style = titleStyle(), color = LognColors.textPrimary)
        Text(Str.Nickname.success_body(context), style = bodyStyle(), color = LognColors.textSecondary)
    }
    SheetButton(Str.Nickname.close(context), filled = false, onClick = onClose)
}

@Composable
private fun SheetButton(
    title: String,
    filled: Boolean,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    val shape = RoundedCornerShape(Radius.sm)
    val base =
        Modifier
            .fillMaxWidth()
            .height(NicknameMetrics.button)
    val styled =
        if (filled) {
            base.background(if (enabled) LognColors.accent else LognColors.buttonDisabled, shape)
        } else {
            base.border(LognStroke.hairline, LognColors.lineStrong, shape)
        }
    Box(styled.clickable(enabled = enabled, role = Role.Button, onClick = onClick), contentAlignment = Alignment.Center) {
        Text(
            title,
            style = LognFont.sans(NicknameMetrics.BUTTON_SIZE, if (filled) FontWeight.SemiBold else FontWeight.Medium),
            color =
                when {
                    !enabled -> LognColors.textDim
                    filled -> LognColors.onAccent
                    else -> LognColors.textPrimary
                },
        )
    }
}

/** O nome do placar no cabeçalho do Perfil: o apelido em Sans, o anônimo em mono. */
@Composable
fun ProfileName(view: ViewModel) {
    val context = LocalContext.current
    val nickname = view.profileNickname
    Text(
        leaderboardName(context, view.profileAnonNumber, nickname),
        style =
            if (nickname != null) {
                LognFont.sans(NicknameMetrics.NICKNAME_SIZE, FontWeight.SemiBold)
            } else {
                LognFont.mono(NicknameMetrics.ANON_SIZE, FontWeight.SemiBold)
            },
        color = LognColors.textPrimary,
        maxLines = 1,
    )
}

/** "Escolher apelido": contorno em acento, lápis à esquerda. */
@Composable
fun ChooseNicknameButton(onClick: () -> Unit) {
    val context = LocalContext.current
    val shape = RoundedCornerShape(Radius.sm)
    Row(
        Modifier
            .height(NicknameMetrics.chooseButton)
            .border(LognStroke.hairline, LognColors.accent, shape)
            .clickable(role = Role.Button, onClick = onClick)
            .padding(horizontal = NicknameMetrics.choosePaddingH),
        horizontalArrangement = Arrangement.spacedBy(NicknameMetrics.chooseGap),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        sh.logn.app.ui.components.Icon(sh.logn.app.ui.components.LognIcon.Pencil, LognColors.accentInk, NicknameMetrics.chooseIcon)
        Text(Str.Nickname.choose(context), style = LognFont.sans(NicknameMetrics.CHOOSE_SIZE, FontWeight.SemiBold), color = LognColors.accentInk)
    }
}

/** O avatar de quem ainda é "jogador #N": "#" em mono, sobre a superfície. */
@Composable
fun AnonAvatar(size: androidx.compose.ui.unit.Dp) {
    Box(
        Modifier
            .size(size)
            .background(LognColors.surface, androidx.compose.foundation.shape.CircleShape)
            .border(LognStroke.hairline, LognColors.lineStrong, androidx.compose.foundation.shape.CircleShape),
        contentAlignment = Alignment.Center,
    ) {
        Text("#", style = LognFont.mono(NicknameMetrics.ANON_AVATAR_SIZE), color = LognColors.textSecondary)
    }
}

private fun titleStyle(): TextStyle = LognFont.sans(NicknameMetrics.TITLE_SIZE, FontWeight.SemiBold)

private fun bodyStyle(): TextStyle = LognFont.sans(NicknameMetrics.BODY_SIZE).copy(lineHeight = NicknameMetrics.BODY_LINE.sp)

private fun labelStyle(): TextStyle = LognFont.mono(NicknameMetrics.LABEL_SIZE, tracking = NicknameMetrics.LABEL_TRACKING)
