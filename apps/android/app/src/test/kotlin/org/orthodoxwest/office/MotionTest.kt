package org.orthodoxwest.office

import android.app.Application
import android.os.Looper
import android.view.View
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.lifecycle.SavedStateHandle
import androidx.test.core.app.ApplicationProvider
import java.time.LocalDate
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.hourNames
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

/** Pages move the way they lead, and the screen never shows a page before its content. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class MotionTest {
    @get:Rule
    val compose = createComposeRule()

    private val app: Application = ApplicationProvider.getApplicationContext()
    private val lent = LocalDate.of(2026, 3, 15)

    @Test
    fun aPageOfTheSameKindStepsByItsPlaceInTime() {
        assertEquals(Motion.NEXT, stepBetween(Page.Hour(lent, "lauds"), Page.Hour(lent, "prime")))
        assertEquals(Motion.PREVIOUS, stepBetween(Page.Hour(lent, "lauds"), Page.Hour(lent.minusDays(1), "compline")))
        assertEquals(Motion.NEXT, stepBetween(Page.Home(lent), Page.Home(lent.plusDays(1))))
        assertEquals(Motion.PREVIOUS, stepBetween(Page.Ordo(2026, 3), Page.Ordo(2025, 12)))
        assertEquals(Motion.FADE, stepBetween(Page.Ordo(2026, 3, 1), Page.Ordo(2026, 3, 15)))
    }

    @Test
    fun theScreenWaitsForContentAndBackRevealsThePageBehind() {
        val vm = OfficeViewModel(app, SavedStateHandle())
        settle(vm)
        val home = vm.shown!!
        vm.open(Page.Hour(lent, "vespers"))
        assertEquals(Motion.FORWARD, vm.motion)
        // Home stays on screen until Vespers is composed, and Vespers never appears without its
        // content. Vespers composes off the main thread and may already be ready here, so either
        // page may be the one shown, but nothing else.
        val opening = vm.shown!!
        assertTrue(opening == home || (opening.entry == vm.entry && opening.content is Content.Hour))
        settle(vm)
        assertEquals(Page.Hour(lent, "vespers"), vm.shown!!.entry.page)
        // Home is at hand behind it, and Back shows it at once.
        assertEquals(home.entry, vm.behind!!.entry)
        vm.back()
        assertEquals(Motion.BACK, vm.motion)
        assertEquals(home.entry, vm.shown!!.entry)
        assertNull(vm.behind)
    }

    @Test
    fun theScreenStaysAwakeAsOneHourGivesWayToTheNext() {
        val vespers = Office.core.compose("vespers", 2026, 3, 15, "private")
        val compline = Office.core.compose("compline", 2026, 3, 15, "private")
        var shown by mutableStateOf(Shown(Entry(1, Page.Hour(lent, "vespers")), Content.Hour(vespers)))
        lateinit var view: View
        compose.setContent {
            view = LocalView.current
            OfficeTheme(choice = ThemeChoice.NAVE) {
                OfficeApp(
                    shown = shown, behind = null, motion = Motion.NEXT, onBack = null, today = lent, hours = hourNames(),
                    form = "private", theme = ThemeChoice.NAVE, textSize = TextSize.DEFAULT, insets = PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = ReminderStore(app).load(), reminderStatus = ReminderStatus(notificationsAllowed = true, exactAllowed = true),
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                    entries = listOf(shown.entry.id),
                )
            }
        }
        assertTrue(view.keepScreenOn)
        shown = Shown(Entry(2, Page.Hour(lent, "compline")), Content.Hour(compline))
        compose.waitForIdle()
        assertTrue(view.keepScreenOn)
    }

    /** Runs the main looper until the current visit's content is on screen. */
    private fun settle(vm: OfficeViewModel) {
        val until = System.currentTimeMillis() + 30_000
        while (vm.shown?.entry != vm.entry && System.currentTimeMillis() < until) {
            shadowOf(Looper.getMainLooper()).idle()
            Thread.sleep(5)
        }
        assertEquals(vm.entry, vm.shown?.entry)
    }
}
