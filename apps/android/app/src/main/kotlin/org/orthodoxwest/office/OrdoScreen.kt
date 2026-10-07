package org.orthodoxwest.office

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.text.InlineTextContent
import androidx.compose.foundation.text.appendInlineContent
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.SnapshotStateMap
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.hideFromAccessibility
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.Placeholder
import androidx.compose.ui.text.PlaceholderVerticalAlign
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import java.time.LocalDate
import java.time.Month
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle as JavaTextStyle
import java.util.Locale
import kotlinx.coroutines.launch
import org.orthodoxwest.office.core.CommemorationView
import org.orthodoxwest.office.core.OrdoDayView
import org.orthodoxwest.office.core.OrdoMonthView
import org.orthodoxwest.office.core.OrdoYearView
import org.orthodoxwest.office.core.TabulaRowView

/** One month of the ordo, as the web's month page: year and month navigation, then the days. */
@Composable
fun OrdoScreen(
    month: OrdoMonthView?,
    year: Int,
    monthNumber: Int,
    today: LocalDate,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onMonth: (Int, Int) -> Unit,
    onToday: () -> Unit,
    onYear: (Int) -> Unit,
    onDay: (LocalDate) -> Unit,
    focusDay: Int = 0,
) {
    val p = LocalPalette.current
    val wide = LocalWide.current
    var allDetails by rememberSaveable(year, monthNumber) { mutableStateOf(false) }
    // A day asked for (the web's #d-date) is brought into view once, when the month is ready:
    // from the first frame when it already is, so the page comes in at that day. After that
    // the reader's own scroll position stands, restored or not.
    val focusItem = FIRST_DAY_ITEM + (if (wide) 1 else 0) + focusDay - 1
    var focused by rememberSaveable(year, monthNumber, focusDay) { mutableStateOf(focusDay == 0 || month != null) }
    val listState = rememberSaveable(year, monthNumber, saver = LazyListState.Saver) {
        LazyListState(if (focusDay != 0 && month != null) focusItem else 0)
    }
    val scope = rememberCoroutineScope()
    LaunchedEffect(month != null, focused) {
        if (month != null && !focused) {
            listState.scrollToItem(focusItem)
            focused = true
        }
    }
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "chrome") { chrome() }
        item(key = "header") { OrdoHeader(year, monthNumber, null, today, onMonth, onToday, onYear, ordoColumn().padding(top = 17.6.dp)) }
        item(key = "tools") {
            Row(ordoColumn().padding(top = 8.dp).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "§ Fasting · ",
                    style = Type.small.copy(fontSize = 12.sp, lineHeight = 17.4.sp, color = p.muted),
                )
                FishIcon(p.muted, Modifier.padding(end = 3.dp))
                Text("Abstinence", style = Type.small.copy(fontSize = 12.sp, lineHeight = 17.4.sp, color = p.muted))
                Spacer(Modifier.weight(1f))
                Text(
                    if (allDetails) "Hide office details" else "Show office details",
                    Modifier.tap { allDetails = !allDetails }.padding(vertical = 12.dp),
                    style = Type.small.copy(color = p.accent, textDecoration = TextDecoration.Underline),
                )
            }
        }
        item(key = "ranks") { RankKey(ordoColumn().padding(bottom = 5.6.dp)) }
        item(key = "month") {
            MonthHeading(Month.of(monthNumber).getDisplayName(JavaTextStyle.FULL, Locale.US), today.year == year && today.monthValue == monthNumber, ordoColumn().padding(top = 10.dp)) {
                scope.launch { listState.animateScrollToItem(0) }
            }
        }
        if (wide) item(key = "columns") { DayColumns(ordoColumn()) }
        if (month == null) {
            item(key = "wait") { Text("Preparing the month…", ordoColumn().padding(vertical = 32.dp), style = Type.small.copy(color = p.muted, textAlign = TextAlign.Center)) }
        } else {
            itemsIndexed(month.days, key = { _, d -> "d-${d.date.day}" }) { _, d ->
                DayRow(d, LocalDate.of(d.date.year, d.date.month, d.date.day) == today, allDetails, onDay, ordoColumn())
            }
        }
        item(key = "continuation") {
            val prev = if (monthNumber == 1) year - 1 to 12 else year to monthNumber - 1
            val next = if (monthNumber == 12) year + 1 to 1 else year to monthNumber + 1
            Continuation(
                previousLabel = "Previous month",
                previous = Month.of(prev.second).getDisplayName(JavaTextStyle.FULL, Locale.US),
                onPrevious = { onMonth(prev.first, prev.second) },
                middle = "$year Ordo",
                onMiddle = { onYear(year) },
                nextLabel = "Next month",
                next = Month.of(next.second).getDisplayName(JavaTextStyle.FULL, Locale.US),
                onNext = { onMonth(next.first, next.second) },
                modifier = ordoColumn().padding(top = 44.dp),
            )
        }
        item(key = "footer") { Footer() }
    }
}

/** The ordo's column: the prayer measure on a phone, the web's 56rem calendar on a wide screen. */
@Composable
private fun ordoColumn(): Modifier = if (LocalWide.current) Modifier.widthIn(max = 896.dp).fillMaxWidth().padding(horizontal = 24.dp) else Modifier.measure()

/**
 * The ordo's title, year navigation and month strip. `month` is the month shown, or null on the
 * year's frontispiece, which takes "Anno Domini" and `roman` for its subtitle. A wide screen sets
 * the title beside the year's navigation, and the twelve months on one line.
 */
@Composable
private fun OrdoHeader(
    year: Int,
    month: Int?,
    roman: String?,
    today: LocalDate,
    onMonth: (Int, Int) -> Unit,
    onToday: () -> Unit,
    onYear: (Int) -> Unit,
    modifier: Modifier,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val wide = LocalWide.current
    // Previous and next year keep the page's kind: a month's, or the frontispiece.
    val sameView = { y: Int -> if (month == null) onYear(y) else onMonth(y, month) }
    val title: @Composable () -> Unit = {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Headpiece()
            Text(
                "$year Ordo",
                Modifier.padding(top = 2.dp).semantics { heading() },
                style = Type.body.copy(fontSize = if (wide) 34.4.sp else 28.sp, lineHeight = if (wide) 39.6.sp else 34.sp, color = p.text, fontFeatureSettings = "lnum"),
            )
            // The frontispiece reads as the printed ordo's title page.
            val subtitle = if (month == null && !roman.isNullOrEmpty()) "Anno Domini $roman" else "Feasts & daily observances"
            Text(subtitle, Modifier.padding(top = 4.8.dp), style = Type.small.copy(color = p.muted, fontSize = if (wide) 13.6.sp else Type.small.fontSize))
        }
    }
    val yearNav: @Composable (Modifier) -> Unit = { m ->
        Row(m.height(IntrinsicSize.Min).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
            val nav = Type.body.copy(fontSize = 12.8.sp, lineHeight = 20.5.sp, color = p.accent, fontFeatureSettings = "lnum")
            val cell = if (wide) Modifier else Modifier.weight(1f)
            Text("‹ ${year - 1}", cell.tap(label = "Previous year, ${year - 1}") { sameView(year - 1) }.padding(12.dp), style = nav.copy(textAlign = TextAlign.Center))
            Divider(p.border)
            Text("Today", cell.tap(action = "open today in the ordo", onClick = onToday).padding(12.dp).goldUnderline(today.year == year, p.goldLine, if (wide) 12.dp else 24.dp), style = nav.copy(textAlign = TextAlign.Center))
            Divider(p.border)
            Text("${year + 1} ›", cell.tap(label = "Next year, ${year + 1}") { sameView(year + 1) }.padding(12.dp), style = nav.copy(textAlign = TextAlign.Center))
        }
    }
    // The month strip: the shown month underlined in gold, and today's month marked with the
    // lozenge, on its underline or, on a wide screen, on the strip's hairline beneath.
    val strip: @Composable () -> Unit = {
        Month.entries.chunked(if (wide) 12 else 6).forEach { row ->
            Row(Modifier.fillMaxWidth()) {
                row.forEach { m ->
                    val current = m.value == month
                    val todays = today.year == year && today.monthValue == m.value
                    Box(
                        Modifier.weight(1f).heightIn(min = 44.dp)
                            .then(
                                if (!wide) Modifier else Modifier.drawBehind {
                                    val y = size.height - 1.dp.toPx()
                                    if (current) drawLine(p.goldLine, Offset(0f, y), Offset(size.width, y), 2.dp.toPx())
                                    if (todays) lozenge(Offset(size.width / 2f, if (current) y else size.height - 0.5.dp.toPx()), 3.dp.toPx(), o.flat, null)
                                },
                            )
                            .tap(label = m.getDisplayName(JavaTextStyle.FULL, Locale.US), selected = current) { onMonth(year, m.value) },
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(
                            m.getDisplayName(JavaTextStyle.SHORT, Locale.US).uppercase(),
                            Modifier.drawBehind {
                                if (wide) return@drawBehind
                                val y = size.height + 4.dp.toPx()
                                if (current) drawLine(p.goldLine, Offset(-14.dp.toPx(), y), Offset(size.width + 14.dp.toPx(), y), 2.dp.toPx())
                                if (todays) lozenge(Offset(size.width / 2f, y), 3.dp.toPx(), o.flat, null)
                            },
                            style = Type.label(12.48f, 0.06f).copy(color = if (current) (if (wide) p.accent else p.text) else p.muted),
                        )
                    }
                }
            }
        }
    }
    val rules = Modifier.drawBehind {
        // A hairline below; the ordo opens like the hours, its headpiece alone with no rule above.
        val w = 1.dp.toPx()
        drawLine(p.border, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
    }
    if (wide) {
        // The web's desktop header: the title beside the year's navigation, then the months on one line.
        Column(modifier) {
            Row(Modifier.fillMaxWidth().then(rules).padding(top = 13.4.dp, bottom = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                title()
                Spacer(Modifier.weight(1f).widthIn(min = 32.dp))
                yearNav(Modifier)
            }
            Column(Modifier.fillMaxWidth().drawBehind { drawLine(p.border, Offset(0f, size.height - 0.5.dp.toPx()), Offset(size.width, size.height - 0.5.dp.toPx()), 1.dp.toPx()) }) { strip() }
        }
    } else {
        Column(modifier.then(rules).padding(top = 13.4.dp, bottom = 16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
            title()
            yearNav(Modifier.fillMaxWidth().padding(top = 8.dp))
            Hairline(p.border, Modifier.padding(top = 8.dp))
            strip()
        }
    }
}

/** The first day's item in a month's list: after the chrome, header, tools and month heading (and a wide screen's column headings). */
private const val FIRST_DAY_ITEM = 4

/**
 * A year's frontispiece, as the web's /calendar/{year}: the title page, the month strip, and the
 * Tabula Temporaria. Each date leads to its day in the ordo.
 */
@Composable
fun OrdoYearScreen(
    view: OrdoYearView,
    today: LocalDate,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onMonth: (Int, Int) -> Unit,
    onToday: () -> Unit,
    onYear: (Int) -> Unit,
    onDay: (LocalDate) -> Unit,
) {
    val p = LocalPalette.current
    val listState = rememberSaveable(view.year, saver = LazyListState.Saver) { LazyListState() }
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "chrome") { chrome() }
        item(key = "header") { OrdoHeader(view.year, null, view.roman, today, onMonth, onToday, onYear, ordoColumn().padding(top = 17.6.dp)) }
        item(key = "tabula") {
            Column(ordoColumn().padding(top = 40.dp)) {
                TabulaHeading()
                if (LocalWide.current) {
                    // The web's desktop tabula: the figures in one line, the two tables side by side.
                    Figures(view.figures, 4, Modifier.padding(top = 24.dp))
                    Row(Modifier.padding(top = 24.dp), horizontalArrangement = Arrangement.spacedBy(40.dp)) {
                        TabulaTable("Moveable feasts", view.moveable, onDay, Modifier.weight(1f))
                        TabulaTable("Ember days", view.ember, onDay, Modifier.weight(1f))
                    }
                } else {
                    Figures(view.figures, 2, Modifier.padding(top = 24.dp))
                    TabulaTable("Moveable feasts", view.moveable, onDay, Modifier.padding(top = 24.dp))
                    TabulaTable("Ember days", view.ember, onDay, Modifier.padding(top = 24.dp))
                }
            }
        }
        item(key = "footer") { Footer() }
    }
}

/** "Tabula Temporaria", a titulus in small capitals over the ornament's double rule, its lozenge at the centre. */
@Composable
private fun TabulaHeading() {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    Text(
        "Tabula Temporaria",
        Modifier.fillMaxWidth().semantics { heading() }.drawBehind {
            val w = 1.dp.toPx()
            drawLine(o.line, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
            drawLine(o.line, Offset(0f, size.height - w * 2.5f), Offset(size.width, size.height - w * 2.5f), w)
            // Ringed in the page's ground, as the web's box-shadow parts the rules around it.
            val c = Offset(size.width / 2f, size.height - w * 1.5f)
            lozenge(c, 4.5.dp.toPx(), p.bg, null)
            lozenge(c, 2.5.dp.toPx(), o.flat, null)
        }.padding(vertical = 6.4.dp),
        style = Type.body.copy(fontSize = 21.6.sp, lineHeight = 28.sp, color = p.titulus, fontFeatureSettings = "smcp", letterSpacing = 1.3.sp),
    )
}

/**
 * The year's four figures, `perRow` to a row, on a painted tablet: the surface thinned so the wall
 * shows through, framed in the lining ruled twice (a thinner line 3dp inside the first), with a
 * quatrefoil knop at each corner where the rules stop short. Each numeral large in the accent, its
 * name beneath.
 */
@Composable
private fun Figures(figures: List<TabulaRowView>, perRow: Int, modifier: Modifier) {
    val p = LocalPalette.current
    Column(
        modifier.fillMaxWidth()
            .background(p.surface.copy(alpha = p.surface.alpha * 0.6f))
            .drawBehind {
                val knop = 14.dp.toPx()
                val w = 1.dp.toPx()
                val inner = p.lining.copy(alpha = 0.4f)
                for ((inset, ink) in listOf(0f to p.lining, 3.dp.toPx() to inner)) {
                    val a = inset + w / 2f
                    drawLine(ink, Offset(knop, a), Offset(size.width - knop, a), w)
                    drawLine(ink, Offset(knop, size.height - a), Offset(size.width - knop, size.height - a), w)
                    drawLine(ink, Offset(a, knop), Offset(a, size.height - knop), w)
                    drawLine(ink, Offset(size.width - a, knop), Offset(size.width - a, size.height - knop), w)
                }
                for (x in listOf(0f, size.width - knop)) {
                    for (y in listOf(0f, size.height - knop)) quatrefoil(Offset(x, y), knop, p.lining)
                }
            }
            .padding(start = 6.4.dp, end = 6.4.dp, top = 17.6.dp, bottom = 16.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp),
    ) {
        figures.chunked(perRow).forEach { pair ->
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                pair.forEach { f ->
                    Column(
                        Modifier.weight(1f).semantics(mergeDescendants = true) {},
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        // Lining figures stand 4 and 25 level with XIII and D.
                        Text(f.value, style = Type.body.copy(fontSize = 25.6.sp, lineHeight = 30.7.sp, color = p.accent, fontFeatureSettings = "lnum"))
                        Text(f.label.uppercase(), Modifier.padding(top = 3.dp), style = Type.label(11.2f, 0.06f).copy(color = p.muted, textAlign = TextAlign.Center))
                    }
                }
            }
        }
    }
}

/** A titled list of dates, ruled between rows; a date that cannot share its row takes the next line whole. */
@Composable
private fun TabulaTable(title: String, rows: List<TabulaRowView>, onDay: (LocalDate) -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    Column(modifier.fillMaxWidth()) {
        Text(title.uppercase(), Modifier.fillMaxWidth().semantics { heading() }.padding(vertical = 5.6.dp), style = Type.label(11.2f, 0.06f).copy(color = p.muted))
        Hairline(p.border)
        rows.forEach { r ->
            val date = r.date?.toLocalDate()
            FlowRow(
                Modifier.fillMaxWidth().heightIn(min = 44.dp)
                    .then(if (date != null) Modifier.tap(label = "${r.label}, ${r.value}", action = "open the day in the ordo") { onDay(date) } else Modifier)
                    .padding(vertical = 8.8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalArrangement = Arrangement.Center,
            ) {
                Text(r.label, Modifier.padding(end = 16.dp), style = Type.body.copy(color = p.text))
                Text(r.value, Modifier.weight(1f, fill = false).fillMaxWidth(), softWrap = false, style = Type.body.copy(color = p.accent, textAlign = TextAlign.End))
            }
            Hairline(p.border)
        }
    }
}

/** A Kalendar's month: its rubricated heading, in the titulus, over the ornament's double rule. */
@Composable
private fun MonthHeading(name: String, isTodaysMonth: Boolean, modifier: Modifier, onTop: () -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    Row(
        modifier.drawBehind {
            val w = 1.dp.toPx()
            drawLine(o.line, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
            drawLine(o.line, Offset(0f, size.height - w * 2.5f), Offset(size.width, size.height - w * 2.5f), w)
            if (isTodaysMonth) {
                // A 6dp square in the lining, ringed 2dp in the page's ground where it parts the rule.
                val c = Offset(size.width / 2f, size.height - w * 1.5f)
                lozenge(c, 7.07.dp.toPx(), p.bg, null)
                lozenge(c, 4.24.dp.toPx(), p.lining, null)
            }
        }.padding(vertical = 6.4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(name, Modifier.weight(1f).semantics { heading() }, style = Type.body.copy(fontSize = 21.6.sp, lineHeight = 28.sp, color = p.titulus, fontFeatureSettings = "smcp", letterSpacing = 1.3.sp))
        Text("↑", Modifier.tap(label = "Back to the top", onClick = onTop).padding(horizontal = 14.dp, vertical = 6.dp), style = Type.body.copy(fontSize = 16.sp, lineHeight = 20.sp, color = p.muted))
    }
}

@Composable
private fun DayRow(d: OrdoDayView, isToday: Boolean, allDetails: Boolean, onDay: (LocalDate) -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    val open = rememberSaveable(d.date, saver = Disclosures) { mutableStateMapOf<String, Boolean>() }
    val details = open["details"] ?: allDetails
    val date = LocalDate.of(d.date.year, d.date.month, d.date.day)
    val hasDetails = d.hasLauds() || d.hoursPreces || d.hasVespers()
    // One stop for a screen reader: the date, its colour and observances, and the feast.
    val spoken = listOfNotNull(
        date.format(DateTimeFormatter.ofPattern("EEEE, MMMM d", Locale.US)) + if (isToday) ", today" else "",
        d.feast,
        "liturgical color: ${d.color}",
        "fasting".takeIf { d.fast },
        "abstinence".takeIf { d.abstinence },
        d.rank.takeIf { it.isNotEmpty() }?.let { "rank $it" },
    ).joinToString(". ")
    val buttons: @Composable RowScope.() -> Unit = {
        if (d.commemorations.isNotEmpty()) {
            val n = d.commemorations.size
            SmallDisclosure("$n commemoration${if (n > 1) "s" else ""}", open["comms"] == true) { open["comms"] = open["comms"] != true }
        }
        if (hasDetails) SmallDisclosure("Office details", details) { open["details"] = !details }
    }
    val unfolded: @Composable ColumnScope.() -> Unit = {
        Unfold(open["comms"] == true) { d.commemorations.forEach { Text(it, style = Type.small.copy(fontSize = 13.6.sp, lineHeight = 20.4.sp, color = p.muted, fontStyle = FontStyle.Italic)) } }
        // Shown for the whole month at once, the rows stay where they are.
        Unfold(details && hasDetails, reveal = open["details"] != null) { Digest(d) }
    }
    val anyOpen = open["comms"] == true || (details && hasDetails)
    if (LocalWide.current) return DayTableRow(d, date, isToday, spoken, onDay, modifier, anyOpen, buttons, unfolded)
    val body: @Composable ColumnScope.() -> Unit = {
        Row(horizontalArrangement = Arrangement.spacedBy(9.6.dp)) { buttons() }
        unfolded()
    }
    // Today is painted, not selected: a ground of the frieze's wash ruled top and bottom in the gold
    // line, inset 8dp from the gutters, the day's number in gold.
    Column(
        modifier.then(
            if (!isToday) Modifier else Modifier.drawBehind {
                val inset = 8.dp.toPx()
                val w = 1.dp.toPx()
                // Above the row's hairline.
                val h = size.height - w
                drawRect(p.inscriptionWash, Offset(inset, 0f), Size(size.width - 2 * inset, h))
                drawRect(p.goldLine, Offset(inset, 0f), Size(size.width - 2 * inset, w))
                drawRect(p.goldLine, Offset(inset, h - w), Size(size.width - 2 * inset, w))
            },
        ),
    ) {
        Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min).padding(top = 12.8.dp, bottom = 6.4.dp)) {
            // The day's liturgical colour as a rail beside its date.
            Box(Modifier.width(3.dp).height(40.dp).background(dayColor(d.color)))
            Column(Modifier.width(59.dp).tap(label = spoken, action = "open the day") { onDay(date) }, horizontalAlignment = Alignment.CenterHorizontally) {
                Text("${d.date.day}", style = Type.body.copy(fontSize = 21.6.sp, lineHeight = 23.76.sp, color = if (isToday) p.gold else p.accent, fontFeatureSettings = "lnum"))
                Text(d.weekday.uppercase(), style = Type.label(12f, 0.06f).copy(color = p.muted, lineHeight = 16.8.sp))
                if (isToday) Text("Today", style = Type.small.copy(fontSize = 11.sp, lineHeight = 14.sp, color = p.accent))
            }
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.Top) {
                    // The date's stop already says the feast; the feast is a second target for the eye only.
                    FeastName(d, Type.body.copy(fontSize = 16.sp, lineHeight = 21.6.sp), Modifier.weight(1f).tap { onDay(date) }.semantics { hideFromAccessibility() })
                    // The phone's marks stay quiet, as the web's card sets them: the name carries the rank's ink.
                    Row(Modifier.padding(start = 8.dp, top = 2.dp).clearAndSetSemantics {}, horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                        if (d.fast) Text("§", style = Type.small.copy(fontSize = 12.sp, color = p.muted))
                        if (d.abstinence) FishIcon(p.muted)
                        if (d.rank.isNotEmpty()) Text(d.rank, style = Type.small.copy(fontSize = 12.sp, lineHeight = 16.8.sp, color = p.muted))
                    }
                }
                body()
            }
        }
        Hairline(p.border)
    }
}

/** The desktop table's column widths (`.month-table`): day, weekday, the feast, fasting, abstinence, rank. */
private val DayCol = 44.8.dp
private val WeekCol = 48.dp
private val FlagCol = 41.6.dp
private val RankCol = 51.2.dp

/** The table's column headings, over a hairline. */
@Composable
private fun DayColumns(modifier: Modifier) {
    val p = LocalPalette.current
    val th = Type.label(11.2f, 0.06f).copy(color = p.muted)
    Column(modifier.clearAndSetSemantics {}) {
        Row(Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 5.6.dp)) {
            Text("DAY", Modifier.width(DayCol).padding(start = 10.4.dp), style = th.copy(textAlign = TextAlign.Center))
            Text("WK", Modifier.width(WeekCol).padding(horizontal = 7.2.dp), style = th)
            Text("FEAST / OBSERVANCE", Modifier.weight(1f).padding(horizontal = 7.2.dp), style = th)
            Text("FAST", Modifier.width(FlagCol), style = th.copy(textAlign = TextAlign.Center))
            Text("ABST.", Modifier.width(FlagCol), style = th.copy(textAlign = TextAlign.Center))
            Text("RANK", Modifier.width(RankCol).padding(end = 7.2.dp), style = th.copy(textAlign = TextAlign.End))
        }
        Hairline(p.border)
    }
}

/**
 * A day as a row of the desktop table: its colour as a rail beside the date, then the columns. The
 * disclosures share the feast's line, 1.25rem after the name (the web's inline `.day-disclosures`
 * from 701px), so a month reads as one line per day; one that is open (`anyOpen`) drops below the
 * name at the column's full width, with what it unfolds.
 */
@Composable
private fun DayTableRow(
    d: OrdoDayView,
    date: LocalDate,
    isToday: Boolean,
    spoken: String,
    onDay: (LocalDate) -> Unit,
    modifier: Modifier,
    anyOpen: Boolean,
    buttons: @Composable RowScope.() -> Unit,
    unfolded: @Composable ColumnScope.() -> Unit,
) {
    val p = LocalPalette.current
    val rail = dayColor(d.color)
    // Today is painted, not selected: the frieze's wash, ruled top and bottom in the gold line
    // across the row, the day's number in gold.
    Column(
        modifier.then(
            if (!isToday) Modifier else Modifier.drawBehind {
                val w = 1.dp.toPx()
                val h = size.height - w
                drawRect(p.inscriptionWash, size = Size(size.width, h))
                drawRect(p.goldLine, size = Size(size.width, w))
                drawRect(p.goldLine, Offset(0f, h - w), Size(size.width, w))
            },
        ),
    ) {
        // The web's `.month-table td`: 0.5rem above and below, from 701px.
        Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min).padding(vertical = 8.dp)) {
            Column(
                Modifier.width(DayCol).fillMaxHeight()
                    .drawBehind { drawRect(rail, Offset(0f, 1.6.dp.toPx()), Size(3.dp.toPx(), size.height - 1.6.dp.toPx())) }
                    .tap(label = spoken, action = "open the day") { onDay(date) }
                    .padding(start = 10.4.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text("${d.date.day}", style = Type.body.copy(fontSize = 19.2.sp, lineHeight = 23.sp, color = if (isToday) p.gold else p.accent, fontFeatureSettings = "lnum"))
                if (isToday) Text("Today", style = Type.small.copy(fontSize = 11.2.sp, lineHeight = 15.7.sp, color = p.accent))
            }
            Text(d.weekday, Modifier.width(WeekCol).padding(horizontal = 7.2.dp, vertical = 3.dp).clearAndSetSemantics {}, style = Type.small.copy(fontSize = 12.8.sp, color = p.muted))
            Column(Modifier.weight(1f).padding(horizontal = 7.2.dp)) {
                // The date's stop already says the feast; the feast is a second target for the eye only.
                val name = Type.body.copy(fontSize = 16.sp, lineHeight = 22.4.sp)
                if (anyOpen) {
                    FeastName(d, name, Modifier.tap { onDay(date) }.semantics { hideFromAccessibility() })
                    Row(horizontalArrangement = Arrangement.spacedBy(9.6.dp)) { buttons() }
                } else {
                    Row {
                        FeastName(d, name, Modifier.weight(1f, fill = false).alignByBaseline().tap { onDay(date) }.semantics { hideFromAccessibility() })
                        Row(Modifier.padding(start = 20.dp).alignByBaseline(), horizontalArrangement = Arrangement.spacedBy(9.6.dp)) { buttons() }
                    }
                }
                unfolded()
            }
            // Fasting and abstinence stay quiet, so red in a row means rank.
            Box(Modifier.width(FlagCol).clearAndSetSemantics {}, contentAlignment = Alignment.TopCenter) {
                if (d.fast) Text("§", style = Type.body.copy(fontSize = 16.sp, color = p.muted, fontWeight = FontWeight.Bold))
            }
            Box(Modifier.width(FlagCol).padding(top = 6.dp).clearAndSetSemantics {}, contentAlignment = Alignment.TopCenter) {
                if (d.abstinence) FishIcon(p.muted)
            }
            // A Kalendar has no hyperlinks: the rank keeps its ink and loses the rule.
            Box(Modifier.width(RankCol).padding(end = 7.2.dp, top = 3.dp).clearAndSetSemantics {}, contentAlignment = Alignment.TopEnd) {
                if (d.rank.isNotEmpty()) Text(d.rank, style = Type.small.copy(fontSize = 12.48.sp, lineHeight = 17.5.sp, color = rankInk(d.rank, p)))
            }
        }
        Hairline(p.border)
    }
}

/** The ranks spelled out, each abbreviation in its days' ink, as the web's key: a phone has no titles to hover. */
private val RankNames = listOf(
    "1cl" to "first class",
    "2cl" to "second class",
    "gd" to "greater double",
    "d" to "double",
    "sd" to "semi-double",
    "s" to "simple",
    "f2" to "privileged feria",
)

@Composable
private fun RankKey(modifier: Modifier) {
    val p = LocalPalette.current
    // No-break spaces keep each abbreviation with its name when the line wraps.
    val key = buildAnnotatedString {
        RankNames.forEachIndexed { i, (abbr, name) ->
            if (i > 0) append(" · ")
            withStyle(SpanStyle(color = rankInk(abbr, p))) { append(abbr) }
            append("\u00A0" + name.replace(' ', '\u00A0'))
        }
    }
    Text(
        key,
        modifier.clearAndSetSemantics { contentDescription = "Ranks: " + RankNames.joinToString(", ") { it.second } },
        style = Type.small.copy(fontSize = 12.sp, lineHeight = 17.4.sp, color = p.muted),
    )
}

/**
 * A day's rank read from its ink, as in a Book of Hours: the great feasts red-letter, doubles
 * slate blue, lesser days black.
 */
private fun rankInk(rank: String, p: Palette): Color = when (rank) {
    "1cl", "2cl", "gd" -> p.rubric
    "d" -> p.kalendarBlue
    else -> p.text
}

/** A first-class feast's small painted cross, 0.62em with 0.32em after it, on the name's baseline. */
private val FirstClassCross = mapOf(
    "cross" to InlineTextContent(Placeholder(0.94.em, 0.62.em, PlaceholderVerticalAlign.AboveBaseline)) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.CenterStart) {
            PaintedCross(Modifier.fillMaxHeight().aspectRatio(1f, matchHeightConstraintsFirst = true))
        }
    },
)

/** A feast's name in its rank's ink, a first-class feast's after a small painted cross. */
@Composable
private fun FeastName(d: OrdoDayView, style: TextStyle, modifier: Modifier) {
    val first = d.rank == "1cl"
    Text(
        buildAnnotatedString {
            if (first) appendInlineContent("cross", "✠")
            append(d.feast)
        },
        modifier,
        style = style.copy(color = rankInk(d.rank, LocalPalette.current)),
        inlineContent = if (first) FirstClassCross else emptyMap(),
    )
}

private fun OrdoDayView.hasLauds() = benedictusAntiphon.isNotEmpty() || laudsPreces || laudsSuffrage || laudsComms.isNotEmpty()
private fun OrdoDayView.hasVespers() = magnificatAntiphon.isNotEmpty() || vespersPreces || vespersSuffrage || vespersComms.isNotEmpty() || vespersNote.isNotEmpty()

@Composable
private fun SmallDisclosure(label: String, open: Boolean, onToggle: () -> Unit) {
    Row(Modifier.heightIn(min = 44.dp).tap(onClick = onToggle).disclosed(open), verticalAlignment = Alignment.CenterVertically) {
        Text(label, style = Type.small.copy(fontSize = 12.sp, lineHeight = 16.8.sp, color = LocalPalette.current.muted))
        Caret(open, LocalPalette.current.muted)
    }
}

/** The office digest: each hour's gospel antiphon, preces, suffrage, and commemorations. */
@Composable
private fun Digest(d: OrdoDayView) {
    val p = LocalPalette.current
    Column(
        Modifier.fillMaxWidth().padding(bottom = 8.dp).drawBehind { drawLine(p.goldLine, Offset(0f, 0f), Offset(0f, size.height), 1.dp.toPx()) }.padding(8.dp),
    ) {
        if (d.hasLauds()) {
            DigestHour("Lauds", emptyList(), d.benedictusAntiphon.takeIf { it.isNotEmpty() }?.let { "Ben. “$it”" }, d.laudsPreces, d.laudsSuffrage, d.laudsComms)
        }
        if (d.hoursPreces) DigestHour("Hours", emptyList(), null, preces = true, suffrage = false, comms = emptyList())
        if (d.hasVespers()) {
            DigestHour("Vespers", listOfNotNull(d.vespersNote.takeIf { it.isNotEmpty() }), d.magnificatAntiphon.takeIf { it.isNotEmpty() }?.let { "Mag. “$it”" }, d.vespersPreces, d.vespersSuffrage, d.vespersComms)
        }
    }
}

@Composable
private fun DigestHour(name: String, notes: List<String>, antiphon: String?, preces: Boolean, suffrage: Boolean, comms: List<CommemorationView>) {
    val p = LocalPalette.current
    val line = Type.small.copy(fontSize = 13.6.sp, lineHeight = 20.4.sp, color = p.muted, fontStyle = FontStyle.Italic)
    Column(Modifier.padding(bottom = 4.dp)) {
        Text(name.uppercase(), Modifier.padding(bottom = 2.4.dp), style = Type.label(12f, 0.06f).copy(color = p.accent, lineHeight = 18.sp))
        notes.forEach { Text(it, style = line) }
        if (antiphon != null || preces || suffrage) {
            Text(
                buildAnnotatedString {
                    if (antiphon != null) append(antiphon)
                    withStyle(SpanStyle(color = p.rubric, fontStyle = FontStyle.Normal)) {
                        if (preces) append((if (length > 0) "  " else "") + "Preces")
                        if (suffrage) append((if (length > 0) "  " else "") + "Suffrage")
                    }
                },
                style = line,
            )
        }
        comms.forEach { c -> Text("Com. ${c.name}" + if (c.incipit.isNotEmpty()) " “${c.incipit}”" else "", style = line) }
    }
}

/** A day's disclosures the reader has opened or closed, as saved state can hold them: "details=1". */
private val Disclosures = listSaver<SnapshotStateMap<String, Boolean>, String>(
    save = { m -> m.map { (k, v) -> "$k=${if (v) 1 else 0}" } },
    restore = { l -> mutableStateMapOf<String, Boolean>().apply { l.forEach { put(it.substringBefore('='), it.endsWith("=1")) } } },
)
