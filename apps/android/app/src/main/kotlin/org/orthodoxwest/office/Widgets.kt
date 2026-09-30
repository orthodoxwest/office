package org.orthodoxwest.office

import android.app.AlarmManager
import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Bundle
import android.util.SizeF
import android.widget.RemoteViews
import androidx.compose.ui.graphics.toArgb
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.temporal.ChronoUnit
import java.util.Locale
import org.orthodoxwest.office.core.HomeView
import org.orthodoxwest.office.core.currentOffice

/** The home-screen widget: the day, its feast and colour, and the invitation to the hour now. */
class OfficeWidget : AppWidgetProvider() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action == Widgets.REFRESH) refreshing(context) else super.onReceive(context, intent)
    }

    override fun onUpdate(context: Context, manager: AppWidgetManager, ids: IntArray) = refreshing(context)

    override fun onAppWidgetOptionsChanged(context: Context, manager: AppWidgetManager, id: Int, options: Bundle) = refreshing(context)

    override fun onDisabled(context: Context) = Widgets.stop(context)

    /** Composing the day reads the corpus the first time in a process; keep it off the main thread. */
    private fun refreshing(context: Context) {
        val pending = goAsync()
        Thread {
            try {
                Widgets.refresh(context)
            } finally {
                pending.finish()
            }
        }.start()
    }
}

/**
 * Keeps the widgets current. Each refresh sets a non-waking alarm for the next change of hour
 * (or midnight), so a sleeping phone is never woken for it: the widget catches up as the
 * screen comes on.
 */
object Widgets {
    const val REFRESH = "org.orthodoxwest.office.WIDGET_REFRESH"

    /** Opens home for today, as the brand link does. */
    const val HOME = "org.orthodoxwest.office.HOME"

    private val DAY = DateTimeFormatter.ofPattern("EEEE, MMMM d", Locale.US)

    fun refresh(context: Context, now: LocalDateTime = LocalDateTime.now()) {
        val manager = AppWidgetManager.getInstance(context)
        val ids = manager.getAppWidgetIds(ComponentName(context, OfficeWidget::class.java))
        if (ids.isEmpty()) return stop(context)
        manager.updateAppWidget(ids, views(context, now))
        val at = nextChange(now).atZone(ZoneId.systemDefault()).toInstant().toEpochMilli()
        // Within a minute of the change, and only when the phone is awake anyway.
        context.getSystemService(AlarmManager::class.java).setWindow(AlarmManager.RTC, at, 60_000, alarm(context))
    }

    fun stop(context: Context) = context.getSystemService(AlarmManager::class.java).cancel(alarm(context))

    /** The next clock hour at which the hour now or the day changes. */
    fun nextChange(now: LocalDateTime): LocalDateTime {
        val current = currentOffice(now.hour)
        var t = now.truncatedTo(ChronoUnit.HOURS).plusHours(1)
        while (t.hour != 0 && currentOffice(t.hour) == current) t = t.plusHours(1)
        return t
    }

    /** The full widget, and on Android 12+ a one-row strip for a widget sized to a single row. */
    fun views(context: Context, now: LocalDateTime): RemoteViews {
        val today = now.toLocalDate()
        val home = Office.core.home(today.toCivil(), today.toCivil(), now.hour)
        val theme = ThemeChoice.saved(context)
        val full = layout(context, R.layout.widget_office, home, today, theme)
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S) return full
        return RemoteViews(mapOf(SizeF(180f, 40f) to layout(context, R.layout.widget_office_strip, home, today, theme), SizeF(180f, 100f) to full))
    }

    fun layout(context: Context, layout: Int, home: HomeView, today: LocalDate, theme: ThemeChoice): RemoteViews =
        RemoteViews(context.packageName, layout).apply {
            val full = layout == R.layout.widget_office
            if (full) setTextViewText(R.id.widget_date, today.format(DAY))
            setTextViewText(R.id.widget_feast, home.feast)
            setTextViewText(R.id.widget_pray, home.prayNowLabel)
            setInt(R.id.widget_lozenge, "setColorFilter", dayColor(home.color).toArgb())
            // Default keeps the layout's colours, which follow the phone's light or dark as it changes.
            if (theme != ThemeChoice.DEFAULT) {
                val apse = theme == ThemeChoice.APSE
                val p = if (apse) Apse else Nave
                setInt(android.R.id.background, "setBackgroundResource", if (apse) R.drawable.widget_ground_apse else R.drawable.widget_ground_nave)
                setInt(R.id.widget_pray, "setBackgroundResource", if (apse) R.drawable.widget_invitation_apse else R.drawable.widget_invitation_nave)
                setTextColor(R.id.widget_pray, p.accent.toArgb())
                setTextColor(R.id.widget_feast, (if (full) p.accent else p.text).toArgb())
                if (full) {
                    setTextColor(R.id.widget_date, p.text.toArgb())
                    setInt(R.id.widget_rule_top, "setBackgroundColor", p.goldLine.toArgb())
                    setInt(R.id.widget_rule_bottom, "setBackgroundColor", p.goldLine.toArgb())
                    setInt(R.id.widget_lozenge, "setBackgroundColor", p.surface.toArgb())
                }
            }
            setOnClickPendingIntent(R.id.widget_pray, pray(context, home))
            setOnClickPendingIntent(R.id.widget_day, homeToday(context))
        }

    /** "Pray Vespers": that hour, of the day it belongs to (yesterday's Compline after midnight). */
    private fun pray(context: Context, home: HomeView): PendingIntent {
        val open = Intent(context, MainActivity::class.java)
            .putExtra(EXTRA_HOUR, home.prayNowHour)
            .putExtra(EXTRA_DATE, home.prayNowDate.toLocalDate().toString())
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        return PendingIntent.getActivity(context, 1, open, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    }

    private fun homeToday(context: Context): PendingIntent {
        val open = Intent(context, MainActivity::class.java)
            .setAction(HOME)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        return PendingIntent.getActivity(context, 2, open, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    }

    private fun alarm(context: Context): PendingIntent =
        PendingIntent.getBroadcast(
            context,
            0,
            Intent(context, OfficeWidget::class.java).setAction(REFRESH),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
}
