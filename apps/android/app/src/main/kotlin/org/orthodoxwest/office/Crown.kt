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
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.ColorMatrix
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
 * the page with a wide feather, under a veil of the hour's colour multiplied over it; each lifted
 * unit hands its old ink to its new at its own moment. When it ends nothing is left: the screen
 * is the live, themed screen.
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
internal class Keys(private val offsets: FloatArray, private val values: FloatArray, private val eases: Array<Bezier?>) {
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
    /** presentation's front ease: quickest in its middle, a long tail (the frieze, the snuff, the sinks). */
    val tail = Bezier(0.4f, 0f, 0.3f, 1f)
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

/**
 * The stars of the head's sky, as home's sky drew them (in the card's own coordinates, `centre`
 * and `r` the star's middle and magnitude): the play draws them from the change of look.
 */
internal class SkyStarPaint(val index: Int, val path: Path, val brush: Brush, val alpha: Float, val centre: Offset, val r: Float)

/**
 * The sky's say in the play: from the change of look the play draws the stars itself, above the
 * head's front (dusk kindles them, dawn puts them out), so the sky leaves them out; and which it drew.
 */
class CrownSky internal constructor(private val crown: Crown) {
    internal var drawn: List<SkyStarPaint> = emptyList()

    /** Whether the play has a say in this frame (else every star is simply itself). */
    val active: Boolean get() = crown.play != null

    /** How lit the sky's own stars are: 1, or none while the play draws them. */
    val lit: Float get() = crown.starAlpha()

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
    val veilIn = Bezier(plan.veilIn)
    val veilOut = Bezier(plan.veilOut)

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
    /** The veil's shape: the spread's whole run of sizes, led, played over `vDur`. */
    val vnodes = FloatArray(NODES + 1) { sizeAt(dur * it / NODES) * plan.veilLead }

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
     * it is given, dealt out about the middle one 14ms apart, none further than 60ms from its own
     * moment and never two within 10ms: the wall is shaded to the new look's value before it is
     * crossed, so the rows come back together, in one breath.
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
            var t = max(own - 60f, min(own + 60f, base + (n - mid) * 14f))
            if (n > 0 && t < previous + 10f) t = previous + 10f
            previous = t
            dealt[key] = t
        }
        rows = dealt
        lastDip = dealt.values.maxOrNull() ?: 0f
    }

    /** The play is settled when the page has visibly turned and the last row has its new ink. */
    val settle = maxOf(covered, lastDip + 220f, spreadAt + 1100f, veil1 - 80f, spreadAt + dur * part - 50f) + 80f
    /** The ending beat: the page still for `beat_hold`, then one thing, in `beat_len`. */
    val beat = settle + plan.beatHold.toFloat()
    val end = max(plan.end.toFloat(), beat + plan.beatLen.toFloat() + 30f)

    /** The veil's span: it begins with the spread, and runs longer on a wide screen. */
    val veilEnd = veil1 + if (wideDp) plan.spreadWide.toFloat() else 0f

    /**
     * The vault's stand-in (the head's front over the new screen) leaves early and slowly, from
     * where the page under it is nearly its new self until just before the settle; the old ground
     * kept under it goes first.
     */
    val vaultOut = spreadAt + 0.62f * dur
    val vaultGone = max(vaultOut + 1f, settle - 60f)

    /** A star kindling swells a little past its size and settles. */
    internal val kindleScale = Keys(floatArrayOf(0f, 0.55f, 1f), floatArrayOf(0.86f, plan.starOver, 1f), arrayOf(Ease.kindle, Ease.kindle))

    // ---- The stars: their turns, by magnitude, in ms from the turn (app.js's `g.starTurns`). ----
    private val crossAt = units["cross"]?.rect?.center ?: Offset(ax, apex)
    private val marginIn = (vault.left - origin.x + 9f * u) / W

    /** The turns of the stars the sky drew (in its order), the star by the cross, and the morning star's deepening. */
    class StarTurns(val key: List<Int>, val turns: FloatArray, val nearest: Int, val morningAt: Float)

    fun starTurns(list: List<SkyStarPaint>): StarTurns {
        val at = list.map { Offset(origin.x + it.centre.x, origin.y + it.centre.y) }
        val nearest = at.indices.minByOrNull { (at[it] - crossAt).getDistance() } ?: -1
        val settleR = settle - turn
        val beatR = beat - turn
        val out = FloatArray(list.size) { beatR }
        // Brightest first; equal stars keep the field's order.
        val rest = list.indices.filter { it != nearest }.sortedWith(compareByDescending<Int> { list[it].r }.thenBy { it })
        val bright = rest.take(plan.starsBright.toInt())
        val field = rest.drop(plan.starsBright.toInt()).toMutableList()
        val apart = plan.starsBrightApart.toFloat()
        fun dark(i: Int) = frontAt(at[i].y) - turn
        fun seed(i: Int) = ((i * 7919) % 97) / 97f
        var morningAt = 0f
        if (dusk) {
            val first = if (nearest < 0) 150f else max(150f, dark(nearest) + 60f)
            if (nearest >= 0) out[nearest] = first
            val lastOne = if (field.size > 1) field.removeAt(field.size - 1) else -1
            val alone = plan.eveningStarAlone.toFloat()
            bright.forEachIndexed { n, i -> out[i] = max(first + alone + n * apart, dark(i) + 100f) }
            val from = first + alone + bright.size * apart
            val cap = settleR - 0.3f * plan.starKindle.toFloat()
            val to = max(from, cap)
            field.forEachIndexed { n, i ->
                var t = from + (to - from) * (n + 0.6f * (seed(i) - 0.5f)) / max(1, field.size - 1)
                t = maxOf(from, min(to, t), dark(i) + 100f)
                // A star out in the moulding's margin waits for the wall's night.
                if (shape((at[i].x - origin.x) / W, (ys - at[i].y) / W) < marginIn) {
                    t = max(t, turnedAt(at[i].x, at[i].y, 0.5f) - turn + 200f)
                }
                out[i] = min(t, cap)
            }
            if (lastOne >= 0) out[lastOne] = beatR
        } else {
            val linger = plan.dawnStarLinger.toFloat()
            val until = plan.dawnFieldOut * settleR - bright.size * apart
            field.reversed().forEachIndexed { n, i ->
                val t = 40f + (until - 40f) * (n + 0.6f * (seed(i) - 0.5f)) / max(1, field.size - 1)
                out[i] = max(0f, minOf(t, until, dark(i) + linger))
            }
            bright.reversed().forEachIndexed { n, i -> out[i] = max(0f, min(until + (n + 1) * apart, dark(i) + linger)) }
            if (nearest >= 0) {
                out[nearest] = beatR
                // When the plaster's front reaches the morning star: its gold deepens to stand on it.
                morningAt = dark(nearest) - 300f
            }
        }
        return StarTurns(list.map { it.index }, out, nearest, morningAt)
    }
}

/**
 * One play: its direction, the choice it lands on, and where it stands.
 */
internal class Play(val plan: CrownPlan, val choice: ThemeChoice, val sums: Sums, val k: Float) {
    val dusk = plan.toDark
    // (State: the sky reads them, to leave its stars to the play from the change of look.)
    var turned by mutableStateOf(false)
    var capturing by mutableStateOf(false)
    var old: ImageBitmap? = null
    var crossOld: ImageBitmap? = null
    var crossNew: ImageBitmap? = null
    var crossCatch: ImageBitmap? = null
    var crossEmber: ImageBitmap? = null
    /** The stars' turns, for the stars the sky last drew. */
    var stars: Sums.StarTurns? = null
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
    /** One soft tick under the finger at the turn, as the look itself changes. */
    var tick: () -> Unit = {}
    /** Saves the choice (at the tap), applies it (at the turn), or makes it the Menu's way. */
    var save: (ThemeChoice) -> Unit = {}
    var apply: (ThemeChoice) -> Unit = {}
    var plain: (ThemeChoice) -> Unit = {}
    /** Why the last tap took the Menu's change rather than the play, for tests and debugging. */
    internal var plainBecause: String? = null

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
        plainBecause = when {
            h == null -> "no head"
            k <= 0f -> "animations off"
            h.black -> "black day"
            h.picking -> "picking a date"
            rootSize == IntSize.Zero -> "no root size"
            h.width <= 0f -> "no head width"
            units["cross"] == null -> "no cross"
            else -> null
        }
        if (plainBecause != null || h == null) {
            plain(choice)
            return
        }
        start(choice, h, k)
    }

    private fun start(choice: ThemeChoice, h: CrownHead, k: Float) {
        val toDark = choice.dark(deviceDark)
        val plan = crownPlan(toDark)
        val sums = runCatching { Sums(plan, h, units, rootSize, density, screenDp > 700f) }
            .onFailure { plainBecause = "sums: $it" }.getOrNull()
        if (sums == null) {
            plain(choice)
            return
        }
        save(choice)
        // The timetable is performed plan.pace times slower than it is written.
        val p = Play(plan, choice, sums, k * plan.pace)
        p.gilt = toDark && h.gilt
        val cross = units["cross"]!!
        p.crossNew = look(cross.size) { h.cross(this, toDark) }
        p.crossCatch = look(cross.size) { consecrationCross(h.catchInk) }
        p.crossEmber = look(cross.size) { consecrationCross(rgb(plan.crossEmber)) }
        firmament.drawn = emptyList()
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
            plainBecause = "no still"
            finish(pending = plain)
            return
        }
        apply(p.choice)
        p.turned = true
        tick()
    }

    /**
     * A touch somewhere other than the cross: what is left runs out quickly, and the touch goes
     * on to what it touched. Before the turn there is nothing to play out: the Menu's change.
     */
    internal fun touched(at: Offset) {
        val p = play ?: return
        if (head?.tapBox?.contains(at) == true) return
        if (!p.turned) {
            plainBecause = "touched before the turn"
            finish(pending = plain)
            return
        }
        if (t >= p.sums.settle) return
        val left = p.sums.end - t
        p.rate = max(p.rate, max(1f, left * p.plan.pace / p.plan.rush.toFloat()))
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

    /** At rest, and until the turn, each star is the sky's; from the turn the play draws it. */
    internal fun starAlpha(): Float {
        val p = play ?: return 1f
        return if (p.capturing || p.turned) 0f else 1f
    }

    // ---- Drawing. ----

    private val catchKeys = Keys(floatArrayOf(0f, 0.4f, 1f), floatArrayOf(0f, 0.6f, 0f), arrayOf(Ease.catch, Ease.catch))
    private val catchHold = Keys(floatArrayOf(0f, 0.34f, 1f), floatArrayOf(0f, 0.55f, 0f), arrayOf(Ease.inOut, Ease.catch))
    private val kindleAlpha = Keys(floatArrayOf(0f, 0.55f, 1f), floatArrayOf(0f, 1f, 1f), arrayOf(Ease.kindle, Ease.kindle))

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
            // The head keeps its old ground until its own front has passed, whatever the page
            // behind has done (the web's `.home-back`), and lets it go just before the vault's
            // stand-in leaves.
            val back = 1f - clamp01((t - (s.vaultOut - 80f)) / 80f)
            if (back > 0f) layered(full, back) {
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
                    } else {
                        // Dawn: ahead of the rose, a wider band of the night's teal, lifted.
                        belt(s, t, 1.5f, rgb(p.plan.skyTeal, 0.6f), arrayOf(0f to Color.Transparent, 0.52f to Color.Transparent, 0.66f to Color.Black.copy(alpha = 0.6f), 0.76f to Color.Black, 0.9f to Color.Black.copy(alpha = 0.4f), 1f to Color.Transparent))
                    }
                    // The belt: rose-lilac riding ahead of the front on the sky it has not reached.
                    belt(s, t, 1.22f, rgb(p.plan.skyRoseLilac, if (p.dusk) 0.62f else 0.5f), arrayOf(0f to Color.Transparent, 0.6f to Color.Transparent, 0.69f to Color.Black.copy(alpha = 0.5f), 0.79f to Color.Black, 0.9f to Color.Black.copy(alpha = 0.45f), 1f to Color.Transparent))
                }
            }
            veil(p, t)
        }

        horizon(p, t)

        if (old != null) {
            // The head's front: the new ground inside the slate course, with what the light leaves.
            // It leaves early and slowly, from where the page under it is nearly its new self.
            val r = s.frontRadius(t)
            val stand = 1f - Ease.warm.at((t - s.vaultOut) / (s.vaultGone - s.vaultOut))
            if (stand > 0f) layered(full, stand) {
                clipPath(s.vaultPath) {
                    drawLayer(layer)
                    if (!p.dusk) plasterShade(p, t)
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

        // The stars, above the vault both ways, from the change of look.
        if (old != null) stars(p, t)

        // The units, each handing its ink over at its moment.
        for ((key, unit) in units) {
            if (key == "cross" || lifted[key] != true) continue
            val (out, into) = when {
                !p.turned -> 1f to 0f
                key == "frieze" -> Ease.tail.at((t - p.plan.frieze.at.toFloat()) / p.plan.frieze.len.toFloat()).let { 1f - it to it }
                key.startsWith("w-") -> {
                    val w = s.words[key] ?: turn
                    (1f - Ease.inkOut.at((t - w + 230f) / 240f)) to Ease.inkIn.at((t - w + 80f) / 360f)
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

        // The cross, in its two looks, the ember gold it keeps (dusk) or takes first (dawn), and
        // the light it catches.
        val cross = units["cross"]
        if (cross != null && lifted["cross"] == true) {
            val oldAlpha: Float
            val newAlpha: Float
            val emberAlpha: Float
            val catchAlpha: Float
            if (p.dusk) {
                oldAlpha = 1f - Ease.inOut.at((t - 780f) / 300f)
                newAlpha = Ease.inOut.at((t - 80f) / 760f)
                catchAlpha = if (p.gilt) catchKeys.at((t - 80f) / 760f) else 0f
                emberAlpha = 0.85f * Ease.rise.at((t - turn - 500f) / 900f) * (1f - Ease.rise.at((t - s.beat + 970f) / 600f))
            } else {
                val beat = Ease.inOut.at((t - s.beat) / p.plan.beatLen.toFloat())
                oldAlpha = 1f - beat
                newAlpha = beat
                catchAlpha = catchHold.at((t - 60f) / 1300f)
                emberAlpha = 0.85f * Ease.rise.at((t - turn - 900f) / 800f) * (1f - beat)
            }
            p.crossOld?.let { if (oldAlpha > 0f) drawImage(it, cross.at, alpha = oldAlpha) }
            p.crossNew?.let { if (newAlpha > 0f) drawImage(it, cross.at, alpha = newAlpha) }
            p.crossEmber?.let { if (emberAlpha > 0f) drawImage(it, cross.at, alpha = emberAlpha) }
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
     * Draws `block` multiplied over what is under it, on a white ground of its own: where it is
     * clear it changes nothing, and a phone whose multiply is Porter-Duff's draws it the same.
     */
    private inline fun DrawScope.multiplied(bounds: Rect, block: DrawScope.() -> Unit) {
        drawIntoCanvas { it.saveLayer(bounds, Paint().apply { blendMode = BlendMode.Multiply }) }
        drawRect(Color.White, bounds.topLeft, bounds.size)
        block()
        drawIntoCanvas { it.restore() }
    }

    /**
     * A belt riding ahead of the head's front on the old ground: a wash seen through a ring on the
     * front's own ellipse, `k` times its size.
     */
    private fun DrawScope.belt(s: Sums, t: Float, k: Float, colour: Color, ring: Array<Pair<Float, Color>>) {
        layered(Rect(Offset.Zero, size), 1f) {
            drawRect(colour, s.vault.topLeft, s.vault.size)
            val c = Offset(s.vault.center.x, s.cy)
            scale(s.a, 1f, pivot = c) {
                drawRect(
                    Brush.radialGradient(*ring, center = c, radius = max(1f, k * s.frontRadius(t))),
                    Offset(c.x - size.width, -size.height), Size(size.width * 2, size.height * 3), blendMode = BlendMode.DstIn,
                )
            }
        }
    }

    /**
     * The veil: the hour's colour multiplied over the turning wall (so it only ever shades: the
     * plaster keeps its texture and every line its ink), in the page's own growing shape, a little
     * ahead of it, thinning a little outward from the arch.
     */
    private fun DrawScope.veil(p: Play, t: Float) {
        val s = p.sums
        if (t < s.spreadAt || t < s.veil0 || t > s.veilEnd) return
        val plan = p.plan
        val x = (t - s.veil0) / (s.veilEnd - s.veil0)
        val opacity = when {
            x < plan.veilTop -> plan.veilPeak * s.veilIn.at(x / plan.veilTop)
            x < plan.veilGone -> plan.veilPeak * (1f - s.veilOut.at((x - plan.veilTop) / (plan.veilGone - plan.veilTop)))
            else -> 0f
        }
        if (opacity <= 0f) return
        val ink = rgb(plan.veilInk)
        val full = Rect(Offset.Zero, size)
        val c = Offset(s.ax, s.ys)
        multiplied(full) {
            layered(full, opacity) {
                scale(1f, 1.9f / 1.5f, pivot = c) {
                    drawRect(
                        Brush.radialGradient(
                            0f to ink, 0.28f to ink.copy(alpha = 0.94f), 0.58f to ink.copy(alpha = 0.82f), 1f to ink.copy(alpha = plan.veilFar),
                            center = c, radius = 1.5f * s.W,
                        ),
                        Offset(-size.width, -size.height), Size(size.width * 3, size.height * 3),
                    )
                }
                drawRect(s.maskBrush(s.veilScale(t)), blendMode = BlendMode.DstIn)
            }
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
     * whole niche takes a raking ochre, and the afterglow rises off the horizon, yellow ochre with
     * red ochre only at the horizon; at dawn the night lifts from the horizon to its teal, then a
     * rose-lilac, then a peach, and last and thin a lime white.
     */
    private fun DrawScope.horizon(p: Play, t: Float) {
        val s = p.sums
        val turn = s.turn
        val rect = Rect(s.vault.left, s.vault.bottom - s.room, s.vault.right, s.vault.bottom)
        val plan = p.plan
        clipPath(s.vaultPath) {
            if (p.dusk) {
                val gone = clamp01((t - turn - 1750f) / 500f)
                // The golden hour on the whole niche, deeper toward the horizon.
                val wash = Ease.rise.at((t - 120f) / 1300f) * (1f - gone)
                if (wash > 0f) {
                    drawRect(
                        Brush.verticalGradient(
                            0f to rgb(plan.skyOchre, 0.16f), 0.6f to rgb(plan.skyOchre, 0.28f), 1f to rgb(plan.skyOchre, 0.4f),
                            startY = s.vault.top, endY = s.vault.bottom,
                        ),
                        s.vault.topLeft, s.vault.size, alpha = wash,
                    )
                }
                val x = Ease.rise.at((t - 250f) / 1250f)
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
                val gone = clamp01((t - turn - 1100f) / 700f)
                // One wash, three pigments deep, rising: the teal leads, then rose, then peach.
                val first = Ease.rise.at((t - 150f) / 1100f)
                val high = Rect(rect.left, rect.bottom - 1.5f * s.room, rect.right, rect.bottom)
                mound(
                    high,
                    Brush.verticalGradient(
                        0f to rgb(plan.skyTeal, 0f), 0.18f to rgb(plan.skyTeal, 0.22f), 0.38f to rgb(plan.skyTeal, 0.5f),
                        0.62f to rgb(plan.skyRoseLilac, 0.5f), 0.84f to Color(206, 150, 124).copy(alpha = 0.8f), 1f to rgb(plan.skyPeach, 0.92f),
                        startY = high.top, endY = high.bottom,
                    ),
                    1.3f * s.washRx, stops, 0.3f + 0.7f * first, first * (1f - gone),
                )
                // The lime comes last, thin, and only low.
                val light = Ease.rise.at((t - 700f) / 900f)
                val low = Rect(rect.left, rect.bottom - 0.5f * s.room, rect.right, rect.bottom)
                mound(
                    low,
                    Brush.verticalGradient(0f to rgb(plan.skyLime, 0f), 0.6f to rgb(plan.skyLime, 0.3f), 1f to rgb(plan.skyLime, 0.45f), startY = low.top, endY = low.bottom),
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
        val x = if (p.dusk) Ease.tail.at((t - s.turn - 1900f) / 1500f) else Ease.tail.at((t - s.turn - 900f) / 1600f)
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

    /**
     * Dawn's plaster does not arrive white: it comes up under a warm shade, peach low and rose
     * above, multiplied over it, which lifts on a long tail.
     */
    private fun DrawScope.plasterShade(p: Play, t: Float) {
        val s = p.sums
        val f = 1f - Ease.rise.at((t - s.turn - 250f) / 2300f)
        if (f <= 0f) return
        // Multiplying by the shade mixed with white at its strength is the shade at that strength.
        drawRect(
            Brush.verticalGradient(
                0f to mix(rgb(p.plan.skyRoseLilac), Color.White, f), 0.45f to mix(Color(206, 158, 140), Color.White, f), 1f to mix(rgb(p.plan.skyPeach), Color.White, f),
                startY = s.vault.top, endY = s.vault.bottom,
            ),
            s.vault.topLeft, s.vault.size, blendMode = BlendMode.Multiply,
        )
    }

    /**
     * The stars, by magnitude, each at its turn from the change of look: at dusk each kindles,
     * swelling a little past its size and settling; at dawn each goes out, and the morning star by
     * the cross deepens its gold as the plaster reaches it and goes out at the ending beat.
     */
    private fun DrawScope.stars(p: Play, t: Float) {
        if (!p.turned) return
        val drawn = firmament.drawn
        if (drawn.isEmpty()) return
        val s = p.sums
        val key = drawn.map { it.index }
        val turns = p.stars?.takeIf { it.key == key } ?: s.starTurns(drawn).also { p.stars = it }
        val since = t - s.turn
        translate(s.origin.x, s.origin.y) {
            drawn.forEachIndexed { i, star ->
                val local = since - turns.turns[i]
                if (p.dusk) {
                    val a = kindleAlpha.at(local / p.plan.starKindle.toFloat())
                    if (a <= 0f) return@forEachIndexed
                    val grown = s.kindleScale.at(local / p.plan.starKindle.toFloat())
                    scale(grown, pivot = star.centre) { drawPath(star.path, star.brush, alpha = star.alpha * a) }
                } else if (i == turns.nearest) {
                    val a = 1f - Ease.inOut.at(local / p.plan.beatLen.toFloat())
                    if (a <= 0f) return@forEachIndexed
                    val deep = Ease.inOut.at((since - turns.morningAt) / 700f)
                    drawPath(star.path, star.brush, alpha = star.alpha * a, colorFilter = if (deep > 0f) morning(deep) else null)
                } else {
                    val a = 1f - Ease.tail.at(local / p.plan.starSnuff.toFloat())
                    if (a > 0f) drawPath(star.path, star.brush, alpha = star.alpha * a)
                }
            }
        }
    }

    /** The morning star's gold deepened, `e` of the way to brightness 0.7 and saturation 1.8 (CSS's filters). */
    private fun morning(e: Float): ColorFilter {
        val b = 1f - 0.3f * e
        val k = 1f + 0.8f * e
        val r = 0.213f
        val g = 0.715f
        val bl = 0.072f
        return ColorFilter.colorMatrix(
            ColorMatrix(
                floatArrayOf(
                    b * (r + (1 - r) * k), b * (g - g * k), b * (bl - bl * k), 0f, 0f,
                    b * (r - r * k), b * (g + (1 - g) * k), b * (bl - bl * k), 0f, 0f,
                    b * (r - r * k), b * (g - g * k), b * (bl + (1 - bl) * k), 0f, 0f,
                    0f, 0f, 0f, 1f, 0f,
                ),
            ),
        )
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
