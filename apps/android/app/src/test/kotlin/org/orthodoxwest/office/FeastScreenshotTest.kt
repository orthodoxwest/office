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

    // A first-class feast: the gilt cross, the doubled course, the collect's initial.
    @Test
    @Config(qualifiers = "w390dp-h2000dp-xxhdpi")
    fun feastFirstNave() { show(Page.Feast("annunciation"), ThemeChoice.NAVE); shoot("feast-first-nave") }

    @Test
    @Config(qualifiers = "w390dp-h2000dp-xxhdpi")
    fun feastFirstApse() { show(Page.Feast("annunciation"), ThemeChoice.APSE); shoot("feast-first-apse") }

    // A black day is bare: no pool behind the head and no wash in the standing.
    @Test
    @Config(qualifiers = "w390dp-h2000dp-xxhdpi")
    fun feastBlack() { show(Page.Feast("all-souls"), ThemeChoice.NAVE); shoot("feast-black") }

    // A monastic observance: the bare head, and the standing's prose over the next keeping.
    @Test
    @Config(qualifiers = "w390dp-h1400dp-xxhdpi")
    fun monasticFeast() { show(Page.Feast("solemnity-st-benedict"), ThemeChoice.NAVE); shoot("feast-monastic") }

    @Test
    @Config(qualifiers = "w390dp-h1400dp-xxhdpi")
    fun monasticFeastApse() { show(Page.Feast("solemnity-st-benedict"), ThemeChoice.APSE); shoot("feast-monastic-apse") }

    // The rail from 760dp, with a first-class feast's upright course, and the desk's wider one.
    @Test
    @Config(qualifiers = "w820dp-h1300dp-xhdpi")
    fun feastTablet() { show(Page.Feast("annunciation"), ThemeChoice.NAVE); shoot("feast-tablet") }

    @Test
    @Config(qualifiers = "w820dp-h1300dp-xhdpi")
    fun feastTabletApse() { show(Page.Feast("annunciation"), ThemeChoice.APSE); shoot("feast-tablet-apse") }

    @Test
    @Config(qualifiers = "w1024dp-h1300dp-xhdpi")
    fun feastDesk() { show(Page.Feast("st-mark"), ThemeChoice.NAVE); shoot("feast-desk") }

    // The head says the feast's usual date over its name, and the standing the next keeping alone.
    @Test
    @Config(qualifiers = "w390dp-h2000dp-xxhdpi")
    fun headAndStanding() {
        show(Page.Feast("annunciation"), ThemeChoice.NAVE)
        val view = core.feastPage("annunciation", today.toCivil())!!
        compose.onNodeWithText(view.titulus).assertExists()
        compose.onNodeWithText(view.name).assertExists()
        compose.onNodeWithText("${core_words.daysHours} ›").assertExists()
        assertEquals("first", view.register)
    }

    // On the day itself the standing's way on is today's hours.
    @Test
    @Config(qualifiers = "w390dp-h2000dp-xxhdpi")
    fun keptTodaySaysSo() {
        val day = LocalDate.of(2027, 3, 25)
        val view = core.feastPage("annunciation", day.toCivil())!!
        assertEquals(true, view.next?.today)
        var opened: Page? = null
        compose.setContent {
            OfficeTheme(choice = ThemeChoice.NAVE, season = "") {
                FeastScreen(view, chrome = {}, insets = PaddingValues(), onFind = {}, onFeast = {}, onDay = { opened = Page.Home(it) })
            }
        }
        compose.onNodeWithText("${core_words.todaysHours} ›").assertExists()
        shoot("feast-today")
        compose.onNodeWithText("${core_words.todaysHours} ›").performClick()
        assertEquals(Page.Home(day), opened)
    }

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
        private val core_words by lazy { org.orthodoxwest.office.core.feastWords() }
    }
}
