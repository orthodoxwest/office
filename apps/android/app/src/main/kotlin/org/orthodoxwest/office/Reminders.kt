package org.orthodoxwest.office

import android.Manifest
import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.compose.ui.graphics.toArgb
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import java.time.DayOfWeek
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
 * Keeps the next few days of reminders registered with the alarm service. Each alarm carries its
 * notification's words, so it posts at once, with no engine to load; every firing, boot, clock
 * change, update and app visit syncs again, so the window always runs ahead.
 */
object ReminderScheduler {
    /** Days ahead kept scheduled: enough to ride out a phone left off for a day. */
    const val HORIZON_DAYS = 3

    private const val SCHEDULED = "scheduled"

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
        val exact = Build.VERSION.SDK_INT < Build.VERSION_CODES.S || alarms.canScheduleExactAlarms()
        for (a in wanted) {
            val pending = a.pendingIntent(context)
            val t = a.at.toInstant().toEpochMilli()
            // Exact when the reader allows it; otherwise the system's nearest, still through Doze.
            if (exact) alarms.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, t, pending)
            else alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, t, pending)
        }
        val codes = wanted.map { it.code }.toSet()
        state.getStringSet(SCHEDULED, emptySet())!!.mapNotNull { it.toIntOrNull() }.filter { it !in codes }.forEach { code ->
            PendingIntent.getBroadcast(context, code, Intent(context, ReminderReceiver::class.java).setAction(ReminderReceiver.FIRE), PendingIntent.FLAG_NO_CREATE or PendingIntent.FLAG_IMMUTABLE)
                ?.let { alarms.cancel(it); it.cancel() }
        }
        state.edit().putStringSet(SCHEDULED, codes.map { it.toString() }.toSet()).apply()
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
        if (intent.action == FIRE) Notifications.post(context, intent)
        // Syncing reads the corpus the first time in a process; keep it off the main thread.
        val pending = goAsync()
        Thread {
            try {
                ReminderScheduler.sync(context)
            } finally {
                pending.finish()
            }
        }.start()
    }

    companion object {
        const val FIRE = "org.orthodoxwest.office.REMINDER"
    }
}

object Notifications {
    const val CHANNEL = "hours"

    fun ensureChannel(context: Context) {
        val manager = context.getSystemService(NotificationManager::class.java)
        if (manager.getNotificationChannel(CHANNEL) != null) return
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL, "Hours of prayer", NotificationManager.IMPORTANCE_DEFAULT).apply {
                description = "A reminder at each hour you pray"
            },
        )
    }

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
            .setAutoCancel(true)
            .build()
        @Suppress("MissingPermission")
        NotificationManagerCompat.from(context).notify(code, notification)
    }
}
