package org.orthodoxwest.office

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.EnterExitState
import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.FastOutLinearInEasing
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearOutSlowInEasing
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.expandVertically
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.shrinkVertically
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.IndicationNodeFactory
import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.Orientation
import androidx.compose.foundation.gestures.draggable
import androidx.compose.foundation.gestures.rememberDraggableState
import androidx.compose.foundation.interaction.FocusInteraction
import androidx.compose.foundation.interaction.InteractionSource
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.PressInteraction
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.foundation.relocation.BringIntoViewRequester
import androidx.compose.foundation.relocation.bringIntoViewRequester
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.dropShadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.RectangleShape
import androidx.compose.ui.graphics.drawscope.ContentDrawScope
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.isSpecified
import androidx.compose.ui.graphics.shadow.Shadow
import androidx.compose.ui.hapticfeedback.HapticFeedback
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.layout.LayoutCoordinates
import androidx.compose.ui.node.CompositionLocalConsumerModifierNode
import androidx.compose.ui.node.DelegatableNode
import androidx.compose.ui.node.DrawModifierNode
import androidx.compose.ui.node.currentValueOf
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.res.imageResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.DpOffset
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.isSpecified
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupProperties
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.Month
import java.time.YearMonth
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle as JavaTextStyle
import java.util.Locale
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import org.orthodoxwest.office.core.preferenceHints

/** The reading measure's side gutter (`--page-gutter` on a phone). */
val Gutter: Dp = 16.dp

/** Wide screens keep the web's prayer measure (`.elements`, 38rem). */
val Measure: Dp = 608.dp

fun Modifier.measure(): Modifier = this.widthIn(max = Measure).fillMaxWidth().padding(horizontal = Gutter)

/** A tap target with no ripple: the web's controls mark state, not touches; a pressed one only dims a little, or, a row, takes the light ([Lit]). */
fun Modifier.tap(
    role: Role = Role.Button,
    label: String? = null,
    selected: Boolean? = null,
    action: String? = null,
    lit: Lit? = null,
    source: MutableInteractionSource? = null,
    onClick: () -> Unit,
): Modifier =
    this.semantics {
        // What a screen reader says for a control drawn as a glyph ("‹", "↑"), in place of the glyph.
        if (label != null) contentDescription = label
        if (selected != null) this.selected = selected
    }.clickable(
        // A part of a row of several targets lights the row, which draws the light from the same
        // `source` ([Modifier.indication] with a [Lit]); the part itself neither dims nor lights.
        interactionSource = source,
        indication = if (source != null) null else lit ?: Dim,
        role = role,
        onClickLabel = action,
        onClick = onClick,
    )

/** A checkbox row: the box and its words are one control, announced checked or not. */
fun Modifier.check(checked: Boolean, onChange: (Boolean) -> Unit): Modifier =
    this.toggleable(checked, interactionSource = null, indication = Dim, role = Role.Checkbox, onValueChange = onChange)

/**
 * A light tick under the finger as a choice is made (a setting, a prayer form, a day, a month
 * swiped), as the iOS app gives; never for a page opened. The phone's own touch-feedback setting
 * governs it.
 */
fun HapticFeedback.chose() = performHapticFeedback(HapticFeedbackType.SegmentTick)

/**
 * The tick of a checkbox turned on or off. Before Android 14, whose segment and toggle constants
 * (API 34) an older phone does not know, Compose plays them through `ViewCompat`, which falls back
 * to the context click (a tick, and on) and the clock's tick (off), so no guard is needed here.
 */
fun HapticFeedback.toggled(on: Boolean) = performHapticFeedback(if (on) HapticFeedbackType.ToggleOn else HapticFeedbackType.ToggleOff)

/**
 * A pressed control's answer to the finger: it dims at once to Tempo's quiet (the web's bare
 * words, the iOS app's `Quiet`), and comes back over Tempo's touch when let go, so even the briefest tap is seen to land. A control that takes input focus from a
 * keyboard or D-pad (never by touch, where a tap does not take focus) wears the focus ring.
 */
internal object Dim : IndicationNodeFactory {
    override fun create(interactionSource: InteractionSource): DelegatableNode = DimNode(interactionSource)
    override fun equals(other: Any?) = other === this
    override fun hashCode() = 0
}

private class DimNode(private val source: InteractionSource) : Modifier.Node(), DrawModifierNode, CompositionLocalConsumerModifierNode {
    private val alpha = Animatable(1f)
    private val layer = Paint()
    /** Whether the control has the focus, and so wears the ring. */
    private var focused by mutableStateOf(false)

    override fun onAttach() {
        // Undispatched, so the first focus after the clickable makes this node lazily is heard.
        coroutineScope.launch(start = CoroutineStart.UNDISPATCHED) {
            val presses = mutableListOf<PressInteraction.Press>()
            val focuses = mutableListOf<FocusInteraction.Focus>()
            source.interactions.collect { interaction ->
                when (interaction) {
                    is PressInteraction.Press -> presses.add(interaction)
                    is PressInteraction.Release -> presses.remove(interaction.press)
                    is PressInteraction.Cancel -> presses.remove(interaction.press)
                    is FocusInteraction.Focus -> {
                        focuses.add(interaction)
                        focused = true
                        return@collect
                    }
                    is FocusInteraction.Unfocus -> {
                        focuses.remove(interaction.focus)
                        focused = focuses.isNotEmpty()
                        return@collect
                    }
                }
                if (presses.isEmpty()) {
                    launch { alpha.animateTo(1f, Tempo.touchOut.spec()) }
                } else {
                    launch { alpha.snapTo(Tempo.TOUCH_QUIET) }
                }
            }
        }
    }

    override fun ContentDrawScope.draw() {
        val a = alpha.value
        if (a >= 1f) {
            drawContent()
        } else {
            layer.alpha = a
            drawContext.canvas.saveLayer(Rect(Offset.Zero, size), layer)
            drawContent()
            drawContext.canvas.restore()
        }
        if (focused) focusRing(currentValueOf(LocalPalette).gold)
    }
}

/**
 * The web's ring (`:focus-visible`): a 2px line 3px off the control, square, here in the gold of
 * the app's functional marks (the web's home and date link wear gold). Drawn in place, as the
 * web's appears. [Dim] and [Lit] both wear it.
 */
private fun DrawScope.focusRing(gold: Color) {
    val width = FOCUS_RING.toPx()
    val gap = FOCUS_GAP.toPx() + width / 2f
    drawRect(
        gold,
        topLeft = Offset(-gap, -gap),
        size = Size(size.width + 2 * gap, size.height + 2 * gap),
        style = Stroke(width),
    )
}

/**
 * Where a row's pool of light lies: an ellipse with no edge (the web's
 * `radial-gradient(closest-side, pool, pool 55% at 52%, transparent)`), centred [at] the fraction
 * of the row's width, [width] and [height] its fractions of the row's, or, where [span] is given,
 * that wide from [lead] before the row's start; [outsetX] and [outsetY] reach past its edges, and it
 * is at most [max] across.
 */
data class PoolShape(
    val at: Float = 0.5f,
    val width: Float = 1f,
    val height: Float = 1f,
    val outsetX: Dp = 0.dp,
    val outsetY: Dp = 0.dp,
    val max: Dp = Dp.Infinity,
    val lead: Dp = 0.dp,
    val span: Dp = Dp.Unspecified,
) {
    companion object {
        /** A row's long lozenge under its words (the web's `ellipse 44% 58% at 30% 50%`). */
        val Row = PoolShape(at = 0.3f, width = 0.88f, height = 1.16f)
    }
}

/** Home's pool of light in [color], laid as [shape] over a box of this scope's size, at [alpha]. */
fun DrawScope.poolOfLight(color: Color, shape: PoolShape, alpha: Float = 1f) {
    if (alpha <= 0f) return
    val w = if (shape.span.isSpecified) shape.span.toPx() else minOf(size.width * shape.width + 2 * shape.outsetX.toPx(), if (shape.max == Dp.Infinity) Float.MAX_VALUE else shape.max.toPx())
    val h = size.height * shape.height + 2 * shape.outsetY.toPx()
    val cx = if (shape.span.isSpecified) w / 2f - shape.lead.toPx() else size.width * shape.at
    val r = w / 2f
    val c = Offset(cx, size.height / 2f)
    val pool = color.copy(alpha = color.alpha * alpha)
    scale(1f, h / w, pivot = c) { drawCircle(poolBrush(pool, c, r), r, c) }
}

/**
 * Light under the finger, beside [Dim] for rows: a pressed row (home's hours, the ordo's days, the
 * picker's days, About's hours, the menu's cells) lies under home's pool of light, the edgeless
 * pool that marks the hour that is now, at once, and lets it go over Tempo's touch; nothing scales.
 * [strength] is how much of the pool a press shows (home's hours take 70% by day; the moonlit pool
 * is faint by night). The hour that is now is lit already ([current]): a press deepens its pool by
 * half again and its words withdraw to 70% (the web's `.home-hour-link.is-current:active`). A row
 * reached by keyboard or D-pad wears the focus ring, as [Dim]'s controls do.
 */
data class Lit(val pool: Color, val shape: PoolShape = PoolShape(), val strength: Float = 1f, val current: Boolean = false) : IndicationNodeFactory {
    override fun create(interactionSource: InteractionSource): DelegatableNode = LitNode(interactionSource, this)
}

private class LitNode(private val source: InteractionSource, private val lit: Lit) : Modifier.Node(), DrawModifierNode, CompositionLocalConsumerModifierNode {
    private val held = Animatable(0f)
    private val layer = Paint()
    /** Whether the row has the focus, and so wears the ring. */
    private var focused by mutableStateOf(false)

    override fun onAttach() {
        // Undispatched, as [Dim]'s, so the first focus is heard.
        coroutineScope.launch(start = CoroutineStart.UNDISPATCHED) {
            val presses = mutableListOf<PressInteraction.Press>()
            val focuses = mutableListOf<FocusInteraction.Focus>()
            source.interactions.collect { interaction ->
                when (interaction) {
                    is PressInteraction.Press -> presses.add(interaction)
                    is PressInteraction.Release -> presses.remove(interaction.press)
                    is PressInteraction.Cancel -> presses.remove(interaction.press)
                    is FocusInteraction.Focus -> {
                        focuses.add(interaction)
                        focused = true
                        return@collect
                    }
                    is FocusInteraction.Unfocus -> {
                        focuses.remove(interaction.focus)
                        focused = focuses.isNotEmpty()
                        return@collect
                    }
                }
                if (presses.isEmpty()) {
                    launch { held.animateTo(0f, Tempo.touchOut.spec()) }
                } else {
                    launch { held.snapTo(1f) }
                }
            }
        }
    }

    override fun ContentDrawScope.draw() {
        val h = held.value
        poolOfLight(lit.pool, lit.shape, h * if (lit.current) 0.5f else lit.strength)
        if (h > 0f && lit.current) {
            layer.alpha = 1f - (1f - Tempo.TOUCH_QUIET) * h
            drawContext.canvas.saveLayer(Rect(Offset.Zero, size), layer)
            drawContent()
            drawContext.canvas.restore()
        } else {
            drawContent()
        }
        if (focused) focusRing(currentValueOf(LocalPalette).gold)
    }
}

/** The focus ring's line, and its distance from the control (style.css `:focus-visible`). */
private val FOCUS_RING = 2.dp
private val FOCUS_GAP = 3.dp

/** "Expanded" or "Collapsed", after a disclosure's name. */
fun Modifier.disclosed(open: Boolean): Modifier = this.semantics { stateDescription = if (open) "Expanded" else "Collapsed" }

/**
 * The disclosure caret, as the web's `▾`/`▴`: in the ink of the words it opens, a little lighter. Drawn as the web's glyph
 * measures beside a label (5.5 by 4.5 at 0.85em, centred on the capitals) and scaled with the reader's text size, rather
 * than left to whichever face sets `▾`.
 */
@Composable
fun Caret(open: Boolean, ink: Color, lighter: Boolean = true) = Caret(open, ink, lighter, 5.5f, 5.5f, 4.5f)

/**
 * The caret beside a section heading that opens (the web's `.session-prayers summary::after`): the
 * glyph at 0.75em of the 19.2px heading, 0.35em from it, rather than the small label's.
 */
@Composable
fun HeadingCaret(open: Boolean, ink: Color) = Caret(open, ink, true, 5.8f, 8.1f, 6.6f)

@Composable
private fun Caret(open: Boolean, ink: Color, lighter: Boolean, gapSp: Float, wSp: Float, hSp: Float) {
    val (gap, w, h) = with(LocalDensity.current) { Triple(gapSp.sp.toDp(), wSp.sp.toDp(), hSp.sp.toDp()) }
    val fill = if (lighter) ink.copy(alpha = ink.alpha * 0.7f) else ink
    // Drawn, not read: the disclosure says expanded or collapsed.
    Canvas(Modifier.padding(start = gap).size(w, h).clearAndSetSemantics {}) {
        val path = Path().apply {
            if (open) {
                moveTo(0f, size.height); lineTo(size.width, size.height); lineTo(size.width / 2, 0f)
            } else {
                moveTo(0f, 0f); lineTo(size.width, 0f); lineTo(size.width / 2, size.height)
            }
            close()
        }
        drawPath(path, fill)
    }
}

/** A current control's underline: gold for a chosen setting, or `width` thick in the lining for the current page. */
fun Modifier.goldUnderline(on: Boolean, color: Color, inset: Dp = 0.dp, width: Dp = 1.dp): Modifier = if (!on) this else this.drawBehind {
    val y = size.height - width.toPx() / 2f
    drawLine(color, Offset(inset.toPx(), y), Offset(size.width - inset.toPx(), y), width.toPx())
}

/** The web's breakpoint (style.css `min-width: 701px`): the desktop composition from here up. */
val WideFrom: Dp = 701.dp

/** Whether the page is laid out at the web's desktop widths: a tablet, or a phone on its side. */
val LocalWide = staticCompositionLocalOf { false }

/** Where the hours' header holds its links beside the brand and Settings (style.css `max-width: 999px`). */
val RankedBelow: Dp = 1000.dp

/**
 * Whether a wide page is narrower than the hours' header holds on one line (701–999dp): there an
 * hour's links take a rank of their own under the brand and Settings.
 */
val LocalRanked = staticCompositionLocalOf { false }

/** The reader's theme and text size, for the wide header's Settings (the phone's are in the menu). */
class Prefs(val theme: ThemeChoice, val onTheme: (ThemeChoice) -> Unit, val textSize: TextSize, val onTextSize: (TextSize) -> Unit)

val LocalPrefs = staticCompositionLocalOf<Prefs?> { null }

/** The reader's setting for reading the Martyrology at Prime (off by default), for the menu's last row. */
class MartyrologyChoice(val on: Boolean, val onChange: (Boolean) -> Unit)

val LocalMartyrology = staticCompositionLocalOf { MartyrologyChoice(false) {} }

/** The reader's Chant setting, offered only once a text has a score. */
class ChantChoice(val offered: Boolean, val on: Boolean, val onChange: (Boolean) -> Unit)

val LocalChant = staticCompositionLocalOf { ChantChoice(false, false) {} }

/**
 * The header's overlay, at most one open at a time: the menu on a narrow screen, the wide
 * header's Settings on a wide one. A tap outside, Back, any move to another page and crossing the
 * breakpoint all close it. The web holds the same three states (app.js) and so does iOS
 * (Model.swift).
 */
enum class Overlay {
    NONE,
    MENU,
    SETTINGS;

    /** After a tap on `shown`'s control: it opens, in place of any other, or closes if it was open. */
    fun toggled(shown: Overlay): Overlay = if (this == shown) NONE else shown
}

/**
 * Where the site's navigation leads, and which of it is the page shown: the day's hours on an
 * hour page, then the Ordo, Reminders and About (`about` is its name, the core's). The menu sets
 * it out on a phone, the header inline on a wide screen.
 */
class SiteNav(
    val hours: List<String>,
    val currentHour: String?,
    val onHour: ((String) -> Unit)?,
    val onOrdo: () -> Unit,
    val ordoCurrent: Boolean,
    val onReminders: () -> Unit,
    val remindersCurrent: Boolean,
    val about: String,
    val onAbout: () -> Unit,
    val aboutCurrent: Boolean,
    val settingsOpen: Boolean = false,
    val onSettings: () -> Unit = {},
)

/** The header beam: the consecration cross and "Daily Office" home, and the menu, or on a wide screen the links themselves. */
@Composable
fun SiteHeader(onHome: () -> Unit, menuOpen: Boolean, onMenu: () -> Unit, nav: SiteNav? = null) {
    val p = LocalPalette.current
    val wide = LocalWide.current && nav != null
    // From 701 to 999dp an hour's links don't fit beside the brand and Settings: left in the row
    // they would crowd it, so they take a centred rank of their own beneath, as a book sets its
    // running head over the page.
    val rank = wide && LocalRanked.current && nav?.onHour != null
    Column {
        Row(
            // The web's nav shell: held to 68rem, so the whole list fits on one line.
            Modifier.fillMaxWidth().wrapContentWidth().widthIn(max = if (wide) 1088.dp else Dp.Infinity).fillMaxWidth()
                .padding(start = Gutter, end = Gutter, top = 6.4.dp, bottom = if (rank) 0.dp else 5.6.dp).heightIn(min = 44.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            // (On home each of the header's words is a row of the crown's play: Crown.kt.)
            Row(Modifier.crownUnit("r-brand").tap(label = "Daily Office, home", onClick = onHome).padding(vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                // The brand's mark, 18dp: the roundel that ends every hour. Centred on the
                // row it sits on the text box's middle, 0.21em above the baseline (0.71em
                // ascent, 0.29em descent); the capitals' middle is 0.325em above it. Raised by
                // the difference, as the web's phone header raises it, it centres on the caps.
                ConsecrationCross(Modifier.offset(y = -(14.4f * 0.115f).dp).size(18.dp))
                // A word space follows the mark's margin on a wide screen; the phone's brand is a
                // flex box, whose words drop the space they begin with.
                Text(if (LocalWide.current) " DAILY OFFICE" else "DAILY OFFICE", Modifier.padding(start = 6.08.dp), style = Type.brand.copy(color = p.text))
            }
            Spacer(Modifier.weight(1f))
            if (rank) {
                InlineNav(nav!!, links = false, modifier = Modifier.crownUnit("r-nav"))
            } else if (wide) {
                InlineNav(nav!!, modifier = Modifier.crownUnit("r-nav"))
            } else {
                Row(Modifier.crownUnit("r-menu").tap(label = "Menu", onClick = onMenu).disclosed(menuOpen).padding(start = 12.8.dp, end = 3.2.dp).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("MENU", style = Type.menu.copy(color = p.accent))
                    Caret(menuOpen, p.accent)
                }
            }
        }
        if (rank) {
            Box(Modifier.fillMaxWidth().padding(top = 2.4.dp, bottom = 5.6.dp), contentAlignment = Alignment.Center) {
                InlineNav(nav!!, settings = false, modifier = Modifier.crownUnit("r-links"))
            }
        }
        // The beam: a 4dp course of oak, its upper edge catching the light as a timber's arris does.
        Box(
            Modifier.fillMaxWidth().height(5.dp).drawBehind {
                drawRect(p.materialHighlight, size = size.copy(height = 1.dp.toPx()))
                drawRect(p.oak.copy(alpha = 0.55f), Offset(0f, 1.dp.toPx()), size.copy(height = 4.dp.toPx()))
            },
        )
    }
}

/**
 * The desktop header's links (`.site-menu nav`): muted, the current one in ink over the lining's
 * rule; Reminders quieter. `links` and `settings` choose the pages' links and Settings after them,
 * which a ranked header sets apart.
 */
@Composable
private fun InlineNav(nav: SiteNav, links: Boolean = true, settings: Boolean = true, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        @Composable
        fun link(label: String, current: Boolean, secondary: Boolean = false, onClick: () -> Unit) {
            val style = if (secondary) Type.label(11.52f, 0.04f) else Type.label(12.48f, 0.06f)
            val ink = if (current) p.text else p.muted
            Box(Modifier.heightIn(min = 44.dp).tap(selected = current, onClick = onClick).padding(horizontal = 4.8.dp), contentAlignment = Alignment.Center) {
                Text(
                    label.uppercase(),
                    Modifier.drawBehind {
                        if (current) drawLine(p.lining, Offset(0f, size.height + 4.dp.toPx()), Offset(size.width, size.height + 4.dp.toPx()), 1.5.dp.toPx())
                    },
                    style = style.copy(color = ink),
                )
            }
        }
        if (links) {
            nav.onHour?.let { onHour ->
                nav.hours.forEach { h -> link(hourLabel(h), h == nav.currentHour) { onHour(h) } }
                Box(Modifier.padding(horizontal = 2.4.dp).width(1.dp).height(24.dp).background(p.border))
            }
            link("Ordo", nav.ordoCurrent, onClick = nav.onOrdo)
            link("Reminders", nav.remindersCurrent, secondary = true, onClick = nav.onReminders)
            // A wide hour's header is full with the seven hours: About is left to the other pages'
            // headers, as on the web.
            if (nav.onHour == null) link(nav.about, nav.aboutCurrent, secondary = true, onClick = nav.onAbout)
        }
        // Settings closes the links, quiet as Reminders; its panel holds the theme and text size.
        if (settings) {
            Row(
                Modifier.heightIn(min = 44.dp).tap(label = "Settings", onClick = nav.onSettings).disclosed(nav.settingsOpen)
                    .padding(start = 8.8.dp, end = 3.2.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("SETTINGS", style = Type.label(11.52f, 0.04f).copy(color = p.muted))
                Caret(nav.settingsOpen, p.muted)
            }
        }
    }
}

/** A row of equal cells in the menu grid, 44dp tall. */
@Composable
private fun MenuRow(content: @Composable RowScope.() -> Unit) {
    Row(Modifier.fillMaxWidth().heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically, content = content)
}

/**
 * A menu cell, muted until it is the one: a `page` (an hour, the Ordo, Reminders, About) is in ink
 * over the lining's rule when current, a chosen setting in the accent over the ornament's
 * (`--ornament`). `spoken` names a cell drawn as a glyph.
 */
@Composable
private fun RowScope.MenuCell(label: String, current: Boolean, style: TextStyle, page: Boolean = false, spoken: String? = null, onClick: () -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val haptics = LocalHapticFeedback.current
    Box(
        Modifier.weight(1f).heightIn(min = 44.dp).tap(label = spoken, selected = current, lit = Lit(homeInk(p).nowPool)) {
            if (!page && !current) haptics.chose()
            onClick()
        }
            .then(if (page) Modifier.goldUnderline(current, p.lining, width = 1.5.dp) else Modifier.goldUnderline(current, o.flat)),
        contentAlignment = Alignment.Center,
    ) { Text(label, style = style.copy(color = if (!current) p.muted else if (page) p.text else p.accent, textAlign = TextAlign.Center)) }
}

/**
 * The site menu's dropdown panel: on an hour, the day's hours (2/3/2 as on home, a hairline between
 * the bands); the Ordo, Reminders and About; then the Theme, Text and Martyrology rows, the current
 * choice in the accent over the ornament's rule. The pages are muted, the current one underlined in
 * the lining. `prefsOnly` is the wide header's Settings: the preference rows alone, under the
 * header's end at `end`.
 */
@Composable
fun MenuPanel(
    currentHour: String?,
    onHour: ((String) -> Unit)?,
    onOrdo: () -> Unit,
    onOrdoCurrent: Boolean,
    onReminders: () -> Unit,
    onRemindersCurrent: Boolean,
    about: String,
    onAbout: () -> Unit,
    onAboutCurrent: Boolean,
    theme: ThemeChoice,
    onTheme: (ThemeChoice) -> Unit,
    textSize: TextSize,
    onTextSize: (TextSize) -> Unit,
    onDismiss: () -> Unit,
    topOffset: Dp,
    prefsOnly: Boolean = false,
    end: Dp = Gutter,
    visible: Boolean = true,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val nav = Type.label(13.12f, 0.06f)
    val plaster = ImageBitmap.imageResource(p.panel)
    // The panel drops a little as it fades in, and lifts as it fades out, on Tempo's settle
    // (leaving faster than it came); it stays in the window until it has gone.
    val shown = remember { MutableTransitionState(false) }
    shown.targetState = visible
    if (!shown.currentState && !shown.targetState && shown.isIdle) return
    Popup(
        alignment = Alignment.TopEnd,
        // The popup's window holds the panel's shadow too (`ShadowRoom` about it), so it stands
        // that much further out and up.
        offset = with(LocalDensity.current) { IntOffset(-(end - ShadowRoom).roundToPx(), (topOffset - ShadowRise).roundToPx()) },
        onDismissRequest = onDismiss,
        properties = PopupProperties(focusable = visible),
    ) {
        val drop = with(LocalDensity.current) { 8.dp.roundToPx() }
        AnimatedVisibility(
            shown,
            enter = fadeIn(Tempo.settleIn.spec()) + slideInVertically(Tempo.settleIn.spec()) { -drop },
            exit = fadeOut(Tempo.settleOut.spec()) + slideOutVertically(Tempo.settleOut.spec()) { -drop / 2 },
        ) {
            Column(
                Modifier
                    .padding(start = ShadowRoom, end = ShadowRoom, top = ShadowRise, bottom = ShadowRoom + 8.dp)
                    // The web's soft shadow under the panel: `0 .45rem 1rem rgba(36, 28, 23, .09)`.
                    .dropShadow(RectangleShape, Shadow(radius = 16.dp, color = Color(36, 28, 23).copy(alpha = 0.09f), offset = DpOffset(0.dp, 7.2.dp)))
                    .width(if (prefsOnly) 288.dp else 336.dp)
                    .background(p.surface)
                    .then(if (p.dark) Modifier.drawBehind { nightPlaster(plaster) } else Modifier)
                    .border(1.dp, p.border)
                    // The panel's top edge is the painted line (`--ornament-line`), with the surface's
                    // edge catching the light just inside it (`inset 0 1px 0 var(--surface-edge)`).
                    .drawBehind {
                        drawRect(o.line, size = size.copy(height = 2.dp.toPx()))
                        val w = 1.dp.toPx()
                        drawRect(p.surfaceEdge, Offset(w, 2.dp.toPx()), size.copy(width = size.width - 2 * w, height = w))
                    }
                    .padding(10.4.dp),
            ) {
                if (onHour != null && !prefsOnly) {
                    listOf(listOf("lauds", "prime"), listOf("terce", "sext", "none"), listOf("vespers", "compline")).forEachIndexed { i, row ->
                        // A hairline between the bands, so the grouping reads without labels.
                        if (i > 0) Hairline(p.surfaceEdge)
                        MenuRow { row.forEach { h -> MenuCell(hourLabel(h).uppercase(), h == currentHour, nav, page = true) { onHour(h) } } }
                        Spacer(Modifier.height(2.4.dp))
                    }
                    Hairline(p.border, Modifier.padding(vertical = 4.dp))
                }
                if (!prefsOnly) {
                    MenuRow {
                        MenuCell("ORDO", onOrdoCurrent, nav, page = true, onClick = onOrdo)
                        // Habit setup, not an hour: quieter than the Ordo, as on the web (`.nav-secondary`).
                        MenuCell("REMINDERS", onRemindersCurrent, Type.label(11.52f, 0.04f), page = true, onClick = onReminders)
                    }
                    // About as quiet as Reminders, in the first half of a row of its own, as the
                    // web's grid sets it.
                    Spacer(Modifier.height(2.4.dp))
                    MenuRow {
                        MenuCell(about.uppercase(), onAboutCurrent, Type.label(11.52f, 0.04f), page = true, onClick = onAbout)
                        Spacer(Modifier.weight(1f))
                    }
                    Hairline(p.border, Modifier.padding(top = 6.4.dp))
                    Spacer(Modifier.height(6.4.dp))
                }
                MenuRow {
                    Text("THEME", Modifier.width(54.dp).padding(start = 10.4.dp), style = Type.label(10.56f, 0.08f).copy(color = p.muted))
                    ThemeChoice.entries.forEach { t -> MenuCell(t.label.uppercase(), t == theme, Type.label(12f, 0.06f)) { onTheme(t) } }
                }
                // The names are the church's, so a line says what they are, as on the web.
                PrefHint(
                    buildAnnotatedString {
                        prefName("Nave")
                        append(" is the church by day, and ")
                        prefName("Apse")
                        append(" is its vault by night. ")
                        prefName("Default")
                        append(" follows your device.")
                    },
                )
                Hairline(p.surfaceEdge)
                MenuRow {
                    Text("TEXT", Modifier.width(54.dp).padding(start = 10.4.dp), style = Type.label(10.56f, 0.08f).copy(color = p.muted))
                    TextSize.entries.forEach { s ->
                        val size = when (s) { TextSize.SMALL -> 13f; TextSize.DEFAULT -> 16f; TextSize.LARGE -> 20f }
                        val spoken = when (s) { TextSize.SMALL -> "Smaller text"; TextSize.DEFAULT -> "Default text size"; TextSize.LARGE -> "Larger text" }
                        MenuCell("A", s == textSize, Type.label(size, 0f), spoken = spoken) { onTextSize(s) }
                    }
                }
                // The Martyrology at Prime: Off and On under the last two themes, the longer label
                // taking the first theme's cell, as on the web.
                val martyrology = LocalMartyrology.current
                Hairline(p.surfaceEdge)
                MenuRow {
                    Row(Modifier.width(54.dp)) {
                        Text(
                            "MARTYROLOGY",
                            // Said as the web's group names the row.
                            Modifier.semantics { contentDescription = "Martyrology at Prime" }
                                .padding(start = 10.4.dp).wrapContentWidth(Alignment.Start, unbounded = true),
                            style = Type.label(10.56f, 0.08f).copy(color = p.muted),
                            maxLines = 1,
                            softWrap = false,
                        )
                    }
                    Spacer(Modifier.weight(1f))
                    listOf("OFF" to false, "ON" to true).forEach { (label, on) ->
                        MenuCell(label, on == martyrology.on, Type.label(12f, 0.06f)) { martyrology.onChange(on) }
                    }
                }
                PrefHint(AnnotatedString(PREF_HINTS.martyrology))
                // Chant, laid out as the Martyrology's row, once a hymn or Marian antiphon has a score.
                val chant = LocalChant.current
                if (chant.offered) {
                    Hairline(p.surfaceEdge)
                    MenuRow {
                        Text("CHANT", Modifier.width(54.dp).padding(start = 10.4.dp), style = Type.label(10.56f, 0.08f).copy(color = p.muted))
                        Spacer(Modifier.weight(1f))
                        listOf("OFF" to false, "ON" to true).forEach { (label, on) ->
                            MenuCell(label, on == chant.on, Type.label(12f, 0.06f), spoken = "Chant ${label.lowercase()}") { chant.onChange(on) }
                        }
                    }
                    PrefHint(AnnotatedString(PREF_HINTS.chant))
                }
            }
        }
    }
}

/** A button's name in its hint, upright in small caps as the button is in capitals (the web's `.pref-name`). */
private fun AnnotatedString.Builder.prefName(name: String) {
    withStyle(SpanStyle(fontStyle = FontStyle.Normal, fontFeatureSettings = ALL_SMALL_CAPS, letterSpacing = 0.05.em)) { append(name) }
}

/**
 * By night the menu opens on home's blue plaster (the phone panel's baked Apse plaster, its ground,
 * limewash and veil), not a flat slab of the surface: the web's `.site-menu-panel` by night. The
 * plaster is fitted to the panel's width from the top, and covers it.
 */
private fun DrawScope.nightPlaster(plaster: ImageBitmap) {
    val scale = maxOf(size.width / plaster.width, size.height / plaster.height)
    val w = kotlin.math.ceil(plaster.width * scale).toInt()
    val h = kotlin.math.ceil(plaster.height * scale).toInt()
    drawImage(plaster, dstOffset = IntOffset((size.width.toInt() - w) / 2, 0), dstSize = IntSize(w, h))
}

/** The room the menu panel's popup keeps about it for the panel's shadow: at its sides and foot, and above it. */
private val ShadowRoom: Dp = 16.dp
private val ShadowRise: Dp = 9.dp

/** The lines under the Martyrology and Chant rows, as the web's (`presentation`). */
private val PREF_HINTS by lazy { preferenceHints() }

/** A quiet italic line under a menu row whose names need saying (the web's `.pref-hint`). */
@Composable
private fun PrefHint(text: AnnotatedString) {
    val p = LocalPalette.current
    Text(
        text,
        Modifier.fillMaxWidth().padding(start = 10.4.dp, end = 10.4.dp, top = 7.2.dp, bottom = 9.6.dp),
        style = Type.small.copy(fontSize = 12.8.sp, lineHeight = 17.3.sp, color = p.muted, fontStyle = FontStyle.Italic),
    )
}

/** A small uppercase disclosure label with its caret ("Change date ▾"), muted, in the accent while open. */
@Composable
fun Disclosure(
    label: String,
    open: Boolean,
    onToggle: () -> Unit,
    value: String? = null,
    modifier: Modifier = Modifier,
    style: TextStyle? = null,
    upper: Boolean = true,
) {
    val p = LocalPalette.current
    val ink = if (open) p.accent else (style?.color?.takeIf { it.isSpecified } ?: p.muted)
    val cased = { s: String -> if (upper) s.uppercase() else s }
    Row(modifier.heightIn(min = 44.dp).tap(onClick = onToggle).disclosed(open).padding(horizontal = 8.dp), verticalAlignment = Alignment.CenterVertically) {
        Text(
            buildAnnotatedString {
                append(cased(label))
                if (value != null) withStyle(SpanStyle(color = p.text)) { append(" " + cased(value)) }
            },
            style = (style ?: Type.control).copy(color = ink),
            softWrap = style == null,
        )
        Caret(open, ink)
    }
}

/**
 * A page's scroll that follows what opens on it, so the page rises to show `target`'s foot and
 * `margin` under it (home's turn to the calendar): `viewport` is the scroll's own box and `foot`
 * the height at its foot the system bars cover.
 */
class Follow(val scroll: ScrollState, val viewport: () -> LayoutCoordinates?, val foot: Float, val margin: Float) {
    /** What the page brings into view, set where it is placed. */
    var target: LayoutCoordinates? = null

    /** How far the target's foot, and the margin under it, stand below what the screen shows. */
    fun overflow(): Float {
        val view = viewport()?.takeIf { it.isAttached } ?: return 0f
        val placed = target?.takeIf { it.isAttached } ?: return 0f
        return view.localBoundingBoxOf(placed, clipBounds = false).bottom + margin - (view.size.height - foot)
    }
}

/** Set by a page whose scroll follows what opens on it, as home's does. */
val LocalFollow = staticCompositionLocalOf<Follow?> { null }

/**
 * A disclosure's contents, unfolding down from its control and folding back up into it. With
 * `reveal`, once unfolded it is scrolled into view if it opened past the screen's edge; never
 * when it was open already, as a row scrolled into view would be. `instant` shows or hides it at
 * once, for a change made under a fade of its own (the ordo's "Show office details").
 */
@Composable
fun Unfold(visible: Boolean, modifier: Modifier = Modifier, reveal: Boolean = true, instant: Boolean = false, content: @Composable ColumnScope.() -> Unit) {
    val bring = remember { BringIntoViewRequester() }
    AnimatedVisibility(
        visible,
        modifier.bringIntoViewRequester(bring),
        enter = if (instant) EnterTransition.None else expandVertically(tween(UNFOLD_MS, easing = FastOutSlowInEasing), expandFrom = Alignment.Top) +
            fadeIn(tween(UNFOLD_MS - 60, delayMillis = 60, easing = LinearOutSlowInEasing)),
        exit = if (instant) ExitTransition.None else shrinkVertically(tween(FOLD_MS, easing = FastOutSlowInEasing), shrinkTowards = Alignment.Top) +
            fadeOut(tween(FOLD_MS / 2, easing = FastOutLinearInEasing)),
    ) {
        LaunchedEffect(Unit) {
            if (!reveal || transition.currentState == EnterExitState.Visible) return@LaunchedEffect
            snapshotFlow { transition.currentState }.first { it == EnterExitState.Visible }
            bring.bringIntoView()
        }
        Column(content = content)
    }
}

/**
 * How long a disclosure takes to unfold and to fold, here, for the hour's sections and for home's
 * turn to the calendar: Tempo's unfold, on the standard curve, as iOS's `unfolding` and
 * `folding` and the web's disclosures (style.css, "Disclosures unfold").
 */
internal val UNFOLD_MS = Tempo.unfoldIn.ms
internal val FOLD_MS = Tempo.unfoldOut.ms

/**
 * The ordo's "Show office details": the month fades out, every day changes at once, and it fades
 * back in, as on the web (app.js, calendarExpand) and iOS (OrdoScreen's turnAll).
 */
internal const val MONTH_OUT_MS = 70
internal const val MONTH_IN_MS = 140

/** The picker's span of years, as the web's (app.js PICKER_FIRST_YEAR, PICKER_LAST_YEAR). */
private val PICKER_YEARS = 1950..2150

/**
 * The hand-set date picker (app.js): Previous · Today · Next, then a month grid whose days
 * are the choices. Sundays are red; the chosen day takes the accent on the gold underline, as
 * a chosen prayer form does, and today a quiet wash. The title turns the grid into the year's
 * months, and back; both views keep six weeks' height, so turning between them moves nothing
 * below. The grid is seven thumb-wide days at most, centred (`.date-picker`). `centred` sets
 * Previous · Today · Next together in the middle (`.day-nav`) rather than at the edges, as the
 * web's home does at every width and an hour above 540px.
 */
@Composable
fun DatePicker(shown: LocalDate, today: LocalDate, centred: Boolean = false, onPick: (LocalDate) -> Unit) {
    val p = LocalPalette.current
    val haptics = LocalHapticFeedback.current
    var month by remember(shown) { mutableStateOf(YearMonth.from(shown).coerceIn(PICKER_YEARS)) }
    var months by remember(shown) { mutableStateOf(false) }
    val link = Type.body.copy(fontSize = 16.sp, lineHeight = 24.sp, color = p.accent)
    Column(Modifier.fillMaxWidth().padding(top = 8.dp)) {
        Hairline(p.border, Modifier.padding(horizontal = 24.dp))
        // "Previous day" and "Next day", worded (`presentation::PREVIOUS_DAY`), so they read as the
        // day's turn and the month's arrows below as the month's; gathered, they stand at the
        // grid's edges.
        Row(
            (if (centred) Modifier.align(Alignment.CenterHorizontally).widthIn(max = 44.dp * 7) else Modifier).fillMaxWidth().padding(top = 8.dp).heightIn(min = 44.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("← ${DAY_NAV.previous}", Modifier.tap(label = DAY_NAV.previous) { onPick(shown.minusDays(1)) }.padding(horizontal = if (centred) 4.dp else 16.dp), style = link)
            Spacer(Modifier.weight(1f))
            if (shown != today) Text("TODAY", Modifier.tap { onPick(today) }, style = Type.control.copy(color = p.muted))
            Spacer(Modifier.weight(1f))
            Text("${DAY_NAV.next} →", Modifier.tap(label = DAY_NAV.next) { onPick(shown.plusDays(1)) }.padding(horizontal = if (centred) 4.dp else 16.dp), style = link)
        }
        Column(Modifier.align(Alignment.CenterHorizontally).widthIn(max = 44.dp * 7).fillMaxWidth()) {
            // A hairline parts the day's turn above from the month's below.
            Hairline(p.border.copy(alpha = p.border.alpha * 0.58f), Modifier.padding(top = 4.dp))
            Row(Modifier.fillMaxWidth().padding(top = 8.dp).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
                // A month's step in the days, a year's in the months; none past the picker's span.
                val step = if (months) 12L else 1L
                val back = month.minusMonths(step).takeIf { it.year in PICKER_YEARS }
                val forward = month.plusMonths(step).takeIf { it.year in PICKER_YEARS }
                val stepStyle = link.copy(fontSize = 22.sp)
                // Each step a day's cell wide, over the first and last columns.
                Box(
                    Modifier.size(44.dp).then(if (back != null) Modifier.tap(label = if (months) "Previous year" else "Previous month") { month = back } else Modifier),
                    contentAlignment = Alignment.Center,
                ) { Text("‹", style = if (back != null) stepStyle else stepStyle.copy(color = p.border)) }
                val title = if (months) "${month.year}" else "${month.month.getDisplayName(JavaTextStyle.FULL, Locale.US)} ${month.year}"
                Row(
                    Modifier.weight(1f).heightIn(min = 44.dp)
                        .tap(label = if (months) "$title, back to days" else "$title, choose another month") { months = !months },
                    horizontalArrangement = Arrangement.Center,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(title.uppercase(), style = Type.label(11.52f, 0.1f).copy(color = p.text, textAlign = TextAlign.Center))
                    // Set like the disclosure labels around it, with their gold caret.
                    Caret(months, p.goldLine, lighter = false)
                }
                Box(
                    Modifier.size(44.dp).then(if (forward != null) Modifier.tap(label = if (months) "Next year" else "Next month") { month = forward } else Modifier),
                    contentAlignment = Alignment.Center,
                ) { Text("›", style = if (forward != null) stepStyle else stepStyle.copy(color = p.border)) }
            }
            // A swipe across the days turns the month, as the arrows do, with a tick as it commits:
            // the one turn with no control under the finger (the iOS app's too).
            val swipe = with(LocalDensity.current) { 48.dp.toPx() }
            var dragged by remember { mutableFloatStateOf(0f) }
            val daysAlpha by animateFloatAsState(if (months) 0f else 1f, tween(180), label = "days")
            Box(
                Modifier.fillMaxWidth().draggable(
                    rememberDraggableState { dragged += it },
                    Orientation.Horizontal,
                    enabled = !months,
                    onDragStarted = { dragged = 0f },
                    onDragStopped = {
                        val to = if (dragged < -swipe) month.plusMonths(1) else if (dragged > swipe) month.minusMonths(1) else null
                        if (to != null && to.year in PICKER_YEARS) {
                            haptics.chose()
                            month = to
                        }
                    },
                ),
            ) {
                // The days always lay out, for the height both views share; the months cover them.
                AnimatedContent(
                    month,
                    Modifier.alpha(daysAlpha).then(if (months) Modifier.clearAndSetSemantics {} else Modifier),
                    transitionSpec = {
                        val on = if (targetState > initialState) 1 else -1
                        (slideInHorizontally(tween(260, easing = FastOutSlowInEasing)) { on * it / 5 } + fadeIn(tween(180, delayMillis = 60))) togetherWith
                            (slideOutHorizontally(tween(260, easing = FastOutSlowInEasing)) { -on * it / 5 } + fadeOut(tween(90)))
                    },
                    label = "month",
                ) { m -> Column { Days(m, shown, today, live = !months, onPick) } }
                if (months || daysAlpha < 1f) {
                    MonthGrid(month.year, shown, today, Modifier.matchParentSize().alpha(1f - daysAlpha), live = months) { m ->
                        month = YearMonth.of(month.year, m)
                        months = false
                    }
                }
            }
        }
    }
}

/** Change date's day links, worded as the web's. */
private val DAY_NAV by lazy { org.orthodoxwest.office.core.dayNavWords() }

/**
 * Today in the picker lies under a pool of light with no edge, as the hour that is now does on
 * home (`--now-pool`): ochre by day, moonlit blue by night.
 */
private fun Modifier.todayPool(p: Palette): Modifier = drawBehind {
    val pool = homeInk(p).nowPool
    val r = minOf(size.width, size.height) / 2f
    drawCircle(androidx.compose.ui.graphics.Brush.radialGradient(0f to pool, 0.52f to pool.copy(alpha = pool.alpha * 0.55f), 1f to Color.Transparent, center = center, radius = r), r, center)
}

private fun YearMonth.coerceIn(years: IntRange): YearMonth = when {
    year < years.first -> YearMonth.of(years.first, 1)
    year > years.last -> YearMonth.of(years.last, 12)
    else -> this
}

/** A month's days in six weeks always, blank where the month is not; `live` false while the months cover them. */
@Composable
private fun Days(month: YearMonth, shown: LocalDate, today: LocalDate, live: Boolean, onPick: (LocalDate) -> Unit) {
    val p = LocalPalette.current
    val haptics = LocalHapticFeedback.current
    val days = listOf(DayOfWeek.SUNDAY) + DayOfWeek.entries.filter { it != DayOfWeek.SUNDAY }
    // The weekday letters are for the eye; each day below names itself in full.
    Row(Modifier.fillMaxWidth().clearAndSetSemantics {}) {
        days.forEach { d ->
            Text(
                d.getDisplayName(JavaTextStyle.NARROW, Locale.US),
                Modifier.weight(1f).padding(vertical = 8.dp),
                style = Type.label(10.56f, 0.1f).copy(color = if (d == DayOfWeek.SUNDAY) p.rubric else p.muted, textAlign = TextAlign.Center),
            )
        }
    }
    val lead = month.atDay(1).dayOfWeek.value % 7
    val cells = (0 until 42).map { i -> (i - lead + 1).takeIf { it in 1..month.lengthOfMonth() }?.let(month::atDay) }
    cells.chunked(7).forEach { week ->
        Row(Modifier.fillMaxWidth()) {
            week.forEach { day ->
                Box(
                    Modifier
                        .weight(1f)
                        .heightIn(min = 44.dp)
                        .then(if (day == today) Modifier.todayPool(p) else Modifier)
                        .then(if (day != null && live) Modifier.tap(label = spokenDay(day, today), selected = day == shown, lit = Lit(homeInk(p).nowPool)) {
                            haptics.chose()
                            onPick(day)
                        } else Modifier),
                    contentAlignment = Alignment.Center,
                ) {
                    if (day != null) {
                        // A chosen Sunday keeps its rubric over the gold rule.
                        val ink = when {
                            day.dayOfWeek == DayOfWeek.SUNDAY -> p.rubric
                            day == shown -> p.accent
                            else -> p.text
                        }
                        Text(
                            day.dayOfMonth.toString(),
                            Modifier.goldUnderline(day == shown, p.goldLine),
                            style = Type.body.copy(fontSize = 18.sp, lineHeight = 30.sp, color = ink, fontFeatureSettings = "lnum"),
                        )
                    }
                }
            }
        }
    }
}

/** The year's months, three to a row: the chosen day's month underlined in gold, today's washed. */
@Composable
private fun MonthGrid(year: Int, shown: LocalDate, today: LocalDate, modifier: Modifier, live: Boolean, onMonth: (Int) -> Unit) {
    val p = LocalPalette.current
    Column(modifier, verticalArrangement = Arrangement.SpaceEvenly) {
        (1..12).chunked(3).forEach { row ->
            Row(Modifier.fillMaxWidth()) {
                row.forEach { m ->
                    val name = Month.of(m).getDisplayName(JavaTextStyle.FULL, Locale.US)
                    val chosen = shown.year == year && shown.monthValue == m
                    Box(Modifier.weight(1f), contentAlignment = Alignment.Center) {
                        Text(
                            Month.of(m).getDisplayName(JavaTextStyle.SHORT, Locale.US).uppercase(),
                            Modifier
                                .then(if (today.year == year && today.monthValue == m) Modifier.todayPool(p) else Modifier)
                                .then(if (live) Modifier.tap(label = "$name $year", selected = chosen, lit = Lit(homeInk(p).nowPool)) { onMonth(m) } else Modifier.clearAndSetSemantics {})
                                .heightIn(min = 44.dp)
                                .goldUnderline(chosen, p.goldLine)
                                .padding(horizontal = 14.4.dp, vertical = 13.dp),
                            style = Type.label(11.5f, 0.1f).copy(color = if (chosen) p.accent else p.text),
                        )
                    }
                }
            }
        }
    }
}

/**
 * "How are you praying?": the three prayer forms, remembered on this device, as the web's
 * `.leader-selector fieldset`: a small uppercase legend over a list at most 23rem wide, ruled
 * above, below and between, the chosen form's ring filled in the accent and its words in the
 * accent over a gold underline.
 */
@Composable
fun FormChooser(form: String, onForm: (String) -> Unit) {
    val p = LocalPalette.current
    val haptics = LocalHapticFeedback.current
    Column(Modifier.widthIn(max = 368.dp).fillMaxWidth().padding(top = 12.dp, bottom = 8.dp)) {
        Text("HOW ARE YOU PRAYING?", Modifier.padding(bottom = 10.4.dp), style = Type.label(11.2f, 0.1f).copy(color = p.muted))
        Column(Modifier.fillMaxWidth().drawBehind {
            val w = 1.dp.toPx()
            drawLine(p.border, Offset(0f, w / 2f), Offset(size.width, w / 2f), w)
            drawLine(p.border, Offset(0f, size.height - w / 2f), Offset(size.width, size.height - w / 2f), w)
        }.padding(vertical = 1.dp)) {
            PRAYER_FORMS.forEachIndexed { i, (value, _, phrase) ->
                val chosen = value == form
                if (i > 0) Hairline(p.surfaceEdge)
                Row(Modifier.fillMaxWidth().heightIn(min = 44.dp).tap(role = Role.RadioButton, selected = chosen) {
                    if (!chosen) haptics.chose()
                    onForm(value)
                }.padding(horizontal = 5.6.dp, vertical = 10.4.dp), verticalAlignment = Alignment.CenterVertically) {
                    // The web's ring: a 1px circle on the ground; chosen, the accent with the ground
                    // inset 3px inside its line.
                    Box(Modifier.size(14.4.dp).background(if (chosen) p.accent else p.bg, CircleShape).border(1.dp, if (chosen) p.accent else p.muted, CircleShape), contentAlignment = Alignment.Center) {
                        if (chosen) Box(Modifier.size(12.4.dp).border(3.dp, p.bg, CircleShape))
                    }
                    Spacer(Modifier.width(12.8.dp))
                    Text(
                        phrase,
                        Modifier.goldUnderline(chosen, p.goldLine),
                        style = Type.body.copy(fontSize = 14.4.sp, lineHeight = 20.16.sp, color = if (chosen) p.accent else p.text),
                    )
                }
            }
        }
        Text("Remembered on this device. Change it any time.", Modifier.padding(top = 11.2.dp), style = Type.small.copy(fontSize = 12.8.sp, lineHeight = 18.56.sp, color = p.muted))
    }
}

/**
 * The way on at a page's end: the previous and next with their labels, and the middle link between.
 * `ruled` sets it as home sets its hours (an hour's end, the web's `.page-hour .hour-continuation`):
 * labels in true small capitals, the names a size up, home's gilt points between the three, and
 * one rule struck under them with home's pen where the wall's field begins.
 */
@Composable
fun Continuation(
    previousLabel: String,
    previous: String?,
    onPrevious: () -> Unit,
    middle: String,
    onMiddle: () -> Unit,
    nextLabel: String,
    next: String?,
    onNext: () -> Unit,
    modifier: Modifier = Modifier,
    ruled: Boolean = false,
) {
    val p = LocalPalette.current
    val small = if (ruled) Type.label(13.76f, 0.08f).copy(color = p.muted, fontFeatureSettings = ALL_SMALL_CAPS) else Type.label(11.2f, 0.08f).copy(color = p.muted)
    val name = Type.body.copy(fontSize = if (ruled) 16.96.sp else 16.sp, lineHeight = 22.sp, color = p.accent)
    val middleStyle = if (ruled) small else Type.label(12.16f, 0.06f).copy(color = p.muted)
    val rule = p.lining.copy(alpha = 0.55f)
    fun label(text: String) = if (ruled) text else text.uppercase()
    Column(modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
        Row(
            Modifier.fillMaxWidth()
                .then(
                    if (!ruled) Modifier else Modifier.drawBehind {
                        val u = density
                        drawPath(ruledLine(0f, size.height - 0.5f * u, size.width, size.height - 0.5f * u, 1.2f * u, 51, u), rule)
                    }.padding(top = 8.8.dp, bottom = 9.6.dp),
                ),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f).then(if (previous != null) Modifier.tap(label = "$previousLabel: $previous", onClick = onPrevious) else Modifier), horizontalAlignment = Alignment.Start) {
                if (previous != null) {
                    Text(label(previousLabel), style = small)
                    Text("← $previous", style = name)
                }
            }
            if (ruled && previous != null) GiltPoint()
            Text(label(middle), Modifier.tap(label = middle, onClick = onMiddle).padding(horizontal = if (ruled) 12.dp else 6.4.dp, vertical = 12.dp), style = middleStyle)
            if (ruled && next != null) GiltPoint()
            Column(Modifier.weight(1f).then(if (next != null) Modifier.tap(label = "$nextLabel: $next", onClick = onNext) else Modifier), horizontalAlignment = Alignment.End) {
                if (next != null) {
                    Text(label(nextLabel), style = small)
                    Text("$next →", style = name)
                }
            }
        }
    }
}

/**
 * A painter's reserve for an inscription: a feathered clearing of the wall's own colour behind a
 * line of lettering, so the powdering and the vault never run under the letters (the web's
 * `footer > p` on home and the hours: `radial-gradient(closest-side, var(--bg) 58%, transparent)`
 * over 0.6rem × 1.75rem of padding the margin takes back). It draws outside the text's box, so the
 * foot keeps its height.
 */
fun Modifier.reserve(ground: Color): Modifier = drawBehind {
    val w = size.width + 2 * 28.dp.toPx()
    val h = size.height + 2 * 9.6.dp.toPx()
    // closest-side: an ellipse touching the padded box's sides, drawn as a circle squashed to it.
    scale(scaleX = 1f, scaleY = h / w, pivot = center) {
        drawCircle(Brush.radialGradient(0f to ground, 0.58f to ground, 1f to Color.Transparent, center = center, radius = w / 2), radius = w / 2, center = center)
    }
}

/**
 * The page's foot: the tailpiece that closes every page, the Office's name, and any `matter` a page
 * adds under it (an hour's report line). `gap` stands above it and `bottom` below. On a page with a
 * field on its wall (home, the hours) the name stands on a `reserve`. The preferences are in the
 * menu, or on a wide screen under Settings.
 */
@Composable
fun Footer(modifier: Modifier = Modifier, gap: Dp = 53.6.dp, bottom: Dp = 24.dp, reserve: Boolean = false, matter: (@Composable () -> Unit)? = null) {
    val p = LocalPalette.current
    Column(modifier.fillMaxWidth().padding(top = gap, bottom = bottom), horizontalAlignment = Alignment.CenterHorizontally) {
        Tailpiece()
        Text("Benedictine Divine Office", Modifier.padding(top = 8.8.dp).then(if (reserve) Modifier.reserve(p.bg) else Modifier), style = Type.small.copy(color = p.muted))
        matter?.invoke()
    }
}

/** A day as a screen reader says it in the picker: "Sunday, March 15, today". */
fun spokenDay(day: LocalDate, today: LocalDate): String =
    day.format(DateTimeFormatter.ofPattern("EEEE, MMMM d", Locale.US)) + if (day == today) ", today" else ""
