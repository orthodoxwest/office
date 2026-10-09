package org.orthodoxwest.office

import android.content.Context
import android.content.res.Configuration
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.requiredSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
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
 * The same pages at a tablet's and a desktop's width, for review beside the web's desktop
 * snapshots (1280×900), and at a phone's in landscape.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w1280dp-h900dp-mdpi")
class WideScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private val core get() = Office.core
    private val lent = LocalDate.of(2026, 3, 15)

    private fun show(
        page: Page,
        theme: ThemeChoice,
        today: LocalDate = lent,
        form: String = "private",
        reminders: ReminderSettings = ReminderStore(ApplicationProvider.getApplicationContext()).load(),
        status: ReminderStatus = ReminderStatus(notificationsAllowed = true, exactAllowed = true),
    ) {
        val home = (page as? Page.Home)?.let { core.home(it.date.toCivil(), today.toCivil(), 18) }
        val hour = (page as? Page.Hour)?.let { core.compose(it.hour, it.date.year, it.date.monthValue, it.date.dayOfMonth, form) }
        val ordo = (page as? Page.Ordo)?.let { core.ordoMonth(it.year, it.month) }
        compose.setContent {
            OfficeTheme(choice = theme, season = home?.ornament ?: hour?.ornament ?: "") {
                OfficeApp(
                    page, today, hourNames(), home, hour, ordo, null, form, theme, TextSize.DEFAULT, PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = reminders, reminderStatus = status,
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                )
            }
        }
    }

    private fun shoot(name: String) = compose.onRoot().captureRoboImage("build/screenshots/wide-$name.png")

    @Test fun home() { show(Page.Home(lent), ThemeChoice.NAVE); shoot("home") }

    @Test fun homeApse() { show(Page.Home(lent), ThemeChoice.APSE); shoot("home-apse") }

    @Test fun lauds() { show(Page.Hour(lent, "lauds"), ThemeChoice.NAVE); shoot("lauds") }

    @Test fun ordo() { show(Page.Ordo(2026, 3), ThemeChoice.NAVE); shoot("ordo") }

    @Test fun year() { show(Page.Year(2026), ThemeChoice.NAVE); shoot("year") }

    @Test fun reminders() { show(Page.Reminders, ThemeChoice.NAVE); shoot("reminders") }

    @Test fun aboutPage() { show(Page.About, ThemeChoice.NAVE); shoot("about") }

    /** About closes the header's links on home, as on every page but an hour. */
    @Test
    fun aboutInTheHeader() {
        show(Page.Home(lent), ThemeChoice.NAVE)
        compose.onNodeWithText("ABOUT").assertExists()
    }

    /** A wide hour's header is full with the seven hours, so About is left out of it, as on the web. */
    @Test
    fun aboutLeavesAWideHoursHeader() {
        show(Page.Hour(lent, "lauds"), ThemeChoice.NAVE)
        compose.onAllNodesWithText("ABOUT").assertCountEquals(0)
    }

    /** The theme and text size wait under the header's Settings; no page ends in them. */
    @Test
    fun settings() {
        show(Page.Home(lent), ThemeChoice.NAVE)
        compose.onAllNodesWithText("APSE").assertCountEquals(0)
        compose.onNodeWithText("SETTINGS").performClick()
        compose.onNodeWithText("APSE").assertExists()
        compose.onNodeWithText("NAVE").performClick()
        captureScreenRoboImage("build/screenshots/wide-settings.png")
    }

    /** Upright, the niche stands centred in the room with the light on it, as on the web. */
    @Test
    @Config(qualifiers = "w800dp-h1280dp-mdpi")
    fun tabletHome() { show(Page.Home(lent), ThemeChoice.NAVE); shoot("tablet-home") }

    @Test
    @Config(qualifiers = "w800dp-h1280dp-mdpi")
    fun tabletLauds() { show(Page.Hour(lent, "lauds"), ThemeChoice.APSE); shoot("tablet-lauds") }

    @Test
    @Config(qualifiers = "w844dp-h390dp-mdpi")
    fun landscapeHome() { show(Page.Home(lent), ThemeChoice.NAVE); shoot("landscape-home") }

    @Test
    @Config(qualifiers = "w844dp-h390dp-mdpi")
    fun landscapeCompline() { show(Page.Hour(lent, "compline"), ThemeChoice.NAVE); shoot("landscape-compline") }
}
