package org.orthodoxwest.office

import android.app.Application
import android.content.Context
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
                "site appearance:nave screen:mobile visit:first client:android",
                "vespers appearance:apse screen:mobile prayer-form:priest visit:first client:android",
                "reminders appearance:apse screen:mobile visit:first client:android",
            ),
            posted.map { it.second },
        )
        // One identifier all day, in the form the server accepts.
        assertEquals(1, posted.map { it.first }.toSet().size)
        assertTrue(posted[0].first.matches(Regex("[0-9a-f]{32}")))
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
        // The first beacon reports the day already kept.
        usage().record(UsageEvent.Home(CivilDate(2026, 3, 21)), dark = false, form = "private")
        assertTrue(posted.single().second.contains("visit:returning"))
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
}
