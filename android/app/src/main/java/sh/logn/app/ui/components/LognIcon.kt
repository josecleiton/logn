package sh.logn.app.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Fill
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.Dp

/** Um traço do ícone: o caminho, e se ele é preenchido ou contornado. */
private class Part(
    val d: String,
    val filled: Boolean = false,
    /** Recorte: desenhado na cor do fundo por cima do preenchido (o "x" do círculo). */
    val knockout: Boolean = false,
)

/**
 * Os ícones do app, no lugar dos SF Symbols de iOS: traço de 2 na grade de 24, pontas
 * redondas, como os ícones de assunto do DS. Sem `material-icons-extended` (ADR 0023).
 */
enum class LognIcon(
    private vararg val parts: Part,
) {
    Close(Part("M6 6 L18 18 M18 6 L6 18")),

    // O `eye` e o `eye.slash` do campo de senha.
    Eye(Part("M2.5 12 C5 7.2 8.3 5 12 5 C15.7 5 19 7.2 21.5 12 C19 16.8 15.7 19 12 19 C8.3 19 5 16.8 2.5 12 Z M12 9 A3 3 0 1 1 11.99 9 Z")),
    EyeSlash(
        Part("M2.5 12 C5 7.2 8.3 5 12 5 C15.7 5 19 7.2 21.5 12 C19 16.8 15.7 19 12 19 C8.3 19 5 16.8 2.5 12 Z M12 9 A3 3 0 1 1 11.99 9 Z"),
        Part("M4 4 L20 20"),
    ),
    Check(Part("M5 12.5 L10 17.5 L19 7")),
    ChevronLeft(Part("M15 5 L8 12 L15 19")),
    ChevronRight(Part("M9 5 L16 12 L9 19")),
    ChevronDown(Part("M5 9 L12 16 L19 9")),
    ArrowRight(Part("M4 12 H20 M14 6 L20 12 L14 18")),
    ArrowClockwise(Part("M20 12 A8 8 0 1 1 17.66 6.34 M20 4 V9 H15")),
    ArrowCounterclockwise(Part("M4 12 A8 8 0 1 0 6.34 6.34 M4 4 V9 H9")),
    Heart(Part("M12 20 C12 20 3.5 14.5 3.5 8.8 C3.5 6 5.7 4 8.3 4 C10 4 11.3 5 12 6.2 C12.7 5 14 4 15.7 4 C18.3 4 20.5 6 20.5 8.8 C20.5 14.5 12 20 12 20 Z")),
    HeartFill(
        Part(
            "M12 20 C12 20 3.5 14.5 3.5 8.8 C3.5 6 5.7 4 8.3 4 C10 4 11.3 5 12 6.2 C12.7 5 14 4 15.7 4 C18.3 4 20.5 6 20.5 8.8 C20.5 14.5 12 20 12 20 Z",
            filled = true,
        ),
    ),
    Square(Part("M6 3.5 H18 Q20.5 3.5 20.5 6 V18 Q20.5 20.5 18 20.5 H6 Q3.5 20.5 3.5 18 V6 Q3.5 3.5 6 3.5 Z")),
    CheckSquareFill(
        Part("M6 3 H18 Q21 3 21 6 V18 Q21 21 18 21 H6 Q3 21 3 18 V6 Q3 3 6 3 Z", filled = true),
        Part("M7.5 12.5 L10.5 15.5 L16.5 8.5", knockout = true),
    ),
    CheckCircleFill(
        Part("M12 2 A10 10 0 1 1 11.99 2 Z", filled = true),
        Part("M7.5 12.5 L10.5 15.5 L16.5 8.5", knockout = true),
    ),
    XCircleFill(
        Part("M12 2 A10 10 0 1 1 11.99 2 Z", filled = true),
        Part("M8.5 8.5 L15.5 15.5 M15.5 8.5 L8.5 15.5", knockout = true),
    ),
    ExclamationCircleFill(
        Part("M12 2 A10 10 0 1 1 11.99 2 Z", filled = true),
        Part("M12 7 V13 M12 16.5 V16.6", knockout = true),
    ),
    ExclamationTriangleFill(
        Part("M12 3 L22 20.5 H2 Z", filled = true),
        Part("M12 9.5 V14 M12 17 V17.1", knockout = true),
    ),
    WifiSlash(Part("M2.5 8.5 A14 14 0 0 1 21.5 8.5 M5.5 12 A9.5 9.5 0 0 1 18.5 12 M8.8 15.5 A4.8 4.8 0 0 1 15.2 15.5 M12 19 V19.1 M3 3 L21 21")),
    Trash(Part("M4 7 H20 M9 7 V4 H15 V7 M6 7 L7 20 H17 L18 7 M10 11 V16 M14 11 V16")),
    TestTube(Part("M9 3 V17 A3 3 0 0 0 15 17 V3 M7.5 3 H16.5 M9 11 H15")),
    Box(Part("M12 3 L20.5 7.5 V16.5 L12 21 L3.5 16.5 V7.5 Z M3.5 7.5 L12 12 L20.5 7.5 M12 12 V21")),
    Logout(Part("M14 4 H6 V20 H14 M10 12 H21 M17 8 L21 12 L17 16")),
    Person(Part("M12 12 A4 4 0 1 0 11.99 12 Z M4.5 21 C4.5 16.8 7.8 14.5 12 14.5 C16.2 14.5 19.5 16.8 19.5 21")),
    PauseCircle(Part("M12 2.5 A9.5 9.5 0 1 1 11.99 2.5 Z M10 8.5 V15.5 M14 8.5 V15.5")),
    Lock(Part("M6 11 H18 V20.5 H6 Z M8.5 11 V7.5 A3.5 3.5 0 0 1 15.5 7.5 V11")),
    KeyFill(
        Part("M8 14.5 A5 5 0 1 1 7.99 14.5 Z M11.5 11 L20 2.5 M16.5 6 L19 8.5 M18.5 4 L21 6.5"),
        Part("M8 14.5 A5 5 0 1 1 7.99 14.5 Z", filled = true),
    ),
    Drive(Part("M3 13 L6 5 H18 L21 13 V19 H3 Z M3 13 H21 M16.5 16 H16.6")),
    Clipboard(Part("M9 4 H6 V21 H18 V4 H15 M9 3 H15 V6 H9 Z")),
    ClockBack(Part("M4.5 12 A7.5 7.5 0 1 0 6.7 6.7 M3.5 4.5 V9 H8 M12 8 V12 L15 14")),
    ClockFill(
        Part("M12 2 A10 10 0 1 1 11.99 2 Z", filled = true),
        Part("M12 7 V12 L15.5 14", knockout = true),
    ),
    ;

    internal val drawParts: List<Pair<String, Int>>
        get() =
            parts.map { part ->
                val kind =
                    when {
                        part.knockout -> KNOCKOUT
                        part.filled -> FILL
                        else -> STROKE
                    }
                part.d to kind
            }

    internal companion object {
        const val STROKE = 0
        const val FILL = 1
        const val KNOCKOUT = 2
        const val GRID = 24f
        const val STROKE_WIDTH = 2f
    }
}

/**
 * Desenha o ícone. Decorativo por padrão: quem precisa de rótulo o põe no alvo de toque,
 * não aqui. `knockout` é a cor do fundo sobre a qual o ícone cheio aparece.
 */
@Composable
fun Icon(
    icon: LognIcon,
    color: Color,
    size: Dp,
    modifier: Modifier = Modifier,
    knockout: Color = Color.Black,
    strokeWidth: Float = LognIcon.STROKE_WIDTH,
) {
    val paths = remember(icon) { icon.drawParts.map { (d, kind) -> PathParser().parsePathString(d).toPath() to kind } }
    Canvas(modifier.size(size).clearAndSetSemantics { }) {
        val s = this.size.width / LognIcon.GRID
        val stroke = Stroke(strokeWidth, cap = StrokeCap.Round, join = StrokeJoin.Round)
        scale(s, s, pivot = Offset.Zero) {
            for ((path, kind) in paths) {
                when (kind) {
                    LognIcon.FILL -> drawPath(path, color, style = Fill)
                    LognIcon.KNOCKOUT -> drawPath(path, knockout, style = stroke)
                    else -> drawPath(path, color, style = stroke)
                }
            }
        }
    }
}
