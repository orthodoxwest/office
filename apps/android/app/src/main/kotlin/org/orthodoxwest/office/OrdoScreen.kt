package org.orthodoxwest.office

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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.LocalDate
import java.time.Month
import java.time.format.TextStyle as JavaTextStyle
import java.util.Locale
import kotlinx.coroutines.launch
import org.orthodoxwest.office.core.CommemorationView
import org.orthodoxwest.office.core.OrdoDayView
import org.orthodoxwest.office.core.OrdoMonthView

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
    onDay: (LocalDate) -> Unit,
) {
    val p = LocalPalette.current
    var allDetails by remember(year, monthNumber) { mutableStateOf(false) }
    val listState = remember(year, monthNumber) { LazyListState() }
    val scope = rememberCoroutineScope()
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "chrome") { chrome() }
        item(key = "header") { OrdoHeader(year, monthNumber, today, onMonth, Modifier.measure().padding(top = 17.6.dp)) }
        item(key = "tools") {
            Row(Modifier.measure().padding(top = 8.dp).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
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
        item(key = "month") {
            MonthHeading(Month.of(monthNumber).getDisplayName(JavaTextStyle.FULL, Locale.US), today.year == year && today.monthValue == monthNumber, Modifier.measure().padding(top = 10.dp)) {
                scope.launch { listState.animateScrollToItem(0) }
            }
        }
        if (month == null) {
            item(key = "wait") { Text("Preparing the month…", Modifier.measure().padding(vertical = 32.dp), style = Type.small.copy(color = p.muted, textAlign = TextAlign.Center)) }
        } else {
            itemsIndexed(month.days, key = { _, d -> "d-${d.date.day}" }) { _, d ->
                DayRow(d, LocalDate.of(d.date.year, d.date.month, d.date.day) == today, allDetails, onDay, Modifier.measure())
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
                onMiddle = { scope.launch { listState.animateScrollToItem(0) } },
                nextLabel = "Next month",
                next = Month.of(next.second).getDisplayName(JavaTextStyle.FULL, Locale.US),
                onNext = { onMonth(next.first, next.second) },
                modifier = Modifier.measure().padding(top = 44.dp),
            )
        }
        item(key = "footer") { Footer() }
    }
}

@Composable
private fun OrdoHeader(year: Int, month: Int, today: LocalDate, onMonth: (Int, Int) -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    Column(
        modifier.drawBehind {
            // The frontispiece's double rule above, a hairline below.
            val w = 1.dp.toPx()
            drawLine(p.goldLine, Offset(0f, w / 2f), Offset(size.width, w / 2f), w)
            drawLine(p.goldLine, Offset(0f, w * 2.5f), Offset(size.width, w * 2.5f), w)
            drawLine(p.border, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
        }.padding(top = 13.4.dp, bottom = 16.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Headpiece()
        Text("$year Ordo", Modifier.padding(top = 2.dp), style = Type.body.copy(fontSize = 28.sp, lineHeight = 34.sp, color = p.text, fontFeatureSettings = "lnum"))
        Text("Feasts & daily observances", Modifier.padding(top = 4.8.dp), style = Type.small.copy(color = p.muted))
        Row(Modifier.fillMaxWidth().padding(top = 8.dp).height(44.dp), verticalAlignment = Alignment.CenterVertically) {
            val nav = Type.body.copy(fontSize = 12.8.sp, lineHeight = 20.5.sp, color = p.accent, fontFeatureSettings = "lnum")
            Text("‹ ${year - 1}", Modifier.weight(1f).tap { onMonth(year - 1, month) }.padding(12.dp), style = nav.copy(textAlign = TextAlign.Center))
            Divider(p.border)
            Text("Today", Modifier.weight(1f).tap { onMonth(today.year, today.monthValue) }.padding(12.dp).goldUnderline(today.year == year, p.goldLine, 24.dp), style = nav.copy(textAlign = TextAlign.Center))
            Divider(p.border)
            Text("${year + 1} ›", Modifier.weight(1f).tap { onMonth(year + 1, month) }.padding(12.dp), style = nav.copy(textAlign = TextAlign.Center))
        }
        Hairline(p.border, Modifier.padding(top = 8.dp))
        // The month strip: six to a row, the shown month underlined with its lozenge.
        Month.entries.chunked(6).forEach { row ->
            Row(Modifier.fillMaxWidth()) {
                row.forEach { m ->
                    val current = m.value == month
                    Box(Modifier.weight(1f).height(44.dp).tap { onMonth(year, m.value) }, contentAlignment = Alignment.Center) {
                        Text(
                            m.getDisplayName(JavaTextStyle.SHORT, Locale.US).uppercase(),
                            Modifier.drawBehind {
                                if (current) {
                                    val y = size.height + 4.dp.toPx()
                                    drawLine(p.goldLine, Offset(-14.dp.toPx(), y), Offset(size.width + 14.dp.toPx(), y), 2.dp.toPx())
                                    lozenge(Offset(size.width / 2f, y), 3.dp.toPx(), o.flat, null)
                                }
                            },
                            style = Type.label(12.48f, 0.06f).copy(color = if (current) p.text else p.muted),
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun MonthHeading(name: String, isTodaysMonth: Boolean, modifier: Modifier, onTop: () -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    Row(
        modifier.drawBehind {
            val w = 1.dp.toPx()
            drawLine(o.line, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
            drawLine(o.line, Offset(0f, size.height - w * 2.5f), Offset(size.width, size.height - w * 2.5f), w)
            if (isTodaysMonth) lozenge(Offset(size.width / 2f, size.height - w * 1.5f), 4.dp.toPx(), o.flat, null)
        }.padding(vertical = 6.4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(name, Modifier.weight(1f), style = Type.body.copy(fontSize = 21.6.sp, lineHeight = 28.sp, color = p.accent, fontFeatureSettings = "smcp", letterSpacing = 1.3.sp))
        Text("↑", Modifier.tap(onTop).padding(horizontal = 14.dp, vertical = 6.dp), style = Type.body.copy(fontSize = 16.sp, lineHeight = 20.sp, color = p.muted))
    }
}

@Composable
private fun DayRow(d: OrdoDayView, isToday: Boolean, allDetails: Boolean, onDay: (LocalDate) -> Unit, modifier: Modifier) {
    val p = LocalPalette.current
    val open = remember(d.date) { mutableStateMapOf<String, Boolean>() }
    val details = open["details"] ?: allDetails
    val date = LocalDate.of(d.date.year, d.date.month, d.date.day)
    val hasDetails = d.hasLauds() || d.hoursPreces || d.hasVespers()
    Column(modifier.then(if (isToday) Modifier.background(p.pressedWash) else Modifier)) {
        Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min).padding(top = 12.8.dp, bottom = 6.4.dp)) {
            // The day's liturgical colour as a rail beside its date.
            Box(Modifier.width(3.dp).height(40.dp).background(dayColor(d.color)))
            Column(Modifier.width(59.dp).tap { onDay(date) }, horizontalAlignment = Alignment.CenterHorizontally) {
                Text("${d.date.day}", style = Type.body.copy(fontSize = 21.6.sp, lineHeight = 23.76.sp, color = p.accent, fontFeatureSettings = "lnum"))
                Text(d.weekday.uppercase(), style = Type.label(12f, 0.06f).copy(color = p.muted, lineHeight = 16.8.sp))
                if (isToday) Text("Today", style = Type.small.copy(fontSize = 11.sp, lineHeight = 14.sp, color = p.accent))
            }
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.Top) {
                    Text(d.feast, Modifier.weight(1f).tap { onDay(date) }, style = Type.body.copy(fontSize = 16.sp, lineHeight = 21.6.sp, color = p.text))
                    Row(Modifier.padding(start = 8.dp, top = 2.dp), horizontalArrangement = Arrangement.spacedBy(6.dp), verticalAlignment = Alignment.CenterVertically) {
                        if (d.fast) Text("§", style = Type.small.copy(fontSize = 12.sp, color = p.muted))
                        if (d.abstinence) FishIcon(p.muted)
                        if (d.rank.isNotEmpty()) Text(d.rank, Modifier.goldUnderline(true, p.goldLine), style = Type.small.copy(fontSize = 12.sp, lineHeight = 16.8.sp, color = p.muted))
                    }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(9.6.dp)) {
                    if (d.commemorations.isNotEmpty()) {
                        val n = d.commemorations.size
                        SmallDisclosure("$n commemoration${if (n > 1) "s" else ""}", open["comms"] == true) { open["comms"] = open["comms"] != true }
                    }
                    if (hasDetails) SmallDisclosure("Office details", details) { open["details"] = !details }
                }
                if (open["comms"] == true) d.commemorations.forEach { Text(it, style = Type.small.copy(fontSize = 13.6.sp, lineHeight = 20.4.sp, color = p.muted, fontStyle = FontStyle.Italic)) }
                if (details && hasDetails) Digest(d)
            }
        }
        Hairline(p.border)
    }
}

private fun OrdoDayView.hasLauds() = benedictusAntiphon.isNotEmpty() || laudsPreces || laudsSuffrage || laudsComms.isNotEmpty()
private fun OrdoDayView.hasVespers() = magnificatAntiphon.isNotEmpty() || vespersPreces || vespersSuffrage || vespersComms.isNotEmpty() || vespersNote.isNotEmpty()

@Composable
private fun SmallDisclosure(label: String, open: Boolean, onToggle: () -> Unit) {
    Row(Modifier.heightIn(min = 44.dp).tap(onToggle), verticalAlignment = Alignment.CenterVertically) {
        Text(label, style = Type.small.copy(fontSize = 12.sp, lineHeight = 16.8.sp, color = LocalPalette.current.muted))
        Caret(open)
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
