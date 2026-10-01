package org.orthodoxwest.office

import android.content.SharedPreferences
import android.os.Build
import java.net.HttpURLConnection
import java.net.URL
import java.time.Clock
import java.time.LocalDate
import java.time.ZoneId
import java.util.UUID
import java.util.concurrent.Executor
import java.util.concurrent.Executors
import org.orthodoxwest.office.core.UsageClient
import org.orthodoxwest.office.core.UsageEvent
import org.orthodoxwest.office.core.usageBeacon
import org.orthodoxwest.office.core.usageEndpoint

/**
 * The daily usage counts, reported as the web reports them (README, "Usage metrics"): the page
 * shown, the appearance and prayer form it is read in, once a day. The beacon's words come from
 * the Rust core, so the server reads the app's as it reads the web's, and only today's pages
 * (a day either side) count.
 *
 * The server knows a reader only by an identifier it hashes with the day. The app makes a fresh
 * random one each reporting day (America/New_York, as the server reckons days), so nothing it
 * sends ties one day to the next. Best effort: nothing is queued while offline, retried, or
 * shown to the reader. Only release and preview builds report (`BuildConfig.COUNT_USAGE`);
 * debug builds, and so the tests, never do.
 */
class Usage(
    private val prefs: SharedPreferences,
    private val enabled: Boolean = BuildConfig.COUNT_USAGE,
    private val clock: Clock = Clock.systemDefaultZone(),
    private val executor: Executor = sender,
    private val post: (id: String, body: String) -> Boolean = ::send,
) {
    /** Beacons already counted today by this process, as "day body". */
    private val sent = mutableSetOf<String>()

    fun record(event: UsageEvent, dark: Boolean, form: String) {
        if (!enabled) return
        val body = usageBeacon(event, LocalDate.now(clock).toCivil(), dark, form, UsageClient.ANDROID) ?: return
        val day = LocalDate.now(clock.withZone(REPORTING)).toString()
        val key = "$day $body"
        synchronized(sent) {
            if (!sent.add(key)) return
        }
        val id = idFor(day)
        executor.execute {
            // A failure is forgotten, so the next visit tries again.
            if (!runCatching { post(id, body) }.getOrDefault(false)) synchronized(sent) { sent.remove(key) }
        }
    }

    /** The reporting day's identifier: kept all day, replaced the next. */
    private fun idFor(day: String): String = synchronized(prefs) {
        prefs.getString(ID, null)?.takeIf { prefs.getString(DAY, null) == day }
            ?: UUID.randomUUID().toString().replace("-", "").also { prefs.edit().putString(DAY, day).putString(ID, it).apply() }
    }

    companion object {
        private const val DAY = "usage-day"
        private const val ID = "usage-id"
        private val REPORTING: ZoneId = ZoneId.of("America/New_York")

        /** One beacon at a time, off the main thread. */
        private val sender: Executor = Executors.newSingleThreadExecutor { Thread(it, "usage").apply { isDaemon = true } }

        private val agent = "DivineOffice/${BuildConfig.VERSION_NAME} (Android ${Build.VERSION.RELEASE})"

        /**
         * Posts one beacon as the web's page does: the header that marks it as a beacon, and the
         * day's identifier as the cookie the server would otherwise have set.
         */
        private fun send(id: String, body: String): Boolean {
            val connection = URL(usageEndpoint()).openConnection() as HttpURLConnection
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
                connection.responseCode in 200..299
            } finally {
                connection.disconnect()
            }
        }
    }
}
