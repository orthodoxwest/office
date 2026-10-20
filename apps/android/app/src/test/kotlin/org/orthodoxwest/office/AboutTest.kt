package org.orthodoxwest.office

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.orthodoxwest.office.core.about
import org.orthodoxwest.office.core.aboutNewcomer
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

    @Test
    fun aNewcomerIsOfferedTheIntroductionForAWeek() {
        assertTrue(aboutNewcomer(0))
        assertTrue(aboutNewcomer(6))
        assertFalse(aboutNewcomer(7))
        assertFalse(aboutNewcomer(-1))
    }
}
