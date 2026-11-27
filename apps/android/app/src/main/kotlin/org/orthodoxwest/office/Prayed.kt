package org.orthodoxwest.office

import android.content.Context
import androidx.compose.foundation.lazy.LazyListLayoutInfo
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.repeatOnLifecycle
import java.time.LocalDate
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.distinctUntilChanged
import org.orthodoxwest.office.core.EndLeaf
import org.orthodoxwest.office.core.PrayedReading
import org.orthodoxwest.office.core.endLeafNext
import org.orthodoxwest.office.core.endLeafOpens
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
 * How far an hour's visit has been prayed. It lives with the visit's place (`placeSaver`), so an
 * hour reopened at the reader's place takes up the reading where it was and can still earn its
 * mark, as the web's office-place carries it.
 */
class PrayedProgress {
    var reading = PrayedReading(0.0, 0.0)
    var done = false
}

/**
 * Marks `hour` of `date` prayed once its end is reached after its words were read on screen at a
 * praying pace (the core's `PrayedReading`, as the web's): a quick scroll to the end, or a wait
 * there, does not. `spans` gives each office item's key its words' place, from and to, among
 * the `total` shown; `end` is the key of the item that closes the hour. `state` is the visit's
 * reading so far.
 */
@Composable
fun PrayedWatch(date: LocalDate, hour: String, list: LazyListState, spans: Map<Any, Pair<Double, Double>>, total: Double, end: Any, state: PrayedProgress) {
    val context = LocalContext.current
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    val currentSpans by rememberUpdatedState(spans)
    val currentTotal by rememberUpdatedState(total)
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

/**
 * The leaf on the consecration cross that ends `hour` of `date` (the core's `EndLeaf`, as the
 * web's `.hour-end-mark`): gilt from the first paint if home already marks the hour prayed; else
 * it starts gilding once [PrayedWatch] has marked it and the cross then stands wholly on the
 * screen, not as the cross first peeps in. A `bare` hour takes none. The cross stands `top`
 * pixels down the list item keyed `end`, `size` pixels square.
 */
@Composable
fun rememberEndLeaf(date: LocalDate, hour: String, list: LazyListState, bare: Boolean, end: Any, top: Int, size: Int): EndLeaf {
    val context = LocalContext.current
    var leaf by remember(date, hour, bare) { mutableStateOf(endLeafOpens(bare, Prayed.hours(context, date).contains(hour))) }
    LaunchedEffect(date, hour, bare, top, size) {
        snapshotFlow { Prayed.version to endCrossShown(list.layoutInfo, end, top, size) }
            .distinctUntilChanged()
            .collect { (_, shown) ->
                if (leaf == EndLeaf.PAINT && shown) leaf = endLeafNext(leaf, bare, Prayed.hours(context, date).contains(hour), true)
            }
    }
    return leaf
}

/**
 * Whether the cross that ends the hour stands wholly on the screen: the list item keyed `end` is
 * laid out, and the cross, `top` pixels down it and `size` tall, lies between the list's top and
 * its foot above the system bar (the content padding's).
 */
fun endCrossShown(info: LazyListLayoutInfo, end: Any, top: Int, size: Int): Boolean {
    val item = info.visibleItemsInfo.firstOrNull { it.key == end } ?: return false
    return whollyShown(item.offset + top, size, info.viewportEndOffset - info.afterContentPadding)
}

/** Whether a mark from `from`, `size` tall, lies wholly between the list's top (0) and `foot`. */
fun whollyShown(from: Int, size: Int, foot: Int): Boolean = from >= 0 && from + size <= foot
