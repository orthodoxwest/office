package org.orthodoxwest.office

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.toArgb
import androidx.compose.ui.graphics.toPixelMap
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.assertIsFocused
import androidx.compose.ui.test.assertIsNotFocused
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performKeyInput
import androidx.compose.ui.test.pressKey
import androidx.compose.ui.unit.dp
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/** A control reached by keyboard or D-pad wears the web's focus ring, in gold; a touch draws none. */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class FocusRingTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun aControlReachedByKeyboardWearsTheGoldRing() {
        compose.setContent {
            CompositionLocalProvider(LocalPalette provides Nave) {
                Box(Modifier.size(120.dp).background(Nave.bg).testTag("wall"), contentAlignment = Alignment.Center) {
                    Box(Modifier.size(40.dp).testTag("control").tap {})
                }
            }
        }
        // The ring's line runs 3 to 5dp off the control: its middle, 4dp left of the control's
        // left edge (40dp in from the wall's), halfway down.
        val x = with(compose.density) { (40.dp - 4.dp).roundToPx() }
        val y = with(compose.density) { 60.dp.roundToPx() }
        fun at() = compose.onNodeWithTag("wall").captureToImage().toPixelMap()[x, y]

        compose.onNodeWithTag("control").assertIsNotFocused()
        assertEquals(Nave.bg, at())

        compose.onRoot().performKeyInput { pressKey(Key.Tab) }
        compose.onNodeWithTag("control").assertIsFocused()
        assertEquals(Nave.gold, at())
    }

    private fun assertEquals(expected: Color, actual: Color) =
        assertEquals("expected $expected, was $actual", Integer.toHexString(expected.toArgb()), Integer.toHexString(actual.toArgb()))
}
