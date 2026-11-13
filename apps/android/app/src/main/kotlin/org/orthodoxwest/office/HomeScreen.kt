package org.orthodoxwest.office

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.scrollBy
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
import androidx.compose.foundation.layout.offset
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
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.scale
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.layout.LayoutCoordinates
import androidx.compose.ui.layout.layout
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.layout.positionInParent
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.imageResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.LineBreak
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Constraints
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.time.LocalDate
import kotlin.math.roundToInt
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import org.orthodoxwest.office.core.HomeView
import org.orthodoxwest.office.core.hourNote

private val PERIODS = listOf(
    Triple(Period.MORNING, "Morning", listOf("lauds", "prime")),
    Triple(Period.DAY, "Day", listOf("terce", "sext", "none")),
    Triple(Period.EVENING, "Evening", listOf("vespers", "compline")),
)

/**
 * Home: the day's frontispiece, the invitation to pray, and the hours of the day. In a reader's
 * first week `introduction` names the link to About beside Change date.
 */
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
    onFeast: (String) -> Unit = {},
    introduction: String? = null,
    onIntroduction: () -> Unit = {},
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
        // By night the stars are the apse's own, inside the arch (`sky`), on a phone and in the
        // wide niche alike: the wall outside it is plain dark plaster.
        val night = LocalPalette.current.dark
        if (!night) WallField(Modifier.fillMaxSize(), seam = top) { dark ->
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
        // "Change date"'s picker unfolds at the card's foot, under the screen's on a phone: the
        // page rises as it opens (`Unfold`), to show the card's foot and the gutter under it
        // clear of the navigation bar.
        val scroll = rememberScrollState()
        val density = LocalDensity.current
        val follow = remember(scroll, density, bottom) { Follow(scroll, { room }, with(density) { bottom.toPx() }, with(density) { Gutter.toPx() }) }
        CompositionLocalProvider(LocalFollow provides follow) {
            Column(
                Modifier.fillMaxSize().verticalScroll(scroll).padding(top = top, bottom = bottom),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                if (niche != null) {
                    // Home is at least a screen tall, its colophon at the foot, and the niche centred
                    // between the header and the foot, as the web's wide home: a lancet for the
                    // screen's orientation at one scale, which the niche's density carries, so every
                    // measure inside it, its type included, is zoomed as the web's `zoom` does.
                    val layout = NicheLayout(screen, tall)
                    val outer = LocalDensity.current
                    val heavy = (view.penitential.isNotEmpty() && view.commemorations.isNotEmpty()) ||
                        (!view.isToday && (view.commemorations.isNotEmpty() || view.octaveNote.isNotEmpty() || view.penitential.isNotEmpty()))
                    // Where the heaviest day does not fit with the colophon (`--niche-fits`), the header
                    // and niche take the screen and the colophon waits under the fold, rather than
                    // being cut through by it.
                    Column(
                        Modifier.fillMaxWidth().heightIn(min = tall),
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.SpaceBetween,
                    ) {
                        Column(
                            Modifier.fillMaxWidth().then(if (layout.fits) Modifier else Modifier.heightIn(min = tall)),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.SpaceBetween,
                        ) {
                            chrome()
                            CompositionLocalProvider(LocalDensity provides Density(outer.density * layout.zoom, outer.fontScale)) {
                                Frontispiece(
                                    view, date, today, onDate, onHour, onOrdoDay, introduction, onIntroduction,
                                    Modifier.padding(horizontal = 24.dp).padding(top = layout.air).widthIn(max = layout.card).fillMaxWidth()
                                        .onGloballyPositioned { card -> nicheBounds = room?.takeIf { it.isAttached }?.localBoundingBoxOf(card) },
                                    HomeTier(screen = screen, tall = tall, card = layout.card, dyn = outer.fontScale, wide = layout, heavy = heavy),
                                    niche = niche,
                                    onFeast = onFeast,
                                )
                            }
                            // Centred under the header: the niche's own room is the space below it.
                            if (!layout.fits) Spacer(Modifier.height(0.dp))
                        }
                        Footer(reserve = true)
                    }
                } else {
                    // A phone's card stands from the header to the screen's foot, short of it by the
                    // gutter: whatever height home has beyond its own goes to the panel, above the
                    // band. The phone's home has no colophon; the wide home keeps it.
                    val card = minOf(576.dp, screen) - Gutter * 2
                    PhoneHome(
                        tall,
                        header = chrome,
                        card = {
                            Frontispiece(
                                view, date, today, onDate, onHour, onOrdoDay, introduction, onIntroduction,
                                Modifier.widthIn(max = 576.dp).fillMaxWidth().padding(horizontal = Gutter).padding(top = 13.6.dp),
                                HomeTier(screen = screen, tall = tall, card = card, dyn = LocalDensity.current.fontScale),
                                onFeast = onFeast,
                            )
                        },
                        foot = Gutter,
                    )
                }
            }
        }
    }
}

/**
 * A phone's home: the header and the card, at least `height` tall, the card given the height left
 * over above `foot` as its least, so it stands from the header to `foot` above the screen's foot.
 */
@Composable
private fun PhoneHome(height: Dp, header: @Composable () -> Unit, card: @Composable () -> Unit, foot: Dp) {
    Layout(contents = listOf(header, card), modifier = Modifier.fillMaxWidth()) { (headers, cards), constraints ->
        val width = constraints.maxWidth
        val loose = Constraints(maxWidth = width)
        val top = headers.map { it.measure(loose) }
        val below = foot.roundToPx()
        val spare = (height.roundToPx() - top.sumOf { it.height } - below).coerceAtLeast(0)
        val middle = cards.map { it.measure(Constraints(maxWidth = width, minHeight = spare)) }
        val total = maxOf(top.sumOf { it.height } + middle.sumOf { it.height } + below, height.roundToPx())
        layout(width, total) {
            var y = 0
            (top + middle).forEach { it.place((width - it.width) / 2, y); y += it.height }
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
 * wraps to. A wide screen's niche (`wide`) has its own, before its zoom, which the niche's density
 * carries ([NicheLayout]); a heavy day there (`heavy`: a fast with a commemoration, or a day opened
 * from another with a fast, a commemoration or an octave) closes up its title so the niche keeps
 * its height. `card` is the card's width; `dyn` the system's font scale, which sp carries.
 */
private class HomeTier(screen: Dp, tall: Dp, card: Dp, private val dyn: Float, val wide: NicheLayout? = null, val heavy: Boolean = false) {
    val desk = wide != null
    /** The wide niche's hours one to a line (from 1000 high), else its glossed table. */
    val list = wide?.list == true
    private val upright = wide?.upright == true
    private val zoom = wide?.zoom ?: 1f
    /**
     * A niche's small words never fall under 12 on the glass, however small the niche (a short
     * screen): each stands back from the zoom just enough, as the web's.
     */
    private fun legible(size: Float) = maxOf(size, 12f / zoom)
    /** The list of seven's middle column. */
    val listMiddle = wide?.listMiddle ?: 0.dp
    /** A short screen's niche (a phone on its side), fitted down to the invitation. */
    val shortScreen = wide?.short == true
    private val step = if (desk) 0 else if (tall >= 880.dp) 2 else if (tall >= 800.dp) 1 else 0
    private val type = if (desk || screen < 375.dp) 0 else if (tall >= 880.dp) 2 else if (tall >= 830.dp) 1 else 0
    private fun <T> pick(niche: T, vararg phone: T): T = if (desk) niche else phone[minOf(type, phone.size - 1)]
    private fun <T> rise(niche: T, vararg phone: T): T = if (desk) niche else phone[step]

    val arch = wide?.arch ?: rise(NicheArch, PhoneArch, TallArch, TallerArch)
    /**
     * The head's type is fitted to the panel, not to the system's font scale (the web's
     * `--date-size`). The head is as tall as the arch's rise, a share of the card's width, plus
     * whatever height home has to spare, so type set by the font scale floats in it on a phone
     * whose text is set small and crowds it set large. The date is 7% of the card's inline size
     * (its width inside 16dp of padding) or 2.92% of home's height, whichever is more, within
     * bounds that keep the font scale; the feast and the commemorations follow in proportion.
     * The fitted size is in dp, so the font scale sp carries is divided out of it. The usual
     * scale at 375×667 gives the sizes the head had (22, 17 and 14sp), a tall phone the sizes its
     * tier had (25 and 27sp for the date). A niche keeps its own sizes: the date 2.1rem, 2.3rem
     * over the list of seven (2.2rem upright).
     */
    private fun fitted(inline: Float, height: Float, lo: Float, hi: Float) = (maxOf(inline, height) / dyn).coerceIn(lo, hi)
    private val inline = (card - 32.dp).value
    val date = if (desk) (if (list) (if (upright) 35.2f else 36.8f) else 33.6f) else fitted(0.07f * inline, 0.0292f * tall.value, 19.2f, 32f)
    /** The date's line, its size and a fifth, in dp. */
    private val dateLine = (date * dyn * 1.2f).dp
    /**
     * The room under the point for the cross and air, before the date. On a phone the date's tap
     * box is a thumb's height with its line at the foot, so the room gives back the box's slack
     * above the line: the date stands where it stood centred in the box, the feast closer under it.
     * An upright niche sets its title lower in its taller head; a heavy day closes it up.
     */
    val headPad = if (desk) (if (upright) (if (heavy) 118.4.dp else 144.dp) else if (heavy) 92.8.dp else 118.4.dp) else rise(0.dp, 86.4.dp, 105.6.dp, 118.4.dp) - (44.dp - dateLine) / 2
    val crownTop = if (desk) (if (upright) (if (heavy) 62.4.dp else 73.6.dp) else if (heavy) 44.8.dp else 57.6.dp) else rise(0.dp, 33.6.dp, 49.6.dp, 54.4.dp)
    val crownSize = if (desk) (if (heavy) 32.dp else if (list) 40.dp else 36.8.dp) else rise(0.dp, 30.4.dp, 35.2.dp, 40.dp)
    /** Spare height under the head is parted 2:3 above and below the day, else the day is centred in it. */
    val split = !desk && step > 0
    val side = if (desk) 36.dp else 16.dp
    /** The lining stands this far inside the card's edge: 20dp in the niche (`--lining-inset`). */
    val lining = if (desk) 20.dp else PanelInset
    /**
     * How far in from the card's edge the day's words stand: inside the lining's hairline (its
     * inset, then 9dp) with clear air (12dp, a rem in the niche), so a long feast name breaks
     * rather than running over the lining.
     */
    val dayClear = lining + 9.dp + if (desk) 16.dp else 12.dp
    val bottom = if (desk) 17.6.dp else 12.dp
    val dateTracking = date * (if (desk) 0.005f else 0.01f)
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
    val dateMeasure: Dp = if (desk) Dp.Unspecified else clear(card, headPad + 44.dp - dateLine)
    /**
     * The weekday is a titulus in spaced small caps over the rest of the date set 10% larger (6%
     * on a short phone), as the web's `.home-date-weekday` and `.home-date-rest`: the weekday's
     * size, tracking and line, the rest's size and line. A day with no room to give (two
     * commemorations, or one opened from another day) closes the title up on a taller phone. In
     * the niche the weekday is half the date, tracked wider, over the date at its own size.
     */
    private val short = !desk && tall <= 700.dp
    fun weekday(): Float = if (desk) legible(0.5f * date) else maxOf(0.66f * date, 15.2f / dyn)
    val weekdayTracking = if (desk) 0.2f else 0.15f
    fun weekdayLine(heavy: Boolean): Float = if (desk) (if (heavy) 1.4f else 1.7f) else if (short) 1.18f else if (heavy) 1.06f else 1.5f
    fun rest(heavy: Boolean): Float = if (desk) date else date * if (short) 1.06f else if (heavy) 1.02f else 1.1f
    fun restLine(heavy: Boolean): Float = if (desk) 1.1f else if (short) 1.08f else if (heavy) 1f else 1.12f
    val feast = if (desk) legible(if (list) 22.72f else 21.12f) else fitted(0.055f * inline, 0.0215f * tall.value, 15.2f, 24f)
    /**
     * The feast's measure, the head's width at its first line, when the day stands at the head's
     * room: a long name breaks there rather than running over the lining further up the arch. The
     * niche's is 25rem.
     */
    val feastMeasure: Dp = if (desk) 400.dp else clear(card, headPad + 44.dp + 2.dp)
    /** A short phone's commemorations give way, so a past date with them still fits. */
    val commemoration = if (!desk && tall <= 700.dp) 13.6f else if (desk) legible(if (list) 16.64f else 16f) else fitted(0.046f * inline, 0.0175f * tall.value, 12.8f, 17.6f)
    val commemorationLine = if (!desk && tall <= 700.dp) 1.3f else 1.4f
    /** The fasts, as red work: a little larger in the niche. */
    val fast = if (desk) legible(14.4f) else 0f
    /**
     * The day's versicle's size, set only where a phone is over 700 high, and its measure, at most
     * 19rem wide and inside the day's clearance of the lining.
     */
    val versicle: Float? = if (!desk && tall <= 700.dp) null else 15.36f
    val versicleMeasure = minOf(304.dp, card - dayClear * 2)
    /** Below the day, to the band. */
    val dayGap = pick(17.6.dp, 5.6.dp, 12.dp)
    val band = pick(legible(14.4f), 12.8f, 12.8f, 13.6f)
    val bandTracking = if (desk) 0.2f else 0.15f
    val bandPad = pick(2.56.dp, 3.2.dp, 3.2.dp, 3.84.dp)
    val bandGap = pick(17.6.dp, 12.dp, 16.dp, 20.dp)
    val pray = pick(25.6f, 19.2f, 22.4f, 24f)
    /**
     * The invitation's padding above and below its words, its 1px line included, as the web's
     * `.pray-now` (0.68rem 0.72rem on a phone, 0.8rem from 830 high, 0.95rem from 880, 0.85rem
     * 0.9rem in the niche): measured against the web's snapshot, Compose sets the line where the
     * browser does, so nothing is added.
     */
    val prayTop = pick(14.6.dp, 11.9.dp, 13.8.dp, 16.2.dp)
    val prayBottom = pick(15.4.dp, 12.5.dp, 13.8.dp, 16.2.dp)
    val prayGap = pick(14.4.dp, 11.2.dp, 20.dp, 24.dp)
    /** An hour's row: the niche's table 3.5rem, its list 3.1rem (a thumb's height at its least zoom). */
    val row = if (desk) maxOf(if (list) 49.6.dp else 56.dp, (44f / zoom).dp) else pick(56.dp, 44.dp, 54.4.dp, 62.4.dp)
    val hour = if (desk) legible(if (list) 19.84f else 18.56f) else 15.68f
    /** The hour that is now is larger in the niche, as well as lit and rubricated. */
    val hourNow = if (desk) legible(if (list) 22.72f else 21.12f) else hour
    /** Every hour's gloss in the niche; a phone's only under the hour that is now. */
    val gloss = if (desk) legible(if (list) 16f else 13.76f) else 12.48f
    /** The periods' labels: larger in the niche, where they stand beside the list. */
    val period = if (desk) legible(if (list) 15.2f else 13.76f) else 11.52f
    val periodTracking = if (list) 0.16f else if (desk) 0.14f else 0.12f
    val metaGap = pick(8.8.dp, 8.8.dp, 16.dp, 19.2.dp)
    /** The season and "Change date" at the foot, in small caps. */
    val meta = if (desk) legible(15.2f) else 13.44f
}

/** The date with its weekday and the rest each kept whole, so a break never parts a month from its day. */
private fun unbroken(date: String): String {
    val at = date.indexOf(", ")
    if (at < 0) return date
    return date.substring(0, at + 1).replace(' ', '\u00A0') + " " + date.substring(at + 2).replace(' ', '\u00A0')
}

/**
 * The frontispiece's frame (`.home-hero`), the same at every width; the rules within are ruled by
 * hand in the lining (`HourDirectory`, `PrayNow`).
 */
private fun frontispieceFrame(p: Palette): Color = if (p.dark) Color(208, 176, 106).copy(alpha = 0.34f) else Color(87, 52, 33).copy(alpha = 0.3f)

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
    introduction: String?,
    onIntroduction: () -> Unit,
    modifier: Modifier,
    tier: HomeTier,
    niche: NicheTokens? = null,
    onFeast: (String) -> Unit = {},
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val frame = frontispieceFrame(p)
    val panelPlaster = ImageBitmap.imageResource(p.panel)
    var picking by remember { mutableStateOf(false) }
    val follow = LocalFollow.current
    // How far "Change date" has turned the stage from the hours to the calendar, 0 to 1, on the
    // disclosures' curves; while it opens, the page follows the card's foot where it lengthens.
    val turn = remember { Animatable(if (picking) 1f else 0f) }
    LaunchedEffect(picking) {
        coroutineScope {
            launch { turn.animateTo(if (picking) 1f else 0f, tween(if (picking) UNFOLD_MS else FOLD_MS, easing = FastOutSlowInEasing)) }
            if (picking && follow != null) launch {
                // Each frame, the page rises by as much as the card's foot has passed the screen's,
                // to the frame after the turn settles.
                do {
                    val settled = !turn.isRunning && turn.value >= 1f
                    withFrameNanos {}
                    val by = follow.overflow()
                    if (by > 0.5f) follow.scroll.scrollBy(by)
                } while (!settled)
            }
        }
    }
    // The picker's full height as it was last laid out, so the head can ease in step with its
    // unfolding (below); written while the picker is measured, read in the same measure pass.
    val unfolded = remember { IntArray(1) }
    // The hours prayed on this day, marked again when one is prayed.
    val context = LocalContext.current
    val prayed = remember(date, Prayed.version) { Prayed.hours(context, date) }
    val day = dayColor(view.color, p)
    // The lining's inner line is the day's colour, beside the cross on the plaster; a white day's
    // would be tan there, and takes the gold line by day.
    val liningDay = if (!p.dark && view.color == "white") p.goldLine else day
    val desk = tier.desk
    val home = homeInk(p)
    // By night the head is the apse's sky (the web's `.home-sky`), a phone's and the niche's alike.
    val night = p.dark
    val season = view.ornament
    val catch = leafCatch(season)
    val leaf = Leaf(o)
    val black = view.color == "black"
    val first = view.firstClass
    // The day's red work and muted ink, lifted on the sky to keep their contrast.
    val rubric = if (night) home.skyRubric else p.rubric
    val muted = if (night) home.skyMuted else p.muted
    // The frontal: the day's colour inside the invitation's frame; on a white day gold by day and
    // lime white by night on the sky, where the fillet is the one gold line; the lining thinned on
    // a bare black day.
    val frontal = when {
        black -> p.lining.copy(alpha = 0.8f)
        view.color == "white" -> if (night) home.limeWhite else p.goldLine
        else -> day
    }
    val pigment = ImageBitmap.imageResource(R.drawable.pigment)
    val wash = ImageBitmap.imageResource(R.drawable.sky_wash)
    // The painter's reserve round the day's words, which the sky's stars keep clear of.
    val reserve = remember { Reserve() }
    val side = tier.side
    Layout(
        contents = listOf<@Composable () -> Unit>(
            // The consecration cross in the point of the head, with clear air round it: painted
            // through the pigment, gilt on a first-class feast by day, leaf by night on the sky.
            {
                when {
                    night && black -> ConsecrationCross(Modifier.size(tier.crownSize), ink = muted.copy(alpha = 0.7f))
                    night -> ConsecrationCross(Modifier.size(tier.crownSize), brush = Brush.linearGradient(0f to leaf.hi, 0.55f to leaf.mid, 1f to leaf.lo), alpha = 0.35f + 0.65f * catch)
                    first -> ConsecrationCross(Modifier.size(tier.crownSize), brush = giltBrush(o))
                    else -> ConsecrationCross(Modifier.size(tier.crownSize), pigment = pigment)
                }
            },
            // A lining painted round the head on the back wall, down to the inscription band; by
            // night on a phone, the apse's sky with its stars, ringed by a leaf fillet and the day's
            // hairline.
            {
                Spacer(
                    Modifier.drawBehind {
                        if (night) {
                            val crown = tier.crownSize.toPx()
                            val pad = 11.dp.toPx()
                            val clear = reserve.rects.values.flatten().map { Rect(it.left - 14.dp.toPx(), it.top - 6.dp.toPx(), it.right + 14.dp.toPx(), it.bottom + 6.dp.toPx()) } +
                                Rect((size.width - crown) / 2f - pad, tier.crownTop.toPx() - pad, (size.width + crown) / 2f + pad, tier.crownTop.toPx() + crown + pad)
                            sky(tier.arch, size.height, wash, home, leaf, catch, stars = !black, first = first, reserve = clear)
                            if (!black) skyLining(tier.arch, leaf, catch, if (view.color == "white") home.limeWhite else day, first)
                        } else {
                            // A first-class niche by day gilds one fillet round its head, to the frieze.
                            if (desk && first) giltHead(tier.arch, o)
                            lining(tier.arch, tier.lining.toPx(), p.lining, liningDay, pigment)
                        }
                    },
                )
            },
            // The day.
            {
                Column(Modifier.fillMaxWidth().padding(horizontal = tier.dayClear), horizontalAlignment = Alignment.CenterHorizontally) {
                    val dateModifier = Modifier.semantics(mergeDescendants = true) { heading() }.tap(action = "open the ordo", onClick = onOrdoDay)
                    val at = view.dateLabel.indexOf(", ")
                    if (at < 0) {
                        Text(
                            unbroken(view.dateLabel),
                            dateModifier.widthIn(max = tier.dateMeasure),
                            style = Type.body.copy(fontSize = tier.date.sp, lineHeight = (tier.date * 1.2f).sp, letterSpacing = tier.dateTracking.sp, color = p.text, textAlign = TextAlign.Center),
                            onTextLayout = reserve.layout("date"),
                        )
                    } else {
                        // The weekday is a titulus over the rest of the date, on a phone the two a full
                        // thumb's height at least, as the web's, the rest at the foot so the feast
                        // stands close under it. A reader hears "Tuesday, November 10, 2026".
                        val heavy = if (desk) tier.heavy else view.commemorations.size > 1 || !view.isToday
                        Column(
                            if (desk) dateModifier else dateModifier.heightIn(min = 44.dp).wrapContentHeight(Alignment.Bottom),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            val weekday = tier.weekday()
                            Text(
                                view.dateLabel.substring(0, at),
                                reserve.at("weekday").semantics { contentDescription = view.dateLabel.substring(0, at + 1) },
                                style = Type.body.copy(
                                    fontSize = weekday.sp,
                                    lineHeight = (weekday * tier.weekdayLine(heavy)).sp,
                                    letterSpacing = (weekday * tier.weekdayTracking).sp,
                                    fontFeatureSettings = ALL_SMALL_CAPS,
                                    color = p.titulus,
                                    textAlign = TextAlign.Center,
                                ),
                                onTextLayout = reserve.layout("weekday"),
                            )
                            val rest = tier.rest(heavy)
                            Text(
                                unbroken(view.dateLabel.substring(at + 2)),
                                reserve.at("date"),
                                style = Type.body.copy(
                                    fontSize = rest.sp,
                                    lineHeight = (rest * tier.restLine(heavy)).sp,
                                    letterSpacing = (if (desk) tier.dateTracking else rest * 0.005f).sp,
                                    fontFeatureSettings = "onum, pnum",
                                    color = p.text,
                                    textAlign = TextAlign.Center,
                                ),
                                onTextLayout = reserve.layout("date"),
                            )
                        }
                    }
                    // The day's feast leads to its page, underlined as the web's `.feast-link`, though in
                    // its own ink: a text decoration takes no colour.
                    FeastName(
                        view.feastName,
                        view.feastAlias,
                        tier.feast,
                        Modifier.padding(top = 2.dp).widthIn(max = tier.feastMeasure)
                            .then(if (view.feastId.isEmpty()) Modifier else Modifier.tap(action = "open the feast's page") { onFeast(view.feastId) }),
                        linked = view.feastId.isNotEmpty(),
                        reserve = reserve,
                    )
                    if (view.octaveNote.isNotEmpty()) Text(view.octaveNote, reserve.at("octave"), style = Type.small.copy(color = muted), onTextLayout = reserve.layout("octave"))
                    if (view.penitential.isNotEmpty()) {
                        Row(Modifier.padding(top = if (desk && tier.heavy) 0.dp else 12.dp), horizontalArrangement = Arrangement.spacedBy(9.6.dp)) {
                            val fast = if (desk) Type.small.copy(fontSize = tier.fast.sp) else Type.small
                            view.penitential.forEachIndexed { i, it -> Text(it, reserve.at("fast$i"), style = fast.copy(color = rubric, fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 0.75.sp), onTextLayout = reserve.layout("fast$i")) }
                        }
                    }
                    if (view.commemorations.isNotEmpty()) {
                        Column(Modifier.padding(top = 8.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                            Text("ALSO", reserve.at("also"), style = Type.label(10.56f, 0.1f).copy(color = muted), onTextLayout = reserve.layout("also"))
                            // A second commemoration is joined to the first with "and", as the web's
                            // `.commemorations li + li::before`, so the list reads as a sentence.
                            view.commemorations.forEachIndexed { i, name ->
                                val id = view.commemorationIds.getOrElse(i) { "" }
                                val text = buildAnnotatedString {
                                    if (i > 0) withStyle(SpanStyle(fontStyle = FontStyle.Italic)) { append("and ") }
                                    withStyle(SpanStyle(textDecoration = if (id.isEmpty()) null else TextDecoration.Underline)) { append(name) }
                                }
                                Text(
                                    text,
                                    reserve.at("also$i").then(if (id.isEmpty()) Modifier else Modifier.tap(action = "open the feast's page") { onFeast(id) }),
                                    style = Type.small.copy(color = p.text, textAlign = TextAlign.Center, fontSize = tier.commemoration.sp, lineHeight = (tier.commemoration * tier.commemorationLine).sp),
                                    onTextLayout = reserve.layout("also$i"),
                                )
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
                            VersicleLine("℣.", view.versicle, style, rubric, reserve, "versicle")
                            VersicleLine("℟.", view.response, style, rubric, reserve, "response")
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
                            Modifier.heightIn(min = 44.dp).tap { onDate(today) }.padding(vertical = 12.dp).then(reserve.at("today")).goldUnderline(true, p.goldLine),
                            style = Type.menu.copy(color = p.accent),
                            onTextLayout = reserve.layout("today"),
                        )
                    }
                }
            },
            // The inscription band, at the head of the stage.
            {
                Column(Modifier.fillMaxWidth().padding(horizontal = side), horizontalAlignment = Alignment.CenterHorizontally) {
                    // The inscription band: gilt letters on the frieze's green earth, between
                    // oxblood rules each with a gilt fillet inside it, its phrase parted from the
                    // frame by gilt lozenges 5dp square.
                    // The inscription band: flat green earth between oxblood edges and leaf
                    // fillets, ruled by hand (the web's `rule()`), its letters leaf that catches the
                    // light at their heads.
                    Row(
                        Modifier
                            .throughPadding(side)
                            .background(p.inscriptionGround)
                            .drawBehind {
                                val u = density
                                val w = size.width
                                val h = size.height
                                drawPath(ruledLine(0f, 0.6f * u, w, 0.6f * u, 1.3f * u, 11, u), p.inscriptionEdge)
                                drawPath(ruledLine(0f, h - 0.6f * u, w, h - 0.6f * u, 1.3f * u, 12, u), p.inscriptionEdge)
                                val fillet = o.ink.copy(alpha = 0.3f)
                                drawPath(ruledLine(0f, 2.2f * u, w, 2.2f * u, 0.9f * u, 13, u), fillet)
                                drawPath(ruledLine(0f, h - 2.2f * u, w, h - 2.2f * u, 0.9f * u, 14, u), fillet)
                            }
                            .padding(vertical = tier.bandPad),
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        val letters = Brush.verticalGradient(0.2f to mix(o.ink, Color.White, 0.7f), 0.58f to o.ink)
                        Lozenged("Pray the hours", Type.label(tier.band, tier.bandTracking).copy(brush = letters, fontFeatureSettings = ALL_SMALL_CAPS), o.ink)
                    }
                }
            },
            // The stage's first face: the invitation and the hours, which "Change date" turns into
            // the calendar in their place (the web's `.home-hours-face`).
            {
                Column(
                    Modifier.fillMaxWidth().padding(horizontal = side).then(if (picking) Modifier.clearAndSetSemantics {} else Modifier),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Spacer(Modifier.height(tier.bandGap))
                    PrayNow(view.prayNowLabel, tier.pray, tier.prayTop, tier.prayBottom, frontal, if (black) Color.Transparent else home.tabletWash, if (night) home.liningLifted else null) { onHour(LocalDate.of(view.prayNowDate.year, view.prayNowDate.month, view.prayNowDate.day), view.prayNowHour) }
                    Spacer(Modifier.height(tier.prayGap))
                    HourDirectory(view.currentHour, view.currentHourNote, prayed, tier, titulusNow(p, season), home) { onHour(date, it) }
                }
            },
            // The stage's second face: the day's turn and the calendar, always laid out so its
            // height is known before it is turned to; shown only as far as the turn has opened the
            // stage, clipped to it.
            {
                Box(
                    Modifier.fillMaxWidth().padding(horizontal = side).clipToBounds()
                        .then(if (picking) Modifier else Modifier.clearAndSetSemantics {})
                        .layout { measurable, constraints ->
                            val placeable = measurable.measure(constraints.copy(minHeight = 0, maxHeight = Constraints.Infinity))
                            unfolded[0] = placeable.height
                            layout(placeable.width, minOf(placeable.height, constraints.maxHeight)) { placeable.place(0, 0) }
                        },
                    contentAlignment = Alignment.TopCenter,
                ) { DatePicker(date, today, centred = true) { picking = false; onDate(it) } }
            },
            // The season and "Change date", under the stage.
            {
                Column(Modifier.fillMaxWidth().padding(horizontal = side), horizontalAlignment = Alignment.CenterHorizontally) {
                    // Season and date control share one line after the hours (`.home-day-meta`), in
                    // small caps, a gilt point between them, open or shut: the calendar opens in the
                    // hours' place above, not under the control.
                    Spacer(Modifier.height(tier.metaGap + 1.dp))
                    val seasonStyle = Type.label(tier.meta, 0.09f).copy(color = p.muted, fontFeatureSettings = ALL_SMALL_CAPS)
                    // One row throughout, so the control TalkBack just activated is the one that stays
                    // (the web's one <summary>).
                    FlowRow(horizontalArrangement = Arrangement.Center, verticalArrangement = Arrangement.Center) {
                        // A reader's first week: the introduction stands in the season's place
                        // (each hour's header still names it), in the same small capitals and with
                        // the same gilt point after it, and opens About.
                        val lead = introduction ?: view.season
                        if (lead.isNotEmpty()) {
                            val leadModifier = if (introduction == null) Modifier else Modifier.heightIn(min = 44.dp).tap(action = "open About the Office", onClick = onIntroduction)
                            Row(Modifier.align(Alignment.CenterVertically).then(leadModifier), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                                Text(lead, style = seasonStyle.copy(textAlign = TextAlign.Center))
                                Canvas(Modifier.padding(start = 14.4.dp, end = 8.8.dp).size(5.dp)) { lozenge(center, 2.5.dp.toPx(), o.flat.copy(alpha = 0.7f), null) }
                            }
                        }
                        Disclosure("Change date", picking, { picking = !picking }, modifier = Modifier.align(Alignment.CenterVertically), style = seasonStyle, upper = false)
                    }
                }
            },
        ),
        modifier = modifier.onGloballyPositioned { reserve.root = it; follow?.target = it }.drawBehind {
            if (niche != null && !p.dark) {
                // The niche by day: a lancet cut into the wall, its depth told by light on the
                // reveal, the day's colour one fillet at the back wall's edge. By night it is the
                // phone's panel, slate course and sky.
                niche(tier.arch, niche, p, mix(day, niche.revealMid, 0.78f), panelPlaster)
            } else {
                // The panel: the day's colour a little toward the frame as a ring at its edge, so
                // a red or green day edges the head without outshouting the cross.
                panel(tier.arch, p, if (night) home.skyRim else mix(day, frame, 0.7f), frame, panelPlaster)
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
        val band = slots[5].first().measure(full)
        val hours = slots[6].first().measure(full)
        // The calendar as far as the turn has opened the stage: its full height comes from its own
        // measure (`unfolded`), which it takes whatever it is offered.
        val turned = turn.value
        val stage = hours.height + ((unfolded[0] - hours.height) * turned).roundToInt()
        val chooser = slots[7].first().measure(Constraints(minWidth = width, maxWidth = width, minHeight = 0, maxHeight = if (turned > 0f) stage else 0))
        val meta = slots[8].first().measure(full)
        val after = band.height + meta.height
        val headPad = tier.headPad.roundToPx()
        val foot = tier.bottom.roundToPx()
        val day = facts.height + (back?.height ?: 0) + tier.dayGap.roundToPx()
        val rise = (tier.arch.rise * width).roundToInt()
        // The day under the head's room for the cross, and the band no higher than the springing,
        // so it never crosses the arch; on a phone, the height the card is given beyond its own.
        val shut = maxOf(headPad + day, rise, constraints.minHeight - after - hours.height - foot)
        // The versicle takes only the height the head has to spare: where it would make the card
        // taller, the head goes without it. It is fitted to the card with the hours shown, as the
        // web's is, so turning to the calendar never takes it away.
        val verse = versicle?.takeIf { headPad + day + it.height <= shut }
        // "Change date" turns the stage from the hours to the calendar, which is taller. Where the
        // head's spare height holds the difference, it gives it up in step with the turn, so the
        // day rises as the calendar opens and nothing scrolls; where it does not, the head keeps its
        // height, the card lengthens, and the page rises with its foot (`Follow`), the arch never
        // pressed down for a scroll that comes anyway (the web's and iOS's alike).
        val floor = maxOf(headPad + day + (verse?.height ?: 0), rise)
        val room = constraints.minHeight - after - unfolded[0] - foot
        val open = if (room >= floor) room else shut
        val summary = shut - ((shut - open) * turned).roundToInt()
        val spare = summary - headPad - day - (verse?.height ?: 0)
        val wall = walls.first().measure(Constraints.fixed(width, summary))
        val height = summary + after + stage + foot
        // The hours fade out over the turn's first half, the calendar in over all but its start.
        val hoursAlpha = (1f - turned / 0.46f).coerceIn(0f, 1f)
        val chooserAlpha = ((turned - 0.23f) / 0.77f).coerceIn(0f, 1f)
        layout(width, height) {
            wall.place(0, 0)
            mark.place((width - mark.width) / 2, tier.crownTop.roundToPx())
            var y = headPad + if (tier.split) spare * 2 / 5 else spare / 2
            facts.place(0, y)
            y += facts.height
            verse?.let { it.place(0, y); y += it.height }
            back?.place(0, y)
            band.place(0, summary)
            if (turned < 1f) hours.placeWithLayer(0, summary + band.height) { alpha = hoursAlpha }
            if (turned > 0f) chooser.placeWithLayer(0, summary + band.height) { alpha = chooserAlpha }
            meta.place(0, summary + band.height + stage)
        }
    }
}

/**
 * The feast's name broken into balanced lines, as the web's, and its familiar name in italic: beside
 * it where the line has room, else on a line of its own, never broken. A reader hears the two
 * together.
 */
@Composable
private fun FeastName(name: String, alias: String, size: Float, modifier: Modifier, linked: Boolean = false, reserve: Reserve? = null) {
    val p = LocalPalette.current
    val style = Type.body.copy(
        fontSize = size.sp,
        lineHeight = (size * 1.25f).sp,
        color = p.accent,
        textAlign = TextAlign.Center,
        lineBreak = LineBreak.Heading,
        textDecoration = if (linked) TextDecoration.Underline else null,
    )
    FlowRow(modifier.semantics(mergeDescendants = true) {}, horizontalArrangement = Arrangement.spacedBy((size * 0.25f).dp, Alignment.CenterHorizontally)) {
        Text(name, reserve?.at("feast") ?: Modifier, style = style, onTextLayout = reserve?.layout("feast") ?: {})
        if (alias.isNotEmpty()) Text(alias, reserve?.at("alias") ?: Modifier, style = style.copy(fontStyle = FontStyle.Italic), softWrap = false, onTextLayout = reserve?.layout("alias") ?: {})
    }
}

/** A line of the day's versicle: its ℣ or ℟ upright in the rubrics' red (`rubric`), the words in italic, the text's ink a little withdrawn. */
@Composable
private fun VersicleLine(sigil: String, words: String, size: Float, rubric: Color, reserve: Reserve, key: String) {
    val p = LocalPalette.current
    Text(
        buildAnnotatedString {
            withStyle(SpanStyle(fontStyle = FontStyle.Normal, color = rubric)) { append(sigil) }
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
        modifier = reserve.at(key),
        onTextLayout = reserve.layout(key),
    )
}

/**
 * The painter's reserve round the day's words (the web's, from `Range.getClientRects`): each
 * laid-out line of the day block, by its text's layout, in the card's space (`root`). The sky's
 * stars keep clear of it. Written as the text is placed, read only when the sky is drawn.
 */
private class Reserve {
    var root: LayoutCoordinates? = null
    val rects = mutableStateMapOf<String, List<Rect>>()
    private val layouts = HashMap<String, TextLayoutResult>()

    fun layout(key: String): (TextLayoutResult) -> Unit = { layouts[key] = it }

    fun at(key: String): Modifier = Modifier.onGloballyPositioned { c ->
        val r = root?.takeIf { it.isAttached } ?: return@onGloballyPositioned
        val l = layouts[key] ?: return@onGloballyPositioned
        val o = r.localPositionOf(c, Offset.Zero)
        val lines = (0 until l.lineCount).filter { l.getLineRight(it) - l.getLineLeft(it) > 1f }.map {
            Rect(o.x + l.getLineLeft(it), o.y + l.getLineTop(it), o.x + l.getLineRight(it), o.y + l.getLineBottom(it))
        }
        if (rects[key] != lines) rects[key] = lines
    }
}

/** Runs through the card's side padding to the frame's inner edges, as the inscription band does. */
private fun Modifier.throughPadding(side: Dp): Modifier = this.layout { measurable, constraints ->
    val extra = (side * 2).roundToPx()
    val placeable = measurable.measure(constraints.copy(minWidth = constraints.maxWidth + extra, maxWidth = constraints.maxWidth + extra))
    layout(constraints.maxWidth, placeable.height) { placeable.place(-extra / 2, 0) }
}

/**
 * The invitation: a thin wash (`tablet.png`, feathered so no edge of it lies on a rule) in a
 * frame ruled by hand twice, the lining and inside it the day's colour (`frontal`) stopping short
 * of the corners; neither veils with the season, and a bare black day has no wash. Its words are
 * the tituli's red ochre, or by night the lining, lifted on a phone (`lifted`). About's foot sets
 * the same tablet; on home `size`, `top` and `bottom` are the tier's.
 */
@Composable
fun PrayNow(label: String, size: Float, top: Dp, bottom: Dp, frontal: Color, wash: Color, lifted: Color?, onClick: () -> Unit) {
    val p = LocalPalette.current
    val tablet = ImageBitmap.imageResource(R.drawable.tablet)
    Box(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 44.dp)
            .tap(onClick = onClick)
            .drawBehind {
                tabletWash(tablet, wash)
                invitationRules(p.lining, frontal)
            }
            .padding(top = top, bottom = bottom, start = 13.8.dp, end = 13.8.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, style = Type.body.copy(fontSize = size.sp, lineHeight = (size * 1.3f).sp, letterSpacing = (size * 0.012f).sp, color = lifted ?: if (p.dark) p.lining else p.titulus, textAlign = TextAlign.Center))
    }
}

/**
 * The hours by period in horizontal bands, ruled as a fine book rules a table (the web's
 * `.home-hour-links`): a firmer rule above and below and finer ones between the periods, by hand,
 * in the lining, and none upright. The periods are the margin's index in the quiet ink, with no
 * cell of their own; gilt points part the hours in a band. The hour that is now lies under a pool
 * of light with no edge, its name rubricated (`now`) and its gloss under it. In the wide niche
 * every hour carries its gloss and the hour that is now is set larger as well, so it is not told by
 * colour alone; from 1000 high the hours stand one to a line there ([HourList]).
 */
@Composable
private fun HourDirectory(current: String, note: String, prayed: Set<String>, tier: HomeTier, now: Color, home: HomeInk, onHour: (String) -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val desk = tier.desk
    val labelStyle = Type.label(tier.period, tier.periodTracking).copy(color = if (desk) mix(p.accent, p.muted, 0.7f) else mix(p.accent, p.muted, 0.45f), fontFeatureSettings = ALL_SMALL_CAPS)
    // One width for the three period labels, widened past the web's 83dp only when the reader's
    // font size needs it, so the hours still line up in columns.
    val measurer = rememberTextMeasurer()
    val density = LocalDensity.current
    val labelWidth = remember(labelStyle, density) {
        with(density) { PERIODS.maxOf { measurer.measure(it.second, labelStyle).size.width }.toDp() + 8.dp }.coerceAtLeast(if (desk) 80.dp else 83.dp)
    }
    // Where the periods meet, for the rules between them.
    val joints = remember { FloatArray(2) }
    Column(
        Modifier.fillMaxWidth().drawBehind {
            val u = 1.dp.toPx()
            val w = size.width
            val h = size.height
            val frame = p.lining.copy(alpha = 0.78f)
            drawPath(ruledLine(0f, 0.5f * u, w, 0.5f * u, 1.3f * u, 31, u), frame)
            drawPath(ruledLine(0f, h - 0.5f * u, w, h - 0.5f * u, 1.3f * u, 32, u), frame)
            joints.forEachIndexed { i, y -> if (y > 0f) drawPath(ruledLine(0f, y + 0.5f * u, w, y + 0.5f * u, 0.9f * u, 41 + i, u), p.lining.copy(alpha = 0.36f)) }
        },
    ) {
        PERIODS.forEachIndexed { i, (period, label, hours) ->
            if (tier.list) {
                HourList(i, period, label, hours, current, prayed, tier, now, home, labelStyle, joints, onHour)
                return@forEachIndexed
            }
            Row(
                Modifier.fillMaxWidth().height(IntrinsicSize.Min).padding(top = 1.dp)
                    .then(if (i > 0) Modifier.onGloballyPositioned { joints[i - 1] = it.positionInParent().y - 1f } else Modifier),
            ) {
                Column(
                    Modifier.width(labelWidth).fillMaxHeight().padding(vertical = 6.dp).padding(start = 1.4.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center,
                ) {
                    PeriodIcon(period, o.flat, modifier = if (desk) Modifier.scale(1.12f) else Modifier, alpha = 0.9f)
                    Text(label, Modifier.padding(top = 2.dp), softWrap = false, style = labelStyle)
                }
                Spacer(Modifier.width(1.dp))
                Row(Modifier.weight(1f).heightIn(min = tier.row), verticalAlignment = Alignment.CenterVertically) {
                    hours.forEachIndexed { j, h ->
                        val done = h in prayed
                        val isNow = h == current
                        // A phone glosses the hour that is now; the niche every hour.
                        val gloss = if (desk) hourNote(h) else if (isNow) note else ""
                        val said = listOfNotNull(hourLabel(h), gloss.ifEmpty { null }, "now".takeIf { isNow }, "prayed".takeIf { done })
                        // The light rests on the hour that is now: a pool with no edge, at most
                        // 9.5rem across (11rem in the niche).
                        val pool = if (isNow) Modifier.drawBehind {
                            val r = minOf(size.width + (if (desk) 12.dp else 8.dp).toPx(), (if (desk) 176.dp else 152.dp).toPx()) / 2f
                            val ry = (size.height + (if (desk) 8.dp else 4.dp).toPx()) / 2f
                            scale(1f, ry / r, pivot = center) {
                                drawCircle(poolBrush(home.nowPool, center, r), r, center)
                            }
                        } else Modifier
                        Box(
                            Modifier.weight(1f).fillMaxHeight().then(pool).drawBehind {
                                // A gilt point before every hour but the first of its band.
                                if (j > 0) rotate(45f, Offset(0f, size.height / 2f)) {
                                    val d = 3.5.dp.toPx()
                                    drawRect(o.flat.copy(alpha = 0.7f), Offset(-d / 2f, size.height / 2f - d / 2f), Size(d, d))
                                }
                            }.tap(label = if (said.size > 1) said.joinToString(", ") else null) { onHour(h) },
                            contentAlignment = Alignment.Center,
                        ) {
                            // An hour prayed today withdraws to the muted ink, a small gilt cross at its
                            // shoulder, as the web's `.is-prayed`; the cross takes no width.
                            val ink = if (done) p.muted else if (isNow) now else p.text
                            val size = if (isNow) tier.hourNow else tier.hour
                            val name = Type.body.copy(fontSize = size.sp, lineHeight = (size * 1.2f).sp, letterSpacing = 0.31.sp, color = ink)
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Box {
                                    // Never broken mid-word: at the largest font sizes a name steps down to fit its cell.
                                    BasicText(
                                        hourLabel(h),
                                        Modifier.padding(horizontal = 2.dp),
                                        style = name,
                                        maxLines = 1,
                                        autoSize = TextAutoSize.StepBased(minFontSize = 11.sp, maxFontSize = name.fontSize),
                                    )
                                    if (done) PaintedCross(Modifier.align(Alignment.TopEnd).offset(x = 9.dp, y = 2.dp).size(7.dp), ink = p.gold)
                                }
                                // What the hour is, for a newcomer, as the web's `.home-hour-link-note`;
                                // one line, which may overhang its cell a little.
                                if (gloss.isNotEmpty()) {
                                    // A short screen's gloss, held at 12 in so small a niche, takes a
                                    // second line rather than running into its neighbour.
                                    Text(gloss, style = Type.small.copy(fontSize = tier.gloss.sp, lineHeight = (tier.gloss * 1.15f).sp, color = if (isNow) home.nowGloss else p.muted, fontStyle = FontStyle.Italic, textAlign = TextAlign.Center), maxLines = if (tier.shortScreen) 2 else 1, softWrap = tier.shortScreen)
                                }
                            }
                        }
                    }
                }
            }
        }
        Spacer(Modifier.height(1.dp))
    }
}

/**
 * One period of the niche's list of seven (the web's from 1000px high upright, 1146px on its
 * side): its label in the margin beside its hours, and each hour on a line of its own, its name and
 * its gloss on one baseline parted by a gilt point; the label and the hours, read as one block,
 * stand centred in the niche ([NicheLayout.listMiddle]).
 */
@Composable
private fun HourList(
    i: Int,
    period: Period,
    label: String,
    hours: List<String>,
    current: String,
    prayed: Set<String>,
    tier: HomeTier,
    now: Color,
    home: HomeInk,
    labelStyle: androidx.compose.ui.text.TextStyle,
    joints: FloatArray,
    onHour: (String) -> Unit,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    Layout(
        contents = listOf(
            {
                Column(Modifier.padding(end = 22.4.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    PeriodIcon(period, o.flat, modifier = Modifier.scale(1.25f), alpha = 0.9f)
                    Text(label, Modifier.padding(top = 4.dp), softWrap = false, style = labelStyle)
                }
            },
            {
                Column(Modifier.padding(vertical = 4.8.dp)) {
                    hours.forEach { h ->
                        val done = h in prayed
                        val isNow = h == current
                        val gloss = hourNote(h)
                        val said = listOfNotNull(hourLabel(h), gloss.ifEmpty { null }, "now".takeIf { isNow }, "prayed".takeIf { done })
                        // The pool of light under the hour that is now, from a little before its name.
                        val pool = if (isNow) Modifier.drawBehind {
                            val r = 136.dp.toPx()
                            val c = Offset(r - 19.2.dp.toPx(), size.height / 2f)
                            val ry = (size.height + 6.dp.toPx()) / 2f
                            scale(1f, ry / r, pivot = c) {
                                drawCircle(poolBrush(home.nowPool, c, r), r, c)
                            }
                        } else Modifier
                        Row(
                            Modifier.fillMaxWidth().heightIn(min = tier.row).then(pool).tap(label = said.joinToString(", ")) { onHour(h) },
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            val ink = if (done) p.muted else if (isNow) now else p.text
                            val size = if (isNow) tier.hourNow else tier.hour
                            Box(Modifier.width(100.8.dp)) {
                                Text(hourLabel(h), style = Type.body.copy(fontSize = size.sp, lineHeight = (size * 1.2f).sp, letterSpacing = 0.31.sp, color = ink), maxLines = 1, softWrap = false)
                                if (done) PaintedCross(Modifier.align(Alignment.TopEnd).offset(x = (-6).dp, y = 2.dp).size(7.dp), ink = p.gold)
                            }
                            Canvas(Modifier.padding(end = 13.6.dp).size(5.dp)) { lozenge(center, 1.75.dp.toPx(), o.flat.copy(alpha = 0.7f), null) }
                            Text(gloss, style = Type.small.copy(fontSize = tier.gloss.sp, lineHeight = (tier.gloss * 1.15f).sp, color = if (isNow) home.nowGloss else p.muted, fontStyle = FontStyle.Italic), maxLines = 1, softWrap = false)
                        }
                    }
                }
            },
        ),
        modifier = Modifier.fillMaxWidth().padding(top = 1.dp)
            .then(if (i > 0) Modifier.onGloballyPositioned { joints[i - 1] = it.positionInParent().y - 1f } else Modifier),
    ) { (labels, lists), constraints ->
        // grid-template-columns: 1fr middle 1fr, the label in the first column, the hours across
        // the other two.
        val width = constraints.maxWidth
        val middle = tier.listMiddle.roundToPx()
        val side = ((width - middle) / 2).coerceAtLeast(0)
        val list = lists.first().measure(Constraints(maxWidth = (width - side).coerceAtLeast(0)))
        val tag = labels.first().measure(Constraints(maxWidth = side))
        val height = maxOf(list.height, tag.height)
        layout(width, height) {
            tag.place(side - tag.width, (height - tag.height) / 2)
            list.place(side, 0)
        }
    }
}
