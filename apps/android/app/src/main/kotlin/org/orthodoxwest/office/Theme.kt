package org.orthodoxwest.office

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

/** The web app's palette (style.css `:root` and its dark-scheme overrides). */
@Immutable
data class Palette(
    val page: Color,
    val surface: Color,
    val text: Color,
    val rubric: Color,
    val accent: Color,
    val gold: Color,
    val muted: Color,
    val border: Color,
)

val LightPalette = Palette(
    page = Color(0xFFFAF3E9),
    surface = Color(0xFFF6EDDF),
    text = Color(0xFF241C17),
    rubric = Color(0xFF8B1A1A),
    accent = Color(0xFF6B3A1F),
    gold = Color(0xFF9A7328),
    muted = Color(0xFF6B5D54),
    border = Color(0xFFDCCFC3),
)

val DarkPalette = Palette(
    page = Color(0xFF121C28),
    surface = Color(0xFF172232),
    text = Color(0xFFE8E2D0),
    rubric = Color(0xFFD47070),
    accent = Color(0xFFD0B06A),
    gold = Color(0xFFD8BC74),
    muted = Color(0xFF9AA4B0),
    border = Color(0xFF2A3648),
)

/** The liturgical colors of the web's ordo rails. */
fun liturgicalColor(name: String): Color? = when (name) {
    "white" -> Color(0xFFC9B896)
    "red" -> Color(0xFFB02A24)
    "green" -> Color(0xFF3A6B3A)
    "violet" -> Color(0xFF6A3A8A)
    "black" -> Color(0xFF3A3A3A)
    "rose" -> Color(0xFFC4607A)
    else -> null
}

val Garamond = FontFamily(
    Font(R.font.eb_garamond_regular, FontWeight.Normal, FontStyle.Normal),
    Font(R.font.eb_garamond_italic, FontWeight.Normal, FontStyle.Italic),
    Font(R.font.eb_garamond_bold, FontWeight.Bold, FontStyle.Normal),
)

/** Only ✠: the Garamond cut has no cross. */
val CrossFont = FontFamily(Font(R.font.noto_sans_symbols_cross, FontWeight.Bold))

val LocalPalette = staticCompositionLocalOf { LightPalette }

@Composable
fun OfficeTheme(dark: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    val palette = if (dark) DarkPalette else LightPalette
    val scheme = if (dark) {
        darkColorScheme(
            primary = palette.accent,
            background = palette.page,
            surface = palette.page,
            surfaceContainer = palette.surface,
            surfaceContainerHigh = palette.surface,
            onBackground = palette.text,
            onSurface = palette.text,
            onSurfaceVariant = palette.muted,
            outline = palette.border,
        )
    } else {
        lightColorScheme(
            primary = palette.accent,
            background = palette.page,
            surface = palette.page,
            surfaceContainer = palette.surface,
            surfaceContainerHigh = palette.surface,
            onBackground = palette.text,
            onSurface = palette.text,
            onSurfaceVariant = palette.muted,
            outline = palette.border,
        )
    }
    val body = TextStyle(fontFamily = Garamond, fontSize = 19.sp, lineHeight = 27.sp, color = palette.text)
    val typography = MaterialTheme.typography.let { t ->
        t.copy(
            bodyLarge = body,
            bodyMedium = body.copy(fontSize = 17.sp, lineHeight = 24.sp),
            titleLarge = body.copy(fontSize = 22.sp, lineHeight = 28.sp),
            titleMedium = body.copy(fontSize = 19.sp),
            labelLarge = body.copy(fontSize = 17.sp),
            labelMedium = body.copy(fontSize = 15.sp),
        )
    }
    CompositionLocalProvider(LocalPalette provides palette) {
        MaterialTheme(colorScheme = scheme, typography = typography, content = content)
    }
}
