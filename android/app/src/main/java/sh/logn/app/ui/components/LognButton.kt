package sh.logn.app.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.IntOffset
import sh.logn.app.ui.theme.ButtonMetrics
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Radius
import sh.logn.app.ui.theme.Stroke

enum class ButtonVariant { Primary, Secondary, Ghost }

/**
 * `LognButton` de iOS: 52 dp, raio 4, primário SemiBold e os outros Medium. Pressionado,
 * afunda 1 dp, sem sombra nem escala: a hierarquia vem de linha e luminância.
 */
@Composable
fun LognButton(
    title: String,
    variant: ButtonVariant,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    loading: Boolean = false,
    onClick: () -> Unit,
) {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    val shape = RoundedCornerShape(Radius.sm)
    val foreground =
        when {
            !enabled -> LognDark.textDim
            variant == ButtonVariant.Primary -> LognDark.onAccent
            variant == ButtonVariant.Secondary -> LognDark.textPrimary
            else -> LognDark.textSecondary
        }
    val background =
        when {
            !enabled -> LognDark.buttonDisabled
            variant == ButtonVariant.Primary -> LognDark.accent
            else -> Color.Transparent
        }
    Box(
        modifier
            .fillMaxWidth()
            .height(ButtonMetrics.height)
            .offset { IntOffset(0, (if (pressed) ButtonMetrics.pressedOffset else ButtonMetrics.restOffset).roundToPx()) }
            .background(background, shape)
            .then(
                if (variant == ButtonVariant.Secondary) {
                    Modifier.border(Stroke.hairline, LognDark.lineStrong, shape)
                } else {
                    Modifier
                },
            ).clickable(
                interactionSource = interaction,
                indication = null,
                enabled = enabled && !loading,
                role = Role.Button,
                onClick = onClick,
            ),
        contentAlignment = Alignment.Center,
    ) {
        if (loading) {
            CircularProgressIndicator(
                color = foreground,
                strokeWidth = ButtonMetrics.spinnerStroke,
                modifier = Modifier.size(ButtonMetrics.spinner),
            )
        } else {
            Text(
                title,
                style =
                    LognFont.sans(
                        ButtonMetrics.TEXT_SIZE,
                        if (variant == ButtonVariant.Primary) FontWeight.SemiBold else FontWeight.Medium,
                    ),
                color = foreground,
            )
        }
    }
}
