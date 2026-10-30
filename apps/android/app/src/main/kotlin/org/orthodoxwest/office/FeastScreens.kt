package org.orthodoxwest.office

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
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
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.LiveRegionMode
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
 * A feast's page, as the web's /feast/{id}: its rank and date, the next day it is kept (leading to
 * that day), how it is kept in the next eight years, and its texts.
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
    val note = Type.small.copy(fontSize = 14.7.sp, lineHeight = 21.sp, color = p.muted)
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "chrome") { chrome() }
        item(key = "back") {
            Box(feastColumn()) {
                Text(
                    "‹ ${w.findTitle}",
                    Modifier.heightIn(min = 44.dp).tap(onClick = onFind).padding(vertical = 12.dp),
                    style = Type.small.copy(fontSize = 15.2.sp, lineHeight = 20.sp, color = p.accent),
                )
            }
        }
        item(key = "header") { FeastHeader(view.name, view.subtitle, ink = p.rubric, modifier = feastColumn()) }
        item(key = "facts") {
            Column(feastColumn().padding(top = 14.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
                Text(
                    if (view.dateLead.isEmpty()) AnnotatedString(view.date) else buildAnnotatedString {
                        append(view.dateLead + " ")
                        withStyle(SpanStyle(color = p.text)) { append(view.date) }
                    },
                    style = Type.body.copy(fontSize = 17.6.sp, lineHeight = 26.sp, color = p.muted),
                )
                if (view.monasticNote.isNotEmpty()) {
                    Column(
                        Modifier.fillMaxWidth().background(p.surface).drawBehind {
                            drawRect(p.goldLine, size = androidx.compose.ui.geometry.Size(3.dp.toPx(), size.height))
                        }.padding(start = 19.dp, end = 16.dp, top = 12.dp, bottom = 12.dp),
                    ) {
                        Text(w.monasticHeading, style = Type.body.copy(fontSize = 15.5.sp, lineHeight = 22.sp, color = p.titulus, fontFeatureSettings = "smcp", letterSpacing = 0.8.sp))
                        Text(view.monasticNote, style = Type.body.copy(fontSize = 15.5.sp, lineHeight = 22.5.sp, color = p.text))
                    }
                    view.keptByAll?.let { all ->
                        Text(
                            buildAnnotatedString {
                                append(w.everyoneKeeps + " ")
                                withStyle(SpanStyle(color = p.accent)) { append(all.name) }
                                append(" " + view.keptByAllNote)
                            },
                            Modifier.tap(action = "open the feast's page") { onFeast(all.id) },
                            style = note,
                        )
                    }
                }
                view.next?.let { next -> NextBox(next.label, next.dateLabel, next.color) { onDay(next.date.toLocalDate()) } }
            }
        }
        item(key = "years") {
            Column(feastColumn().padding(top = 28.dp)) {
                SectionHeading(view.yearsHeading)
                Text(w.yearsHint, Modifier.padding(top = 6.dp, bottom = 4.dp), style = note)
                view.years.forEachIndexed { i, y -> YearRow(y, first = i == 0, onDay = onDay) }
            }
        }
        if (view.texts.isNotEmpty()) {
            item(key = "texts") {
                Column(feastColumn().padding(top = 28.dp)) {
                    SectionHeading(w.textsHeading)
                    Text(view.textsSource, Modifier.padding(top = 6.dp), style = note.copy(fontStyle = FontStyle.Italic))
                    view.texts.forEach { t ->
                        Text(
                            buildAnnotatedString {
                                append(t.label)
                                if (t.common) {
                                    withStyle(SpanStyle(color = p.muted, fontStyle = FontStyle.Italic, fontFeatureSettings = "", letterSpacing = 0.sp, fontSize = 13.6.sp)) {
                                        append("  " + w.commonMark)
                                    }
                                }
                            },
                            Modifier.padding(top = 18.dp, bottom = 2.dp).semantics { heading() },
                            style = Type.body.copy(fontSize = 16.sp, lineHeight = 22.sp, color = p.rubric, fontFeatureSettings = "smcp", letterSpacing = 0.8.sp),
                        )
                        // Each text keeps the hour's rhythm between its blocks, a source's blank lines included.
                        val line = LocalDensity.current.bodyLine()
                        t.blocks.forEachIndexed { i, b -> Block(b, Modifier.padding(top = if (i == 0) 0.dp else b.space.of(line))) }
                    }
                    view.wholeOffice?.let { whole ->
                        Text(
                            buildAnnotatedString {
                                append(w.wholeOffice + " ")
                                withStyle(SpanStyle(color = p.accent)) { append(whole.label) }
                            },
                            Modifier.padding(top = 18.dp).tap(action = "open the day") { onDay(whole.date.toLocalDate()) },
                            style = note,
                        )
                    }
                }
            }
        }
        item(key = "footer") { Footer() }
    }
}

/** A page's headpiece, its title in `ink`, and its subtitle, over a hairline (the ordo's header). */
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

/** A section's heading in the tituli's small capitals over the lining (`.feast-section h2`). */
@Composable
private fun SectionHeading(text: String) {
    val p = LocalPalette.current
    Text(
        text,
        Modifier.fillMaxWidth().semantics { heading() }.drawBehind {
            val w = 1.dp.toPx()
            drawLine(p.lining, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
        }.padding(bottom = 5.dp),
        style = Type.body.copy(fontSize = 20.sp, lineHeight = 26.sp, color = p.titulus, fontFeatureSettings = "smcp", letterSpacing = 1.6.sp),
    )
}

/**
 * The next day the feast is kept, framed as the invitation is (the lining ruled twice), the day's
 * colour on its rail; a tap opens that day.
 */
@Composable
private fun NextBox(label: String, date: String, color: String, onOpen: () -> Unit) {
    val p = LocalPalette.current
    val rail = railColor(color, p)
    Column(
        Modifier.fillMaxWidth().padding(3.dp).drawBehind {
            val w = 1.dp.toPx()
            val out = 3.dp.toPx() + w / 2f
            drawRect(p.lining, Offset(-out, -out), androidx.compose.ui.geometry.Size(size.width + 2 * out, size.height + 2 * out), style = Stroke(w))
            drawRect(p.lining, Offset(w / 2f, w / 2f), androidx.compose.ui.geometry.Size(size.width - w, size.height - w), style = Stroke(w))
        }.background(p.surface).drawBehind {
            drawRect(rail, size = androidx.compose.ui.geometry.Size(3.dp.toPx(), size.height))
        }.tap(label = "$label: $date", action = "open the day", onClick = onOpen).padding(start = 17.6.dp, end = 16.dp, top = 12.8.dp, bottom = 13.6.dp),
    ) {
        Text(label, style = Type.body.copy(fontSize = 13.6.sp, lineHeight = 18.sp, color = p.muted, fontFeatureSettings = "smcp", letterSpacing = 1.1.sp))
        Row(Modifier.padding(top = 2.dp), verticalAlignment = Alignment.Bottom) {
            Text(date, Modifier.weight(1f), style = Type.body.copy(fontSize = 18.4.sp, lineHeight = 25.sp, color = p.text))
            Text("${FEAST_WORDS.daysHours} ›", Modifier.padding(start = 12.dp), style = Type.body.copy(fontSize = 16.sp, lineHeight = 25.sp, color = p.rubric))
        }
    }
}

/** A year's keeping: its year, date and status on the day's colour rail, leading to that day. */
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
                Text(y.label, style = Type.body.copy(fontSize = 15.2.sp, lineHeight = 21.sp, color = keepingInk(y.kind, p), fontStyle = if (y.kind == "monastic" || y.kind == "not-kept") FontStyle.Italic else FontStyle.Normal))
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
