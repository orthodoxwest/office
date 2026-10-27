package org.orthodoxwest.office

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.github.takahirom.roborazzi.captureRoboImage
import java.time.LocalDate
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.OfficeCore
import org.orthodoxwest.office.core.hourNames
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * The search for a feast and a feast's page, from the Rust core, for review beside the web's
 * /calendar/find and /feast pages.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h2400dp-xxhdpi")
class FeastScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private val today = LocalDate.of(2026, 10, 27)

    private fun show(page: Page, theme: ThemeChoice, onOpen: (Page) -> Unit = {}) {
        val find = (page as? Page.Find)?.let { core.findFeasts(it.query, today.year) }
        val feast = (page as? Page.Feast)?.let { core.feastPage(it.id, today.toCivil()) }
        val home = (page as? Page.Home)?.let { core.home(it.date.toCivil(), today.toCivil(), 18) }
        val ordo = (page as? Page.Ordo)?.let { core.ordoMonth(it.year, it.month) }
        compose.setContent {
            OfficeTheme(choice = theme, season = home?.ornament ?: "") {
                OfficeApp(
                    page, today, hourNames(), home, null, ordo, null, "private", theme, TextSize.DEFAULT, PaddingValues(),
                    onOpen = onOpen, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = ReminderStore(ApplicationProvider.getApplicationContext()).load(),
                    reminderStatus = ReminderStatus(notificationsAllowed = true, exactAllowed = true),
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                    find = find, feast = feast,
                )
            }
        }
    }

    private fun shoot(name: String) = compose.onRoot().captureRoboImage("build/screenshots/$name.png")

    @Test
    fun findNave() { show(Page.Find("raphael"), ThemeChoice.NAVE); shoot("find-nave") }

    @Test
    fun findApse() { show(Page.Find("raphael"), ThemeChoice.APSE); shoot("find-apse") }

    @Test
    fun findEmpty() { show(Page.Find(""), ThemeChoice.NAVE); shoot("find-empty") }

    @Test
    @Config(qualifiers = "w390dp-h3200dp-xxhdpi")
    fun feastNave() { show(Page.Feast("st-raphael-of-brooklyn"), ThemeChoice.NAVE); shoot("feast-nave") }

    @Test
    @Config(qualifiers = "w390dp-h3200dp-xxhdpi")
    fun feastApse() { show(Page.Feast("st-raphael-of-brooklyn"), ThemeChoice.APSE); shoot("feast-apse") }

    @Test
    fun monasticFeast() { show(Page.Feast("solemnity-st-benedict"), ThemeChoice.NAVE); shoot("feast-monastic") }

    @Test
    @Config(qualifiers = "w1024dp-h2400dp-xhdpi")
    fun feastTablet() { show(Page.Feast("st-mark"), ThemeChoice.NAVE); shoot("feast-tablet") }

    // The day's commemoration leads to its feast's page.
    @Test
    @Config(qualifiers = "w390dp-h844dp-xxhdpi")
    fun homeLeadsToTheFeast() {
        var opened: Page? = null
        show(Page.Home(LocalDate.of(2026, 2, 27)), ThemeChoice.NAVE) { opened = it }
        shoot("home-commemoration")
        compose.onNodeWithText("St Raphael of Brooklyn", substring = true).performClick()
        assertEquals(Page.Feast("st-raphael-of-brooklyn"), opened)
    }

    // The ordo's field opens the search.
    @Test
    fun ordoOpensTheSearch() {
        var opened: Page? = null
        show(Page.Ordo(2026, 2), ThemeChoice.NAVE) { opened = it }
        shoot("ordo-search")
        compose.onNodeWithText("Find a feast or saint").performClick()
        assertEquals(Page.Find(""), opened)
    }

    @Test
    fun pagesSurviveSavedState() {
        for (page in listOf(Page.Find("st raphael"), Page.Find(""), Page.Feast("st-mark"))) assertEquals(page, decodePage(page.encode()))
    }

    companion object {
        private val core by lazy { OfficeCore() }
    }
}
