package org.orthodoxwest.office

import android.app.Application
import android.os.Looper
import androidx.lifecycle.SavedStateHandle
import androidx.test.core.app.ApplicationProvider
import java.time.LocalDate
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

/** Pages move the way they lead, and the screen never shows a page before its content. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class MotionTest {
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
        // Home stays on screen until Vespers is composed.
        assertEquals(home.entry, vm.shown!!.entry)
        settle(vm)
        assertEquals(Page.Hour(lent, "vespers"), vm.shown!!.entry.page)
        // Home is at hand behind it, and Back shows it at once.
        assertEquals(home.entry, vm.behind!!.entry)
        vm.back()
        assertEquals(Motion.BACK, vm.motion)
        assertEquals(home.entry, vm.shown!!.entry)
        assertNull(vm.behind)
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
