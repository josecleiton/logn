package sh.logn.app.ui.auth

import android.content.ClipboardManager
import android.content.Context
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextAlign
import sh.logn.app.ui.LocalDispatch
import sh.logn.app.ui.components.ButtonVariant
import sh.logn.app.ui.components.Icon
import sh.logn.app.ui.components.LognButton
import sh.logn.app.ui.components.LognIcon
import sh.logn.app.ui.theme.FieldMetrics
import sh.logn.app.ui.theme.IconMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.core.LogN.Event
import sh.logn.core.LogN.ViewModel
import sh.logn.coreshell.i18n.Str

private const val OTP_LENGTH = 6

/**
 * Os seis campos do código, a colagem e a confirmação. Espelho de OTPInputView.swift.
 *
 * Um campo de verdade por trás das seis caixas: o teclado numérico, o preenchimento
 * automático do SMS/e-mail e o apagar funcionam como num campo só, e as caixas só
 * desenham o que ele tem. O código completo sobe uma vez por valor.
 */
@Composable
fun OtpInput(
    view: ViewModel,
    email: String,
    purpose: String,
    onCode: (String) -> Unit,
) {
    val context = LocalContext.current
    val dispatch = LocalDispatch.current
    var field by remember { mutableStateOf(TextFieldValue("")) }
    var lastSubmitted by rememberSaveable { mutableStateOf("") }
    val focus = remember { FocusRequester() }
    val verifyLocked = view.authCooldownSeconds > 0u
    val resendLocked = view.resendCooldownSeconds > 0u
    val code = field.text
    val canVerify = code.length == OTP_LENGTH && !verifyLocked

    // `force` passa por cima da guarda: depois de um 429 o mesmo código sobe de novo.
    fun submit(force: Boolean = false) {
        if (code.length != OTP_LENGTH || (!force && code == lastSubmitted) || verifyLocked) return
        lastSubmitted = code
        onCode(code)
        dispatch(Event.VerifyOTP(email, code, purpose))
    }

    fun set(text: String) {
        val digits = text.filter(Char::isDigit).take(OTP_LENGTH)
        field = TextFieldValue(digits, TextRange(digits.length))
    }

    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }
    LaunchedEffect(code) { if (code.length == OTP_LENGTH) submit() }

    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(Space.lg), horizontalAlignment = Alignment.CenterHorizontally) {
        BasicTextField(
            value = field,
            onValueChange = { set(it.text) },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
            cursorBrush = SolidColor(Color.Transparent),
            textStyle = LognFont.mono(FieldMetrics.OTP_TEXT_SIZE).copy(color = Color.Transparent),
            modifier = Modifier.focusRequester(focus),
            decorationBox = { inner ->
                Row(horizontalArrangement = Arrangement.spacedBy(Space.sm)) {
                    for (i in 0 until OTP_LENGTH) {
                        val focused = i == code.length.coerceAtMost(OTP_LENGTH - 1)
                        val label = Str.Otp.digit_accessibility(context, i + 1)
                        val shape = RoundedCornerShape(Radius.sm)
                        Box(
                            Modifier
                                .size(FieldMetrics.otpWidth, FieldMetrics.otpHeight)
                                .background(LognDark.surface, shape)
                                .border(
                                    if (focused) FieldMetrics.focusStroke else Stroke.hairline,
                                    if (focused) LognDark.accent else LognDark.line,
                                    shape,
                                ).semantics { contentDescription = label },
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(
                                code.getOrNull(i)?.toString().orEmpty(),
                                style = LognFont.mono(FieldMetrics.OTP_TEXT_SIZE, FontWeight.SemiBold),
                                color = LognDark.textPrimary,
                                textAlign = TextAlign.Center,
                            )
                        }
                    }
                }
                Box(Modifier.size(Space.xs)) { inner() }
            },
        )

        Row(
            Modifier
                .heightIn(min = Space.minTouch)
                .clickable(role = Role.Button) {
                    val pasted = clipboardText(context).filter(Char::isDigit)
                    if (pasted.length == OTP_LENGTH) set(pasted)
                },
            horizontalArrangement = Arrangement.spacedBy(Space.sm),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(LognIcon.Clipboard, LognDark.textSecondary, IconMetrics.md)
            Text(Str.Otp.paste_code(context), style = LognFont.label, color = LognDark.textSecondary)
        }

        LognButton(
            if (verifyLocked) Str.Status.wait_seconds(context, view.authCooldownSeconds.toInt()) else Str.Otp.verify(context),
            ButtonVariant.Primary,
            enabled = canVerify && !view.isAuthenticating,
        ) { submit(force = true) }

        LinkText(
            if (resendLocked) Str.Status.wait_seconds(context, view.resendCooldownSeconds.toInt()) else Str.Otp.resend_code(context),
            if (resendLocked) LognDark.textDim else LognDark.textSecondary,
            enabled = !resendLocked,
            underline = !resendLocked,
        ) { dispatch(Event.RequestOTP(email, purpose)) }
    }
}

private fun clipboardText(context: Context): String {
    val clipboard = context.getSystemService(ClipboardManager::class.java) ?: return ""
    return clipboard.primaryClip
        ?.getItemAt(0)
        ?.coerceToText(context)
        ?.toString()
        .orEmpty()
}
