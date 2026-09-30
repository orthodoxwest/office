package org.orthodoxwest.office

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import java.time.LocalDate
import org.orthodoxwest.office.core.BlockKind
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.HourView

/** The Lauds, Prime and Vespers preparation opens, as on the web; the other hours' stays closed. */
private val OPEN_PREPARATION = setOf("lauds", "prime", "vespers")

/** An hour, as the web's hour page: colour band, header, title, controls, the office, its epilogue. */
@Composable
fun HourScreen(
    view: HourView,
    date: LocalDate,
    today: LocalDate,
    hours: List<String>,
    form: String,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onDate: (LocalDate) -> Unit,
    onForm: (String) -> Unit,
    onHour: (String) -> Unit,
    onAllHours: () -> Unit,
) {
    val p = LocalPalette.current
    // Like the web's wake lock: the screen stays on while an hour is open.
    val host = LocalView.current
    DisposableEffect(host) {
        host.keepScreenOn = true
        onDispose { host.keepScreenOn = false }
    }
    val open = remember(view.hour, view.dateLabel) {
        mutableStateMapOf<Int, Boolean>().apply {
            view.sections.forEachIndexed { i, s -> if (s.collapsible) put(i, view.hour in OPEN_PREPARATION) }
        }
    }
    val listState = remember(view.hour, view.dateLabel) { LazyListState() }
    val index = hours.indexOf(view.hour)
    LazyColumn(
        Modifier.fillMaxSize(),
        state = listState,
        contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        item(key = "band") { Box(Modifier.fillMaxWidth().height(3.dp).background(dayColor(view.color))) }
        item(key = "chrome") { chrome() }
        item(key = "title") { HourTitle(view, date, today, form, onDate, onForm) }
        // Each block's space depends on the one before it, across sections.
        var prev: BlockView? = null
        var afterClosed = false
        view.sections.forEachIndexed { i, section ->
            if (section.collapsible) {
                val expanded = open[i] == true
                item(key = "toggle-$i") {
                    Row(
                        Modifier.measure().padding(top = 5.6.dp, bottom = if (expanded) 12.8.dp else 0.dp).heightIn(min = 44.dp).tap { open[i] = !expanded },
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(section.label, style = Type.heading.copy(color = p.text))
                        Caret(expanded)
                    }
                }
                prev = null
                afterClosed = !expanded
                if (!expanded) return@forEachIndexed
            }
            section.blocks.forEachIndexed { j, block ->
                val heading = block.kind == BlockKind.HEADING
                val gap = when {
                    prev != null -> gapBefore(prev, block)
                    afterClosed -> if (heading) 23.2.dp else 14.dp
                    else -> if (heading) 12.dp else 0.dp
                }
                afterClosed = false
                item(key = "$i-$j") { Block(block, Modifier.measure().padding(top = gap)) }
                prev = block
            }
        }
        item(key = "epilogue") {
            Epilogue(
                previous = hours.getOrNull(index - 1),
                next = hours.getOrNull(index + 1),
                reportUrl = view.reportUrl,
                onHour = onHour,
                onAllHours = onAllHours,
            )
        }
    }
}

@Composable
private fun HourTitle(view: HourView, date: LocalDate, today: LocalDate, form: String, onDate: (LocalDate) -> Unit, onForm: (String) -> Unit) {
    val p = LocalPalette.current
    var picking by remember { mutableStateOf(false) }
    var choosing by remember { mutableStateOf(false) }
    Column(Modifier.measure().padding(top = 17.6.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        // The headpiece is set into the title's upper rule; the day's colour reaches the lower one's lozenge.
        Box(contentAlignment = Alignment.Center) {
            DoubleRule(gap = 132.8.dp)
            Headpiece()
        }
        Text(view.title.uppercase(), style = Type.hourTitle.copy(color = p.text))
        DoubleRule(lozenge = dayColor(view.color))
        Text(
            listOf(view.dateLabel, view.feast, view.seasonLabel).filter { it.isNotEmpty() }.joinToString(" · "),
            Modifier.padding(top = 2.dp),
            style = Type.meta.copy(color = p.muted),
        )
        if (date != today) {
            Text(
                "GO TO TODAY",
                Modifier.padding(top = 5.6.dp).heightIn(min = 44.dp).tap { onDate(today) }.padding(vertical = 12.dp).goldUnderline(true, p.goldLine),
                style = Type.menu.copy(color = p.accent),
            )
        }
        Row(Modifier.padding(top = 4.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Disclosure("Change date", picking, { picking = !picking; choosing = false })
            Disclosure("Prayer form:", choosing, { choosing = !choosing; picking = false }, value = PRAYER_FORMS.first { it.first == form }.second)
        }
        if (picking) DatePicker(date, today) { picking = false; onDate(it) }
        if (choosing) FormChooser(form) { choosing = false; onForm(it) }
        Spacer(Modifier.height(7.2.dp))
        Hairline(p.border)
    }
}

/** After the prayer: the other hours, the report link, and the foot. In Apse the vault fades in here. */
@Composable
private fun Epilogue(previous: String?, next: String?, reportUrl: String, onHour: (String) -> Unit, onAllHours: () -> Unit) {
    val p = LocalPalette.current
    val context = LocalContext.current
    Box(Modifier.fillMaxWidth()) {
        VaultField(Modifier.matchParentSize(), listOf(0f to 0f, 0.35f to 0f, 0.7f to 0.8f, 1f to 1f))
        Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
            Continuation(
                previousLabel = "Previous hour",
                previous = previous?.let(::hourLabel),
                onPrevious = { previous?.let(onHour) },
                middle = "All hours",
                onMiddle = onAllHours,
                nextLabel = "Next hour",
                next = next?.let(::hourLabel),
                onNext = { next?.let(onHour) },
                modifier = Modifier.measure().padding(top = 44.dp),
            )
            Text(
                buildAnnotatedString {
                    append("Spotted an error on this page? ")
                    withStyle(SpanStyle(color = p.accent, textDecoration = TextDecoration.Underline)) { append("Report a problem") }
                },
                Modifier.measure().padding(top = 40.dp).tap { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(reportUrl))) },
                style = Type.small.copy(color = p.muted, textAlign = TextAlign.Center),
            )
            Footer(diamond = !p.dark)
        }
    }
}
