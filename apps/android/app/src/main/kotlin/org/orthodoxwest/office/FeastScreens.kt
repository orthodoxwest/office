package org.orthodoxwest.office

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.res.imageResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.foundation.layout.offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.layout.positionInParent
import androidx.compose.ui.semantics.isTraversalGroup
import androidx.compose.ui.semantics.traversalIndex
import androidx.compose.ui.text.style.LineBreak
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.Dp
import org.orthodoxwest.office.core.BlockKind
import java.time.LocalDate
import kotlinx.coroutines.delay
import org.orthodoxwest.office.core.FeastPageView
import org.orthodoxwest.office.core.FeastYearView
import org.orthodoxwest.office.core.FindView
import org.orthodoxwest.office.core.feastWords

/** The search and feast page's fixed words, as every front sets them. */
val FEAST_WORDS by lazy { feastWords() }

/** The feast pages' column: the prayer measure on a phone, the web's 44rem on a wide screen. */
@Composable
private fun feastColumn(): Modifier = if (LocalWide.current) Modifier.widthIn(max = 704.dp).fillMaxWidth().padding(horizontal = 24.dp) else Modifier.measure()

/**
 * The search for a feast or saint, as the web's /calendar/find: the field, then each feast found
 * with how it is kept this year and next. The field searches as the reader types.
 */
@Composable
fun FindScreen(
    view: FindView,
    query: String,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onQuery: (String) -> Unit,
    onFeast: (String) -> Unit,
    onDay: (LocalDate) -> Unit,
) {
    val p = LocalPalette.current
    val listState = rememberSaveable(saver = LazyListState.Saver) { LazyListState() }
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "chrome") { chrome() }
        item(key = "header") { FeastHeader(FEAST_WORDS.findTitle, view.subtitle, ink = p.text, modifier = feastColumn().padding(top = 17.6.dp)) }
        item(key = "field") { SearchField(query, onQuery, feastColumn().padding(top = 16.dp)) }
        if (view.count.isNotEmpty()) {
            item(key = "count") {
                Text(
                    view.count,
                    feastColumn().padding(top = 14.dp).semantics { liveRegion = LiveRegionMode.Polite },
                    style = Type.small.copy(fontSize = 14.4.sp, lineHeight = 21.sp, color = p.muted),
                )
            }
        }
        items(view.results, key = { it.id }) { r ->
            Column(feastColumn().padding(top = 16.dp)) {
                Row(verticalAlignment = Alignment.Top) {
                    Text(
                        r.name,
                        Modifier.weight(1f).tap(action = "open the feast's page") { onFeast(r.id) }.semantics { heading() },
                        style = Type.body.copy(fontSize = 21.6.sp, lineHeight = 27.sp, color = rankTone(r.rankTone, p)),
                    )
                    if (r.rank.isNotEmpty()) {
                        Text(
                            r.rank,
                            Modifier.padding(start = 12.dp, top = 4.dp).semantics { contentDescription = r.rankFull },
                            style = Type.small.copy(fontSize = 15.2.sp, lineHeight = 20.sp, color = rankTone(r.rankTone, p)),
                        )
                    }
                }
                Text(
                    buildAnnotatedString {
                        append(r.meta)
                        append(" · ")
                        withStyle(SpanStyle(color = p.accent)) { append(FEAST_WORDS.everyYear) }
                    },
                    Modifier.padding(top = 3.dp, bottom = 6.dp).tap(action = "open the feast's page") { onFeast(r.id) },
                    style = Type.small.copy(fontSize = 14.7.sp, lineHeight = 21.sp, color = p.muted),
                )
                r.years.forEachIndexed { i, y -> YearRow(y, first = i == 0, onDay = onDay) }
                Hairline(p.border, Modifier.padding(top = 12.dp))
            }
        }
        item(key = "footer") { Footer() }
    }
}

/**
 * A feast's page, as the web's /feast/{id} ("The feast page" in style.css): a leaf of home's
 * painted wall. Its head is home's day block turned about, the feast leading and its usual date
 * the titulus over it; then its standing (the next day it is kept, leading to that day), its
 * texts with the collect first, and how it is kept in the next eight years, ruled as home's
 * hours. The lines are [ruledLine]'s, with the web's seeds: the head 51–58, the standing 61–69,
 * the headings 71 on, the years 81 on.
 */
@Composable
fun FeastScreen(
    view: FeastPageView,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onFind: () -> Unit,
    onFeast: (String) -> Unit,
    onDay: (LocalDate) -> Unit,
) {
    val p = LocalPalette.current
    val w = FEAST_WORDS
    val listState = rememberSaveable(view.id, saver = LazyListState.Saver) { LazyListState() }
    BoxWithConstraints(Modifier.fillMaxSize()) {
    val tablet = maxWidth >= FeastRailFrom
    val desk = maxWidth >= FeastDeskFrom
    val rail = tablet && view.texts.isNotEmpty()
    val first = view.register == "first"
    // From the rail's width the back link and the head span the texts and the rail together.
    val column = if (tablet) Modifier.widthIn(max = 992.dp).fillMaxWidth().padding(horizontal = 24.dp) else feastColumn()
    // With no texts the standing and the years keep one narrow column under the head.
    val leaf = if (tablet) Modifier.widthIn(max = 624.dp).fillMaxWidth().padding(horizontal = 24.dp) else feastColumn()
    val railWidth = if (desk) 304.dp else 272.dp
    val frontal = feastFrontal(view.color)
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "chrome") { chrome() }
        item(key = "back") {
            Box(column) {
                Text(
                    "‹ ${w.findTitle}",
                    Modifier.heightIn(min = 44.dp).tap(onClick = onFind).padding(vertical = 12.dp),
                    style = Type.small.copy(fontSize = 15.2.sp, lineHeight = 20.sp, color = p.muted, fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 1.37.sp),
                )
            }
        }
        item(key = "header") { FeastHead(view, frontal, size = if (desk) 2 else if (tablet) 1 else 0, modifier = column) }
        // The standing first, then the texts, then the years; from 760dp the standing and the
        // years make a rail beside the texts (style.css `.feast-body`), and on a first-class feast
        // one upright course, the lining and the feast's colour, runs between them.
        if (rail) {
            item(key = "body") {
                Row(
                    column.padding(top = 6.4.dp).drawBehind {
                        if (!first) return@drawBehind
                        val u = 1.dp.toPx()
                        val railLeft = size.width - railWidth.toPx()
                        val textsRight = if (desk) minOf(FeastTextsMax.toPx(), railLeft - 48f * u) else railLeft - 40f * u
                        val x = (textsRight + railLeft) / 2f
                        drawPath(ruledLine(x, 4f * u, x, size.height - 4f * u, 1.25f * u, 57, u), p.lining)
                        drawPath(ruledLine(x + 3.5f * u, 6f * u, x + 3.5f * u, size.height - 6f * u, 0.8f * u, 58, u), frontal.copy(alpha = frontal.alpha * 0.62f))
                    },
                ) {
                    Box(Modifier.weight(1f).padding(end = if (desk) 48.dp else 40.dp)) {
                        FeastTexts(view, onDay, if (desk) Modifier.widthIn(max = FeastTextsMax) else Modifier)
                    }
                    Column(Modifier.width(railWidth)) {
                        Standing(view, tablet && !desk, onFeast, onDay)
                        FeastYears(view, seed = 72, onDay = onDay, modifier = Modifier.padding(top = 22.4.dp))
                    }
                }
            }
        } else {
            item(key = "standing") { Box(leaf.padding(top = if (tablet) 6.4.dp else 0.dp)) { Standing(view, tablet && !desk, onFeast, onDay) } }
            if (view.texts.isNotEmpty()) item(key = "texts") { FeastTexts(view, onDay, leaf.padding(top = 22.4.dp)) }
            item(key = "years") { FeastYears(view, seed = if (view.texts.isEmpty()) 71 else 72, onDay = onDay, modifier = leaf.padding(top = 22.4.dp)) }
        }
        item(key = "footer") { Footer() }
    }
    }
}

/** The search page's headpiece, its title in `ink`, and its subtitle, over a hairline (the ordo's header). */
@Composable
private fun FeastHeader(title: String, subtitle: String, ink: Color, modifier: Modifier) {
    val p = LocalPalette.current
    Column(
        modifier.drawBehind {
            val w = 1.dp.toPx()
            drawLine(p.border, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
        }.padding(top = 13.4.dp, bottom = 16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Headpiece()
        Text(
            title,
            Modifier.padding(top = 2.dp).semantics { heading() },
            style = Type.body.copy(fontSize = 28.sp, lineHeight = 34.sp, color = ink, textAlign = TextAlign.Center, fontFeatureSettings = "lnum"),
        )
        Text(subtitle, Modifier.padding(top = 4.8.dp), style = Type.small.copy(color = p.muted, textAlign = TextAlign.Center))
    }
}

/**
 * The colour a feast's or a day's paint takes on this page (the web's `--frontal`): the day's
 * colour, the ornament's gold on a white day, and the lining thinned on a black one.
 */
@Composable
private fun feastFrontal(color: String?): Color {
    val p = LocalPalette.current
    return when (color) {
        null, "" -> p.lining
        "white" -> LocalOrnament.current.flat
        "black" -> p.lining.copy(alpha = 0.8f)
        else -> railColor(color, p)
    }
}

/** A point of paint: a small square stood on its corner (`.feast-point`, the years' point). */
@Composable
private fun PaintPoint(side: Dp, color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(side * 1.42f).clearAndSetSemantics {}) {
        rotate(45f) { drawRect(color, Offset((size.width - side.toPx()) / 2f, (size.height - side.toPx()) / 2f), Size(side.toPx(), side.toPx())) }
    }
}

/** A titulus: spaced small capitals in the tituli's ink, old-style figures among them. */
@Composable
private fun titulusType(size: Float, tracking: Float = 0.15f) =
    Type.body.copy(fontSize = size.sp, lineHeight = (size * 1.5f).sp, color = LocalPalette.current.titulus, fontFeatureSettings = "smcp, c2sc, onum", letterSpacing = (size * tracking).sp, textAlign = TextAlign.Center)

/**
 * The head (`.feast-head`): a ruled course of the lining broken by the consecration cross, the
 * feast's usual date as a titulus, its name, and its rank and colour in italic parted by a point
 * of its colour. The register follows the rank: a first-class feast's cross is gilt, its course
 * doubled by a finer rule in its colour and its name a size up; a commemoration or a monastic
 * observance (`plain`) has a bare head, its name in the text's ink a size down. A lit place lies
 * on the wall behind it, with no edge, except on a plain or a black one. By night the name is in
 * the text's ink. A screen reader hears the name, then the titulus, then the rank. `size` is 0 on
 * a phone, 1 from the rail's width and 2 from the desk's.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun FeastHead(view: FeastPageView, frontal: Color, size: Int, modifier: Modifier) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val first = view.register == "first"
    val plain = view.register == "plain"
    val lit = !plain && view.color != "black"
    val pigment = ImageBitmap.imageResource(R.drawable.pigment)
    val crown = if (first) 37.6.dp else 33.6.dp
    val nameSize = when {
        first -> floatArrayOf(33.6f, 40f, 43.2f)
        plain -> floatArrayOf(27.2f, 31.2f, 33.6f)
        else -> floatArrayOf(31.2f, 36.8f, 40f)
    }[size]
    val pool = if (p.dark) Color(150, 178, 220).copy(alpha = 0.17f) else Color(255, 253, 248).copy(alpha = 0.9f)
    Box(modifier.padding(top = if (plain) 6.4.dp else 20.8.dp)) {
        Column(
            Modifier.fillMaxWidth().drawBehind {
                val u = 1.dp.toPx()
                val w = this.size.width
                if (lit) {
                    // radial-gradient(ellipse 50% 46% at 50% 46%, pool, transparent), over the head and a little beyond.
                    val box = Rect(-16f * u, -40f * u, w + 16f * u, this.size.height + 24f * u)
                    val centre = Offset(box.center.x, box.top + 0.46f * box.height)
                    val rx = 0.5f * box.width
                    val ry = (if (p.dark) 0.48f else 0.46f) * box.height
                    scale(1f, ry / rx, pivot = centre) { drawCircle(Brush.radialGradient(0f to pool, 1f to Color.Transparent, center = centre, radius = rx), rx, centre) }
                }
                if (plain) return@drawBehind
                val mid = w / 2f
                val gap = (if (first) 27f else 24f) * u
                drawPath(ruledLine(0f, 0.5f * u, mid - gap, 0.5f * u, 1.25f * u, 51, u), p.lining)
                drawPath(ruledLine(mid + gap, 0.5f * u, w, 0.5f * u, 1.25f * u, 52, u), p.lining)
                if (first) {
                    val day = frontal.copy(alpha = frontal.alpha * 0.62f)
                    drawPath(ruledLine(1.5f * u, 4f * u, mid - gap - 3f * u, 4f * u, 0.8f * u, 53, u), day)
                    drawPath(ruledLine(mid + gap + 3f * u, 4f * u, w - 1.5f * u, 4f * u, 0.8f * u, 54, u), day)
                }
            }.padding(start = 8.dp, end = 8.dp, top = if (plain) 3.2.dp else 25.6.dp, bottom = if (plain) 16.dp else if (size == 2) 25.6.dp else 18.4.dp)
                .semantics { isTraversalGroup = true },
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(view.titulus, Modifier.semantics { traversalIndex = 1f }, style = titulusType(16f))
            Text(
                view.name,
                Modifier.padding(top = 1.6.dp).semantics { heading(); traversalIndex = 0f },
                style = Type.body.copy(
                    fontSize = nameSize.sp, lineHeight = (nameSize * 1.14f).sp, color = if (plain || p.dark) p.text else p.rubric,
                    textAlign = TextAlign.Center, lineBreak = LineBreak.Heading, fontFeatureSettings = "lnum",
                ),
            )
            val rank = Type.small.copy(fontSize = 15.68.sp, lineHeight = 23.5.sp, color = p.muted, fontStyle = FontStyle.Italic, textAlign = TextAlign.Center)
            FlowRow(
                Modifier.padding(top = 8.8.dp).clearAndSetSemantics { contentDescription = view.subtitle; traversalIndex = 2f },
                horizontalArrangement = Arrangement.spacedBy(9.6.dp, Alignment.CenterHorizontally),
                itemVerticalAlignment = Alignment.CenterVertically,
            ) {
                view.subtitle.split(" · ").forEachIndexed { i, part ->
                    if (i > 0) PaintPoint(5.5.dp, frontal)
                    Text(part, style = rank)
                }
            }
        }
        if (!plain) {
            val cross = Modifier.align(Alignment.TopCenter).offset(y = -crown / 2).size(crown).clearAndSetSemantics {}
            if (first) ConsecrationCross(cross, brush = giltBrush(o)) else ConsecrationCross(cross, pigment = pigment)
        }
    }
}

/** A section's heading: a titulus over one stroke of the pen (`.feast-section h2`). */
@Composable
private fun SectionHeading(text: String, seed: Int, gap: Dp = 4.8.dp) {
    val p = LocalPalette.current
    Text(
        text,
        Modifier.padding(bottom = gap).fillMaxWidth().semantics { heading() }.drawBehind {
            val u = 1.dp.toPx()
            drawPath(ruledLine(0f, size.height - 0.5f * u, size.width, size.height - 0.5f * u, 1.3f * u, seed, u), p.lining.copy(alpha = 0.78f))
        }.padding(bottom = 6.4.dp),
        style = titulusType(17.28f).copy(textAlign = TextAlign.Start),
    )
}

/** The width from which the feast page sets its standing and years as a rail beside its texts. */
private val FeastRailFrom = 760.dp

/** The width from which the texts keep their own measure and the rail stands off from them. */
private val FeastDeskFrom = 960.dp
private val FeastTextsMax = 560.dp

/** The note's type under the feast page's headings. */
@Composable
private fun noteType() = Type.small.copy(fontSize = 14.72.sp, lineHeight = 20.6.sp, color = LocalPalette.current.muted)

/**
 * The feast's standing, drawn as home's invitation (`.feast-standing`): the wash feathered by the
 * tablet mask, the lining's frame by hand, and inside it a finer frame in the colour of the day
 * it is next kept, stopping short of the corners. It says one thing, centred: when the feast is
 * next kept, and the way to that day, "Today's hours" when that is today. A monastic observance
 * keeps its heading, the feast everyone keeps and its note as prose, with a ruled hairline before
 * the next keeping. A black day has no wash. `narrow` is the tablet's rail.
 */
@Composable
private fun Standing(view: FeastPageView, narrow: Boolean, onFeast: (String) -> Unit, onDay: (LocalDate) -> Unit) {
    val p = LocalPalette.current
    val w = FEAST_WORDS
    val monastic = view.monasticNote.isNotEmpty()
    val next = view.next
    if (!monastic && next == null) return
    val tablet = ImageBitmap.imageResource(R.drawable.tablet)
    val wash = if (next?.color == "black") Color.Transparent else homeInk(p).tabletWash
    val frontal = feastFrontal(next?.color)
    Column(
        Modifier.fillMaxWidth().drawBehind {
            tabletWash(tablet, wash)
            invitationRules(p.lining, frontal, seed = 61, day = 0.62f)
        }.padding(
            start = if (monastic) 18.4.dp else 17.6.dp, end = if (monastic) 18.4.dp else 17.6.dp,
            top = if (monastic) 16.dp else 13.6.dp, bottom = if (monastic) 14.4.dp else 15.2.dp,
        ),
        verticalArrangement = Arrangement.spacedBy(if (monastic) 8.dp else 5.6.dp),
    ) {
        if (monastic) {
            Text(w.monasticHeading, Modifier.fillMaxWidth().semantics { heading() }, style = titulusType(15.2f))
            view.keptByAll?.let { all ->
                Text(
                    buildAnnotatedString {
                        append(w.everyoneKeeps + " ")
                        withStyle(SpanStyle(color = p.accent, textDecoration = TextDecoration.Underline)) { append(all.name) }
                        append(" " + view.keptByAllNote)
                    },
                    Modifier.tap(action = "open the feast's page") { onFeast(all.id) },
                    style = Type.body.copy(fontSize = 16.8.sp, lineHeight = 24.sp, color = p.text),
                )
            }
            Text(view.monasticNote, style = noteType())
        }
        next?.let {
            val go = if (it.today) w.todaysHours else w.daysHours
            Column(
                Modifier.fillMaxWidth().heightIn(min = 48.dp)
                    .then(if (monastic) Modifier.padding(top = 4.8.dp) else Modifier)
                    .tap(label = "${it.label}: ${it.dateLabel}. $go", action = "open the day") { onDay(it.date.toLocalDate()) }
                    .then(
                        if (!monastic) Modifier else Modifier.drawBehind {
                            val u = 1.dp.toPx()
                            drawPath(ruledLine(0f, 0.5f * u, size.width, 0.5f * u, 0.9f * u, 69, u), p.lining.copy(alpha = 0.36f))
                        }.padding(top = 11.2.dp),
                    ),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(it.label, style = titulusType(13.76f))
                Text(
                    it.dateLabel,
                    style = Type.body.copy(fontSize = if (narrow) 19.2.sp else 20.8.sp, lineHeight = if (narrow) 25.sp else 27.sp, color = p.text, textAlign = TextAlign.Center, lineBreak = LineBreak.Heading, fontFeatureSettings = "onum"),
                )
                Text(
                    "$go ›",
                    Modifier.padding(top = 4.dp),
                    style = Type.body.copy(fontSize = 19.2.sp, lineHeight = 26.sp, letterSpacing = 0.23.sp, color = if (p.dark) p.lining else p.rubric, textAlign = TextAlign.Center),
                )
            }
        }
    }
}

/**
 * The feast's texts under their heading and source, the collect first, then the day it is said in
 * full (`.feast-texts`). Each text's label is in the tituli's ink, and the collect opens on the
 * hours' dropped initial.
 */
@Composable
private fun FeastTexts(view: FeastPageView, onDay: (LocalDate) -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    val w = FEAST_WORDS
    val note = noteType()
    Column(modifier) {
        SectionHeading(w.textsHeading, seed = 71)
        Text(view.textsSource, Modifier.padding(top = 1.6.dp, bottom = 4.8.dp), style = note.copy(fontSize = 14.4.sp, fontStyle = FontStyle.Italic))
        view.texts.forEachIndexed { n, t ->
            Text(
                buildAnnotatedString {
                    append(t.label)
                    if (t.common) {
                        withStyle(SpanStyle(color = p.muted, fontStyle = FontStyle.Italic, fontFeatureSettings = "", letterSpacing = 0.sp, fontSize = 13.6.sp)) {
                            append("  " + w.commonMark)
                        }
                    }
                },
                Modifier.padding(top = 16.dp, bottom = 2.4.dp).semantics { heading() },
                style = Type.body.copy(fontSize = 16.sp, lineHeight = 22.sp, color = p.titulus, fontFeatureSettings = "smcp", letterSpacing = 0.8.sp),
            )
            // Each text keeps the hour's rhythm between its blocks, a source's blank lines included.
            // The collect, the first text when a feast has one, opens as a collect does in the hours.
            val line = LocalDensity.current.bodyLine()
            val collect = n == 0 && t.prayer
            t.blocks.forEachIndexed { i, b ->
                val block = if (collect && i == 0 && b.kind == BlockKind.PARAGRAPH) b.copy(dropCap = true) else b
                Block(block, Modifier.padding(top = if (i == 0) 0.dp else b.space.of(line)))
            }
        }
        view.wholeOffice?.let { whole ->
            Text(
                buildAnnotatedString {
                    append(w.wholeOffice + " ")
                    withStyle(SpanStyle(color = p.accent, textDecoration = TextDecoration.Underline)) { append(whole.label) }
                },
                Modifier.padding(top = 17.6.dp).heightIn(min = 44.dp).tap(action = "open the day") { onDay(whole.date.toLocalDate()) },
                style = note,
            )
        }
    }
}

/**
 * The next eight years, ruled as home's hour table (`.feast-years`): a firmer rule above and
 * below and finer ones between, by hand, and none upright; no colour bars and no chevrons.
 */
@Composable
private fun FeastYears(view: FeastPageView, seed: Int, onDay: (LocalDate) -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    // Where each row starts, for the rules between them.
    val tops = remember(view.years.size) { FloatArray(view.years.size) }
    Column(modifier) {
        SectionHeading(view.yearsHeading, seed = seed, gap = 0.dp)
        Column(
            Modifier.fillMaxWidth().drawBehind {
                if (tops.isEmpty()) return@drawBehind
                val u = 1.dp.toPx()
                val frame = p.lining.copy(alpha = 0.78f)
                tops.forEachIndexed { i, y ->
                    drawPath(ruledLine(0f, y + 0.5f * u, size.width, y + 0.5f * u, (if (i == 0) 1.3f else 0.9f) * u, 81 + i, u), if (i == 0) frame else p.lining.copy(alpha = 0.36f))
                }
                drawPath(ruledLine(0f, size.height - 0.5f * u, size.width, size.height - 0.5f * u, 1.3f * u, 81 + tops.size, u), frame)
            },
        ) {
            view.years.forEachIndexed { i, y -> FeastYearRow(y, Modifier.onGloballyPositioned { tops[i] = it.positionInParent().y }, onDay) }
        }
    }
}

/**
 * A year on the feast's page (`.feast-years .find-year`): its number in old-style figures, a point
 * in that year's day colour, the date, and its status only where it differs from the feast's
 * usual. The row is a touch target tall and leads to that day.
 */
@Composable
private fun FeastYearRow(y: FeastYearView, modifier: Modifier, onDay: (LocalDate) -> Unit) {
    val p = LocalPalette.current
    val date = y.date?.toLocalDate()
    val spoken = listOf("${y.year}", y.dateLabel, y.label.takeUnless { y.usual }.orEmpty(), y.note).filter { it.isNotEmpty() }.joinToString(", ")
    val point = when (y.color) {
        "white" -> LocalOrnament.current.flat
        "" -> p.lining
        else -> railColor(y.color, p)
    }
    val dateStyle = Type.body.copy(fontSize = 17.28.sp, lineHeight = 25.sp, color = p.text)
    Box(
        modifier.fillMaxWidth().heightIn(min = 48.dp)
            .then(if (date != null) Modifier.tap(label = spoken, action = "open the day") { onDay(date) } else Modifier.clearAndSetSemantics { contentDescription = spoken })
            .padding(start = 4.8.dp, end = 4.8.dp, top = 6.7.dp, bottom = 6.1.dp),
        contentAlignment = Alignment.CenterStart,
    ) {
        Row {
            Text(
                "${y.year}",
                Modifier.width(46.4.dp).alignByBaseline(),
                style = Type.body.copy(fontSize = 17.28.sp, lineHeight = 25.sp, color = mix(p.titulus, p.muted, 0.55f), fontFeatureSettings = "onum", letterSpacing = 0.69.sp),
            )
            // The point stands at the middle of the date's line.
            Box(Modifier.padding(horizontal = 8.8.dp).width(11.2.dp).height(with(LocalDensity.current) { dateStyle.lineHeight.toDp() }), contentAlignment = Alignment.Center) {
                PaintPoint(4.5.dp, point.copy(alpha = point.alpha * 0.85f))
            }
            Column(Modifier.weight(1f).alignByBaseline()) {
                Text(y.dateLabel, style = dateStyle)
                if (!y.usual) Text(y.label, style = Type.body.copy(fontSize = 14.72.sp, lineHeight = 21.sp, color = keepingInk(y.kind, p), fontStyle = if (y.kind == "monastic" || y.kind == "not-kept") FontStyle.Italic else FontStyle.Normal))
                if (y.note.isNotEmpty()) Text(y.note, style = Type.small.copy(fontSize = 13.76.sp, lineHeight = 18.sp, color = p.muted))
            }
        }
    }
}

/**
 * A year's keeping in the search's results: its year, date and status on the day's colour rail,
 * leading to that day.
 */
@Composable
private fun YearRow(y: FeastYearView, first: Boolean, onDay: (LocalDate) -> Unit) {
    val p = LocalPalette.current
    val date = y.date?.toLocalDate()
    val spoken = listOf("${y.year}", y.dateLabel, y.label, y.note).filter { it.isNotEmpty() }.joinToString(", ")
    Column(Modifier.fillMaxWidth()) {
        if (!first) DottedRule(p.border)
        Row(
            Modifier.fillMaxWidth().height(IntrinsicSize.Min).heightIn(min = 44.dp)
                .then(if (date != null) Modifier.tap(label = spoken, action = "open the day") { onDay(date) } else Modifier.clearAndSetSemantics { contentDescription = spoken })
                .padding(vertical = 8.dp),
            verticalAlignment = Alignment.Top,
        ) {
            Box(Modifier.width(3.dp).fillMaxHeight().background(railColor(y.color, p)))
            Text("${y.year}", Modifier.padding(start = 10.dp).width(44.dp), style = Type.body.copy(fontSize = 16.sp, lineHeight = 24.sp, color = p.muted, fontFeatureSettings = "onum"))
            Column(Modifier.weight(1f)) {
                Text(y.dateLabel, style = Type.body.copy(fontSize = 17.6.sp, lineHeight = 24.sp, color = p.text))
                if (!y.usual) Text(y.label, style = Type.body.copy(fontSize = 15.2.sp, lineHeight = 21.sp, color = keepingInk(y.kind, p), fontStyle = if (y.kind == "monastic" || y.kind == "not-kept") FontStyle.Italic else FontStyle.Normal))
                if (y.note.isNotEmpty()) Text(y.note, style = Type.small.copy(fontSize = 13.8.sp, lineHeight = 18.sp, color = p.muted))
            }
            if (date != null) Text("›", Modifier.align(Alignment.CenterVertically).padding(start = 8.dp), style = Type.body.copy(fontSize = 16.sp, color = p.goldLine))
        }
    }
}

@Composable
private fun DottedRule(color: Color) {
    Canvas(Modifier.fillMaxWidth().height(1.dp)) {
        val dot = 1.dp.toPx()
        var x = 0f
        while (x < size.width) {
            drawRect(color, Offset(x, 0f), androidx.compose.ui.geometry.Size(dot, dot))
            x += dot * 3
        }
    }
}

/** A year's status in its ink: commemorated red, transferred slate blue, the rest the text's or muted. */
private fun keepingInk(kind: String, p: Palette): Color = when (kind) {
    "commemorated" -> p.rubric
    "transferred" -> p.kalendarBlue
    "monastic" -> p.titulus
    "not-kept" -> p.muted
    else -> p.text
}

/** A feast's name in its rank's ink, as the ordo marks ranks. */
private fun rankTone(tone: String, p: Palette): Color = when (tone) {
    "greater" -> p.rubric
    "double" -> p.kalendarBlue
    else -> p.text
}

/**
 * The search field, as the web's: a magnifier, the words, and a gold rule beneath. It searches as
 * the reader types, a moment after the last key, and puts the cursor in itself when empty.
 */
@Composable
fun SearchField(query: String, onQuery: (String) -> Unit, modifier: Modifier, autofocus: Boolean = true) {
    val p = LocalPalette.current
    var text by rememberSaveable { mutableStateOf(query) }
    val focus = remember { FocusRequester() }
    val focusManager = LocalFocusManager.current
    LaunchedEffect(text) {
        if (text != query) {
            delay(250)
            onQuery(text)
        }
    }
    LaunchedEffect(Unit) { if (autofocus && query.isEmpty()) runCatching { focus.requestFocus() } }
    Row(
        modifier.drawBehind {
            val w = 1.dp.toPx()
            drawLine(p.goldLine, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
        }.heightIn(min = 48.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Magnifier(p.goldLine)
        Spacer(Modifier.width(10.dp))
        Box(Modifier.weight(1f), contentAlignment = Alignment.CenterStart) {
            if (text.isEmpty()) {
                Text(FEAST_WORDS.findPrompt, Modifier.clearAndSetSemantics {}, style = Type.body.copy(fontSize = 17.sp, lineHeight = 24.sp, color = p.muted, fontStyle = FontStyle.Italic))
            }
            BasicTextField(
                value = text,
                onValueChange = { text = it.take(100) },
                modifier = Modifier.fillMaxWidth().focusRequester(focus).semantics { contentDescription = FEAST_WORDS.findPrompt },
                singleLine = true,
                textStyle = Type.body.copy(fontSize = 17.sp, lineHeight = 24.sp, color = p.text),
                cursorBrush = SolidColor(p.accent),
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Words, autoCorrectEnabled = false, imeAction = ImeAction.Search),
                keyboardActions = KeyboardActions(onSearch = { onQuery(text); focusManager.clearFocus() }),
            )
        }
        if (text.isNotEmpty()) {
            Text(
                "Clear",
                Modifier.heightIn(min = 44.dp).tap(label = "Clear the search") { text = ""; onQuery("") }.padding(start = 12.dp, top = 12.dp, bottom = 12.dp),
                style = Type.small.copy(fontSize = 15.2.sp, color = p.accent),
            )
        }
    }
}

/**
 * The ordo's way into the search: the field as the web draws it on the ordo, which opens the
 * search page with the cursor in its field.
 */
@Composable
fun SearchEntry(onOpen: () -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    Row(
        modifier.drawBehind {
            val w = 1.dp.toPx()
            drawLine(p.goldLine, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
        }.heightIn(min = 48.dp).tap(label = FEAST_WORDS.findPrompt, action = "search", onClick = onOpen),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Magnifier(p.goldLine)
        Spacer(Modifier.width(10.dp))
        Text(FEAST_WORDS.findPrompt, Modifier.weight(1f), style = Type.body.copy(fontSize = 17.sp, lineHeight = 24.sp, color = p.muted, fontStyle = FontStyle.Italic))
    }
}

/** The search's magnifier (macros.html `feast_search`), viewBox 20×20. */
@Composable
private fun Magnifier(ink: Color) {
    Canvas(Modifier.size(17.dp).clearAndSetSemantics {}) {
        val u = size.width / 20f
        val stroke = Stroke(1.6f * u, cap = StrokeCap.Round)
        drawCircle(ink, radius = 5.5f * u, center = Offset(8.5f * u, 8.5f * u), style = stroke)
        drawLine(ink, Offset(12.6f * u, 12.6f * u), Offset(17f * u, 17f * u), 1.6f * u, cap = StrokeCap.Round)
    }
}
