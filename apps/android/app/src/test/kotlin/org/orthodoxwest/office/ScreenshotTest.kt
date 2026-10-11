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
import androidx.compose.ui.test.junit4.v2.createComposeRule
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

    /** A first-class day's hour stands in its frame. */
    @Test
    fun laudsChristmasNave() { show(Page.Hour(LocalDate.of(2026, 12, 25), "lauds"), ThemeChoice.NAVE); shoot("lauds-christmas-nave") }

    @Test
    fun laudsChristmasApse() { show(Page.Hour(LocalDate.of(2026, 12, 25), "lauds"), ThemeChoice.APSE); shoot("lauds-christmas-apse") }

    // The head's type is fitted to the panel: a small system font scale leaves it as it is.
    @Test
    @Config(fontScale = 0.85f)
    fun homeSmallSystemText() { show(Page.Home(lent), ThemeChoice.NAVE); shoot("home-small-system-text") }

    // A tablet: the prayer a step larger from 920dp, and the ordo's days one line each.
    @Test
    @Config(qualifiers = "w1024dp-h3000dp-xhdpi")
    fun vespersTablet() { show(Page.Hour(lent, "vespers"), ThemeChoice.NAVE); shoot("vespers-tablet") }

    @Test
    @Config(qualifiers = "w1024dp-h1366dp-xhdpi")
    fun ordoTablet() { show(Page.Ordo(2026, 3), ThemeChoice.NAVE); shoot("ordo-tablet") }

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

    /**
     * Android's Bold text setting (and Samsung's font weight) adds 300 to every weight. The page
     * must keep Garamond's small caps and ℣/℟ with a heavier stroke, not fall to a partial face.
     */
    @Test
    @Config(qualifiers = "w390dp-h1400dp-xxhdpi")
    fun boldTextSetting() {
        val res = ApplicationProvider.getApplicationContext<Context>().resources
        val bold = Configuration(res.configuration).apply { fontWeightAdjustment = 300 }
        @Suppress("DEPRECATION")
        res.updateConfiguration(bold, res.displayMetrics)
        show(Page.Hour(LocalDate.of(2026, 9, 30), "terce"), ThemeChoice.APSE)
        shoot("bold-text-terce")
    }

    /** The launcher shortcuts' icons, masked to the circle most launchers draw. */
    @Test
    @Config(qualifiers = "w390dp-h140dp-xxhdpi")
    fun shortcutIcons() {
        compose.setContent {
            Row(Modifier.fillMaxSize().background(Color(0xFF3C3C3C)), horizontalArrangement = Arrangement.SpaceEvenly, verticalAlignment = Alignment.CenterVertically) {
                listOf(R.drawable.shortcut_lauds_glyph, R.drawable.shortcut_vespers_glyph, R.drawable.shortcut_compline_glyph, R.drawable.shortcut_ordo_glyph).forEach {
                    // A 108dp layer, of which a launcher shows the middle 72dp.
                    Box(Modifier.size(72.dp).clip(CircleShape).background(Color(0xFF121C28)), contentAlignment = Alignment.Center) {
                        Image(painterResource(it), null, Modifier.requiredSize(108.dp))
                    }
                }
            }
        }
        shoot("shortcut-icons")
    }

    @Test
    @Config(qualifiers = "w390dp-h1400dp-xxhdpi")
    fun ordoYearNave() { show(Page.Year(2026), ThemeChoice.NAVE); shoot("ordo-year-nave") }

    @Test
    fun datePicker() {
        show(Page.Hour(lent, "lauds"), ThemeChoice.NAVE, today = LocalDate.of(2026, 3, 18))
        compose.onNodeWithText("CHANGE DATE", substring = true).performClick()
        compose.waitForIdle()
        shoot("date-picker-nave")
    }

    @Test
    fun datePickerMonths() {
        show(Page.Hour(lent, "lauds"), ThemeChoice.NAVE, today = LocalDate.of(2026, 3, 18))
        compose.onNodeWithText("CHANGE DATE", substring = true).performClick()
        compose.onNodeWithText("MARCH 2026").performClick()
        compose.waitForIdle()
        shoot("date-picker-months-nave")
    }

    @Test
    @Config(qualifiers = "w390dp-h1500dp-xxhdpi")
    fun remindersNave() { show(Page.Reminders, ThemeChoice.NAVE); shoot("reminders-nave") }

    @Test
    @Config(qualifiers = "w390dp-h1500dp-xxhdpi")
    fun remindersOnApse() {
        val on = ReminderStore(ApplicationProvider.getApplicationContext()).load().copy(on = true)
        show(Page.Reminders, ThemeChoice.APSE, reminders = on, status = ReminderStatus(notificationsAllowed = true, exactAllowed = false))
        shoot("reminders-on-apse")
    }

    companion object {
        private val core by lazy { OfficeCore() }
    }
}
