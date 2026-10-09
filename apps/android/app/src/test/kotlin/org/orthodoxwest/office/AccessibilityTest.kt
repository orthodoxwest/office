package org.orthodoxwest.office

import android.app.Application
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.github.takahirom.roborazzi.captureRoboImage
import java.time.LocalDate
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.hourNames
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/** What a screen reader finds on each page: headings, named controls, and their states. */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class AccessibilityTest {
    @get:Rule
    val compose = createComposeRule()

    private val app: Application = ApplicationProvider.getApplicationContext()
    private val lent = LocalDate.of(2026, 3, 15)

    private fun show(page: Page, reminders: ReminderSettings = ReminderStore(app).load(), newcomer: Boolean = false) {
        val core = Office.core
        val home = (page as? Page.Home)?.let { core.home(it.date.toCivil(), lent.toCivil(), 18) }
        val hour = (page as? Page.Hour)?.let { core.compose(it.hour, it.date.year, it.date.monthValue, it.date.dayOfMonth, "private") }
        val ordo = (page as? Page.Ordo)?.let { core.ordoMonth(it.year, it.month) }
        compose.setContent {
            OfficeTheme(choice = ThemeChoice.NAVE) {
                OfficeApp(
                    page, lent, hourNames(), home, hour, ordo, null, "private", ThemeChoice.NAVE, TextSize.DEFAULT, PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = reminders, reminderStatus = ReminderStatus(notificationsAllowed = true, exactAllowed = true),
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                    newcomer = newcomer,
                )
            }
        }
    }

    private fun role(r: Role) = SemanticsMatcher.expectValue(SemanticsProperties.Role, r)
    private fun state(s: String) = SemanticsMatcher.expectValue(SemanticsProperties.StateDescription, s)

    @Test
    @Config(qualifiers = "w390dp-h2400dp-xxhdpi")
    fun anHourHasHeadingsAndSpeaksItsSigils() {
        show(Page.Hour(lent, "vespers"))
        compose.onNodeWithContentDescription("Vespers").assert(isHeading())
        compose.onNodeWithContentDescription("Opening Versicles").assert(isHeading())
        compose.onNodeWithContentDescription("Versicle. O God, sign of the cross, make speed to save me.").assertExists()
        compose.onNodeWithContentDescription("Menu").assert(role(Role.Button)).assert(state("Collapsed"))
        compose.onNodeWithContentDescription("Daily Office, home").assert(role(Role.Button))
    }

    @Test
    fun theDatePickerNamesEachDay() {
        show(Page.Hour(lent, "vespers"))
        compose.onNodeWithText("CHANGE DATE", substring = true).assert(state("Collapsed")).performClick()
        compose.onNodeWithText("CHANGE DATE", substring = true).assert(state("Expanded"))
        compose.onNodeWithContentDescription("Sunday, March 15, today").assertIsSelected()
        compose.onNodeWithContentDescription("Previous month").assert(role(Role.Button))
        compose.onAllNodesWithText("How are you praying?").fetchSemanticsNodes().let { assertTrue(it.isEmpty()) }
    }

    @Test
    fun remindersAreCheckboxes() {
        show(Page.Reminders)
        compose.onNodeWithText("Vespers").assert(role(Role.Checkbox)).assertIsOn()
        compose.onNodeWithText("Prime").assertIsOff()
        compose.onNodeWithContentDescription("Sunday").assert(role(Role.Checkbox)).assertIsOn()
        compose.onNodeWithText("Set prayer reminders").assert(isHeading())
    }

    @Test
    fun anOrdoDayIsOneStop() {
        show(Page.Ordo(2026, 3))
        compose.onNode(hasContentDescription("Sunday, March 1. I Sunday in Lent. liturgical color: violet", substring = true)).assert(role(Role.Button))
    }

    @Test
    @Config(qualifiers = "w390dp-h3200dp-xxhdpi")
    fun aboutHasHeadingsAndOpensEachHour() {
        show(Page.About)
        compose.onNodeWithText("About the Office").assert(isHeading())
        compose.onNodeWithText("The seven hours").assert(isHeading())
        compose.onNodeWithText("Lauds").assert(role(Role.Button))
        compose.onNodeWithText("Compline").assert(role(Role.Button))
    }

    @Test
    fun aNewcomerIsOfferedTheIntroduction() {
        show(Page.Home(lent), newcomer = true)
        compose.onNodeWithText("INTRODUCTION").assert(role(Role.Button))
    }

    /** Android's largest font size (200%), for review beside the default captures. */
    @Test
    @Config(fontScale = 2.0f, qualifiers = "w390dp-h2400dp-xxhdpi")
    fun largestFontHour() {
        show(Page.Hour(lent, "compline"))
        compose.onRoot().captureRoboImage("build/screenshots/font-200-compline.png")
    }

    @Test
    @Config(fontScale = 2.0f, qualifiers = "w390dp-h2400dp-xxhdpi")
    fun largestFontHome() {
        show(Page.Home(lent))
        compose.onRoot().captureRoboImage("build/screenshots/font-200-home.png")
    }

    @Test
    @Config(fontScale = 2.0f, qualifiers = "w390dp-h4800dp-xxhdpi")
    fun largestFontAbout() {
        show(Page.About)
        compose.onRoot().captureRoboImage("build/screenshots/font-200-about.png")
    }

    @Test
    @Config(fontScale = 2.0f, qualifiers = "w390dp-h2400dp-xxhdpi")
    fun largestFontReminders() {
        show(Page.Reminders)
        compose.onRoot().captureRoboImage("build/screenshots/font-200-reminders.png")
    }
}
