package org.orthodoxwest.office

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutLinearInEasing
import androidx.compose.animation.core.LinearOutSlowInEasing
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.scrollBy
import androidx.compose.foundation.indication
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
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
import androidx.compose.foundation.layout.sizeIn
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
import androidx.compose.runtime.State
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
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
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.clipRect
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.layout.layout
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalDensity
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
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import java.time.LocalDate
import java.time.Month
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle as JavaTextStyle
import java.util.Locale
import kotlin.math.roundToInt
import kotlinx.coroutines.launch
import org.orthodoxwest.office.core.CommemorationView
import org.orthodoxwest.office.core.CourseMonthView
import org.orthodoxwest.office.core.OrdoDayView
import org.orthodoxwest.office.core.OrdoMonthView
import org.orthodoxwest.office.core.OrdoYearView
import org.orthodoxwest.office.core.TabulaRowView
import org.orthodoxwest.office.core.coursePlace
import org.orthodoxwest.office.core.courseSpan

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
    onFind: () -> Unit = {},
    onFeast: (String) -> Unit = {},
    focusDay: Int = 0,
) {
    val p = LocalPalette.current
    val wide = LocalWide.current
    var allDetails by rememberSaveable(year, monthNumber) { mutableStateOf(false) }
    // "Show office details" fades the month out, changes every day at once and fades it back in,
    // rather than thirty days unfolding together. A press while it turns is let go. The system's
    // animator scale times the fades, so with animations off the days simply change.
    val monthShade = remember { Animatable(1f) }
    var turningAll by remember { mutableStateOf(false) }
    // A day asked for (the web's #d-date) is brought into view once, when the month is ready:
    // from the first frame when it already is, so the page comes in at that day. After that
    // the reader's own scroll position stands, restored or not. The day before it comes to rest
    // under the heading, so the day is found among its neighbours, as the web keeps a row above
    // it (`scroll-margin-top`); the month's first day has none before it here.
    val focusItem = FIRST_DAY_ITEM + (if (wide) 1 else 0) + maxOf(1, focusDay - 1) - 1
    var focused by rememberSaveable(year, monthNumber, focusDay) { mutableStateOf(focusDay == 0 || month != null) }
    // The day comes to rest under the month's pinned heading, not behind it: the heading's
    // measured height, or before its first layout an estimate (its padding, and the taller of
    // its name's line and the ↑'s 48dp).
    var headingPx by remember { mutableIntStateOf(0) }
    val estimate = with(LocalDensity.current) { (10.dp.toPx() + maxOf(28.sp.toPx() + 12.8.dp.toPx(), 48.dp.toPx())).roundToInt() }
    val pinned = if (headingPx > 0) headingPx else estimate
    // Where a day was brought to rest by the estimate, set right once the heading is measured.
    val restedAt = remember(year, monthNumber, focusDay) { mutableIntStateOf(0) }
    val listState = rememberSaveable(year, monthNumber, saver = LazyListState.Saver) {
        if (focusDay != 0 && month != null) restedAt.intValue = pinned
        LazyListState(if (focusDay != 0 && month != null) focusItem else 0, if (focusDay != 0 && month != null) -pinned else 0)
    }
    val scope = rememberCoroutineScope()
    // How far the month's heading stands under the status bar: none while the rows above it hold
    // it clear, the whole bar once it is pinned.
    val barPx = with(LocalDensity.current) { insets.calculateTopPadding().roundToPx() }
    val underBar by remember(barPx, listState) {
        derivedStateOf {
            val above = listState.layoutInfo.visibleItemsInfo.firstOrNull { it.key == "ranks" }
            if (above == null) barPx else (-(above.offset + above.size)).coerceIn(0, barPx)
        }
    }
    LaunchedEffect(month != null, focused) {
        if (month != null && !focused) {
            listState.scrollToItem(focusItem, -pinned)
            restedAt.intValue = if (headingPx > 0) 0 else pinned
            focused = true
        }
    }
    LaunchedEffect(headingPx) {
        val rested = restedAt.intValue
        if (headingPx > 0 && rested > 0) {
            if (rested != headingPx) listState.scrollBy((rested - headingPx).toFloat())
            restedAt.intValue = 0
        }
    }
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "chrome") { chrome() }
        item(key = "header") { OrdoHeader(year, monthNumber, null, month?.course, today, { 1f }, onMonth, onToday, onYear, ordoColumn().padding(top = 17.6.dp)) }
        // The web's search field under the ordo's title, which opens the search page.
        item(key = "find") { SearchEntry(onFind, ordoColumn().padding(top = 8.dp)) }
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
                    Modifier.tap {
                        if (!turningAll) {
                            turningAll = true
                            scope.launch {
                                try {
                                    monthShade.animateTo(0f, tween(MONTH_OUT_MS, easing = FastOutLinearInEasing))
                                    allDetails = !allDetails
                                    monthShade.animateTo(1f, tween(MONTH_IN_MS, easing = LinearOutSlowInEasing))
                                } finally {
                                    turningAll = false
                                }
                            }
                        }
                    }.padding(vertical = 12.dp),
                    style = Type.small.copy(color = p.accent, textDecoration = TextDecoration.Underline),
                )
            }
        }
        item(key = "ranks") { RankKey(ordoColumn().padding(bottom = 5.6.dp)) }
        // The month's heading pins while its days scroll beneath it (the web's sticky `.month h2`),
        // so its ↑ back to the top stays in reach. It carries the wall's own copy, opaque, so the
        // rows passing under it never show through.
        stickyHeader(key = "month") {
            // Pinned at the list's top edge, which runs under the status bar, the heading keeps its
            // words below the bar: the wall's copy grows up behind it as it reaches the bar.
            Box(Modifier.fillMaxWidth().wallBehind().padding(top = with(LocalDensity.current) { underBar.toDp() }), contentAlignment = Alignment.TopCenter) {
                MonthHeading(
                    Month.of(monthNumber).getDisplayName(JavaTextStyle.FULL, Locale.US),
                    Modifier.onSizeChanged { headingPx = it.height }.then(ordoColumn()).padding(top = 10.dp),
                ) {
                    scope.launch { listState.animateScrollToItem(0) }
                }
            }
        }
        if (wide) item(key = "columns") { DayColumns(ordoColumn().graphicsLayer { alpha = monthShade.value }) }
        if (month == null) {
            item(key = "wait") { Text("Preparing the month…", ordoColumn().padding(vertical = 32.dp), style = Type.small.copy(color = p.muted, textAlign = TextAlign.Center)) }
        } else {
            itemsIndexed(month.days, key = { _, d -> "d-${d.date.day}" }) { _, d ->
                DayRow(d, LocalDate.of(d.date.year, d.date.month, d.date.day) == today, allDetails, turningAll, onDay, onFeast, ordoColumn().graphicsLayer { alpha = monthShade.value })
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
    course: List<CourseMonthView>?,
    today: LocalDate,
    laid: () -> Float,
    onMonth: (Int, Int) -> Unit,
    onToday: () -> Unit,
    onYear: (Int) -> Unit,
    modifier: Modifier,
) {
    val p = LocalPalette.current
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
            // Gilt points part the three, as they part home's hours.
            GiltPoint()
            Text("Today", cell.tap(action = "open today in the ordo", onClick = onToday).padding(12.dp).goldUnderline(true, p.goldLine, if (wide) 12.dp else 24.dp), style = nav.copy(textAlign = TextAlign.Center))
            GiltPoint()
            Text("${year + 1} ›", cell.tap(label = "Next year, ${year + 1}") { sameView(year + 1) }.padding(12.dp), style = nav.copy(textAlign = TextAlign.Center))
        }
    }
    // The month strip: the shown month underlined in gold along its cell's foot, and under each
    // month the year's course (`CourseBand`), with today's lozenge hung beneath at today's place.
    val strip: @Composable () -> Unit = {
        Month.entries.chunked(if (wide) 12 else 6).forEach { row ->
            Row(Modifier.fillMaxWidth()) {
                row.forEach { m ->
                    val current = m.value == month
                    val todays = today.year == year && today.monthValue == m.value
                    Column(Modifier.weight(1f)) {
                        Box(
                            Modifier.fillMaxWidth().heightIn(min = 44.dp)
                                .drawBehind {
                                    val w = 2.dp.toPx()
                                    if (current) drawLine(p.goldLine, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
                                }
                                .tap(label = m.getDisplayName(JavaTextStyle.FULL, Locale.US), selected = current) { onMonth(year, m.value) },
                            contentAlignment = Alignment.Center,
                        ) {
                            Text(
                                m.getDisplayName(JavaTextStyle.SHORT, Locale.US).uppercase(),
                                style = Type.label(12.48f, 0.06f).copy(color = if (current) p.accent else p.muted),
                            )
                        }
                        CourseBand(course?.getOrNull(m.value - 1), if (todays) today.dayOfMonth else null) { (laid() * 12f - (m.value - 1)).coerceIn(0f, 1f) }
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
            // The course's own hairline is the strip's foot now (the web's band in `.month-jump`).
            Column(Modifier.fillMaxWidth()) { strip() }
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

/**
 * One month's band of the year's course under the strip, as the web's `.month-course`: the rows'
 * colour rails laid on their side, a day to a 31st of the month's width (`courseSpan`), so a short
 * month ends early and the strip's hairline, the band's lowest line, shows in its tail; each run's
 * edges snapped to whole pixels, as the web's `crispEdges`. Today's lozenge hangs beneath the band
 * at today's place. Its 11dp are kept whether the course has come or not, so nothing moves when
 * it does; drawn for the eye alone. `laid` is how much of it the frontispiece's stroke has
 * uncovered (0 to 1), read as it draws.
 */
@Composable
private fun CourseBand(band: CourseMonthView?, today: Int?, laid: () -> Float) {
    val p = LocalPalette.current
    Canvas(Modifier.fillMaxWidth().height(11.dp)) {
        val top = 1.dp.toPx().roundToInt().toFloat()
        val height = 3.dp.toPx().roundToInt().toFloat()
        val hair = 1.dp.toPx().roundToInt().coerceAtLeast(1).toFloat()
        drawRect(p.border, Offset(0f, top + height - hair), Size(size.width, hair))
        if (band != null) {
            val span = courseSpan().toFloat()
            // The stroke uncovers the band from the left; nothing else moves.
            clipRect(right = size.width * laid()) {
                for (r in band.runs) {
                    val x0 = (size.width * r.start / span).roundToInt().toFloat()
                    val x1 = (size.width * (r.start + r.days) / span).roundToInt().toFloat()
                    if (x1 > x0) drawRect(railColor(r.color, p), Offset(x0, top), Size(x1 - x0, height))
                }
            }
        }
        if (today != null) {
            // The web's 0.22rem square turned on its point, centred 7.5dp under the cell.
            lozenge(Offset((size.width * coursePlace(today)).toFloat(), 7.5.dp.toPx()), 2.5.dp.toPx(), p.gold, null)
        }
    }
}

/** Whether this launch has laid the year's course in yet: once a launch. */
private object CourseLay {
    var done = false
}

/** The first day's item in a month's list: after the chrome, header, search, tools, rank key and month heading (and a wide screen's column headings). */
private const val FIRST_DAY_ITEM = 6

/**
 * A year's frontispiece, as the web's /calendar/{year}: the title page, the month strip with the
 * year's course under it (`course`, from the engine once it has it), and the Tabula Temporaria.
 * Each date leads to its day in the ordo.
 */
@Composable
fun OrdoYearScreen(
    view: OrdoYearView,
    course: List<CourseMonthView>?,
    today: LocalDate,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onMonth: (Int, Int) -> Unit,
    onToday: () -> Unit,
    onYear: (Int) -> Unit,
    onDay: (LocalDate) -> Unit,
    onFind: () -> Unit = {},
) {
    val p = LocalPalette.current
    val listState = rememberSaveable(view.year, saver = LazyListState.Saver) { LazyListState() }
    // The year's course is laid in by one stroke across the twelve months, the first time a
    // frontispiece shows it this launch (the web's once a session); with animations off the
    // stroke is over at once.
    val stroke = remember { Animatable(if (CourseLay.done) 1f else 0f) }
    LaunchedEffect(course != null) {
        if (course != null && !CourseLay.done) {
            CourseLay.done = true
            stroke.animateTo(1f, tween(Tempo.COURSE_LAY_MS, easing = Tempo.courseLayCurve))
        }
    }
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "chrome") { chrome() }
        item(key = "header") { OrdoHeader(view.year, null, view.roman, course, today, { stroke.value }, onMonth, onToday, onYear, ordoColumn().padding(top = 17.6.dp)) }
        item(key = "find") { SearchEntry(onFind, ordoColumn().padding(top = 8.dp)) }
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

/** A Kalendar's month: its rubricated heading, in the titulus, over the ornament's double rule and its lozenge (`.month h2`). */
@Composable
private fun MonthHeading(name: String, modifier: Modifier, onTop: () -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    Row(
        modifier.drawBehind {
            val w = 1.dp.toPx()
            drawLine(o.line, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
            drawLine(o.line, Offset(0f, size.height - w * 2.5f), Offset(size.width, size.height - w * 2.5f), w)
            // A 6dp square in the lining, ringed 2dp in the page's ground where it parts the rule.
            val c = Offset(size.width / 2f, size.height - w * 1.5f)
            lozenge(c, 7.07.dp.toPx(), p.bg, null)
            lozenge(c, 4.24.dp.toPx(), p.lining, null)
        },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // The name keeps the heading's padding; the ↑ takes a 48dp target, which stands into it.
        Text(name, Modifier.weight(1f).padding(vertical = 6.4.dp).semantics { heading() }, style = Type.body.copy(fontSize = 21.6.sp, lineHeight = 28.sp, color = p.titulus, fontFeatureSettings = "smcp", letterSpacing = 1.3.sp))
        Box(Modifier.sizeIn(minWidth = 48.dp, minHeight = 48.dp).tap(label = "Back to the top", onClick = onTop), contentAlignment = Alignment.Center) {
            Text("↑", Modifier.clearAndSetSemantics {}, style = Type.body.copy(fontSize = 16.sp, lineHeight = 20.sp, color = p.muted))
        }
    }
}

@Composable
private fun DayRow(d: OrdoDayView, isToday: Boolean, allDetails: Boolean, turningAll: Boolean, onDay: (LocalDate) -> Unit, onFeast: (String) -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    val open = rememberSaveable(d.date, saver = Disclosures) { mutableStateMapOf<String, Boolean>() }
    val details = open["details"] ?: allDetails
    val date = LocalDate.of(d.date.year, d.date.month, d.date.day)
    // One disclosure holds the day's commemorations and its office, as the web's
    // `.day-office-details`.
    val hasDetails = d.dayComms().isNotEmpty() || d.hasLauds() || d.hoursPreces || d.hasVespers()
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
        if (hasDetails) SmallDisclosure("Office details", details) { open["details"] = !details }
    }
    val unfolded: @Composable ColumnScope.() -> Unit = {
        // Shown for the whole month at once, the rows stay where they are, and change at once
        // while the month is faded.
        Unfold(details && hasDetails, reveal = open["details"] != null, instant = turningAll) { Digest(d, onFeast) }
    }
    val anyOpen = details && hasDetails
    if (LocalWide.current) return DayTableRow(d, date, isToday, spoken, onDay, modifier, anyOpen, buttons, unfolded)
    val body: @Composable ColumnScope.() -> Unit = {
        // Today is said once, in the gold of its number, at the right of today's second line, which
        // is otherwise empty: no row keeps a line for it.
        Row(Modifier.fillMaxWidth().padding(top = 1.dp), horizontalArrangement = Arrangement.spacedBy(9.6.dp), verticalAlignment = Alignment.CenterVertically) {
            buttons()
            if (isToday) {
                Spacer(Modifier.weight(1f))
                Text("Today", Modifier.padding(end = 8.dp).semantics { hideFromAccessibility() }, style = TodayNote)
            }
        }
        unfolded()
    }
    // Today is painted, not selected: home's invitation wash, thinner, ruled top and bottom in the
    // gold line inset 8dp from the gutters and thinning out toward them, the day's number in gold.
    Column(
        modifier.then(
            if (!isToday) Modifier else Modifier.drawBehind {
                val inset = 8.dp.toPx()
                val w = 1.dp.toPx()
                // Above the row's hairline.
                val h = size.height - w
                val wash = homeInk(p).todayWash
                drawRect(
                    androidx.compose.ui.graphics.Brush.horizontalGradient(
                        0f to wash.copy(alpha = 0f),
                        (48.dp.toPx() / size.width) to wash,
                        (1f - 48.dp.toPx() / size.width) to wash,
                        1f to wash.copy(alpha = 0f),
                        startX = inset,
                        endX = size.width - inset,
                    ),
                    Offset(inset, 0f),
                    Size(size.width - 2 * inset, h),
                )
                drawRect(p.goldLine, Offset(inset, 0f), Size(size.width - 2 * inset, w))
                drawRect(p.goldLine, Offset(inset, h - w), Size(size.width - 2 * inset, w))
            },
        ),
    ) {
        // A finger on the date or the name lights the whole row: home's pool of light, a long
        // lozenge under its words ([Lit]); the parts themselves neither dim nor light.
        val held = remember { MutableInteractionSource() }
        val reach = railReach(held)
        val inks = railInks(d, p)
        // A row as tall as its type (the web's `.day-feast` on a phone: 0.7rem above, 0.6rem below).
        Row(
            Modifier.fillMaxWidth().indication(held, Lit(homeInk(p).nowPool, PoolShape.Row.copy(at = 0.44f))).height(IntrinsicSize.Min)
                // The day's liturgical colour as a rail beside its date, bound to its octave's days.
                .drawBehind { dayRail(inks, reach.value, top = 12.8.dp.toPx(), bottom = (12.8 + 37.6).dp.toPx(), d) }
                .padding(top = 11.2.dp, bottom = 9.6.dp),
        ) {
            // The rail's room, as tall as it stands at rest.
            Spacer(Modifier.width(3.dp).height((1.6 + 37.6).dp))
            Column(Modifier.width(59.dp).tap(label = spoken, action = "open the day", source = held) { onDay(date) }, horizontalAlignment = Alignment.CenterHorizontally) {
                Text("${d.date.day}", style = Type.body.copy(fontSize = 21.6.sp, lineHeight = 23.76.sp, color = if (isToday) p.gold else p.accent, fontFeatureSettings = "lnum"))
                Text(d.weekday.uppercase(), style = Type.label(12f, 0.06f).copy(color = p.muted, lineHeight = 16.8.sp))
            }
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.Top) {
                    // The date's stop already says the feast; the feast is a second target for the eye only.
                    FeastName(d, Type.body.copy(fontSize = 16.sp, lineHeight = 21.6.sp), Modifier.weight(1f).tap(source = held) { onDay(date) }.semantics { hideFromAccessibility() })
                    // The phone's marks stay quiet, as the web's card sets them: the name carries the rank's ink.
                    // Each names itself in a note when tapped ([OrdoMarks]).
                    Row(Modifier.padding(start = 8.dp, top = 2.dp), horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                        OrdoMarks(d, fast = { Text("§", style = Type.small.copy(fontSize = 12.sp, color = p.muted)) }, rank = { Text(d.rank, style = Type.small.copy(fontSize = 12.sp, lineHeight = 16.8.sp, color = p.muted)) })
                    }
                }
                Monastic(d)
                body()
            }
        }
        RowHairline(d)
    }
}

/**
 * How far a day's rail has run toward the row's full height: 1 while the row is held, reached over
 * Tempo's reach, and 0 at rest, gone back to over its touch (the web's `.month-table
 * tr.day.is-pressed` rail).
 */
@Composable
private fun railReach(held: InteractionSource): State<Float> {
    val pressed by held.collectIsPressedAsState()
    return animateFloatAsState(if (pressed) 1f else 0f, if (pressed) Tempo.touchReach.spec() else Tempo.touchOut.spec(), label = "rail")
}

/**
 * A day's rail's inks ([dayRail]): the rail's colour, the inset's, and the far ends of a vigil's
 * thread, the feast's rail below the vigil's row and the vigil's above the feast's.
 */
private class RailInks(val rail: Color, val inset: Color?, val leadTo: Color?, val leadFrom: Color?)

private fun railInks(d: OrdoDayView, p: Palette): RailInks {
    fun ink(name: String) = name.takeIf { it.isNotEmpty() }?.let { railColor(it, p) }
    return RailInks(railColor(d.railColor, p), ink(d.railInset), ink(d.railLeadTo), ink(d.railLeadFrom))
}

/** A vigil's thread is set at 70%, its colour halfway at the rule (the web's `.day-thread`). */
private const val THREAD_ALPHA = 0.7f

/** Halfway from the vigil's colour to the feast's, in Oklab (the web's `color-mix(in oklab, …)`). */
private fun threadMid(vigil: Color, feast: Color): Color = lerp(vigil, feast, 0.5f)

/**
 * A day's rail at the row's leading edge: from `top` to `bottom` at rest, the row's full height at
 * `reach` 1. Days bound together (`presentation::rails`, the web's `.bound-above`, `.bound-below`):
 * within an octave the rail runs up to the row's top and down through the hairline below into the
 * next day's, so the octave's days read as one long bar in the octave's colour; a feast that
 * impedes the octave, or a day of it in another colour, stands beside it as a lone bar of its own
 * colour, `inset` (the web's `.day-inset`).
 *
 * A vigil said as the day's office leads into its feast the next day (needs ruling #838, question
 * 3(d); the web's `.day-thread`): a thread a third of the rail's width, on its centre, runs from
 * the vigil's lone foot to the row's foot, across the hairline ([RowHairline]), and from the
 * feast's row's top to its lone top, its colour passing from the vigil's into the feast's. It lies
 * under the rail, which covers it while the row is held, and each end runs 1dp under its rail, so
 * no rounding of the two edges leaves a line of ground between them.
 */
private fun DrawScope.dayRail(inks: RailInks, reach: Float, top: Float, bottom: Float, d: OrdoDayView) {
    val w = 1.dp.toPx()
    inks.leadTo?.let {
        val brush = Brush.verticalGradient(listOf(inks.rail, threadMid(inks.rail, it)), startY = bottom - w, endY = size.height)
        drawRect(brush, Offset(w, bottom - w), Size(w, size.height - bottom + w), alpha = THREAD_ALPHA)
    }
    inks.leadFrom?.let {
        val brush = Brush.verticalGradient(listOf(threadMid(it, inks.rail), inks.rail), startY = 0f, endY = top + w)
        drawRect(brush, Offset(w, 0f), Size(w, top + w), alpha = THREAD_ALPHA)
    }
    val restTop = if (d.boundAbove) 0f else top
    // Through the row's hairline, which is drawn below the row.
    val end = if (d.boundBelow) size.height + 1.dp.toPx() else size.height
    val restBottom = if (d.boundBelow) end else bottom
    val from = restTop * (1f - reach)
    val to = restBottom + (end - restBottom) * reach
    drawRect(inks.rail, Offset(0f, from), Size(3.dp.toPx(), to - from))
    if (inks.inset != null) drawRect(inks.inset, Offset(6.dp.toPx(), top), Size(3.dp.toPx(), bottom - top))
}

/**
 * The ordo's bracketed line for an observance kept by monastics and oblates only (the web's
 * `.day-monastic`): quiet like a commemoration, never read as the day.
 */
@Composable
private fun Monastic(d: OrdoDayView) {
    val style = Type.small.copy(fontSize = 13.6.sp, lineHeight = 20.4.sp, color = LocalPalette.current.muted, fontStyle = FontStyle.Italic)
    d.monastic.forEach { m ->
        Text("[${m.heading}] ${m.rank}", Modifier.padding(top = 2.4.dp), style = style)
        if (m.office.isNotEmpty()) Text(m.office, style = style)
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
    val inks = railInks(d, p)
    val margin = LocalConfiguration.current.screenWidthDp >= 1000
    // Today is painted, not selected: home's invitation wash, thinner, ruled top and bottom in the
    // gold line across the row, the day's number in gold.
    Column(
        modifier.then(
            if (!isToday) Modifier else Modifier.drawBehind {
                val w = 1.dp.toPx()
                val h = size.height - w
                drawRect(homeInk(p).todayWash, size = Size(size.width, h))
                drawRect(p.goldLine, size = Size(size.width, w))
                drawRect(p.goldLine, Offset(0f, h - w), Size(size.width, w))
            },
        ),
    ) {
        // A finger on the date or the name lights the whole row ([Lit]), as on a phone.
        val held = remember { MutableInteractionSource() }
        val reach = railReach(held)
        // The web's `.month-table td`: 0.5rem above and below, from 701px.
        Row(
            Modifier.fillMaxWidth().indication(held, Lit(homeInk(p).nowPool, PoolShape.Row)).height(IntrinsicSize.Min)
                // The day's liturgical colour as a rail under the date's column, bound to its octave's days.
                .drawBehind { dayRail(inks, reach.value, top = (8 + 1.6).dp.toPx(), bottom = size.height - 8.dp.toPx(), d) }
                .padding(vertical = 8.dp),
        ) {
            // From 1000dp Today is a note in the margin beside today's band (the web's), taking no
            // room in the row; narrower, the table has no margin and it stands under the number.
            if (isToday && margin) {
                Text(
                    "Today",
                    Modifier.semantics { hideFromAccessibility() }.layout { m, c ->
                        val note = m.measure(c.copy(minWidth = 0, maxWidth = Constraints.Infinity, minHeight = 0))
                        layout(0, 0) { note.place(-note.width - 12.8.dp.roundToPx(), 1.6.dp.roundToPx()) }
                    },
                    style = TodayNote.copy(fontSize = 13.6.sp),
                )
            }
            Column(
                Modifier.width(DayCol).fillMaxHeight()
                    .tap(label = spoken, action = "open the day", source = held) { onDay(date) }
                    .padding(start = 10.4.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text("${d.date.day}", style = Type.body.copy(fontSize = 19.2.sp, lineHeight = 23.sp, color = if (isToday) p.gold else p.accent, fontFeatureSettings = "lnum"))
                if (isToday && !margin) Text("Today", style = Type.small.copy(fontSize = 11.2.sp, lineHeight = 15.7.sp, color = p.accent))
            }
            Text(d.weekday, Modifier.width(WeekCol).padding(horizontal = 7.2.dp, vertical = 3.dp).clearAndSetSemantics {}, style = Type.small.copy(fontSize = 12.8.sp, color = p.muted))
            Column(Modifier.weight(1f).padding(horizontal = 7.2.dp)) {
                // The date's stop already says the feast; the feast is a second target for the eye only.
                val name = Type.body.copy(fontSize = 16.sp, lineHeight = 22.4.sp)
                // A bracketed line sits under the name, so the disclosures drop below it as on the web.
                if (anyOpen || d.monastic.isNotEmpty()) {
                    FeastName(d, name, Modifier.tap(source = held) { onDay(date) }.semantics { hideFromAccessibility() })
                    Monastic(d)
                    Row(horizontalArrangement = Arrangement.spacedBy(9.6.dp)) { buttons() }
                } else {
                    Row {
                        FeastName(d, name, Modifier.weight(1f, fill = false).alignByBaseline().tap(source = held) { onDay(date) }.semantics { hideFromAccessibility() })
                        Row(Modifier.padding(start = 20.dp).alignByBaseline(), horizontalArrangement = Arrangement.spacedBy(9.6.dp)) { buttons() }
                    }
                }
                unfolded()
            }
            // Fasting and abstinence stay quiet, so red in a row means rank. Each mark names itself
            // in a note when tapped ([OrdoMarks]).
            Box(Modifier.width(FlagCol), contentAlignment = Alignment.TopCenter) {
                OrdoMarks(d, fast = { Text("§", style = Type.body.copy(fontSize = 16.sp, color = p.muted, fontWeight = FontWeight.Bold)) }, abstinence = false, rank = null)
            }
            Box(Modifier.width(FlagCol).padding(top = 6.dp), contentAlignment = Alignment.TopCenter) {
                OrdoMarks(d, fast = null, rank = null)
            }
            // A Kalendar has no hyperlinks: the rank keeps its ink and loses the rule.
            Box(Modifier.width(RankCol).padding(end = 7.2.dp, top = 3.dp), contentAlignment = Alignment.TopEnd) {
                OrdoMarks(d, fast = null, abstinence = false, rank = { Text(d.rank, style = Type.small.copy(fontSize = 12.48.sp, lineHeight = 17.5.sp, color = rankInk(d.rank, p))) })
            }
        }
        RowHairline(d)
    }
}

/**
 * The rule under a day, leaving the rail's width where the rail runs on through it into the next
 * day, and carrying a vigil's thread across it, at its colour halfway into the feast's ([dayRail]).
 */
@Composable
private fun RowHairline(d: OrdoDayView) {
    val p = LocalPalette.current
    val bound = if (d.boundBelow) Modifier.padding(start = 3.dp) else Modifier
    val thread = d.railLeadTo.takeIf { it.isNotEmpty() }?.let { threadMid(railColor(d.railColor, p), railColor(it, p)) }
    Hairline(
        p.border,
        if (thread == null) bound else bound.drawWithContent {
            drawContent()
            drawRect(thread, Offset(1.dp.toPx(), 0f), Size(1.dp.toPx(), size.height), alpha = THREAD_ALPHA)
        },
    )
}

/** What the § and the fish say in their notes, as the web's titles word them (`presentation::FAST_DAY`). */
private val MARK_WORDS by lazy { org.orthodoxwest.office.core.ordoMarkWords() }

/**
 * A day's marks, each naming itself in the note a glossed word opens when tapped (the web's
 * `.month-table abbr`): the fast's § as `fast` draws it, the fish (unless `abstinence` is false),
 * and the rank's letters as `rank` draws them, its name the day's `rankFull`. A null draws none.
 * The date's stop already says them all to a screen reader; each is a stop of its own as well,
 * which opens its note.
 */
@Composable
private fun OrdoMarks(d: OrdoDayView, fast: (@Composable () -> Unit)?, abstinence: Boolean = true, rank: (@Composable () -> Unit)?) {
    val p = LocalPalette.current
    if (d.fast && fast != null) NoteMark(MARK_WORDS.fasting, "Fasting") { fast() }
    if (d.abstinence && abstinence) NoteMark(MARK_WORDS.abstinence, "Abstinence") { FishIcon(p.muted) }
    if (d.rank.isNotEmpty() && rank != null) {
        if (d.rankFull.isNotEmpty()) NoteMark(d.rankFull, "Rank ${d.rank}") { rank() } else rank()
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

/**
 * The day's commemorations not already named with an antiphon at its Lauds or Vespers (the web's
 * `build_month`), each with its feast's page.
 */
private fun OrdoDayView.dayComms(): List<CommemorationView> {
    val named = (laudsComms + vespersComms).map { it.name }.toSet()
    return commemorations.mapIndexed { i, name -> CommemorationView(name, "", commemorationIds.getOrElse(i) { "" }) }.filter { it.name !in named }
}

private fun OrdoDayView.hasLauds() = benedictusAntiphon.isNotEmpty() || laudsPreces || laudsSuffrage || laudsComms.isNotEmpty()
private fun OrdoDayView.hasVespers() = magnificatAntiphon.isNotEmpty() || vespersPreces || vespersSuffrage || vespersComms.isNotEmpty() || vespersNote.isNotEmpty()

@Composable
private fun SmallDisclosure(label: String, open: Boolean, onToggle: () -> Unit) {
    // As tall as its type: no row keeps a 44dp line for it. Compose still answers a touch over the
    // platform's whole minimum target round it, as the web draws its thumb box.
    Row(Modifier.tap(onClick = onToggle).disclosed(open), verticalAlignment = Alignment.CenterVertically) {
        Text(label, style = Type.small.copy(fontSize = 12.sp, lineHeight = 18.sp, color = LocalPalette.current.muted))
        Caret(open, LocalPalette.current.muted)
    }
}

/**
 * "Today", in small caps and the gold of today's number, darkened by day (the web's
 * `--today-ink`): the gold reads 3.9:1 on limewash, too light for type this small.
 */
private val TodayNote: TextStyle
    @Composable get() = Type.label(13.12f, 0.1f).copy(color = LocalPalette.current.let { if (it.dark) it.gold else Color(0xFF76561A) }, fontFeatureSettings = ALL_SMALL_CAPS)

/** The office digest: each hour's gospel antiphon, preces, suffrage, and commemorations. */
@Composable
private fun Digest(d: OrdoDayView, onFeast: (String) -> Unit) {
    val p = LocalPalette.current
    Column(
        Modifier.fillMaxWidth().padding(bottom = 8.dp).drawBehind { drawLine(p.goldLine, Offset(0f, 0f), Offset(0f, size.height), 1.dp.toPx()) }.padding(8.dp),
    ) {
        // The day's commemorations first, under "Day", as the hours' rows are set.
        val day = d.dayComms()
        if (day.isNotEmpty()) DigestHour("Day", emptyList(), null, preces = false, suffrage = false, comms = day, onFeast = onFeast)
        if (d.hasLauds()) {
            DigestHour("Lauds", emptyList(), d.benedictusAntiphon.takeIf { it.isNotEmpty() }?.let { "Ben. “$it”" }, d.laudsPreces, d.laudsSuffrage, d.laudsComms, onFeast)
        }
        if (d.hoursPreces) DigestHour("Hours", emptyList(), null, preces = true, suffrage = false, comms = emptyList(), onFeast = onFeast)
        if (d.hasVespers()) {
            DigestHour("Vespers", listOfNotNull(d.vespersNote.takeIf { it.isNotEmpty() }), d.magnificatAntiphon.takeIf { it.isNotEmpty() }?.let { "Mag. “$it”" }, d.vespersPreces, d.vespersSuffrage, d.vespersComms, onFeast)
        }
    }
}

@Composable
private fun DigestHour(name: String, notes: List<String>, antiphon: String?, preces: Boolean, suffrage: Boolean, comms: List<CommemorationView>, onFeast: (String) -> Unit) {
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
        // A commemoration's name leads to its feast's page, underlined as the web's `.feast-link`.
        comms.forEach { c ->
            Text(
                buildAnnotatedString {
                    append("Com. ")
                    withStyle(SpanStyle(textDecoration = if (c.feastId.isEmpty()) null else TextDecoration.Underline)) { append(c.name) }
                    if (c.incipit.isNotEmpty()) append(" “${c.incipit}”")
                },
                if (c.feastId.isEmpty()) Modifier else Modifier.tap(action = "open the feast's page") { onFeast(c.feastId) },
                style = line,
            )
        }
    }
}

/** A day's disclosures the reader has opened or closed, as saved state can hold them: "details=1". */
private val Disclosures = listSaver<SnapshotStateMap<String, Boolean>, String>(
    save = { m -> m.map { (k, v) -> "$k=${if (v) 1 else 0}" } },
    restore = { l -> mutableStateMapOf<String, Boolean>().apply { l.forEach { put(it.substringBefore('='), it.endsWith("=1")) } } },
)
