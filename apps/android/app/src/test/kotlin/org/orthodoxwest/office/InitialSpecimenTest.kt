package org.orthodoxwest.office

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.unit.dp
import com.github.takahirom.roborazzi.captureRoboImage
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.BlockKind
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.RunStyle
import org.orthodoxwest.office.core.RunView
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Every capital as a psalm's dropped initial, one 120dp slot each, for review beside the web's
 * specimen of the same verse (recalibrate both when the initial's face or sizing changes).
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h3200dp-xxhdpi")
class InitialSpecimenTest {
    @get:Rule
    val compose = createComposeRule()

    @Test
    fun psalmInitials() {
        compose.setContent {
            OfficeTheme(choice = ThemeChoice.NAVE) {
                Column(Modifier.fillMaxWidth().background(LocalPalette.current.bg).padding(horizontal = Gutter)) {
                    SPECIMEN.forEach { word ->
                        Box(Modifier.fillMaxWidth().height(120.dp).padding(top = 8.dp)) { Block(verse(word)) }
                    }
                }
            }
        }
        compose.onRoot().captureRoboImage("build/screenshots/initials-psalm.png")
    }

    /** The web's adaptive settings: elevated and divided psalms, a raised one-line chapter, a short responsory. */
    @Test
    fun adaptiveOpenings() {
        val openings = listOf(
            pointed(BlockKind.VERSE, "Praise the Lord", " all ye heathen."),
            pointed(BlockKind.VERSE, "Lord thou hast been our refuge", " from one age."),
            BlockView(BlockKind.PARAGRAPH, "", true, true, listOf(RunView("Be sober, be vigilant.", RunStyle.PLAIN))),
            pointed(BlockKind.RESPONSE, "He shall deliver thee,", " From the snare of the hunter."),
        )
        compose.setContent {
            OfficeTheme(choice = ThemeChoice.NAVE) {
                Column(Modifier.fillMaxWidth().background(LocalPalette.current.bg).padding(horizontal = Gutter)) {
                    openings.forEach { Box(Modifier.fillMaxWidth().height(120.dp).padding(top = 8.dp)) { Block(it) } }
                }
            }
        }
        compose.onRoot().captureRoboImage("build/screenshots/initials-adaptive.png")
    }

    private fun pointed(kind: BlockKind, first: String, second: String) =
        BlockView(kind, "", true, true, listOf(RunView(first, RunStyle.PLAIN), RunView("\u00a0*", RunStyle.MEDIANT), RunView(second, RunStyle.PLAIN)))

    private fun verse(word: String) = BlockView(
        BlockKind.VERSE, "", true, true,
        listOf(
            RunView("$word the Lord, O my soul, and all that is within me", RunStyle.PLAIN),
            RunView(" *", RunStyle.MEDIANT),
            RunView(" praise his holy Name, for he is gracious, and his mercy endureth for ever.", RunStyle.PLAIN),
        ),
    )

    companion object {
        /** An opening word for every capital (the web's specimen uses the same). */
        val SPECIMEN = listOf(
            "And", "Blessed", "Come", "Deliver", "Except", "From", "God", "Have", "In", "Judge", "Keep", "Lord", "My",
            "Not", "O", "Praise", "Quicken", "Rejoice", "Save", "The", "Unto", "Verily", "When", "Xerxes", "Ye", "Zion",
        )
    }
}
