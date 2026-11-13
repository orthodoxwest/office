package org.orthodoxwest.office

import android.graphics.Bitmap
import android.graphics.Matrix
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.runtime.withFrameNanos
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Canvas
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.ImageShader
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.ShaderBrush
import androidx.compose.ui.graphics.TileMode
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.drawscope.CanvasDrawScope
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.layer.GraphicsLayer
import androidx.compose.ui.graphics.layer.drawLayer
import androidx.compose.ui.graphics.rememberGraphicsLayer
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.PointerEventType
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onGloballyPositioned
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.layout.positionInRoot
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.unit.toSize
import kotlin.math.abs
import kotlin.math.atan2
import kotlin.math.ceil
import kotlin.math.floor
import kotlin.math.hypot
import kotlin.math.max
import kotlin.math.min
import kotlin.math.pow
import kotlin.math.roundToInt
import kotlin.math.sqrt
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import org.orthodoxwest.office.core.CrownEase
import org.orthodoxwest.office.core.CrownPlan
import org.orthodoxwest.office.core.CrownRgb
import org.orthodoxwest.office.core.afterCrownTap
import org.orthodoxwest.office.core.crownPlan

/*
 * Home's crown: a tap on the consecration roundel at the head of the arch turns the look over,
 * Nave to Apse or Apse to Nave (presentation::crown::after_crown_tap), and the change plays as
 * dusk or dawn: the web's "Home: the crown's play" (app.js and style.css), drawn natively. The
 * play's timetable, eases and pigments come from presentation through the core (`crownPlan`);
 * the first act's own figures (the cross, the light on the horizon, the stars) are style.css's,
 * mirrored here as app.js mirrors the plan.
 *
 * How it is put together. The whole screen is recorded into one layer each frame ([crowning]).
 * At the tap the cross is lifted out of the screen and drawn by the play in its two looks. At
 * the turn (the plan's `turn`) every unit of the page that hands its ink over (the day's lines,
 * the frieze, the rows: [crownUnit]) is lifted too, its old look kept as an image, the screen as
 * it stands is kept as a still, and the theme changes under it. From then on the new screen is
 * drawn live over the still: through a front crossing the head of the arch (down from above the
 * point at dusk, up from under the frieze at dawn), and through the arch's own shape grown over
 * the page with a wide feather and a coloured veil; each lifted unit hands its old ink to its new
 * at its own moment. When it ends nothing is left: the screen is the live, themed screen.
 */

/** The crown's play at the app's root, for home to reach; none in tests that do not ask for it. */
val LocalCrown = staticCompositionLocalOf<Crown?> { null }

/** The crown while home is composed: only home's own parts (its header included) are its units. */
internal val LocalCrownStage = staticCompositionLocalOf<Crown?> { null }

/** A cubic-bezier ease, as CSS writes it. */
internal class Bezier(private val x1: Float, private val y1: Float, private val x2: Float, private val y2: Float) {
    constructor(e: CrownEase) : this(e.x1, e.y1, e.x2, e.y2)

    private fun axis(a: Float, b: Float, t: Float) = 3 * a * t * (1 - t) * (1 - t) + 3 * b * t * t * (1 - t) + t * t * t

    /** The eased progress at time x (0..1). */
    fun at(x: Float): Float {
        if (x <= 0f) return 0f
        if (x >= 1f) return 1f
        var lo = 0f
        var hi = 1f
        repeat(24) {
            val mid = (lo + hi) / 2
            if (axis(x1, x2, mid) < x) lo = mid else hi = mid
        }
        return axis(y1, y2, (lo + hi) / 2)
    }

    /** The time (0..1) at which the eased progress is p. */
    fun timeAt(p: Float): Float {
        if (p <= 0f) return 0f
        if (p >= 1f) return 1f
        var lo = 0f
        var hi = 1f
        repeat(24) {
            val mid = (lo + hi) / 2
            if (axis(y1, y2, mid) < p) lo = mid else hi = mid
        }
        return axis(x1, x2, (lo + hi) / 2)
    }
}

private fun clamp01(x: Float) = x.coerceIn(0f, 1f)

private fun smooth(a: Float, b: Float, x: Float): Float {
    val t = clamp01((x - a) / (b - a))
    return t * t * (3 - 2 * t)
}

/** Keyframes on one value: (offset, value), each stretch eased by its own curve (CSS's per-keyframe timing). */
private class Keys(private val offsets: FloatArray, private val values: FloatArray, private val eases: Array<Bezier?>) {
    fun at(x: Float): Float {
        if (x <= offsets.first()) return values.first()
        if (x >= offsets.last()) return values.last()
        val i = (0 until offsets.size - 1).first { x < offsets[it + 1] }
        val local = (x - offsets[i]) / (offsets[i + 1] - offsets[i])
        val e = eases[i]?.at(local) ?: local
        return values[i] + (values[i + 1] - values[i]) * e
    }
}

/** style.css's eases for the first act. */
private object Ease {
    val inOut = Bezier(0.4f, 0f, 0.4f, 1f)
    val catch = Bezier(0.4f, 0f, 0.6f, 1f)
    val rise = Bezier(0.3f, 0f, 0.3f, 1f)
    val warm = Bezier(0.3f, 0f, 0.5f, 1f)
    val kindle = Bezier(0.3f, 0.2f, 0.3f, 1f)
    val inkOut = Bezier(0.4f, 0f, 0.7f, 1f)
    val inkIn = Bezier(0.3f, 0f, 0.4f, 1f)
}

private fun rgb(c: CrownRgb, alpha: Float = 1f) = Color(c.r.toInt(), c.g.toInt(), c.b.toInt()).copy(alpha = alpha)

/**
 * A part of home that hands its ink over at a moment of its own while the crown plays: one of the
 * day's lines (`w-`), a row (`r-`), the frieze or the cross. It is recorded into a layer each
 * frame, and drawn by the play rather than in place while it is lifted.
 */
internal class CrownUnit(val layer: GraphicsLayer) {
    var at = Offset.Zero
    var size = IntSize.Zero
    var old: ImageBitmap? = null
    val rect get() = Rect(at, size.toSize())
}

/**
 * What home tells the crown of its frontispiece: where the card stands on the screen, its arch, how
 * far down the frieze is, the day's colour and the cross in either look.
 */
internal class CrownHead {
    var origin = Offset.Zero
    var width = 0f
    /** The frieze's top, from the card's top: the foot of the head. */
    var band = 0f
    var arch: Arch = PhoneArch
    var black = false
    var picking = false
    var tapBox = Rect.Zero
    /** The cross in the dark look (true) or the light, drawn to the scope's size. */
    var cross: DrawScope.(dark: Boolean) -> Unit = {}
    /** The leaf's highlight on the night, for the light the cross catches. */
    var catchInk: Color = Color.White
    /** A first-class feast's gilt cross by day. */
    var gilt = false
}

/** The stars of the head's sky, as home's sky drew them: for the play to draw on at dawn. */
internal class SkyStarPaint(val index: Int, val path: Path, val brush: Brush, val alpha: Float)

/**
 * The sky's say in the play: how lit each of its stars is (dusk kindles them, dawn puts them out)
 * and how large, and which it drew.
 */
class CrownSky internal constructor(private val crown: Crown) {
    internal var drawn: List<SkyStarPaint> = emptyList()

    /** Whether the play has a say in this frame (else every star is simply itself). */
    val active: Boolean get() = crown.play != null

    fun alpha(index: Int): Float = crown.starAlpha(index)

    fun scale(index: Int): Float = crown.starScale(index)

    internal fun record(stars: List<SkyStarPaint>) {
        drawn = stars
    }
}

private const val MID = 0.89f
private const val CORE = 0.78f
private const val NODES = 8

/** The sums for one play: no reads of the page after the tap. */
internal class Sums(val plan: CrownPlan, head: CrownHead, units: Map<String, CrownUnit>, root: IntSize, density: Float, wideDp: Boolean) {
    val dusk = plan.toDark
    val u = density
    val turn = plan.turn.toFloat()
    val frontAt0 = plan.front.at.toFloat()
    val frontLen = plan.front.len.toFloat()
    val spreadAt = plan.spread.at.toFloat()
    val veil0 = plan.veilFrom.toFloat()
    val veil1 = plan.veilTo.toFloat()
    val frontEase = Bezier(plan.frontEase)
    val spreadEase = Bezier(plan.spreadEase)
    val glowEase = Bezier(plan.glowEase)

    // ---- The vault: the head inside the slate course, where the sky lies by night. ----
    val W = head.width
    val origin = head.origin
    val arch = head.arch
    val vaultPath: Path = archPath(arch, W, -5f * u, head.band + 2f * u).apply { translate(origin) }
    val vault: Rect = vaultPath.getBounds().let { Rect(it.left, it.top, it.right, origin.y + head.band) }
    val apex = vault.top
    val hc = vault.bottom - apex
    val a: Float
    val cy: Float
    val r0: Float
    val ra: Float
    val rEnd: Float

    init {
        val w = vault.width
        if (dusk) {
            // About a point 0.6 of the head above its own, twice as wide as tall.
            a = 2f
            cy = apex - 0.6f * hc
            r0 = 0.6f * hc
            ra = (vault.bottom - cy) / MID
            rEnd = max(sqrt(w * w / (4 * a * a) + 1.6f * hc * 1.6f * hc) / CORE + 4 * u, ra + 1)
        } else {
            // A low dome about a point 0.3 of the head under the horizon.
            a = 2.5f
            cy = vault.bottom + 0.3f * hc
            r0 = 0.3f * hc
            ra = (cy - apex) / MID
            rEnd = max(1.3f * hc / CORE + 4 * u, ra + 1)
        }
    }

    /** When the front's middle is at a height y of the screen (ms from the tap). */
    fun frontAt(y: Float): Float = frontAt0 + 0.95f * frontLen * frontEase.timeAt((abs(y - cy) / MID - r0) / (ra - r0))

    /** The front's radius at a time. */
    fun frontRadius(t: Float): Float {
        val p = (t - frontAt0) / frontLen
        return when {
            p <= 0f -> r0
            p < 0.95f -> r0 + (ra - r0) * frontEase.at(p / 0.95f)
            p < 1f -> ra + (rEnd - ra) * (p - 0.95f) / 0.05f
            else -> rEnd
        }
    }

    // ---- The spread: the arch closed with a round foot, grown about the springing's middle. ----
    val fw = plan.spreadFeather
    val ax = origin.x + W / 2f
    val ys = origin.y + arch.rise * W
    private val F = 0.5f
    val s0 = 1f / (1f + fw)
    private val D = max(0.05f, (vault.bottom - ys) / (W * s0) - F * (1 + fw))
    private val turnAngle: Float
    private val archLen = hypot(arch.haunch - arch.centre, arch.depth)

    init {
        val h = arch.haunch
        val c = arch.centre
        val cyA = -arch.depth
        turnAngle = atan2(cyA + (-cyA) * arch.radius / archLen, c + (h - c) * arch.radius / archLen - h)
    }

    private fun left(x: Float, y: Float): Float {
        if (y <= 0f) return x
        val h = arch.haunch
        if (atan2(y, x - h) >= turnAngle) return h - hypot(x - h, y)
        return arch.radius - hypot(x - arch.centre, y + arch.depth)
    }

    /** How far inside the shape a point is, in card widths (x from the left springing, y up from it). */
    fun shape(x: Float, y: Float): Float = if (y >= -D) min(left(x, y), left(1 - x, y)) else 0.5f * (1 - hypot((x - 0.5f) / 0.5f, (y + D) / F))

    private val corners = listOf(Offset(0f, 0f), Offset(root.width.toFloat(), 0f), Offset(0f, root.height.toFloat()), Offset(root.width.toFloat(), root.height.toFloat()))
    val s1: Float = run {
        fun corner(sc: Float) = corners.minOf { shape((it.x - ax) / (W * sc) + 0.5f, (ys - it.y) / (W * sc)) }
        var s = 1f
        while (corner(s) < fw / 2 && s < 60f) s += 0.05f
        s * 1.02f
    }
    val dur = plan.spread.len.toFloat() + if (wideDp) plan.spreadWide.toFloat() else 0f
    val shapeDur = (dur * plan.spreadShapePart).roundToInt().toFloat()
    val vDur = (dur * plan.veilPart).roundToInt().toFloat()
    val part = plan.spreadGlowPart
    val m0 = fw / 2 + 0.012f
    val top = arch.rise + 2.4f * m0
    val deep = D + F * (1 + fw) + 0.02f
    val nodes = FloatArray(NODES + 1) { s0 * (s1 / s0).pow((it.toFloat() / NODES).pow(plan.spreadBend)) }
    private fun sizeAt(tau: Float): Float {
        val e = spreadEase.at(tau / dur)
        return 0.5f * s0 * (s1 / s0).pow(e) + 0.5f * (s0 + e * (s1 - s0))
    }
    val vnodes = FloatArray(NODES + 1) {
        val tn = vDur * it / NODES
        sizeAt(if (tn <= 400f) tn else 400f + (tn - 400f) * (dur - 400f) / (vDur - 400f)) * plan.veilLead
    }

    /** The page's scale and opacity at a time. */
    fun spreadScale(t: Float): Float = interpolate(nodes, (t - spreadAt) / shapeDur)
    fun glow(t: Float): Float = glowEase.at(clamp01((t - spreadAt) / dur) / part)
    fun veilScale(t: Float): Float = interpolate(vnodes, (t - spreadAt) / vDur)

    private fun interpolate(n: FloatArray, x: Float): Float {
        val f = clamp01(x) * NODES
        val i = floor(f).toInt().coerceIn(0, NODES - 1)
        return n[i] + (f - i) * (n[i + 1] - n[i])
    }

    fun alphaAt(x: Float, y: Float, t: Float): Float {
        val sc = spreadScale(t)
        return smooth(-fw / 2, fw / 2, shape((x - ax) / (W * sc) + 0.5f, (ys - y) / (W * sc))) * glow(t)
    }

    /** The first time the page under a point is `level` turned. */
    fun turnedAt(x: Float, y: Float, level: Float): Float {
        for (i in 0..120) {
            val t = spreadAt + dur * i / 120
            if (alphaAt(x, y, t) >= level) return t
        }
        return spreadAt + dur
    }

    /** The mask image: the shape feathered, small (the feather is wide), transparent at every edge. */
    val mask: ImageBitmap = run {
        val nx = 120
        val q = nx / (1 + 2 * m0)
        val ny = ceil((top + deep) * q).toInt()
        val px = IntArray(nx * ny)
        for (row in 0 until ny) {
            val y = top - (row + 0.5f) / q
            for (i in 0 until nx) {
                val alpha = (255 * smooth(-fw / 2, fw / 2, shape((i + 0.5f) / q - m0, y))).roundToInt()
                px[row * nx + i] = alpha shl 24
            }
        }
        Bitmap.createBitmap(px, nx, ny, Bitmap.Config.ARGB_8888).asImageBitmap()
    }

    /** A brush of the mask at a scale, placed about the springing's middle, clear beyond it. */
    fun maskBrush(sc: Float): Brush {
        val shader = ImageShader(mask, TileMode.Clamp, TileMode.Clamp)
        val w = (1 + 2 * m0) * W * sc
        val h = (top + deep) * W * sc
        shader.setLocalMatrix(
            Matrix().apply {
                setScale(w / mask.width, h / mask.height)
                postTranslate(ax - w / 2, ys - top * W * sc)
            },
        )
        return ShaderBrush(shader)
    }

    val covered: Float = listOf(
        Offset(0f, 0f), Offset(root.width.toFloat(), 0f), Offset(0f, root.height.toFloat()), Offset(root.width.toFloat(), root.height.toFloat()),
        Offset(root.width / 2f, root.height.toFloat()), Offset(root.width / 2f, 0f),
    ).maxOf { turnedAt(it.x, it.y, 0.96f) } + 40f

    // ---- The light on the horizon, under the last line of the day. ----
    val room: Float = run {
        val last = units.filterKeys { it.startsWith("w-") }.values.maxOfOrNull { it.at.y + it.size.height } ?: (vault.bottom - 0.34f * hc)
        max(0.13f * hc, min(0.5f * hc, vault.bottom - last - 4 * u))
    }
    val washRx = min(0.48f * vault.width, 2.6f * room)

    // ---- Each unit's moment. ----
    /** The day's lines dip as the head's front reaches them. */
    val words: Map<String, Float> = units.filterKeys { it.startsWith("w-") }.mapValues { (_, unit) -> frontAt(unit.at.y + unit.size.height * 0.6f) }

    /**
     * The rows change ink quickly, one after another, where the page under each can carry the ink
     * it is given, dealt out about the middle one 32ms apart and none further than 130ms from its
     * own moment: no frame with every label mid-change.
     */
    val rows: Map<String, Float>
    val lastDip: Float

    init {
        val order = units.filterKeys { it.startsWith("r-") }.map { (key, unit) ->
            val r = unit.rect
            key to turnedAt(r.center.x, r.center.y, plan.inkAt)
        }.sortedBy { it.second }
        val mid = (order.size - 1) / 2
        val base = if (order.isEmpty()) 0f else order[mid].second
        val dealt = LinkedHashMap<String, Float>()
        var previous = Float.NEGATIVE_INFINITY
        order.forEachIndexed { n, (key, own) ->
            var t = max(own - 130f, min(own + 130f, base + (n - mid) * 32f))
            if (n > 0 && t < previous + 25f) t = previous + 25f
            previous = t
            dealt[key] = t
        }
        rows = dealt
        lastDip = dealt.values.maxOrNull() ?: 0f
    }

    /** The play is settled when the page has visibly turned and the last row has its new ink. */
    val settle = maxOf(covered, lastDip + 220f, spreadAt + 1100f, veil1 - 80f, spreadAt + dur * part - 50f) + 80f
    val end = maxOf(plan.end.toFloat(), covered + 20f, settle + 90f)

    // ---- The stars: their turns. ----
    val stars = skyStars(arch)
    val starTurns: FloatArray = run {
        if (stars.isEmpty()) return@run FloatArray(0)
        val big = stars.maxOf { it.r }
        val small = stars.minOf { it.r }
        val cross = units["cross"]?.rect?.center ?: Offset(ax, apex)
        val at = stars.map { Offset(origin.x + it.x * W, origin.y + it.y * W) }
        val nearest = at.indices.minByOrNull { (at[it] - cross).getDistance() } ?: -1
        FloatArray(stars.size) { i ->
            val rank = if (big > small) (big - stars[i].r) / (big - small) else 0f
            val seed = ((i * 7919) % 97) / 97f
            if (dusk) {
                // In the wake of the dark, the largest first.
                min(frontAt(at[i].y) + 160f + 620f * rank + 260f * seed, end - 1050f)
            } else if (i == nearest) {
                // A morning star lingers by the cross.
                1750f
            } else {
                // Out a little ahead of the light, the smallest and lowest first.
                (frontAt(at[i].y) - 850f - 450f * rank - 160f * seed).coerceIn(150f, 1650f)
            }
        }
    }
}

/**
 * One play: its direction, the choice it lands on, and where it stands.
 */
internal class Play(val plan: CrownPlan, val choice: ThemeChoice, val sums: Sums, val k: Float) {
    val dusk = plan.toDark
    var turned = false
    var capturing = false
    var old: ImageBitmap? = null
    var crossOld: ImageBitmap? = null
    var crossNew: ImageBitmap? = null
    var crossCatch: ImageBitmap? = null
    var gilt = false
    /** A second tap on the cross, kept for the settle. */
    var again = false
    /** When the cross was last tapped, in ms of the play's clock (the tap that began it at 0). */
    var lastTap = 0f
    var rate = 1f
    var job: Job? = null
}

/**
 * Home's crown and its play. One per window: the root records the screen through it
 * ([crowning]), home registers its frontispiece and units, and the cross's tap calls [tap].
 */
@Stable
class Crown internal constructor(internal val layer: GraphicsLayer, private val scope: CoroutineScope) {
    /** The reader's saved choice, the device's look, and the system's animation scale. */
    var choice: () -> ThemeChoice = { ThemeChoice.DEFAULT }
    var deviceDark = false
    var animationScale: () -> Float = { 1f }
    /** Saves the choice (at the tap), applies it (at the turn), or makes it the Menu's way. */
    var save: (ThemeChoice) -> Unit = {}
    var apply: (ThemeChoice) -> Unit = {}
    var plain: (ThemeChoice) -> Unit = {}

    internal var play: Play? by mutableStateOf(null)
    /** The play's clock: ms from the tap on the plan's timetable. */
    internal var t by mutableFloatStateOf(0f)
    internal val lifted = mutableStateMapOf<String, Boolean>()
    internal val units = HashMap<String, CrownUnit>()
    internal var head: CrownHead? = null
    internal var rootSize = IntSize.Zero
    internal var density = 1f
    internal var screenDp = 0f
    val firmament = CrownSky(this)

    /** What the last tap turned away from, and what it wrote: in memory only. */
    private var turnedFrom: Pair<ThemeChoice, ThemeChoice>? = null

    val playing: Boolean get() = play != null

    /** The look a tap lands on: the core's rule, with the tap back kept in memory. */
    internal fun next(): ThemeChoice {
        val stored = choice()
        val restore = turnedFrom?.takeIf { it.second == stored }?.first
        val after = afterCrownTap(stored.name.lowercase(), deviceDark, restore?.name?.lowercase())
        val next = ThemeChoice.valueOf(after.choice.uppercase())
        turnedFrom = after.restore?.let { ThemeChoice.valueOf(it.uppercase()) to next }
        return next
    }

    /** A tap on the cross. */
    fun tap() {
        val p = play
        if (p != null) {
            val t = this.t
            if (t < p.sums.settle) {
                // Kept, once: the way back plays at the settle. Not the taps of a double tap or a
                // flurry; and before the turn, only a tap well after the last. (Timed on the play's
                // own clock, in real ms.)
                val apart = p.plan.tapApart.toFloat()
                val since = (t - p.lastTap) * p.k
                if ((p.turned || since > 2 * apart) && since > apart) p.again = !p.again
                p.lastTap = t
                return
            }
            finish()
        }
        val choice = next()
        val h = head
        val k = animationScale()
        if (h == null || k <= 0f || h.black || h.picking || rootSize == IntSize.Zero || h.width <= 0f || units["cross"] == null) {
            plain(choice)
            return
        }
        start(choice, h, k)
    }

    private fun start(choice: ThemeChoice, h: CrownHead, k: Float) {
        val toDark = choice.dark(deviceDark)
        val plan = crownPlan(toDark)
        val sums = runCatching { Sums(plan, h, units, rootSize, density, screenDp > 700f) }.getOrNull()
        if (sums == null) {
            plain(choice)
            return
        }
        save(choice)
        val p = Play(plan, choice, sums, k)
        p.gilt = toDark && h.gilt
        val cross = units["cross"]!!
        p.crossNew = look(cross.size) { h.cross(this, toDark) }
        p.crossCatch = look(cross.size) { consecrationCross(h.catchInk) }
        t = 0f
        play = p
        p.job = scope.launch {
            p.crossOld = runCatching { cross.layer.toImageBitmap() }.getOrNull()
            if (play !== p) return@launch
            if (p.crossOld != null) lifted["cross"] = true
            var last = -1L
            while (play === p) {
                withFrameNanos { now ->
                    if (last >= 0) t += (now - last) / 1_000_000f * p.rate / p.k
                    last = now
                }
                if (play !== p) break
                if (!p.turned && t >= p.sums.turn) {
                    capture(p)
                    last = -1L
                    continue
                }
                if (p.again && t >= p.sums.settle) {
                    finish()
                    tap()
                    break
                }
                if (t >= p.sums.end) finish()
            }
        }
    }

    private fun look(size: IntSize, draw: DrawScope.() -> Unit): ImageBitmap? = runCatching {
        val image = ImageBitmap(max(1, size.width), max(1, size.height))
        CanvasDrawScope().draw(Density(density), LayoutDirection.Ltr, Canvas(image), size.toSize(), draw)
        image
    }.getOrNull()

    /**
     * The turn: the units' old looks and the screen as it stands are kept, then the theme changes
     * under them. The first act holds its breath for the frames this takes.
     */
    private suspend fun capture(p: Play) {
        p.capturing = true
        for ((key, unit) in units.entries.toList()) {
            if (key == "cross") continue
            unit.old = runCatching { unit.layer.toImageBitmap() }.getOrNull()
            if (unit.old != null) lifted[key] = true
        }
        withFrameNanos {}
        withFrameNanos {}
        if (play !== p) return
        p.old = runCatching { layer.toImageBitmap() }.getOrNull()
        if (p.old == null) {
            // No still to play over: the Menu's change.
            finish(pending = plain)
            return
        }
        apply(p.choice)
        p.turned = true
    }

    /**
     * A touch somewhere other than the cross: what is left runs out quickly, and the touch goes
     * on to what it touched. Before the turn there is nothing to play out: the Menu's change.
     */
    internal fun touched(at: Offset) {
        val p = play ?: return
        if (head?.tapBox?.contains(at) == true) return
        if (!p.turned) {
            finish(pending = plain)
            return
        }
        if (t >= p.sums.settle) return
        val left = p.sums.end - t
        p.rate = max(p.rate, max(1f, left / p.plan.rush.toFloat()))
    }

    /** Scrolling, a new page or a new size: the play ends at once. */
    fun cut() = finish()

    /**
     * Ends the play, leaving nothing behind: the screen is the live, themed screen. A choice not
     * yet applied is applied by `pending`.
     */
    fun finish(pending: (ThemeChoice) -> Unit = apply) {
        val p = play ?: return
        play = null
        p.job?.cancel()
        lifted.clear()
        units.values.forEach { it.old = null; it.layer.alpha = 1f }
        p.old = null
        if (!p.turned) pending(p.choice)
    }

    // ---- The stars' say (the sky asks while it draws). ----

    private val kindleAlpha = Keys(floatArrayOf(0f, 0.55f, 1f), floatArrayOf(0f, 1f, 1f), arrayOf(Ease.kindle, Ease.kindle))
    private val kindleScale = Keys(floatArrayOf(0f, 0.55f, 1f), floatArrayOf(0.86f, 1.06f, 1f), arrayOf(Ease.kindle, Ease.kindle))

    internal fun starAlpha(index: Int): Float {
        val p = play ?: return 1f
        val d = p.sums.starTurns.getOrNull(index) ?: return 1f
        return if (p.dusk) {
            kindleAlpha.at((t - d) / 800f)
        } else if (p.capturing || p.turned) {
            // At dawn the still is made without them, and the play draws them on.
            0f
        } else {
            1f - Ease.catch.at((t - d) / 650f)
        }
    }

    internal fun starScale(index: Int): Float {
        val p = play ?: return 1f
        if (!p.dusk) return 1f
        val d = p.sums.starTurns.getOrNull(index) ?: return 1f
        return kindleScale.at((t - d) / 800f)
    }

    // ---- Drawing. ----

    private val catchKeys = Keys(floatArrayOf(0f, 0.4f, 1f), floatArrayOf(0f, 0.6f, 0f), arrayOf(Ease.catch, Ease.catch))
    private val catchHold = Keys(floatArrayOf(0f, 0.34f, 1f), floatArrayOf(0f, 0.55f, 0f), arrayOf(Ease.inOut, Ease.catch))
    private val veilDusk = Keys(
        floatArrayOf(0f, 0.329f, 0.467f, 0.733f, 1f),
        floatArrayOf(0f, 0.75f, 1f, 0.8f, 0f),
        arrayOf(Bezier(0.4f, 0f, 0.7f, 1f), Bezier(0.3f, 0.3f, 0.6f, 1f), Bezier(0.4f, 0f, 0.6f, 1f), Bezier(0.4f, 0f, 0.5f, 1f)),
    )

    internal fun draw(scope: DrawScope, p: Play) = with(scope) {
        val s = p.sums
        val t = this@Crown.t
        val turn = s.turn
        val old = p.old
        val full = Rect(Offset.Zero, size)
        if (old == null) drawLayer(layer) else drawImage(old)

        if (old != null) {
            // The page beyond the arch, through the arch's own shape grown, feathered.
            val glow = s.glow(t)
            if (glow > 0f && t >= s.spreadAt) layered(full, glow) {
                drawLayer(layer)
                drawRect(s.maskBrush(s.spreadScale(t)), blendMode = BlendMode.DstIn)
            }
            veil(p, t)
            // The head keeps its old ground until its own front has passed, whatever the page
            // behind has done (the web's `.home-back`).
            clipPath(s.vaultPath) {
                drawImage(old)
                if (p.dusk) {
                    // The plaster still lit takes a trace of the afterglow as the night comes down.
                    val warm = Ease.warm.at((t - turn - 100f) / 900f)
                    if (warm > 0f) {
                        drawRect(
                            Brush.verticalGradient(
                                0f to rgb(p.plan.skyOchre, 0.1f), 0.55f to rgb(p.plan.skyOchre, 0.15f), 1f to rgb(p.plan.skyRedOchre, 0.2f),
                                startY = s.vault.top, endY = s.vault.bottom,
                            ),
                            s.vault.topLeft, s.vault.size, alpha = warm,
                        )
                    }
                }
            }
            if (!p.dusk) {
                // Dawn: the stars go out ahead of the light, under it.
                translate(s.origin.x, s.origin.y) {
                    for (star in firmament.drawn) {
                        val d = s.starTurns.getOrNull(star.index) ?: continue
                        val a = 1f - Ease.catch.at((t - d) / 650f)
                        if (a > 0f) drawPath(star.path, star.brush, alpha = star.alpha * a)
                    }
                }
            }
        }

        horizon(p, t)

        if (old != null) {
            // The head's front: the new ground inside the slate course, with what the light leaves.
            val r = s.frontRadius(t)
            layered(full, 1f) {
                clipPath(s.vaultPath) {
                    drawLayer(layer)
                    vaultLight(p, t)
                }
                val c = Offset(s.vault.center.x, s.cy)
                scale(s.a, 1f, pivot = c) {
                    drawRect(
                        Brush.radialGradient(
                            0f to Color.Black, 0.78f to Color.Black, 0.835f to Color.Black.copy(alpha = 0.844f), 0.89f to Color.Black.copy(alpha = 0.5f),
                            0.945f to Color.Black.copy(alpha = 0.156f), 1f to Color.Transparent,
                            center = c, radius = max(1f, r),
                        ),
                        Offset(c.x - size.width, -size.height), Size(size.width * 2, size.height * 3), blendMode = BlendMode.DstIn,
                    )
                }
            }
        }

        // The units, each handing its ink over at its moment.
        for ((key, unit) in units) {
            if (key == "cross" || lifted[key] != true) continue
            val (out, into) = when {
                !p.turned -> 1f to 0f
                key == "frieze" -> Ease.inOut.at((t - s.spreadAt - 250f) / 900f).let { 1f - it to it }
                key.startsWith("w-") -> {
                    val w = s.words[key] ?: turn
                    (1f - Ease.inkOut.at((t - w + 230f) / 240f)) to Ease.inkIn.at((t - w - 40f) / 420f)
                }
                else -> {
                    val r = s.rows[key] ?: s.settle
                    clamp01((t - r + 70f) / p.plan.inkHandover.toFloat()).let { 1f - it to it }
                }
            }
            if (out > 0f) unit.old?.let { drawImage(it, unit.at, alpha = out) }
            if (into > 0f) translate(unit.at.x, unit.at.y) {
                unit.layer.alpha = into
                drawLayer(unit.layer)
                unit.layer.alpha = 1f
            }
        }

        // The cross, in its two looks and the light it catches.
        val cross = units["cross"]
        if (cross != null && lifted["cross"] == true) {
            val (oldAlpha, newAlpha, catchAlpha) = if (p.dusk) {
                Triple(
                    1f - Ease.inOut.at((t - 780f) / 300f),
                    Ease.inOut.at((t - 80f) / 760f),
                    max(if (p.gilt) catchKeys.at((t - 80f) / 760f) else 0f, catchKeys.at((t - turn - 2700f) / 600f)),
                )
            } else {
                Triple(1f - Ease.inOut.at((t - turn - 2230f) / 300f), Ease.inOut.at((t - turn - 1550f) / 700f), catchHold.at((t - 60f) / 1300f))
            }
            p.crossOld?.let { if (oldAlpha > 0f) drawImage(it, cross.at, alpha = oldAlpha) }
            p.crossNew?.let { if (newAlpha > 0f) drawImage(it, cross.at, alpha = newAlpha) }
            p.crossCatch?.let { if (catchAlpha > 0f) drawImage(it, cross.at, alpha = catchAlpha) }
        }
    }

    /** Draws `block` into a layer of its own, composited at `alpha`. */
    private inline fun DrawScope.layered(bounds: Rect, alpha: Float, block: DrawScope.() -> Unit) {
        drawIntoCanvas { it.saveLayer(bounds, Paint().apply { this.alpha = alpha }) }
        block()
        drawIntoCanvas { it.restore() }
    }

    /**
     * The veil: the hour's colour laid over the page in the page's own growing shape, a little
     * ahead of it, thinning out from the arch: never a sheet. Dawn's ink turns rose-umber through
     * the wall's dark and grey half.
     */
    private fun DrawScope.veil(p: Play, t: Float) {
        val s = p.sums
        if (t < s.spreadAt || t < s.veil0 || t > s.veil1) return
        val x = (t - s.veil0) / (s.veil1 - s.veil0)
        val peak = p.plan.veilPeak
        val low = p.plan.dawnVeilLow
        val opacity: Float
        val ink: Color
        val stops: FloatArray
        if (p.dusk) {
            opacity = peak * veilDusk.at(x)
            ink = rgb(p.plan.veilInk)
            stops = floatArrayOf(1f, 0.86f, 0.5f, 0.2f)
        } else {
            opacity = Keys(
                floatArrayOf(0f, 0.305f, 0.436f, 0.667f, 0.795f, 1f),
                floatArrayOf(0f, peak * 1.13f, low, low, peak, 0f),
                arrayOf(Ease.catch, Ease.catch, null, Ease.catch, Ease.rise),
            ).at(x)
            val blush = rgb(p.plan.veilInk)
            val umber = rgb(p.plan.dawnVeilUmber)
            val share = Keys(floatArrayOf(0f, 0.28f, 0.4f, 0.64f, 0.795f, 1f), floatArrayOf(0f, 0f, 1f, 1f, 0f, 0f), arrayOfNulls<Bezier>(5)).at(x)
            ink = mix(umber, blush, share)
            stops = floatArrayOf(1f, 0.88f, 0.66f, 0.46f)
        }
        if (opacity <= 0f) return
        val full = Rect(Offset.Zero, size)
        val c = Offset(s.ax, s.ys)
        layered(full, opacity) {
            scale(1f, 1.9f / 1.5f, pivot = c) {
                drawRect(
                    Brush.radialGradient(
                        0f to ink.copy(alpha = stops[0]), 0.28f to ink.copy(alpha = stops[1]), 0.58f to ink.copy(alpha = stops[2]), 1f to ink.copy(alpha = stops[3]),
                        center = c, radius = 1.5f * s.W,
                    ),
                    Offset(-size.width, -size.height), Size(size.width * 3, size.height * 3),
                )
            }
            drawRect(s.maskBrush(s.veilScale(t)), blendMode = BlendMode.DstIn)
        }
    }

    /** A wash on the horizon: a colour brush under a low elliptical mound, risen or sunk about its foot. */
    private fun DrawScope.mound(rect: Rect, colour: Brush, rx: Float, stops: Array<Pair<Float, Color>>, rise: Float, alpha: Float) {
        if (alpha <= 0f || rect.height <= 0f || rx <= 0f) return
        val foot = Offset(rect.center.x, rect.bottom)
        val h = rect.height
        layered(Rect(Offset.Zero, size), alpha) {
            scale(1f, rise, pivot = foot) {
                drawRect(colour, rect.topLeft, rect.size)
                // The mound's ellipse, rx across and the wash's height up, about the middle of its
                // foot; beyond it the gradient's last stop (clear) holds.
                val sx = rx / h
                val reach = rect.width / sx + h
                scale(sx, 1f, pivot = foot) {
                    drawRect(
                        Brush.radialGradient(*stops, center = foot, radius = h),
                        Offset(foot.x - reach, rect.top), Size(reach * 2, h),
                        blendMode = BlendMode.DstIn,
                    )
                }
            }
        }
    }

    /**
     * The first act's light on the horizon, inside the head under the day's words: at dusk the
     * afterglow, yellow ochre with red ochre only at the horizon; at dawn first light, lime white
     * over a rose ochre that comes before it, parted from the sky by a rose-lilac.
     */
    private fun DrawScope.horizon(p: Play, t: Float) {
        val s = p.sums
        val turn = s.turn
        val rect = Rect(s.vault.left, s.vault.bottom - s.room, s.vault.right, s.vault.bottom)
        val plan = p.plan
        clipPath(s.vaultPath) {
            if (p.dusk) {
                val x = Ease.rise.at((t - 250f) / 1250f)
                val gone = clamp01((t - turn - 1550f) / 300f)
                val dense = screenDp <= 700f
                val ochre = Brush.verticalGradient(
                    0f to rgb(plan.skyOchre, 0f), 0.32f to rgb(plan.skyOchre, if (dense) 0.2f else 0.1f),
                    0.7f to rgb(plan.skyOchre, if (dense) 0.52f else 0.26f), 1f to rgb(plan.skyOchre, if (dense) 0.62f else 0.3f),
                    startY = rect.top, endY = rect.bottom,
                )
                val red = Brush.verticalGradient(
                    0f to rgb(plan.skyRedOchre, 0f), 0.68f to rgb(plan.skyRedOchre, 0f), 0.86f to rgb(plan.skyRedOchre, if (dense) 0.22f else 0.1f),
                    1f to rgb(plan.skyRedOchre, if (dense) 0.5f else 0.24f),
                    startY = rect.top, endY = rect.bottom,
                )
                val stops = arrayOf(0f to Color.Black, 0.34f to Color.Black.copy(alpha = 0.85f), 0.66f to Color.Black.copy(alpha = 0.4f), 1f to Color.Transparent)
                val a = x * (1f - gone)
                mound(rect, ochre, 1.6f * rect.width, stops, 0.3f + 0.7f * x, a)
                mound(rect, red, 1.6f * rect.width, stops, 0.3f + 0.7f * x, a)
            } else {
                val stops = arrayOf(0f to Color.Black, 0.3f to Color.Black.copy(alpha = 0.9f), 0.64f to Color.Black.copy(alpha = 0.45f), 1f to Color.Transparent)
                val gone = clamp01((t - turn - 800f) / 600f)
                // Under it, and before it, the colour of the hour.
                val before = Ease.rise.at((t - 220f) / 650f)
                mound(
                    rect,
                    Brush.verticalGradient(
                        0f to rgb(plan.skyRoseLilac, 0f), 0.18f to rgb(plan.skyRoseLilac, 0.14f), 0.45f to Color(196, 146, 140).copy(alpha = 0.42f),
                        0.78f to Color(206, 150, 124).copy(alpha = 0.8f), 1f to Color(214, 160, 128).copy(alpha = 0.95f),
                        startY = rect.top, endY = rect.bottom,
                    ),
                    s.washRx, stops, 0.3f + 0.7f * before, before * (1f - gone),
                )
                val light = Ease.rise.at((t - 450f) / 800f)
                mound(
                    rect,
                    Brush.verticalGradient(
                        0f to rgb(plan.skyRoseLilac, 0f), 0.16f to rgb(plan.skyRoseLilac, 0.12f), 0.34f to rgb(plan.skyRoseLilac, 0.28f),
                        0.54f to rgb(plan.skyLime, 0.5f), 0.84f to rgb(plan.skyLime, 0.9f), 1f to rgb(plan.skyLime, 0.92f),
                        startY = rect.top, endY = rect.bottom,
                    ),
                    s.washRx, stops, 0.3f + 0.7f * light, light * (1f - gone),
                )
            }
        }
    }

    /** What the light leaves on the new ground, low in the head: an ember on the night, a warmth on the plaster. Both go out. */
    private fun DrawScope.vaultLight(p: Play, t: Float) {
        val s = p.sums
        val h = 0.22f * (s.vault.bottom - s.vault.top)
        val rect = Rect(s.vault.left, s.vault.bottom - h, s.vault.right, s.vault.bottom)
        val x = if (p.dusk) Ease.catch.at((t - s.turn - 2250f) / 1000f) else Ease.catch.at((t - s.turn - 850f) / 1400f)
        val colour = if (p.dusk) {
            Brush.verticalGradient(0f to rgb(p.plan.skyEmber, 0f), 0.55f to rgb(p.plan.skyEmber, 0.32f), 1f to rgb(p.plan.skyEmber, 0.64f), startY = rect.top, endY = rect.bottom)
        } else {
            Brush.verticalGradient(0f to rgb(p.plan.skyOchre, 0f), 0.5f to rgb(p.plan.skyOchre, 0.08f), 1f to rgb(p.plan.skyOchre, 0.2f), startY = rect.top, endY = rect.bottom)
        }
        val stops = arrayOf(
            0f to Color.Black, 0.26f to Color.Black.copy(alpha = 0.85f), 0.52f to Color.Black.copy(alpha = 0.4f), 0.7f to Color.Black.copy(alpha = 0.12f),
            0.84f to Color.Transparent, 1f to Color.Transparent,
        )
        mound(rect, colour, 0.58f * rect.width, stops, 1f - 0.6f * x, 1f - x)
    }
}

@Composable
fun rememberCrown(): Crown {
    val layer = rememberGraphicsLayer()
    val scope = rememberCoroutineScope()
    return remember(layer) { Crown(layer, scope) }
}

/**
 * Records the screen for the crown and, while it plays, draws the play in its place. A touch
 * anywhere but the cross is let through, and runs the play out quickly; a new size ends it.
 */
fun Modifier.crowning(crown: Crown): Modifier = this
    .onSizeChanged {
        if (crown.rootSize != IntSize.Zero && crown.rootSize != it) crown.cut()
        crown.rootSize = it
    }
    .pointerInput(crown) {
        awaitPointerEventScope {
            while (true) {
                val event = awaitPointerEvent(PointerEventPass.Initial)
                if (event.type == PointerEventType.Press) event.changes.firstOrNull()?.let { crown.touched(it.position) }
            }
        }
    }
    .drawWithContent {
        crown.density = density
        crown.screenDp = size.width / density
        crown.layer.record { this@drawWithContent.drawContent() }
        val p = crown.play
        if (p == null) drawLayer(crown.layer) else crown.draw(this, p)
    }

/**
 * Makes this part of home a unit of the crown's play (`key`: `w-` a line of the day, `r-` a row,
 * `frieze`, `cross`): recorded into a layer of its own, and drawn by the play while lifted.
 * Elsewhere, or with no crown, it is nothing.
 */
@Composable
internal fun Modifier.crownUnit(key: String): Modifier {
    val crown = LocalCrownStage.current ?: return this
    val layer = rememberGraphicsLayer()
    val unit = remember(layer) { CrownUnit(layer) }
    DisposableEffect(crown, key, unit) {
        crown.units[key] = unit
        onDispose { if (crown.units[key] === unit) crown.units.remove(key) }
    }
    return this
        .onGloballyPositioned { unit.at = it.positionInRoot(); unit.size = it.size }
        .drawWithContent {
            unit.layer.record { this@drawWithContent.drawContent() }
            if (crown.lifted[key] != true) drawLayer(unit.layer)
        }
}

/** The crown's play holds `head` while home shows it, and ends when home goes. */
@Composable
internal fun CrownHeadEffect(crown: Crown?, head: CrownHead?) {
    DisposableEffect(crown, head) {
        if (crown != null && head != null) crown.head = head
        onDispose {
            if (crown != null && crown.head === head) {
                crown.finish()
                crown.head = null
            }
        }
    }
}
