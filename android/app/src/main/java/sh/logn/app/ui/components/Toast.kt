package sh.logn.app.ui.components

import android.os.Build
import android.view.HapticFeedbackConstants
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.scale
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.IntOffset
import kotlinx.coroutines.delay
import sh.logn.app.ui.theme.LocalReduceMotion
import sh.logn.app.ui.theme.LognDark
import sh.logn.app.ui.theme.LognFont
import sh.logn.app.ui.theme.Space
import sh.logn.app.ui.theme.Stroke
import sh.logn.app.ui.theme.ToastMetrics
import sh.logn.coreshell.i18n.Str
import kotlin.math.roundToInt

enum class ToastVariant(
    val icon: LognIcon,
    val tint: Color,
    /** Erro fica mais porque costuma pedir releitura; sucesso é o mais curto. */
    val millis: Long,
) {
    Attention(LognIcon.ExclamationCircleFill, LognDark.warn, 3_500),
    Error(LognIcon.XCircleFill, LognDark.wrong, 5_000),
    Success(LognIcon.CheckCircleFill, LognDark.correct, 2_500),
}

data class ToastMessage(
    val id: String,
    val variant: ToastVariant,
    val text: String,
    val icon: LognIcon? = null,
)

/**
 * Aviso curto do topo, um por vez, sem pilha. Espelho de Toast.swift: com o mesmo `id`,
 * o aviso só pulsa e reinicia o tempo; com outro, substitui o da tela.
 */
object ToastCenter {
    var current by mutableStateOf<ToastMessage?>(null)
        private set

    /** Muda a cada repetição do mesmo aviso: a pílula pulsa. */
    var pulse by mutableIntStateOf(0)
        private set

    /** Muda a cada vez que o relógio recomeça. */
    var clock by mutableIntStateOf(0)
        private set

    var holding by mutableStateOf(false)
        private set

    /** Soltou sem dispensar: sobra pouco tempo, que a pessoa já leu. */
    var shortClock by mutableStateOf(false)
        private set

    fun show(
        variant: ToastVariant,
        text: String,
        id: String,
        icon: LognIcon? = null,
    ) {
        if (current?.id == id) pulse++ else current = ToastMessage(id, variant, text, icon)
        shortClock = false
        clock++
    }

    fun dismiss() {
        current = null
        holding = false
    }

    fun hold() {
        holding = true
    }

    fun release() {
        holding = false
        shortClock = true
        clock++
    }
}

/** A pílula por cima de tudo, no topo. */
@Composable
fun ToastHost(modifier: Modifier = Modifier) {
    val message = ToastCenter.current
    val reduceMotion = LocalReduceMotion.current
    val view = LocalView.current
    val context = LocalContext.current
    val density = LocalDensity.current
    var dragY by remember { mutableFloatStateOf(0f) }
    var pulsing by remember { mutableStateOf(false) }
    val scale by animateFloatAsState(if (pulsing) ToastMetrics.PULSE_SCALE else 1f, tween(PULSE_MILLIS), label = "pulse")

    // Entrada: vibração e anúncio uma vez por aviso novo.
    LaunchedEffect(message?.id) {
        val m = message ?: return@LaunchedEffect
        dragY = 0f
        val haptic =
            when {
                Build.VERSION.SDK_INT < Build.VERSION_CODES.R -> HapticFeedbackConstants.KEYBOARD_TAP
                m.variant == ToastVariant.Success -> HapticFeedbackConstants.CONFIRM
                else -> HapticFeedbackConstants.REJECT
            }
        view.performHapticFeedback(haptic)
        val spoken =
            when (m.variant) {
                ToastVariant.Attention -> Str.Toast.attention(context)
                ToastVariant.Error -> Str.Toast.error(context)
                ToastVariant.Success -> Str.Toast.success(context)
            }
        @Suppress("DEPRECATION")
        view.announceForAccessibility("$spoken. ${m.text}")
    }
    LaunchedEffect(ToastCenter.pulse) {
        if (ToastCenter.pulse == 0 || reduceMotion) return@LaunchedEffect
        pulsing = true
        delay(PULSE_MILLIS.toLong())
        pulsing = false
    }
    // O relógio: reinicia a cada repetição e para enquanto o dedo segura.
    LaunchedEffect(message?.id, ToastCenter.clock, ToastCenter.holding) {
        val m = message ?: return@LaunchedEffect
        if (ToastCenter.holding) return@LaunchedEffect
        val extra = if (m.text.length > LONG_TEXT) LONG_EXTRA_MILLIS else 0L
        delay(if (ToastCenter.shortClock) RELEASE_MILLIS else m.variant.millis + extra)
        ToastCenter.dismiss()
    }

    Box(modifier.fillMaxWidth().statusBarsPadding().padding(horizontal = Space.screenMargin, vertical = Space.xs)) {
        AnimatedVisibility(
            visible = message != null,
            enter = if (reduceMotion) fadeIn() else slideInVertically { -it } + scaleIn(initialScale = ENTER_SCALE) + fadeIn(),
            exit = if (reduceMotion) fadeOut() else slideOutVertically { -it } + fadeOut(),
            modifier = Modifier.align(Alignment.TopCenter),
        ) {
            val shown = message ?: return@AnimatedVisibility
            val shape = RoundedCornerShape(ToastMetrics.radius)
            val dismissPx = with(density) { ToastMetrics.dismissDrag.toPx() }
            Row(
                Modifier
                    .offset { IntOffset(0, dragY.roundToInt()) }
                    .scale(scale)
                    .shadow(ToastMetrics.shadow, shape)
                    .background(LognDark.surfaceRaised, shape)
                    .border(Stroke.hairline, LognDark.lineStrong, shape)
                    .heightIn(min = ToastMetrics.minHeight)
                    .padding(start = Space.md, end = Space.lg, top = Space.md, bottom = Space.md)
                    // O texto já saiu como anúncio; a pílula some antes de alguém chegar nela.
                    .clearAndSetSemantics { }
                    .pointerInput(shown.id) {
                        detectVerticalDragGestures(
                            onDragStart = { ToastCenter.hold() },
                            onDragEnd = {
                                if (dragY < -dismissPx) ToastCenter.dismiss() else ToastCenter.release()
                                dragY = 0f
                            },
                            onDragCancel = {
                                dragY = 0f
                                ToastCenter.release()
                            },
                        ) { _, dy ->
                            val next = dragY + dy
                            dragY = if (next > 0) next * ToastMetrics.DRAG_RESISTANCE else next
                        }
                    },
                horizontalArrangement = Arrangement.spacedBy(Space.sm),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(shown.icon ?: shown.variant.icon, shown.variant.tint, IconSize, knockout = LognDark.surfaceRaised)
                Text(shown.text, style = LognFont.sans(ToastMetrics.TEXT_SIZE, FontWeight.Medium), color = LognDark.textPrimary)
            }
        }
    }
}

private val IconSize = sh.logn.app.ui.theme.IconMetrics.lg
private const val PULSE_MILLIS = 160
private const val ENTER_SCALE = 0.9f

/** Uns 32 caracteres cabem numa linha; a segunda ganha 1 s. */
private const val LONG_TEXT = 32
private const val LONG_EXTRA_MILLIS = 1_000L
private const val RELEASE_MILLIS = 1_500L
