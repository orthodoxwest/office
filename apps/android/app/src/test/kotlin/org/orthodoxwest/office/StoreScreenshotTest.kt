package org.orthodoxwest.office

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
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
 * Google Play listing screenshots in build/screenshots/store/, so CI's android-screenshots
 * artifact carries them: 1080×1920, the 9:16 portrait shape Play features.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w360dp-h640dp-xxhdpi")
class StoreScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private val allSaints = LocalDate.of(2026, 11, 1)

    private fun show(page: Page, theme: ThemeChoice, today: LocalDate, form: String = "private") {
        val home = (page as? Page.Home)?.let { core.home(it.date.toCivil(), today.toCivil(), 18) }
        val hour = (page as? Page.Hour)?.let { core.compose(it.hour, it.date.year, it.date.monthValue, it.date.dayOfMonth, form) }
        val ordo = (page as? Page.Ordo)?.let { core.ordoMonth(it.year, it.month) }
        val reminders = ReminderStore(ApplicationProvider.getApplicationContext()).load().copy(on = true)
        compose.setContent {
            OfficeTheme(choice = theme, season = home?.ornament ?: hour?.ornament ?: "") {
                OfficeApp(
                    page, today, hourNames(), home, hour, ordo, null, form, theme, TextSize.DEFAULT, PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = reminders, reminderStatus = ReminderStatus(notificationsAllowed = true, exactAllowed = true),
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                )
            }
        }
    }

    private fun shoot(name: String) = compose.onRoot().captureRoboImage("build/screenshots/store/$name.png")

    @Test
    fun homeNave() { show(Page.Home(allSaints), ThemeChoice.NAVE, allSaints); shoot("home-nave") }

    @Test
    fun homeApse() { show(Page.Home(allSaints), ThemeChoice.APSE, allSaints); shoot("home-apse") }

    @Test
    fun laudsNave() { show(Page.Hour(allSaints, "lauds"), ThemeChoice.NAVE, allSaints); shoot("lauds-nave") }

    @Test
    fun vespersNave() { show(Page.Hour(allSaints, "vespers"), ThemeChoice.NAVE, allSaints); shoot("vespers-nave") }

    @Test
    fun complineApse() { show(Page.Hour(allSaints, "compline"), ThemeChoice.APSE, allSaints); shoot("compline-apse") }

    @Test
    fun ordoNave() { show(Page.Ordo(2026, 11), ThemeChoice.NAVE, allSaints); shoot("ordo-nave") }

    @Test
    fun ordoApse() { show(Page.Ordo(2026, 11), ThemeChoice.APSE, allSaints); shoot("ordo-apse") }

    @Test
    fun remindersNave() { show(Page.Reminders, ThemeChoice.NAVE, allSaints); shoot("reminders-nave") }

    @Test
    fun menuApse() {
        show(Page.Hour(allSaints, "lauds"), ThemeChoice.APSE, allSaints)
        compose.onNodeWithText("MENU").performClick()
        compose.waitForIdle()
        captureScreenRoboImage("build/screenshots/store/menu-apse.png")
    }

    companion object {
        private val core by lazy { OfficeCore() }
    }
}
