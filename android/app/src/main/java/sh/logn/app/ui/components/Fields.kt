package sh.logn.app.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.contentType
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.toggleableState
import androidx.compose.ui.state.ToggleableState
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import sh.logn.app.ui.theme.FieldMetrics
import sh.logn.app.ui.theme.LoginMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Stroke

/** O que o campo recebe: muda o teclado e o preenchimento automático. */
enum class FieldKind { Email, Password, NewPassword, Number, Text }

/**
 * O campo de texto do DS: 52 de altura, `surface` com borda `line`, Plex Mono 14 e o
 * prompt em `textDim`. `error` troca a borda (a confirmação de senha que não bate).
 */
@Composable
fun LognTextField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    kind: FieldKind,
    modifier: Modifier = Modifier,
    error: Boolean = false,
    errorColor: Color = LognDark.wrong,
    textStyle: TextStyle = LognFont.mono(FieldMetrics.TEXT_SIZE),
    imeAction: ImeAction = ImeAction.Next,
    onDone: () -> Unit = {},
) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val shape = RoundedCornerShape(Radius.sm)
    val border =
        when {
            error -> errorColor
            focused -> LognDark.lineStrong
            else -> LognDark.line
        }
    BasicTextField(
        value = value,
        onValueChange = onValueChange,
        singleLine = true,
        interactionSource = interaction,
        textStyle = textStyle.copy(color = LognDark.textPrimary),
        cursorBrush = SolidColor(LognDark.accent),
        visualTransformation =
            if (kind == FieldKind.Password || kind == FieldKind.NewPassword) {
                PasswordVisualTransformation()
            } else {
                VisualTransformation.None
            },
        keyboardOptions =
            KeyboardOptions(
                capitalization = KeyboardCapitalization.None,
                autoCorrectEnabled = false,
                keyboardType =
                    when (kind) {
                        FieldKind.Email -> KeyboardType.Email
                        FieldKind.Password, FieldKind.NewPassword -> KeyboardType.Password
                        FieldKind.Number -> KeyboardType.NumberPassword
                        FieldKind.Text -> KeyboardType.Text
                    },
                imeAction = imeAction,
            ),
        keyboardActions = KeyboardActions(onDone = { onDone() }, onGo = { onDone() }),
        modifier =
            modifier
                .fillMaxWidth()
                .height(FieldMetrics.height)
                .semantics {
                    contentDescription = placeholder
                    // O preenchimento automático sabe o que é cada campo, e a senha nova
                    // do cadastro não é oferecida como a salva.
                    when (kind) {
                        FieldKind.Email -> contentType = ContentType.EmailAddress + ContentType.Username
                        FieldKind.Password -> contentType = ContentType.Password
                        FieldKind.NewPassword -> contentType = ContentType.NewPassword
                        FieldKind.Number, FieldKind.Text -> Unit
                    }
                },
        decorationBox = { inner ->
            Box(
                Modifier
                    .fillMaxWidth()
                    .height(FieldMetrics.height)
                    .background(LognDark.surface, shape)
                    .border(Stroke.hairline, border, shape)
                    .padding(horizontal = FieldMetrics.paddingH),
                contentAlignment = Alignment.CenterStart,
            ) {
                if (value.isEmpty()) Text(placeholder, style = textStyle, color = LognDark.textDim)
                inner()
            }
        },
    )
}

/**
 * A caixa de marcar das telas de cadastro: o quadrado do SF Symbols e o texto ao lado,
 * a linha inteira tocável.
 */
@Composable
fun LognCheckbox(
    checked: Boolean,
    label: String,
    modifier: Modifier = Modifier,
    onToggle: () -> Unit,
) {
    Row(
        modifier
            .fillMaxWidth()
            .semantics { toggleableState = ToggleableState(checked) }
            .clickable(role = Role.Checkbox, onClick = onToggle),
        horizontalArrangement = Arrangement.spacedBy(FieldMetrics.checkboxGap),
    ) {
        Icon(
            if (checked) LognIcon.CheckSquareFill else LognIcon.Square,
            color = if (checked) LognDark.accent else LognDark.textDim,
            size = FieldMetrics.checkbox,
            knockout = LognDark.canvas,
        )
        Text(label, style = LognFont.sans(LoginMetrics.CHECK_SIZE), color = LognDark.textSecondary)
    }
}
