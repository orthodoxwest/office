package org.orthodoxwest.office

import android.app.Application
import android.content.Context
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import java.time.LocalDate
import java.time.LocalDateTime
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.orthodoxwest.office.core.CivilDate
import org.orthodoxwest.office.core.HomeView
import org.orthodoxwest.office.core.HourView
import org.orthodoxwest.office.core.OfficeCore
import org.orthodoxwest.office.core.OrdoMonthView
import org.orthodoxwest.office.core.hourNames

/** The prayer forms: value, the control's label, and the chooser's phrase (hour.html). */
val PRAYER_FORMS = listOf(
    Triple("private", "Private", "Praying privately"),
    Triple("deacon", "Deacon", "With others, led by a deacon"),
    Triple("priest", "Priest", "With others, led by a priest"),
)

/** Capitalizes an hour's name: "lauds" → "Lauds". */
fun hourLabel(hour: String): String = hour.replaceFirstChar { it.titlecase() }

fun CivilDate.toLocalDate(): LocalDate = LocalDate.of(year, month, day)
fun LocalDate.toCivil(): CivilDate = CivilDate(year, monthValue, dayOfMonth)

/** The app's pages, as the web's routes: home for a day, an hour of a day, a month of the ordo. */
sealed interface Page {
    data class Home(val date: LocalDate) : Page
    data class Hour(val date: LocalDate, val hour: String) : Page
    data class Ordo(val year: Int, val month: Int) : Page
}

/** What is shown, the reader's remembered choices, and the composed content once ready. */
class OfficeViewModel(app: Application) : AndroidViewModel(app) {
    private val prefs = app.getSharedPreferences("office", Context.MODE_PRIVATE)

    // Parsing the corpus takes a moment; it starts at once, off the main thread.
    private val core = viewModelScope.async(Dispatchers.Default) { OfficeCore() }
    private var loading: Job? = null

    val hours: List<String> = hourNames()

    /** The pages behind the current one, for Back. */
    private val stack = mutableStateListOf<Page>(Page.Home(LocalDate.now()))
    val page: Page get() = stack.last()
    val canGoBack: Boolean get() = stack.size > 1

    var form: String by mutableStateOf(prefs.getString("form", "private") ?: "private")
        private set
    var theme: ThemeChoice by mutableStateOf(enumValueOrDefault(prefs.getString("theme", null), ThemeChoice.DEFAULT))
        private set
    var textSize: TextSize by mutableStateOf(enumValueOrDefault(prefs.getString("text-size", null), TextSize.DEFAULT))
        private set

    var home: HomeView? by mutableStateOf(null)
        private set
    var hour: HourView? by mutableStateOf(null)
        private set
    var ordo: OrdoMonthView? by mutableStateOf(null)
        private set
    var error: String? by mutableStateOf(null)
        private set

    val today: LocalDate get() = LocalDate.now()

    init {
        load()
    }

    /** Opens `page` over the current one; a page of the same kind replaces it, as a link would. */
    fun open(next: Page) {
        if (stack.last()::class == next::class) stack[stack.lastIndex] = next else stack.add(next)
        load()
    }

    /** Back: the previous page, or false when home is all that is left. */
    fun back(): Boolean {
        if (stack.size <= 1) return false
        stack.removeAt(stack.lastIndex)
        load()
        return true
    }

    /** Home for today, clearing the way back, as the brand link does. */
    fun goHome() {
        stack.clear()
        stack.add(Page.Home(today))
        load()
    }

    fun chooseForm(value: String) {
        form = value
        prefs.edit().putString("form", value).apply()
        load()
    }

    fun chooseTheme(value: ThemeChoice) {
        theme = value
        prefs.edit().putString("theme", value.name).apply()
    }

    fun chooseTextSize(value: TextSize) {
        textSize = value
        prefs.edit().putString("text-size", value.name).apply()
    }

    /** The ornament season of what is shown, which retints the gilding. */
    val season: String get() = when (page) {
        is Page.Home -> home?.ornament.orEmpty()
        is Page.Hour -> hour?.ornament.orEmpty()
        is Page.Ordo -> ""
    }

    private fun load() {
        val shown = page
        loading?.cancel()
        loading = viewModelScope.launch {
            try {
                val office = core.await()
                when (shown) {
                    is Page.Home -> home = withContext(Dispatchers.Default) {
                        val now = LocalDateTime.now()
                        office.home(shown.date.toCivil(), now.toLocalDate().toCivil(), now.hour)
                    }
                    is Page.Hour -> hour = withContext(Dispatchers.Default) {
                        office.compose(shown.hour, shown.date.year, shown.date.monthValue, shown.date.dayOfMonth, form)
                    }
                    is Page.Ordo -> {
                        if (ordo?.let { it.year != shown.year || it.month != shown.month } == true) ordo = null
                        ordo = withContext(Dispatchers.Default) { office.ordoMonth(shown.year, shown.month) }
                    }
                }
                error = null
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                error = e.message ?: e.toString()
            }
        }
    }
}

private inline fun <reified T : Enum<T>> enumValueOrDefault(name: String?, default: T): T =
    enumValues<T>().firstOrNull { it.name == name } ?: default
