package org.orthodoxwest.office

import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.Path
import kotlin.math.PI
import kotlin.math.atan2
import kotlin.math.hypot
import kotlin.math.sqrt

/**
 * Home's pointed head, the web's (tools/genarch.py): a steep four-centred arch, short haunch arcs
 * centred on the springing line so the head leaves the jambs without a kink, then long upper arcs
 * centred below it, meeting at the point. Its figures are in card widths: the head rises `rise`
 * from the springing to the point; each haunch is an arc of radius `haunch` centred on the
 * springing line `haunch` in from its side; each upper arc, of radius `radius`, is centred
 * `centre` in from its side and `depth` below the springing.
 */
data class Arch(val rise: Float, val haunch: Float, val centre: Float, val depth: Float, val radius: Float)

// genarch:begin: tools/genarch.py writes this block; edit the script, not the figures.
/** The phone head, for a phone: a 144° point. */
val PhoneArch = Arch(rise = 0.44f, haunch = 0.3f, centre = 0.9f, depth = 0.781098f, radius = 1.284944f)
/** The tall head, for a phone from 800 high: a 131° point. */
val TallArch = Arch(rise = 0.56f, haunch = 0.4f, centre = 1f, depth = 0.535519f, radius = 1.204227f)
/** The taller head, for a phone from 880 high: a 123° point. */
val TallerArch = Arch(rise = 0.64f, haunch = 0.4f, centre = 1.05f, depth = 0.364522f, radius = 1.145236f)
/** The niche head, for a wide screen's niche: a 146° point. */
val NicheArch = Arch(rise = 0.4f, haunch = 0.24f, centre = 0.86f, depth = 0.795773f, radius = 1.248789f)
// genarch:end

private fun degrees(radians: Float): Float = radians * 180f / PI.toFloat()

/**
 * The head of a card `width` wide, `d` beyond its edge (inside it when negative), offset as a
 * moulding's lines are: each arc keeps its centre and gains `d` on its radius, and the upper arcs
 * meet on the centre line. The card's top is the point of the head itself, its springing `rise`
 * widths below; the jambs run down to `foot`. `open` leaves the foot unclosed, for a line painted
 * round the head.
 */
fun archPath(a: Arch, width: Float, d: Float, foot: Float, open: Boolean = false): Path {
    val spring = a.rise * width
    val haunch = a.haunch * width
    val centre = a.centre * width
    val below = spring + a.depth * width
    val radius = a.radius * width + d
    // Angles clockwise from three o'clock: the haunch meets the upper arc on the line through
    // their centres, and the upper arcs meet on the centre line.
    val tangent = degrees(atan2(-a.depth, a.haunch - a.centre)) + 360f
    val across = width / 2f - centre
    val point = degrees(atan2(-sqrt(radius * radius - across * across), across)) + 360f
    return Path().apply {
        moveTo(-d, foot)
        lineTo(-d, spring)
        arcTo(Rect(Offset(haunch, spring), haunch + d), 180f, tangent - 180f, false)
        arcTo(Rect(Offset(centre, below), radius), tangent, point - tangent, false)
        arcTo(Rect(Offset(width - centre, below), radius), 540f - point, point - tangent, false)
        arcTo(Rect(Offset(width - haunch, spring), haunch + d), 540f - tangent, tangent - 180f, false)
        lineTo(width + d, foot)
        if (!open) close()
    }
}

/**
 * How wide the head of a card `width` wide stands `y` below the card's top, between lines `d`
 * beyond its edges (inside them when negative) as [archPath] draws them: the jambs below the
 * springing, the haunches above it, then the upper arcs, and nothing above the point.
 */
fun archChord(a: Arch, width: Float, d: Float, y: Float): Float {
    val spring = a.rise * width
    val haunch = a.haunch * width
    val centre = a.centre * width
    val below = spring + a.depth * width
    // The haunch gives way to the upper arc on the line through their centres.
    val join = spring - (haunch + d) * a.depth / hypot(a.depth, a.centre - a.haunch)
    val left = when {
        y >= spring -> -d
        y >= join -> haunch - sqrt((haunch + d) * (haunch + d) - (spring - y) * (spring - y))
        else -> {
            val radius = a.radius * width + d
            val up = below - y
            if (up >= radius) return 0f
            centre - sqrt(radius * radius - up * up)
        }
    }
    return maxOf(0f, width - 2f * left)
}
