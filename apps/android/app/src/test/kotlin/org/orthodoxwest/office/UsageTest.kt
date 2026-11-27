package org.orthodoxwest.office

import android.app.Application
import android.content.Context
import android.provider.Settings
import androidx.test.core.app.ApplicationProvider
import java.time.Clock
import java.time.Instant
import java.time.ZoneId
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.CivilDate
import org.orthodoxwest.office.core.UsageEvent
import org.orthodoxwest.office.core.usageEndpoint
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The daily usage beacon: what counts, once a day, under an identifier that lasts the day. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class UsageTest {
    private val app: Application = ApplicationProvider.getApplicationContext()
    private val prefs = app.getSharedPreferences("usage-test", Context.MODE_PRIVATE)
    private val posted = mutableListOf<Pair<String, String>>()
    private val endpoints = mutableListOf<String>()
    private var online = true
    private var advertised: String? = null

    // A reader in California, early on 15 March 2026.
    private var now = Instant.parse("2026-03-15T15:00:00Z")
    private val california = ZoneId.of("America/Los_Angeles")
    private val clock = object : Clock() {
        override fun getZone(): ZoneId = california
        override fun withZone(zone: ZoneId): Clock = Clock.fixed(now, zone)
        override fun instant(): Instant = now
    }

    private fun usage() = Usage(prefs, enabled = true, clock = clock, executor = { it.run() }) { endpoint, id, body ->
        if (online) {
            posted += id to body
            endpoints += endpoint
        }
        Usage.Reply(online, advertised.takeIf { online })
    }

    private val today = CivilDate(2026, 3, 15)

    @Before
    fun clear() {
        prefs.edit().clear().commit()
    }

    @Test
    fun eachPageCountsOnceADay() {
        val usage = usage()
        usage.record(UsageEvent.Home(today), dark = false, form = "private")
        usage.record(UsageEvent.Home(today), dark = false, form = "private")
        usage.record(UsageEvent.Hour(today, "vespers"), dark = true, form = "priest")
        usage.record(UsageEvent.RemindersOn, dark = true, form = "priest")
        assertEquals(
            listOf(
                "site appearance:nave screen:mobile visit:first client:android week:first month:first version:2 tenure:week",
                "vespers appearance:apse screen:mobile prayer-form:priest visit:first client:android week:first month:first version:2 tenure:week",
                "reminders appearance:apse screen:mobile visit:first client:android week:first month:first version:2 tenure:week",
            ),
            posted.map { it.second },
        )
        // One identifier all day, in the form the server accepts.
        assertEquals(1, posted.map { it.first }.toSet().size)
        assertTrue(posted[0].first.matches(Regex("[0-9a-f]{32}")))
    }

    @Test
    fun anHourWithAScoreSaysWhetherItsNotationWasShown() {
        val usage = usage()
        usage.record(UsageEvent.Hour(today, "compline"), dark = false, form = "private", chant = false)
        usage.record(UsageEvent.Hour(today, "compline"), dark = false, form = "private", chant = true)
        usage.record(UsageEvent.Home(today), dark = false, form = "private", chant = true)
        assertEquals(
            listOf(
                "compline appearance:nave screen:mobile prayer-form:private chant:off visit:first client:android week:first month:first version:2 tenure:week",
                "compline appearance:nave screen:mobile prayer-form:private chant:on visit:first client:android week:first month:first version:2 tenure:week",
                "site appearance:nave screen:mobile visit:first client:android week:first month:first version:2 tenure:week",
            ),
            posted.map { it.second },
        )
    }

    @Test
    fun theArchiveIsNotCounted() {
        val usage = usage()
        usage.record(UsageEvent.Hour(CivilDate(2019, 3, 4), "lauds"), dark = false, form = "private")
        usage.record(UsageEvent.Ordo(2031), dark = false, form = "private")
        usage.record(UsageEvent.Hour(CivilDate(2026, 3, 14), "compline"), dark = false, form = "private")
        assertEquals(listOf("compline"), posted.map { it.second.substringBefore(' ') })
    }

    @Test
    fun theIdentifierLastsOneReportingDay() {
        usage().record(UsageEvent.Home(today), dark = false, form = "private")
        // 9:30 in the evening in California is already the next day in New York: a new
        // identifier, and home counts again, though the reader's own date has not turned.
        now = Instant.parse("2026-03-16T04:30:00Z")
        val later = usage()
        later.record(UsageEvent.Home(today), dark = false, form = "private")
        later.record(UsageEvent.Hour(today, "compline"), dark = false, form = "private")
        assertEquals(3, posted.size)
        assertNotEquals(posted[0].first, posted[1].first)
        assertEquals(posted[1].first, posted[2].first)
    }

    @Test
    fun anInstallationIsNewOnItsFirstDayOnly() {
        usage().record(UsageEvent.Home(today), dark = false, form = "private")
        now = Instant.parse("2026-03-16T15:00:00Z")
        usage().record(UsageEvent.Home(CivilDate(2026, 3, 16)), dark = false, form = "private")
        assertEquals(listOf("visit:first", "visit:returning"), posted.map { Regex("visit:\\w+").find(it.second)!!.value })
    }

    // Sunday 15 March 2026 closes a week; Monday opens the next in the same month.
    @Test
    fun aWeekAndAMonthOpenOnTheirFirstCountedDay() {
        fun on(instant: String, date: CivilDate) {
            now = Instant.parse(instant)
            val usage = usage()
            usage.record(UsageEvent.Home(date), dark = false, form = "private")
            usage.record(UsageEvent.Hour(date, "lauds"), dark = false, form = "private")
        }
        on("2026-03-15T15:00:00Z", today)
        on("2026-03-16T15:00:00Z", CivilDate(2026, 3, 16))
        on("2026-03-18T15:00:00Z", CivilDate(2026, 3, 18))
        on("2026-04-01T15:00:00Z", CivilDate(2026, 4, 1))
        assertEquals(
            listOf("week:first month:first", "week:first month:again", "week:again month:again", "week:first month:first")
                .flatMap { listOf(it, it) },
            posted.map { Regex("week:\\w+ month:\\w+").find(it.second)!!.value },
        )
    }

    // A beacon that never arrived leaves the week unopened: the next day's is still its first.
    @Test
    fun aWeekOpensOnlyWhenABeaconIsCounted() {
        online = false
        usage().record(UsageEvent.Home(today), dark = false, form = "private")
        online = true
        now = Instant.parse("2026-03-17T15:00:00Z")
        usage().record(UsageEvent.Home(CivilDate(2026, 3, 17)), dark = false, form = "private")
        now = Instant.parse("2026-03-18T15:00:00Z")
        usage().record(UsageEvent.Home(CivilDate(2026, 3, 18)), dark = false, form = "private")
        assertEquals(listOf("week:first month:first", "week:again month:again"), posted.map { Regex("week:\\w+ month:\\w+").find(it.second)!!.value })
    }

    // How long it has been counted, last week's days on the new week's first, an hour at its own
    // time by the phone's clock and prayed, and the reminders: as the core writes them.
    @Test
    fun anInstallationSaysHowItKeepsTheHours() {
        usage().record(UsageEvent.Home(today), dark = false, form = "private")
        // Monday at 8 in the morning in California: Prime's time.
        now = Instant.parse("2026-03-16T15:00:00Z")
        val monday = CivilDate(2026, 3, 16)
        val usage = usage()
        usage.record(UsageEvent.Hour(monday, "prime"), dark = false, form = "private", prayed = true, reminders = true)
        usage.record(UsageEvent.Hour(monday, "vespers"), dark = false, form = "private", reminders = false)
        assertEquals(
            listOf(
                "prime appearance:nave screen:mobile prayer-form:private visit:returning client:android week:first month:again version:2 tenure:week last-week:1 timely:prime prayed:prime reminder:on",
                "vespers appearance:nave screen:mobile prayer-form:private visit:returning client:android week:first month:again version:2 tenure:week reminder:off",
            ),
            posted.drop(1).map { it.second },
        )
    }

    // An hour whose beacon did not arrive is said on the next that does, and then no more.
    @Test
    fun anHourThatCouldNotReportIsSaidNextTime() {
        val usage = usage()
        online = false
        usage.record(UsageEvent.Hour(today, "prime"), dark = false, form = "private")
        online = true
        usage.record(UsageEvent.Home(today), dark = false, form = "private")
        usage.record(UsageEvent.Hour(today, "vespers"), dark = false, form = "private")
        assertEquals(listOf(true, false), posted.map { " offline:yes" in it.second })
    }

    @Test
    fun anInstallationThatReportedBeforeIsReturning() {
        // An app updated from a build that counted readers but not new ones.
        prefs.edit().putString("usage-day", "2026-03-10").putString("usage-id", "0".repeat(32)).commit()
        usage().record(UsageEvent.Home(today), dark = false, form = "private")
        assertTrue(posted.single().second.contains("visit:returning"))
    }

    /** Home's first-week introduction counts from the day Usage keeps, whether or not it reports. */
    @Test
    fun daysAreCountedFromTheFirst() {
        val quiet = Usage(prefs, enabled = false, clock = clock, executor = { it.run() }) { _, _, _ -> Usage.Reply(true) }
        assertEquals(0, quiet.daysSinceFirst())
        now = Instant.parse("2026-03-21T15:00:00Z")
        assertEquals(6, quiet.daysSinceFirst())
        // The introduction's day is not the beacon's: a reader whose first hours went uncounted
        // is still new on the day their first beacon goes out.
        usage().record(UsageEvent.Home(CivilDate(2026, 3, 21)), dark = false, form = "private")
        assertTrue(posted.single().second.contains("visit:first"))
        assertEquals(6, quiet.daysSinceFirst())
    }

    @Test
    fun anInstallationThatReportedBeforeIsNoNewcomer() {
        prefs.edit().putString("usage-day", "2026-03-10").putString("usage-id", "0".repeat(32)).commit()
        assertEquals(-1, usage().daysSinceFirst())
    }

    @Test
    fun theSiteIsTheEndpointsHost() {
        assertEquals(usageEndpoint().removeSuffix("/api/usage"), Usage.site(prefs))
        advertised = "https://example.org/api/usage"
        usage().record(UsageEvent.Home(today), dark = false, form = "private")
        assertEquals("https://example.org", Usage.site(prefs))
    }

    @Test
    fun beaconsFollowTheSiteToTheAddressItNames() {
        advertised = "https://example.org/api/usage"
        val usage = usage()
        usage.record(UsageEvent.Home(today), dark = false, form = "private")
        usage.record(UsageEvent.Hour(today, "lauds"), dark = false, form = "private")
        // Anything but a plain HTTPS usage endpoint is ignored.
        advertised = "http://elsewhere.example/api/usage"
        usage.record(UsageEvent.Hour(today, "vespers"), dark = false, form = "private")
        usage().record(UsageEvent.Hour(today, "compline"), dark = false, form = "private")
        assertEquals(
            listOf(usageEndpoint(), "https://example.org/api/usage", "https://example.org/api/usage", "https://example.org/api/usage"),
            endpoints,
        )
    }

    @Test
    fun aFailedBeaconIsTriedAgainOnTheNextVisit() {
        val usage = usage()
        online = false
        usage.record(UsageEvent.Home(today), dark = false, form = "private")
        online = true
        usage.record(UsageEvent.Home(today), dark = false, form = "private")
        assertEquals(1, posted.size)
    }

    @Test
    fun debugBuildsNeverReport() {
        assertFalse(BuildConfig.COUNT_USAGE)
        Usage(prefs, clock = clock, executor = { it.run() }) { _, id, body -> posted += id to body; Usage.Reply(true) }
            .record(UsageEvent.Home(today), dark = false, form = "private")
        assertTrue(posted.isEmpty())
    }

    // Play's pre-launch report crawls every upload on fresh test devices; none is a reader.
    @Test
    fun testDevicesAreNotCounted() {
        assertTrue(Usage.counts(app, build = true))
        assertFalse(Usage.counts(app, build = false))
        Settings.System.putString(app.contentResolver, "firebase.test.lab", "true")
        try {
            assertFalse(Usage.counts(app, build = true))
        } finally {
            Settings.System.putString(app.contentResolver, "firebase.test.lab", "false")
        }
    }

    // A test phone is wiped minutes after the install; a reader's later pages count, as New.
    @Test
    fun aFreshInstallationWaitsBeforeReporting() {
        val installed = now.minusSeconds(600)
        val usage = Usage(prefs, enabled = true, clock = clock, installed = installed, executor = { it.run() }) { _, id, body ->
            posted += id to body
            Usage.Reply(true)
        }
        usage.record(UsageEvent.Home(today), dark = false, form = "private")
        assertTrue(posted.isEmpty())
        now = now.plus(Usage.SETTLING)
        usage.record(UsageEvent.Home(today), dark = false, form = "private")
        assertEquals(listOf("site appearance:nave screen:mobile visit:first client:android week:first month:first version:2 tenure:week"), posted.map { it.second })
    }
}
