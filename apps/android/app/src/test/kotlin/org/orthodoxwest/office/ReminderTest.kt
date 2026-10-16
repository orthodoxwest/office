package org.orthodoxwest.office

import android.Manifest
import android.app.AlarmManager
import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import androidx.test.core.app.ApplicationProvider
import java.time.DayOfWeek
import java.time.Duration
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.ZonedDateTime
import java.util.TimeZone
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.shadows.ShadowAlarmManager

/** The schedule the alarm service holds, and what a fired reminder shows. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class ReminderTest {
    private val app: Application = ApplicationProvider.getApplicationContext()
    private val zone = ZoneId.of("America/New_York")
    private val alarms get() = shadowOf(app.getSystemService(AlarmManager::class.java))

    private val systemZone = TimeZone.getDefault()

    @After
    fun restoreZone() = TimeZone.setDefault(systemZone)

    @Before
    fun allow() {
        // The notification's clock time is the phone's.
        TimeZone.setDefault(TimeZone.getTimeZone(zone))
        ShadowAlarmManager.setCanScheduleExactAlarms(true)
        shadowOf(app).grantPermissions(Manifest.permission.POST_NOTIFICATIONS)
    }

    /** What the alarm will deliver. Robolectric offers no replacement for the deprecated field. */
    @Suppress("DEPRECATION")
    private val ShadowAlarmManager.ScheduledAlarm.intent: Intent get() = shadowOf(operation).savedIntent

    private fun turn(on: Boolean) {
        val store = ReminderStore(app)
        store.save(store.load().copy(on = on))
    }

    @Test
    fun nothingIsScheduledUntilRemindersAreOn() {
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 5, 0, 0, 0, zone))
        assertEquals(0, alarms.scheduledAlarms.size)
    }

    private fun times() = alarms.scheduledAlarms.map { Instant.ofEpochMilli(it.triggerAtMs).atZone(zone).toLocalDateTime().toString() }.sorted()

    @Test
    fun theWebsDefaultHoursAreKeptAWeekAhead() {
        turn(on = true)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 5, 0, 0, 0, zone))
        // Lauds, Vespers and Compline from the 15th through the 22nd, ten minutes early.
        val times = times()
        assertEquals(24, times.size)
        assertEquals("2026-03-15T06:35", times.first())
        assertTrue(times.contains("2026-03-15T17:50"))
        assertEquals("2026-03-22T20:50", times.last())
    }

    @Test
    fun aDueReminderIsNotRescheduledAndTurningOffCancelsTheRest() {
        turn(on = true)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 18, 0, 0, 0, zone))
        // Lauds and Vespers of the 15th are past; the day's Compline and seven more days remain.
        assertEquals(22, alarms.scheduledAlarms.size)
        turn(on = false)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 18, 0, 0, 0, zone))
        assertEquals(0, alarms.scheduledAlarms.size)
    }

    @Test
    fun aFiredReminderSyncsOnlyOnceTheWindowIsNearlySpent() {
        turn(on = true)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 5, 0, 0, 0, zone))
        // The last is Compline on the 22nd: a week of firings wakes no engine, then the next refills.
        assertEquals(false, ReminderScheduler.due(app, ZonedDateTime.of(2026, 3, 15, 6, 35, 0, 0, zone)))
        assertEquals(false, ReminderScheduler.due(app, ZonedDateTime.of(2026, 3, 21, 20, 50, 0, 0, zone)))
        assertEquals(true, ReminderScheduler.due(app, ZonedDateTime.of(2026, 3, 22, 6, 35, 0, 0, zone)))
    }

    @Test
    fun aWeeklyReminderHasItsNextWaitingWhenItFires() {
        val store = ReminderStore(app)
        store.save(store.load().copy(on = true, days = setOf(DayOfWeek.SUNDAY)))
        // Sunday's Vespers fires, and the sync that follows keeps the next Sunday's.
        val vespers = ZonedDateTime.of(2026, 3, 15, 17, 50, 0, 0, zone)
        assertTrue(ReminderScheduler.due(app, vespers))
        ReminderScheduler.sync(app, vespers)
        assertEquals(listOf("2026-03-15T20:50", "2026-03-22T06:35", "2026-03-22T17:50", "2026-03-22T20:50"), times())
    }

    @Test
    fun remindersRingTheBellOnTheirOwnChannel() {
        val manager = app.getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(NotificationChannel("hours", "Hours of prayer", NotificationManager.IMPORTANCE_DEFAULT))
        Notifications.ensureChannel(app)
        assertEquals(Notifications.bell(app), manager.getNotificationChannel(Notifications.CHANNEL).sound)
        // The first channel, which rang the phone's default sound, is gone.
        assertNull(manager.getNotificationChannel("hours"))
    }

    @Test
    fun inTenMinutesPutsTheReminderAwayAndRingsAgain() {
        turn(on = true)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 5, 0, 0, 0, zone))
        Notifications.post(app, alarms.scheduledAlarms.minBy { it.triggerAtMs }.intent)
        val notifications = shadowOf(app.getSystemService(NotificationManager::class.java))
        val later = notifications.allNotifications.single().actions.single()
        assertEquals("Remind me in 10 min", later.title)
        val snooze = shadowOf(later.actionIntent).savedIntent
        assertEquals(ReminderReceiver.SNOOZE, snooze.action)
        val before = alarms.scheduledAlarms.size
        ReminderScheduler.snooze(app, snooze, ZonedDateTime.of(2026, 3, 15, 6, 36, 0, 0, zone))
        assertEquals(0, notifications.allNotifications.size)
        assertEquals(before + 1, alarms.scheduledAlarms.size)
        val again = alarms.scheduledAlarms.single { Instant.ofEpochMilli(it.triggerAtMs).atZone(zone).toLocalDateTime().toString() == "2026-03-15T06:46" }
        // The same words, ready to post again.
        Notifications.post(app, again.intent)
        assertEquals("Lauds · 6:45 AM", notifications.allNotifications.single().extras.getString("android.title"))
    }

    @Test
    fun aFiredReminderNamesTheOfficeAndTheDayAndOpensTheHour() {
        turn(on = true)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 5, 0, 0, 0, zone))
        val fired = alarms.scheduledAlarms.minBy { it.triggerAtMs }
        val intent: Intent = fired.intent
        Notifications.post(app, intent)
        val posted = shadowOf(app.getSystemService(NotificationManager::class.java)).allNotifications.single()
        assertEquals("Lauds · 6:45 AM", posted.extras.getString("android.title"))
        assertEquals("III Sunday in Lent", posted.extras.getString("android.text"))
        val open = shadowOf(posted.contentIntent).savedIntent
        assertEquals("lauds", open.getStringExtra(EXTRA_HOUR))
        assertEquals("2026-03-15", open.getStringExtra(EXTRA_DATE))
    }

    @Test
    fun aReminderShowsTheClockTimeAndClearsItselfOnceStale() {
        turn(on = true)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 5, 0, 0, 0, zone))
        val intent = alarms.scheduledAlarms.minBy { it.triggerAtMs }.intent
        // Fired on time, ten minutes before Lauds: it stays until an hour after Lauds begins.
        Notifications.post(app, intent, ZonedDateTime.of(2026, 3, 15, 6, 35, 0, 0, zone).toInstant())
        val posted = shadowOf(app.getSystemService(NotificationManager::class.java)).allNotifications.single()
        assertEquals(false, posted.extras.getBoolean("android.showWhen"))
        assertEquals(Duration.ofMinutes(70).toMillis(), posted.timeoutAfter)
        // Fired long after (the phone was off): still a snooze's length to see it.
        val late = Notifications.lingers(ZonedDateTime.of(2026, 3, 15, 6, 45, 0, 0, zone).toInstant(), ZonedDateTime.of(2026, 3, 15, 9, 0, 0, 0, zone).toInstant())
        assertEquals(ReminderScheduler.SNOOZE, late)
    }

    private fun followSun(latitude: Double, longitude: Double) {
        val store = ReminderStore(app)
        store.save(store.load().copy(on = true, followSun = true, place = StoredPlace(latitude, longitude, Instant.parse("2026-06-20T16:00:00Z"))))
    }

    @Test
    fun theSunTimesTheHoursAtThePlaceKept() {
        // Philadelphia at midsummer: Lauds at sunrise (5:32), Vespers at sunset (8:33), Compline
        // an hour after it rather than at nine; each ten minutes early.
        followSun(39.9526, -75.1652)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 6, 21, 3, 0, 0, 0, zone))
        val times = times()
        assertEquals(24, times.size)
        assertEquals(listOf("2026-06-21T05:22", "2026-06-21T20:23", "2026-06-21T21:23"), times.take(3))
    }

    @Test
    fun eachDayTakesItsOwnOffsetAcrossAClockChange() {
        followSun(39.9526, -75.1652)
        val settings = ReminderStore(app).load()
        // Summer time ends on 1 November 2026.
        val place = settings.sunPlace(LocalDate.of(2026, 10, 30), 4, zone)!!
        assertEquals(listOf(-240, -240, -300, -300), place.utcOffsets)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 10, 30, 3, 0, 0, 0, zone))
        // Sunrise moves an hour earlier on the clock with it.
        val lauds = times().filter { it.substring(11, 13).toInt() < 9 }
        val oct31 = lauds.single { it.startsWith("2026-10-31") }
        val nov1 = lauds.single { it.startsWith("2026-11-01") }
        val minutes = { t: String -> t.substring(11, 13).toInt() * 60 + t.substring(14, 16).toInt() }
        assertTrue(minutes(oct31) - minutes(nov1) in 55..65)
    }

    @Test
    fun pastTheLimitTheChosenTimesStand() {
        followSun(64.84, -147.72)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 6, 21, 3, 0, 0, 0, zone))
        assertEquals("2026-06-21T06:35", times().first())
        assertFalse(ReminderStore(app).load().sunActive)
    }

    @Test
    fun thePlaceIsKeptAndForgotten() {
        followSun(39.9526, -75.1652)
        val kept = ReminderStore(app).load()
        assertTrue(kept.followSun && kept.sunActive)
        assertEquals(StoredPlace(39.9526, -75.1652, Instant.parse("2026-06-20T16:00:00Z")), kept.place)
        ReminderStore(app).save(kept.copy(followSun = false, place = null))
        val off = ReminderStore(app).load()
        assertFalse(off.followSun)
        assertNull(off.place)
        assertNull(off.sunPlace(LocalDate.of(2026, 6, 21), 8, zone))
    }

    @Test
    fun theSunsTimesForTheScreen() {
        followSun(39.9526, -75.1652)
        val today = ReminderStore(app).load().sunTimes(LocalDate.of(2026, 6, 21), zone)!!
        assertEquals(LocalTime.of(5, 32), today["lauds"])
        assertEquals(LocalTime.of(13, 3), today["sext"])
        assertEquals(LocalTime.of(21, 33), today["compline"])
    }
}
