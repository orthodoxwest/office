package org.orthodoxwest.office

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.layout.layout
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextIndent
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import org.orthodoxwest.office.core.BlockKind
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.RunStyle
import org.orthodoxwest.office.core.SectionView

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
    return when {
        heading(cur) -> 38.dp
        heading(prev) -> if (cur.kind == BlockKind.CHAPTER_REF) 24.dp else 27.dp
        cur.kind == BlockKind.GAP || prev.kind == BlockKind.GAP -> 4.8.dp
        prev.kind == BlockKind.ITEM_LABEL -> 9.dp
        // A note on the antiphon sits close under it, as the web's `.unrepeated-note`.
        cur.kind == BlockKind.ANTIPHON_NOTE -> 2.4.dp
        cur.kind == BlockKind.ITEM_LABEL -> if (prev.kind == BlockKind.ANTIPHON || prev.kind == BlockKind.ANTIPHON_NOTE) 10.dp else 30.dp
        prev.kind == BlockKind.CHAPTER_REF -> 15.dp
        prev.kind == BlockKind.LATIN_TITLE -> 8.dp
        prev.kind == BlockKind.SPEAKER -> 3.2.dp
        // A closing antiphon, then the next group's opening one: the threshold between groups.
        cur.kind == BlockKind.ANTIPHON && prev.kind == BlockKind.ANTIPHON -> 49.dp
        // A closing antiphon sits close under its psalm's last verse or Gloria.
        cur.kind == BlockKind.ANTIPHON && (prev.kind == BlockKind.VERSE || (prev.kind == BlockKind.PARAGRAPH && !prev.startsElement)) -> 6.dp
        cur.kind == BlockKind.VERSE && prev.kind == BlockKind.VERSE -> 4.8.dp
        // The Gloria Patri after a psalm's last verse.
        cur.kind == BlockKind.PARAGRAPH && prev.kind == BlockKind.VERSE -> 13.6.dp
        cur.kind == BlockKind.STANZA && prev.kind == BlockKind.STANZA -> 12.dp
        cur.startsElement -> 14.dp
        else -> 4.8.dp
    }
}

/**
 * One block of a composed hour, styled after the web's classes for the same text. A hymn's
 * stanzas are set in `column`, the width of the hymn's longest line (see [hymnColumns]). A
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
        BlockKind.SPEAKER -> Text(runs(block), m.fillMaxWidth(), style = Type.speaker.copy(color = p.rubric))
        // Body antiphons hang left: the sigil opens the line, wrapped lines clear it.
        BlockKind.ANTIPHON -> Text(
            buildAnnotatedString {
                withStyle(SpanStyle(color = p.titulus, fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 1.4.sp)) { append(block.marker) }
                append(" ")
                append(runs(block))
            },
            m.fillMaxWidth(),
            style = text.copy(textIndent = TextIndent(restLine = 21.6.sp)),
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
        // web's fit-content `.hymn-verses`. A wrapped line hangs beneath its own start.
        BlockKind.STANZA -> Box(m.fillMaxWidth(), contentAlignment = Alignment.TopCenter) {
            val style = verse.copy(textIndent = TextIndent(restLine = 20.sp))
            val inColumn = if (column != null) Modifier.width(column) else Modifier
            if (block.dropCap) Opening(block, style, inColumn, textStart = 0.dp) else Text(runs(block), inColumn.fillMaxWidth(), style = style)
        }
        BlockKind.PARAGRAPH, BlockKind.CHANT_LINE -> {
            if (block.dropCap) Opening(block, text, m, textStart = 0.dp) else Text(runs(block), m.fillMaxWidth(), style = text)
        }
    }
}

/**
 * An opening with its initial: a painted capital two lines deep when the text wraps beside it,
 * or raised on the line when the text is short (the web's adaptive initial). The rest of the
 * first word, or the next word after a lone O or I, turns to small caps as the eye leaves the
 * capital. The capital stands at the measure's edge (a psalm's hangs into the verse gutter);
 * `textStart` places the lines that run on below it.
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
    val plain = style.copy(textIndent = null)
    BoxWithConstraints(modifier.fillMaxWidth()) {
        val width = constraints.maxWidth
        val capStyle = plain.copy(fontSize = style.fontSize * 3.05f, lineHeight = style.fontSize * 3.05f)
        val cap = remember(letter, capStyle) { measurer.measure(letter, capStyle) }
        val gap = with(density) { (style.fontSize * 3.05f * 0.06f).toPx() }
        val besideStart = (cap.size.width + gap).toInt()
        val beside = remember(rest, plain, width, besideStart) {
            measurer.measure(rest, plain, constraints = Constraints(maxWidth = (width - besideStart).coerceAtLeast(1)))
        }
        if (raised || beside.lineCount < 2) {
            // Raised: the capital stands on the first line's baseline and rises above it.
            val raisedStyle = plain.copy(fontSize = style.fontSize * 2.1f, lineHeight = style.fontSize * 2.1f)
            val small = remember(letter, raisedStyle) { measurer.measure(letter, raisedStyle) }
            val w = with(density) { small.size.width.toDp() + 2.dp }
            val h = with(density) { small.size.height.toDp() }
            Row {
                Box(Modifier.size(w, h).alignBy { small.firstBaseline.toInt() }) {
                    Canvas(Modifier.size(w, h)) { initial(small) }
                }
                Text(rest, Modifier.alignByBaseline(), style = plain)
            }
            return@BoxWithConstraints
        }
        // Two lines beside the capital; the remainder runs on at the text edge.
        val split = beside.getLineEnd(1, visibleEnd = false)
        // Two lines of a hymn end at the stanza's own line break: it belongs to neither part.
        val first = rest.subSequence(0, split).let { if (it.text.endsWith("\n")) it.subSequence(0, it.length - 1) else it }
        val after = rest.subSequence(split, rest.length).let { if (it.text.startsWith("\n")) it.subSequence(1, it.length) else it }
        // Seat the capital's foot on the second line's baseline.
        val capTop = (beside.getLineBaseline(1) - cap.firstBaseline).toInt()
        Column {
            Box {
                Canvas(
                    Modifier
                        .offset { IntOffset(0, capTop) }
                        .size(with(density) { cap.size.width.toDp() }, with(density) { cap.size.height.toDp() }),
                ) { initial(cap) }
                Text(first, Modifier.padding(start = with(density) { besideStart.toDp() }), style = plain)
            }
            if (after.isNotEmpty()) Text(after, Modifier.padding(start = textStart), style = style)
        }
    }
}

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
        BlockKind.ANTIPHON -> "Antiphon."
        BlockKind.VERSICLE, BlockKind.RESPONSE -> block.marker.replace("℣.", "Versicle.").replace("℟.", "Response.")
        BlockKind.VERSE -> ""
        else -> block.marker
    }
    return listOf(marker, words).filter { it.isNotEmpty() }.joinToString(" ")
}

private const val CROSS_MARK = "\uE000"

/** The web's `.hymn-verses` max-width, 28rem. */
private val HymnMax: Dp = 448.dp

/** The web's `.hymn-line` hanging padding (1.1rem at a phone's width), which its fit-content column counts. */
private val HymnHang: Dp = 17.6.dp

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
    val texts = sections.map { s -> s.blocks.map { if (it.kind == BlockKind.STANZA) runs(it) else null } }
    return remember(sections, style, density) {
        fun width(t: AnnotatedString) = measurer.measure(t, style, softWrap = false).size.width
        val cap = style.copy(fontSize = style.fontSize * 3.05f, lineHeight = style.fontSize * 3.05f)
        val gap = with(density) { (style.fontSize * 3.05f * 0.06f).toPx() }
        fun stanzaWidth(block: BlockView, text: AnnotatedString): Int {
            var start = 0
            val lines = text.text.split('\n').map { line -> text.subSequence(start, start + line.length).also { start += line.length + 1 } }
            return lines.mapIndexed { i, line ->
                val w = width(line)
                if (!block.dropCap || i > 1 || line.text.isBlank()) return@mapIndexed w
                // The initial stands beside the first two lines: its width and gap, less the letter it replaces.
                val letter = line.text.trimStart().take(1)
                val beside = measurer.measure(letter, cap).size.width + gap.toInt()
                if (i == 0) w - width(AnnotatedString(letter)) + beside else w + beside
            }.maxOrNull() ?: 0
        }
        val out = mutableMapOf<Pair<Int, Int>, Dp>()
        sections.forEachIndexed { si, s ->
            var run = mutableListOf<Int>()
            var widest = 0
            fun close() {
                val col = with(density) { widest.toDp() + HymnHang }.coerceAtMost(HymnMax)
                run.forEach { out[si to it] = col }
                run = mutableListOf()
                widest = 0
            }
            s.blocks.forEachIndexed { bi, b ->
                when (b.kind) {
                    BlockKind.STANZA -> {
                        run.add(bi)
                        widest = maxOf(widest, stanzaWidth(b, texts[si][bi]!!))
                    }
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
    RunStyle.MEDIANT -> SpanStyle(color = p.muted)
    RunStyle.CROSS -> SpanStyle(color = p.rubric, fontFamily = CrossFont, fontSize = 0.8.em)
    RunStyle.PRAYED -> SpanStyle(color = p.text)
    RunStyle.SECRET -> SpanStyle(color = p.unsaid)
    // A psalm's Latin incipit is muted beside its titulus, as Latin titles are.
    RunStyle.LATIN -> SpanStyle(color = p.muted, fontStyle = FontStyle.Italic, fontFeatureSettings = NO_SMALL_CAPS, letterSpacing = 0.4.sp)
    RunStyle.KICKER -> SpanStyle(fontSize = 0.7.em, color = p.muted, letterSpacing = 0.1.em)
    RunStyle.POSTURE -> SpanStyle(color = p.rubric, fontSize = 0.9.em)
}
