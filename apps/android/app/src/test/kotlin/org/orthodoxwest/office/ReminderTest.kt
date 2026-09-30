package org.orthodoxwest.office

import android.Manifest
import android.app.AlarmManager
import android.app.Application
import android.app.NotificationManager
import android.content.Intent
import androidx.test.core.app.ApplicationProvider
import java.time.DayOfWeek
import java.time.Instant
import java.time.ZoneId
import java.time.ZonedDateTime
import org.junit.Assert.assertEquals
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

    @Before
    fun allow() {
        ShadowAlarmManager.setCanScheduleExactAlarms(true)
        shadowOf(app).grantPermissions(Manifest.permission.POST_NOTIFICATIONS)
    }

    private fun turn(on: Boolean) {
        val store = ReminderStore(app)
        store.save(store.load().copy(on = on))
    }

    @Test
    fun nothingIsScheduledUntilRemindersAreOn() {
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 5, 0, 0, 0, zone))
        assertEquals(0, alarms.scheduledAlarms.size)
    }

    private fun times() = alarms.scheduledAlarms.map { Instant.ofEpochMilli(it.triggerAtTime).atZone(zone).toLocalDateTime().toString() }.sorted()

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
    fun aFiredReminderNamesTheOfficeAndTheDayAndOpensTheHour() {
        turn(on = true)
        ReminderScheduler.sync(app, ZonedDateTime.of(2026, 3, 15, 5, 0, 0, 0, zone))
        val fired = alarms.scheduledAlarms.minBy { it.triggerAtTime }
        val intent: Intent = shadowOf(fired.operation).savedIntent
        Notifications.post(app, intent)
        val posted = shadowOf(app.getSystemService(NotificationManager::class.java)).allNotifications.single()
        assertEquals("Lauds", posted.extras.getString("android.title"))
        assertEquals("III Sunday in Lent", posted.extras.getString("android.text"))
        val open = shadowOf(posted.contentIntent).savedIntent
        assertEquals("lauds", open.getStringExtra(EXTRA_HOUR))
        assertEquals("2026-03-15", open.getStringExtra(EXTRA_DATE))
    }
}
