package org.orthodoxwest.office

import android.app.Application
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.saveable.SaverScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.hasScrollToIndexAction
import androidx.compose.ui.test.junit4.StateRestorationTester
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.performScrollToIndex
import androidx.lifecycle.SavedStateHandle
import androidx.test.core.app.ApplicationProvider
import java.time.LocalDate
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.PrayedReading
import org.orthodoxwest.office.core.hourNames
import org.orthodoxwest.office.core.reopensAtPlace
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** A return after Android has closed the app in the background lands where the reader was. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class PlaceTest {
    @get:Rule
    val compose = createComposeRule()

    private val app: Application = ApplicationProvider.getApplicationContext()
    private val lent = LocalDate.of(2026, 3, 15)

    @Test
    fun theWayBackIsRestored() {
        val saved = SavedStateHandle()
        val before = OfficeViewModel(app, saved)
        before.open(Page.Ordo(2026, 3))
        before.open(Page.Hour(lent, "vespers"))
        // A new process, handed the saved state.
        val after = OfficeViewModel(app, saved)
        assertEquals(Page.Hour(lent, "vespers"), after.page)
        assertEquals(before.entries.toList(), after.entries.toList())
        after.back()
        assertEquals(Page.Ordo(2026, 3), after.page)
        // New visits never reuse a restored one's id, and so never its scroll position.
        after.open(Page.Hour(lent, "compline"))
        assertTrue(after.entry.id > before.entries.maxOf { it.id })
    }

    @Test
    fun eachVisitKeepsItsScrollPosition() {
        val hour = Office.core.compose("vespers", 2026, 3, 15, "private")
        var entry by mutableLongStateOf(1)
        val restoring = StateRestorationTester(compose)
        restoring.setContent {
            OfficeTheme(choice = ThemeChoice.NAVE) {
                OfficeApp(
                    Page.Hour(lent, "vespers"), lent, hourNames(), null, hour, null, null, "private", ThemeChoice.NAVE, TextSize.DEFAULT, PaddingValues(),
                    onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                    reminders = ReminderStore(app).load(), reminderStatus = ReminderStatus(notificationsAllowed = true, exactAllowed = true),
                    onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                    entry = entry, entries = listOf(1L, 2L),
                )
            }
        }
        val list = compose.onNode(hasScrollToIndexAction())
        fun position() = list.fetchSemanticsNode().config[SemanticsProperties.VerticalScrollAxisRange].value()
        list.performScrollToIndex(12)
        val read = position()
        assertTrue(read > 0f)
        restoring.emulateSavedInstanceStateRestore()
        assertEquals(read, position())
        // Another visit to the same hour starts at the top, and the first keeps its place.
        entry = 2
        compose.waitForIdle()
        assertEquals(0f, position())
        entry = 1
        compose.waitForIdle()
        assertEquals(read, position())
    }

    /**
     * The ribbon: an hour Android closed in the background opens at the reader's place only
     * within the core's limit and before it is prayed; one left by the app's own ways (saved in
     * front) keeps its place for Back.
     */
    @Test
    fun anHourTakenAwayOpensAtItsPlaceOnlyWithinTheLimit() {
        val now = System.currentTimeMillis()
        fun reopened(leftAt: Long, prayed: Boolean = false): Int {
            val saver = placeSaver(leftAt = { leftAt }, reopens = { at -> reopensAtPlace(prayed, (System.currentTimeMillis() - at) / 1000.0) })
            val saved = with(saver) { SaverScope { true }.save(LazyListState(12, 40)) }!!
            return saver.restore(saved)!!.firstVisibleItemIndex
        }
        assertEquals(12, reopened(0L))
        assertEquals(12, reopened(now - 10 * 60_000L))
        assertEquals(0, reopened(now - 46 * 60_000L))
        assertEquals(0, reopened(now - 60_000L, prayed = true))
    }

    /**
     * The visit's place carries the prayed watch's reading, so an hour reopened at the place can
     * still earn its mark; an hour reopened at its top begins the reading again, and a place
     * saved before the reading was carried still restores.
     */
    @Test
    fun theKeptPlaceCarriesTheReading() {
        val now = System.currentTimeMillis()
        fun carried(leftAt: Long): PrayedReading? {
            var got: PrayedReading? = null
            val saver = placeSaver(
                leftAt = { leftAt },
                reopens = { at -> reopensAtPlace(false, (System.currentTimeMillis() - at) / 1000.0) },
                reading = { PrayedReading(640.0, 512.0) },
                carry = { got = it },
            )
            val saved = with(saver) { SaverScope { true }.save(LazyListState(12, 40)) }!!
            saver.restore(saved)
            return got
        }
        assertEquals(PrayedReading(640.0, 512.0), carried(now - 10 * 60_000L))
        assertEquals(PrayedReading(640.0, 512.0), carried(0L))
        assertEquals(null, carried(now - 46 * 60_000L))
        val older = placeSaver({ 0L }, { true }).restore(listOf(12L, 40L, now - 60_000L))!!
        assertEquals(12, older.firstVisibleItemIndex)
    }
}
