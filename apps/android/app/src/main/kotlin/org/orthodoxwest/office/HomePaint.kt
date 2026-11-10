package org.orthodoxwest.office

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathFillType
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.math.sin

/**
 * Home's painted page, the web's "Home: the painted page" (style.css): the colours the page's
 * furniture takes beyond the palette. [TokensTest] checks them against the stylesheet.
 *
 * By day the invitation lies on a thin wash of yellow ochre and the hour that is now under a
 * pool of the same light; by night both are moonlit blue. The apse's sky, set by night in the
 * head of a phone's arch, is the parish vault's slate blue (`sky*`), ringed by the slate course's
 * outer ring in shade (`skyRim`); a white day's line on it is lime white, so the leaf fillet stays
 * the one gold line; the day's words on it take a lifted muted ink and rubric, and the invitation
 * a lifted lining. The leaf is the season's gilding matted toward the soft, rosy leaf of the
 * parish's photographs (`leafMatte*`).
 */
data class HomeInk(
    val tabletWash: Color,
    val nowPool: Color,
    val nowGloss: Color,
    val skyDeep: Color = Color(0xFF172231),
    val sky: Color = Color(0xFF233440),
    val skyLow: Color = Color(0xFF2F474D),
    val skyRim: Color = Color(0xFF0C131D),
    val limeWhite: Color = Color(0xFFE9E3D0),
    val skyMuted: Color = Color(0xFFB9C3C8),
    val skyRubric: Color = Color(0xFFF0A39C),
    val liningLifted: Color = Color(0xFFE0A686),
)

/**
 * CSS's color-mix in sRGB: `a` at `share` and `b` the rest, premultiplied, as CSS mixes
 * translucent colours. Here rather than beside home's layout, so the tokens test can reach it
 * without loading the screen's drawings.
 */
internal fun mix(a: Color, b: Color, share: Float): Color {
    val alpha = a.alpha * share + b.alpha * (1 - share)
    if (alpha == 0f) return Color.Transparent
    fun channel(x: Float, y: Float) = (x * a.alpha * share + y * b.alpha * (1 - share)) / alpha
    return Color(channel(a.red, b.red), channel(a.green, b.green), channel(a.blue, b.blue), alpha)
}

/** `--ochre`, whose thin washes are Nave's tablet and pool. */
val Ochre = Color(0xFFC9973F)
val LeafMatteHi = Color(0xFFC9A182)
val LeafMatte = Color(0xFFA98765)
val LeafMatteLo = Color(0xFF7D6248)

/** `--leaf-shadow`: the darker leaf a gilt edge falls to. */
val LeafShadow = Color(0xFF8A6A22)

fun homeInk(p: Palette): HomeInk = if (p.dark) {
    HomeInk(tabletWash = Color(132, 160, 204).copy(alpha = 0.12f), nowPool = Color(140, 170, 216).copy(alpha = 0.24f), nowGloss = Color(0xFFC3CCD8))
} else {
    HomeInk(tabletWash = Ochre.copy(alpha = 0.19f), nowPool = Ochre.copy(alpha = 0.34f), nowGloss = mix(p.text, p.muted, 0.55f))
}

/**
 * The current hour's name on its pool: the tituli's ink, lifted by night in the quiet seasons and
 * Passiontide (`--titulus-now`).
 */
fun titulusNow(p: Palette, season: String): Color = when {
    p.dark && season == "quiet" -> Color(0xFFCFC39F)
    p.dark && season == "passiontide" -> Color(0xFFCDC4DC)
    else -> p.titulus
}

/** How much light the leaf catches: the quiet seasons leave it unlit, Passiontide veils it. */
fun leafCatch(season: String): Float = when (season) {
    "quiet" -> 0.6f
    "passiontide" -> 0.7f
    else -> 1f
}

/** The apse's leaf: the season's gilding pair matted toward the parish's own (`--sky-leaf*`). */
class Leaf(o: Ornament) {
    val hi = mix(o.hi, LeafMatteHi, 0.74f)
    val mid = mix(o.flat, LeafMatte, 0.7f)
    val lo = mix(o.lo, LeafMatteLo, 0.62f)
}

/**
 * A ruled line, as the web's `rule()` in app.js: one stroke of a pen from (x1, y1) to (x2, y2),
 * horizontal or vertical, `w` wide, its weight breathing with the hand (seeded, so a line is the
 * same every time) and lifting to a point over the last 4.5 units at each end. The same seeds as
 * the web's, so the fronts rule alike.
 */
fun ruledLine(x1: Float, y1: Float, x2: Float, y2: Float, w: Float, seed: Int, unit: Float = 1f): Path {
    var s = seed.toLong() and 0xFFFFFFFFL
    fun next(): Float {
        s = (s * 1664525L + 1013904223L) and 0xFFFFFFFFL
        return (s.toDouble() / 4294967296.0).toFloat()
    }
    val horizontal = abs(x2 - x1) >= abs(y2 - y1)
    val length = if (horizontal) x2 - x1 else y2 - y1
    val n = max(6, (length / unit / 14f).roundToInt())
    val phase1 = next() * 6.28f
    val phase2 = next() * 6.28f
    val drift = (next() - 0.5f) * 0.5f
    val above = ArrayList<Pair<Float, Float>>()
    val below = ArrayList<Pair<Float, Float>>()
    for (i in 0..n) {
        val t = i.toFloat() / n
        val d = t * length
        val end = min(1f, min(d, length - d) / (4.5f * unit))
        val weight = w * (0.86f + 0.16f * sin(phase1 + t * 5.1f) + 0.08f * sin(phase2 + t * 17f)) * (0.35f + 0.65f * end)
        val c = (drift * (t - 0.5f) + 0.12f * sin(phase2 + t * 3.3f)) * unit
        above.add(d to c - weight / 2f)
        below.add(d to c + weight / 2f)
    }
    val points = above + below.reversed()
    return Path().apply {
        points.forEachIndexed { i, (d, o) ->
            val x = if (horizontal) x1 + d else x1 + o
            val y = if (horizontal) y1 + o else y1 + d
            if (i == 0) moveTo(x, y) else lineTo(x, y)
        }
        close()
    }
}

/** The invitation's double frame (the web's `.pray-now` rules): the lining, then the frontal inside it, short of the corners. */
fun DrawScope.invitationRules(lining: Color, frontal: Color) {
    val u = density
    val w = size.width
    val h = size.height
    val g = 3.5f * u
    drawPath(ruledLine(0f, 0.5f * u, w, 0.5f * u, 1.25f * u, 21, u), lining)
    drawPath(ruledLine(0f, h - 0.5f * u, w, h - 0.5f * u, 1.25f * u, 22, u), lining)
    drawPath(ruledLine(0.5f * u, 0f, 0.5f * u, h, 1.25f * u, 23, u), lining)
    drawPath(ruledLine(w - 0.5f * u, 0f, w - 0.5f * u, h, 1.25f * u, 24, u), lining)
    val day = frontal.copy(alpha = frontal.alpha * 0.5f)
    drawPath(ruledLine(g + 1.5f * u, g, w - g - 1.5f * u, g, 0.7f * u, 25, u), day)
    drawPath(ruledLine(g + 1.5f * u, h - g, w - g - 1.5f * u, h - g, 0.7f * u, 26, u), day)
    drawPath(ruledLine(g, g + 1.5f * u, g, h - g - 1.5f * u, 0.7f * u, 27, u), day)
    drawPath(ruledLine(w - g, g + 1.5f * u, w - g, h - g - 1.5f * u, 0.7f * u, 28, u), day)
}

/** The wash under the invitation, `tablet.png` stretched to it and tinted. */
fun DrawScope.tabletWash(tablet: ImageBitmap, wash: Color) {
    if (wash.alpha == 0f) return
    val u = density
    drawImage(
        tablet,
        dstOffset = IntOffset(u.roundToInt(), u.roundToInt()),
        dstSize = IntSize((size.width - 2 * u).roundToInt(), (size.height - 2 * u).roundToInt()),
        colorFilter = androidx.compose.ui.graphics.ColorFilter.tint(wash, androidx.compose.ui.graphics.BlendMode.SrcIn),
    )
}

/**
 * A brush of the pigment mask (`pigment.png`, tools/genpaint.py), its 256px tile drawn `tile` dp
 * across and repeated: paint with it under a tint (`ColorFilter.tint(ink, BlendMode.SrcIn)`), so
 * the colour stays the token and only the pigment's unevenness comes through.
 */
fun DrawScope.pigmentBrush(pigment: ImageBitmap, tile: Float): Brush {
    val shader = androidx.compose.ui.graphics.ImageShader(pigment, androidx.compose.ui.graphics.TileMode.Repeated, androidx.compose.ui.graphics.TileMode.Repeated)
    val k = tile * density / pigment.width
    shader.setLocalMatrix(android.graphics.Matrix().apply { setScale(k, k) })
    return androidx.compose.ui.graphics.ShaderBrush(shader)
}

/** One of the sky's gold-leaf stars: its centre in card widths, its size and outline in dp. */
class SkyStar(val x: Float, val y: Float, val r: Float, val leaf: Float, val outline: FloatArray)

private fun parse(field: String): List<SkyStar> = field.split(';').map { star ->
    val v = star.split(' ').map { it.toFloat() }
    SkyStar(v[0], v[1], v[2], v[3], v.drop(4).toFloatArray())
}

private val Fields by lazy { mapOf(PhoneArch to parse(StarFields.PHONE), TallArch to parse(StarFields.TALL), TallerArch to parse(StarFields.TALLER)) }

/** The stars of a head's field (tools/genstars.py), or none for a head with no sky. */
fun skyStars(arch: Arch): List<SkyStar> = Fields[arch] ?: emptyList()

/**
 * The apse's sky inside a phone's head by night (the web's `.home-sky`): the slate from deep to
 * light, a lighter and greener glow toward the frieze, and the vault's brushwork (`sky_wash.jpg`)
 * laid over it, clipped to the head inside the slate course and down to the frieze (`band`, from
 * the card's top). Then its gold-leaf stars, leaving out every one that would touch the painter's
 * reserve (`reserve`, rectangles in this scope already padded) or come within 9dp of the frieze;
 * `first` sets them a size larger and fully lit. The fillet and hairline round it are the
 * lining's (`skyLining`).
 */
fun DrawScope.sky(arch: Arch, band: Float, wash: ImageBitmap, ink: HomeInk, leaf: Leaf, catch: Float, stars: Boolean, first: Boolean, reserve: List<Rect>) {
    val u = density
    val w = size.width
    val head = archPath(arch, w, -5f * u, band + 2f * u)
    clipPath(head) {
        val top = -2f * 16f * u
        drawRect(Brush.verticalGradient(0.1f to ink.skyDeep, 0.9f to ink.sky, startY = top, endY = band), Offset(0f, top), Size(w, band - top + 2f * u))
        // radial-gradient(ellipse 85% 60% at 50% 100%, sky-low, transparent)
        val rx = 0.85f * w
        val ry = 0.6f * (band - top)
        val k = rx / ry
        drawContext.transform.scale(1f, 1f / k, Offset(w / 2f, band))
        drawRect(Brush.radialGradient(0f to ink.skyLow, 1f to Color.Transparent, center = Offset(w / 2f, band), radius = rx), Offset(0f, band - rx), Size(w, 2f * rx))
        drawContext.transform.scale(1f, k, Offset(w / 2f, band))
        // The brushwork: at least 23rem wide, run down from the top, in soft light.
        val tileW = max(w, 23f * 16f * u)
        val tileH = tileW * wash.height / wash.width
        var y = top
        while (y < band) {
            drawImage(wash, dstOffset = IntOffset(((w - tileW) / 2f).roundToInt(), y.roundToInt()), dstSize = IntSize(tileW.roundToInt(), tileH.roundToInt()), blendMode = androidx.compose.ui.graphics.BlendMode.Softlight)
            y += tileH
        }
        if (!stars) return@clipPath
        val scale = if (first) 1.14f else 1f
        val light = Brush.radialGradient(0f to leaf.hi, 0.6f to leaf.mid, 1f to leaf.lo, center = Offset(w / 2f, band * 0.62f), radius = w * 0.72f)
        for (s in skyStars(arch)) {
            val cx = s.x * w
            val cy = s.y * w
            val r = s.r * scale * u
            if (cy + r > band - 9f * u) continue
            if (reserve.any { cx + r > it.left && cx - r < it.right && cy + r > it.top && cy - r < it.bottom }) continue
            val path = Path().apply {
                fillType = PathFillType.NonZero
                for (i in s.outline.indices step 2) {
                    val px = cx + s.outline[i] * scale * u
                    val py = cy + s.outline[i + 1] * scale * u
                    if (i == 0) moveTo(px, py) else lineTo(px, py)
                }
                close()
            }
            drawPath(path, light, alpha = catch * if (first) min(1f, s.leaf + 0.25f) else s.leaf)
        }
    }
}

/**
 * The sky's two lines, where the lining stands by day: one matte leaf fillet against the slate
 * (1.5dp, 2dp on a first-class feast), then the day's colour as a hairline 6dp inside it; open
 * at the foot, down to this scope's foot.
 */
fun DrawScope.skyLining(arch: Arch, leaf: Leaf, catch: Float, hairline: Color, first: Boolean) {
    val u = density
    val t = (if (first) 2f else 1.5f) * u
    val fillet = Brush.verticalGradient(0f to leaf.lo, 0.38f to leaf.hi, 0.7f to leaf.mid, 1f to leaf.lo, startY = -32f * u, endY = size.height)
    drawPath(archPath(arch, size.width, -5f * u - t / 2f, size.height, open = true), fillet, alpha = if (first) 1f else 0.9f * catch, style = Stroke(t, miter = 2f))
    drawPath(archPath(arch, size.width, -11.5f * u, size.height, open = true), hairline.copy(alpha = hairline.alpha * 0.85f), style = Stroke(u, miter = 2f))
}
