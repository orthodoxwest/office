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
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.DayOfWeek
import java.time.LocalTime
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle as JavaTextStyle
import java.util.Locale

/** Whether the phone will let reminders through, and on the minute. */
data class ReminderStatus(val notificationsAllowed: Boolean, val exactAllowed: Boolean)

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
) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val is24 = remember { DateFormat.is24HourFormat(context) }
    val clock = remember(is24) { DateTimeFormatter.ofPattern(if (is24) "HH:mm" else "h:mm a", Locale.getDefault()) }
    var editing by remember { mutableStateOf<HourReminder?>(null) }
    val body = Type.body.copy(color = p.text)
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        chrome()
        Column(Modifier.measure().padding(top = 24.dp)) {
            PlainHeadpiece()
            Text("Set prayer reminders", Modifier.padding(top = 10.dp), style = body.copy(fontSize = 26.sp, lineHeight = 32.sp))
            Text(
                "Choose the hours you pray and their times. Each reminder names the office and the feast of the day. " +
                    "Your schedule stays on this phone, and reminders come without a connection.",
                Modifier.padding(top = 10.dp),
                style = body.copy(color = p.muted),
            )
            Fieldset("Hours", Modifier.padding(top = 28.dp)) {
                settings.hours.forEachIndexed { i, h ->
                    if (i > 0) Hairline(p.border)
                    Row(Modifier.fillMaxWidth().heightIn(min = 46.dp), verticalAlignment = Alignment.CenterVertically) {
                        Tick(h.chosen) { onChange(settings.withHour(h.copy(chosen = !h.chosen))) }
                        Text(h.name, Modifier.weight(1f).padding(start = 12.dp).tap { onChange(settings.withHour(h.copy(chosen = !h.chosen))) }, style = body)
                        val label = h.time.format(clock)
                        if (h.chosen) {
                            Text(
                                label,
                                Modifier.border(1.dp, p.border).background(p.bg).tap { editing = h }.padding(horizontal = 10.dp, vertical = 5.dp),
                                style = body.copy(fontFeatureSettings = "lnum"),
                            )
                        } else {
                            Text(label, Modifier.padding(horizontal = 11.dp), style = body.copy(color = p.muted.copy(alpha = 0.6f), fontFeatureSettings = "lnum"))
                        }
                    }
                }
            }
            Fieldset("Days", Modifier.padding(top = 28.dp)) {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(18.dp)) {
                    WEEK.forEach { d ->
                        val on = d in settings.days
                        Row(Modifier.heightIn(min = 44.dp).tap { onChange(settings.copy(days = if (on) settings.days - d else settings.days + d)) }, verticalAlignment = Alignment.CenterVertically) {
                            Tick(on, null)
                            Text(d.getDisplayName(JavaTextStyle.SHORT, Locale.US), Modifier.padding(start = 8.dp), style = body)
                        }
                    }
                }
            }
            Fieldset("Remind me", Modifier.padding(top = 28.dp)) {
                var open by remember { mutableStateOf(false) }
                Box {
                    Row(
                        Modifier.border(1.dp, p.border).background(p.bg).tap { open = true }.padding(horizontal = 12.dp, vertical = 8.dp).width(220.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(REMINDER_LEADS.first { it.first == settings.lead }.second, Modifier.weight(1f), style = body.copy(fontFeatureSettings = "lnum"))
                        Caret(open)
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
                    Note("Notifications are off for the Divine Office, so reminders cannot appear.", "Allow notifications", onAllowNotifications)
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

/** The web's plain headpiece: a short rule either side of the cross, set to the left. */
@Composable
private fun PlainHeadpiece() {
    val o = LocalOrnament.current
    Row(verticalAlignment = Alignment.CenterVertically) {
        Canvas(Modifier.width(22.dp).height(1.dp)) { drawRect(o.line) }
        Text("✠", Modifier.padding(horizontal = 8.dp), style = TextStyle(fontFamily = CrossFont, fontSize = 11.sp, color = o.flat))
        Canvas(Modifier.width(22.dp).height(1.dp)) { drawRect(o.line) }
    }
}

/** A bordered group with its legend set into the top rule, as a web fieldset. */
@Composable
private fun Fieldset(legend: String, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    val p = LocalPalette.current
    Box(modifier.fillMaxWidth()) {
        Column(Modifier.padding(top = 8.dp).fillMaxWidth().border(1.dp, p.border).background(p.surface).padding(horizontal = 14.dp, vertical = 10.dp)) {
            Spacer(Modifier.height(4.dp))
            content()
        }
        Text(
            legend,
            Modifier.padding(start = 12.dp).background(p.surface).padding(horizontal = 6.dp),
            style = Type.label(10.56f, 0.1f).copy(color = p.accent, fontFeatureSettings = ALL_SMALL_CAPS, lineHeight = 16.sp),
        )
    }
}

/** A checkbox in the accent colour, as the web's `accent-color` sets them. */
@Composable
private fun Tick(checked: Boolean, onClick: (() -> Unit)?) {
    val p = LocalPalette.current
    Canvas(Modifier.size(20.dp).then(if (onClick != null) Modifier.tap(onClick) else Modifier).padding(1.dp)) {
        val r = 2.dp.toPx()
        if (checked) {
            drawRoundRect(p.accent, cornerRadius = CornerRadius(r))
            val tick = Path().apply {
                moveTo(size.width * 0.24f, size.height * 0.52f)
                lineTo(size.width * 0.42f, size.height * 0.70f)
                lineTo(size.width * 0.78f, size.height * 0.32f)
            }
            drawPath(tick, p.bg, style = Stroke(2.dp.toPx()))
        } else {
            drawRoundRect(p.muted, cornerRadius = CornerRadius(r), style = Stroke(1.2.dp.toPx()))
        }
    }
}

@Composable
private fun FilledButton(label: String, modifier: Modifier, onClick: () -> Unit) {
    val p = LocalPalette.current
    Box(modifier.fillMaxWidth().heightIn(min = 46.dp).background(p.accent).tap(onClick), contentAlignment = Alignment.Center) {
        Text(label, style = Type.body.copy(fontSize = 17.sp, lineHeight = 24.sp, color = p.bg))
    }
}

@Composable
private fun OutlineButton(label: String, modifier: Modifier, onClick: () -> Unit) {
    val p = LocalPalette.current
    Box(modifier.heightIn(min = 44.dp).border(1.dp, p.border).tap(onClick).padding(horizontal = 16.dp), contentAlignment = Alignment.Center) {
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
        Text(action, Modifier.heightIn(min = 44.dp).tap(onAction).padding(vertical = 10.dp).goldUnderline(true, p.goldLine), style = Type.menu.copy(color = p.accent))
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
