package org.orthodoxwest.office

import android.graphics.BlurMaskFilter
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathOperation
import androidx.compose.ui.graphics.asAndroidPath
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.drawIntoCanvas
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.nativeCanvas
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.roundToInt
import kotlin.math.sin
import kotlin.math.sqrt

/**
 * The wide home's niche and chapel light ("Home niche" in style.css): on a wide screen the
 * frontispiece is one lancet niche cut into the wall, its head genarch's landscape [NicheArch] or
 * upright [UprightArch], its size one scale ([nicheZoom]), and the room lit toward it. By day its
 * depth is told by light on the reveal ([niche]); by night it is the phone's panel and apse sky
 * ([panel], [sky]) on a plain wall, so the phone and the wide home are one building.
 */
data class NicheTokens(
    val recess: Color,
    val sheen: Color,
    val warm: Color,
    /** The ground the niche is cleared to; the Nave's is none. */
    val clear: Color?,
    val pool: Color,
    val shade: Color,
    val beam: Color,
    /** The reveal by day (`--reveal-*`): shade on the left jamb, light on the right and the sill. */
    val revealShade: Color = Color(0xFFCBBCAA),
    val revealMid: Color = Color(0xFFE6DCCD),
    val revealLit: Color = Color(0xFFFFFDF6),
    val revealSill: Color = Color(0xFFFBF6EA),
    val revealSillEdge: Color = Color(0xFFD9CCBA),
    val revealSoffit: Color = Color(96, 66, 44).copy(alpha = 0.3f),
    val arris: Color = Color(255, 253, 246).copy(alpha = 0.9f),
    val arrisTurn: Color = Color(255, 253, 246).copy(alpha = 0.2f),
    val arrisShade: Color = Color(96, 66, 44).copy(alpha = 0.34f),
    val cast: Color = Color(84, 52, 30).copy(alpha = 0.3f),
)

fun nicheTokens(p: Palette): NicheTokens = if (!p.dark) {
    NicheTokens(
        recess = Color(70, 40, 20).copy(alpha = 0.2f),
        sheen = Color.White.copy(alpha = 0.14f),
        warm = Color(232, 176, 84).copy(alpha = 0.2f),
        clear = null,
        pool = Color(240, 188, 96).copy(alpha = 0.2f),
        shade = Color(88, 60, 38).copy(alpha = 0.36f),
        beam = Color(255, 251, 238).copy(alpha = 0.62f),
    )
} else {
    NicheTokens(
        recess = Color.Black.copy(alpha = 0.42f),
        sheen = Color.Transparent,
        warm = Color(208, 176, 106).copy(alpha = 0.09f),
        clear = p.bg,
        pool = Color(214, 168, 90).copy(alpha = 0.08f),
        shade = Color(3, 7, 12).copy(alpha = 0.62f),
        beam = Color(200, 214, 234).copy(alpha = 0.08f),
    )
}

/**
 * The niche's layout at a size of screen (`width` by `height`, the height home has under the
 * bars), as style.css's "Home niche" sets it: a landscape or upright lancet, the glossed table or,
 * from 1000 high, the list of seven, and one scale for the whole niche.
 */
class NicheLayout(width: Dp, height: Dp) {
    /** An upright screen takes the taller head and a narrower lancet. */
    val upright = height > width
    /** Where the screen has the height, the hours stand one to a line. */
    val list = height >= 1000.dp
    val arch = if (upright) UprightArch else NicheArch
    /** The card's width before the zoom (`--card-w`). */
    val card: Dp = if (upright) 552.dp else 600.dp
    /** The air above the niche before the zoom (`--niche-air`). */
    val air: Dp = if (upright) 38.4.dp else 25.6.dp

    /**
     * The one scale (`--niche-zoom`): the height the screen leaves under the header, the air and
     * the colophon, over the niche's own height at 1 on its heaviest day, no more than a share of
     * the screen's width, never under the least (where every gloss is still 12 on the glass: a
     * laptop's niche keeps its size and lets the colophon go under the fold), and at most 1.5.
     */
    val zoom: Float = run {
        val rem = 16f
        val fit = rem * when {
            upright && list -> 57f
            upright -> 45f
            list -> 54.2f
            else -> 42f
        }
        val chrome = rem * when {
            upright -> 15.6f
            list -> 14.75f
            else -> 14f
        }
        val least = if (upright) 0.9f else 0.96f
        val share = if (upright) 0.8f else 0.62f
        nicheZoom(width.value, height.value, chrome, fit, share, card.value, least)
    }
}

/**
 * `clamp(least, min((height - chrome) / fit, share * width / card), 1.5)`: style.css's
 * `--niche-zoom`, and Niche.swift's `nicheZoom`.
 */
fun nicheZoom(width: Float, height: Float, chrome: Float, fit: Float, share: Float, card: Float, least: Float): Float =
    maxOf(least, minOf((height - chrome) / fit, share * width / card, 1.5f))

/** Draws `path` blurred, for a CSS blur of `blur` (its Gaussian sigma is half the radius). */
private fun DrawScope.blurred(path: Path, color: Color, blur: Float) {
    if (color.alpha == 0f) return
    val paint = android.graphics.Paint().apply {
        isAntiAlias = true
        this.color = color.toArgb()
        if (blur > 0f) maskFilter = BlurMaskFilter(blur / 2f, BlurMaskFilter.Blur.NORMAL)
    }
    drawIntoCanvas { it.nativeCanvas.drawPath(path.asAndroidPath(), paint) }
}

/** The wall outside `shape`, drawn only for the shade its edge casts inside it. */
private fun DrawScope.outside(shape: Path): Path {
    val wall = Path().apply { addRect(Rect(-size.width, -size.height, 2 * size.width, 2 * size.height)) }
    return Path.combine(PathOperation.Difference, wall, shape)
}

/**
 * The niche by day behind the frontispiece, under the pointed head `a`, in the order the web's
 * courses stack, outermost first: the arris where the wall turns into the recess, catching the
 * light on the right and lost in shade on the left; the reveal (`--reveal`, 1.2rem), the left jamb
 * in shade and the right lit, the soffit turning from the light toward the point, the sill between
 * the jambs mitred at its corners and facing the light; the day's colour as one fillet at the back
 * wall's edge (`fillet`); then the back wall, the phone's plaster under a warm glow and sheen, with
 * the shade the head and left jamb cast on it. Each is the arch offset by its own distance. By
 * night the niche is the phone's panel ([panel]) and its sky.
 */
fun DrawScope.niche(a: Arch, t: NicheTokens, p: Palette, fillet: Color, plaster: ImageBitmap) {
    val rem = 16.dp.toPx()
    val px = 1.dp.toPx()
    val w = size.width
    val h = size.height
    val r = 1.2f * rem
    fun shape(d: Float) = archPath(a, w, d, h + d)
    // linear-gradient(100deg, ...) across the courses: near enough level, left to right.
    drawPath(shape(r + 1.5f * px), Brush.horizontalGradient(0f to t.arrisShade, 0.36f to t.arrisShade, 0.52f to t.arrisTurn, 0.7f to t.arris, startX = -r, endX = w + r))
    val reveal = shape(r)
    drawPath(reveal, Brush.horizontalGradient(0f to t.revealShade, 0.34f to t.revealShade, 0.56f to t.revealMid, 0.8f to t.revealLit, startX = -r, endX = w + r))
    clipPath(reveal) {
        drawRect(Brush.verticalGradient(0f to t.revealSoffit, 1f to Color.Transparent, startY = -r, endY = 0.5f * w), topLeft = Offset(-r, -r), size = Size(w + 2 * r, 0.5f * w + r))
        // The sill, mitred: from the jambs' outer corners in to the back wall's.
        val sill = Path().apply {
            moveTo(-r, h + r)
            lineTo(w + r, h + r)
            lineTo(w, h)
            lineTo(0f, h)
            close()
        }
        drawPath(sill, Brush.verticalGradient(0f to t.revealSillEdge, 0.35f to t.revealSill, 1f to t.revealSill, startY = h, endY = h + r))
    }
    drawPath(shape(1.5f * px), fillet)
    val face = shape(0f)
    drawPath(face, p.surface)
    clipPath(face) {
        // The plaster, cover-fitted, then the warm pool under the head and the sheen falling from
        // it, laid from 2rem above the card as the web's courses are.
        val scale = maxOf(w / plaster.width, h / plaster.height)
        val fitted = IntSize((plaster.width * scale).roundToInt(), (plaster.height * scale).roundToInt())
        drawImage(plaster, dstOffset = IntOffset((w.roundToInt() - fitted.width) / 2, 0), dstSize = fitted)
        val top = -2f * rem
        val height = h - top
        val rx = w * 0.7f
        val ry = height * 0.5f
        val crown = Offset(w / 2f, top)
        scale(1f, ry / rx, pivot = crown) {
            drawCircle(Brush.radialGradient(0f to t.warm, 0.72f to Color.Transparent, center = crown, radius = rx), radius = rx, center = crown)
        }
        drawRect(Brush.verticalGradient(0f to t.sheen, 0.3f to Color.Transparent, startY = top, endY = h), topLeft = Offset(0f, top), size = Size(w, height))
        // drop-shadow(0.5rem 0.7rem 0.8rem cast) drop-shadow(0.1rem 0.15rem 0.12rem cast): the head
        // and the left jamb throw their shade on the back wall.
        val wall = outside(face)
        translate(0.5f * rem, 0.7f * rem) { blurred(wall, t.cast, 0.8f * rem) }
        translate(0.1f * rem, 0.15f * rem) { blurred(wall, t.cast, 0.12f * rem) }
    }
}

/**
 * A first-class day's fillet of matte leaf round the niche's head by day (`.home-summary::before`),
 * 3dp over the back wall's edge, open at the foot and down to this scope's foot (the frieze), as
 * the parish's chancel arch is gilt; the jambs and the sill keep the day's fillet.
 */
fun DrawScope.giltHead(a: Arch, o: Ornament) {
    val gilt = Brush.horizontalGradient(0f to mix(o.lo, o.hi, 0.6f), 0.3f to mix(o.lo, o.hi, 0.6f), 0.62f to o.hi, 0.82f to mix(o.hi, Color.White, 0.7f), 1f to o.hi)
    drawPath(archPath(a, size.width, 0f, size.height, open = true), gilt, style = Stroke(3.dp.toPx(), miter = 2f))
}

/**
 * The lining painted round the head on a panel's or niche's back wall (`.home-lining`): the arch
 * again, `inset` inside the card's edge, as a 2dp band of the lining and the day's colour
 * (`hairline`) as a line 8dp inside the band's outer edge; round the head only and open below,
 * down to this scope's foot (the inscription band), so it reads as paint on the wall rather than
 * another edge of the arch. The scope is the card's width, its top the card's.
 */
fun DrawScope.lining(a: Arch, inset: Float, color: Color, hairline: Color, pigment: ImageBitmap? = null) {
    // A miter limit of 2 keeps the point sharp, and bevels the hair's-breadth step that may join
    // the upper arcs there rather than drawing it out into a spike. The pigment lies unevenly on
    // the limewash (pigment.png, as the web's mask).
    val paint = pigment?.let { pigmentBrush(it, 128f) }
    fun line(d: Float, weight: Float, ink: Color) {
        val path = archPath(a, size.width, d - weight / 2f, size.height, open = true)
        if (paint == null) {
            drawPath(path, ink, style = Stroke(weight, miter = 2f))
        } else {
            drawPath(path, paint, style = Stroke(weight, miter = 2f), colorFilter = ColorFilter.tint(ink, BlendMode.SrcIn))
        }
    }
    line(-inset, 2.dp.toPx(), color)
    line(-inset - 8.dp.toPx(), 1.dp.toPx(), hairline)
}

/**
 * A phone's frontispiece: a painted panel under the pointed head `a`, in the niche's family
 * (`.home-hero`), its courses outermost first: a soft halo of the wall's ground that keeps the
 * field off it, the day's colour as a ring at its edge (`ring`), the frame, then the panel with
 * the shade its head casts and, by day, the light caught under the head's edge. By night (the
 * web's Apse phone panel) the frame is a course of slate lit from above, the panel stands a step
 * above the sky with moonlight under the cross, its shade is soft and wide, and the card casts a
 * shadow on the sky instead of the halo.
 */
fun DrawScope.panel(a: Arch, p: Palette, ring: Color, frame: Color, plaster: ImageBitmap) {
    val rem = 16.dp.toPx()
    val px = 1.dp.toPx()
    fun shape(d: Float) = archPath(a, size.width, d, size.height + d)
    val outer = shape(1.5.dp.toPx())
    if (p.dark) {
        // drop-shadow(0 0 0.35rem bg) drop-shadow(0 0.5rem 1rem black 70%): a clearing of the
        // sky's own colour that keeps the stars off the card's edge, over the shadow the card
        // casts on the sky. The second shadow is cast by the card and its clearing, so it reaches
        // as far as the two in quadrature.
        translate(0f, 0.5f * rem) { blurred(outer, Color.Black.copy(alpha = 0.7f), sqrt(0.35f * 0.35f + 1f) * rem) }
        blurred(outer, p.bg, 0.35f * rem)
    } else {
        // drop-shadow(0 0 0.5rem bg) drop-shadow(0 0 0.75rem bg): the second blurs the first
        // again, so it reaches as far as the two in quadrature.
        blurred(outer, p.bg, sqrt(0.5f * 0.5f + 0.75f * 0.75f) * rem)
        blurred(outer, p.bg, 0.5f * rem)
    }
    drawPath(outer, ring)
    val edge = shape(0f)
    drawPath(edge, p.surface)
    if (p.dark) {
        // --arch-stone: the slate course, lit from above, from the courses' top (2rem above the
        // point) to the springing, and level below it.
        drawPath(edge, Brush.verticalGradient(listOf(Color(0xFF3A485E), Color(0xFF2A3648)), startY = -2f * rem, endY = a.rise * size.width))
    } else {
        drawPath(edge, frame)
    }
    val face = shape(if (p.dark) -5 * px else -px)
    drawPath(face, p.surface)
    clipPath(face) {
        // The plaster under the day's words, cover-fitted to the panel as the web's `.home-arch-fill`.
        val scale = maxOf(size.width / plaster.width, size.height / plaster.height)
        val fitted = IntSize((plaster.width * scale).roundToInt(), (plaster.height * scale).roundToInt())
        drawImage(plaster, dstOffset = IntOffset((size.width.roundToInt() - fitted.width) / 2, (size.height.roundToInt() - fitted.height) / 2), dstSize = fitted)
        val wall = outside(face)
        if (p.dark) {
            // The course casts its shade from round the head only, inside the head's own sides
            // (the web's `.home-arch-shade::before`): the web paints it down the sides and foot on
            // the panel, under no shade.
            val head = Path.combine(PathOperation.Intersect, wall, Path().apply { addRect(Rect(5 * px, -size.height, size.width - 5 * px, a.rise * size.width)) })
            // --panel-glow: moonlight under the cross, radii the panel's width and 1.06 of it,
            // centred 0.39 of it below the courses' top (2rem above the point). The web lays it
            // under the plaster and veils both by half; laid over the baked plaster at half its
            // strength it lifts the panel as much and keeps the trowel marks.
            val w = size.width
            val center = Offset(w / 2f, 0.39f * w - 2f * rem)
            scale(1f, 1.06f, pivot = center) {
                drawCircle(Brush.radialGradient(0f to Color(150, 178, 220).copy(alpha = 0.1f), 1f to Color.Transparent, center = center, radius = w), radius = w, center = center)
            }
            // drop-shadow(0 1px 0 moonlit 14%) drop-shadow(0 0.25rem 1rem black 30%): the head's
            // shade soft and wide, and over it the light under the head's edge.
            translate(0f, 0.25f * rem) { blurred(head, Color.Black.copy(alpha = 0.3f), rem) }
            translate(0f, px) { drawPath(head, Color(200, 214, 234).copy(alpha = 0.14f)) }
        } else {
            // drop-shadow(0 0.3rem 0.4rem recess): the shade the head casts on the panel.
            translate(0f, 0.3f * rem) { blurred(wall, nicheTokens(p).recess, 0.4f * rem) }
            // drop-shadow(0 1px 0 white 45%): the light caught under the head's edge.
            translate(0f, px) { drawPath(wall, Color.White.copy(alpha = 0.45f)) }
        }
    }
}

/**
 * Where the pool of light falls on the niche: two thirds of the way down, where the web's
 * viewport-centred pool meets its vertically centred niche at desktop sizes.
 */
const val POOL_DEPTH = 0.66f

/**
 * The chapel's light over the wall, fixed to the screen: a warm pool on the niche (`niche`, its
 * bounds in this layer; the web's 50% 54% until it is placed), the room's edges in shade, and a
 * shaft from a high window spending itself before the floor.
 */
@Composable
fun ChapelLight(t: NicheTokens, niche: Rect? = null, modifier: Modifier = Modifier) {
    Canvas(modifier.fillMaxSize().graphicsLayer { compositingStrategy = CompositingStrategy.Offscreen }) {
        // CSS's radial ellipse: a circle of radius rx, squeezed to ry about its centre, over the whole screen.
        fun ellipse(cx: Float, cy: Float, rx: Float, ry: Float, vararg stops: Pair<Float, Color>) {
            val k = rx / ry
            scale(1f, 1f / k, pivot = Offset(cx, cy)) {
                drawRect(Brush.radialGradient(*stops, center = Offset(cx, cy), radius = rx), topLeft = Offset(0f, cy - cy * k), size = Size(size.width, size.height * k))
            }
        }
        // The shaft: linear-gradient(100deg, transparent 43%, beam 49%, transparent 57%), masked
        // toward the floor. Drawn first, so the mask (DstIn) touches it alone.
        val a = Math.toRadians(100.0)
        val dir = Offset(sin(a).toFloat(), -cos(a).toFloat())
        val len = abs(size.width * dir.x) + abs(size.height * dir.y)
        val c = Offset(size.width / 2f, size.height / 2f)
        drawRect(
            Brush.linearGradient(
                0.43f to Color.Transparent,
                0.49f to t.beam,
                0.57f to Color.Transparent,
                start = c - dir * (len / 2f),
                end = c + dir * (len / 2f),
            ),
        )
        drawRect(
            Brush.verticalGradient(0f to Color.Black, 0.38f to Color.Black.copy(alpha = 0.55f), 0.74f to Color.Transparent),
            blendMode = BlendMode.DstIn,
        )
        val pool = niche?.let { Offset(it.center.x, it.top + it.height * POOL_DEPTH) } ?: Offset(size.width * 0.5f, size.height * 0.54f)
        ellipse(pool.x, pool.y, size.width * 0.4f, size.height * 0.46f, 0f to t.pool, 0.7f to Color.Transparent)
        ellipse(size.width * 0.5f, size.height * 0.52f, size.width * 0.74f, size.height * 0.8f, 0.34f to Color.Transparent, 1f to t.shade)
    }
}
