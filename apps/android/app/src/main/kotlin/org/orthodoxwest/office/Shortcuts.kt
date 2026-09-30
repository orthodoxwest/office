package org.orthodoxwest.office

import android.content.Context
import android.content.Intent
import androidx.core.content.pm.ShortcutInfoCompat
import androidx.core.content.pm.ShortcutManagerCompat
import androidx.core.graphics.drawable.IconCompat
import java.time.LocalDate
import java.time.LocalDateTime
import org.orthodoxwest.office.core.currentOffice

/** Long-pressing the app's icon offers the hours most kept at home, and the ordo. */
object Shortcuts {
    const val OPEN = "org.orthodoxwest.office.OPEN"
    const val ORDO = "ordo"

    private val SHOWN = listOf(
        Triple("lauds", "Lauds", R.drawable.shortcut_lauds),
        Triple("vespers", "Vespers", R.drawable.shortcut_vespers),
        Triple("compline", "Compline", R.drawable.shortcut_compline),
        Triple(ORDO, "Ordo", R.drawable.shortcut_ordo),
    )

    /**
     * Published from code rather than res/xml: a static shortcut names its package outright,
     * and the preview's differs from the store app's. The intents carry no date, so each opens
     * the day it is tapped.
     */
    fun publish(context: Context) {
        val shortcuts = SHOWN.mapIndexed { rank, (target, label, icon) ->
            ShortcutInfoCompat.Builder(context, target)
                .setShortLabel(label)
                .setLongLabel(if (target == ORDO) "This month in the ordo" else "Pray $label")
                .setIcon(IconCompat.createWithResource(context, icon))
                .setIntent(Intent(context, MainActivity::class.java).setAction(OPEN).putExtra(EXTRA_HOUR, target))
                .setRank(rank)
                .build()
        }
        ShortcutManagerCompat.setDynamicShortcuts(context, shortcuts)
    }

    /**
     * The page a shortcut opens at `now`. An hour opens today's, except Compline in the small
     * hours, which belongs to the day before, as home's "Pray now" reckons it.
     */
    fun page(target: String, now: LocalDateTime = LocalDateTime.now()): Page? {
        val today = now.toLocalDate()
        if (target == ORDO) return Page.Ordo(today.year, today.monthValue, today.dayOfMonth)
        if (SHOWN.none { it.first == target }) return null
        val current = currentOffice(now.hour)
        val date: LocalDate = if (current.hour == target) today.plusDays(current.dayOffset.toLong()) else today
        return Page.Hour(date, target)
    }
}
