package sh.logn.app.ui.theme

import androidx.compose.ui.unit.dp

// `Space` e `Radius` de iOS, e as medidas que as telas de iOS escrevem soltas, reunidas
// por componente. Fora de ui/theme, o portão barra número com `.dp`/`.sp`.

object Space {
    val xs = 4.dp
    val sm = 8.dp
    val md = 12.dp
    val lg = 16.dp
    val xl = 24.dp
    val xxl = 32.dp
    val xxxl = 48.dp
    val screenMargin = 20.dp
    val listGap = 8.dp

    /** Piso de toque em Android (Material): 48 dp. */
    val minTouch = 48.dp

    /** Alvos em partida. */
    val matchTouch = 56.dp
}

object Radius {
    val xs = 2.dp
    val sm = 4.dp
    val md = 8.dp
}

object Stroke {
    val hairline = 1.dp
}

object ButtonMetrics {
    val height = 52.dp
    val restOffset = 0.dp
    val pressedOffset = 1.dp
    val spinner = 16.dp
    val spinnerStroke = 2.dp
    const val TEXT_SIZE = 15f
}

object SplashMetrics {
    val symbolHeight = 72.dp
    val symbolRise = 28.dp
    val symbolRest = 0.dp
    const val TITLE_SIZE = 28f
    const val TITLE_TRACKING = -0.035f
    const val LINE_TEXT_SIZE = 12f
    const val NOTICE_TEXT_SIZE = 13f
    val lineGap = 6.dp
    val lineIcon = 12.dp
    val progressHeight = 2.dp
    const val RISE_MILLIS = 900
    const val PROGRESS_MILLIS = 300
    const val CHOICE_MILLIS = 200
}
