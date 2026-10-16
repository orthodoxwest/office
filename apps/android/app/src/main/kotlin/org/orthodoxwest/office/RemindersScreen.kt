package org.orthodoxwest.office

import android.text.format.DateFormat
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
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
import androidx.compose.material3.TextButton
import androidx.compose.material3.TimePicker
import androidx.compose.material3.TimePickerDefaults
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.isTraversalGroup
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.traversalIndex
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle as JavaTextStyle
import java.util.Locale
import org.orthodoxwest.office.core.sunUnavailableNote

/** Whether the phone will let reminders through, and on the minute, and tell the app where it is. */
data class ReminderStatus(val notificationsAllowed: Boolean, val exactAllowed: Boolean, val locationAllowed: Boolean = true)

/** Monday first, as the web's Days row. */
private val WEEK = DayOfWeek.entries.toList()

/**
 * The reminders page, set as the web's: the hours with their times, the days, how long before;
 * then, where the web gives a calendar link, the switch that asks the phone itself.
 */
@OptIn(ExperimentalLayoutApi::class)
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
                settings.hours.forEachIndexed { i, h ->
                    if (i > 0) Hairline(p.border)
                    Row(Modifier.fillMaxWidth().heightIn(min = 46.dp), verticalAlignment = Alignment.CenterVertically) {
                        // The box and the hour's name are one checkbox; its time is a control of its own.
                        Row(
                            Modifier.weight(1f).heightIn(min = 46.dp).check(h.chosen) {
                                haptics.toggled(it)
                                onChange(settings.withHour(h.copy(chosen = it)))
                            },
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Tick(h.chosen)
                            // Muted until chosen, as the web's `.reminder-hour-row.is-selected`.
                            Text(h.name, Modifier.padding(start = 12.dp), style = body.copy(color = if (h.chosen) p.text else p.muted))
                        }
                        val bySun = sun?.get(h.hour)?.takeIf { h.hour != "compline" }
                        val label = (bySun ?: h.time).format(clock)
                        if (bySun != null) {
                            SunTime(label, h, h.chosen)
                        } else if (h.chosen) {
                            Text(
                                label,
                                Modifier.border(1.dp, p.border).background(p.bg).tap(label = "${h.name} at $label", action = "change the time") { editing = h }
                                    .padding(horizontal = 10.dp, vertical = 5.dp),
                                style = body.copy(fontFeatureSettings = "lnum"),
                            )
                        } else {
                            Text(label, Modifier.padding(horizontal = 11.dp), style = body.copy(color = p.muted.copy(alpha = 0.6f), fontFeatureSettings = "lnum"))
                        }
                    }
                    if (sun != null && h.chosen && h.hour == "compline") ComplineNote(sun[h.hour]?.takeIf { it != h.time }?.format(clock))
                }
            }
            Group("Days", Modifier.padding(top = 28.dp)) {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(18.dp)) {
                    WEEK.forEach { d ->
                        val on = d in settings.days
                        Row(
                            Modifier.heightIn(min = 44.dp).check(on) {
                                haptics.toggled(it)
                                onChange(settings.copy(days = if (it) settings.days + d else settings.days - d))
                            }
                                .semantics { contentDescription = d.getDisplayName(JavaTextStyle.FULL, Locale.US) },
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Tick(on)
                            Text(d.getDisplayName(JavaTextStyle.SHORT, Locale.US), Modifier.padding(start = 8.dp), style = body)
                        }
                    }
                }
            }
            Group("Remind me", Modifier.padding(top = 28.dp)) {
                var open by remember { mutableStateOf(false) }
                Box {
                    Row(
                        Modifier.border(1.dp, p.border).background(p.bg).tap(action = "choose when") { open = true }.disclosed(open)
                            .padding(horizontal = 12.dp, vertical = 8.dp).width(220.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(REMINDER_LEADS.first { it.first == settings.lead }.second, Modifier.weight(1f), style = body.copy(fontFeatureSettings = "lnum"))
                        Caret(open, p.text)
                    }
                    DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
                        REMINDER_LEADS.forEach { (minutes, label) ->
                            DropdownMenuItem(text = { Text(label, style = body.copy(fontFeatureSettings = "lnum")) }, onClick = { open = false; onChange(settings.copy(lead = minutes)) })
                        }
                    }
                }
            }
            DoubleRule(Modifier.padding(top = 28.dp))
            Text("On this phone", Modifier.padding(top = 16.dp), style = body.copy(fontSize = 22.sp, lineHeight = 28.sp))
            if (settings.on) {
                Text(
                    if (settings.hours.none { it.chosen } || settings.days.isEmpty()) "Reminders are on, but no hour or day is chosen." else "Reminders are on.",
                    Modifier.padding(top = 8.dp),
                    style = body.copy(color = p.muted),
                )
                OutlineButton("Turn off reminders", Modifier.padding(top = 12.dp), onTurnOff)
                if (!status.notificationsAllowed) {
                    Note("Notifications are off for the Daily Office, so reminders cannot appear.", "Allow notifications", onAllowNotifications)
                }
                if (!status.exactAllowed) {
                    Note("Reminders may come a few minutes late. For the exact minute, allow alarms and reminders.", "Allow alarms", onAllowExact)
                }
            } else {
                FilledButton("Turn on reminders", Modifier.padding(top = 12.dp), onTurnOn)
            }
        }
        Footer()
    }
    editing?.let { h -> TimeDialog(h.time, is24, onDismiss = { editing = null }) { t -> editing = null; onChange(settings.withHour(h.copy(time = t))) } }
}

private fun ReminderSettings.withHour(h: HourReminder) = copy(hours = hours.map { if (it.hour == h.hour) h else it })

/** What marks each hour's time by the sun, under the time. */
private val SUN_MARKS = mapOf(
    "lauds" to "sunrise",
    "prime" to "first hour",
    "terce" to "third hour",
    "sext" to "noon by the sun",
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
        } else if (settings.followSun) {
            Text(
                "Lauds at sunrise, Vespers at sunset, and the little hours at the first, third, sixth and ninth hours of daylight. " +
                    "Times for where you were when you last opened the app.",
                Modifier.padding(top = 2.dp, bottom = 10.dp),
                style = Type.body.copy(fontSize = 16.sp, lineHeight = 23.sp, color = p.muted),
            )
        }
        if (settings.followSun && !status.locationAllowed) {
            Note("Allow location to time reminders by the sun.", "Open settings", onAllowLocation)
            Spacer(Modifier.height(6.dp))
        }
        Hairline(p.border)
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

/** The web's headpiece, set to the left: a painted cross between two short rules. */
@Composable
private fun PlainHeadpiece() {
    val ink = LocalPalette.current.lining
    val rule = ink.copy(alpha = 0.55f)
    Row(verticalAlignment = Alignment.CenterVertically) {
        Canvas(Modifier.width(22.dp).height(1.dp)) { drawRect(rule) }
        Text("✠", Modifier.padding(horizontal = 8.dp), style = TextStyle(fontFamily = CrossFont, fontSize = 11.sp, color = ink))
        Canvas(Modifier.width(22.dp).height(1.dp)) { drawRect(rule) }
    }
}

/**
 * A group of the page's controls under its name, set as home's "Pray the hours" band: the name
 * in small caps between gilt lozenges, a gold fillet above and below, and the rows on the page
 * itself beneath. A screen reader takes the name as the group's heading.
 */
@Composable
private fun Group(name: String, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    val p = LocalPalette.current
    // Home's band is gilt on its green earth; on the bare page the gold reads where the gilt would fade.
    Column(modifier.fillMaxWidth().semantics { isTraversalGroup = true }) {
        Lozenged(
            name,
            Type.label(11f, 0.16f).copy(color = p.gold, fontFeatureSettings = ALL_SMALL_CAPS),
            p.gold,
            Modifier.fillMaxWidth()
                .drawBehind {
                    val w = 1.dp.toPx()
                    drawLine(p.goldLine, Offset(0f, w / 2), Offset(size.width, w / 2), w)
                    drawLine(p.goldLine, Offset(0f, size.height - w / 2), Offset(size.width, size.height - w / 2), w)
                }
                .padding(vertical = 7.dp)
                .semantics { heading(); traversalIndex = -1f },
        )
        Spacer(Modifier.height(6.dp))
        content()
    }
}

/**
 * A checkbox set by hand, as the web's are: a ruled square, 0.95rem, that fills with oak inside a
 * margin of the ground when chosen. The platform's own box belongs to no theme here. The row
 * around it is the control.
 */
@Composable
private fun Tick(checked: Boolean) {
    val p = LocalPalette.current
    Canvas(Modifier.size(20.dp)) {
        val side = 15.2.dp.toPx()
        val corner = Offset((size.width - side) / 2, (size.height - side) / 2)
        val box = Size(side, side)
        val rule = 1.dp.toPx()
        drawRect(p.bg, corner, box)
        if (checked) {
            val inset = rule + 3.dp.toPx()
            drawRect(p.accent, corner + Offset(inset, inset), Size(side - 2 * inset, side - 2 * inset))
        }
        drawRoundRect(
            if (checked) p.accent else p.muted,
            corner + Offset(rule / 2, rule / 2),
            Size(side - rule, side - rule),
            CornerRadius(1.dp.toPx()),
            style = Stroke(rule),
        )
    }
}

@Composable
private fun FilledButton(label: String, modifier: Modifier, onClick: () -> Unit) {
    val p = LocalPalette.current
    Box(modifier.fillMaxWidth().heightIn(min = 46.dp).background(p.accent).tap(onClick = onClick), contentAlignment = Alignment.Center) {
        Text(label, style = Type.body.copy(fontSize = 17.sp, lineHeight = 24.sp, color = p.bg))
    }
}

@Composable
private fun OutlineButton(label: String, modifier: Modifier, onClick: () -> Unit) {
    val p = LocalPalette.current
    Box(modifier.heightIn(min = 44.dp).border(1.dp, p.border).tap(onClick = onClick).padding(horizontal = 16.dp), contentAlignment = Alignment.Center) {
        Text(label, style = Type.body.copy(fontSize = 16.sp, lineHeight = 22.sp, color = p.accent))
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

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun TimeDialog(initial: LocalTime, is24: Boolean, onDismiss: () -> Unit, onPick: (LocalTime) -> Unit) {
    val p = LocalPalette.current
    val state = rememberTimePickerState(initial.hour, initial.minute, is24)
    AlertDialog(
        onDismissRequest = onDismiss,
        containerColor = p.surface,
        confirmButton = { TextButton(onClick = { onPick(LocalTime.of(state.hour, state.minute)) }) { Text("Set", color = p.accent) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel", color = p.muted) } },
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
