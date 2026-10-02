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

/**
 * The desktop home's niche and chapel light ("Home niche" in style.css): on a wide screen the
 * frontispiece is set into the wall under a low round head, with a stone moulding, the day's
 * colour as a trim, and the room lit toward it. Phones set it in a round-headed panel ([panel]).
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

/** The round head's height: clamp(5rem, 2rem + 6vw, 8rem). */
fun nicheHead(screen: Dp): Dp = (32.dp + screen * 0.06f).coerceIn(80.dp, 128.dp)

/**
 * The niche's outline, `outset` beyond the card: a low elliptical head across the whole width
 * (border-radius: 50% 50% 0 0 / head head 0 0), square below.
 */
fun nichePath(size: Size, head: Float, outset: Float): Path = Path().apply {
    val d = outset
    val ry = head + d
    moveTo(-d, size.height + d)
    lineTo(-d, -d + ry)
    arcTo(Rect(-d, -d, size.width + d, -d + 2 * ry), 180f, 180f, false)
    lineTo(size.width + d, size.height + d)
    close()
}

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

/**
 * The niche behind the frontispiece, in the order the web's box-shadows stack, bottom first: a
 * clearing of the Apse's ground, the shadow under the head, the moulding's edge and stone, the
 * day's colour; then the lit recess, its shadow under the head, and the frame. The painted
 * lining ([nicheLining]) is the phone panel's, restated at the niche's scale.
 */
fun DrawScope.niche(t: NicheTokens, p: Palette, day: Color, head: Float, frame: Color) {
    val rem = 16.dp.toPx()
    t.clear?.let {
        blurred(nichePath(size, head, 1.4f * rem), it, 1.8f * rem)
        drawPath(nichePath(size, head, 1.25f * rem), it)
    }
    // 0 1.5rem 3rem -0.75rem: the head casts its shade down the wall.
    translate(0f, 1.5f * rem) { blurred(nichePath(size, head, -0.75f * rem), t.shade, 3f * rem) }
    drawPath(nichePath(size, head, 0.75f * rem + 1.dp.toPx()), t.edge)
    drawPath(nichePath(size, head, 0.75f * rem), t.stone)
    // The day's colour is a hint at the niche's edge, not a second frame.
    drawPath(nichePath(size, head, 1.5.dp.toPx()), day)
    val shape = nichePath(size, head, 0f)
    drawPath(shape, p.surface)
    clipPath(shape) {
        // A warm pool under the head, and a sheen falling from it.
        val rx = size.width * 0.7f
        val ry = size.height * 0.5f
        scale(1f, ry / rx, pivot = Offset(size.width / 2f, 0f)) {
            drawCircle(
                Brush.radialGradient(0f to t.warm, 0.72f to Color.Transparent, center = Offset(size.width / 2f, 0f), radius = rx),
                radius = rx,
                center = Offset(size.width / 2f, 0f),
            )
        }
        drawRect(Brush.verticalGradient(0f to t.sheen, 0.3f to Color.Transparent))
        // inset 0 2.6rem 2.6rem -2rem: everything outside the shape, spread 2rem and dropped
        // 2.6rem, blurred, and seen through the shape: a recess in shadow under the head.
        val hole = nichePath(size, head, 2f * rem).apply { translate(Offset(0f, 2.6f * rem)) }
        val all = Path().apply { addRect(Rect(-4 * rem, -4 * rem, size.width + 4 * rem, size.height + 4 * rem)) }
        blurred(Path.combine(PathOperation.Difference, all, hole), t.recess, 2.6f * rem)
    }
    drawPath(nichePath(size, head, -1.dp.toPx()), frame, style = Stroke(2.dp.toPx()))
}

/**
 * The lining painted round the head on a panel's or niche's back wall: a 2dp band of the lining,
 * and the day's colour (`hairline`) as a line 8dp inside it, round the head only and open below,
 * so it reads as paint on the wall rather than another edge of the arch. Its outer edge stands
 * `side` in from this scope's sides (less than nothing runs it out past them) and `top` down from
 * its top, and runs to the scope's foot; its head is an ellipse across the whole width with
 * vertical radius `ry`, which CSS's inherited radius keeps for both lines.
 */
fun DrawScope.nicheLining(color: Color, hairline: Color, top: Float, side: Float, ry: Float) {
    fun line(inset: Float, weight: Float, ink: Color) {
        val x = side + inset + weight / 2f
        val y = top + inset + weight / 2f
        val r = ry - weight / 2f
        val path = Path().apply {
            moveTo(x, size.height)
            lineTo(x, y + r)
            arcTo(Rect(x, y, size.width - x, y + 2 * r), 180f, 180f, false)
            lineTo(size.width - x, size.height)
        }
        drawPath(path, ink, style = Stroke(weight))
    }
    val band = 2.dp.toPx()
    line(0f, band, color)
    line(band + 8.dp.toPx(), 1.dp.toPx(), hairline)
}

/**
 * A phone's frontispiece: a round-headed painted panel in the niche's family (`.home-hero`), its
 * shadows bottom first: a soft halo of the wall's ground that keeps the field off it, the day's
 * colour as a ring at its edge, then the surface, a highlight along its top (by day), the shade
 * under its head, and the frame.
 */
fun DrawScope.panel(p: Palette, day: Color, frame: Color, head: Float) {
    val rem = 16.dp.toPx()
    // 0 0 1.5rem 0.5rem by day, 0 0 1.25rem 0.35rem by night.
    if (p.dark) blurred(nichePath(size, head, 0.35f * rem), p.bg, 1.25f * rem) else blurred(nichePath(size, head, 0.5f * rem), p.bg, 1.5f * rem)
    drawPath(nichePath(size, head, 1.5.dp.toPx()), day)
    val shape = nichePath(size, head, 0f)
    drawPath(shape, p.surface)
    clipPath(shape) {
        // inset 0 1px 0: the light along the head, the shape less itself dropped a pixel.
        if (!p.dark) {
            val dropped = nichePath(size, head, 0f).apply { translate(Offset(0f, 1.dp.toPx())) }
            drawPath(Path.combine(PathOperation.Difference, shape, dropped), Color.White.copy(alpha = 0.45f))
        }
        // inset 0 1.5rem 1.5rem -1.25rem: the shade under the head, as the niche's recess.
        val hole = nichePath(size, head, 1.25f * rem).apply { translate(Offset(0f, 1.5f * rem)) }
        val all = Path().apply { addRect(Rect(-4 * rem, -4 * rem, size.width + 4 * rem, size.height + 4 * rem)) }
        blurred(Path.combine(PathOperation.Difference, all, hole), nicheTokens(p).recess, 1.5f * rem)
    }
    drawPath(nichePath(size, head, -0.5.dp.toPx()), frame, style = Stroke(1.dp.toPx()))
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
