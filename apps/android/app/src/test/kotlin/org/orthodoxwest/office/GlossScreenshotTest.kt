package org.orthodoxwest.office

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.ui.test.ExperimentalTestApi
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasScrollToIndexAction
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performCustomAccessibilityActionWithLabel
import androidx.compose.ui.test.performScrollToNode
import androidx.test.core.app.ApplicationProvider
import com.github.takahirom.roborazzi.captureRoboImage
import com.github.takahirom.roborazzi.captureScreenRoboImage
import java.time.LocalDate
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.hourNames
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * A rubric's "secretly" and "entirely in secret", as the web's `.gloss-term`: the term's
 * hairline, and its note opened over the word, for review beside the web in both themes.
 */
@OptIn(ExperimentalTestApi::class)
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class GlossScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private fun open(hour: String, rubric: String, term: String, theme: ThemeChoice, name: String) {
        val lent = LocalDate.of(2026, 3, 15)
        val view = Office.core.compose(hour, 2026, 3, 15, "private")
        val glossed = view.sections.flatMap { it.blocks }.flatMap { it.runs }.mapNotNull { it.gloss?.title }
        assert(glossed.isNotEmpty()) { "no glossed term in $hour" }
        compose.setContent {
            OfficeTheme(choice = theme) {
                OfficeApp(
                    Page.Hour(lent, hour), lent, hourNames(), null, view, null, null, "private", theme, TextSize.DEFAULT, PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = ReminderStore(ApplicationProvider.getApplicationContext()).load(), reminderStatus = ReminderStatus(true, true),
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                )
            }
        }
        val node = hasContentDescription(rubric, substring = true)
        compose.onNode(hasScrollToIndexAction()).performScrollToNode(node)
        compose.waitForIdle()
        compose.onRoot().captureRoboImage("build/screenshots/gloss-$name-closed.png")
        compose.onNode(node).performCustomAccessibilityActionWithLabel("Explain “$term”")
        compose.waitForIdle()
        captureScreenRoboImage("build/screenshots/gloss-$name-open.png")
    }

    @Test fun terceNave() = open("terce", "is said secretly", "secretly", ThemeChoice.NAVE, "terce-nave")
    @Test fun terceApse() = open("terce", "is said secretly", "secretly", ThemeChoice.APSE, "terce-apse")
    @Test fun complineApse() = open("compline", "entirely in secret", "entirely in secret", ThemeChoice.APSE, "compline-apse")

    /** The apps gloss the same words the web does. */
    @Test
    fun theNotesAreTheWebs() {
        val notes = Office.core.compose("compline", 2026, 3, 15, "private").sections.flatMap { it.blocks }.flatMap { it.runs }.mapNotNull { it.gloss }
        assertEquals(listOf("Entirely in secret", "Secretly"), notes.map { it.title })
        assertEquals("The whole prayer, shown in grey, is said silently.", notes[0].note)
    }
}
