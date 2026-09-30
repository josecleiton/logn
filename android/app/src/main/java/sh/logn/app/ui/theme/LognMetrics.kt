package sh.logn.app.ui.theme

import androidx.compose.ui.unit.dp

// `Space` e `Radius` de iOS, e as medidas que as telas de iOS escrevem soltas, reunidas
// por componente. Fora de ui/theme, o portão barra número com `.dp`/`.sp`.

object Space {
    val xxs = 2.dp
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

/** O símbolo da marca: abaixo desta largura o brilho vira ruído. */
object BrandMetrics {
    val shineMinWidth = 24.dp
    val lockupGap = 14.dp

    /** Símbolo de 58 ao lado de texto de 46, como no lockup do DS. */
    const val LOCKUP_SYMBOL_RATIO = 58f / 46f
    const val LOCKUP_TRACKING = -0.035f
}

object BalloonMetrics {
    val marqueeWidth = 13.dp
    val marqueeGap = 7.dp
    const val MARQUEE_ALPHA = 0.5f
}

/** Ícones de traço, na grade de 24 do DS. */
object IconMetrics {
    val sm = 12.dp
    val md = 16.dp
    val lg = 20.dp
    val xl = 34.dp
    val hero = 48.dp
    val touch = 44.dp
}

object FieldMetrics {
    val height = 52.dp
    val paddingH = 14.dp
    val focusStroke = 2.dp
    val checkbox = 18.dp
    val checkboxGap = 10.dp
    val otpWidth = 44.dp
    val otpHeight = 56.dp
    const val TEXT_SIZE = 14f
    const val OTP_TEXT_SIZE = 24f
    const val CODE_TEXT_SIZE = 20f
}

object LoginMetrics {
    val marqueeTop = 14.dp
    val marqueeToBrand = 35.dp
    val brandToForm = 40.dp
    val guestHeight = 46.dp
    val coreDot = 6.dp
    val linkMinHeight = 44.dp
    val dash = 4.dp
    const val BRAND_SIZE = 40f
    const val SLOGAN_SIZE = 11f
    const val SLOGAN_TRACKING = 0.16f
    const val DIVIDER_SIZE = 10f
    const val DIVIDER_TRACKING = 0.14f
    const val LINK_SIZE = 13.5f
    const val GUEST_SIZE = 14f
    const val GUEST_SUB_SIZE = 10.5f
    const val CORE_SIZE = 10f
    const val CORE_TRACKING = 0.1f
    const val PROVIDER_SIZE = 15f
    const val CHECK_SIZE = 13f
}

object ToastMetrics {
    val minHeight = 48.dp
    val radius = 24.dp
    val shadow = 15.dp
    val dismissDrag = 24.dp
    const val TEXT_SIZE = 15f
    const val PULSE_SCALE = 1.04f
    const val DRAG_RESISTANCE = 0.2f
}

object LegalMetrics {
    val metaSkeletonWidth = 180.dp
    val metaSkeletonHeight = 10.dp
    val skeletonTitle = 14.dp
    val skeletonLine = 10.dp
    val skeletonShort = 200.dp
    val failureButtonWidth = 160.dp
    const val TITLE_SIZE = 17f
    const val META_SIZE = 11f
    const val LINK_SIZE = 13f

    /** Teto do zoom de texto: o AX3 de iOS. */
    const val MAX_TEXT_ZOOM = 200
}

object TrackMetrics {
    val balloonHighlightMin = 24.dp
    val chipPaddingH = 6.dp
    val chipPaddingV = 2.dp
    val dayGap = 2.dp
    val dayHeight = 10.dp
    val dayRadius = 1.dp
    const val CHIP_SIZE = 10.5f
    const val DIMMED_ALPHA = 0.45f
}

/** A árvore: régua da exploração 4b (passo 88 entre colunas, cordinha de 28). */
object TreeMetrics {
    val columnPitch = 88.dp
    val topInset = 16.dp
    val bottomInset = 40.dp
    val stringGap = 28.dp
    val completed = 56.dp
    val active = 70.dp
    val locked = 54.dp
    val labelActive = 34.dp
    val label = 22.dp
    val labelPaddingH = 9.dp
    val labelPaddingV = 4.dp
    val labelRadius = 3.dp
    val degreeBadge = 18.dp
    val degreeOffsetX = 5.dp
    val degreeOffsetY = 2.dp
    val legendPaddingH = 18.dp
    val legendBottom = 14.dp
    val dash = 4.dp
    val dashGap = 6.dp
    val edgeWidth = 2.5.dp
    val activeEdgeWidth = 3.dp
    val checkIcon = 9.dp
    const val CHECK_STROKE = 3.5f
    const val LABEL_SIZE = 11.5f
    const val LABEL_SUB_SIZE = 9.5f
    const val LABEL_SUB_TRACKING = 0.08f
    const val COUNT_SIZE = 10f
    const val DEGREE_SIZE = 9f
    const val LEGEND_SIZE = 9.5f
    const val LEGEND_TRACKING = 0.12f
    const val ICON_RATIO = 0.36f
    const val ICON_RATIO_LOCKED = 0.34f
    const val ICON_CENTER = 0.42f
    const val ICON_STROKE_COMPLETED = 2.4f
    const val ICON_STROKE_ACTIVE = 2.2f
    const val EDGE_SLACK = 0.45f
    const val EDGE_ALPHA = 0.8f
}

object NodeSheetMetrics {
    val handleWidth = 36.dp
    val handleHeight = 3.dp
    val balloon = 56.dp
    val icon = 21.dp
    val neighbourIcon = 17.dp
    val problemBalloon = 36.dp
    val problemSlot = 44.dp
    val statPaddingH = 14.dp
    val statPaddingV = 12.dp
    val headerGap = 14.dp
    val neighbourGap = 9.dp
    const val TITLE_SIZE = 19f
    const val STATE_SIZE = 11f
    const val STATE_TRACKING = 0.1f
    const val COLUMN_SIZE = 10f
    const val COLUMN_TRACKING = 0.14f
    const val NEIGHBOUR_SIZE = 13.5f
    const val PROBLEM_LETTER_SIZE = 12f
    const val PROBLEM_CAPTION_SIZE = 9f
    const val STAT_LABEL_TRACKING = 0.12f
    const val STAT_VALUE_SIZE = 16f
    const val PROBLEM_OUTLINE = 6f
}

object HomeMetrics {
    val avatar = 40.dp
    val avatarBadge = 11.dp
    val avatarBadgeStroke = 2.dp
    val headerTop = 16.dp
    val headerBottom = 12.dp
    val trackBalloon = 16.dp
    val badgePaddingH = 5.dp
    val badgePaddingV = 2.dp
    val skeletonRow = 64.dp
    val navPaddingV = 13.dp
    val navIndicator = 2.dp
    val warnIcon = 11.dp
    val expiredBalloon = 34.dp
    val expiredTop = 48.dp
    val onboardingTop = 40.dp
    val onboardingSide = 22.dp
    val onboardingBottom = 26.dp
    val optionPadding = 14.dp
    val optionStroke = 1.5.dp
    const val HEADER_SIZE = 20f
    const val HEADER_TRACKING = -0.02f
    const val TRACK_NAME_SIZE = 17f
    const val META_SIZE = 10f
    const val BADGE_SIZE = 9.5f
    const val BADGE_TRACKING = 0.06f
    const val NAV_SIZE = 10.5f
    const val NAV_TRACKING = 0.1f
    const val AVATAR_TEXT_RATIO = 15f / 40f
    const val AVATAR_FILL_ALPHA = 0.18f
    const val ARENA_TITLE_SIZE = 21f
    const val ONBOARDING_TITLE_SIZE = 26f
    const val ONBOARDING_BODY_SIZE = 14.5f
    const val OPTION_TITLE_SIZE = 15.5f
    const val EXPIRED_TITLE_SIZE = 24f
    const val STAT_VALUE_SIZE = 17f
}

object RootMetrics {
    /** A faixa dos termos, acima da barra de abas (96 no iOS). */
    val bannerAboveTabs = 96.dp
}

object TermsMetrics {
    val sideMargin = 22.dp
    val arrowSlot = 20.dp
    val cellPaddingV = 11.dp
    val cellGap = 3.dp
    val signSlot = 22.dp
    val signBar = 2.dp
    val rowRadius = 3.dp
    val rowPaddingH = 10.dp
    val footerTop = 14.dp
    val box = 20.dp
    val boxRadius = 3.dp
    val boxStroke = 1.5.dp
    val declineTop = 28.dp
    val bannerPaddingStart = 14.dp
    const val EYEBROW_SIZE = 11f
    const val EYEBROW_TRACKING = 0.14f
    const val HEADLINE_SIZE = 23f
    const val CELL_LABEL_SIZE = 9.5f
    const val CELL_LABEL_TRACKING = 0.12f
    const val PLACE_TRACKING = 0.08f
    const val CELL_VALUE_SIZE = 17f
    const val CELL_DATE_SIZE = 10.5f
    const val LIST_LABEL_SIZE = 10f
    const val LIST_LABEL_TRACKING = 0.12f
    const val SIGN_SIZE = 14f
    const val BODY_SIZE = 13f
    const val CHECK_STROKE = 3f
}

object NoticeMetrics {
    val sideMargin = 34.dp
    val bottom = 28.dp
    val buttonHeight = 50.dp
    val cardPadding = 16.dp
    const val TITLE_SIZE = 18f
    const val BODY_SIZE = 14f
    const val BODY_LINE = 21f
    const val UNDO_SIZE = 11.5f
    const val CARD_TITLE_SIZE = 15f
    const val CARD_BODY_SIZE = 13.5f
    const val CARD_BODY_LINE = 20f
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
