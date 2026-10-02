package org.orthodoxwest.office

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.BasicText
import androidx.compose.foundation.text.TextAutoSize
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.LayoutCoordinates
import androidx.compose.ui.layout.layout
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.LocalDate
import org.orthodoxwest.office.core.HomeView

private val PERIODS = listOf(
    Triple(Period.MORNING, "Morning", listOf("lauds", "prime")),
    Triple(Period.DAY, "Day", listOf("terce", "sext", "none")),
    Triple(Period.EVENING, "Evening", listOf("vespers", "compline")),
)

/** Home: the day's frontispiece, the invitation to pray, and the hours of the day. */
@Composable
fun HomeScreen(
    view: HomeView,
    date: LocalDate,
    today: LocalDate,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onDate: (LocalDate) -> Unit,
    onHour: (LocalDate, String) -> Unit,
    onOrdoDay: () -> Unit,
) {
    // Where the niche stands in the room, which the chapel light follows.
    var room by remember { mutableStateOf<LayoutCoordinates?>(null) }
    var nicheBounds by remember { mutableStateOf<Rect?>(null) }
    BoxWithConstraints(Modifier.fillMaxSize().onGloballyPositioned { room = it }) {
        // Apse: one fixed field, anchored top centre, clearing the header.
        VaultField(Modifier.fillMaxSize(), listOf(0f to 0f, 0.09f to 0f, 0.16f to 0.9f, 0.6f to 0.7f, 1f to 0.3f))
        // A wide screen sets the frontispiece in a niche, and lights the room toward it.
        val screen = maxWidth
        val screenHeight = maxHeight
        val niche = if (LocalWide.current) nicheTokens(LocalPalette.current) else null
        if (niche != null) ChapelLight(niche, nicheBounds)
        val top = insets.calculateTopPadding()
        val bottom = insets.calculateBottomPadding()
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(top = top, bottom = bottom),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            // A wide screen centres the niche between the header and the foot, as the web's desktop
            // home does; a phone sets its card under the header.
            Column(
                Modifier.fillMaxWidth().then(if (niche != null) Modifier.heightIn(min = screenHeight - top - bottom) else Modifier),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = if (niche != null) Arrangement.SpaceBetween else Arrangement.Top,
            ) {
                chrome()
                if (niche != null) {
                    // The moulding stands 0.75rem out from the card; room for it below the header.
                    Frontispiece(
                        view, date, today, onDate, onHour, onOrdoDay,
                        Modifier.padding(horizontal = 24.dp).padding(top = 40.dp, bottom = 12.dp).widthIn(max = nicheWidth(screen)).fillMaxWidth()
                            .onGloballyPositioned { card -> nicheBounds = room?.takeIf { it.isAttached }?.localBoundingBoxOf(card) },
                        niche = niche,
                        head = nicheHead(screen),
                    )
                } else {
                    Frontispiece(view, date, today, onDate, onHour, onOrdoDay, Modifier.widthIn(max = 576.dp).fillMaxWidth().padding(horizontal = Gutter).padding(top = 16.dp))
                }
                // Home already opens on the cross: its foot takes no diamond.
                Footer(diamond = false)
            }
        }
    }
}

@Composable
private fun Frontispiece(
    view: HomeView,
    date: LocalDate,
    today: LocalDate,
    onDate: (LocalDate) -> Unit,
    onHour: (LocalDate, String) -> Unit,
    onOrdoDay: () -> Unit,
    modifier: Modifier,
    niche: NicheTokens? = null,
    head: Dp = 0.dp,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    var picking by remember { mutableStateOf(false) }
    val day = dayColor(view.color)
    val desk = niche != null
    val side = if (desk) 28.dp else 16.dp
    // The card's top padding: on a wide screen, room under the niche's head for the crown's cross
    // and the lining's arch.
    val top = if (desk) head * 0.62f + 33.6.dp else 18.2.dp
    Box(
        if (niche != null) {
            // The niche: a low round head, the stone moulding, the day's colour as its trim.
            val frame = if (p.dark) Color(208, 176, 106).copy(alpha = 0.34f) else Color(87, 52, 33).copy(alpha = 0.3f)
            modifier.drawBehind { niche(niche, p, day, head.toPx(), frame) }
        } else {
            modifier
                .background(p.surface)
                .border(1.dp, p.border)
                .drawBehind {
                    // The day's colour as the frame's top edge, like a vestment's trim.
                    drawRect(day, size = size.copy(height = 3.dp.toPx()))
                    // Book-cover tooling 0.35rem inside the frame (the top's inside the trim).
                    val w = 1.dp.toPx()
                    val inset = 5.6.dp.toPx() + w * 1.5f
                    val topInset = inset + 2.dp.toPx()
                    drawRect(
                        o.flat.copy(alpha = 0.18f),
                        Offset(inset, topInset),
                        Size(size.width - 2 * inset, size.height - topInset - inset),
                        style = Stroke(w),
                    )
                }
        },
    ) {
        // The consecration cross crowns the niche's head; a phone's card opens with it below.
        if (desk) ConsecrationCross(Modifier.align(Alignment.TopCenter).padding(top = head * 0.36f - 2.dp).size(36.dp))
        Column(
            Modifier.fillMaxWidth().padding(start = side, end = side, top = top, bottom = if (desk) 20.dp else 16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            if (!desk) {
                ConsecrationCross(Modifier.size(30.4.dp))
                Spacer(Modifier.height(8.8.dp))
            }
            // The day, down to the inscription band. On a wide screen a lining is painted round
            // the niche's head on its back wall, 26dp inside the moulding, ending at the band.
            Column(
                Modifier.fillMaxWidth()
                    .then(if (desk) Modifier.drawBehind { nicheLining(p.lining, 28.dp.toPx() - top.toPx(), head.toPx() - 26.dp.toPx()) } else Modifier)
                    .padding(bottom = 11.2.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    view.dateLabel,
                    Modifier.semantics { heading() }.tap(action = "open the ordo", onClick = onOrdoDay),
                    style = Type.body.copy(fontSize = if (desk) 25.92.sp else 22.08.sp, lineHeight = if (desk) 31.1.sp else 26.5.sp, letterSpacing = if (desk) 0.39.sp else 0.22.sp, color = p.text, textAlign = TextAlign.Center),
                )
                Text(view.feast, Modifier.padding(top = 2.dp), style = Type.body.copy(fontSize = if (desk) 18.72.sp else 17.28.sp, lineHeight = if (desk) 23.4.sp else 21.6.sp, color = p.accent, textAlign = TextAlign.Center))
                if (view.octaveNote.isNotEmpty()) Text(view.octaveNote, style = Type.small.copy(color = p.muted))
                if (!view.isToday) {
                    Text(
                        "GO TO TODAY",
                        Modifier.heightIn(min = 44.dp).tap { onDate(today) }.padding(vertical = 12.dp).goldUnderline(true, p.goldLine),
                        style = Type.menu.copy(color = p.accent),
                    )
                }
                if (view.penitential.isNotEmpty()) {
                    Row(Modifier.padding(top = 12.dp), horizontalArrangement = Arrangement.spacedBy(9.6.dp)) {
                        view.penitential.forEach { Text(it, style = Type.small.copy(color = p.rubric, fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 0.75.sp)) }
                    }
                }
                if (view.commemorations.isNotEmpty()) {
                    Column(Modifier.padding(top = 8.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                        Text("ALSO", style = Type.label(10.56f, 0.1f).copy(color = p.muted))
                        view.commemorations.forEach { Text(it, style = Type.small.copy(color = p.text, textAlign = TextAlign.Center, fontSize = 14.sp, lineHeight = 20.sp)) }
                    }
                }
            }
            // The inscription band: gilt letters on the frieze's green earth, between oxblood
            // rules, its phrase parted from the frame by gilt lozenges 5dp square.
            Row(
                Modifier
                    .throughPadding(side)
                    .background(p.inscriptionGround)
                    .drawBehind {
                        drawLine(p.inscriptionEdge, Offset(0f, 0f), Offset(size.width, 0f), 1.dp.toPx())
                        drawLine(p.inscriptionEdge, Offset(0f, size.height), Offset(size.width, size.height), 1.dp.toPx())
                    }
                    .padding(vertical = 3.2.dp),
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Canvas(Modifier.size(7.dp)) { lozenge(center, size.minDimension / 2f, o.ink, null) }
                Text("Pray the hours", Modifier.padding(horizontal = 12.dp), style = Type.label(12.8f, if (desk) 0.17f else 0.16f).copy(color = o.ink, fontFeatureSettings = ALL_SMALL_CAPS))
                Canvas(Modifier.size(7.dp)) { lozenge(center, size.minDimension / 2f, o.ink, null) }
            }
            Spacer(Modifier.height(13.6.dp))
            PrayNow(view.prayNowLabel, desk) { onHour(LocalDate.of(view.prayNowDate.year, view.prayNowDate.month, view.prayNowDate.day), view.prayNowHour) }
            Spacer(Modifier.height(12.8.dp))
            HourDirectory(view.currentHour, desk) { onHour(date, it) }
            // Season and date control share one line after the invitation.
            Hairline(p.border, Modifier.padding(top = 11.2.dp))
            if (view.season.isNotEmpty()) Text(view.season, Modifier.padding(top = 4.8.dp), style = Type.small.copy(color = p.muted))
            Disclosure("Change date", picking, { picking = !picking })
            if (picking) DatePicker(date, today) { picking = false; onDate(it) }
        }
    }
}

/** Runs through the card's side padding to the frame's inner edges, as the inscription band does. */
private fun Modifier.throughPadding(side: Dp): Modifier = this.layout { measurable, constraints ->
    val extra = (side * 2).roundToPx()
    val placeable = measurable.measure(constraints.copy(minWidth = constraints.maxWidth + extra, maxWidth = constraints.maxWidth + extra))
    layout(constraints.maxWidth, placeable.height) { placeable.place(-extra / 2, 0) }
}

/** The invitation: a single painted line, which does not veil with the season. */
@Composable
private fun PrayNow(label: String, desk: Boolean, onClick: () -> Unit) {
    val p = LocalPalette.current
    Box(
        Modifier
            .fillMaxWidth()
            .tap(onClick = onClick)
            .drawBehind {
                val w = 1.dp.toPx()
                drawRect(p.lining, Offset(w / 2f, w / 2f), size.copy(width = size.width - w, height = size.height - w), style = Stroke(w))
            }
            .padding(vertical = if (desk) 12.dp else 13.9.dp, horizontal = 15.8.dp),
        contentAlignment = Alignment.Center,
    ) {
        // On Apse the words are ivory, so the frame carries the colour.
        Text(label, style = Type.body.copy(fontSize = if (desk) 20.sp else 19.2.sp, lineHeight = if (desk) 26.sp else 24.96.sp, letterSpacing = 0.38.sp, color = if (p.dark) p.text else p.accent))
    }
}

/** The hours by period in horizontal bands, the current one underlined in gold. */
@Composable
private fun HourDirectory(current: String, desk: Boolean, onHour: (String) -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    // The desktop's labels are in the accent, their column 5.25rem.
    val labelStyle = Type.label(11.52f, 0.08f).copy(color = if (desk) p.accent else p.muted, fontFeatureSettings = ALL_SMALL_CAPS)
    // One width for the three period labels, widened past the web's 83dp only when the reader's
    // font size needs it, so the hours still line up in columns.
    val measurer = rememberTextMeasurer()
    val density = LocalDensity.current
    val labelWidth = remember(labelStyle, density) {
        with(density) { PERIODS.maxOf { measurer.measure(it.second, labelStyle).size.width }.toDp() + 8.dp }.coerceAtLeast(if (desk) 84.dp else 83.dp)
    }
    Column(Modifier.fillMaxWidth().border(1.dp, p.border)) {
        PERIODS.forEachIndexed { i, (period, label, hours) ->
            if (i > 0) Hairline(p.border)
            Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min)) {
                Column(
                    Modifier.width(labelWidth).fillMaxHeight().background(p.inscriptionWash).padding(vertical = 6.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center,
                ) {
                    PeriodIcon(period, o.flat)
                    Text(label, Modifier.padding(top = 2.dp), softWrap = false, style = labelStyle)
                }
                Box(Modifier.width(1.dp).fillMaxHeight().background(p.border))
                Row(Modifier.weight(1f).heightIn(min = if (desk) 46.dp else 44.dp), verticalAlignment = Alignment.CenterVertically) {
                    hours.forEachIndexed { j, h ->
                        if (j > 0) Divider(p.border)
                        Box(Modifier.weight(1f).fillMaxHeight().tap(label = if (h == current) "${hourLabel(h)}, now" else null) { onHour(h) }, contentAlignment = Alignment.Center) {
                            val name = Type.body.copy(fontSize = if (desk) 16.sp else 15.68.sp, lineHeight = 18.8.sp, letterSpacing = 0.31.sp, color = if (h == current) p.accent else p.text)
                            // Never broken mid-word: at the largest font sizes a name steps down to fit its cell.
                            BasicText(
                                hourLabel(h),
                                Modifier.padding(horizontal = 2.dp).goldUnderline(h == current, p.goldLine),
                                style = name,
                                maxLines = 1,
                                autoSize = TextAutoSize.StepBased(minFontSize = 11.sp, maxFontSize = name.fontSize),
                            )
                        }
                    }
                }
            }
        }
    }
}
