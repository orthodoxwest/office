package org.orthodoxwest.office

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import com.github.takahirom.roborazzi.captureRoboImage
import com.github.takahirom.roborazzi.captureScreenRoboImage
import java.time.LocalDate
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.OfficeCore
import org.orthodoxwest.office.core.hourNames
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Renders real pages from the Rust core into build/screenshots/, for review beside the web's
 * visual snapshots (.web-tools/tests/visual.spec.js-snapshots). Tall captures show most of a page.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class ScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private val lent = LocalDate.of(2026, 3, 15)

    private fun show(page: Page, theme: ThemeChoice, today: LocalDate = lent, form: String = "private") {
        val home = (page as? Page.Home)?.let { core.home(it.date.toCivil(), today.toCivil(), 18) }
        val hour = (page as? Page.Hour)?.let { core.compose(it.hour, it.date.year, it.date.monthValue, it.date.dayOfMonth, form) }
        val ordo = (page as? Page.Ordo)?.let { core.ordoMonth(it.year, it.month) }
        compose.setContent {
            OfficeTheme(choice = theme, season = home?.ornament ?: hour?.ornament ?: "") {
                OfficeApp(
                    page, today, hourNames(), home, hour, ordo, null, form, theme, TextSize.DEFAULT, PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                )
            }
        }
    }

    private fun shoot(name: String) = compose.onRoot().captureRoboImage("build/screenshots/$name.png")

    @Test
    fun homeNave() { show(Page.Home(lent), ThemeChoice.NAVE); shoot("home-nave") }

    @Test
    fun homeApse() { show(Page.Home(lent), ThemeChoice.APSE); shoot("home-apse") }

    @Test
    fun homeAnotherDay() { show(Page.Home(LocalDate.of(2026, 12, 25)), ThemeChoice.NAVE); shoot("home-christmas") }

    @Test
    fun menuOnAnHour() {
        show(Page.Hour(lent, "lauds"), ThemeChoice.APSE)
        compose.onNodeWithText("MENU").performClick()
        compose.waitForIdle()
        captureScreenRoboImage("build/screenshots/menu-hour-apse.png")
    }

    @Test
    fun laudsNave() { show(Page.Hour(lent, "lauds"), ThemeChoice.NAVE, today = lent.plusDays(3)); shoot("lauds-nave") }

    @Test
    fun laudsApse() { show(Page.Hour(lent, "lauds"), ThemeChoice.APSE, today = lent.plusDays(3)); shoot("lauds-apse") }

    @Test
    @Config(qualifiers = "w390dp-h6000dp-xxhdpi")
    fun laudsTall() { show(Page.Hour(lent, "lauds"), ThemeChoice.NAVE); shoot("lauds-tall") }

    @Test
    @Config(qualifiers = "w390dp-h6000dp-xxhdpi")
    fun complinePriestTall() { show(Page.Hour(LocalDate.of(2026, 9, 30), "compline"), ThemeChoice.APSE, form = "priest"); shoot("compline-priest-tall") }

    @Test
    @Config(qualifiers = "w390dp-h6000dp-xxhdpi")
    fun vespersEastertideTall() { show(Page.Hour(LocalDate.of(2026, 4, 12), "vespers"), ThemeChoice.NAVE); shoot("vespers-eastertide-tall") }

    @Test
    @Config(qualifiers = "w390dp-h2400dp-xxhdpi")
    fun ordoNave() { show(Page.Ordo(2026, 3), ThemeChoice.NAVE); shoot("ordo-nave") }

    @Test
    fun ordoApse() { show(Page.Ordo(2026, 3), ThemeChoice.APSE); shoot("ordo-apse") }

    @Test
    fun datePicker() {
        show(Page.Hour(lent, "lauds"), ThemeChoice.NAVE, today = LocalDate.of(2026, 3, 18))
        compose.onNodeWithText("CHANGE DATE", substring = true).performClick()
        compose.waitForIdle()
        shoot("date-picker-nave")
    }

    companion object {
        private val core by lazy { OfficeCore() }
    }
}
