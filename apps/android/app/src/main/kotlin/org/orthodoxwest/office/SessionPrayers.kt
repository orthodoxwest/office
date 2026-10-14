package org.orthodoxwest.office

import android.content.Context

/**
 * Whether an hour's prayers before or after the Office open, as the web's `office-prayers-open`:
 * the hour's default (Lauds, Prime and Vespers open, the other hours' closed) until the reader
 * opens or closes them, then that choice, kept on this device for the hour's section
 * ("sext/Prayers Before the Office"). A section a day leaves out (the Triduum's prayers after the
 * Office) keeps its choice for when it returns.
 */
object SessionPrayers {
    private val OPEN = setOf("lauds", "prime", "vespers")
    private const val PREFIX = "prayers-open."

    private fun prefs(context: Context) = context.getSharedPreferences("office", Context.MODE_PRIVATE)

    private fun key(hour: String, label: String) = "$PREFIX$hour/$label"

    fun opens(context: Context, hour: String, label: String): Boolean {
        val prefs = prefs(context)
        val key = key(hour, label)
        return if (prefs.contains(key)) prefs.getBoolean(key, false) else hour in OPEN
    }

    fun choose(context: Context, hour: String, label: String, open: Boolean) {
        prefs(context).edit().putBoolean(key(hour, label), open).apply()
    }
}
