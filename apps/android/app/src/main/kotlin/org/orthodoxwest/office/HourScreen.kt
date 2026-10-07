package org.orthodoxwest.office

import android.content.Intent
import android.net.Uri
import android.view.View
import androidx.compose.animation.core.FastOutLinearInEasing
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.VisibilityThreshold
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.LinearOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListItemInfo
import androidx.compose.foundation.lazy.LazyListLayoutInfo
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.runtime.snapshots.SnapshotStateMap
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.semantics.ProgressBarRangeInfo
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.progressBarRangeInfo
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import java.time.LocalDate
import java.util.WeakHashMap
import org.orthodoxwest.office.core.BlockKind
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.HourView

/** How many hours hold each window awake. */
private val hoursAwake = WeakHashMap<View, Int>()

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
    // Counted, as one hour can still be leaving while the next comes in.
    DisposableEffect(host) {
        hoursAwake[host] = (hoursAwake[host] ?: 0) + 1
        host.keepScreenOn = true
        onDispose {
            val left = (hoursAwake[host] ?: 1) - 1
            hoursAwake[host] = left
            if (left == 0) host.keepScreenOn = false
        }
    }
    // Saved with the scroll position: an opened section moves everything below it.
    val open = rememberSaveable(view.hour, view.dateLabel, saver = OpenSections) {
        mutableStateMapOf<Int, Boolean>().apply {
            view.sections.forEachIndexed { i, s -> if (s.collapsible) put(i, view.hour in OPEN_PREPARATION) }
        }
    }
    val listState = rememberSaveable(view.hour, view.dateLabel, saver = LazyListState.Saver) { LazyListState() }
    // The office below a section moves to make room only while the section opens or closes,
    // not when a new text size or prayer form sets it again.
    var unfolding by remember { mutableStateOf(false) }
    LaunchedEffect(unfolding) {
        if (unfolding) {
            delay(600)
            unfolding = false
        }
    }
    val placement = if (unfolding) spring(stiffness = Spring.StiffnessMediumLow, visibilityThreshold = IntOffset.VisibilityThreshold) else null
    val columns = hymnColumns(view.sections)
    val index = hours.indexOf(view.hour)
    BoxWithConstraints(Modifier.fillMaxSize()) {
        // A first-class day's frame takes a little of a phone's measure (the web's 12px side
        // columns under 920px); where the lines stand outside the measure, the text keeps it.
        val inset = if (view.firstClass && maxWidth < FrameClear) FrameInset else 0.dp
        LazyColumn(
            Modifier.fillMaxSize().firstClassFrame(view.firstClass, listState, p.lining),
            state = listState,
            contentPadding = PaddingValues(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            item(key = "band") { Box(Modifier.fillMaxWidth().height(3.dp).background(dayColor(view.color))) }
            item(key = "chrome") { chrome() }
            item(key = "title") { Box(Modifier.padding(horizontal = inset)) { HourTitle(view, date, today, form, onDate, onForm) } }
            // Each block's space depends on the one before it, across sections.
            var prev: BlockView? = null
            var afterClosed = false
            // Whether anything of the office stands above: its first part takes no cross.
            var begun = false
            view.sections.forEachIndexed { i, section ->
                if (section.collapsible) {
                    val expanded = open[i] == true
                    item(key = "toggle-$i") {
                        Row(
                            Modifier.animateItem(placementSpec = placement).measure().padding(horizontal = inset).padding(top = 5.6.dp, bottom = if (expanded) 12.8.dp else 0.dp).heightIn(min = 44.dp)
                                .semantics { heading() }.tap { unfolding = true; open[i] = !expanded }.disclosed(expanded),
                            horizontalArrangement = Arrangement.Center,
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(section.label, style = Type.heading.copy(color = p.titulus))
                            Caret(expanded, p.titulus)
                        }
                    }
                    prev = null
                    afterClosed = !expanded
                    begun = true
                    if (!expanded) return@forEachIndexed
                }
                section.blocks.forEachIndexed { j, block ->
                    val heading = block.kind == BlockKind.HEADING
                    val gap = when {
                        prev != null -> gapBefore(prev, block)
                        afterClosed -> if (heading) 23.2.dp else 14.dp
                        else -> if (heading) 12.dp else 0.dp
                    }
                    // A small painted cross before each of the office's parts after the first, as the
                    // web's `.elements > .section-heading`: a section's heading, or a heading standing
                    // as an element of its own, the hymn's and the chapter's included; none in the
                    // preparation.
                    val part = (heading || block.kind == BlockKind.COMMEMORATION_HEADING) && !section.collapsible
                    val cross = part && begun
                    begun = true
                    afterClosed = false
                    item(key = "$i-$j") { Block(block, Modifier.animateItem(fadeInSpec = UNFOLD_FADE, placementSpec = placement, fadeOutSpec = FOLD_FADE).measure().padding(horizontal = inset).padding(top = gap), column = columns[i to j], cross = cross) }
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
        ProgressHairline(listState)
    }
}

/**
 * The frame of a first-class day's hour, as the web's `.rank-first-class`: a thin double line in
 * the lining down each side, from the hour's title to 20dp above the epilogue, where the
 * consecration cross closes the hour. On a phone the lines stand 5dp from the screen's edges and
 * the text gives them [FrameInset]; on a wide screen they stand 40dp outside the measure, as the
 * web's from 920px. Drawn from the list's layout, so the frame scrolls with the page and runs
 * the whole screen while the title is above it and the epilogue below.
 */
private fun Modifier.firstClassFrame(on: Boolean, state: LazyListState, lining: Color): Modifier {
    if (!on) return this
    return drawBehind {
        val info = state.layoutInfo
        val items = info.visibleItemsInfo
        if (items.isEmpty()) return@drawBehind
        fun y(item: LazyListItemInfo) = (item.offset - info.viewportStartOffset).toFloat()
        val title = items.firstOrNull { it.key == "title" }
        val top = when {
            title != null -> y(title)
            // The title has scrolled past: the frame runs in from the top of the screen.
            items.first().key.let { it != "band" && it != "chrome" } -> 0f
            else -> return@drawBehind
        }
        val epilogue = items.firstOrNull { it.key == "epilogue" }
        val bottom = if (epilogue != null) y(epilogue) - 20.dp.toPx() else size.height
        if (bottom <= top) return@drawBehind
        val side = if (size.width < FrameClear.toPx()) 5.dp.toPx() else (size.width - Measure.toPx()) / 2f - 24.dp.toPx()
        val px = 1.dp.toPx()
        val ink = lining.copy(alpha = 0.6f)
        for (x in listOf(side, size.width - side - 3 * px)) {
            drawRect(ink, Offset(x, top), Size(px, bottom - top))
            drawRect(ink, Offset(x + 2 * px, top), Size(px, bottom - top))
        }
    }
}

/** What the office's text gives a first-class frame on a phone, each side (the web's 12px side columns). */
private val FrameInset = 12.dp

/** The width from which a first-class frame stands clear of the measure: room for 40dp outside the text each side. */
private val FrameClear = Measure + 80.dp

/** The items above the office: the colour band, the header and the title. */
private const val BEFORE_OFFICE = 3

/**
 * The gold hairline across the top, as the web's `.hour-scroll-progress`: how far through the
 * prayer itself. It stays empty through the band, header and title, starts as the office reaches
 * the top of the screen, and is full once the office's end reaches the bottom; the hour's ending
 * keeps it full. The band scrolls away with the page here, so the line runs along the screen's top
 * edge, above the status bar's icons: just below them it would strike through the text passing
 * under the bar. Rows not yet laid out are taken at the average height of those that have been.
 */
@Composable
private fun ProgressHairline(list: LazyListState, modifier: Modifier = Modifier) {
    val color = LocalOrnament.current.flat
    // Each row's height as last laid out, by index; begun again when the rows change in number
    // (a section opened or closed) or in width.
    val heights = remember { HashMap<Int, Int>() }
    val shape = remember { IntArray(2) }
    var ratio by remember { mutableFloatStateOf(0f) }
    LaunchedEffect(list) {
        snapshotFlow { list.layoutInfo }.collect { info ->
            if (info.totalItemsCount != shape[0] || info.viewportSize.width != shape[1]) {
                heights.clear()
                shape[0] = info.totalItemsCount
                shape[1] = info.viewportSize.width
            }
            info.visibleItemsInfo.forEach { heights[it.index] = it.size }
            ratio = officeRead(info, heights)
        }
    }
    Box(
        modifier.fillMaxWidth().height(2.dp)
            .semantics {
                progressBarRangeInfo = ProgressBarRangeInfo(ratio, 0f..1f)
                contentDescription = "Progress through the prayer text"
            }
            .drawBehind { drawRect(color, size = size.copy(width = size.width * ratio)) },
    )
}

/** How much of the office has passed the screen, from 0 to 1. */
private fun officeRead(info: LazyListLayoutInfo, heights: Map<Int, Int>): Float {
    val start = BEFORE_OFFICE
    // The hour's ending, the last row.
    val end = info.totalItemsCount - 1
    if (end <= start) return 0f
    val items = info.visibleItemsInfo
    // Offsets run from the top of the screen below the status bar.
    val bottom = info.viewportEndOffset - info.afterContentPadding
    val ending = items.firstOrNull { it.index == end }
    if (ending != null && ending.offset <= bottom) return 1f
    val top = items.lastOrNull { it.offset <= 0 } ?: return 0f
    if (top.index < start) return 0f
    val known = (start until end).mapNotNull { heights[it] }
    val average = if (known.isEmpty()) 1f else known.average().toFloat()
    fun height(i: Int) = heights[i]?.toFloat() ?: average
    val passed = (start until top.index).sumOf { height(it).toDouble() }.toFloat() - top.offset
    // Once the ending is in sight its distance is known exactly.
    val left = if (ending != null) {
        (ending.offset - bottom).toFloat()
    } else {
        (start until end).sumOf { height(it).toDouble() }.toFloat() - bottom - passed
    }
    return if (passed + left <= 0f) 1f else (passed / (passed + left)).coerceIn(0f, 1f)
}

@Composable
private fun HourTitle(view: HourView, date: LocalDate, today: LocalDate, form: String, onDate: (LocalDate) -> Unit, onForm: (String) -> Unit) {
    val p = LocalPalette.current
    var picking by remember { mutableStateOf(false) }
    var choosing by remember { mutableStateOf(false) }
    Column(Modifier.measure().padding(top = 17.6.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        // Painted double rules frame the title. The hour's sign is set alone into the upper rule,
        // broken only for it; the day's colour reaches the lower one's lozenge. The rules take
        // the title's width, at least 24rem (the measure, on a phone), as the web's h1.
        Column(Modifier.widthIn(min = 384.dp).width(IntrinsicSize.Max), horizontalAlignment = Alignment.CenterHorizontally) {
            // One height whatever the sign (the web's 1.05rem headpiece), so the title stands at
            // the same place on every hour.
            Box(Modifier.fillMaxWidth().height(16.8.dp), contentAlignment = Alignment.Center) {
                DoubleRule(gap = 40.dp, color = p.lining, heavy = true)
                HourSign(view.hour)
            }
            Text(
                view.title.uppercase(),
                Modifier.padding(horizontal = 20.8.dp).semantics { heading(); contentDescription = view.title },
                style = Type.hourTitle.copy(color = p.text),
            )
            DoubleRule(lozenge = dayColor(view.color), color = p.lining)
        }
        val meta = listOf(view.dateLabel, view.feast, view.seasonLabel).filter { it.isNotEmpty() }
        // On a phone the date stands on its own line above the day's name, so a line never ends on
        // the separator (the web's `.hour-meta-part:first-child` to 700px); wide screens keep one line.
        val metaLines = if (LocalWide.current || meta.size < 2) listOf(meta.joinToString(" · ")) else listOf(meta[0], meta.drop(1).joinToString(" · "))
        Column(Modifier.padding(top = 2.dp).semantics(mergeDescendants = true) { contentDescription = meta.joinToString(". ") }, horizontalAlignment = Alignment.CenterHorizontally) {
            for (line in metaLines) Text(line, style = Type.meta.copy(color = p.muted, textAlign = TextAlign.Center))
        }
        if (date != today) {
            Text(
                "GO TO TODAY",
                Modifier.padding(top = 5.6.dp).heightIn(min = 44.dp).tap { onDate(today) }.padding(vertical = 12.dp).goldUnderline(true, p.goldLine),
                style = Type.menu.copy(color = p.accent),
            )
        }
        // Side by side, or one above the other when the reader's font size leaves no room.
        FlowRow(Modifier.padding(top = 4.dp), horizontalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterHorizontally)) {
            Disclosure("Change date", picking, { picking = !picking; choosing = false })
            Disclosure("Prayer form:", choosing, { choosing = !choosing; picking = false }, value = PRAYER_FORMS.first { it.first == form }.second)
        }
        Unfold(picking) { DatePicker(date, today) { picking = false; onDate(it) } }
        Unfold(choosing) { FormChooser(form) { choosing = false; onForm(it) } }
        Spacer(Modifier.height(7.2.dp))
        Hairline(p.lining.copy(alpha = 0.3f))
    }
}

/**
 * After the prayer: the consecration cross that ends the hour, the other hours, and the foot, with
 * the report line under its colophon. The wall's field shows only below the hour navigation: the
 * Apse vault by night, the Nave's powdering by day.
 */
@Composable
private fun Epilogue(previous: String?, next: String?, reportUrl: String, onHour: (String) -> Unit, onAllHours: () -> Unit) {
    val p = LocalPalette.current
    val context = LocalContext.current
    Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.fillMaxWidth()) {
        // The hour ends on one mark, as painted where the bishop anointed the walls; the links
        // and the foot below carry no other.
        ConsecrationCross(Modifier.padding(top = 26.4.dp).size(40.dp))
        Continuation(
            previousLabel = "Previous hour",
            previous = previous?.let(::hourLabel),
            onPrevious = { previous?.let(onHour) },
            middle = "All hours",
            onMiddle = onAllHours,
            nextLabel = "Next hour",
            next = next?.let(::hourLabel),
            onNext = { next?.let(onHour) },
            modifier = Modifier.measure().padding(top = 83.2.dp),
        )
        Box(Modifier.fillMaxWidth()) {
            // The field is phased from the seam where the ending's air (3.25rem) meets the
            // footer. By night it fades in over the first rem and thins down the footer.
            WallField(Modifier.matchParentSize(), seam = EndingAir) { dark ->
                val seam = EndingAir.toPx() / size.height
                if (dark) {
                    listOf(0f to 0f, 16.dp.toPx() / size.height to 1f, seam + 0.45f * (1f - seam) to 1f, seam + 0.8f * (1f - seam) to 0.5f, 1f to 0.25f)
                } else {
                    listOf(0f to 1f, 1f to 1f)
                }
            }
            Footer(Modifier.padding(top = EndingAir), reserve = true) {
                Text(
                    buildAnnotatedString {
                        append("Spotted an error on this page? ")
                        withStyle(SpanStyle(color = p.accent, textDecoration = TextDecoration.Underline)) { append("Report a problem") }
                    },
                    Modifier.measure().padding(top = 4.8.dp).reserve(p.bg).tap(action = "report a problem") { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(reportUrl))) },
                    style = Type.small.copy(fontSize = 11.84.sp, lineHeight = 18.9.sp, color = p.muted, textAlign = TextAlign.Center),
                )
            }
        }
    }
}

/** Air below the hour navigation (`.hour-epilogue`'s bottom padding), where the wall's field begins. */
private val EndingAir = 52.dp

/** A section's blocks fade in as it opens, the office below moving down to make room, and out as it closes. */
private val UNFOLD_FADE = tween<Float>(220, delayMillis = 60, easing = LinearOutSlowInEasing)
private val FOLD_FADE = tween<Float>(120, easing = FastOutLinearInEasing)

/** Which collapsible sections are open, as saved state can hold it. */
private val OpenSections = listSaver<SnapshotStateMap<Int, Boolean>, Int>(
    save = { m -> m.flatMap { (i, open) -> listOf(i, if (open) 1 else 0) } },
    restore = { l -> mutableStateMapOf<Int, Boolean>().apply { l.chunked(2).forEach { (i, open) -> put(i, open == 1) } } },
)
