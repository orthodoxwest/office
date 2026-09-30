package org.orthodoxwest.office

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.foundation.background
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.CompositingStrategy
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** An SVG path, as the web's templates draw it. */
private fun svg(d: String): Path = PathParser().parsePathString(d).toPath()

// Headpiece sprig (macros.html `headpiece`), viewBox 56×20.
private val SprigRule = svg("M2 10h52")
private val SprigStem = svg("M2 15C20 15 34 11 54 5")
private val SprigLeaves = svg(
    "M14 14C8 14 7 9 6 6c6 0 9 3 8 8Zm12-2c-6-1-7-6-7-9 6 1 9 4 7 9Zm11-3c-5-2-5-6-4-9 5 3 7 5 4 9Z" +
        "M16 14c-3 4-7 5-10 4 2-4 6-5 10-4Zm14-3c-2 5-6 7-10 7 1-4 5-7 10-7Zm14-4c-1 5-5 7-9 7 1-4 5-7 9-7Z",
)

// Home frame corner (home.html), viewBox 32×32.
private val CornerRule = svg("M3 29V3h26")
private val CornerLeaf = svg("M7 7c7 0 12 4 13 11C13 18 8 14 7 7Zm0 0 9 8")

/** Home's period engravings (home.html), viewBox 24×16, stroked. */
enum class Period(val paths: List<Path>) {
    MORNING(listOf(svg("M2 13h20M7 13a5 5 0 0 1 10 0M12 2v3M4.2 5.2l2.1 2.1M19.8 5.2l-2.1 2.1"))),
    DAY(
        listOf(
            svg("M8.5 8a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0 -7 0"),
            svg("M12 0.75v1.5M12 13.75v1.5M4.75 8h1.5M17.75 8h1.5M6.9 2.9l1.05 1.05M16.05 12.05l1.05 1.05M6.9 13.1l1.05-1.05M16.05 3.95l1.05-1.05"),
        ),
    ),
    EVENING(listOf(svg("m12 1 1.8 4.7L19 7.5l-5.2 1.8L12 14l-1.8-4.7L5 7.5l5.2-1.8Z"))),
}

// The ordo's abstinence fish (calendar.html `icon-fish`), viewBox 24×12.
private val Fish = svg("M1 6 C5 1.2 13 1.2 17 6 C13 10.8 5 10.8 1 6 Z M16.5 6 L23 1.5 L21.2 6 L23 10.5 Z")

// The Apse vault tile (style.css `--apse-star-tile`), viewBox 132×132.
private val VaultRibs = svg("M0 0L132 132M0 66L66 0M66 132L132 66")
private val VaultStars = svg(
    "M33 25.5L33.52 31.75L36.39 29.61L34.25 32.48L40.5 33L34.25 33.52L36.39 36.39L33.52 34.25L33 40.5L32.48 34.25L29.61 36.39L31.75 33.52L25.5 33L31.75 32.48L29.61 29.61L32.48 31.75Z" +
        "M99 91.5L99.52 97.75L102.39 95.61L100.25 98.48L106.5 99L100.25 99.52L102.39 102.39L99.52 100.25L99 106.5L98.48 100.25L95.61 102.39L97.75 99.52L91.5 99L97.75 98.48L95.61 95.61L98.48 97.75Z",
)
private val VaultSparks = svg("M99 28.2L99.64 32.36L102.2 33L99.64 33.64L99 37.8L98.36 33.64L95.8 33L98.36 32.36ZM33 94.2L33.64 98.36L36.2 99L33.64 99.64L33 103.8L32.36 99.64L29.8 99L32.36 98.36Z")

/** Draws `paths` from a `vw`×`vh` viewBox fitted and centred in this scope, as SVG's default `meet`. */
private fun DrawScope.fitted(vw: Float, vh: Float, mirror: Boolean = false, flip: Boolean = false, draw: DrawScope.() -> Unit) {
    val k = minOf(size.width / vw, size.height / vh)
    translate((size.width - vw * k) / 2f, (size.height - vh * k) / 2f) {
        scale(if (mirror) -k else k, if (flip) -k else k, pivot = Offset.Zero) {
            translate(if (mirror) -vw else 0f, if (flip) -vh else 0f) { draw() }
        }
    }
}

@Composable
private fun Sprig(mirror: Boolean) {
    val o = LocalOrnament.current
    Canvas(Modifier.size(53.dp, 14.4.dp).graphicsLayer { alpha = 0.55f }) {
        fitted(56f, 20f, mirror = mirror) {
            drawPath(SprigRule, o.flat, style = Stroke(0.8f))
            drawPath(SprigStem, o.flat, style = Stroke(0.8f))
            drawPath(SprigLeaves, o.flat)
        }
    }
}

/** The headpiece above a page's title: sprig, cross, sprig. */
@Composable
fun Headpiece(modifier: Modifier = Modifier) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
        Sprig(mirror = false)
        Text("✠", style = TextStyle(fontFamily = CrossFont, fontSize = 11.sp, color = LocalOrnament.current.flat))
        Sprig(mirror = true)
    }
}

/**
 * The double gold hairline that frames a title, broken by `gap` at its centre (where the
 * headpiece sits) and optionally interrupted by a lozenge in the day's colour.
 */
@Composable
fun DoubleRule(modifier: Modifier = Modifier, gap: Dp = 0.dp, lozenge: Color? = null) {
    val o = LocalOrnament.current
    Canvas(modifier.fillMaxWidth().height(9.dp)) {
        val mid = size.height / 2f
        val half = gap.toPx() / 2f
        val line = 1.dp.toPx()
        for (y in listOf(mid - 1.5.dp.toPx(), mid + 1.5.dp.toPx())) {
            if (half > 0f) {
                drawLine(o.line, Offset(0f, y), Offset(size.width / 2f - half, y), line)
                drawLine(o.line, Offset(size.width / 2f + half, y), Offset(size.width, y), line)
            } else {
                drawLine(o.line, Offset(0f, y), Offset(size.width, y), line)
            }
        }
        if (lozenge != null) lozenge(Offset(size.width / 2f, mid), 4.5.dp.toPx(), lozenge, o.flat)
    }
}

/** A diamond, the lorica boards' lozenge: filled, with a gilt edge. */
fun DrawScope.lozenge(center: Offset, r: Float, fill: Color, edge: Color?) {
    val p = Path().apply {
        moveTo(center.x, center.y - r)
        lineTo(center.x + r, center.y)
        lineTo(center.x, center.y + r)
        lineTo(center.x - r, center.y)
        close()
    }
    drawPath(p, fill)
    if (edge != null) drawPath(p, edge, style = Stroke(0.8.dp.toPx()))
}

/** A small free-standing gilt lozenge, as the ✦ that closes a page. */
@Composable
fun Diamond(modifier: Modifier = Modifier, size: Dp = 7.dp) {
    val o = LocalOrnament.current
    Canvas(modifier.size(size)) { lozenge(center, this.size.minDimension / 2f, o.flat, null) }
}

/** One of the four tooled corners of home's frontispiece. */
@Composable
fun FrameCorner(modifier: Modifier, mirror: Boolean, flip: Boolean) {
    val o = LocalOrnament.current
    Canvas(modifier.size(26.dp).graphicsLayer { alpha = 0.5f }) {
        fitted(32f, 32f, mirror = mirror, flip = flip) {
            drawPath(CornerRule, o.flat, style = Stroke(1f))
            drawPath(CornerLeaf, o.flat, style = Stroke(1f))
        }
    }
}

@Composable
fun PeriodIcon(period: Period, color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(19.dp, 13.dp).graphicsLayer { alpha = 0.7f }) {
        fitted(24f, 16f) { period.paths.forEach { drawPath(it, color, style = Stroke(1.2f)) } }
    }
}

@Composable
fun FishIcon(color: Color, modifier: Modifier = Modifier) {
    Canvas(modifier.size(18.dp, 9.dp)) { fitted(24f, 12f) { drawPath(Fish, color) } }
}

/** The plaster wall behind every page, baked from the web's layers (tools/bake-plaster.py). */
@Composable
fun PlasterWall() {
    Image(painterResource(LocalPalette.current.plaster), null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
}

/**
 * The Apse vault (apse-vault.md): a diaper of hairline ribs with eight-ray stars, their soft
 * halos, and four-ray sparks, in the ornament's gold, faded by stops of (fraction, alpha).
 */
fun DrawScope.vaultTiles(ink: Color, fade: List<Pair<Float, Float>>) {
    val tile = 132.dp.toPx()
    val k = tile / 132f
    // Phase from the top centre, as the web anchors home's field.
    val x0 = (size.width / 2f) % tile - tile
    var y = 0f
    while (y < size.height) {
        var x = x0
        while (x < size.width) {
            translate(x, y) {
                scale(k, k, pivot = Offset.Zero) {
                    for (c in listOf(Offset(33f, 33f), Offset(99f, 99f))) {
                        drawCircle(
                            Brush.radialGradient(0f to ink.copy(alpha = 0.28f), 0.3f to ink.copy(alpha = 0.09f), 1f to Color.Transparent, center = c, radius = 12f),
                            radius = 12f,
                            center = c,
                        )
                    }
                    drawPath(VaultRibs, ink.copy(alpha = 0.07f), style = Stroke(1.2f))
                    drawPath(VaultStars, ink.copy(alpha = 0.78f))
                    drawCircle(ink.copy(alpha = 0.78f), 1.7f, Offset(33f, 33f))
                    drawCircle(ink.copy(alpha = 0.78f), 1.7f, Offset(99f, 99f))
                    drawPath(VaultSparks, ink.copy(alpha = 0.5f))
                }
            }
            x += tile
        }
        y += tile
    }
    val mask = Brush.verticalGradient(*fade.map { (at, a) -> at to Color.Black.copy(alpha = a) }.toTypedArray())
    drawRect(mask, blendMode = BlendMode.DstIn)
}

/** A field of the Apse vault behind `content`, faded by stops of (fraction, alpha). */
@Composable
fun VaultField(modifier: Modifier, fade: List<Pair<Float, Float>>) {
    val palette = LocalPalette.current
    if (!palette.dark) return
    val ink = LocalOrnament.current.flat
    Canvas(modifier.graphicsLayer { compositingStrategy = CompositingStrategy.Offscreen }) { vaultTiles(ink, fade) }
}

/** A hairline across the measure. */
@Composable
fun Hairline(color: Color, modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth().height(1.dp).background(color))
}

/** A short vertical hairline between neighbouring items. */
@Composable
fun Divider(color: Color, height: Dp = 14.dp) {
    Box(Modifier.width(1.dp).height(height).background(color))
}
