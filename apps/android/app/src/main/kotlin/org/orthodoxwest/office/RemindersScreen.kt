package org.orthodoxwest.office

import android.text.format.DateFormat
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.FastOutLinearInEasing
import androidx.compose.animation.core.LinearOutSlowInEasing
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Text
import androidx.compose.material3.TimePicker
import androidx.compose.material3.TimePickerDefaults
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.isTraversalGroup
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.traversalIndex
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle as JavaTextStyle
import java.time.ZonedDateTime
import java.util.Locale
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.orthodoxwest.office.core.RuleHourChoice
import org.orthodoxwest.office.core.reminderNext
import org.orthodoxwest.office.core.reminderPeriods
import org.orthodoxwest.office.core.reminderRule
import org.orthodoxwest.office.core.sunUnavailableNote

/** Whether the phone will let reminders through, and on the minute, and tell the app where it is. */
data class ReminderStatus(val notificationsAllowed: Boolean, val exactAllowed: Boolean, val locationAllowed: Boolean = true)

/** Monday first, as the web's Days row. */
private val WEEK = DayOfWeek.entries.toList()

/**
 * The reminders page, set as the web's: the hours with their times, the days, how long before;
 * then, where the web gives a calendar link, the switch that asks the phone itself.
 */
@Composable
fun RemindersScreen(
    settings: ReminderSettings,
    status: ReminderStatus,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onChange: (ReminderSettings) -> Unit,
    onTurnOn: () -> Unit,
    onTurnOff: () -> Unit,
    onAllowNotifications: () -> Unit,
    onAllowExact: () -> Unit,
    today: LocalDate = LocalDate.now(),
    onFollowSun: (Boolean) -> Unit = {},
    onAllowLocation: () -> Unit = {},
) {
    val p = LocalPalette.current
    val haptics = LocalHapticFeedback.current
    val context = LocalContext.current
    val is24 = remember { DateFormat.is24HourFormat(context) }
    val clock = remember(is24) { DateTimeFormatter.ofPattern(if (is24) "HH:mm" else "h:mm a", Locale.getDefault()) }
    var editing by remember { mutableStateOf<HourReminder?>(null) }
    val body = Type.body.copy(color = p.text)
    // Today's times by the sun, while the hours follow it.
    val sun = remember(settings, today) { settings.sunTimes(today, ZoneId.systemDefault()) }
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        chrome()
        Column(Modifier.measure().padding(top = 24.dp)) {
            PlainHeadpiece()
            Text("Set prayer reminders", Modifier.padding(top = 10.dp).semantics { heading() }, style = body.copy(fontSize = 26.sp, lineHeight = 32.sp))
            Text(
                "Choose the hours you pray and their times. Each reminder names the office and the feast of the day. " +
                    "Your schedule stays on this phone, and reminders come without a connection.",
                Modifier.padding(top = 10.dp),
                style = body.copy(color = p.muted),
            )
            Times(settings, status, Modifier.padding(top = 28.dp), onFollowSun, onAllowLocation)
            Group("Hours", Modifier.padding(top = 28.dp)) {
                // The hours in home's three periods, each beside its painted sun or moon, as home and
                // About set them; the periods parted by the table's rule.
                val o = LocalOrnament.current
                val tableRule = if (p.dark) Color(208, 176, 106).copy(alpha = 0.24f) else Color(107, 58, 31).copy(alpha = 0.22f)
                // Where the name and its time would crowd (a narrow phone, or large text), the period
                // keeps its sun or moon and gives up its word, as the web's does.
                val compact = LocalDensity.current.fontScale > 1.3f || LocalConfiguration.current.screenWidthDp < 360
                PERIODS.forEachIndexed { i, period ->
                    if (i > 0) Hairline(tableRule)
                    Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min).semantics { isTraversalGroup = true }) {
                        Column(
                            Modifier.width(if (compact) 32.dp else 70.4.dp).fillMaxHeight().clearAndSetSemantics {},
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.Center,
                        ) {
                            PeriodIcon(periodOf(period.label), o.flat)
                            if (!compact) {
                                Spacer(Modifier.height(2.56.dp))
                                Text(
                                    period.label,
                                    style = Type.label(11.52f, 0.08f).copy(color = p.muted, fontFeatureSettings = ALL_SMALL_CAPS, lineHeight = 13.8.sp),
                                    maxLines = 1,
                                )
                            }
                        }
                        Box(Modifier.width(1.dp).fillMaxHeight().drawBehind { drawRect(tableRule) })
                        Column(Modifier.weight(1f).padding(start = 9.6.dp)) {
                            period.hours.mapNotNull { slug -> settings.hours.firstOrNull { it.hour == slug } }.forEachIndexed { j, h ->
                                if (j > 0) Hairline(p.border)
                                HourRow(h, settings, sun, clock, onChange) { editing = h }
                            }
                        }
                    }
                }
            }
            Group("Days", Modifier.padding(top = 28.dp)) {
                // One ruled week, as the date picker sets its week: a chosen day's name in ink over the
                // gold rule of a chosen thing, an unchosen day's muted with none, Sunday in rubric; the
                // hand-set box beneath is still the control's sign.
                val hairline = p.border.copy(alpha = p.border.alpha * 0.58f)
                Row(Modifier.fillMaxWidth()) {
                    WEEK.forEachIndexed { i, d ->
                        val on = d in settings.days
                        val sunday = d == DayOfWeek.SUNDAY
                        val ink by animateColorAsState(
                            when {
                                sunday && on -> p.rubric
                                sunday -> p.rubric.copy(alpha = 0.6f)
                                on -> p.text
                                else -> p.muted
                            },
                            tickSpec(on),
                            label = "day ink",
                        )
                        val rule by animateFloatAsState(if (on) 1f else 0f, tickSpec(on), label = "day rule")
                        Column(
                            Modifier.weight(1f).heightIn(min = 44.dp).check(on) {
                                haptics.toggled(it)
                                onChange(settings.copy(days = if (it) settings.days + d else settings.days - d))
                            }
                                .semantics { contentDescription = d.getDisplayName(JavaTextStyle.FULL, Locale.US) }
                                .drawBehind { if (i > 0) drawRect(hairline, size = size.copy(width = 1.dp.toPx())) }
                                .padding(top = 8.dp, bottom = 8.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            Text(
                                d.getDisplayName(JavaTextStyle.SHORT, Locale.US),
                                Modifier.fillMaxWidth().drawBehind {
                                    val w = 1.dp.toPx()
                                    drawLine(p.goldLine.copy(alpha = p.goldLine.alpha * rule), Offset(0f, size.height - w / 2), Offset(size.width, size.height - w / 2), w)
                                }.padding(bottom = 3.dp),
                                style = body.copy(fontSize = 15.2.sp, lineHeight = 20.sp, color = ink, textAlign = TextAlign.Center),
                            )
                            Spacer(Modifier.height(6.dp))
                            Tick(on)
                        }
                    }
                }
            }
            Group("Remind me", Modifier.padding(top = 28.dp)) {
                var open by remember { mutableStateOf(false) }
                Box {
                    // A worded line with the app's caret, not a boxed field.
                    Row(
                        Modifier.fillMaxWidth().heightIn(min = 46.dp).tap(action = "choose when") { open = true }.disclosed(open)
                            .padding(start = 2.dp, end = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(REMINDER_LEADS.first { it.first == settings.lead }.second, Modifier.weight(1f), style = body.copy(fontFeatureSettings = "lnum"))
                        Caret(open, p.goldLine, lighter = false)
                    }
                    // The lead's list in the church's frame: square, on the surface, ruled as the menu panel.
                    DropdownMenu(
                        expanded = open,
                        onDismissRequest = { open = false },
                        shape = RectangleShape,
                        containerColor = p.surface,
                        border = BorderStroke(1.dp, p.border),
                    ) {
                        REMINDER_LEADS.forEach { (minutes, label) ->
                            DropdownMenuItem(text = { Text(label, style = body.copy(fontFeatureSettings = "lnum")) }, onClick = { open = false; onChange(settings.copy(lead = minutes)) })
                        }
                    }
                }
            }
            RuleLines(settings, sun, clock, status, today, Modifier.padding(top = 18.dp))
            // The web's "Your calendar link": a titulus over the tablet, no rule between.
            Text(
                "On this phone",
                Modifier.fillMaxWidth().padding(top = 32.dp).semantics { heading() },
                style = Type.label(13.12f, 0.12f).copy(color = p.titulus, fontFeatureSettings = ALL_SMALL_CAPS, textAlign = TextAlign.Center),
            )
            // The tablet stays where it is: its words change in place to "Reminders are on", and the
            // way to turn them off unfolds beneath it as a worded link, not a boxed button.
            TabletButton(
                if (settings.on) "Reminders are on" else "Turn on reminders",
                Modifier.padding(top = 12.dp),
                enabled = !settings.on,
                onClick = onTurnOn,
            )
            Unfold(settings.on, reveal = false) {
                Text(
                    "Turn off reminders",
                    Modifier.align(Alignment.CenterHorizontally).padding(top = 6.dp).heightIn(min = 44.dp).tap(onClick = onTurnOff)
                        .padding(vertical = 10.dp).goldUnderline(true, p.goldLine),
                    style = Type.body.copy(fontSize = 16.sp, lineHeight = 22.sp, color = p.accent),
                )
                if (!status.notificationsAllowed) {
                    Note("Notifications are off for the Daily Office, so reminders cannot appear.", "Allow notifications", onAllowNotifications)
                }
                if (!status.exactAllowed) {
                    Note("Reminders may come a few minutes late. For the exact minute, allow alarms and reminders.", "Allow alarms", onAllowExact)
                }
            }
        }
        Footer()
    }
    editing?.let { h -> TimeDialog(h, is24, onDismiss = { editing = null }) { t -> editing = null; onChange(settings.withHour(h.copy(time = t))) } }
}

private fun ReminderSettings.withHour(h: HourReminder) = copy(hours = hours.map { if (it.hour == h.hour) h else it })

/** The hours in home's three periods (`presentation::about`), as home and About group them. */
private val PERIODS by lazy { reminderPeriods() }

/** How a tick answers, on every front as the web's: 140ms out to the chosen state, 120ms back. */
private fun <T> tickSpec(on: Boolean) = if (on) tween<T>(140, easing = LinearOutSlowInEasing) else tween<T>(120, easing = FastOutLinearInEasing)

/**
 * One hour: the box and its name are one checkbox, and its time a control of its own. The figures,
 * their gold rule and the caret answer the tick together; an unchosen hour's faint time chooses the
 * hour when tapped (hidden from TalkBack, for which the checkbox is the way in).
 */
@Composable
private fun HourRow(
    h: HourReminder,
    settings: ReminderSettings,
    sun: Map<String, LocalTime>?,
    clock: DateTimeFormatter,
    onChange: (ReminderSettings) -> Unit,
    onEdit: () -> Unit,
) {
    val p = LocalPalette.current
    val haptics = LocalHapticFeedback.current
    val body = Type.body.copy(color = p.text)
    val choose = {
        haptics.toggled(true)
        onChange(settings.withHour(h.copy(chosen = true)))
    }
    val name by animateColorAsState(if (h.chosen) p.text else p.muted, tickSpec(h.chosen), label = "hour ink")
    val figures by animateColorAsState(if (h.chosen) p.accent else p.muted.copy(alpha = 0.55f), tickSpec(h.chosen), label = "time ink")
    val chosen by animateFloatAsState(if (h.chosen) 1f else 0f, tickSpec(h.chosen), label = "time rule")
    Row(Modifier.fillMaxWidth().heightIn(min = 46.dp), verticalAlignment = Alignment.CenterVertically) {
        Row(
            Modifier.weight(1f).heightIn(min = 46.dp).check(h.chosen) {
                haptics.toggled(it)
                onChange(settings.withHour(h.copy(chosen = it)))
            },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Tick(h.chosen)
            // Muted until chosen, as the web's `.reminder-hour-row.is-selected`.
            Text(h.name, Modifier.padding(start = 12.dp), style = body.copy(color = name))
        }
        val bySun = sun?.get(h.hour)?.takeIf { h.hour != "compline" }
        val label = (bySun ?: h.time).format(clock)
        if (bySun != null) {
            if (h.chosen) {
                SunTime(label, h, true)
            } else {
                Box(Modifier.heightIn(min = 44.dp).clearAndSetSemantics {}.tap(onClick = choose), contentAlignment = Alignment.Center) { SunTime(label, h, false) }
            }
        } else {
            // The accent's figures on the chosen control's gold rule, with the caret that says the
            // time can be changed (the web's time field).
            Row(
                Modifier.heightIn(min = 44.dp).then(
                    if (h.chosen) {
                        Modifier.tap(label = "${h.name} at $label", action = "change the time", onClick = onEdit)
                    } else {
                        Modifier.clearAndSetSemantics {}.tap(onClick = choose)
                    },
                ),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(
                    Modifier.drawBehind {
                        val w = 1.dp.toPx()
                        drawLine(p.goldLine, Offset(0f, size.height - w / 2), Offset(size.width * chosen, size.height - w / 2), w)
                    }.padding(start = 2.dp, end = 2.dp, top = 3.dp, bottom = 3.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(label, style = body.copy(color = figures, fontFeatureSettings = "lnum, tnum"))
                    Box(Modifier.alpha(chosen)) { Caret(false, p.goldLine, lighter = false) }
                }
            }
        }
    }
    if (sun != null && h.chosen && h.hour == "compline") ComplineNote(sun[h.hour]?.takeIf { it != h.time }?.format(clock))
}

/**
 * The rule, written out (`presentation::reminders`): what the choices ask for, in one sentence,
 * and under it the reminder that will really come next, from the scheduler's own plan. Two lines
 * are kept, so a change of a day or a time moves nothing below; the sentence cross-fades as it
 * changes (90ms out, 180ms in after a beat, the app's fade pair).
 */
@Composable
private fun RuleLines(
    settings: ReminderSettings,
    sun: Map<String, LocalTime>?,
    clock: DateTimeFormatter,
    status: ReminderStatus,
    today: LocalDate,
    modifier: Modifier,
) {
    val p = LocalPalette.current
    val chosen = settings.hours.filter { it.chosen }.map { RuleHourChoice(it.hour, it.time.format(clock), sun != null && it.hour != "compline") }
    val said = remember(chosen, settings.days, settings.lead) { reminderRule(chosen, settings.weekdays, settings.lead) }.joinToString("") { it.text }
    val empty = settings.hours.none { it.chosen } || settings.days.isEmpty()
    var next by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(settings, status.notificationsAllowed, today) {
        next = if (!settings.on || !status.notificationsAllowed) {
            null
        } else {
            withContext(Dispatchers.Default) {
                runCatching {
                    val now = ZonedDateTime.now()
                    ReminderScheduler.plan(settings, now).minByOrNull { it.at }?.let { a ->
                        reminderNext(a.hour, now.toLocalDate().toCivil(), a.at.toLocalDate().toCivil(), a.at.format(clock))
                    }
                }.getOrNull()
            }
        }
    }
    Column(modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
        AnimatedContent(said, transitionSpec = { ruleFade() }, label = "rule") { words ->
            Text(
                words,
                Modifier.fillMaxWidth().heightIn(min = 54.dp),
                style = Type.body.copy(fontSize = 18.sp, lineHeight = 27.sp, color = if (empty) p.muted else p.text, textAlign = TextAlign.Center, fontFeatureSettings = "lnum"),
            )
        }
        AnimatedContent(next?.takeIf { settings.on && !empty }, transitionSpec = { ruleFade() }, label = "next") { line ->
            if (line != null) {
                Text(
                    line,
                    Modifier.fillMaxWidth().padding(top = 4.dp),
                    style = Type.body.copy(fontSize = 16.sp, lineHeight = 23.sp, color = p.muted, textAlign = TextAlign.Center, fontFeatureSettings = "lnum"),
                )
            }
        }
    }
}

/** The rule's cross-fade: the old words out in 90ms, the new in over 180ms after a beat. */
private fun ruleFade() = fadeIn(tween(180, delayMillis = 150, easing = LinearOutSlowInEasing)) togetherWith fadeOut(tween(90, easing = FastOutLinearInEasing))

/** What marks each hour's time by the sun, under the time. */
private val SUN_MARKS = mapOf(
    "lauds" to "sunrise",
    "prime" to "first hour",
    "terce" to "third hour",
    "sext" to "solar noon",
    "none" to "ninth hour",
    "vespers" to "sunset",
)

/**
 * Whether the hours keep the reader's times or follow the sun, the reason the sun cannot be
 * followed here, and the way to let the app know where it is. Kept apart from the page around it,
 * to be restyled on its own. Past the latitude limit the sun's row is greyed and cannot be chosen;
 * choosing the reader's own times forgets the place, so the sun can be chosen again elsewhere.
 */
@Composable
private fun Times(
    settings: ReminderSettings,
    status: ReminderStatus,
    modifier: Modifier,
    onFollowSun: (Boolean) -> Unit,
    onAllowLocation: () -> Unit,
) {
    val p = LocalPalette.current
    val haptics = LocalHapticFeedback.current
    val body = Type.body.copy(color = p.text)
    val unavailable = settings.place?.sunAvailable == false
    // Past the limit the reader's own times are in force, whatever was chosen; the choice is kept
    // so the sun takes over again when the app is next opened nearer the equator.
    val bySun = settings.followSun && !unavailable
    Group("Times", modifier) {
        @Composable
        fun Choice(label: String, selected: Boolean, enabled: Boolean, sun: Boolean) {
            Row(
                Modifier.fillMaxWidth().heightIn(min = 46.dp).alpha(if (enabled) 1f else 0.5f)
                    .selectable(selected, enabled = enabled, role = Role.RadioButton) {
                        if (!selected) {
                            haptics.toggled(true)
                            onFollowSun(sun)
                        }
                    },
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Mark(selected)
                Text(label, Modifier.padding(start = 12.dp), style = body)
            }
        }
        Column(Modifier.selectableGroup()) {
            Choice("At times I choose", !bySun, enabled = true, sun = false)
            Hairline(p.border)
            Choice("By the sun where I am", bySun, enabled = !unavailable, sun = true)
        }
        if (unavailable) {
            Remark(sunUnavailableNote())
        }
        // Choosing the sun unfolds its paragraph, as a disclosure does, rather than jumping it in.
        Unfold(settings.followSun && !unavailable, reveal = false) {
            Text(
                "Lauds at sunrise, Vespers at sunset, and the little hours at the first, third, sixth and ninth hours of daylight. " +
                    "Times for where you were when you last opened the app.",
                Modifier.padding(top = 2.dp, bottom = 10.dp),
                style = Type.body.copy(fontSize = 16.sp, lineHeight = 23.sp, color = p.muted),
            )
        }
        Unfold(settings.followSun && !status.locationAllowed, reveal = false) {
            Note("Allow location to time reminders by the sun.", "Open settings", onAllowLocation)
            Spacer(Modifier.height(6.dp))
        }
    }
}

/** An hour's time as the sun sets it today: plain, with what marks it beneath; faint when not chosen. */
@Composable
private fun SunTime(label: String, h: HourReminder, chosen: Boolean) {
    val p = LocalPalette.current
    val time = Type.body.copy(fontFeatureSettings = "lnum")
    if (!chosen) {
        Text(label, Modifier.padding(horizontal = 11.dp), style = time.copy(color = p.muted.copy(alpha = 0.6f)))
        return
    }
    val mark = SUN_MARKS[h.hour].orEmpty()
    Column(
        Modifier.padding(horizontal = 11.dp, vertical = 4.dp).semantics(mergeDescendants = true) { contentDescription = "${h.name} at $label, $mark" },
        horizontalAlignment = Alignment.End,
    ) {
        Text(label, style = time.copy(color = p.text))
        Text(mark, style = Type.label(11f, 0.08f).copy(color = p.gold, fontFeatureSettings = ALL_SMALL_CAPS, lineHeight = 14.sp))
    }
}

/**
 * Under a chosen Compline while the hours follow the sun: its time is the reader's, but it waits
 * for the evening; `tonight` is the later time the sun sets for it today, if any.
 */
@Composable
private fun ComplineNote(tonight: String?) {
    val p = LocalPalette.current
    Text(
        if (tonight != null) "Tonight at $tonight, an hour after sunset." else "Your time, moved later when the sun sets late.",
        Modifier.padding(start = 32.dp, bottom = 8.dp),
        style = Type.body.copy(fontSize = 14.sp, lineHeight = 20.sp, color = p.muted),
    )
}

/** The radio mark, the round sister of [Tick]: a ruled circle, an oak dot inside a margin of the ground when chosen. */
@Composable
private fun Mark(selected: Boolean) {
    val p = LocalPalette.current
    Canvas(Modifier.size(20.dp)) {
        val r = 15.2.dp.toPx() / 2
        val rule = 1.dp.toPx()
        drawCircle(p.bg, r, center)
        if (selected) drawCircle(p.accent, r - rule - 3.dp.toPx(), center)
        drawCircle(if (selected) p.accent else p.muted, r - rule / 2, center, style = Stroke(rule))
    }
}

/** A remark set as a [Note] is, with its gold rule, but with nothing to do about it. */
@Composable
private fun Remark(text: String) {
    val p = LocalPalette.current
    Text(
        text,
        Modifier.fillMaxWidth().padding(top = 6.dp, bottom = 12.dp).drawBehind { drawLine(p.goldLine, Offset(0f, 0f), Offset(0f, size.height), 1.dp.toPx()) }.padding(start = 12.dp),
        style = Type.body.copy(fontSize = 16.sp, lineHeight = 23.sp, color = p.muted),
    )
}

/** The web's headpiece, set to the left: a painted cross between two short rules (Reminders, About). */
@Composable
fun PlainHeadpiece() {
    val ink = LocalPalette.current.lining
    val rule = ink.copy(alpha = 0.55f)
    Row(verticalAlignment = Alignment.CenterVertically) {
        Canvas(Modifier.width(22.dp).height(1.dp)) { drawRect(rule) }
        Text("✠", Modifier.padding(horizontal = 8.dp), style = TextStyle(fontFamily = CrossFont, fontSize = 11.sp, color = ink))
        Canvas(Modifier.width(22.dp).height(1.dp)) { drawRect(rule) }
    }
}

/**
 * A group of the page's controls under its name, ruled as home's hours are and as the web's
 * Reminders fieldsets: the name in the titulus's small caps over a rule of the lining, the rows on
 * the page itself, parted by hairlines, and a closing rule. A screen reader takes the name as the
 * group's heading.
 */
@Composable
private fun Group(name: String, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    val p = LocalPalette.current
    val rule = p.lining.copy(alpha = 0.6f)
    Column(
        modifier.fillMaxWidth().semantics { isTraversalGroup = true }.drawBehind {
            val w = 1.dp.toPx()
            drawLine(rule, Offset(0f, size.height - w / 2), Offset(size.width, size.height - w / 2), w)
        },
    ) {
        Text(
            name,
            Modifier.fillMaxWidth()
                .drawBehind {
                    val w = 1.dp.toPx()
                    drawLine(rule, Offset(0f, size.height - w / 2), Offset(size.width, size.height - w / 2), w)
                }
                .padding(bottom = 5.dp)
                .semantics { heading(); traversalIndex = -1f },
            style = Type.label(13.12f, 0.12f).copy(color = p.titulus, fontFeatureSettings = ALL_SMALL_CAPS),
        )
        content()
    }
}

/**
 * A checkbox set by hand, as the web's are: a ruled square, 0.95rem, that fills with oak inside a
 * margin of the ground when chosen. The platform's own box belongs to no theme here. The row
 * around it is the control. The oak square grows from the box's centre as the name takes its ink,
 * in the web's 140ms (120ms back); with the phone's animations off it is simply there.
 */
@Composable
private fun Tick(checked: Boolean) {
    val p = LocalPalette.current
    val fill by animateFloatAsState(if (checked) 1f else 0f, tickSpec(checked), label = "tick")
    val edge by animateColorAsState(if (checked) p.accent else p.muted, tickSpec(checked), label = "tick edge")
    Canvas(Modifier.size(20.dp)) {
        val side = 15.2.dp.toPx()
        val corner = Offset((size.width - side) / 2, (size.height - side) / 2)
        val box = Size(side, side)
        val rule = 1.dp.toPx()
        drawRect(p.bg, corner, box)
        if (fill > 0f) {
            val inner = (side - 2 * (rule + 3.dp.toPx())) * fill
            drawRect(p.accent, Offset(center.x - inner / 2, center.y - inner / 2), Size(inner, inner))
        }
        drawRoundRect(
            edge,
            corner + Offset(rule / 2, rule / 2),
            Size(side - rule, side - rule),
            CornerRadius(1.dp.toPx()),
            style = Stroke(rule),
        )
    }
}

/** A note about the phone's permissions, with the way to change them. */
@Composable
private fun Note(text: String, action: String, onAction: () -> Unit) {
    val p = LocalPalette.current
    Column(
        Modifier.fillMaxWidth().padding(top = 16.dp).drawBehind { drawLine(p.goldLine, Offset(0f, 0f), Offset(0f, size.height), 1.dp.toPx()) }.padding(start = 12.dp),
    ) {
        Text(text, style = Type.body.copy(fontSize = 16.sp, lineHeight = 23.sp, color = p.muted))
        Text(action, Modifier.heightIn(min = 44.dp).tap(onClick = onAction).padding(vertical = 10.dp).goldUnderline(true, p.goldLine), style = Type.menu.copy(color = p.accent))
    }
}

/**
 * The time of one hour's reminder, on Material's own dial and its keyboard entry, in the church's
 * frame: a square dialog ruled as the menu panel with the painted line across its top, the hour's
 * name as a titulus beside its period's sun or moon, and "Set" and "Cancel" as the app's worded
 * links. The dial's figures are left as Material sets them.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun TimeDialog(hour: HourReminder, is24: Boolean, onDismiss: () -> Unit, onPick: (LocalTime) -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val state = rememberTimePickerState(hour.time.hour, hour.time.minute, is24)
    val period = PERIODS.firstOrNull { hour.hour in it.hours }?.let { periodOf(it.label) } ?: Period.DAY
    val link = Type.body.copy(fontSize = 17.6.sp, lineHeight = 24.sp)
    AlertDialog(
        onDismissRequest = onDismiss,
        modifier = Modifier.border(1.dp, p.border, RectangleShape).drawWithContent {
            drawContent()
            drawRect(o.line, size = size.copy(height = 2.dp.toPx()))
        },
        shape = RectangleShape,
        containerColor = p.surface,
        title = {
            Row(Modifier.semantics(mergeDescendants = true) { heading() }, verticalAlignment = Alignment.CenterVertically) {
                PeriodIcon(period, o.flat)
                Text(hour.name, Modifier.padding(start = 6.dp), style = Type.label(13.12f, 0.12f).copy(color = p.titulus, fontFeatureSettings = ALL_SMALL_CAPS))
            }
        },
        confirmButton = {
            Text(
                "Set",
                Modifier.heightIn(min = 44.dp).tap { onPick(LocalTime.of(state.hour, state.minute)) }.padding(horizontal = 8.dp, vertical = 10.dp).goldUnderline(true, p.goldLine),
                style = link.copy(color = p.accent),
            )
        },
        dismissButton = {
            Text("Cancel", Modifier.heightIn(min = 44.dp).tap(onClick = onDismiss).padding(horizontal = 8.dp, vertical = 10.dp), style = link.copy(color = p.muted))
        },
        text = {
            TimePicker(
                state,
                colors = TimePickerDefaults.colors(
                    clockDialColor = p.bg,
                    selectorColor = p.accent,
                    timeSelectorSelectedContainerColor = p.pressedWash,
                    timeSelectorSelectedContentColor = p.text,
                    timeSelectorUnselectedContainerColor = p.bg,
                    timeSelectorUnselectedContentColor = p.muted,
                    periodSelectorSelectedContainerColor = p.pressedWash,
                    periodSelectorSelectedContentColor = p.text,
                    periodSelectorUnselectedContentColor = p.muted,
                    periodSelectorBorderColor = p.border,
                ),
            )
        },
    )
}
