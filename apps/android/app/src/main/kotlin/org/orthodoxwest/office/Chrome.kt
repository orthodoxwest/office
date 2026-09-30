package org.orthodoxwest.office

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.Month
import java.time.YearMonth
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle as JavaTextStyle
import java.util.Locale

/** The reading measure's side gutter (`--page-gutter` on a phone). */
val Gutter: Dp = 16.dp

/** Wide screens keep the web's prayer measure (`.elements`, 38rem). */
val Measure: Dp = 608.dp

fun Modifier.measure(): Modifier = this.widthIn(max = Measure).fillMaxWidth().padding(horizontal = Gutter)

/** A tap target with no ripple: the web's controls mark state, not touches. */
fun Modifier.tap(role: Role = Role.Button, label: String? = null, selected: Boolean? = null, action: String? = null, onClick: () -> Unit): Modifier =
    this.semantics {
        // What a screen reader says for a control drawn as a glyph ("‹", "↑"), in place of the glyph.
        if (label != null) contentDescription = label
        if (selected != null) this.selected = selected
    }.clickable(interactionSource = null, indication = null, role = role, onClickLabel = action, onClick = onClick)

/** A checkbox row: the box and its words are one control, announced checked or not. */
fun Modifier.check(checked: Boolean, onChange: (Boolean) -> Unit): Modifier =
    this.toggleable(checked, interactionSource = null, indication = null, role = Role.Checkbox, onValueChange = onChange)

/** "Expanded" or "Collapsed", after a disclosure's name. */
fun Modifier.disclosed(open: Boolean): Modifier = this.semantics { stateDescription = if (open) "Expanded" else "Collapsed" }

/** The disclosure caret, gold as the web's `▾`/`▴`. */
@Composable
fun Caret(open: Boolean) {
    // Drawn, not read: the disclosure says expanded or collapsed.
    Text(if (open) " ▴" else " ▾", Modifier.clearAndSetSemantics {}, style = TextStyle(fontSize = 9.sp, color = LocalPalette.current.goldLine))
}

/** A current control's gold underline. */
fun Modifier.goldUnderline(on: Boolean, color: Color, inset: Dp = 0.dp): Modifier = if (!on) this else this.drawBehind {
    val y = size.height - 1.dp.toPx()
    drawLine(color, Offset(inset.toPx(), y), Offset(size.width - inset.toPx(), y), 1.dp.toPx())
}

/** The web's breakpoint (style.css `min-width: 701px`): the desktop composition from here up. */
val WideFrom: Dp = 701.dp

/** Whether the page is laid out at the web's desktop widths: a tablet, or a phone on its side. */
val LocalWide = staticCompositionLocalOf { false }

/** The reader's theme and text size, for the wide footer's controls (the phone's are in the menu). */
class Prefs(val theme: ThemeChoice, val onTheme: (ThemeChoice) -> Unit, val textSize: TextSize, val onTextSize: (TextSize) -> Unit)

val LocalPrefs = staticCompositionLocalOf<Prefs?> { null }

/**
 * Where the site's navigation leads, and which of it is the page shown: the day's hours on an
 * hour page, then the Ordo and Reminders. The menu sets it out on a phone, the header inline
 * on a wide screen.
 */
class SiteNav(
    val hours: List<String>,
    val currentHour: String?,
    val onHour: ((String) -> Unit)?,
    val onOrdo: () -> Unit,
    val ordoCurrent: Boolean,
    val onReminders: () -> Unit,
    val remindersCurrent: Boolean,
)

/** The header beam: "✠ Daily Office" home, and the menu, or on a wide screen the links themselves. */
@Composable
fun SiteHeader(onHome: () -> Unit, menuOpen: Boolean, onMenu: () -> Unit, nav: SiteNav? = null) {
    val p = LocalPalette.current
    val wide = LocalWide.current && nav != null
    Column {
        Row(
            // The web's nav shell: held to 68rem, so the whole list fits on one line.
            Modifier.fillMaxWidth().wrapContentWidth().widthIn(max = if (wide) 1088.dp else Dp.Infinity).fillMaxWidth()
                .padding(start = Gutter, end = Gutter, top = 6.4.dp, bottom = 5.6.dp).heightIn(min = 44.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                buildAnnotatedString {
                    withStyle(SpanStyle(color = LocalOrnament.current.flat, fontFamily = CrossFont, fontSize = 11.sp)) { append("✠") }
                    append(" DAILY OFFICE")
                },
                Modifier.tap(label = "Daily Office, home", onClick = onHome).padding(vertical = 10.dp),
                style = Type.brand.copy(color = p.text),
            )
            Spacer(Modifier.weight(1f))
            if (wide) {
                InlineNav(nav)
            } else {
                Row(Modifier.tap(label = "Menu", onClick = onMenu).disclosed(menuOpen).padding(start = 12.8.dp, end = 3.2.dp).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("MENU", style = Type.menu.copy(color = p.accent))
                    Caret(menuOpen)
                }
            }
        }
        Hairline(p.oakLine)
    }
}

/** The desktop header's links (`.site-menu nav`): the current one in ink over an accent rule; Reminders quieter. */
@Composable
private fun InlineNav(nav: SiteNav) {
    val p = LocalPalette.current
    Row(verticalAlignment = Alignment.CenterVertically) {
        @Composable
        fun link(label: String, current: Boolean, secondary: Boolean = false, onClick: () -> Unit) {
            val style = if (secondary) Type.label(11.52f, 0.04f) else Type.label(12.48f, 0.06f)
            val ink = if (current) p.text else if (secondary) p.muted else p.accent
            Box(Modifier.heightIn(min = 44.dp).tap(selected = current, onClick = onClick).padding(horizontal = 4.8.dp), contentAlignment = Alignment.Center) {
                Text(
                    label.uppercase(),
                    Modifier.drawBehind {
                        if (current) drawLine(if (secondary) p.muted else p.accent, Offset(0f, size.height + 4.dp.toPx()), Offset(size.width, size.height + 4.dp.toPx()), 1.dp.toPx())
                    },
                    style = style.copy(color = ink),
                )
            }
        }
        nav.onHour?.let { onHour ->
            nav.hours.forEach { h -> link(hourLabel(h), h == nav.currentHour) { onHour(h) } }
            Box(Modifier.padding(horizontal = 2.4.dp).width(1.dp).height(24.dp).background(p.border))
        }
        link("Ordo", nav.ordoCurrent, onClick = nav.onOrdo)
        link("Reminders", nav.remindersCurrent, secondary = true, onClick = nav.onReminders)
    }
}

/** A row of equal cells in the menu grid, 44dp tall. */
@Composable
private fun MenuRow(content: @Composable RowScope.() -> Unit) {
    Row(Modifier.fillMaxWidth().heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically, content = content)
}

@Composable
private fun RowScope.MenuCell(label: String, current: Boolean, style: TextStyle, color: Color, onClick: () -> Unit) {
    val p = LocalPalette.current
    Box(
        Modifier.weight(1f).heightIn(min = 44.dp).tap(selected = current, onClick = onClick).goldUnderline(current, p.goldLine, inset = 0.dp),
        contentAlignment = Alignment.Center,
    ) { Text(label, style = style.copy(color = if (current) p.text else color, textAlign = TextAlign.Center)) }
}

/**
 * The site menu's dropdown panel: on an hour, the day's hours (2/3/2 as on home); the Ordo;
 * then the Theme and Text rows, the current choice underlined in gold.
 */
@Composable
fun MenuPanel(
    currentHour: String?,
    onHour: ((String) -> Unit)?,
    onOrdo: () -> Unit,
    onOrdoCurrent: Boolean,
    onReminders: () -> Unit,
    onRemindersCurrent: Boolean,
    theme: ThemeChoice,
    onTheme: (ThemeChoice) -> Unit,
    textSize: TextSize,
    onTextSize: (TextSize) -> Unit,
    onDismiss: () -> Unit,
    topOffset: Dp,
) {
    val p = LocalPalette.current
    val nav = Type.label(13.12f, 0.06f)
    Popup(
        alignment = Alignment.TopEnd,
        offset = with(LocalDensity.current) { IntOffset(-Gutter.roundToPx(), topOffset.roundToPx()) },
        onDismissRequest = onDismiss,
        properties = PopupProperties(focusable = true),
    ) {
        Column(
                Modifier
                    .width(336.dp)
                    .background(p.surface)
                    .border(1.dp, p.border)
                    .drawBehind { drawRect(p.goldLine, size = size.copy(height = 2.dp.toPx())) }
                    .padding(10.4.dp),
            ) {
                if (onHour != null) {
                    listOf(listOf("lauds", "prime"), listOf("terce", "sext", "none"), listOf("vespers", "compline")).forEach { row ->
                        MenuRow { row.forEach { h -> MenuCell(hourLabel(h).uppercase(), h == currentHour, nav, p.accent) { onHour(h) } } }
                        Spacer(Modifier.height(2.4.dp))
                    }
                    Hairline(p.border, Modifier.padding(vertical = 4.dp))
                }
                MenuRow {
                    MenuCell("ORDO", onOrdoCurrent, nav, p.accent, onOrdo)
                    // Habit setup, not an hour: quieter than the Ordo, as on the web.
                    MenuCell("REMINDERS", onRemindersCurrent, Type.label(12f, 0.06f), p.muted, onReminders)
                }
                Hairline(p.border, Modifier.padding(top = 6.4.dp))
                Spacer(Modifier.height(6.4.dp))
                MenuRow {
                    Text("THEME", Modifier.width(54.dp).padding(start = 10.4.dp), style = Type.label(10.56f, 0.08f).copy(color = p.muted))
                    ThemeChoice.entries.forEach { t -> MenuCell(t.label.uppercase(), t == theme, Type.label(12f, 0.06f), p.accent) { onTheme(t) } }
                }
                MenuRow {
                    Text("TEXT", Modifier.width(54.dp).padding(start = 10.4.dp), style = Type.label(10.56f, 0.08f).copy(color = p.muted))
                    TextSize.entries.forEach { s ->
                        val size = when (s) { TextSize.SMALL -> 13f; TextSize.DEFAULT -> 16f; TextSize.LARGE -> 20f }
                        MenuCell("A", s == textSize, Type.label(size, 0f), p.muted) { onTextSize(s) }
                    }
                }
            }
    }
}

/** A small uppercase disclosure label with its caret ("Change date ▾"). */
@Composable
fun Disclosure(label: String, open: Boolean, onToggle: () -> Unit, value: String? = null, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    Row(modifier.heightIn(min = 44.dp).tap(onClick = onToggle).disclosed(open).padding(horizontal = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(
            buildAnnotatedString {
                append(label.uppercase())
                if (value != null) withStyle(SpanStyle(color = p.text)) { append(" " + value.uppercase()) }
            },
            style = Type.control.copy(color = p.muted),
        )
        Caret(open)
    }
}

/** The picker's span of years, as the web's (app.js PICKER_FIRST_YEAR, PICKER_LAST_YEAR). */
private val PICKER_YEARS = 1950..2150

/**
 * The hand-set date picker (app.js): Previous · Today · Next, then a month grid whose days
 * are the choices. Sundays are red; the chosen day takes the gold underline, as a chosen
 * prayer form does, and today a quiet wash. The title turns the grid into the year's months,
 * and back; both views keep six weeks' height, so turning between them moves nothing below.
 */
@Composable
fun DatePicker(shown: LocalDate, today: LocalDate, onPick: (LocalDate) -> Unit) {
    val p = LocalPalette.current
    var month by remember(shown) { mutableStateOf(YearMonth.from(shown).coerceIn(PICKER_YEARS)) }
    var months by remember(shown) { mutableStateOf(false) }
    val link = Type.body.copy(fontSize = 16.sp, lineHeight = 24.sp, color = p.accent)
    Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
        Hairline(p.border, Modifier.padding(horizontal = 24.dp))
        Row(Modifier.fillMaxWidth().padding(top = 8.dp).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("← Previous", Modifier.tap(label = "Previous day") { onPick(shown.minusDays(1)) }.padding(horizontal = 16.dp), style = link)
            Spacer(Modifier.weight(1f))
            if (shown != today) Text("TODAY", Modifier.tap { onPick(today) }, style = Type.control.copy(color = p.muted))
            Spacer(Modifier.weight(1f))
            Text("Next →", Modifier.tap(label = "Next day") { onPick(shown.plusDays(1)) }.padding(horizontal = 16.dp), style = link)
        }
        Row(Modifier.fillMaxWidth().padding(top = 12.dp).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
            // A month's step in the days, a year's in the months; none past the picker's span.
            val step = if (months) 12L else 1L
            val back = month.minusMonths(step).takeIf { it.year in PICKER_YEARS }
            val forward = month.plusMonths(step).takeIf { it.year in PICKER_YEARS }
            val stepStyle = link.copy(fontSize = 22.sp)
            Text(
                "‹",
                Modifier.then(if (back != null) Modifier.tap(label = if (months) "Previous year" else "Previous month") { month = back } else Modifier)
                    .padding(horizontal = 24.dp),
                style = if (back != null) stepStyle else stepStyle.copy(color = p.muted.copy(alpha = 0.35f)),
            )
            val title = if (months) "${month.year}" else "${month.month.getDisplayName(JavaTextStyle.FULL, Locale.US)} ${month.year}"
            Row(
                Modifier.weight(1f).heightIn(min = 44.dp)
                    .tap(label = if (months) "$title, back to days" else "$title, choose another month") { months = !months },
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(title.uppercase(), style = Type.label(12.8f, 0.1f).copy(color = p.text, textAlign = TextAlign.Center))
                Caret(months)
            }
            Text(
                "›",
                Modifier.then(if (forward != null) Modifier.tap(label = if (months) "Next year" else "Next month") { month = forward } else Modifier)
                    .padding(horizontal = 24.dp),
                style = if (forward != null) stepStyle else stepStyle.copy(color = p.muted.copy(alpha = 0.35f)),
            )
        }
        Box(Modifier.fillMaxWidth().padding(horizontal = 8.dp)) {
            // The days always lay out, for the height both views share; the months cover them.
            Column(Modifier.then(if (months) Modifier.alpha(0f).clearAndSetSemantics {} else Modifier)) { Days(month, shown, today, live = !months, onPick) }
            if (months) {
                MonthGrid(month.year, shown, today, Modifier.matchParentSize()) { m ->
                    month = YearMonth.of(month.year, m)
                    months = false
                }
            }
        }
    }
}

private fun YearMonth.coerceIn(years: IntRange): YearMonth = when {
    year < years.first -> YearMonth.of(years.first, 1)
    year > years.last -> YearMonth.of(years.last, 12)
    else -> this
}

/** A month's days in six weeks always, blank where the month is not; `live` false while the months cover them. */
@Composable
private fun Days(month: YearMonth, shown: LocalDate, today: LocalDate, live: Boolean, onPick: (LocalDate) -> Unit) {
    val p = LocalPalette.current
    val days = listOf(DayOfWeek.SUNDAY) + DayOfWeek.entries.filter { it != DayOfWeek.SUNDAY }
    // The weekday letters are for the eye; each day below names itself in full.
    Row(Modifier.fillMaxWidth().clearAndSetSemantics {}) {
        days.forEach { d ->
            Text(
                d.getDisplayName(JavaTextStyle.NARROW, Locale.US),
                Modifier.weight(1f).padding(vertical = 8.dp),
                style = Type.label(11.5f, 0.08f).copy(color = p.muted, textAlign = TextAlign.Center),
            )
        }
    }
    val lead = month.atDay(1).dayOfWeek.value % 7
    val cells = (0 until 42).map { i -> (i - lead + 1).takeIf { it in 1..month.lengthOfMonth() }?.let(month::atDay) }
    cells.chunked(7).forEach { week ->
        Row(Modifier.fillMaxWidth()) {
            week.forEach { day ->
                Box(
                    Modifier
                        .weight(1f)
                        .heightIn(min = 44.dp)
                        .then(if (day == today) Modifier.background(p.pressedWash) else Modifier)
                        .then(if (day != null && live) Modifier.tap(label = spokenDay(day, today), selected = day == shown) { onPick(day) } else Modifier),
                    contentAlignment = Alignment.Center,
                ) {
                    if (day != null) {
                        Text(
                            day.dayOfMonth.toString(),
                            Modifier.goldUnderline(day == shown, p.goldLine),
                            style = Type.body.copy(fontSize = 18.sp, lineHeight = 30.sp, color = if (day.dayOfWeek == DayOfWeek.SUNDAY) p.rubric else p.text, fontFeatureSettings = "lnum"),
                        )
                    }
                }
            }
        }
    }
}

/** The year's months, three to a row: the chosen day's month underlined in gold, today's washed. */
@Composable
private fun MonthGrid(year: Int, shown: LocalDate, today: LocalDate, modifier: Modifier, onMonth: (Int) -> Unit) {
    val p = LocalPalette.current
    Column(modifier, verticalArrangement = Arrangement.SpaceEvenly) {
        (1..12).chunked(3).forEach { row ->
            Row(Modifier.fillMaxWidth()) {
                row.forEach { m ->
                    val name = Month.of(m).getDisplayName(JavaTextStyle.FULL, Locale.US)
                    val chosen = shown.year == year && shown.monthValue == m
                    Box(Modifier.weight(1f), contentAlignment = Alignment.Center) {
                        Text(
                            Month.of(m).getDisplayName(JavaTextStyle.SHORT, Locale.US).uppercase(),
                            Modifier
                                .then(if (today.year == year && today.monthValue == m) Modifier.background(p.gold.copy(alpha = 0.1f)) else Modifier)
                                .tap(label = "$name $year", selected = chosen) { onMonth(m) }
                                .heightIn(min = 44.dp)
                                .goldUnderline(chosen, p.goldLine)
                                .padding(horizontal = 14.4.dp, vertical = 13.dp),
                            style = Type.label(11.5f, 0.1f).copy(color = if (chosen) p.accent else p.text),
                        )
                    }
                }
            }
        }
    }
}

/** "How are you praying?": the three prayer forms, remembered on this device. */
@Composable
fun FormChooser(form: String, onForm: (String) -> Unit) {
    val p = LocalPalette.current
    Column(Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 4.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Text("How are you praying?", style = Type.body.copy(fontSize = 16.sp, lineHeight = 24.sp, color = p.muted))
        Spacer(Modifier.height(4.dp))
        PRAYER_FORMS.forEach { (value, _, phrase) ->
            Row(Modifier.heightIn(min = 44.dp).tap(role = Role.RadioButton, selected = value == form) { onForm(value) }, verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(16.dp).border(1.dp, if (value == form) p.gold else p.border, CircleShape), contentAlignment = Alignment.Center) {
                    if (value == form) Box(Modifier.size(8.dp).background(p.gold, CircleShape))
                }
                Spacer(Modifier.width(10.dp))
                Text(phrase, style = Type.body.copy(fontSize = 17.sp, lineHeight = 24.sp, color = p.text))
            }
        }
        Text("Remembered on this device. Change it any time.", Modifier.padding(top = 4.dp), style = Type.small.copy(color = p.muted))
    }
}

/**
 * The continuation after a page's content: the diamond, then the previous item, the way back
 * to all of them, and the next item.
 */
@Composable
fun Continuation(
    previousLabel: String,
    previous: String?,
    onPrevious: () -> Unit,
    middle: String,
    onMiddle: () -> Unit,
    nextLabel: String,
    next: String?,
    onNext: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val p = LocalPalette.current
    val small = Type.label(11.2f, 0.08f).copy(color = p.muted)
    val name = Type.body.copy(fontSize = 16.sp, lineHeight = 22.sp, color = p.accent)
    Column(modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
        Diamond(size = 7.dp)
        Row(Modifier.fillMaxWidth().padding(top = 24.dp), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f).then(if (previous != null) Modifier.tap(label = "$previousLabel: $previous", onClick = onPrevious) else Modifier), horizontalAlignment = Alignment.Start) {
                if (previous != null) {
                    Text(previousLabel.uppercase(), style = small)
                    Text("← $previous", style = name)
                }
            }
            Text(middle.uppercase(), Modifier.tap(label = middle, onClick = onMiddle).padding(horizontal = 6.4.dp, vertical = 12.dp), style = Type.label(12.16f, 0.06f).copy(color = p.muted))
            Column(Modifier.weight(1f).then(if (next != null) Modifier.tap(label = "$nextLabel: $next", onClick = onNext) else Modifier), horizontalAlignment = Alignment.End) {
                if (next != null) {
                    Text(nextLabel.uppercase(), style = small)
                    Text("$next →", style = name)
                }
            }
        }
    }
}

/** The page's foot: the diamond and the Office's name, and on a wide screen the reader's preferences. */
@Composable
fun Footer(modifier: Modifier = Modifier, diamond: Boolean = true) {
    val p = LocalPalette.current
    val prefs = LocalPrefs.current
    Column(modifier.fillMaxWidth().padding(top = 40.dp, bottom = 24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        if (diamond) Diamond(Modifier.padding(bottom = 16.dp), size = 7.dp)
        Text("Benedictine Divine Office", style = Type.small.copy(color = p.muted))
        if (LocalWide.current && prefs != null) FooterPrefs(prefs)
    }
}

/**
 * The desktop footer's preferences (`.footer-prefs`): Default / Nave / Apse above, the three
 * text sizes below, between hairline courses rather than in boxes, the chosen one in the
 * accent over a gold rule, as the current hour is marked on home.
 */
@Composable
private fun FooterPrefs(prefs: Prefs) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    @Composable
    fun Course(content: @Composable RowScope.() -> Unit) {
        Row(
            Modifier.height(IntrinsicSize.Min).drawBehind {
                drawLine(p.surfaceEdge, Offset(0f, 0f), Offset(size.width, 0f), 1.dp.toPx())
                drawLine(p.surfaceEdge, Offset(0f, size.height), Offset(size.width, size.height), 1.dp.toPx())
            },
            content = content,
        )
    }
    @Composable
    fun RowScope.Option(label: String, chosen: Boolean, last: Boolean, style: TextStyle, spoken: String, onClick: () -> Unit) {
        Box(
            Modifier.fillMaxHeight().heightIn(min = 32.dp).tap(label = spoken, selected = chosen, onClick = onClick)
                .drawBehind {
                    if (!last) drawLine(p.surfaceEdge, Offset(size.width, 0f), Offset(size.width, size.height), 1.dp.toPx())
                    if (chosen) drawLine(o.flat, Offset(0f, size.height - 0.5.dp.toPx()), Offset(size.width, size.height - 0.5.dp.toPx()), 1.dp.toPx())
                }
                .padding(horizontal = 10.4.dp, vertical = 6.4.dp),
            contentAlignment = Alignment.Center,
        ) { Text(label, style = style.copy(color = if (chosen) p.accent else p.muted)) }
    }
    Column(Modifier.padding(top = 13.6.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Course {
            ThemeChoice.entries.forEachIndexed { i, t ->
                Option(t.label.uppercase(), t == prefs.theme, i == ThemeChoice.entries.lastIndex, Type.label(11.52f, 0.06f), "${t.label} theme") { prefs.onTheme(t) }
            }
        }
        Course {
            TextSize.entries.forEachIndexed { i, s ->
                // The size ladder is the label.
                val size = when (s) { TextSize.SMALL -> 10.4f; TextSize.DEFAULT -> 13.7f; TextSize.LARGE -> 18.7f }
                val spoken = when (s) { TextSize.SMALL -> "Smaller text"; TextSize.DEFAULT -> "Default text size"; TextSize.LARGE -> "Larger text" }
                Option("A", s == prefs.textSize, i == TextSize.entries.lastIndex, Type.label(size, 0f), spoken) { prefs.onTextSize(s) }
            }
        }
    }
}

/** A day as a screen reader says it in the picker: "Sunday, March 15, today". */
fun spokenDay(day: LocalDate, today: LocalDate): String =
    day.format(DateTimeFormatter.ofPattern("EEEE, MMMM d", Locale.US)) + if (day == today) ", today" else ""
