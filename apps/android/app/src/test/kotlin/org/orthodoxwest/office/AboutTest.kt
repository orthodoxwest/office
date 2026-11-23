package org.orthodoxwest.office

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.orthodoxwest.office.core.CivilDate
import org.orthodoxwest.office.core.about
import org.orthodoxwest.office.core.aboutNewcomer
import org.orthodoxwest.office.core.aboutNow
import org.orthodoxwest.office.core.aboutTimeSpoken
import org.orthodoxwest.office.core.dayNameGloss
import org.orthodoxwest.office.core.hourNames
import org.orthodoxwest.office.core.ordoColors
import java.time.LocalTime
import java.time.format.DateTimeFormatter
import java.util.Locale

/** About the Office as the core gives it: the page AboutScreen sets, and the first week home offers it in. */
class AboutTest {
    private val view = about()

    @Test
    fun thePageHasItsTitleAndTheSevenHours() {
        assertTrue(view.kicker.isNotBlank())
        assertTrue(view.title.isNotBlank())
        assertEquals(hourNames(), view.periods.flatMap { p -> p.hours.map { it.hour } })
        val hours = view.periods.flatMap { it.hours }
        assertTrue(hours.all { it.name.isNotBlank() && it.gloss.isNotBlank() && it.time.isNotBlank() })
        assertTrue(hours.zipWithNext().all { (a, b) -> a.angle > b.angle })
        assertEquals(1, view.blocks.count { it.kind == "hours" })
        assertEquals(1, view.blocks.count { it.kind == "specimen" })
        assertEquals(1, view.blocks.count { it.kind == "ordo_ribbon" })
        assertEquals(6, view.ordoColors.size)
        assertEquals(6, ordoColors().size)
        assertTrue(view.specimen.rubric.isNotBlank())
        assertTrue(view.specimen.deaconNote.isNotBlank())
        assertTrue(view.specimen.priestNote.isNotBlank())
        assertEquals("Feria", dayNameGloss("Feria")?.term)
    }

    /** AboutScreen opens the ordo and the reminders itself, and leaves the rest to the browser. */
    @Test
    fun linksGoOnlyWhereTheAppCanFollow() {
        val links = view.blocks.flatMap { it.runs }.map { it.link }.filter { it.isNotEmpty() }
        assertTrue(links.isNotEmpty())
        for (link in links) assertTrue(link, link in listOf("/calendar", "/reminders", "/privacy") || link.startsWith("https://"))
    }

    /** While the hours follow the sun, the table gives today's sun's times, and Compline keeps its words. */
    @Test
    fun theSunTimesTheHoursButCompline() {
        val clock = DateTimeFormatter.ofPattern("h:mm a", Locale.US)
        assertEquals(view.periods, sunPeriods(view.periods, null, clock))
        val sun = mapOf(
            "lauds" to LocalTime.of(6, 48), "prime" to LocalTime.of(7, 43), "terce" to LocalTime.of(9, 31),
            "sext" to LocalTime.of(12, 14), "none" to LocalTime.of(14, 15), "vespers" to LocalTime.of(17, 19),
            "compline" to LocalTime.of(21, 0),
        )
        val times = sunPeriods(view.periods, sun, clock).flatMap { it.hours }.associate { it.hour to it.time }
        assertEquals("sunrise · 6:48 am", times["lauds"])
        assertEquals("first hour · 7:43 am", times["prime"])
        assertEquals("third hour · 9:31 am", times["terce"])
        assertEquals("solar noon · 12:14 pm", times["sext"])
        assertEquals("ninth hour · 2:15 pm", times["none"])
        assertEquals("sunset · 5:19 pm", times["vespers"])
        assertEquals("sunrise, 6:48 am", aboutTimeSpoken(times.getValue("lauds")))
        assertEquals(view.periods.last().hours.last().time, times["compline"])
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
