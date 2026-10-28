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
import java.time.ZoneId
import java.util.UUID
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import org.orthodoxwest.office.core.UsageClient
import org.orthodoxwest.office.core.UsageEvent
import org.orthodoxwest.office.core.usageAdvertisedEndpoint
import org.orthodoxwest.office.core.usageBeacon
import org.orthodoxwest.office.core.usageEndpoint
import org.orthodoxwest.office.core.usageEndpointHeader

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
 * only whether today is that day (new) or not (returning). Best effort: nothing is queued while
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
     * `chant` an hour's `HourView.chant`: whether its scores were shown, if it had any.
     */
    fun record(event: UsageEvent, dark: Boolean, form: String, martyrology: Boolean? = null, chant: Boolean? = null) {
        if (!enabled) return
        if (installed != null && Duration.between(installed, clock.instant()) < SETTLING) return
        val day = LocalDate.now(clock.withZone(REPORTING)).toString()
        val first = firstDay(day) == day
        val body = usageBeacon(event, LocalDate.now(clock).toCivil(), dark, form, UsageClient.ANDROID, martyrology, first, chant) ?: return
        val key = "$day $body"
        synchronized(sent) {
            if (!sent.add(key)) return
        }
        val id = idFor(day)
        val endpoint = synchronized(prefs) { prefs.getString(ENDPOINT, null) } ?: usageEndpoint()
        executor.execute {
            val reply = runCatching { post(endpoint, id, body) }.getOrDefault(Reply(false))
            // A failure is forgotten, so the next visit tries again.
            if (!reply.ok) synchronized(sent) { sent.remove(key) }
            reply.endpoint?.let(::usageAdvertisedEndpoint)?.let { next ->
                synchronized(prefs) { prefs.edit().putString(ENDPOINT, next).apply() }
            }
        }
    }

    /**
     * The reporting day the app was first counted. An installation that reported before this
     * was kept (it holds a day's identifier) is from an earlier day.
     */
    private fun firstDay(day: String): String = synchronized(prefs) {
        prefs.getString(FIRST, null)
            ?: (if (prefs.contains(DAY)) BEFORE else day).also { prefs.edit().putString(FIRST, it).apply() }
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
        private const val BEFORE = "before"
        private const val ENDPOINT = "usage-endpoint"
        private val REPORTING: ZoneId = ZoneId.of("America/New_York")

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
