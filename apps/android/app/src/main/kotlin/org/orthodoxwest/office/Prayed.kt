package org.orthodoxwest.office

import android.content.Context
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import java.time.LocalDate
import kotlinx.coroutines.delay
import org.orthodoxwest.office.core.PrayedReading
import org.orthodoxwest.office.core.prayedReadingAdvance
import org.orthodoxwest.office.core.prayedReadingDone

/**
 * The hours prayed, for home's marks, as the web's `office-prayed`: "2026-10-07/lauds", kept on
 * this device a week.
 */
object Prayed {
    private const val KEY = "prayed"
    private const val KEEP_DAYS = 7L

    /** Bumped on each mark, so a home already composed marks it too. */
    var version by mutableIntStateOf(0)
        private set

    private fun prefs(context: Context) = context.getSharedPreferences("office", Context.MODE_PRIVATE)

    /** The hours prayed on `date`. */
    fun hours(context: Context, date: LocalDate): Set<String> =
        prefs(context).getStringSet(KEY, emptySet()).orEmpty()
            .filter { it.substringBefore('/') == date.toString() }
            .map { it.substringAfter('/') }
            .toSet()

    fun mark(context: Context, date: LocalDate, hour: String) {
        val oldest = LocalDate.now().minusDays(KEEP_DAYS)
        val kept = prefs(context).getStringSet(KEY, emptySet()).orEmpty().filter {
            runCatching { !LocalDate.parse(it.substringBefore('/')).isBefore(oldest) }.getOrDefault(false)
        }
        prefs(context).edit().putStringSet(KEY, (kept + "$date/$hour").toSet()).apply()
        version++
    }
}

/**
 * Marks `hour` of `date` prayed once its end is reached after its words were read on screen at a
 * praying pace (the core's `PrayedReading`, as the web's): a quick scroll to the end, or a wait
 * there, does not. `spans` gives each office item's key its words' place, from and to, among
 * the `total` shown; `end` is the key of the item that closes the hour.
 */
@Composable
fun PrayedWatch(date: LocalDate, hour: String, list: LazyListState, spans: Map<Any, Pair<Double, Double>>, total: Double, end: Any) {
    val context = LocalContext.current
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    val currentSpans by rememberUpdatedState(spans)
    val currentTotal by rememberUpdatedState(total)
    val state = remember(date, hour) { object { var reading = PrayedReading(0.0, 0.0); var done = false } }
    LaunchedEffect(date, hour) {
        lifecycle.repeatOnLifecycle(Lifecycle.State.RESUMED) {
            var last = System.nanoTime()
            while (!state.done) {
                delay(1000)
                val now = System.nanoTime()
                val seconds = (now - last) / 1e9
                last = now
                val info = list.layoutInfo
                val shown = info.visibleItemsInfo.filter { it.offset + it.size > info.viewportStartOffset && it.offset < info.viewportEndOffset }
                val words = shown.mapNotNull { currentSpans[it.key] }
                val atEnd = shown.any { it.key == end }
                val first = words.minOfOrNull { it.first } ?: if (atEnd) currentTotal else 0.0
                val through = words.maxOfOrNull { it.second } ?: first
                state.reading = prayedReadingAdvance(state.reading, first, through, seconds)
                if (prayedReadingDone(state.reading, currentTotal, atEnd)) {
                    state.done = true
                    Prayed.mark(context, date, hour)
                }
            }
        }
    }
}
