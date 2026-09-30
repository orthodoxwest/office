package org.orthodoxwest.office

import android.app.Application
import android.content.Context
import androidx.compose.runtime.getValue
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
import org.orthodoxwest.office.core.HourView
import org.orthodoxwest.office.core.OfficeCore
import org.orthodoxwest.office.core.currentOffice
import org.orthodoxwest.office.core.hourNames

/** The prayer forms the engine composes, with their menu labels. */
val PRAYER_FORMS = listOf("private" to "Private", "deacon" to "Deacon", "priest" to "Priest")

/** Which office is shown, and the composed hour once it is ready. */
class OfficeViewModel(app: Application) : AndroidViewModel(app) {
    private val prefs = app.getSharedPreferences("office", Context.MODE_PRIVATE)

    // Parsing the corpus takes a moment; it starts at once, off the main thread.
    private val core = viewModelScope.async(Dispatchers.Default) { OfficeCore() }
    private var composing: Job? = null

    val hours: List<String> = hourNames()

    var date: LocalDate by mutableStateOf(LocalDate.now())
        private set
    var hour: String by mutableStateOf("lauds")
        private set
    var form: String by mutableStateOf(prefs.getString("form", "private") ?: "private")
        private set
    var view: HourView? by mutableStateOf(null)
        private set
    var error: String? by mutableStateOf(null)
        private set

    init {
        goToNow()
    }

    /** The office being prayed now, by the web's schedule. */
    fun goToNow() {
        val now = LocalDateTime.now()
        val current = currentOffice(now.hour)
        show(now.toLocalDate().plusDays(current.dayOffset.toLong()), current.hour)
    }

    fun show(date: LocalDate = this.date, hour: String = this.hour) {
        this.date = date
        this.hour = hour
        compose()
    }

    fun chooseForm(form: String) {
        this.form = form
        prefs.edit().putString("form", form).apply()
        compose()
    }

    private fun compose() {
        val (d, h, f) = Triple(date, hour, form)
        composing?.cancel()
        composing = viewModelScope.launch {
            try {
                view = withContext(Dispatchers.Default) { core.await().compose(h, d.year, d.monthValue, d.dayOfMonth, f) }
                error = null
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                error = e.message ?: e.toString()
            }
        }
    }
}
