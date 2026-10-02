package org.orthodoxwest.office

import android.graphics.Paint
import android.graphics.Rect
import android.graphics.Typeface
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Shadow
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.ParagraphStyle
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.TextMeasurer
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.BaselineShift
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextIndent
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import androidx.core.content.res.ResourcesCompat
import org.orthodoxwest.office.core.BlockKind
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.RunStyle
import org.orthodoxwest.office.core.SectionView
import org.orthodoxwest.office.core.capHeight
import org.orthodoxwest.office.core.initialFit
import org.orthodoxwest.office.core.initialProfileEm
import org.orthodoxwest.office.core.initialSize
import org.orthodoxwest.office.core.raisedInitialGap
import org.orthodoxwest.office.core.raisedInitialSize
import kotlin.math.roundToInt

/** The verse gutter (`--verse-gutter`, 1.8rem): verse numbers and ℣/℟ sit in it, text beyond it. */
val VerseGutter: Dp = 28.8.dp

/** The ℣/℟ column and the space after it (the web's `.sigil`, 1.4rem + 0.4rem): together the verse gutter. */
private val SigilGap: Dp = 6.4.dp
private val SigilColumn: Dp = VerseGutter - SigilGap

/**
 * A sigil set right-aligned in its column. A mark wider than the column hangs up to `hang` into
 * the margin before it; only beyond that (a large system font) does the column widen, so the
 * mark never wraps or leaves the page.
 */
private fun Modifier.sigilColumn(hang: Dp): Modifier = layout { measurable, constraints ->
    val mark = measurable.measure(constraints.copy(minWidth = 0))
    val width = maxOf(SigilColumn.roundToPx(), mark.width - hang.roundToPx())
    layout(width, mark.height) { mark.place(width - mark.width, 0) }
}

/** Latin within a small-caps label is set in lower case italic, as the web's `.psalm-incipit`. */
private const val NO_SMALL_CAPS = "'smcp' 0, 'c2sc' 0, lnum"

/**
 * Space above `cur`, after `prev`, in the web's rhythm at a phone's width (measured from the
 * rendered hour): a clear threshold between psalm groups, air between elements, and the
 * lines of one element close together.
 */
fun gapBefore(prev: BlockView?, cur: BlockView): Dp {
    if (prev == null) return 0.dp
    val heading = { b: BlockView -> b.kind == BlockKind.HEADING || b.kind == BlockKind.COMMEMORATION_HEADING }
    val antiphon = { b: BlockView -> b.kind == BlockKind.ANTIPHON || b.kind == BlockKind.ANNOUNCED_ANTIPHON }
    val note = { b: BlockView -> b.kind == BlockKind.ANTIPHON_NOTE || b.kind == BlockKind.ANNOUNCEMENT_NOTE }
    return when {
        heading(cur) -> 38.dp
        heading(prev) -> if (cur.kind == BlockKind.CHAPTER_REF) 24.dp else 27.dp
        cur.kind == BlockKind.GAP || prev.kind == BlockKind.GAP -> 4.8.dp
        prev.kind == BlockKind.ITEM_LABEL -> 9.dp
        // A note on the antiphon sits close under it, as the web's `.unrepeated-note`.
        note(cur) -> 2.4.dp
        cur.kind == BlockKind.ITEM_LABEL -> if (antiphon(prev) || note(prev)) 10.dp else 30.dp
        prev.kind == BlockKind.CHAPTER_REF -> 15.dp
        prev.kind == BlockKind.LATIN_TITLE -> 8.dp
        prev.kind == BlockKind.SPEAKER -> 3.2.dp
        // A closing antiphon, then the next group's opening one: the threshold between groups.
        antiphon(cur) && prev.kind == BlockKind.ANTIPHON -> 49.dp
        // A closing antiphon sits close under its psalm's last verse or Gloria.
        cur.kind == BlockKind.ANTIPHON && (prev.kind == BlockKind.VERSE || (prev.kind == BlockKind.GLORIA_PATRI && !prev.startsElement)) -> 6.dp
        cur.kind == BlockKind.VERSE && prev.kind == BlockKind.VERSE -> 4.8.dp
        // The Gloria Patri after a psalm's last verse.
        cur.kind == BlockKind.GLORIA_PATRI && prev.kind == BlockKind.VERSE -> 13.6.dp
        cur.kind == BlockKind.STANZA && prev.kind == BlockKind.STANZA -> 12.dp
        // A hymn's rubric keeps a stanza's distance from the stanzas around it.
        cur.kind == BlockKind.HYMN_RUBRIC && prev.kind == BlockKind.STANZA -> 12.dp
        cur.kind == BlockKind.STANZA && prev.kind == BlockKind.HYMN_RUBRIC -> 12.dp
        cur.startsElement -> 14.dp
        else -> 4.8.dp
    }
}

/**
 * One block of a composed hour, styled after the web's classes for the same text. A hymn's
 * stanzas and rubrics are set in `column`, the width of the hymn's longest line (see [hymnColumns]). A
 * heading with `cross` stands under a small painted cross, between the office's parts.
 */
@Composable
fun Block(block: BlockView, modifier: Modifier = Modifier, column: Dp? = null, cross: Boolean = false) {
    // One stop for a screen reader, in words (spoken): the drawn initial and gutter marks are for the eye.
    val m = if (block.kind == BlockKind.GAP) modifier else modifier.clearAndSetSemantics {
        contentDescription = spoken(block)
        if (block.kind == BlockKind.HEADING || block.kind == BlockKind.COMMEMORATION_HEADING) heading()
    }
    val p = LocalPalette.current
    val text = Type.body.copy(color = p.text)
    val verse = Type.verse.copy(color = p.text)
    when (block.kind) {
        BlockKind.GAP -> Spacer(m.height(8.dp))
        // Tituli, as painted in red ochre on the limewash (gilt on the Apse night), clear of the rubrics' red.
        BlockKind.HEADING, BlockKind.COMMEMORATION_HEADING -> Column(m.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
            if (cross) PaintedCross(Modifier.padding(bottom = 11.2.dp).size(9.92.dp))
            Text(runs(block), Modifier.fillMaxWidth(), style = Type.heading.copy(color = p.titulus))
        }
        BlockKind.ITEM_LABEL -> Text(runs(block), m.fillMaxWidth(), style = Type.itemLabel.copy(color = p.titulus))
        BlockKind.LATIN_TITLE, BlockKind.CANTICLE_SECTION -> Text(
            runs(block),
            m.fillMaxWidth(),
            style = text.copy(color = p.muted, fontStyle = FontStyle.Italic, textAlign = TextAlign.Center),
        )
        BlockKind.CHAPTER_REF, BlockKind.SCRIPTURE_REF -> Text(runs(block), m.fillMaxWidth(), style = Type.reference.copy(color = p.rubric))
        BlockKind.RUBRIC -> Text(runs(block), m.fillMaxWidth(), style = Type.rubric.copy(color = p.rubric))
        // Under the antiphon's words, past its "Ant." (the antiphon's hanging indent, which scales with text).
        BlockKind.ANTIPHON_NOTE -> Text(
            runs(block),
            m.fillMaxWidth().padding(start = with(LocalDensity.current) { 21.6.sp.toDp() }),
            style = Type.rubric.copy(color = p.rubric),
        )
        // Under an announcement, centred beneath its words.
        BlockKind.ANNOUNCEMENT_NOTE -> Text(
            runs(block),
            m.fillMaxWidth().padding(horizontal = 4.dp),
            style = Type.rubric.copy(color = p.rubric, textAlign = TextAlign.Center),
        )
        BlockKind.SPEAKER -> Text(runs(block), m.fillMaxWidth(), style = Type.speaker.copy(color = p.rubric))
        // Body antiphons hang left: the sigil opens the line, wrapped lines clear it. An announcement's
        // opening words stand centred over the psalm's label, as the web's `.antiphon-announce`.
        BlockKind.ANTIPHON, BlockKind.ANNOUNCED_ANTIPHON -> Text(
            buildAnnotatedString {
                withStyle(SpanStyle(color = p.titulus, fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 1.4.sp)) { append(block.marker) }
                append(" ")
                append(runs(block))
            },
            m.fillMaxWidth(),
            style = if (block.kind == BlockKind.ANNOUNCED_ANTIPHON) {
                text.copy(textAlign = TextAlign.Center)
            } else {
                text.copy(textIndent = TextIndent(restLine = 21.6.sp))
            },
        )
        BlockKind.VERSE -> when {
            block.dropCap -> Opening(block, verse, m, textStart = VerseGutter)
            block.marker.isEmpty() -> Text(runs(block), m.fillMaxWidth().padding(start = VerseGutter), style = verse)
            else -> Row(m.fillMaxWidth()) {
                Text(
                    block.marker,
                    Modifier.widthIn(min = VerseGutter).padding(end = 8.dp).alignByBaseline(),
                    softWrap = false,
                    style = Type.verseNumber.copy(color = p.muted),
                )
                Text(runs(block), Modifier.alignByBaseline(), style = verse)
            }
        }
        // The Gloria Patri keeps the verses' edge; each line's wrap steps in (the web's `.source-line`, 1.1rem).
        BlockKind.GLORIA_PATRI -> Text(
            runs(block),
            m.fillMaxWidth().padding(start = VerseGutter),
            style = verse.copy(textIndent = TextIndent(restLine = 17.6.sp)),
        )
        BlockKind.VERSICLE, BlockKind.RESPONSE, BlockKind.ALL -> when {
            block.dropCap -> Opening(block, text, m, textStart = 0.dp, raised = true)
            block.marker.isEmpty() -> Text(runs(block), m.fillMaxWidth(), style = text)
            // "Blessing." is too wide to hang in the page gutter: it stands above its words,
            // both on the edge the ℣/℟ lines' words share, as the web's phone layout sets it.
            block.kind == BlockKind.VERSICLE && block.marker.length > 2 -> Column(m.fillMaxWidth().padding(start = VerseGutter)) {
                Text(block.marker, style = text.copy(color = p.rubric))
                Text(runs(block), style = text)
            }
            else -> Row(m.fillMaxWidth()) {
                // "All:" hangs its extra width into the page gutter, keeping its words on that edge.
                val hang = if (block.kind == BlockKind.ALL) Gutter else 0.dp
                Text(block.marker, Modifier.sigilColumn(hang).alignByBaseline(), softWrap = false, style = text.copy(color = p.rubric))
                Spacer(Modifier.width(SigilGap))
                Text(runs(block), Modifier.alignByBaseline(), style = text)
            }
        }
        // The hymn's column, centred: the rag balanced by an equal indent on the left, as the
        // web's fit-content `.hymn-verses`. A wrapped line hangs beneath its own start (`.hymn-line`, 1.1rem).
        BlockKind.STANZA -> Box(m.fillMaxWidth(), contentAlignment = Alignment.TopCenter) {
            val style = verse.copy(textIndent = TextIndent(restLine = 17.6.sp))
            val inColumn = if (column != null) Modifier.width(column) else Modifier
            if (block.dropCap) Opening(block, style, inColumn, textStart = 0.dp) else Text(runs(block), inColumn.fillMaxWidth(), style = style)
        }
        // A rubric among the stanzas, centred in the hymn's column.
        BlockKind.HYMN_RUBRIC -> Box(m.fillMaxWidth(), contentAlignment = Alignment.TopCenter) {
            val inColumn = if (column != null) Modifier.width(column) else Modifier
            Text(runs(block), inColumn.fillMaxWidth(), style = Type.rubric.copy(color = p.rubric, textAlign = TextAlign.Center))
        }
        BlockKind.PARAGRAPH, BlockKind.CHANT_LINE -> {
            if (block.dropCap) Opening(block, text, m, textStart = 0.dp) else Text(runs(block), m.fillMaxWidth(), style = text)
        }
    }
}

/**
 * An opening with its initial, set as the web's adaptive initial sets it. A painted capital two
 * lines deep stands beside the text as it wraps; a short responsory, or prose that fits on its
 * line, takes a smaller capital raised on that line; a psalm keeps its full capital, raised over
 * a single line (elevated), and may break after its mediant rather than leave a stub of a second
 * line (divided). Hymns and the Marian antiphon always drop. The rest of the first word, or the
 * next word after a lone O or I, turns to small caps as the eye leaves the capital. The capital
 * stands at the measure's edge (a psalm's hangs into the verse gutter); `textStart` places the
 * lines that run on below it.
 */
@Composable
private fun Opening(block: BlockView, style: TextStyle, modifier: Modifier, textStart: Dp, raised: Boolean = false) {
    val (letter, rest) = splitInitial(block, runs(block))
    if (letter == null) {
        Text(rest, modifier.fillMaxWidth().padding(start = textStart), style = style)
        return
    }
    val ochre = LocalOrnament.current.flat
    val density = LocalDensity.current
    // Flat ochre, as a painter laid it, with a hint of the brush's edge 1dp below; it veils and
    // brightens with the season's gilding.
    fun DrawScope.initial(letter: TextLayoutResult) {
        drawText(letter, color = ochre.copy(alpha = 0.3f), topLeft = Offset(0f, 1.dp.toPx()))
        drawText(letter, color = ochre)
    }
    val measurer = rememberTextMeasurer()
    val face = garamond()
    val plain = style.copy(textIndent = null)
    // Raised, the capital is a letter of its line: 1.65 times the text, its line's height unchanged.
    val fit = initialFit(letter)
    val raisedText = with(density) {
        buildAnnotatedString {
            val shadow = Shadow(ochre.copy(alpha = 0.3f), Offset(0f, 1.dp.toPx()), 0f)
            val after = raisedInitialGap() + fit.raisedTuck / raisedInitialSize()
            withStyle(SpanStyle(color = ochre, fontSize = raisedInitialSize().em, letterSpacing = after.em, shadow = shadow)) { append(letter) }
            append(rest)
        }
    }
    BoxWithConstraints(modifier.fillMaxWidth()) {
        val width = constraints.maxWidth
        val start = with(density) { textStart.toPx() }
        val cap = remember(letter, plain, density, face) { dropCap(letter, plain, measurer, density, face) }
        // The lines beside the capital start at its fitted edge, the first line's opening word tucked
        // in or out from there; the nearer of the two is the column's edge.
        val (line1, line2) = cap.lines(start)
        val besideStart = minOf(line1, line2)
        val (firstIndent, restIndent) = with(density) { (line1 - besideStart).toSp() to (line2 - besideStart).toSp() }
        // The first line's indent holds to the first line only: a line after a forced break (a hymn's
        // next metrical line, a divided psalm's second half) starts as the second line does.
        fun indented(text: AnnotatedString): AnnotatedString = buildAnnotatedString {
            val nl = text.text.indexOf('\n')
            withStyle(ParagraphStyle(textIndent = TextIndent(firstIndent, restIndent))) { append(if (nl < 0) text else text.subSequence(0, nl)) }
            if (nl >= 0) withStyle(ParagraphStyle(textIndent = TextIndent(restIndent, restIndent))) { append(text.subSequence(nl + 1, text.length)) }
        }
        // Over a single line, the capital rises above it: the text starts at its lower contour, untucked.
        val elevatedStyle = with(density) { plain.copy(textIndent = TextIndent(firstLine = (maxOf(cap.edge, start) - start).toSp())) }
        // A line beside a dropped capital starts with its word, as CSS drops a space at a line's start
        // (after a lone O); a raised capital keeps the space, sharing its line.
        val body = rest.withoutLeadingSpace()
        val setting = remember(body, raisedText, firstIndent, restIndent, elevatedStyle, width, besideStart, raised, block.kind) {
            fun beside(text: AnnotatedString) =
                measurer.measure(indented(text), plain, constraints = Constraints(maxWidth = (width - besideStart.toInt()).coerceAtLeast(1)))
            val natural = beside(body)
            val divided = divideAtMediant(body)?.let { it to beside(it) }
            val psalm = block.kind == BlockKind.VERSE
            when {
                raised -> OpeningSetting.Raised
                psalm && natural.lineCount < 2 -> {
                    val elevated = measurer.measure(body, elevatedStyle, constraints = Constraints(maxWidth = (width - start.toInt()).coerceAtLeast(1)))
                    // Raising can widen the line past the measure: two whole half-verses beside a
                    // dropped capital are then better than a new short tail.
                    if (elevated.lineCount > 1 && divided != null && divided.second.lineCount == 2) {
                        OpeningSetting.Dropped(divided.first, divided.second)
                    } else {
                        OpeningSetting.Elevated(elevated)
                    }
                }
                // A one- or two-word tail under less than 30% of the first line: break at the mediant
                // instead, if that keeps two lines and clearly balances them.
                psalm && divided != null && natural.lineCount == 2 && words(natural, body, 1) <= 2 &&
                    lineWidth(natural, 1) < lineWidth(natural, 0) * 0.3f &&
                    divided.second.lineCount == 2 && balance(divided.second) > balance(natural) + 0.15f ->
                    OpeningSetting.Dropped(divided.first, divided.second)
                block.kind == BlockKind.PARAGRAPH &&
                    measurer.measure(raisedText, plain, constraints = Constraints(maxWidth = (width - start.toInt()).coerceAtLeast(1))).lineCount < 2 ->
                    OpeningSetting.Raised
                else -> OpeningSetting.Dropped(body, natural)
            }
        }
        when (setting) {
            OpeningSetting.Raised -> Text(raisedText, Modifier.fillMaxWidth().padding(start = textStart), style = plain)
            is OpeningSetting.Elevated -> {
                // One line pitch above the text, into which the capital rises; its ink top meets the
                // cap height of that empty line, its foot the text's baseline.
                val capTop = setting.layout.firstBaseline + cap.drop - cap.glyph.firstBaseline
                Column(Modifier.drawBehind { translate(cap.left, capTop) { initial(cap.glyph) } }) {
                    Spacer(Modifier.height(with(density) { style.lineHeight.toDp() }))
                    Text(body, Modifier.padding(start = textStart), style = elevatedStyle)
                }
            }
            is OpeningSetting.Dropped -> {
                val text = setting.text
                val shown = indented(text)
                val beside = setting.layout
                // Two lines beside the capital (three past a descending tail); the remainder runs on at the text edge.
                val split = beside.getLineEnd(minOf(cap.rows, beside.lineCount) - 1, visibleEnd = false)
                // Two lines of a hymn end at the stanza's own line break: it belongs to neither part. The
                // lines beside keep their indents; the remainder is the plain text after them (`shown`
                // has no first break, its paragraphs split there instead).
                val first = shown.subSequence(0, split).let { if (it.text.endsWith("\n")) it.subSequence(0, it.length - 1) else it }
                val nl = text.text.indexOf('\n')
                val after = text.subSequence(if (nl in 0 until split) split + 1 else split, text.length)
                    .let { if (it.text.startsWith("\n")) it.subSequence(1, it.length) else it }
                // The capital's ink top meets the first line's cap height; its foot then stands on the second baseline.
                val capTop = beside.getLineBaseline(0) + cap.drop - cap.glyph.firstBaseline
                Column {
                    // Painted behind the lines beside it, taking no room of its own: the lines set the height.
                    Box(Modifier.drawBehind { translate(cap.left, capTop) { initial(cap.glyph) } }) {
                        Text(first, Modifier.padding(start = with(density) { besideStart.toDp() }), style = plain)
                    }
                    if (after.isNotEmpty()) Text(after, Modifier.padding(start = textStart), style = style)
                }
            }
        }
    }
}

/** How an opening's initial is set (see [Opening]). */
private sealed interface OpeningSetting {
    data object Raised : OpeningSetting
    class Elevated(val layout: TextLayoutResult) : OpeningSetting
    class Dropped(val text: AnnotatedString, val layout: TextLayoutResult) : OpeningSetting
}

/** The opening with a break after its mediant (the web's `.initial-divided`), if it has one. */
private fun divideAtMediant(text: AnnotatedString): AnnotatedString? {
    val at = text.text.indexOf('*').takeIf { it >= 0 } ?: return null
    return buildAnnotatedString {
        append(text.subSequence(0, at + 1))
        append("\n")
        append(text.subSequence(at + 1, text.length).withoutLeadingSpace())
    }
}

private fun lineWidth(layout: TextLayoutResult, line: Int) = layout.getLineRight(line) - layout.getLineLeft(line)

private fun words(layout: TextLayoutResult, text: AnnotatedString, line: Int) =
    text.text.substring(layout.getLineStart(line), layout.getLineEnd(line)).split(Regex("\\s+")).count { it.isNotEmpty() }

/** How evenly a two-line setting fills its lines: the shorter over the longer. */
private fun balance(layout: TextLayoutResult): Float {
    if (layout.lineCount != 2) return 0f
    val (a, b) = lineWidth(layout, 0) to lineWidth(layout, 1)
    return minOf(a, b) / maxOf(a, b)
}

/**
 * A dropped initial's glyph and where it stands, in px: `left` places the glyph from the measure's
 * edge, `drop` its baseline below the first line's. The lines beside it start at its `edge`, the
 * first line's opening word moved by `tuck`.
 */
private class DropCap(val glyph: TextLayoutResult, val left: Float, val drop: Float, val edge: Float, val tuck: Float, val rows: Int) {
    /**
     * Where the first and second lines start, for text whose own edge is `textStart`: as beside the
     * web's float, a line never starts short of the text's edge (an I or a V hanging into a psalm's
     * gutter), and the tuck moves the first line's word from wherever its line starts.
     */
    fun lines(textStart: Float): Pair<Float, Float> {
        val second = maxOf(edge, textStart)
        return second + tuck to second
    }
}

/**
 * A two-line initial as the web's `initial-letter: 2` sets it: its cap height spans a line pitch and
 * the text's cap height, and it is fitted by its ink (see `render_blocks::initials`) and the
 * capital's optical profile.
 */
private fun dropCap(letter: String, style: TextStyle, measurer: TextMeasurer, density: Density, face: Typeface): DropCap {
    val fit = initialFit(letter)
    val leading = if (style.lineHeight.isSp) style.lineHeight.value / style.fontSize.value else 1.65f
    val size = style.fontSize * initialSize(leading)
    val glyph = measurer.measure(letter, style.copy(textIndent = null, fontSize = size, lineHeight = size))
    return with(density) {
        val ink = Rect().also { Paint().apply { typeface = face; textSize = size.toPx() }.getTextBounds(letter, 0, letter.length, it) }
        // The profiles are measured in the web's declared initial, not the size it is drawn at.
        val em = initialProfileEm() * style.fontSize.toPx()
        val left = fit.hang * em
        DropCap(
            glyph, left - ink.left, -capHeight() * style.fontSize.toPx() - ink.top,
            left + ink.width() + fit.gap * em, fit.tuck * style.fontSize.toPx(), if (fit.depth > 0f) 3 else 2,
        )
    }
}

/** The text face, for an initial's ink. */
@Composable
private fun garamond(): Typeface {
    val context = LocalContext.current
    return remember(context) { ResourcesCompat.getFont(context, R.font.eb_garamond_regular) ?: Typeface.SERIF }
}

private fun AnnotatedString.withoutLeadingSpace() = subSequence(text.indexOfFirst { !it.isWhitespace() }.let { if (it < 0) length else it }, length)

private fun Char.inWord() = isLetter() || this == '\'' || this == '’' || this == '-'

/**
 * The opening letter and the text after it, with the small-caps transition applied. Only an
 * opening whose first run is ordinary spoken text takes an initial, or a psalm's whose opening
 * words go unsaid after its antiphon: the initial stays gilt, the words after it muted.
 */
private fun splitInitial(block: BlockView, text: AnnotatedString): Pair<String?, AnnotatedString> {
    val first = block.runs.firstOrNull()?.style
    if (first != RunStyle.PLAIN && !(first == RunStyle.SECRET && block.kind == BlockKind.VERSE)) return null to text
    val s = text.text
    val at = s.indexOfFirst { !it.isWhitespace() }
    if (at < 0 || !s[at].isLetter()) return null to text
    val rest = text.subSequence(at + 1, text.length)
    // The small caps take the rest of this word, or the next word after a lone letter.
    val r = rest.text
    var start = 0
    var end = r.indexOfFirst { !it.inWord() }.let { if (it < 0) r.length else it }
    if (end == 0) {
        start = r.indexOfFirst { !it.isWhitespace() }.coerceAtLeast(0)
        end = (start until r.length).firstOrNull { !r[it].inWord() } ?: r.length
    }
    val styled = buildAnnotatedString {
        append(rest)
        if (end > start) addStyle(SpanStyle(fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 0.04.em), start, end)
    }
    return s.substring(at, at + 1) to styled
}

/**
 * A block as a screen reader says it: ℣ and ℟ named, the pointing marks (the mediant's * and
 * the flex †) turned to the pauses they mark, printed verse numbers left silent, and ✠ said as
 * the sign of the cross.
 */
fun spoken(block: BlockView): String {
    val words = block.runs.joinToString("") { run ->
        when (run.style) {
            // The pause between half-verses, as the printed psalter's colon.
            RunStyle.MEDIANT -> ": "
            RunStyle.CROSS -> CROSS_MARK
            RunStyle.BREAK -> " "
            // A posture cue within a verse ("Sit.") is an aside to the words around it.
            RunStyle.POSTURE -> " (${run.text.trim()}) "
            else -> run.text
        }
    }
        .replace("†", ", ")
        .replace("·", ".")
        .replace("℣.", "Versicle.").replace("℟.", "Response.")
        .replace("℣", "Versicle").replace("℟", "Response")
        .replace(Regex("[\\s\u00a0]+"), " ")
        .replace(Regex("[,;:]?\\s*$CROSS_MARK\\s*"), ", sign of the cross, ")
        // No pause doubled: a mark after punctuation, or a comma before it, gives way.
        .replace(Regex("([,.;:!?])\\s*[,:]"), "$1")
        .replace(Regex(",\\s*([,.;:!?])"), "$1")
        .replace(Regex(" ([,.;:!?])"), "$1")
        .trim()
        .removePrefix(", ")
        .removeSuffix(",")
        .replaceFirstChar { it.uppercase() }
    val marker = when (block.kind) {
        BlockKind.ANTIPHON, BlockKind.ANNOUNCED_ANTIPHON -> "Antiphon."
        BlockKind.VERSICLE, BlockKind.RESPONSE -> block.marker.replace("℣.", "Versicle.").replace("℟.", "Response.")
        BlockKind.VERSE -> ""
        else -> block.marker
    }
    return listOf(marker, words).filter { it.isNotEmpty() }.joinToString(" ")
}

private const val CROSS_MARK = "\uE000"

/** The web's `.hymn-verses` max-width, 28rem. */
private val HymnMax: Dp = 448.dp

/**
 * Slack on a hymn's column, so its longest line does not wrap on rounding. The web's `.hymn-line`
 * hang adds nothing to the column: its padding and negative text-indent cancel in `fit-content`.
 */
private val HymnSlack: Dp = 1.dp

/**
 * Each hymn's column width, by (section, block) index of its stanzas: its longest metrical line,
 * the opening initial included, capped at 28rem. The web centres `.hymn-verses` on the same
 * measure (`width: fit-content`). A hymn is a run of stanzas, with any rubric or gap among them.
 */
@Composable
fun hymnColumns(sections: List<SectionView>): Map<Pair<Int, Int>, Dp> {
    val p = LocalPalette.current
    val measurer = rememberTextMeasurer()
    val density = LocalDensity.current
    val style = Type.verse.copy(color = p.text)
    val face = garamond()
    val texts = sections.map { s -> s.blocks.map { if (it.kind == BlockKind.STANZA) runs(it) else null } }
    return remember(sections, style, density) {
        fun width(t: AnnotatedString) = measurer.measure(t, style, softWrap = false).size.width
        fun stanzaWidth(block: BlockView, text: AnnotatedString): Int {
            var start = 0
            val lines = text.text.split('\n').map { line -> text.subSequence(start, start + line.length).also { start += line.length + 1 } }
            return lines.mapIndexed { i, line ->
                val w = width(line)
                if (!block.dropCap || i > 1 || line.text.isBlank()) return@mapIndexed w
                // The initial stands beside the first two lines, which start at its fitted edges.
                val letter = line.text.trimStart().take(1)
                val (line1, line2) = dropCap(letter, style, measurer, density, face).lines(0f)
                if (i == 0) width(line.subSequence(line.text.indexOf(letter) + 1, line.length).withoutLeadingSpace()) + line1.roundToInt() else w + line2.roundToInt()
            }.maxOrNull() ?: 0
        }
        val out = mutableMapOf<Pair<Int, Int>, Dp>()
        sections.forEachIndexed { si, s ->
            var run = mutableListOf<Int>()
            var widest = 0
            fun close() {
                val col = with(density) { widest.toDp() + HymnSlack }.coerceAtMost(HymnMax)
                if (widest > 0) run.forEach { out[si to it] = col }
                run = mutableListOf()
                widest = 0
            }
            s.blocks.forEachIndexed { bi, b ->
                when (b.kind) {
                    BlockKind.STANZA -> {
                        run.add(bi)
                        widest = maxOf(widest, stanzaWidth(b, texts[si][bi]!!))
                    }
                    // A hymn's rubric takes its column without widening it (the web's width: 0; min-width: 100%).
                    BlockKind.HYMN_RUBRIC -> run.add(bi)
                    BlockKind.RUBRIC, BlockKind.GAP -> Unit
                    else -> close()
                }
            }
            close()
        }
        out
    }
}

/** A block's runs as styled text. */
@Composable
fun runs(block: BlockView): AnnotatedString {
    val p = LocalPalette.current
    return buildAnnotatedString {
        for (run in block.runs) {
            val style = runStyle(run.style, p)
            if (style == null) append(run.text) else withStyle(style) { append(run.text) }
        }
    }
}

private fun runStyle(style: RunStyle, p: Palette): SpanStyle? = when (style) {
    RunStyle.PLAIN, RunStyle.BREAK -> null
    // The pointing asterisk is quiet in psalms, antiphons and responsories.
    // Lowered 0.25em to the line's optical middle, as the web's `.mediant` (Garamond draws its
    // asterisk high, as a footnote mark). Compose shifts by the face's ascent, 0.71em.
    RunStyle.MEDIANT -> SpanStyle(color = p.muted, baselineShift = BaselineShift(-0.25f / 0.71f))
    RunStyle.CROSS -> SpanStyle(color = p.rubric, fontFamily = CrossFont, fontSize = 0.8.em)
    RunStyle.PRAYED -> SpanStyle(color = p.text)
    RunStyle.SECRET -> SpanStyle(color = p.unsaid)
    // A psalm's Latin incipit is muted beside its titulus, as Latin titles are.
    RunStyle.LATIN -> SpanStyle(color = p.muted, fontStyle = FontStyle.Italic, fontFeatureSettings = NO_SMALL_CAPS, letterSpacing = 0.4.sp)
    RunStyle.KICKER -> SpanStyle(fontSize = 0.7.em, color = p.muted, letterSpacing = 0.1.em)
    RunStyle.POSTURE -> SpanStyle(color = p.rubric, fontSize = 0.9.em)
}
