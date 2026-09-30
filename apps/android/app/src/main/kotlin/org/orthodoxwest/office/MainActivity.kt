package org.orthodoxwest.office

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.PrimaryScrollableTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import androidx.lifecycle.viewmodel.compose.viewModel
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.HourView

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        setContent {
            OfficeTheme {
                val vm: OfficeViewModel = viewModel()
                OfficeScreen(
                    date = vm.date,
                    hour = vm.hour,
                    hours = vm.hours,
                    form = vm.form,
                    view = vm.view,
                    error = vm.error,
                    onShow = vm::show,
                    onForm = vm::chooseForm,
                    onNow = vm::goToNow,
                )
            }
        }
    }
}

private val BAR_DATE: DateTimeFormatter = DateTimeFormatter.ofPattern("EEE d MMM yyyy")

/** Capitalizes an hour's name for its tab: "lauds" → "Lauds". */
fun hourLabel(hour: String): String = hour.replaceFirstChar { it.titlecase() }

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun OfficeScreen(
    date: LocalDate,
    hour: String,
    hours: List<String>,
    form: String,
    view: HourView?,
    error: String?,
    onShow: (LocalDate, String) -> Unit,
    onForm: (String) -> Unit,
    onNow: () -> Unit,
) {
    val p = LocalPalette.current
    var picking by remember { mutableStateOf(false) }
    Scaffold(
        containerColor = p.page,
        topBar = {
            Column(Modifier.background(p.page).statusBarsPadding()) {
                DateBar(date, form, onDay = { onShow(date.plusDays(it), hour) }, onPick = { picking = true }, onForm = onForm, onNow = onNow)
                PrimaryScrollableTabRow(
                    selectedTabIndex = hours.indexOf(hour).coerceAtLeast(0),
                    containerColor = p.page,
                    contentColor = p.rubric,
                    edgePadding = 8.dp,
                    divider = { HorizontalDivider(color = p.border) },
                ) {
                    hours.forEach { h ->
                        Tab(
                            selected = h == hour,
                            onClick = { onShow(date, h) },
                            text = { Text(hourLabel(h), style = MaterialTheme.typography.labelLarge.copy(letterSpacing = 0.03.em)) },
                            selectedContentColor = p.rubric,
                            unselectedContentColor = p.muted,
                        )
                    }
                }
            }
        },
    ) { padding ->
        when {
            error != null -> Message(error, padding)
            view == null -> Message("Preparing the office…", padding)
            else -> HourPage(view, hours, padding, onHour = { onShow(date, it) })
        }
    }
    if (picking) {
        val state = rememberDatePickerState(initialSelectedDateMillis = date.atStartOfDay().toInstant(ZoneOffset.UTC).toEpochMilli())
        DatePickerDialog(
            onDismissRequest = { picking = false },
            confirmButton = {
                TextButton(onClick = {
                    picking = false
                    state.selectedDateMillis?.let { onShow(Instant.ofEpochMilli(it).atZone(ZoneOffset.UTC).toLocalDate(), hour) }
                }) { Text("Go") }
            },
            dismissButton = { TextButton(onClick = { picking = false }) { Text("Cancel") } },
        ) { DatePicker(state) }
    }
}

@Composable
private fun DateBar(date: LocalDate, form: String, onDay: (Long) -> Unit, onPick: () -> Unit, onForm: (String) -> Unit, onNow: () -> Unit) {
    val p = LocalPalette.current
    var menu by remember { mutableStateOf(false) }
    Row(Modifier.fillMaxWidth().height(52.dp).padding(horizontal = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        TextButton(onClick = { onDay(-1) }) { Text("‹", fontSize = 28.sp, color = p.accent) }
        Text(
            date.format(BAR_DATE),
            Modifier.weight(1f).clickable(onClick = onPick).padding(vertical = 8.dp),
            style = MaterialTheme.typography.titleMedium.copy(textAlign = TextAlign.Center),
        )
        TextButton(onClick = { onDay(1) }) { Text("›", fontSize = 28.sp, color = p.accent) }
        Box {
            TextButton(onClick = { menu = true }) { Text("⋮", fontSize = 22.sp, color = p.accent) }
            DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                DropdownMenuItem(text = { Text("Now") }, onClick = { menu = false; onNow() })
                HorizontalDivider()
                PRAYER_FORMS.forEach { (value, label) ->
                    DropdownMenuItem(
                        text = { Text(if (value == form) "✓ $label" else "   $label") },
                        onClick = { menu = false; onForm(value) },
                    )
                }
            }
        }
    }
}

@Composable
private fun Message(text: String, padding: PaddingValues) {
    Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) {
        Text(text, color = LocalPalette.current.muted, textAlign = TextAlign.Center, modifier = Modifier.padding(24.dp))
    }
}

/** The composed hour: its heading, then each section's blocks, collapsible sections closed. */
@Composable
fun HourPage(view: HourView, hours: List<String>, padding: PaddingValues, onHour: (String) -> Unit) {
    val p = LocalPalette.current
    val open = remember(view) { mutableStateMapOf<Int, Boolean>() }
    val listState = remember(view.hour, view.dateLabel) { LazyListState() }
    val column = Modifier.widthIn(max = 620.dp).fillMaxWidth().padding(horizontal = 20.dp)
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = padding.calculateTopPadding() + 12.dp, bottom = padding.calculateBottomPadding() + 40.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "header") { HourHeader(view, column) }
        view.sections.forEachIndexed { i, section ->
            if (section.collapsible) {
                item(key = "toggle-$i") {
                    val expanded = open[i] == true
                    Text(
                        (if (expanded) "▾  " else "▸  ") + section.label,
                        column.clickable { open[i] = !expanded }.padding(vertical = 10.dp),
                        style = MaterialTheme.typography.bodyMedium.copy(color = p.accent, fontStyle = FontStyle.Italic),
                    )
                }
                if (open[i] == true) blocks(i, section.blocks, column)
            } else {
                blocks(i, section.blocks, column)
            }
        }
        item(key = "next") { NextHour(view.hour, hours, column, onHour) }
    }
}

private fun LazyListScope.blocks(section: Int, blocks: List<BlockView>, modifier: Modifier) {
    items(blocks.size, key = { "$section-$it" }) { Block(blocks[it], modifier) }
}

@Composable
private fun HourHeader(view: HourView, modifier: Modifier) {
    val p = LocalPalette.current
    val type = MaterialTheme.typography
    Column(modifier.padding(bottom = 12.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            view.title,
            style = type.bodyLarge.copy(fontSize = 34.sp, lineHeight = 40.sp, color = p.rubric, fontFeatureSettings = "smcp, c2sc", letterSpacing = 0.04.em),
        )
        Spacer(Modifier.height(4.dp))
        Text(view.dateLabel, style = type.bodyMedium.copy(color = p.muted))
        if (view.feast.isNotEmpty()) {
            Spacer(Modifier.height(6.dp))
            Text(view.feast, style = type.titleMedium.copy(fontStyle = FontStyle.Italic, textAlign = TextAlign.Center))
        }
        val color = liturgicalColor(view.color)
        if (color != null || view.season.isNotEmpty()) {
            Spacer(Modifier.height(8.dp))
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                if (color != null) Box(Modifier.size(10.dp).clip(CircleShape).background(color))
                val season = view.season.replaceFirstChar { it.titlecase() }
                Text(listOf(season, view.color).filter { it.isNotEmpty() }.joinToString(" · "), style = type.labelMedium.copy(color = p.muted))
            }
        }
        Spacer(Modifier.height(10.dp))
        HorizontalDivider(Modifier.widthIn(max = 120.dp), color = p.gold.copy(alpha = 0.5f))
    }
}

@Composable
private fun NextHour(hour: String, hours: List<String>, modifier: Modifier, onHour: (String) -> Unit) {
    val next = hours.getOrNull(hours.indexOf(hour) + 1) ?: return
    Box(modifier.padding(top = 28.dp), contentAlignment = Alignment.Center) {
        TextButton(onClick = { onHour(next) }) {
            Text("${hourLabel(next)} ›", style = MaterialTheme.typography.titleMedium.copy(color = LocalPalette.current.accent))
        }
    }
}
