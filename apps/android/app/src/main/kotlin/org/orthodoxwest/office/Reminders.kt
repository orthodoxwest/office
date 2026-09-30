package org.orthodoxwest.office

import android.Manifest
import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.ContentResolver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.media.AudioAttributes
import android.net.Uri
import android.os.Build
import androidx.compose.ui.graphics.toArgb
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import java.time.DayOfWeek
import java.time.Duration
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.ZonedDateTime
import org.orthodoxwest.office.core.OfficeCore
import org.orthodoxwest.office.core.ReminderChoice
import org.orthodoxwest.office.core.reminderDefaults

/** The engine, loaded once per process: the app and its alarms share it. */
object Office {
    val core: OfficeCore by lazy { OfficeCore() }
}

/** One hour on the reminder page: whether it is chosen, and its time. */
data class HourReminder(val hour: String, val name: String, val chosen: Boolean, val time: LocalTime)

/** Minutes before the hour, as the web's "Remind me" offers them (without its alert-less option). */
val REMINDER_LEADS = listOf(0 to "At the hour", 5 to "5 minutes before", 10 to "10 minutes before", 15 to "15 minutes before", 30 to "30 minutes before")

/** The reader's reminders. Nothing fires until they are turned on. */
data class ReminderSettings(
    val on: Boolean,
    val hours: List<HourReminder>,
    val days: Set<DayOfWeek>,
    val lead: Int,
) {
    /** Sunday first, as the engine counts the week. */
    val weekdays: List<Boolean>
        get() = (listOf(DayOfWeek.SUNDAY) + DayOfWeek.entries.filter { it != DayOfWeek.SUNDAY }).map { it in days }
}

/** The settings in the app's preferences, starting from the web's defaults. */
class ReminderStore(context: Context) {
    private val prefs = context.getSharedPreferences("reminders", Context.MODE_PRIVATE)

    fun load(): ReminderSettings {
        val hours = reminderDefaults().map { d ->
            val saved = prefs.getString("hour.${d.hour}", null)?.split(",")
            if (saved != null && saved.size == 3) {
                HourReminder(d.hour, d.name, saved[0] == "1", LocalTime.of(saved[1].toInt(), saved[2].toInt()))
            } else {
                HourReminder(d.hour, d.name, d.chosen, LocalTime.of(d.hourOfDay, d.minute))
            }
        }
        val days = prefs.getString("days", null)?.split(",")?.mapNotNull { runCatching { DayOfWeek.valueOf(it) }.getOrNull() }?.toSet()
        return ReminderSettings(
            on = prefs.getBoolean("on", false),
            hours = hours,
            days = days ?: DayOfWeek.entries.toSet(),
            lead = prefs.getInt("lead", 10),
        )
    }

    fun save(s: ReminderSettings) {
        prefs.edit().apply {
            putBoolean("on", s.on)
            putInt("lead", s.lead)
            putString("days", s.days.joinToString(",") { it.name })
            s.hours.forEach { h -> putString("hour.${h.hour}", "${if (h.chosen) 1 else 0},${h.time.hour},${h.time.minute}") }
        }.apply()
    }
}

/**
 * Keeps the next week of reminders registered with the alarm service. Each alarm carries its
 * notification's words, so it posts at once, with no engine to load. Every boot, clock change,
 * update and app visit syncs again; a firing syncs only when the window is nearly spent.
 */
object ReminderScheduler {
    /** Days ahead kept scheduled: every weekday falls in the window beyond today, so a weekly reminder always has its next one waiting. */
    const val HORIZON_DAYS = 8

    private const val SCHEDULED = "scheduled"
    private const val LAST = "last"
    private val REFILL = Duration.ofDays(1)
    val SNOOZE: Duration = Duration.ofMinutes(10)

    /** Whether a fired reminder should sync: only once the last one scheduled is within a day, so most wake no engine. */
    fun due(context: Context, now: ZonedDateTime = ZonedDateTime.now()): Boolean {
        val last = context.getSharedPreferences("reminders", Context.MODE_PRIVATE).getLong(LAST, 0)
        return last - now.toInstant().toEpochMilli() < REFILL.toMillis()
    }

    fun sync(context: Context, now: ZonedDateTime = ZonedDateTime.now()) {
        val settings = ReminderStore(context).load()
        val alarms = context.getSystemService(AlarmManager::class.java)
        val state = context.getSharedPreferences("reminders", Context.MODE_PRIVATE)
        val zone: ZoneId = now.zone
        val wanted = if (!settings.on) emptyList() else {
            val chosen = settings.hours.filter { it.chosen }.map { ReminderChoice(it.hour, it.time.hour, it.time.minute) }
            Office.core.reminders(now.toLocalDate().toCivil(), HORIZON_DAYS, chosen, settings.weekdays).mapNotNull { r ->
                val office = LocalDate.of(r.date.year, r.date.month, r.date.day).atTime(r.hourOfDay, r.minute).atZone(zone)
                val at = office.minusMinutes(settings.lead.toLong())
                if (!at.isAfter(now)) null else Alarm(r.hour, office, at, r.title, r.feast, r.summary)
            }
        }
        for (a in wanted) set(alarms, a.at.toInstant().toEpochMilli(), a.pendingIntent(context))
        val codes = wanted.map { it.code }.toSet()
        state.getStringSet(SCHEDULED, emptySet())!!.mapNotNull { it.toIntOrNull() }.filter { it !in codes }.forEach { code ->
            PendingIntent.getBroadcast(context, code, Intent(context, ReminderReceiver::class.java).setAction(ReminderReceiver.FIRE), PendingIntent.FLAG_NO_CREATE or PendingIntent.FLAG_IMMUTABLE)
                ?.let { alarms.cancel(it); it.cancel() }
        }
        state.edit()
            .putStringSet(SCHEDULED, codes.map { it.toString() }.toSet())
            .putLong(LAST, wanted.maxOfOrNull { it.at.toInstant().toEpochMilli() } ?: 0)
            .apply()
    }

    /** "In 10 minutes": the reminder is put away and rings again, with the same words, after the wait. */
    fun snooze(context: Context, reminder: Intent, now: ZonedDateTime = ZonedDateTime.now()) {
        val code = reminderCode(LocalDate.parse(reminder.getStringExtra(EXTRA_DATE) ?: return), reminder.getStringExtra(EXTRA_HOUR) ?: return)
        NotificationManagerCompat.from(context).cancel(code)
        val again = Intent(context, ReminderReceiver::class.java).putExtras(reminder).setAction(ReminderReceiver.FIRE)
        // Its own request code, apart from the schedule's, so a sync neither replaces nor cancels it.
        val pending = PendingIntent.getBroadcast(context, -code, again, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
        set(context.getSystemService(AlarmManager::class.java), now.plus(SNOOZE).toInstant().toEpochMilli(), pending)
    }

    /** Exact when the reader allows it; otherwise the system's nearest, still through Doze. */
    private fun set(alarms: AlarmManager, at: Long, pending: PendingIntent) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S || alarms.canScheduleExactAlarms()) {
            alarms.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pending)
        } else {
            alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, pending)
        }
    }

    /** One scheduled reminder: the office's time, when to remind, and its words. */
    data class Alarm(val hour: String, val office: ZonedDateTime, val at: ZonedDateTime, val title: String, val feast: String, val summary: String) {
        val code: Int get() = reminderCode(office.toLocalDate(), hour)

        fun pendingIntent(context: Context): PendingIntent {
            val intent = Intent(context, ReminderReceiver::class.java)
                .setAction(ReminderReceiver.FIRE)
                .putExtra(EXTRA_HOUR, hour)
                .putExtra(EXTRA_DATE, office.toLocalDate().toString())
                .putExtra(EXTRA_TITLE, title)
                .putExtra(EXTRA_FEAST, feast)
                .putExtra(EXTRA_SUMMARY, summary)
                .putExtra(EXTRA_WHEN, office.toInstant().toEpochMilli())
            return PendingIntent.getBroadcast(context, code, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
        }
    }
}

/** Stable per office and day, so a sync replaces an alarm rather than adding one, and a notification replaces its own. */
fun reminderCode(date: LocalDate, hour: String): Int =
    (date.toEpochDay() * 8 + listOf("lauds", "prime", "terce", "sext", "none", "vespers", "compline").indexOf(hour).coerceAtLeast(0)).toInt()

const val EXTRA_HOUR = "org.orthodoxwest.office.HOUR"
const val EXTRA_DATE = "org.orthodoxwest.office.DATE"
private const val EXTRA_TITLE = "org.orthodoxwest.office.TITLE"
private const val EXTRA_FEAST = "org.orthodoxwest.office.FEAST"
private const val EXTRA_SUMMARY = "org.orthodoxwest.office.SUMMARY"
private const val EXTRA_WHEN = "org.orthodoxwest.office.WHEN"

/** Posts a reminder when its alarm fires, and syncs the schedule on every event that can disturb it. */
class ReminderReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action == SNOOZE) return ReminderScheduler.snooze(context, intent)
        if (intent.action == FIRE) {
            Notifications.post(context, intent)
            if (!ReminderScheduler.due(context)) return
        }
        // Syncing reads the corpus the first time in a process; keep it off the main thread.
        val pending = goAsync()
        Thread {
            try {
                ReminderScheduler.sync(context)
                // A clock or time-zone change, reboot or update moves the widget's hour too.
                if (intent.action != FIRE) Widgets.refresh(context)
            } finally {
                pending.finish()
            }
        }.start()
    }

    companion object {
        const val FIRE = "org.orthodoxwest.office.REMINDER"
        const val SNOOZE = "org.orthodoxwest.office.SNOOZE"
    }
}

object Notifications {
    /**
     * The reminders' channel. A channel's sound is fixed once created, so the bell came with a new
     * one; the first ("hours") rang the phone's default sound and is removed.
     */
    const val CHANNEL = "hours-bell"
    private const val FIRST_CHANNEL = "hours"

    fun ensureChannel(context: Context) {
        val manager = context.getSystemService(NotificationManager::class.java)
        if (manager.getNotificationChannel(CHANNEL) != null) return
        manager.deleteNotificationChannel(FIRST_CHANNEL)
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL, "Hours of prayer", NotificationManager.IMPORTANCE_DEFAULT).apply {
                description = "A bell at each hour you pray"
                val attributes = AudioAttributes.Builder()
                    .setUsage(AudioAttributes.USAGE_NOTIFICATION_EVENT)
                    .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                    .build()
                setSound(bell(context), attributes)
            },
        )
    }

    /** One soft stroke of a tubular bell (res/raw/bell.ogg; see tools/bake-bell.py). */
    fun bell(context: Context): Uri = Uri.parse("${ContentResolver.SCHEME_ANDROID_RESOURCE}://${context.packageName}/${R.raw.bell}")

    fun allowed(context: Context): Boolean =
        (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) &&
            NotificationManagerCompat.from(context).areNotificationsEnabled()

    /** "Vespers", then the day it keeps; a tap opens that hour. */
    fun post(context: Context, alarm: Intent) {
        if (!allowed(context)) return
        ensureChannel(context)
        val hour = alarm.getStringExtra(EXTRA_HOUR) ?: return
        val date = alarm.getStringExtra(EXTRA_DATE) ?: return
        val code = reminderCode(LocalDate.parse(date), hour)
        val open = Intent(context, MainActivity::class.java)
            .putExtra(EXTRA_HOUR, hour)
            .putExtra(EXTRA_DATE, date)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        val tap = PendingIntent.getActivity(context, code, open, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
        val later = PendingIntent.getBroadcast(
            context,
            code,
            Intent(context, ReminderReceiver::class.java).putExtras(alarm).setAction(ReminderReceiver.SNOOZE),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val notification = NotificationCompat.Builder(context, CHANNEL)
            .setSmallIcon(R.drawable.ic_notification)
            .setColor(Nave.gold.toArgb())
            .setContentTitle(alarm.getStringExtra(EXTRA_TITLE))
            .setContentText(alarm.getStringExtra(EXTRA_FEAST))
            .setTicker(alarm.getStringExtra(EXTRA_SUMMARY))
            .setWhen(alarm.getLongExtra(EXTRA_WHEN, System.currentTimeMillis()))
            .setShowWhen(true)
            .setCategory(NotificationCompat.CATEGORY_REMINDER)
            .setContentIntent(tap)
            .addAction(R.drawable.ic_notification, "In 10 minutes", later)
            .setAutoCancel(true)
            .build()
        @Suppress("MissingPermission")
        NotificationManagerCompat.from(context).notify(code, notification)
    }
}
