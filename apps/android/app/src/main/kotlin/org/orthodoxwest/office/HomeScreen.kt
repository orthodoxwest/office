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
        val top = insets.calculateTopPadding()
        val bottom = insets.calculateBottomPadding()
        // The wall's one field, fixed to the screen and phased from its top: by night the vault,
        // clearing the header and thinning toward the foot; by day the powdering, cut square under
        // the beam.
        WallField(Modifier.fillMaxSize(), seam = top) { dark ->
            fun at(d: Dp) = (top + d).toPx() / size.height
            if (dark) {
                listOf(0f to 0f, at(56.dp) to 0f, at(112.dp) to 1f, maxOf(0.78f, at(112.dp)) to 1f, 1f to 0.6f)
            } else {
                listOf(0f to 0f, at(64.dp) to 0f, at(64.dp) to 1f, 1f to 1f)
            }
        }
        // A wide screen sets the frontispiece in a niche, and lights the room toward it.
        val screen = maxWidth
        val screenHeight = maxHeight
        val niche = if (LocalWide.current) nicheTokens(LocalPalette.current) else null
        if (niche != null) ChapelLight(niche, nicheBounds)
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(top = top, bottom = bottom),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            // Home is at least a screen tall, its colophon at the foot. A wide screen centres the
            // niche between the header and the foot, as the web's desktop home does; a phone sets
            // its panel under the header.
            Column(
                Modifier.fillMaxWidth().heightIn(min = screenHeight - top - bottom),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.SpaceBetween,
            ) {
                if (niche != null) {
                    chrome()
                    // The moulding stands 0.75rem out from the card; room for it below the header.
                    Frontispiece(
                        view, date, today, onDate, onHour, onOrdoDay,
                        Modifier.padding(horizontal = 24.dp).padding(top = 40.dp, bottom = 12.dp).widthIn(max = nicheWidth(screen)).fillMaxWidth()
                            .onGloballyPositioned { card -> nicheBounds = room?.takeIf { it.isAttached }?.localBoundingBoxOf(card) },
                        niche = niche,
                        head = nicheHead(screen),
                    )
                    Footer(reserve = true)
                } else {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        chrome()
                        Frontispiece(
                            view, date, today, onDate, onHour, onOrdoDay,
                            Modifier.widthIn(max = 576.dp).fillMaxWidth().padding(horizontal = Gutter).padding(top = 13.6.dp),
                            head = if (screen < 375.dp) 56.dp else 68.dp,
                        )
                    }
                    // The phone's home fits its screen with nothing to spare: the head is paid for
                    // in the footer's gap and padding.
                    Footer(gap = 29.6.dp, bottom = 25.6.dp, reserve = true)
                }
            }
        }
    }
}

/**
 * The frontispiece's painted furniture (`.home-hero`), the same at every width: its frame, the
 * rules within, the period cells' wash (the frieze's green earth, thinned), and the panel's own
 * rules, the lining thinned.
 */
private class FrontispieceInk(val frame: Color, val rule: Color, val band: Color, val panelRule: Color)

private fun frontispieceInk(p: Palette): FrontispieceInk = if (p.dark) {
    FrontispieceInk(Color(208, 176, 106).copy(alpha = 0.34f), Color(208, 176, 106).copy(alpha = 0.24f), Color(208, 176, 106).copy(alpha = 0.045f), p.lining.copy(alpha = 0.45f))
} else {
    FrontispieceInk(Color(87, 52, 33).copy(alpha = 0.3f), Color(107, 58, 31).copy(alpha = 0.22f), p.inscriptionGround.copy(alpha = 0.09f), p.lining.copy(alpha = 0.45f))
}

/** The phone panel's lining stands this far inside its edge. */
private val PanelInset = 9.6.dp

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
    val ink = frontispieceInk(p)
    var picking by remember { mutableStateOf(false) }
    val day = dayColor(view.color, p)
    // The lining's inner line is the day's colour, beside the cross on the plaster; a white day's
    // would be tan there, and takes the gold line by day.
    val liningDay = if (!p.dark && view.color == "white") p.goldLine else day
    val desk = niche != null
    val side = if (desk) 28.dp else 16.dp
    // The card's top padding, room under the head for the lining's arch, then the crown's cross,
    // then air before the date. On a phone: the lining's inset, air, the cross, its clearance. In a
    // niche the cross is measured down from the lining (2dp moulding, 26dp inset, 11dp + 8.8dp of
    // air), not from the head's height: a head that flattens on a narrow screen once ran the
    // lining's crown through the cross.
    val crown = if (desk) 2.dp + 26.dp + 11.dp + 8.8.dp else PanelInset + 13.6.dp
    val top = if (desk) crown + 36.dp + 21.6.dp else crown + 30.4.dp + 20.dp
    Box(
        modifier.drawBehind {
            if (niche != null) {
                // The niche: a low round head, the stone moulding, the day's colour as its trim.
                niche(niche, p, day, head.toPx(), ink.frame)
            } else {
                // The panel: a segmental head, the day's colour as a ring at its edge.
                panel(p, day, ink.frame, head.toPx())
            }
        },
    ) {
        // The consecration cross at the crown of the head, with clear air round it.
        ConsecrationCross(Modifier.align(Alignment.TopCenter).padding(top = crown).size(if (desk) 36.dp else 30.4.dp))
        Column(
            Modifier.fillMaxWidth().padding(start = side, end = side, top = top, bottom = if (desk) 20.dp else 12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            // The day, down to the inscription band. A lining is painted round the head on the
            // back wall, ending at the band: on a phone PanelInset inside the edge, its curve
            // springing 8dp below the head's; on a wide screen 26dp inside the moulding.
            Column(
                Modifier.fillMaxWidth()
                    .drawBehind {
                        if (desk) {
                            nicheLining(p.lining, liningDay, 28.dp.toPx() - top.toPx(), 0f, head.toPx() - 26.dp.toPx())
                        } else {
                            nicheLining(p.lining, liningDay, (PanelInset - top).toPx(), (PanelInset - side).toPx(), (head - PanelInset + 8.dp).toPx())
                        }
                    }
                    .padding(bottom = 9.6.dp),
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
            // rules each with a gilt fillet inside it, its phrase parted from the frame by gilt
            // lozenges 5dp square.
            Row(
                Modifier
                    .throughPadding(side)
                    .background(p.inscriptionGround)
                    .drawBehind {
                        drawLine(p.inscriptionEdge, Offset(0f, 0f), Offset(size.width, 0f), 1.dp.toPx())
                        drawLine(p.inscriptionEdge, Offset(0f, size.height), Offset(size.width, size.height), 1.dp.toPx())
                        val fillet = o.ink.copy(alpha = 0.22f)
                        drawLine(fillet, Offset(0f, 1.5.dp.toPx()), Offset(size.width, 1.5.dp.toPx()), 1.dp.toPx())
                        drawLine(fillet, Offset(0f, size.height - 1.5.dp.toPx()), Offset(size.width, size.height - 1.5.dp.toPx()), 1.dp.toPx())
                    }
                    .padding(vertical = 3.2.dp),
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Canvas(Modifier.size(7.dp)) { lozenge(center, size.minDimension / 2f, o.ink, null) }
                Text("Pray the hours", Modifier.padding(horizontal = 12.dp), style = Type.label(12.8f, if (desk) 0.17f else 0.16f).copy(color = o.ink, fontFeatureSettings = ALL_SMALL_CAPS))
                Canvas(Modifier.size(7.dp)) { lozenge(center, size.minDimension / 2f, o.ink, null) }
            }
            Spacer(Modifier.height(12.dp))
            PrayNow(view.prayNowLabel, desk) { onHour(LocalDate.of(view.prayNowDate.year, view.prayNowDate.month, view.prayNowDate.day), view.prayNowHour) }
            Spacer(Modifier.height(11.2.dp))
            HourDirectory(view.currentHour, desk) { onHour(date, it) }
            // Season and date control share one line after the invitation.
            Hairline(ink.rule, Modifier.padding(top = 8.8.dp))
            if (view.season.isNotEmpty()) Text(view.season, Modifier.padding(top = 3.2.dp), style = Type.small.copy(color = p.muted))
            Disclosure("Change date", picking, { picking = !picking })
            Unfold(picking) { DatePicker(date, today) { picking = false; onDate(it) } }
        }
    }
}

/** Runs through the card's side padding to the frame's inner edges, as the inscription band does. */
private fun Modifier.throughPadding(side: Dp): Modifier = this.layout { measurable, constraints ->
    val extra = (side * 2).roundToPx()
    val placeable = measurable.measure(constraints.copy(minWidth = constraints.maxWidth + extra, maxWidth = constraints.maxWidth + extra))
    layout(constraints.maxWidth, placeable.height) { placeable.place(-extra / 2, 0) }
}

/**
 * The invitation: a painted line with a thinner one 3dp inside it, as a panel's border is ruled
 * twice; neither veils with the season. Its words are the tituli's red ochre, or by night the
 * lining.
 */
@Composable
private fun PrayNow(label: String, desk: Boolean, onClick: () -> Unit) {
    val p = LocalPalette.current
    val inner = frontispieceInk(p).panelRule
    Box(
        Modifier
            .fillMaxWidth()
            .tap(onClick = onClick)
            .drawBehind {
                val w = 1.dp.toPx()
                drawRect(p.lining, Offset(w / 2f, w / 2f), size.copy(width = size.width - w, height = size.height - w), style = Stroke(w))
                val i = w + 3.dp.toPx() + w / 2f
                drawRect(inner, Offset(i, i), size.copy(width = size.width - 2 * i, height = size.height - 2 * i), style = Stroke(w))
            }
            .padding(vertical = if (desk) 12.dp else 13.9.dp, horizontal = 15.8.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, style = Type.body.copy(fontSize = if (desk) 20.sp else 19.2.sp, lineHeight = if (desk) 26.sp else 24.96.sp, letterSpacing = 0.38.sp, color = if (p.dark) p.lining else p.titulus))
    }
}

/**
 * The hours by period in horizontal bands, the current one underlined in gold: framed in the
 * lining thinned, ruled within in the frontispiece's ink, the period cells in its wash.
 */
@Composable
private fun HourDirectory(current: String, desk: Boolean, onHour: (String) -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val ink = frontispieceInk(p)
    // The desktop's labels are in the accent, their column 5.25rem.
    val labelStyle = Type.label(11.52f, 0.08f).copy(color = if (desk) p.accent else p.muted, fontFeatureSettings = ALL_SMALL_CAPS)
    // One width for the three period labels, widened past the web's 83dp only when the reader's
    // font size needs it, so the hours still line up in columns.
    val measurer = rememberTextMeasurer()
    val density = LocalDensity.current
    val labelWidth = remember(labelStyle, density) {
        with(density) { PERIODS.maxOf { measurer.measure(it.second, labelStyle).size.width }.toDp() + 8.dp }.coerceAtLeast(if (desk) 84.dp else 83.dp)
    }
    Column(
        Modifier.fillMaxWidth().drawBehind {
            val w = 1.dp.toPx()
            drawRect(ink.panelRule, Offset(w / 2f, w / 2f), size.copy(width = size.width - w, height = size.height - w), style = Stroke(w))
        },
    ) {
        PERIODS.forEachIndexed { i, (period, label, hours) ->
            if (i > 0) Hairline(ink.rule)
            Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min)) {
                Column(
                    Modifier.width(labelWidth).fillMaxHeight().background(ink.band).padding(vertical = 6.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center,
                ) {
                    PeriodIcon(period, o.flat)
                    Text(label, Modifier.padding(top = 2.dp), softWrap = false, style = labelStyle)
                }
                Box(Modifier.width(1.dp).fillMaxHeight().background(ink.rule))
                Row(Modifier.weight(1f).heightIn(min = if (desk) 46.dp else 44.dp), verticalAlignment = Alignment.CenterVertically) {
                    hours.forEachIndexed { j, h ->
                        if (j > 0) Divider(ink.rule, 16.dp)
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
