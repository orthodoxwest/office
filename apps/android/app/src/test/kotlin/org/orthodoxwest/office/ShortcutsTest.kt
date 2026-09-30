package org.orthodoxwest.office

import android.app.Application
import androidx.core.content.pm.ShortcutManagerCompat
import androidx.test.core.app.ApplicationProvider
import java.time.LocalDate
import java.time.LocalDateTime
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The launcher's shortcuts, and the page each opens when tapped. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class ShortcutsTest {
    private val app: Application = ApplicationProvider.getApplicationContext()

    @Test
    fun theHoursAndTheOrdoAreOffered() {
        Shortcuts.publish(app)
        val shown = ShortcutManagerCompat.getDynamicShortcuts(app).sortedBy { it.rank }
        assertEquals(listOf("Lauds", "Vespers", "Compline", "Ordo"), shown.map { it.shortLabel.toString() })
        assertEquals(Shortcuts.OPEN, shown.first().intent.action)
    }

    @Test
    fun eachOpensTheDayItIsTapped() {
        val evening = LocalDateTime.of(2026, 3, 15, 19, 30)
        assertEquals(Page.Hour(LocalDate.of(2026, 3, 15), "vespers"), Shortcuts.page("vespers", evening))
        assertEquals(Page.Ordo(2026, 3, 15), Shortcuts.page(Shortcuts.ORDO, evening))
        // Compline after midnight is the day before's; Lauds at that hour is already the new day's.
        val late = LocalDateTime.of(2026, 3, 16, 1, 0)
        assertEquals(Page.Hour(LocalDate.of(2026, 3, 15), "compline"), Shortcuts.page("compline", late))
        assertEquals(Page.Hour(LocalDate.of(2026, 3, 16), "lauds"), Shortcuts.page("lauds", late))
        assertNull(Shortcuts.page("matins", evening))
    }
}
