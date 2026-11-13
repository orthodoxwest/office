package org.orthodoxwest.office

import androidx.compose.foundation.IndicationNodeFactory
import androidx.compose.foundation.background
import androidx.compose.foundation.indication
import androidx.compose.foundation.interaction.FocusInteraction
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * A control that takes focus wears the web's focus ring, in gold. The focus is sent to the Dim
 * indication directly: Robolectric does not move input focus on a Tab, and the ring is drawn by
 * the indication whichever way the focus arrives.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class FocusRingTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun aControlReachedByKeyboardWearsTheGoldRing() = wearsTheRing(Dim)

    /** A row that takes the light under the finger wears the same ring. */
    @Test
    fun aLitRowReachedByKeyboardWearsTheGoldRing() = wearsTheRing(Lit(homeInk(Nave).nowPool))

    private fun wearsTheRing(indication: IndicationNodeFactory) {
        val source = MutableInteractionSource()
        compose.setContent {
            CompositionLocalProvider(LocalPalette provides Nave) {
                Box(Modifier.size(120.dp).background(Nave.bg).testTag("wall"), contentAlignment = Alignment.Center) {
                    Box(Modifier.size(40.dp).indication(source, indication))
                }
            }
        }
        // The ring's line runs 3 to 5dp off the control: its middle, 4dp left of the control's
        // left edge (40dp in from the wall's), halfway down.
        val x = with(compose.density) { (40.dp - 4.dp).roundToPx() }
        val y = with(compose.density) { 60.dp.roundToPx() }
        fun at() = compose.onNodeWithTag("wall").captureToImage().toPixelMap()[x, y]

        assertEquals(Nave.bg, at())

        val focus = FocusInteraction.Focus()
        compose.runOnIdle { check(source.tryEmit(focus)) }
        compose.waitForIdle()
        assertEquals(Nave.gold, at())

        compose.runOnIdle { check(source.tryEmit(FocusInteraction.Unfocus(focus))) }
        compose.waitForIdle()
        assertEquals(Nave.bg, at())
    }

    private fun assertEquals(expected: Color, actual: Color) =
        assertEquals("expected $expected, was $actual", Integer.toHexString(expected.toArgb()), Integer.toHexString(actual.toArgb()))
}
