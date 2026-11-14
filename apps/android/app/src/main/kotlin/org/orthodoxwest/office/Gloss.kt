package org.orthodoxwest.office

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.draw.dropShadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.shadow.Shadow
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.LayoutCoordinates
import androidx.compose.ui.layout.boundsInWindow
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.DpOffset
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntRect
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupPositionProvider
import androidx.compose.ui.window.PopupProperties
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.GlossView
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt

/** Marks a glossed term's run in a block's styled text; the item is the run's index in the block. */
internal const val GLOSS = "gloss"

/** A block's glossed rubric terms ("secretly"), by the index of their run. */
fun glosses(block: BlockView): List<Pair<Int, GlossView>> =
    block.runs.withIndex().mapNotNull { (i, run) -> run.gloss?.let { i to it } }

/** The web's `.gloss-note`: 19rem wide, set in 0.95rem, its heading 0.95em of that. */
private val NoteWidth = 304.dp
private val NoteMargin = 12.dp
private val NoteGap = 10.dp
private val Arrow = 6.dp
/** Room about the card in the popup's window for its shadow. */
private val ShadowRoom = 24.dp
private val NoteText = TextStyle(fontFamily = Garamond, fontSize = 15.2.sp, lineHeight = 22.04.sp)
private val NoteTitle = NoteText.copy(fontSize = 14.44.sp, lineHeight = 20.sp, fontFeatureSettings = "smcp", letterSpacing = 0.72.sp)

/**
 * A rubric whose terms a newcomer stumbles on, as the web's `.gloss-term`: each term carries a
 * hairline in the rubric's red at 40% (solid while its note is open), and a tap on it opens its
 * note above the word, so the note never covers the prayer it explains; below only where the top
 * of the window leaves no room. A tap elsewhere or Back closes it. `open` is the open term's run.
 */
@Composable
fun GlossedText(text: AnnotatedString, block: BlockView, open: Int?, onOpen: (Int?) -> Unit, modifier: Modifier, style: TextStyle) {
    val p = LocalPalette.current
    val density = LocalDensity.current
    var layout by remember { mutableStateOf<TextLayoutResult?>(null) }
    // The live coordinates, read as the popup places itself: they follow the page as it scrolls.
    val coordinates = remember { arrayOfNulls<LayoutCoordinates>(1) }
    val terms = text.getStringAnnotations(GLOSS, 0, text.length)
    // Read as a tap lands: the tap handler lives across opens and closes.
    val currentOpen by rememberUpdatedState(open)
    val rise = with(density) { style.fontSize.toPx() * 0.24f }
    Box(modifier) {
        Text(
            text,
            Modifier
                .fillMaxWidth()
                .onGloballyPositioned { coordinates[0] = it }
                .drawBehind {
                    val l = layout ?: return@drawBehind
                    for (term in terms) {
                        val ink = p.rubric.copy(alpha = if (term.item.toInt() == open) 1f else 0.4f)
                        for (r in termRects(l, term.start, term.end)) {
                            val y = l.getLineBaseline(l.getLineForVerticalPosition(r.center.y)) + rise
                            drawLine(ink, Offset(r.left, y), Offset(r.right, y), strokeWidth = 1.dp.toPx())
                        }
                    }
                }
                .pointerInput(text) {
                    // A finger needs more than one word of rubric type.
                    val slop = 10.dp.toPx()
                    detectTapGestures { at ->
                        val l = layout ?: return@detectTapGestures
                        val hit = terms.firstOrNull { t -> termRects(l, t.start, t.end).any { it.inflate(slop).contains(at) } }
                        if (hit != null) onOpen(hit.item.toInt().takeIf { it != currentOpen })
                    }
                },
            style = style,
            onTextLayout = { layout = it },
        )
        val gloss = open?.let { block.runs.getOrNull(it)?.gloss }
        val term = open?.let { i -> terms.firstOrNull { it.item.toInt() == i } }
        val words = if (gloss != null && term != null) NoteWords(gloss.title, gloss.note) else null
        val rects = term?.let {
            {
                val l = layout
                val at = coordinates[0]
                if (l == null || at == null || !at.isAttached) {
                    emptyList()
                } else {
                    termRects(l, it.start, it.end).map { r -> Rect(at.localToWindow(r.topLeft), at.localToWindow(r.bottomRight)) }
                }
            }
        }
        Note(words, rects ?: { emptyList() }, onDismiss = { onOpen(null) })
    }
}

/**
 * A mark whose meaning is not on its face (the ordo's rank letters, its § and fish), as the web's
 * `.month-table abbr`: a tap on it opens its `name` in the note a glossed word opens, above the
 * mark, so the note never covers it. A second tap, a tap elsewhere or Back closes it. Compose
 * answers a touch over the platform's whole minimum target round so small a mark. `label` is what
 * a screen reader calls the mark; the note's words are read as they appear.
 */
@Composable
fun NoteMark(name: String, label: String, modifier: Modifier = Modifier, mark: @Composable () -> Unit) {
    var open by remember { mutableStateOf(false) }
    val coordinates = remember { arrayOfNulls<LayoutCoordinates>(1) }
    Box(
        modifier
            .onGloballyPositioned { coordinates[0] = it }
            .tap(label = label) { open = !open }
            .semantics { stateDescription = if (open) "Expanded" else "Collapsed" },
    ) {
        // The label speaks for the mark: a § or a fish read aloud says nothing.
        Box(Modifier.clearAndSetSemantics {}) { mark() }
    }
    Note(
        if (open) NoteWords("", name) else null,
        rects = { coordinates[0]?.takeIf { it.isAttached }?.let { listOf(it.boundsInWindow()) } ?: emptyList() },
        onDismiss = { open = false },
    )
}

/** A term's box on each line it runs over, in the text's coordinates. */
private fun termRects(l: TextLayoutResult, start: Int, end: Int): List<Rect> {
    val out = mutableListOf<Rect>()
    var i = start
    while (i < end) {
        val line = l.getLineForOffset(i)
        val stop = min(end, l.getLineEnd(line, visibleEnd = true))
        if (stop > i) {
            val left = l.getBoundingBox(i).left
            val right = l.getBoundingBox(stop - 1).right
            out += Rect(min(left, right), l.getLineTop(line), max(left, right), l.getLineBottom(line))
        }
        val next = l.getLineEnd(line)
        if (next <= i) break
        i = next
    }
    return out
}

/**
 * Places the note above its term's first line, or below its last where the window's top (below
 * the status bar, `top`) is too near.
 */
private class NotePosition(
    val rects: () -> List<Rect>,
    val room: Int,
    val top: Int,
    val margin: Int,
    val gap: Int,
    val arrowInset: Float,
) : PopupPositionProvider {
    var arrow by mutableFloatStateOf(0f)
    var below by mutableStateOf(false)

    override fun calculatePosition(anchorBounds: IntRect, windowSize: IntSize, layoutDirection: LayoutDirection, popupContentSize: IntSize): IntOffset {
        val rs = rects()
        if (rs.isEmpty()) return IntOffset(anchorBounds.left, anchorBounds.top)
        val w = popupContentSize.width - 2 * room
        val h = popupContentSize.height - 2 * room
        val isBelow = rs.first().top - h - gap < top + margin
        val r = if (isBelow) rs.last() else rs.first()
        val mid = r.center.x
        val left = (mid - w / 2f).coerceIn(margin.toFloat(), max(margin, windowSize.width - margin - w).toFloat())
        val top = if (isBelow) r.bottom + gap else r.top - h - gap
        below = isBelow
        arrow = (mid - left).coerceIn(arrowInset, w - arrowInset)
        return IntOffset((left - room).roundToInt(), (top - room).roundToInt())
    }
}

/** What a note says: a glossed term's heading and note, or a mark's name alone (no heading). */
data class NoteWords(val title: String, val note: String)

/**
 * The app's one answer to "what does this mean?" (the web's `.gloss-note`), for a rubric's term
 * and an ordo's mark alike: a card of the surface ruled in gold, pointing at what it explains from
 * above (`rects`, in the window). Open while `words` is given; it fades in and out on Tempo's
 * settle, by its ink alone (words are faded, never slid), and holds its last words and place
 * while it goes. With the system's animations off, it comes and goes at once.
 */
@Composable
fun Note(words: NoteWords?, rects: () -> List<Rect>, onDismiss: () -> Unit) {
    val shown = remember { MutableTransitionState(false) }
    shown.targetState = words != null
    // Kept while the card fades out, so it does not empty or lose its place as it goes.
    val kept = remember { arrayOfNulls<Pair<NoteWords, () -> List<Rect>>>(1) }
    if (words != null) kept[0] = words to rects
    val (said, at) = kept[0] ?: return
    if (!shown.currentState && !shown.targetState) return
    val p = LocalPalette.current
    val density = LocalDensity.current
    val current by rememberUpdatedState(at)
    // The page runs under the status bar; the note stays clear of it.
    val inset = WindowInsets.safeDrawing.getTop(density)
    val position = with(density) {
        remember(said, inset) { NotePosition({ current() }, ShadowRoom.roundToPx(), inset, NoteMargin.roundToPx(), NoteGap.roundToPx(), 14.dp.toPx()) }
    }
    // The web's shadows: plaster `0 .4rem 1.1rem` at 16%; on the night sky a deeper black.
    val shadow = if (p.dark) {
        Shadow(radius = 22.4.dp, color = Color.Black.copy(alpha = 0.45f), offset = DpOffset(0.dp, 8.dp))
    } else {
        Shadow(radius = 17.6.dp, color = Color(36, 28, 23).copy(alpha = 0.16f), offset = DpOffset(0.dp, 6.4.dp))
    }
    val shape = RoundedCornerShape(3.dp)
    // Going, it no longer holds the window's touches or Back.
    Popup(popupPositionProvider = position, onDismissRequest = onDismiss, properties = PopupProperties(focusable = words != null)) {
        AnimatedVisibility(shown, enter = fadeIn(Tempo.settleIn.spec()), exit = fadeOut(Tempo.settleOut.spec())) {
            Box(
                Modifier
                    .padding(ShadowRoom)
                    // A mark's name is a word or two: the card is as wide as its words.
                    .then(if (said.title.isEmpty()) Modifier.widthIn(max = NoteWidth).width(IntrinsicSize.Max) else Modifier.width(NoteWidth))
                    .drawWithContent {
                        drawContent()
                        pointer(position.arrow, position.below, p.surface, p.goldLine)
                    },
            ) {
                Column(
                    Modifier
                        .fillMaxWidth()
                        .dropShadow(shape, shadow)
                        .background(p.surface, shape)
                        .border(1.dp, p.goldLine, shape)
                        // The surface's edge catching the light just inside the top (`inset 0 1px 0`).
                        .drawBehind { drawRect(p.surfaceEdge, Offset(1.dp.toPx(), 1.dp.toPx()), size.copy(width = size.width - 2.dp.toPx(), height = 1.dp.toPx())) }
                        .padding(start = 14.4.dp, end = 14.4.dp, top = 11.2.dp, bottom = 12.8.dp)
                        .semantics(mergeDescendants = true) { liveRegion = LiveRegionMode.Polite },
                ) {
                    if (said.title.isNotEmpty()) Text(said.title, style = NoteTitle.copy(color = p.rubric), modifier = Modifier.padding(bottom = 3.2.dp))
                    Text(said.note, style = NoteText.copy(color = p.text))
                }
            }
        }
    }
}

/** The note's pointer at `x` along its card: a corner of the card's gold rule, toward the term. */
private fun DrawScope.pointer(x: Float, below: Boolean, fill: Color, rule: Color) {
    val a = Arrow.toPx()
    val edge = if (below) 0f else size.height
    val tip = if (below) -a else a
    val line = 1.dp.toPx()
    val inside = if (below) line else -line
    val wedge = Path().apply {
        moveTo(x - a, edge + inside)
        lineTo(x, edge + tip)
        lineTo(x + a, edge + inside)
        close()
    }
    drawPath(wedge, fill)
    drawLine(rule, Offset(x - a, edge), Offset(x, edge + tip), line)
    drawLine(rule, Offset(x, edge + tip), Offset(x + a, edge), line)
}
