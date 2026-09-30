package sh.logn.app.ui.theme

import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import sh.logn.app.R

// IBM Plex, a mesma de iOS, empacotada em res/font (nunca de CDN). Em iOS o nome
// PostScript não bate com o do arquivo; aqui o recurso é por arquivo, e o peso vai no
// `Font`.

val PlexSans =
    FontFamily(
        Font(R.font.ibm_plex_sans_regular, FontWeight.Normal),
        Font(R.font.ibm_plex_sans_medium, FontWeight.Medium),
        Font(R.font.ibm_plex_sans_semibold, FontWeight.SemiBold),
    )

val PlexMono =
    FontFamily(
        Font(R.font.ibm_plex_mono_regular, FontWeight.Normal),
        Font(R.font.ibm_plex_mono_medium, FontWeight.Medium),
        Font(R.font.ibm_plex_mono_semibold, FontWeight.SemiBold),
    )

/** `LognFont` de iOS. Os tamanhos em `sp` acompanham a escala de fonte do sistema. */
object LognFont {
    val displayLarge = TextStyle(fontFamily = PlexSans, fontWeight = FontWeight.SemiBold, fontSize = 40.sp)
    val headlineMedium = TextStyle(fontFamily = PlexSans, fontWeight = FontWeight.SemiBold, fontSize = 24.sp)
    val titleMedium = TextStyle(fontFamily = PlexSans, fontWeight = FontWeight.SemiBold, fontSize = 19.sp)
    val bodyLarge = TextStyle(fontFamily = PlexSans, fontSize = 17.sp)
    val bodyMedium = TextStyle(fontFamily = PlexSans, fontSize = 15.sp)
    val code = TextStyle(fontFamily = PlexMono, fontSize = 15.sp)

    /** Plex Mono 11/500 com tracking +0.14em: o `lognLabel()` de iOS. */
    val label =
        TextStyle(fontFamily = PlexMono, fontWeight = FontWeight.Medium, fontSize = 11.sp, letterSpacing = 0.14.em)

    fun sans(
        size: Float,
        weight: FontWeight = FontWeight.Normal,
        tracking: Float = 0f,
    ) = TextStyle(fontFamily = PlexSans, fontWeight = weight, fontSize = size.sp, letterSpacing = tracking.em)

    fun mono(
        size: Float,
        weight: FontWeight = FontWeight.Normal,
        tracking: Float = 0f,
    ) = TextStyle(fontFamily = PlexMono, fontWeight = weight, fontSize = size.sp, letterSpacing = tracking.em)
}
