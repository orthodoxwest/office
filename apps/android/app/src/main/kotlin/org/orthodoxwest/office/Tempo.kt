package org.orthodoxwest.office

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.TweenSpec
import androidx.compose.animation.core.tween

/**
 * The app's motion, `render_blocks::Tempo`: five named motions and three curves, each way of a
 * motion a [Beat]. The web times by the same table (style.css's `--tempo-*` and `--curve-*`) and
 * iOS by Tempo.swift; a test in render-blocks reads this file and fails when a figure here drifts
 * from the table, so change one there first.
 */
object Tempo {
    /** A press: answered at once, let go over a fifth of a second. */
    val touchIn = Beat(ms = 0, delay = 0, curve = Curve.Arrive)
    val touchOut = Beat(ms = 200, delay = 0, curve = Curve.Arrive)

    /** How far bare words dim under the finger ([Dim]), and the hour that is now's words when pressed: the web's `--tempo-touch-quiet`. */
    const val TOUCH_QUIET = 0.7f

    /** What a press moves rather than lights (the ordo day's rail running the row's height) reaches its place over this; it goes back over [touchOut]. */
    val touchReach = Beat(ms = 120, delay = 0, curve = Curve.Arrive)

    /**
     * The year's course laid in under the frontispiece's month strip, once a launch, never with
     * animations off: one stroke crossing January to December, each month its twelfth of it.
     */
    const val COURSE_LAY_MS = 815

    /** The stroke's pace, a brush's: it keeps moving to the end (the web's `--tempo-course-lay-curve`). */
    val courseLayCurve = CubicBezierEasing(0.38f, 0.2f, 0.6f, 0.92f)

    /** The consecration cross under the last Amen taking its gold, once the hour is marked prayed and the cross stands wholly on screen: evenly, by opacity alone (`presentation::EndLeaf`). */
    val gild = Beat(ms = 2000, delay = 0, curve = Curve.Standard)

    /** A small panel settling under the beam, or a word's note fading in; it leaves faster than it came. */
    val settleIn = Beat(ms = 160, delay = 0, curve = Curve.Arrive)
    val settleOut = Beat(ms = 120, delay = 0, curve = Curve.Leave)

    /** A disclosure unfolding, and folding. */
    val unfoldIn = Beat(ms = 260, delay = 0, curve = Curve.Standard)
    val unfoldOut = Beat(ms = 200, delay = 0, curve = Curve.Standard)

    /** Page to page: the old page goes, then the new one comes; never two at once. */
    val turnIn = Beat(ms = 210, delay = 90, curve = Curve.Arrive)
    val turnOut = Beat(ms = 90, delay = 0, curve = Curve.Leave)

    /** The room changing as a whole: theme and text size. */
    val dimIn = Beat(ms = 320, delay = 0, curve = Curve.Standard)
    val dimOut = Beat(ms = 320, delay = 0, curve = Curve.Standard)
}

/** One way of a motion: its length and the wait before it, in milliseconds, on one of the curves. */
data class Beat(val ms: Int, val delay: Int, val curve: Curve) {
    /** The beat as a tween, its wait included. */
    fun <T> spec(): TweenSpec<T> = tween(durationMillis = ms, delayMillis = delay, easing = curve.easing)
}

/**
 * The three curves, as CSS cubic-beziers. They are Compose's `FastOutSlowInEasing`,
 * `LinearOutSlowInEasing` and `FastOutLinearInEasing`, spelled out so the test can read them.
 */
enum class Curve(x1: Float, y1: Float, x2: Float, y2: Float) {
    /** Something that changes in place. */
    Standard(0.4f, 0f, 0.2f, 1f),

    /** Something coming into view, or let go. */
    Arrive(0f, 0f, 0.2f, 1f),

    /** Something going away. */
    Leave(0.4f, 0f, 1f, 1f),
    ;

    val easing = CubicBezierEasing(x1, y1, x2, y2)
}
