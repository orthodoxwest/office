package org.orthodoxwest.office

import android.app.Application
import android.app.AlarmManager
import android.content.Context
import android.content.res.Configuration
import android.os.Build
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.viewModelScope
import java.time.LocalDate
import java.time.LocalDateTime
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.orthodoxwest.office.core.CivilDate
import org.orthodoxwest.office.core.HomeView
import org.orthodoxwest.office.core.HourView
import org.orthodoxwest.office.core.OrdoMonthView
import org.orthodoxwest.office.core.UsageEvent
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

/**
 * The app's pages, as the web's routes: home for a day, an hour of a day, a month of the ordo
 * (brought to `day` when one is asked for, as the web's #d-date), and a year's frontispiece.
 */
sealed interface Page {
    data class Home(val date: LocalDate) : Page
    data class Hour(val date: LocalDate, val hour: String) : Page
    data class Ordo(val year: Int, val month: Int, val day: Int = 0) : Page
    data class Year(val year: Int) : Page
    data object Reminders : Page
}

/** A page as saved state writes it: "hour 2026-03-15 vespers". */
fun Page.encode(): String = when (this) {
    is Page.Home -> "home $date"
    is Page.Hour -> "hour $date $hour"
    is Page.Ordo -> "ordo $year $month $day"
    is Page.Year -> "year $year"
    Page.Reminders -> "reminders"
}

fun decodePage(s: String): Page? = runCatching {
    val f = s.split(" ")
    when (f[0]) {
        "home" -> Page.Home(LocalDate.parse(f[1]))
        "hour" -> Page.Hour(LocalDate.parse(f[1]), f[2]).takeIf { it.hour in hourNames() }
        "ordo" -> Page.Ordo(f[1].toInt(), f[2].toInt(), f.getOrNull(3)?.toInt() ?: 0)
        "year" -> Page.Year(f[1].toInt())
        "reminders" -> Page.Reminders
        else -> null
    }
}.getOrNull()

/** One place on the way back: its page, and an id no other visit shares, which keys its scroll position. */
data class Entry(val id: Long, val page: Page)

/** A page's composed content, ready to draw. */
sealed interface Content {
    data class Home(val view: HomeView) : Content
    data class Hour(val view: HourView) : Content
    data class Ordo(val view: OrdoMonthView) : Content

    /** The year's frontispiece and the reminders, drawn from the page itself. */
    data object Drawn : Content
    data class Failed(val message: String) : Content
}

/** A visit with its content ready: what the screen shows. */
data class Shown(val entry: Entry, val content: Content)

/**
 * How the screen moves to the page shown: deeper, or back; to the next or previous of the same
 * kind (an hour, a day, a month); or, with no order between them, a fade.
 */
enum class Motion { FORWARD, BACK, NEXT, PREVIOUS, FADE }

/** Where `next` stands after `page` of the same kind, in time: later, earlier, or neither. */
fun stepBetween(page: Page, next: Page): Motion {
    val order = when {
        page is Page.Hour && next is Page.Hour ->
            compareValuesBy(page, next, { it.date }, { hourNames().indexOf(it.hour) })
        page is Page.Home && next is Page.Home -> page.date.compareTo(next.date)
        page is Page.Ordo && next is Page.Ordo -> compareValuesBy(page, next, { it.year }, { it.month })
        page is Page.Year && next is Page.Year -> page.year.compareTo(next.year)
        else -> 0
    }
    return when {
        order < 0 -> Motion.NEXT
        order > 0 -> Motion.PREVIOUS
        else -> Motion.FADE
    }
}

/** What is shown, the reader's remembered choices, and the composed content once ready. */
class OfficeViewModel(app: Application, private val saved: SavedStateHandle) : AndroidViewModel(app) {
    private val prefs = app.getSharedPreferences("office", Context.MODE_PRIVATE)

    // Parsing the corpus takes a moment; it starts at once, off the main thread. The alarms share it.
    private val core = viewModelScope.async(Dispatchers.Default) { Office.core }
    private val reminderStore = ReminderStore(app)
    private val usage = Usage(prefs)
    private var loading: Job? = null

    val hours: List<String> = hourNames()

    /**
     * The pages behind the current one, for Back. Kept in saved state, so a return after Android
     * has closed the app in the background lands where the reader was.
     */
    private val stack = mutableStateListOf<Entry>().apply {
        saved.get<ArrayList<String>>(STACK)?.forEach { line ->
            val id = line.substringBefore('|').toLongOrNull() ?: return@forEach
            decodePage(line.substringAfter('|'))?.let { add(Entry(id, it)) }
        }
        if (isEmpty()) add(Entry(0, Page.Home(LocalDate.now())))
    }
    private var nextId = stack.maxOf { it.id } + 1
    val entry: Entry get() = stack.last()
    val entries: List<Entry> get() = stack
    val page: Page get() = entry.page
    val canGoBack: Boolean get() = stack.size > 1

    var form: String by mutableStateOf(prefs.getString("form", "private") ?: "private")
        private set
    var theme: ThemeChoice by mutableStateOf(ThemeChoice.saved(app))
        private set
    var textSize: TextSize by mutableStateOf(enumValueOrDefault(prefs.getString("text-size", null), TextSize.DEFAULT))
        private set
    /** Whether Prime reads the next day's Martyrology; off by default. */
    var martyrology: Boolean by mutableStateOf(prefs.getBoolean("martyrology", false))
        private set
    /** Whether the status bar is hidden while the app is open; off by default. */
    var fullScreen: Boolean by mutableStateOf(prefs.getBoolean("full-screen", false))
        private set

    /**
     * Each visit on the way back with its content, once composed: the page shown keeps the
     * screen until the next is ready, and Back has the page behind at hand to reveal.
     */
    private val contents = mutableStateMapOf<Long, Content>()

    /** The visit on screen: the current one once its content is ready, until then the one before. */
    var shown: Shown? by mutableStateOf(null)
        private set

    /** How the screen moves to the next page shown. */
    var motion: Motion by mutableStateOf(Motion.FADE)
        private set

    /** The page behind the current one, ready to draw, for the back gesture to reveal. */
    val behind: Shown?
        get() = stack.getOrNull(stack.lastIndex - 1)?.let { e -> contents[e.id]?.let { Shown(e, it) } }

    var reminders: ReminderSettings by mutableStateOf(reminderStore.load())
        private set
    var reminderStatus: ReminderStatus by mutableStateOf(ReminderStatus(notificationsAllowed = true, exactAllowed = true))
        private set

    val today: LocalDate get() = LocalDate.now()

    /** The day and clock hour home was last composed at: its highlighted hour and invitation are theirs. */
    private var homeClock: Pair<LocalDate, Int>? = null

    init {
        load()
        // Checked in steps of a minute rather than awaited as one delay to the next hour: a delay
        // stops counting while the phone sleeps, and a return is not always announced.
        viewModelScope.launch {
            while (true) {
                delay(60_000)
                refreshClock()
            }
        }
    }

    /**
     * Home composed again once the clock has passed into another hour or day, so a home opened at
     * Sext doesn't still highlight Sext at Vespers; today's home moves on to the new day. Called
     * every minute and on every return to the app.
     */
    fun refreshClock() {
        val shownAt = homeClock ?: return
        val home = page as? Page.Home ?: return
        val now = LocalDateTime.now()
        if (shownAt == now.toLocalDate() to now.hour) return
        if (home.date == shownAt.first && now.toLocalDate() != shownAt.first) {
            stack[stack.lastIndex] = Entry(entry.id, Page.Home(now.toLocalDate()))
            saved[STACK] = ArrayList(stack.map { "${it.id}|${it.page.encode()}" })
        }
        load()
    }

    /** Opens `page` over the current one; a page of the same kind replaces it, as a link would. */
    fun open(next: Page) {
        if (next == page) return load()
        val e = Entry(nextId++, next)
        if (page::class == next::class) {
            motion = stepBetween(page, next)
            stack[stack.lastIndex] = e
        } else {
            motion = Motion.FORWARD
            stack.add(e)
        }
        moved()
    }

    /** Back: the previous page, or false when home is all that is left. */
    fun back(): Boolean {
        if (stack.size <= 1) return false
        motion = Motion.BACK
        stack.removeAt(stack.lastIndex)
        moved()
        return true
    }

    /** Home for today, clearing the way back, as the brand link does. */
    fun goHome() {
        motion = if (stack.size > 1) Motion.BACK else stepBetween(page, Page.Home(today))
        stack.clear()
        stack.add(Entry(nextId++, Page.Home(today)))
        moved()
    }

    private fun moved() {
        saved[STACK] = ArrayList(stack.map { "${it.id}|${it.page.encode()}" })
        // Content leaves with its visit; a page already composed is shown at once.
        val kept = stack.map { it.id }.toSet()
        contents.keys.retainAll(kept)
        contents[entry.id]?.let { shown = Shown(entry, it) }
        load()
    }

    fun chooseForm(value: String) {
        form = value
        prefs.edit().putString("form", value).apply()
        // Hours composed in the old form are composed again when next shown.
        contents.keys.retainAll(setOf(entry.id))
        load()
    }

    fun chooseMartyrology(value: Boolean) {
        martyrology = value
        prefs.edit().putBoolean("martyrology", value).apply()
        // Prime composed with the old setting is composed again when next shown.
        contents.keys.retainAll(setOf(entry.id))
        load()
    }

    fun chooseFullScreen(value: Boolean) {
        fullScreen = value
        prefs.edit().putBoolean("full-screen", value).apply()
    }

    fun chooseTheme(value: ThemeChoice) {
        theme = value
        prefs.edit().putString("theme", value.name).apply()
        refreshWidgets()
        countVisit()
    }

    fun refreshWidgets() {
        val app = getApplication<Application>()
        viewModelScope.launch(Dispatchers.Default) { runCatching { Widgets.refresh(app) } }
    }

    fun chooseTextSize(value: TextSize) {
        textSize = value
        prefs.edit().putString("text-size", value.name).apply()
    }

    /** Saves the reminder page's choices and, when reminders are on, reschedules at once. */
    fun changeReminders(value: ReminderSettings) {
        // Turning reminders on counts as the web counts a generated feed link.
        if (value.on && !reminders.on) usage.record(UsageEvent.RemindersOn, dark(), form)
        reminders = value
        reminderStore.save(value)
        syncReminders()
    }

    fun setRemindersOn(on: Boolean) = changeReminders(reminders.copy(on = on))

    /** Whether the phone lets notifications through, and on the minute; read again on every return to the app. */
    fun refreshReminderStatus() {
        val app = getApplication<Application>()
        val alarms = app.getSystemService(AlarmManager::class.java)
        reminderStatus = ReminderStatus(
            notificationsAllowed = Notifications.allowed(app),
            exactAllowed = Build.VERSION.SDK_INT < Build.VERSION_CODES.S || alarms.canScheduleExactAlarms(),
        )
    }

    fun syncReminders() {
        val app = getApplication<Application>()
        viewModelScope.launch(Dispatchers.Default) { runCatching { ReminderScheduler.sync(app) } }
    }

    /**
     * Counts the page shown in the day's usage (Usage.kt): whenever it, its theme, its prayer
     * form or the Martyrology setting changes, and on every return to the app. Each is counted
     * once a day.
     */
    fun countVisit() {
        val event = when (val shown = page) {
            is Page.Home -> UsageEvent.Home(shown.date.toCivil())
            is Page.Hour -> UsageEvent.Hour(shown.date.toCivil(), shown.hour)
            is Page.Ordo -> UsageEvent.Ordo(shown.year)
            is Page.Year -> UsageEvent.Ordo(shown.year)
            Page.Reminders -> UsageEvent.RemindersPage
        }
        // Prime reports whether its Martyrology was shown, once the hour on screen is this page's.
        val hour = shown?.takeIf { it.entry.id == entry.id }?.content as? Content.Hour
        usage.record(event, dark(), form, hour?.view?.martyrology)
    }

    /** Whether the Apse is on screen: the reader's choice, or the phone's own dark mode under Default. */
    private fun dark(): Boolean {
        val night = getApplication<Application>().resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK
        return theme.dark(night == Configuration.UI_MODE_NIGHT_YES)
    }

    /** The ornament season of what is shown, which retints the gilding. */
    val season: String get() = when (val c = shown?.content) {
        is Content.Home -> c.view.ornament
        is Content.Hour -> c.view.ornament
        else -> ""
    }

    private fun load() {
        val visit = entry
        val now = LocalDateTime.now()
        homeClock = if (visit.page is Page.Home) now.toLocalDate() to now.hour else null
        loading?.cancel()
        loading = viewModelScope.launch {
            val content = try {
                val office = core.await()
                when (val page = visit.page) {
                    is Page.Home -> Content.Home(
                        withContext(Dispatchers.Default) {
                            office.home(page.date.toCivil(), now.toLocalDate().toCivil(), now.hour)
                        },
                    )
                    is Page.Hour -> Content.Hour(
                        withContext(Dispatchers.Default) {
                            office.compose(page.hour, page.date.year, page.date.monthValue, page.date.dayOfMonth, form, martyrology)
                        },
                    )
                    is Page.Ordo -> Content.Ordo(withContext(Dispatchers.Default) { office.ordoMonth(page.year, page.month) })
                    // The frontispiece is arithmetic, drawn at once from the page itself.
                    is Page.Year -> Content.Drawn
                    Page.Reminders -> {
                        refreshReminderStatus()
                        Content.Drawn
                    }
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Content.Failed(e.message ?: e.toString())
            }
            contents[visit.id] = content
            shown = Shown(visit, content)
            if (content !is Content.Failed) countVisit()
        }
    }
}

private const val STACK = "stack"

private inline fun <reified T : Enum<T>> enumValueOrDefault(name: String?, default: T): T =
    enumValues<T>().firstOrNull { it.name == name } ?: default
