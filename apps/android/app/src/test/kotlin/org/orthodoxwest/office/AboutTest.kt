package org.orthodoxwest.office

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.orthodoxwest.office.core.CivilDate
import org.orthodoxwest.office.core.about
import org.orthodoxwest.office.core.aboutNewcomer
import org.orthodoxwest.office.core.aboutNow
import org.orthodoxwest.office.core.hourNames

/** About the Office as the core gives it: the page AboutScreen sets, and the first week home offers it in. */
class AboutTest {
    private val view = about()

    @Test
    fun thePageHasItsTitleAndTheSevenHours() {
        assertTrue(view.title.isNotBlank())
        assertEquals(hourNames(), view.periods.flatMap { p -> p.hours.map { it.hour } })
        assertTrue(view.periods.flatMap { it.hours }.all { it.name.isNotBlank() && it.gloss.isNotBlank() && it.time.isNotBlank() })
        assertEquals(1, view.blocks.count { it.kind == "hours" })
    }

    /** AboutScreen opens the ordo and the reminders itself, and leaves the rest to the browser. */
    @Test
    fun linksGoOnlyWhereTheAppCanFollow() {
        val links = view.blocks.flatMap { it.runs }.map { it.link }.filter { it.isNotEmpty() }
        assertTrue(links.isNotEmpty())
        for (link in links) assertTrue(link, link in listOf("/calendar", "/reminders", "/privacy") || link.startsWith("https://"))
    }

    /** The table lights the hour home lights, none from midnight to 2 am, and the page ends with home's invitation. */
    @Test
    fun theHourItIsNowIsHomes() {
        val today = CivilDate(2026, 11, 13)
        val afternoon = aboutNow(today, 15)
        assertEquals("none", afternoon.current)
        assertEquals("Pray None", afternoon.prayNowLabel)
        assertEquals(today, afternoon.prayNowDate)
        val late = aboutNow(today, 1)
        assertEquals("", late.current)
        assertEquals("compline", late.prayNowHour)
        assertEquals(CivilDate(2026, 11, 12), late.prayNowDate)
        assertEquals("invitation", view.blocks.last().kind)
    }

    @Test
    fun aNewcomerIsOfferedTheIntroductionForAWeek() {
        assertTrue(aboutNewcomer(0))
        assertTrue(aboutNewcomer(6))
        assertFalse(aboutNewcomer(7))
        assertFalse(aboutNewcomer(-1))
    }
}
