package org.orthodoxwest.office

import android.content.Context
import android.content.SharedPreferences
import android.os.Build
import android.provider.Settings
import java.net.HttpURLConnection
import java.net.URL
import java.time.Clock
import java.time.Duration
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.temporal.ChronoUnit
import java.util.UUID
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import org.orthodoxwest.office.core.UsageClient
import org.orthodoxwest.office.core.UsageEvent
import org.orthodoxwest.office.core.UsageHabits
import org.orthodoxwest.office.core.usageAdvertisedEndpoint
import org.orthodoxwest.office.core.usageBeacon
import org.orthodoxwest.office.core.usageEndpoint
import org.orthodoxwest.office.core.usageEndpointHeader
import org.orthodoxwest.office.core.usagePeriods
import org.orthodoxwest.office.core.usageTally

/**
 * The daily usage counts, reported as the web reports them (DEPLOYMENT.md, "Usage metrics"):
 * the page shown, the appearance and prayer form it is read in, at Prime whether the
 * Martyrology was shown, and on an hour with a score whether its notation was, once a day. The
 * beacon's words come from the Rust core, so the server reads the app's as it reads the web's,
 * and only today's pages (a day either side) count.
 *
 * The server knows a reader only by an identifier it hashes with the day. The app makes a fresh
 * random one each reporting day (America/New_York, as the server reckons days), so nothing it
 * sends ties one day to the next. The app also keeps the day it was first counted, and says
 * only whether today is that day (new) or not (returning); and the last two days a beacon of
 * its own was counted, and says only whether today is the first in its week and in its month,
 * so the server can add up a week's readers. Beside those it says, in buckets and yes-or-no words
 * the core writes (`UsageHabits`): how long ago it was first counted, on the week's first day how
 * many days it was counted the week before (it keeps a tally of the week's days), an hour opened
 * at its own time by the phone's clock, an hour prayed (home's mark), an hour opened that could
 * not report since a beacon last arrived, and whether reminders are on. Best effort: nothing is queued while
 * offline, retried, or shown to the reader. Only release and preview builds report
 * (`BuildConfig.COUNT_USAGE`); debug builds, and so the tests, never do. Nor does a phone in
 * Play's pre-launch report, which installs every uploaded build fresh on several test devices and
 * crawls it; each would count as a new reader. Nothing on those phones reliably says so (not all
 * carry Test Lab's setting, and the Play Store installs the build), but each is wiped within
 * minutes of the install, so an installation reports only once it is [SETTLING] old.
 *
 * Beacons go to the production server until a reply names the site's new address
 * (`usageEndpointHeader`); the app keeps that and posts there from then on.
 */
class Usage(
    private val prefs: SharedPreferences,
    private val enabled: Boolean = BuildConfig.COUNT_USAGE,
    private val clock: Clock = Clock.systemDefaultZone(),
    /** When this installation was first installed; null counts at once. */
    private val installed: Instant? = null,
    private val executor: Executor = sender,
    private val post: (endpoint: String, id: String, body: String) -> Reply = ::send,
) {
    /** A beacon's outcome: whether it counted, and the endpoint the server named, if any. */
    data class Reply(val ok: Boolean, val endpoint: String? = null)

    /** Beacons already counted today by this process, as "day body". */
    private val sent = mutableSetOf<String>()

    /**
     * `martyrology` is Prime's `HourView.martyrology`: whether its reading was shown, if it had one;
     * `chant` an hour's `HourView.chant`: whether its scores were shown, if it had any; `prayed`
     * whether the hour shown is marked prayed; `reminders` whether the app's reminders are on.
     */
    fun record(
        event: UsageEvent,
        dark: Boolean,
        form: String,
        martyrology: Boolean? = null,
        chant: Boolean? = null,
        prayed: Boolean = false,
        reminders: Boolean? = null,
    ) {
        if (!enabled) return
        if (installed != null && Duration.between(installed, clock.instant()) < SETTLING) return
        val day = LocalDate.now(clock.withZone(REPORTING)).toString()
        val firstDay = firstDay(day)
        val first = firstDay == day
        val (periods, habits) = synchronized(prefs) {
            usagePeriods(day, prefs.getString(LAST, null), prefs.getString(PREVIOUS, null)) to UsageHabits(
                reportingDay = day,
                firstDay = firstDay,
                tally = prefs.getString(TALLY, null),
                clockHour = LocalTime.now(clock).hour,
                prayed = prayed,
                offline = prefs.getBoolean(OFFLINE, false),
                reminders = reminders,
            )
        }
        val body = usageBeacon(
            event, LocalDate.now(clock).toCivil(), dark, form, UsageClient.ANDROID, martyrology, first, chant, periods?.week, periods?.month,
            habits,
        ) ?: return
        // Saying a failed hour is not a new page: it neither sends again nor counts apart.
        val key = "$day ${body.replace(" offline:yes", "")}"
        synchronized(sent) {
            if (!sent.add(key)) return
        }
        val id = idFor(day)
        val endpoint = synchronized(prefs) { prefs.getString(ENDPOINT, null) } ?: usageEndpoint()
        executor.execute {
            val reply = runCatching { post(endpoint, id, body) }.getOrDefault(Reply(false))
            // A failure is forgotten, so the next visit tries again. An hour whose beacon did not
            // arrive is said on the next that does.
            if (reply.ok) counted(day, body) else synchronized(sent) { sent.remove(key) }
            if (!reply.ok && event is UsageEvent.Hour) synchronized(prefs) { prefs.edit().putBoolean(OFFLINE, true).apply() }
            reply.endpoint?.let(::usageAdvertisedEndpoint)?.let { next ->
                synchronized(prefs) { prefs.edit().putString(ENDPOINT, next).apply() }
            }
        }
    }

    /**
     * Keeps `day` as the latest reporting day a beacon was counted, and the one before it: what
     * `usagePeriods` reads to say whether a day opens the week and the month. Kept only once a
     * beacon has arrived, so a day that never reported opens nothing.
     */
    private fun counted(day: String, body: String) = synchronized(prefs) {
        val edit = prefs.edit()
        if (" offline:yes" in body) edit.remove(OFFLINE)
        val last = prefs.getString(LAST, null)
        if (last != day) edit.putString(PREVIOUS, last).putString(LAST, day).putString(TALLY, usageTally(prefs.getString(TALLY, null), day))
        edit.apply()
    }

    /**
     * The reporting day the app was first counted. An installation that reported before this
     * was kept (it holds a day's identifier) is from an earlier day.
     */
    private fun firstDay(day: String): String = synchronized(prefs) {
        prefs.getString(FIRST, null)
            ?: (if (prefs.contains(DAY)) BEFORE else day).also { prefs.edit().putString(FIRST, it).apply() }
    }

    /**
     * Days since the reporting day the app was first opened, for home's first-week introduction
     * (`aboutNewcomer`), or -1 for an installation from before first days were kept. The day is
     * kept apart from the beacon's first day, which waits for the install to settle: writing that
     * one here would count a reader who prays in their first hours as returning the next day.
     * The day stays on the phone and is never sent.
     */
    fun daysSinceFirst(): Int {
        val today = LocalDate.now(clock.withZone(REPORTING))
        val first = synchronized(prefs) {
            prefs.getString(OPENED, null) ?: prefs.getString(FIRST, null)
                ?: (if (prefs.contains(DAY)) BEFORE else today.toString()).also { prefs.edit().putString(OPENED, it).apply() }
        }
        if (first == BEFORE) return -1
        return runCatching { ChronoUnit.DAYS.between(LocalDate.parse(first), today).toInt() }.getOrDefault(-1)
    }

    /** The reporting day's identifier: kept all day, replaced the next. */
    private fun idFor(day: String): String = synchronized(prefs) {
        prefs.getString(ID, null)?.takeIf { prefs.getString(DAY, null) == day }
            ?: UUID.randomUUID().toString().replace("-", "").also { prefs.edit().putString(DAY, day).putString(ID, it).apply() }
    }

    companion object {
        private const val DAY = "usage-day"
        private const val ID = "usage-id"
        private const val FIRST = "usage-first"
        private const val LAST = "usage-counted"
        private const val PREVIOUS = "usage-counted-before"
        private const val TALLY = "usage-week-days"
        private const val OFFLINE = "usage-offline"
        private const val BEFORE = "before"
        private const val OPENED = "about-first"
        private const val ENDPOINT = "usage-endpoint"
        private val REPORTING: ZoneId = ZoneId.of("America/New_York")
        private const val USAGE_PATH = "/api/usage"

        /**
         * The site's address, for the pages the app leaves to it (About's Privacy): the usage
         * endpoint's host, which follows the site to the address a reply names.
         */
        fun site(prefs: SharedPreferences): String =
            (synchronized(prefs) { prefs.getString(ENDPOINT, null) } ?: usageEndpoint()).removeSuffix(USAGE_PATH)

        /** One beacon at a time, off the main thread. */
        private val sender: Executor = Executors.newSingleThreadExecutor { Thread(it, "usage").apply { isDaemon = true } }

        /**
         * How old an installation must be before it reports. Play's pre-launch test phones are
         * wiped within minutes of installing a build; a reader's first hours on a new install go
         * uncounted, and every page after them counts as usual (as New, on the first such day).
         * An update keeps the first install time, so testers who update keep counting.
         */
        val SETTLING: Duration = Duration.ofHours(2)

        /**
         * Whether this build on this device reports: not a debug build, nor a device carrying the
         * system setting Test Lab documents
         * (firebase.google.com/docs/test-lab/android/android-studio#modify_instrumented_test_behavior_for).
         */
        fun counts(context: Context, build: Boolean = BuildConfig.COUNT_USAGE): Boolean =
            build && Settings.System.getString(context.contentResolver, TEST_LAB) != "true"

        /** When this app was first installed on the device, if Android says. */
        fun installedAt(context: Context): Instant? = runCatching {
            Instant.ofEpochMilli(context.packageManager.getPackageInfo(context.packageName, 0).firstInstallTime)
        }.getOrNull()

        private const val TEST_LAB = "firebase.test.lab"

        private val agent = "DivineOffice/${BuildConfig.VERSION_NAME} (Android ${Build.VERSION.RELEASE})"

        /**
         * Posts one beacon as the web's page does: the header that marks it as a beacon, and the
         * day's identifier as the cookie the server would otherwise have set.
         */
        private fun send(endpoint: String, id: String, body: String): Reply {
            val connection = URL(endpoint).openConnection() as HttpURLConnection
            return try {
                connection.requestMethod = "POST"
                connection.connectTimeout = 4000
                connection.readTimeout = 4000
                connection.useCaches = false
                connection.instanceFollowRedirects = false
                connection.doOutput = true
                connection.setRequestProperty("X-Office-Usage", "1")
                connection.setRequestProperty("Content-Type", "text/plain")
                connection.setRequestProperty("Cookie", "office-usage=$id")
                connection.setRequestProperty("User-Agent", agent)
                connection.outputStream.use { it.write(body.toByteArray()) }
                Reply(connection.responseCode in 200..299, connection.getHeaderField(usageEndpointHeader()))
            } finally {
                connection.disconnect()
            }
        }
    }
}
