package org.orthodoxwest.office

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasScrollToIndexAction
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performScrollToNode
import androidx.compose.ui.test.performSemanticsAction
import androidx.test.core.app.ApplicationProvider
import com.github.takahirom.roborazzi.captureRoboImage
import java.time.LocalDate
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.hourNames
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Google Play listing screenshots in build/screenshots/store/, so CI's android-screenshots
 * artifact carries them. Every size is 9:16 portrait, the shape Play features: the phone at
 * 1080×1920, a 7-inch tablet at 1224×2176 and a 10-inch one at 1800×3200 (the rail's
 * composition, from 760dp). The pages and the day, the third Sunday of Lent, follow the App
 * Store's set (ios.yml), so both stores show the same screens.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = PHONE)
class StoreScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private val lent = LocalDate.of(2026, 3, 15)

    private fun show(page: Page, theme: ThemeChoice, chant: Boolean = false) {
        val core = Office.core
        val home = (page as? Page.Home)?.let { core.home(it.date.toCivil(), lent.toCivil(), 18) }
        val hour = (page as? Page.Hour)?.let {
            core.compose(it.hour, it.date.year, it.date.monthValue, it.date.dayOfMonth, "private", false, chant)
        }
        val ordo = (page as? Page.Ordo)?.let { core.ordoMonth(it.year, it.month) }
        val feast = (page as? Page.Feast)?.let { core.feastPage(it.id, lent.toCivil()) }
        val reminders = ReminderStore(ApplicationProvider.getApplicationContext()).load().copy(on = true)
        compose.setContent {
            OfficeTheme(choice = theme, season = home?.ornament ?: hour?.ornament ?: "") {
                OfficeApp(
                    page, lent, hourNames(), home, hour, ordo, null, "private", theme, TextSize.DEFAULT, PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = reminders, reminderStatus = ReminderStatus(notificationsAllowed = true, exactAllowed = true),
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                    feast = feast,
                )
            }
        }
        compose.waitForIdle()
    }

    private fun shoot(name: String) = compose.onRoot().captureRoboImage("build/screenshots/store/$name.png")

    private fun home(theme: ThemeChoice) = show(Page.Home(lent), theme)
    private fun feast() = show(Page.Feast("st-benedict"), ThemeChoice.NAVE)
    private fun hour(hour: String, theme: ThemeChoice) = show(Page.Hour(lent, hour), theme)
    private fun ordo() = show(Page.Ordo(2026, 3), ThemeChoice.NAVE)
    private fun year() = show(Page.Year(2026), ThemeChoice.NAVE)

    /** Lent's Vespers hymn sung from its notation, its heading brought near the top of the screen. */
    private fun chant() {
        show(Page.Hour(lent, "vespers"), ThemeChoice.NAVE, chant = true)
        val list = compose.onNode(hasScrollToIndexAction())
        list.performScrollToNode(hasContentDescription("Hymn"))
        compose.waitForIdle()
        val top = compose.onNode(hasContentDescription("Hymn")).fetchSemanticsNode().boundsInRoot.top
        val margin = compose.onRoot().fetchSemanticsNode().size.height * 0.06f
        list.performSemanticsAction(SemanticsActions.ScrollBy) { it(0f, top - margin) }
        compose.waitForIdle()
    }

    // The phone, in the listing's order.
    @Test fun phone1Home() { home(ThemeChoice.NAVE); shoot("phone-1-home") }
    @Test fun phone2Feast() { feast(); shoot("phone-2-feast") }
    @Test fun phone3Chant() { chant(); shoot("phone-3-chant") }
    @Test fun phone4Compline() { hour("compline", ThemeChoice.NAVE); shoot("phone-4-compline") }
    @Test fun phone5ComplineApse() { hour("compline", ThemeChoice.APSE); shoot("phone-5-compline-apse") }
    @Test fun phone6Ordo() { ordo(); shoot("phone-6-ordo") }
    @Test fun phone7Year() { year(); shoot("phone-7-year") }
    @Test fun phone8Reminders() { show(Page.Reminders, ThemeChoice.NAVE); shoot("phone-8-reminders") }

    // The 7-inch tablet.
    @Test @Config(qualifiers = TABLET7) fun tablet7a1Home() { home(ThemeChoice.NAVE); shoot("tablet7-1-home") }
    @Test @Config(qualifiers = TABLET7) fun tablet7a2Chant() { chant(); shoot("tablet7-2-chant") }
    @Test @Config(qualifiers = TABLET7) fun tablet7a3Feast() { feast(); shoot("tablet7-3-feast") }
    @Test @Config(qualifiers = TABLET7) fun tablet7a4Vespers() { hour("vespers", ThemeChoice.NAVE); shoot("tablet7-4-vespers") }
    @Test @Config(qualifiers = TABLET7) fun tablet7a5Ordo() { ordo(); shoot("tablet7-5-ordo") }
    @Test @Config(qualifiers = TABLET7) fun tablet7a6Year() { year(); shoot("tablet7-6-year") }
    @Test @Config(qualifiers = TABLET7) fun tablet7a7HomeApse() { home(ThemeChoice.APSE); shoot("tablet7-7-home-apse") }

    // The 10-inch tablet, as the iPad's set.
    @Test @Config(qualifiers = TABLET10) fun tablet10a1Home() { home(ThemeChoice.NAVE); shoot("tablet10-1-home") }
    @Test @Config(qualifiers = TABLET10) fun tablet10a2Chant() { chant(); shoot("tablet10-2-chant") }
    @Test @Config(qualifiers = TABLET10) fun tablet10a3Feast() { feast(); shoot("tablet10-3-feast") }
    @Test @Config(qualifiers = TABLET10) fun tablet10a4Vespers() { hour("vespers", ThemeChoice.NAVE); shoot("tablet10-4-vespers") }
    @Test @Config(qualifiers = TABLET10) fun tablet10a5Ordo() { ordo(); shoot("tablet10-5-ordo") }
    @Test @Config(qualifiers = TABLET10) fun tablet10a6Year() { year(); shoot("tablet10-6-year") }
    @Test @Config(qualifiers = TABLET10) fun tablet10a7HomeApse() { home(ThemeChoice.APSE); shoot("tablet10-7-home-apse") }
}

private const val PHONE = "w360dp-h640dp-xxhdpi"
private const val TABLET7 = "w612dp-h1088dp-xhdpi"
private const val TABLET10 = "w900dp-h1600dp-xhdpi"
