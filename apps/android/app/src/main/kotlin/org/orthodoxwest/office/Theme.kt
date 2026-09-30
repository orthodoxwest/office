package org.orthodoxwest.office

import androidx.annotation.DrawableRes
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.LineHeightStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.sp

/**
 * The web's design tokens (apps/office-web/static/style.css): Nave is its `:root`, Apse its
 * `[data-theme="dark"]` block. [TokensTest] checks each value against the stylesheet, so a
 * retuned token there fails here until the app follows it.
 */
@Immutable
data class Palette(
    val dark: Boolean,
    val text: Color,
    val bg: Color,
    val accent: Color,
    val gold: Color,
    val goldLine: Color,
    val muted: Color,
    val border: Color,
    val rubric: Color,
    val surface: Color,
    val surfaceEdge: Color,
    val pressedWash: Color,
    val oakLine: Color,
    val inscriptionGround: Color,
    val inscriptionEdge: Color,
    val inscriptionWash: Color,
    @param:DrawableRes val plaster: Int,
)

val Nave = Palette(
    dark = false,
    text = Color(0xFF241C17),
    bg = Color(0xFFFAF3E9),
    accent = Color(0xFF6B3A1F),
    gold = Color(0xFF9A7328),
    goldLine = Color(0xFFC9AC72),
    muted = Color(0xFF6B5D54),
    border = Color(0xFFDCCFC3),
    rubric = Color(0xFF8B1A1A),
    surface = Color(0xFFF6EDDF),
    surfaceEdge = Color(107, 58, 31).copy(alpha = 0.12f),
    pressedWash = Color(107, 58, 31).copy(alpha = 0.09f),
    oakLine = Color(63, 55, 47).copy(alpha = 0.34f),
    inscriptionGround = Color(0xFF34403A),
    inscriptionEdge = Color(0xFF5E2A27),
    inscriptionWash = Color(87, 94, 65).copy(alpha = 0.055f),
    plaster = R.drawable.plaster_nave,
)

val Apse = Palette(
    dark = true,
    text = Color(0xFFE8E2D0),
    bg = Color(0xFF121C28),
    accent = Color(0xFFD0B06A),
    gold = Color(0xFFD8BC74),
    goldLine = Color(0xFF6A5C3A),
    muted = Color(0xFF9AA4B0),
    border = Color(0xFF2A3648),
    rubric = Color(0xFFD47070),
    surface = Color(0xFF172232),
    surfaceEdge = Color(208, 176, 106).copy(alpha = 0.18f),
    pressedWash = Color(208, 176, 106).copy(alpha = 0.12f),
    oakLine = Color(208, 176, 106).copy(alpha = 0.16f),
    inscriptionGround = Color(0xFF263431),
    inscriptionEdge = Color(0xFF5C2B35),
    inscriptionWash = Color(208, 176, 106).copy(alpha = 0.04f),
    plaster = R.drawable.plaster_apse,
)

/**
 * The gilding, which alone follows the season ("Seasonal ornament" in style.css): gold leaf
 * through most of the year, veiled in Passiontide, warmed at Eastertide. Functional gold
 * (controls, selections) stays [Palette.gold].
 */
@Immutable
data class Ornament(val flat: Color, val line: Color, val hi: Color, val lo: Color, val ink: Color) {
    /** The gilded initials' and lozenges' leaf, lit from above. */
    val leaf: Brush get() = Brush.verticalGradient(listOf(hi, lo))
}

fun ornament(palette: Palette, season: String): Ornament = when {
    season == "passiontide" && palette.dark -> Ornament(Color(0xFFB0A4C2), Color(0xFF565070), Color(0xFFCAC1D7), Color(0xFF988AAD), Color(0xFFB0A4C2))
    season == "passiontide" -> Ornament(Color(0xFF756A7E), Color(0xFFB5AABD), Color(0xFF8D8395), Color(0xFF605469), Color(0xFFB0A4C2))
    season == "eastertide" && palette.dark -> Ornament(Color(0xFFE6CF8C), Color(0xFF7A6A44), Color(0xFFF2E2B1), Color(0xFFD3B86F), Color(0xFFE6CF8C))
    season == "eastertide" -> Ornament(Color(0xFFA4731A), Color(0xFFD0B06C), Color(0xFFC8922D), Color(0xFF886011), Color(0xFFE6CF8C))
    palette.dark -> Ornament(palette.gold, palette.goldLine, Color(0xFFE7D295), Color(0xFFC0A25A), Color(0xFFD7B878))
    else -> Ornament(palette.gold, palette.goldLine, Color(0xFFB98D3C), Color(0xFF7D5C1C), Color(0xFFD7B878))
}

/** The liturgical colours of the ordo rails and the band across an hour's top (`.day-color-*`). */
fun dayColor(name: String): Color = when (name) {
    "red" -> Color(0xFFB02A24)
    "green" -> Color(0xFF3A6B3A)
    "violet" -> Color(0xFF6A3A8A)
    "black" -> Color(0xFF3A3A3A)
    "rose" -> Color(0xFFC4607A)
    else -> Color(0xFFC9B896)
}

/** The menu's Theme row: Default follows the device; Nave and Apse are the web's names. */
enum class ThemeChoice(val label: String) { DEFAULT("Default"), NAVE("Nave"), APSE("Apse") }

/** The menu's Text row, scaling the whole page as the web scales its root (93%, 100%, 110%). */
enum class TextSize(val scale: Float) { SMALL(0.93f), DEFAULT(1f), LARGE(1.1f) }

val Garamond = FontFamily(
    Font(R.font.eb_garamond_regular, FontWeight.Normal, FontStyle.Normal),
    Font(R.font.eb_garamond_italic, FontWeight.Normal, FontStyle.Italic),
    Font(R.font.eb_garamond_bold, FontWeight.Bold, FontStyle.Normal),
)

/** Only ✠: the Garamond cut has no cross. */
val CrossFont = FontFamily(Font(R.font.noto_sans_symbols_cross, FontWeight.Bold))

/** Garamond's own small caps for lower and upper case, as `font-variant-caps: all-small-caps`. */
const val ALL_SMALL_CAPS = "smcp, c2sc"

val LocalPalette = staticCompositionLocalOf { Nave }
val LocalOrnament = staticCompositionLocalOf { ornament(Nave, "") }

/**
 * The type scale, in the web's CSS pixels at a phone's width (one CSS px to one dp). Line
 * heights are exact, and trimmed to the text so blocks stack by their margins alone.
 */
object Type {
    private val trim = LineHeightStyle(LineHeightStyle.Alignment.Center, LineHeightStyle.Trim.None)
    private fun face(size: Float, line: Float) = TextStyle(fontFamily = Garamond, fontSize = size.sp, lineHeight = line.sp, lineHeightStyle = trim)

    val body = face(20f, 32f)
    val verse = face(20f, 33f)
    val rubric = face(18f, 27f)
    val heading = face(19.2f, 24.96f).copy(fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 2.112.sp, textAlign = TextAlign.Center)
    val itemLabel = face(20f, 27f).copy(fontFeatureSettings = "$ALL_SMALL_CAPS, lnum", letterSpacing = 1.4.sp, textAlign = TextAlign.Center)
    val reference = face(17f, 23.8f).copy(textAlign = TextAlign.Center)
    val verseNumber = face(15.6f, 25.74f).copy(textAlign = TextAlign.End)
    val speaker = face(17f, 22.1f).copy(fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 0.94.sp)
    val hourTitle = face(24.8f, 39.68f).copy(letterSpacing = 1.984.sp, textAlign = TextAlign.Center)
    val meta = face(14.4f, 20.88f).copy(textAlign = TextAlign.Center)

    /** Uppercase working labels: "Change date", "Menu", continuation labels. */
    fun label(size: Float, tracking: Float) = face(size, size * 1.6f).copy(letterSpacing = (size * tracking).sp)

    val brand = label(13.12f, 0.08f)
    val menu = label(12.48f, 0.08f)
    val control = label(11.52f, 0.1f)
    val small = face(12.48f, 19.97f)
}

@Composable
fun OfficeTheme(
    choice: ThemeChoice = ThemeChoice.DEFAULT,
    textSize: TextSize = TextSize.DEFAULT,
    season: String = "",
    content: @Composable () -> Unit,
) {
    val dark = when (choice) {
        ThemeChoice.DEFAULT -> isSystemInDarkTheme()
        ThemeChoice.NAVE -> false
        ThemeChoice.APSE -> true
    }
    val palette = if (dark) Apse else Nave
    val scheme = (if (dark) darkColorScheme() else lightColorScheme()).copy(
        primary = palette.accent,
        background = palette.bg,
        surface = palette.surface,
        onBackground = palette.text,
        onSurface = palette.text,
        onSurfaceVariant = palette.muted,
        outline = palette.border,
    )
    // Text size scales every measure together, as the web scales its root.
    val density = LocalDensity.current
    val scaled = Density(density.density * textSize.scale, density.fontScale)
    CompositionLocalProvider(
        LocalPalette provides palette,
        LocalOrnament provides ornament(palette, season),
        LocalDensity provides scaled,
    ) {
        MaterialTheme(colorScheme = scheme, typography = MaterialTheme.typography.copy(bodyLarge = Type.body.copy(color = palette.text))) {
            content()
        }
    }
}
