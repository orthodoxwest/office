package org.orthodoxwest.office

import android.graphics.Paint
import android.graphics.Typeface
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.dp
import androidx.core.content.res.ResourcesCompat
import org.orthodoxwest.neuma.Chant
import org.orthodoxwest.neuma.ChantOptions
import org.orthodoxwest.neuma.Ink
import org.orthodoxwest.neuma.Item
import org.orthodoxwest.neuma.LayoutOptions
import org.orthodoxwest.neuma.Page
import org.orthodoxwest.neuma.TextRole
import org.orthodoxwest.neuma.glyphOutline
import org.orthodoxwest.office.core.BlockView

/** Staff spaces per lyric em: neuma's default lyric size, as the web's render-html/src/chant.rs. */
private const val LYRIC_SPACES = 2.45f

/**
 * A chant score (a `BlockKind.SCORE` block, with the Chant setting on): neuma engraves its GABC
 * at the column's width with its lyrics at the hymn's text size, as the web's container query
 * picks the engraving that fits, and the staff takes the theme's lining, as the web's
 * `.chant-score` does. A whole hymn's stanzas after the first carry their number (the marker),
 * in the rubric's red above the score.
 */
@Composable
fun Score(block: BlockView, modifier: Modifier) {
    val p = LocalPalette.current
    val density = LocalDensity.current
    val lyric = with(density) { Type.verse.fontSize.toPx() }
    val ornament = LocalOrnament.current
    val faces = scoreFaces()
    Column(modifier.fillMaxWidth()) {
        if (block.marker.isNotEmpty()) {
            Text(block.marker, Modifier.padding(bottom = 2.dp), style = Type.rubric.copy(color = p.rubric, fontSize = Type.verse.fontSize * 0.85f))
        }
        BoxWithConstraints(Modifier.fillMaxWidth()) {
            val width = constraints.maxWidth.toFloat()
            val page = remember(block.score, block.dropCap, width, lyric) { engrave(block.score, block.dropCap, width, lyric) }
            // A custos may reach past the measure: the whole score is fitted to the column.
            val fit = if (page.width > width) width / page.width else 1f
            Canvas(Modifier.fillMaxWidth().height(with(density) { (page.height * fit).toDp() })) {
                scale(fit, Offset.Zero) { drawScore(page, ScoreInks.of(p, ornament), faces) }
            }
        }
    }
}

/**
 * The score laid out `width` pixels wide with its lyrics `lyric` pixels high; the first stanza
 * alone takes the initial. Set as the web sets it (`page_options` in render-html's chant.rs): a
 * hyphen between every two syllables, the initial three quarters of the Solesmes size and flush
 * with the margin, and the last staff ending at the final bar.
 */
internal fun engrave(gabc: String, initial: Boolean, width: Float, lyric: Float): Page =
    Chant(
        gabc,
        ChantOptions(
            initial = if (initial) 1 else 0,
            annotation = initial,
            everyHyphen = true,
            initialScale = 0.75f,
            initialMargin = 0f,
            shortLastStaff = true,
        ),
    ).use { chant ->
        chant.layout(width, LayoutOptions(scale = lyric / LYRIC_SPACES)).use { it.page() }
    }

/**
 * The theme's inks for a score, the web's `--chant-*` tokens (style.css): neumes a shade off the
 * text's ink, lyrics in it, staves in the lining (thinned on a night ground), the initial gilt as
 * every dropped initial is, the mode and rubric runs in the rubric's red.
 */
internal class ScoreInks(val note: Color, val text: Color, val staff: Color, val rubric: Color, val initial: Color) {
    companion object {
        fun of(p: Palette, ornament: Ornament) = ScoreInks(
            note = mix(p.text, p.bg, 0.86f),
            text = p.text,
            staff = if (p.dark) mix(p.lining, p.bg, 0.62f) else p.lining,
            rubric = p.rubric,
            initial = ornament.flat,
        )
    }
}

internal class ScoreFaces(val regular: Typeface, val italic: Typeface)

@Composable
private fun scoreFaces(): ScoreFaces {
    val context = LocalContext.current
    return remember(context) {
        ScoreFaces(
            ResourcesCompat.getFont(context, R.font.eb_garamond_regular) ?: Typeface.SERIF,
            ResourcesCompat.getFont(context, R.font.eb_garamond_italic) ?: Typeface.create(Typeface.SERIF, Typeface.ITALIC),
        )
    }
}

/** Each glyph's outline, fetched once (neuma's path data, in the units an item's scale converts from). */
private val outlines = HashMap<Int, Path?>()

private fun outline(id: Int): Path? = synchronized(outlines) {
    outlines.getOrPut(id) { glyphOutline(id)?.let { PathParser().parsePathString(it.path).toPath() } }
}

/** neuma's display list in the theme's inks: glyphs and rectangles, then lyrics in EB Garamond with ligatures off. */
internal fun DrawScope.drawScore(page: Page, inks: ScoreInks, faces: ScoreFaces) {
    fun ink(role: Ink) = if (role == Ink.STAFF || role == Ink.LEDGER) inks.staff else inks.note
    val paint = Paint(Paint.ANTI_ALIAS_FLAG)
    for (item in page.items) {
        when (item) {
            is Item.Glyph -> outline(item.glyph)?.let { path ->
                translate(item.x, item.y) { scale(item.scale, Offset.Zero) { drawPath(path, ink(item.role)) } }
            }
            is Item.Rect -> drawRect(ink(item.role), Offset(item.x, item.y), Size(item.w, item.h))
            is Item.Text -> drawIntoCanvas { canvas ->
                var x = item.x
                for (run in item.runs) {
                    paint.typeface = if (run.italic) faces.italic else faces.regular
                    paint.isFakeBoldText = run.bold
                    paint.textSize = item.size
                    paint.fontFeatureSettings = if (run.smallCaps) "'liga' 0, 'smcp' 1" else "'liga' 0"
                    paint.color = when {
                        item.role == TextRole.INITIAL -> inks.initial
                        run.rubric || item.role == TextRole.RUBRIC || item.role == TextRole.ANNOTATION -> inks.rubric
                        else -> inks.text
                    }.toArgb()
                    canvas.nativeCanvas.drawText(run.text, x, item.baseline, paint)
                    x += paint.measureText(run.text)
                }
            }
        }
    }
}
