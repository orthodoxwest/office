package org.orthodoxwest.office

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
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
import androidx.compose.foundation.layout.wrapContentHeight
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
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.layout.LayoutCoordinates
import androidx.compose.ui.layout.layout
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.res.imageResource
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.LineBreak
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.LocalDate
import kotlin.math.roundToInt
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
        // The height home has under the bars, which sets a phone's tier as the web's viewport does.
        val tall = maxHeight - top - bottom
        val niche = if (LocalWide.current) nicheTokens(LocalPalette.current) else null
        if (niche != null) ChapelLight(niche, nicheBounds)
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(top = top, bottom = bottom),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            if (niche != null) {
                // Home is at least a screen tall, its colophon at the foot, and the niche centred
                // between the header and the foot, as the web's desktop home.
                val cardWidth = nicheWidth(screen)
                Column(
                    Modifier.fillMaxWidth().heightIn(min = tall),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.SpaceBetween,
                ) {
                    chrome()
                    // The moulding stands 0.75rem out from the card; room for it below the header.
                    Frontispiece(
                        view, date, today, onDate, onHour, onOrdoDay,
                        Modifier.padding(horizontal = 24.dp).padding(top = 40.dp, bottom = 12.dp).widthIn(max = cardWidth).fillMaxWidth()
                            .onGloballyPositioned { card -> nicheBounds = room?.takeIf { it.isAttached }?.localBoundingBoxOf(card) },
                        HomeTier(desk = true, screen = screen, tall = tall, card = cardWidth, dyn = LocalDensity.current.fontScale),
                        niche = niche,
                    )
                    Footer(reserve = true)
                }
            } else {
                // A phone's card stands from the header to a little above the footer: whatever
                // height home has beyond its own goes to the panel, above the band.
                val card = minOf(576.dp, screen) - Gutter * 2
                PhoneHome(
                    tall,
                    header = chrome,
                    card = {
                        Frontispiece(
                            view, date, today, onDate, onHour, onOrdoDay,
                            Modifier.widthIn(max = 576.dp).fillMaxWidth().padding(horizontal = Gutter).padding(top = 13.6.dp),
                            HomeTier(desk = false, screen = screen, tall = tall, card = card, dyn = LocalDensity.current.fontScale),
                        )
                    },
                    footer = { Footer(gap = 29.6.dp, bottom = 25.6.dp, reserve = true) },
                )
            }
        }
    }
}

/**
 * A phone's home: the header, the card and the footer, at least `height` tall, the card given the
 * height left over as its least, so it stands from the header to a little above the footer.
 */
@Composable
private fun PhoneHome(height: Dp, header: @Composable () -> Unit, card: @Composable () -> Unit, footer: @Composable () -> Unit) {
    Layout(contents = listOf(header, card, footer), modifier = Modifier.fillMaxWidth()) { (headers, cards, footers), constraints ->
        val width = constraints.maxWidth
        val loose = Constraints(maxWidth = width)
        val top = headers.map { it.measure(loose) }
        val foot = footers.map { it.measure(loose) }
        val spare = (height.roundToPx() - top.sumOf { it.height } - foot.sumOf { it.height }).coerceAtLeast(0)
        val middle = cards.map { it.measure(Constraints(maxWidth = width, minHeight = spare)) }
        val above = top.sumOf { it.height } + middle.sumOf { it.height }
        val total = maxOf(above + foot.sumOf { it.height }, height.roundToPx())
        layout(width, total) {
            var y = 0
            (top + middle).forEach { it.place((width - it.width) / 2, y); y += it.height }
            // The colophon at the foot of the screen.
            y = total - foot.sumOf { it.height }
            foot.forEach { it.place((width - it.width) / 2, y); y += it.height }
        }
    }
}

/**
 * Home's measures at a size of screen, as style.css's home tiers set them. A phone's head, its
 * cross and the room above the date go by the height home has (`tall`): from 800 high the card
 * has height to spare, so the head rises further to a sharper point, the cross and the date stand
 * lower in it, and the spare height is parted two to three above and below the day rather than
 * centred about it; from 880 more so. Its larger rows and controls go by width as well, from 375
 * wide and 830 or 880 high: a narrower or shorter phone needs the height for the lines its day
 * wraps to. A wide screen's niche (`desk`) has its own. `card` is the card's width; `dyn` the
 * system's font scale, which sp carries.
 */
private class HomeTier(val desk: Boolean, screen: Dp, tall: Dp, card: Dp, private val dyn: Float) {
    private val step = if (desk) 0 else if (tall >= 880.dp) 2 else if (tall >= 800.dp) 1 else 0
    private val type = if (desk || screen < 375.dp) 0 else if (tall >= 880.dp) 2 else if (tall >= 830.dp) 1 else 0
    private fun <T> pick(niche: T, vararg phone: T): T = if (desk) niche else phone[minOf(type, phone.size - 1)]
    private fun <T> rise(niche: T, vararg phone: T): T = if (desk) niche else phone[step]

    val arch = rise(NicheArch, PhoneArch, TallArch, TallerArch)
    /**
     * The head's type is fitted to the panel, not to the system's font scale (the web's
     * `--date-size`). The head is as tall as the arch's rise, a share of the card's width, plus
     * whatever height home has to spare, so type set by the font scale floats in it on a phone
     * whose text is set small and crowds it set large. The date is 7% of the card's inline size
     * (its width inside 16dp of padding) or 2.92% of home's height, whichever is more, within
     * bounds that keep the font scale; the feast and the commemorations follow in proportion.
     * The fitted size is in dp, so the font scale sp carries is divided out of it. The usual
     * scale at 375×667 gives the sizes the head had (22, 17 and 14sp), a tall phone the sizes its
     * tier had (25 and 27sp for the date). A niche keeps its own sizes.
     */
    private fun fitted(inline: Float, height: Float, lo: Float, hi: Float) = (maxOf(inline, height) / dyn).coerceIn(lo, hi)
    private val inline = (card - 32.dp).value
    val date = if (desk) 25.92f else fitted(0.07f * inline, 0.0292f * tall.value, 19.2f, 32f)
    /** The date's line, its size and a fifth, in dp. */
    private val dateLine = (date * dyn * 1.2f).dp
    /**
     * The room under the point for the cross and air, before the date. On a phone the date's tap
     * box is a thumb's height with its line at the foot, so the room gives back the box's slack
     * above the line: the date stands where it stood centred in the box, the feast closer under it.
     */
    val headPad = rise(120.dp, 86.4.dp, 105.6.dp, 118.4.dp) - if (desk) 0.dp else (44.dp - dateLine) / 2
    val crownTop = rise(48.dp, 33.6.dp, 49.6.dp, 54.4.dp)
    val crownSize = rise(36.dp, 30.4.dp, 35.2.dp, 40.dp)
    /** Spare height under the head is parted 2:3 above and below the day, else the day is centred in it. */
    val split = !desk && step > 0
    val side = if (desk) 28.dp else 16.dp
    private val lining = if (desk) 26.dp else PanelInset
    /**
     * How far in from the card's edge the day's words stand: inside the lining's hairline (its
     * inset, then 9dp) with 12dp of clear air, so a long feast name breaks rather than running
     * over the lining.
     */
    val dayClear = lining + 9.dp + 12.dp
    val bottom = if (desk) 20.dp else 12.dp
    val dateTracking = date * (if (desk) 0.015f else 0.01f)
    /**
     * The head's width `y` below the card's top inside the lining's hairline, less the day's 12dp
     * of air each side: a line of the day set there clears the lining.
     */
    private fun clear(card: Dp, y: Dp): Dp = (archChord(arch, card.value, -(lining + 9.dp).value, y.value) - 24f).dp
    /**
     * The date's measure, the head's width where its first line stands, and no wider than a short
     * phone's: a date too long for it breaks after the weekday, its second line lower where the
     * head is wider.
     */
    val dateMeasure: Dp = if (desk) Dp.Unspecified else minOf(card * pick(0f, 0.233f, 0.3f) + 139.dp, clear(card, headPad + 44.dp - dateLine))
    val feast = if (desk) 18.72f else fitted(0.055f * inline, 0.0215f * tall.value, 15.2f, 24f)
    /**
     * The feast's measure, the head's width at its first line, when the day stands at the head's
     * room: a long name breaks there rather than running over the lining further up the arch.
     */
    val feastMeasure: Dp = if (desk) Dp.Unspecified else clear(card, headPad + 44.dp + 2.dp)
    /** A short phone's commemorations give way, so a past date with them still fits. */
    val commemoration = if (!desk && tall <= 700.dp) 13.6f else if (desk) 14f else fitted(0.046f * inline, 0.0175f * tall.value, 12.8f, 17.6f)
    val commemorationLine = if (!desk && tall <= 700.dp) 1.3f else 1.4f
    /**
     * The day's versicle's size, set only where a phone is over 700 high, and its measure, at most
     * 19rem wide and inside the day's clearance of the lining.
     */
    val versicle: Float? = if (!desk && tall <= 700.dp) null else 15.36f
    /** The invitation's note needs height a short phone lacks, as on the web. */
    val prayNote = desk || tall > 700.dp
    val versicleMeasure = minOf(304.dp, card - dayClear * 2)
    /** Below the day, to the band. */
    val dayGap = pick(16.8.dp, 5.6.dp, 12.dp)
    val band = pick(12.8f, 12.8f, 12.8f, 13.6f)
    val bandPad = pick(3.2.dp, 3.2.dp, 3.2.dp, 3.84.dp)
    val bandGap = pick(15.2.dp, 12.dp, 16.dp, 20.dp)
    val pray = pick(23.2f, 19.2f, 22.4f, 24f)
    val prayPad = pick(13.8.dp, 13.9.dp, 15.5.dp, 17.9.dp)
    val prayGap = pick(15.2.dp, 11.2.dp, 20.dp, 24.dp)
    val row = pick(46.dp, 44.dp, 54.4.dp, 62.4.dp)
    val hour = pick(17.6f, 15.68f)
    val metaGap = pick(14.4.dp, 8.8.dp, 16.dp, 19.2.dp)
}

/** The date with its weekday and the rest each kept whole, so a break never parts a month from its day. */
private fun unbroken(date: String): String {
    val at = date.indexOf(", ")
    if (at < 0) return date
    return date.substring(0, at + 1).replace(' ', '\u00A0') + " " + date.substring(at + 2).replace(' ', '\u00A0')
}

/**
 * CSS's color-mix in sRGB: `a` at `share` and `b` the rest, premultiplied, as CSS mixes
 * translucent colours.
 */
private fun mix(a: Color, b: Color, share: Float): Color {
    val alpha = a.alpha * share + b.alpha * (1 - share)
    if (alpha == 0f) return Color.Transparent
    fun channel(x: Float, y: Float) = (x * a.alpha * share + y * b.alpha * (1 - share)) / alpha
    return Color(channel(a.red, b.red), channel(a.green, b.green), channel(a.blue, b.blue), alpha)
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

/** The phone panel's lining stands this far inside its edge (`--panel-inset`); the niche's 26dp. */
private val PanelInset = 12.dp

@Composable
private fun Frontispiece(
    view: HomeView,
    date: LocalDate,
    today: LocalDate,
    onDate: (LocalDate) -> Unit,
    onHour: (LocalDate, String) -> Unit,
    onOrdoDay: () -> Unit,
    modifier: Modifier,
    tier: HomeTier,
    niche: NicheTokens? = null,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val ink = frontispieceInk(p)
    val panelPlaster = ImageBitmap.imageResource(p.panel)
    var picking by remember { mutableStateOf(false) }
    val day = dayColor(view.color, p)
    // The lining's inner line is the day's colour, beside the cross on the plaster; a white day's
    // would be tan there, and takes the gold line by day.
    val liningDay = if (!p.dark && view.color == "white") p.goldLine else day
    val desk = tier.desk
    val side = tier.side
    Layout(
        contents = listOf<@Composable () -> Unit>(
            // The consecration cross in the point of the head, with clear air round it.
            { ConsecrationCross(Modifier.size(tier.crownSize)) },
            // A lining painted round the head on the back wall, down to the inscription band.
            { Spacer(Modifier.drawBehind { lining(tier.arch, (if (desk) 26.dp else PanelInset).toPx(), p.lining, liningDay) }) },
            // The day.
            {
                Column(Modifier.fillMaxWidth().padding(horizontal = tier.dayClear), horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        unbroken(view.dateLabel),
                        // On a phone a full thumb's height, as the web's, its line at the foot so
                        // the feast stands close under it.
                        Modifier.widthIn(max = tier.dateMeasure).semantics { heading() }.tap(action = "open the ordo", onClick = onOrdoDay)
                            .heightIn(min = if (desk) 0.dp else 44.dp).wrapContentHeight(Alignment.Bottom),
                        style = Type.body.copy(fontSize = tier.date.sp, lineHeight = (tier.date * 1.2f).sp, letterSpacing = tier.dateTracking.sp, color = p.text, textAlign = TextAlign.Center),
                    )
                    FeastName(view.feastName, view.feastAlias, tier.feast, Modifier.padding(top = 2.dp).widthIn(max = tier.feastMeasure))
                    if (view.octaveNote.isNotEmpty()) Text(view.octaveNote, style = Type.small.copy(color = p.muted))
                    if (view.penitential.isNotEmpty()) {
                        Row(Modifier.padding(top = 12.dp), horizontalArrangement = Arrangement.spacedBy(9.6.dp)) {
                            view.penitential.forEach { Text(it, style = Type.small.copy(color = p.rubric, fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 0.75.sp)) }
                        }
                    }
                    if (view.commemorations.isNotEmpty()) {
                        Column(Modifier.padding(top = 8.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                            Text("ALSO", style = Type.label(10.56f, 0.1f).copy(color = p.muted))
                            // A second commemoration is joined to the first with "and", as the web's
                            // `.commemorations li + li::before`, so the list reads as a sentence.
                            view.commemorations.forEachIndexed { i, name ->
                                val text = if (i == 0) AnnotatedString(name) else buildAnnotatedString {
                                    withStyle(SpanStyle(fontStyle = FontStyle.Italic)) { append("and ") }
                                    append(name)
                                }
                                Text(text, style = Type.small.copy(color = p.text, textAlign = TextAlign.Center, fontSize = tier.commemoration.sp, lineHeight = (tier.commemoration * tier.commemorationLine).sp))
                            }
                        }
                    }
                }
            },
            // The day's versicle, when the head has the height to spare for it.
            {
                val style = tier.versicle
                if (style != null && view.versicle.isNotEmpty()) {
                    Box(Modifier.fillMaxWidth().padding(top = 12.dp), contentAlignment = Alignment.TopCenter) {
                        Column(Modifier.widthIn(max = tier.versicleMeasure), horizontalAlignment = Alignment.CenterHorizontally) {
                            VersicleLine("℣.", view.versicle, style)
                            VersicleLine("℟.", view.response, style)
                        }
                    }
                }
            },
            // After the day's facts, so they read together.
            {
                if (!view.isToday) {
                    Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.TopCenter) {
                        Text(
                            "GO TO TODAY",
                            Modifier.heightIn(min = 44.dp).tap { onDate(today) }.padding(vertical = 12.dp).goldUnderline(true, p.goldLine),
                            style = Type.menu.copy(color = p.accent),
                        )
                    }
                }
            },
            // The invitation and the hours, from the inscription band.
            {
                Column(Modifier.fillMaxWidth().padding(horizontal = side), horizontalAlignment = Alignment.CenterHorizontally) {
                    // The inscription band: gilt letters on the frieze's green earth, between
                    // oxblood rules each with a gilt fillet inside it, its phrase parted from the
                    // frame by gilt lozenges 5dp square.
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
                            .padding(vertical = tier.bandPad),
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Canvas(Modifier.size(7.dp)) { lozenge(center, size.minDimension / 2f, o.ink, null) }
                        Text("Pray the hours", Modifier.padding(horizontal = 12.dp), style = Type.label(tier.band, if (desk) 0.17f else 0.16f).copy(color = o.ink, fontFeatureSettings = ALL_SMALL_CAPS))
                        Canvas(Modifier.size(7.dp)) { lozenge(center, size.minDimension / 2f, o.ink, null) }
                    }
                    Spacer(Modifier.height(tier.bandGap))
                    PrayNow(view.prayNowLabel, if (tier.prayNote) view.prayNowNote else "", tier) { onHour(LocalDate.of(view.prayNowDate.year, view.prayNowDate.month, view.prayNowDate.day), view.prayNowHour) }
                    Spacer(Modifier.height(tier.prayGap))
                    HourDirectory(view.currentHour, tier) { onHour(date, it) }
                    // Season and date control share one line after the invitation.
                    Hairline(ink.rule, Modifier.padding(top = tier.metaGap))
                    if (view.season.isNotEmpty()) Text(view.season, Modifier.padding(top = 3.2.dp), style = Type.small.copy(color = p.muted))
                    Disclosure("Change date", picking, { picking = !picking })
                    Unfold(picking) { DatePicker(date, today) { picking = false; onDate(it) } }
                }
            },
        ),
        modifier = modifier.drawBehind {
            if (niche != null) {
                // The niche: a pointed head, the stone moulding, the day's colour as its trim.
                niche(tier.arch, niche, p, day, ink.frame)
            } else {
                // The panel: the day's colour a little toward the frame as a ring at its edge, so
                // a red or green day edges the head without outshouting the cross.
                panel(tier.arch, p, mix(day, ink.frame, 0.7f), ink.frame, panelPlaster)
            }
        },
    ) { slots, constraints ->
        val (cross, walls, dayBlock, verses, backs) = slots
        val width = constraints.maxWidth
        val full = Constraints.fixedWidth(width)
        val mark = cross.first().measure(Constraints())
        val facts = dayBlock.first().measure(full)
        val versicle = verses.firstOrNull()?.measure(full)
        val back = backs.firstOrNull()?.measure(full)
        val after = slots[5].first().measure(full)
        val headPad = tier.headPad.roundToPx()
        val foot = tier.bottom.roundToPx()
        val day = facts.height + (back?.height ?: 0) + tier.dayGap.roundToPx()
        // The day under the head's room for the cross, and the band no higher than the springing,
        // so it never crosses the arch; on a phone, the height the card is given beyond its own.
        val summary = maxOf(headPad + day, (tier.arch.rise * width).roundToInt(), constraints.minHeight - after.height - foot)
        // The versicle takes only the height the head has to spare: where it would make the card
        // taller, the head goes without it.
        val verse = versicle?.takeIf { headPad + day + it.height <= summary }
        val spare = summary - headPad - day - (verse?.height ?: 0)
        val wall = walls.first().measure(Constraints.fixed(width, summary))
        val height = summary + after.height + foot
        layout(width, height) {
            wall.place(0, 0)
            mark.place((width - mark.width) / 2, tier.crownTop.roundToPx())
            var y = headPad + if (tier.split) spare * 2 / 5 else spare / 2
            facts.place(0, y)
            y += facts.height
            verse?.let { it.place(0, y); y += it.height }
            back?.place(0, y)
            after.place(0, summary)
        }
    }
}

/**
 * The feast's name broken into balanced lines, as the web's, and its familiar name in italic: beside
 * it where the line has room, else on a line of its own, never broken. A reader hears the two
 * together.
 */
@Composable
private fun FeastName(name: String, alias: String, size: Float, modifier: Modifier) {
    val p = LocalPalette.current
    val style = Type.body.copy(fontSize = size.sp, lineHeight = (size * 1.25f).sp, color = p.accent, textAlign = TextAlign.Center, lineBreak = LineBreak.Heading)
    FlowRow(modifier.semantics(mergeDescendants = true) {}, horizontalArrangement = Arrangement.spacedBy((size * 0.25f).dp, Alignment.CenterHorizontally)) {
        Text(name, style = style)
        if (alias.isNotEmpty()) Text(alias, style = style.copy(fontStyle = FontStyle.Italic), softWrap = false)
    }
}

/** A line of the day's versicle: its ℣ or ℟ upright in the rubrics' red, the words in italic, the text's ink a little withdrawn. */
@Composable
private fun VersicleLine(sigil: String, words: String, size: Float) {
    val p = LocalPalette.current
    Text(
        buildAnnotatedString {
            withStyle(SpanStyle(fontStyle = FontStyle.Normal, color = p.rubric)) { append(sigil) }
            append(" ")
            append(words)
        },
        style = Type.body.copy(
            fontSize = size.sp,
            lineHeight = (size * 1.32f).sp,
            fontStyle = FontStyle.Italic,
            color = p.text.copy(alpha = p.text.alpha * 0.84f),
            textAlign = TextAlign.Center,
            lineBreak = LineBreak.Heading,
        ),
    )
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
private fun PrayNow(label: String, note: String, tier: HomeTier, onClick: () -> Unit) {
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
            // The note takes some of the box's lower air rather than a line of the panel's height.
            .padding(top = tier.prayPad, bottom = if (note.isEmpty()) tier.prayPad else (tier.prayPad - (tier.pray * 0.3f).dp).coerceAtLeast(2.dp), start = 15.8.dp, end = 15.8.dp),
        contentAlignment = Alignment.Center,
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(label, style = Type.body.copy(fontSize = tier.pray.sp, lineHeight = (tier.pray * 1.3f).sp, letterSpacing = 0.38.sp, color = if (p.dark) p.lining else p.titulus, textAlign = TextAlign.Center))
            // What the hour is, so a newcomer knows what they are opening.
            if (note.isNotEmpty()) {
                val size = maxOf(12.8f, tier.pray * 0.58f)
                Text(
                    note,
                    Modifier.padding(top = (tier.pray * 0.1f).dp),
                    style = Type.small.copy(fontSize = size.sp, lineHeight = (size * 1.25f).sp, color = p.muted, fontStyle = FontStyle.Italic, textAlign = TextAlign.Center),
                )
            }
        }
    }
}

/**
 * The hours by period in horizontal bands, the current one underlined in gold: framed in the
 * lining thinned, ruled within in the frontispiece's ink, the period cells in its wash.
 */
@Composable
private fun HourDirectory(current: String, tier: HomeTier, onHour: (String) -> Unit) {
    val desk = tier.desk
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
                Row(Modifier.weight(1f).heightIn(min = tier.row), verticalAlignment = Alignment.CenterVertically) {
                    hours.forEachIndexed { j, h ->
                        if (j > 0) Divider(ink.rule, 16.dp)
                        Box(Modifier.weight(1f).fillMaxHeight().tap(label = if (h == current) "${hourLabel(h)}, now" else null) { onHour(h) }, contentAlignment = Alignment.Center) {
                            val name = Type.body.copy(fontSize = tier.hour.sp, lineHeight = (tier.hour * 1.2f).sp, letterSpacing = 0.31.sp, color = if (h == current) p.accent else p.text)
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
