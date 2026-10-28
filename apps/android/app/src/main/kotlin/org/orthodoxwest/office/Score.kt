package org.orthodoxwest.office

import android.graphics.Paint
import android.graphics.Typeface
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.ui.Alignment
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.core.content.res.ResourcesCompat
import org.orthodoxwest.neuma.Chant
import org.orthodoxwest.neuma.ChantOptions
import org.orthodoxwest.neuma.Ink
import org.orthodoxwest.neuma.Item
import org.orthodoxwest.neuma.LayoutOptions
import org.orthodoxwest.neuma.LyricFont
import org.orthodoxwest.neuma.Page
import org.orthodoxwest.neuma.TextRole
import org.orthodoxwest.neuma.glyphOutline
import org.orthodoxwest.office.core.BlockView

/** Staff spaces per lyric em: neuma's default lyric size, as the web's render-html/src/chant.rs. */
private const val LYRIC_SPACES = 2.45f

/**
 * A chant score (a `BlockKind.SCORE` block, with the Chant setting on): neuma engraves its GABC
 * at the column's width with its lyrics at the hymn's text size, as the web's chant.js lays it
 * out at its column's, and the staff takes a tint of the theme's lining, as the web's
 * `.chant-score` does. The score keeps to the hymn's measure, as the web's `.hymn-verses`. The
 * opening score carries its mode above it (`.chant-mode`); a whole hymn's stanzas after the first
 * carry their number (the marker), in the rubric's red above the score; and a tune shown once
 * carries a strip of its hymn's verses under it (`.chant-strip`), a tap setting that verse's
 * words under the notes.
 */
@Composable
fun Score(block: BlockView, modifier: Modifier) {
    val p = LocalPalette.current
    val density = LocalDensity.current
    val lyric = with(density) { Type.verse.fontSize.toPx() }
    val ornament = LocalOrnament.current
    val faces = scoreFaces()
    var verse by rememberSaveable(block.score) { mutableIntStateOf(0) }
    val gabc = block.stanzas.getOrNull(verse) ?: block.score
    Box(modifier.fillMaxWidth(), contentAlignment = Alignment.TopCenter) {
        Column(Modifier.widthIn(max = HymnMax).fillMaxWidth()) {
            if (block.marker.isNotEmpty()) {
                Text(block.marker, Modifier.padding(bottom = 2.dp), style = Type.rubric.copy(color = p.rubric, fontSize = Type.verse.fontSize * 0.85f))
            }
            if (block.mode.isNotEmpty()) {
                Text(
                    block.mode,
                    Modifier.padding(bottom = 3.dp).clearAndSetSemantics {},
                    style = Type.verse.copy(color = p.rubric, fontSize = Type.verse.fontSize * 0.8f, fontFeatureSettings = "smcp", letterSpacing = Type.verse.fontSize * 0.06f),
                )
            }
            BoxWithConstraints(Modifier.fillMaxWidth()) {
                val width = constraints.maxWidth.toFloat()
                val initial = block.dropCap && verse == 0
                val page = remember(gabc, initial, block.phrases, width, lyric) { engrave(gabc, initial, block.phrases, width, lyric) }
                // A custos may reach past the measure: the whole score is fitted to the column.
                val fit = if (page.width > width) width / page.width else 1f
                Canvas(Modifier.fillMaxWidth().height(with(density) { (page.height * fit).toDp() })) {
                    scale(fit, Offset.Zero) { drawScore(page, ScoreInks.of(p, ornament), faces, fit) }
                }
            }
            if (block.stanzas.size > 1) {
                VerseStrip(block.stanzas.size, verse) { verse = it }
            }
        }
    }
}

/** The verses a tune can carry, numbered: the one under the notes ringed in the rubric's red (`.chant-strip`). */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun VerseStrip(count: Int, chosen: Int, choose: (Int) -> Unit) {
    val p = LocalPalette.current
    FlowRow(
        Modifier.padding(top = 2.dp, bottom = 10.dp).semantics { contentDescription = "The verse under the notes" },
        horizontalArrangement = Arrangement.spacedBy(4.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        for (i in 0 until count) {
            val on = i == chosen
            Box(
                Modifier
                    .sizeIn(minWidth = 32.dp, minHeight = 32.dp)
                    .border(1.dp, if (on) p.rubric else p.surfaceEdge, CircleShape)
                    .selectable(selected = on, role = Role.RadioButton) { choose(i) }
                    .semantics { contentDescription = "Verse ${i + 1}" },
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    "${i + 1}",
                    style = Type.verse.copy(color = if (on) p.rubric else p.muted, fontSize = Type.verse.fontSize * 0.85f, fontFeatureSettings = "onum"),
                )
            }
        }
    }
}

/**
 * The score laid out `width` pixels wide with its lyrics `lyric` pixels high; the first stanza
 * alone takes the initial. Set as the web sets it (`page_options` in render-html's chant.rs):
 * lyrics measured in the app's own Garamond (the table the core sets, `LyricFont.CUSTOM`); a
 * hyphen between every two syllables; the initial half the Solesmes size and flush with the
 * margin, the mode above the score rather than over it; the last staff ending at the final bar;
 * and a hymn's phrases each on a staff of their own.
 */
internal fun engrave(gabc: String, initial: Boolean, phrases: Boolean, width: Float, lyric: Float): Page =
    Chant(
        gabc,
        ChantOptions(
            initial = if (initial) 1 else 0,
            annotation = false,
            font = LyricFont.CUSTOM,
            everyHyphen = true,
            initialScale = 0.5f,
            initialMargin = 0f,
            shortLastStaff = true,
            phraseLines = phrases,
        ),
    ).use { chant ->
        chant.layout(width, LayoutOptions(scale = lyric / LYRIC_SPACES)).use { it.page() }
    }

/**
 * The theme's inks for a score, the web's `--chant-*` tokens (style.css): neumes and lyrics in
 * the text's ink, staves a tint of the lining (fainter on a night ground), the initial gilt as
 * every dropped initial is, the mode and rubric runs in the rubric's red.
 */
internal class ScoreInks(val note: Color, val text: Color, val staff: Color, val rubric: Color, val initial: Color) {
    companion object {
        fun of(p: Palette, ornament: Ornament) = ScoreInks(
            note = p.text,
            text = p.text,
            staff = mix(p.lining, p.bg, if (p.dark) 0.5f else 0.72f),
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

/**
 * neuma's display list in the theme's inks: glyphs and rectangles, then lyrics in EB Garamond with
 * ligatures off. Drawn at `fit` of its size: staff and ledger lines are snapped to whole device
 * pixels, so every line of a staff is as crisp and as heavy as the others.
 */
internal fun DrawScope.drawScore(page: Page, inks: ScoreInks, faces: ScoreFaces, fit: Float = 1f) {
    fun ink(role: Ink) = if (role == Ink.STAFF || role == Ink.LEDGER) inks.staff else inks.note
    val paint = Paint(Paint.ANTI_ALIAS_FLAG or Paint.SUBPIXEL_TEXT_FLAG)
    for (item in page.items) {
        when (item) {
            is Item.Glyph -> outline(item.glyph)?.let { path ->
                translate(item.x, item.y) { scale(item.scale, Offset.Zero) { drawPath(path, ink(item.role)) } }
            }
            is Item.Rect -> if (item.role == Ink.STAFF || item.role == Ink.LEDGER) {
                val top = Math.round(item.y * fit) / fit
                val height = maxOf(1, Math.round(item.h * fit)) / fit
                drawRect(inks.staff, Offset(item.x, top), Size(item.w, height))
            } else {
                drawRect(ink(item.role), Offset(item.x, item.y), Size(item.w, item.h))
            }
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
