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
import androidx.compose.ui.graphics.CompositingStrategy
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
import androidx.compose.ui.unit.dp
import kotlin.math.abs
import kotlin.math.cos
import kotlin.math.sin
import kotlin.math.sqrt

/**
 * The desktop home's niche and chapel light ("Home niche" in style.css): on a wide screen the
 * frontispiece is set into the wall under a pointed head, with a stone moulding, the day's colour
 * as a trim, and the room lit toward it. Phones set it in a painted panel ([panel]).
 */
data class NicheTokens(
    val stone: Color,
    val edge: Color,
    val recess: Color,
    val sheen: Color,
    val warm: Color,
    /** The ground the niche is cleared to; the Nave's is none. */
    val clear: Color?,
    val pool: Color,
    val shade: Color,
    val beam: Color,
)

fun nicheTokens(p: Palette): NicheTokens = if (!p.dark) {
    NicheTokens(
        stone = Color(0xFFE5DAC9),
        edge = Color(63, 55, 47).copy(alpha = 0.3f),
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
        stone = Color(0xFF243040),
        edge = Color(208, 176, 106).copy(alpha = 0.3f),
        recess = Color.Black.copy(alpha = 0.42f),
        sheen = Color.Transparent,
        warm = Color(208, 176, 106).copy(alpha = 0.09f),
        clear = p.bg,
        pool = Color(214, 168, 90).copy(alpha = 0.08f),
        shade = Color(3, 7, 12).copy(alpha = 0.62f),
        beam = Color(200, 214, 234).copy(alpha = 0.08f),
    )
}

/** The niche's card width at a screen width: clamp(38rem, 10rem + 38vw, 48rem). */
fun nicheWidth(screen: Dp): Dp = (160.dp + screen * 0.38f).coerceIn(608.dp, 768.dp)

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
 * The niche behind the frontispiece, under the pointed head `a`, in the order the web's courses
 * stack, outermost first: the room's shade under the niche and a field of the Apse's ground round
 * both, the clearing, the moulding's edge and stone, the day's colour as a trim; then the frame
 * and the lit recess, with the shade its head casts. Each is the arch offset by its own distance.
 * The painted lining ([lining]) is the phone panel's, restated at the niche's scale.
 */
fun DrawScope.niche(a: Arch, t: NicheTokens, p: Palette, day: Color, frame: Color) {
    val rem = 16.dp.toPx()
    val px = 1.dp.toPx()
    fun shape(d: Float) = archPath(a, size.width, d, size.height + d)
    // The moulding's silhouette, the source of its halo: the clearing, or with none the stone's edge.
    val outer = shape(if (t.clear != null) 1.25f * rem + px else 0.75f * rem + px)
    t.clear?.let {
        // drop-shadow(0 0 0.9rem clear) round the moulding and its shadow, as the second filter.
        translate(0f, 1.25f * rem) { blurred(outer, it.copy(alpha = it.alpha * t.shade.alpha), 1.54f * rem) }
        blurred(outer, it, 0.9f * rem)
    }
    // drop-shadow(0 1.25rem 1.25rem chapel-shade): the niche's shade on the wall below it.
    translate(0f, 1.25f * rem) { blurred(outer, t.shade, 1.25f * rem) }
    t.clear?.let { drawPath(shape(1.25f * rem + px), it) }
    drawPath(shape(0.75f * rem + px), t.edge)
    drawPath(shape(0.75f * rem), t.stone)
    // The day's colour is a hint at the niche's edge, not a second frame.
    drawPath(shape(1.5.dp.toPx()), day)
    // The frame, the card's edge drawn in its rule, and the recess 2dp inside it.
    val edge = shape(0f)
    drawPath(edge, p.surface)
    drawPath(edge, frame)
    val recess = shape(-2 * px)
    drawPath(recess, p.surface)
    clipPath(recess) {
        // A warm pool under the head, and a sheen falling from it, laid from 2rem above the card
        // as the web's courses are.
        val top = -2f * rem
        val height = size.height - top
        val rx = size.width * 0.7f
        val ry = height * 0.5f
        val crown = Offset(size.width / 2f, top)
        scale(1f, ry / rx, pivot = crown) {
            drawCircle(Brush.radialGradient(0f to t.warm, 0.72f to Color.Transparent, center = crown, radius = rx), radius = rx, center = crown)
        }
        drawRect(
            Brush.verticalGradient(0f to t.sheen, 0.3f to Color.Transparent, startY = top, endY = size.height),
            topLeft = Offset(0f, top),
            size = Size(size.width, height),
        )
        // drop-shadow(0 0.55rem 0.9rem recess): the wall outside the arch casts its shade in,
        // and the recess lies in shadow under the head.
        translate(0f, 0.55f * rem) { blurred(outside(recess), t.recess, 0.9f * rem) }
    }
}

/**
 * The lining painted round the head on a panel's or niche's back wall (`.home-lining`): the arch
 * again, `inset` inside the card's edge, as a 2dp band of the lining and the day's colour
 * (`hairline`) as a line 8dp inside the band's outer edge; round the head only and open below,
 * down to this scope's foot (the inscription band), so it reads as paint on the wall rather than
 * another edge of the arch. The scope is the card's width, its top the card's.
 */
fun DrawScope.lining(a: Arch, inset: Float, color: Color, hairline: Color) {
    fun line(d: Float, weight: Float, ink: Color) =
        drawPath(archPath(a, size.width, d - weight / 2f, size.height, open = true), ink, style = Stroke(weight))
    line(-inset, 2.dp.toPx(), color)
    line(-inset - 8.dp.toPx(), 1.dp.toPx(), hairline)
}

/**
 * A phone's frontispiece: a painted panel under the pointed head `a`, in the niche's family
 * (`.home-hero`), its courses outermost first: a soft halo of the wall's ground that keeps the
 * field off it, the day's colour as a ring at its edge (`ring`), the frame, then the panel with
 * the shade its head casts and, by day, the light caught under the head's edge.
 */
fun DrawScope.panel(a: Arch, p: Palette, ring: Color, frame: Color) {
    val rem = 16.dp.toPx()
    val px = 1.dp.toPx()
    fun shape(d: Float) = archPath(a, size.width, d, size.height + d)
    val outer = shape(1.5.dp.toPx())
    // drop-shadow(0 0 0.5rem bg) drop-shadow(0 0 0.75rem bg), by night 0.35rem and 0.6rem: the
    // second blurs the first again, so it reaches as far as the two in quadrature.
    val (near, far) = if (p.dark) 0.35f to 0.6f else 0.5f to 0.75f
    blurred(outer, p.bg, sqrt(near * near + far * far) * rem)
    blurred(outer, p.bg, near * rem)
    drawPath(outer, ring)
    val edge = shape(0f)
    drawPath(edge, p.surface)
    drawPath(edge, frame)
    val face = shape(-px)
    drawPath(face, p.surface)
    clipPath(face) {
        val wall = outside(face)
        // drop-shadow(0 0.3rem 0.4rem recess): the shade the head casts on the panel.
        translate(0f, 0.3f * rem) { blurred(wall, nicheTokens(p).recess, 0.4f * rem) }
        // drop-shadow(0 1px 0 white 45%), by day: the light caught under the head's edge.
        if (!p.dark) translate(0f, px) { drawPath(wall, Color.White.copy(alpha = 0.45f)) }
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
