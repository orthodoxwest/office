package org.orthodoxwest.office

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasScrollToIndexAction
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performScrollToNode
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
 * Hymns scrolled into view, for review beside the web: a short-lined hymn centred on its longest
 * line (Compline's), and a long-lined one filling the measure (Lauds').
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h2400dp-xxhdpi")
class HymnScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private fun hymn(hour: String) {
        val lent = LocalDate.of(2026, 3, 15)
        val view = Office.core.compose(hour, 2026, 3, 15, "private")
        compose.setContent {
            OfficeTheme(choice = ThemeChoice.NAVE) {
                OfficeApp(
                    Page.Hour(lent, hour), lent, hourNames(), null, view, null, null, "private", ThemeChoice.NAVE, TextSize.DEFAULT, PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = ReminderStore(ApplicationProvider.getApplicationContext()).load(), reminderStatus = ReminderStatus(true, true),
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                )
            }
        }
        compose.onNode(hasScrollToIndexAction()).performScrollToNode(hasContentDescription("Hymn"))
        compose.waitForIdle()
        compose.onRoot().captureRoboImage("build/screenshots/hymn-$hour.png")
    }

    @Test fun compline() = hymn("compline")
    @Test fun lauds() = hymn("lauds")
}
