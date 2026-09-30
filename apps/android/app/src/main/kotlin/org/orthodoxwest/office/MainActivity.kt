package org.orthodoxwest.office

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBars
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import java.time.LocalDate
import org.orthodoxwest.office.core.HomeView
import org.orthodoxwest.office.core.HourView
import org.orthodoxwest.office.core.OrdoMonthView

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        setContent {
            val vm: OfficeViewModel = viewModel()
            BackHandler(enabled = vm.canGoBack) { vm.back() }
            OfficeTheme(choice = vm.theme, textSize = vm.textSize, season = vm.season) {
                OfficeApp(
                    page = vm.page,
                    today = vm.today,
                    hours = vm.hours,
                    home = vm.home,
                    hour = vm.hour,
                    ordo = vm.ordo,
                    error = vm.error,
                    form = vm.form,
                    theme = vm.theme,
                    textSize = vm.textSize,
                    insets = WindowInsets.systemBars.asPaddingValues(),
                    onOpen = vm::open,
                    onHome = vm::goHome,
                    onForm = vm::chooseForm,
                    onTheme = vm::chooseTheme,
                    onTextSize = vm::chooseTextSize,
                )
            }
        }
    }
}

/** The app on its plaster wall: whichever page is open, under the shared header and menu. */
@Composable
fun OfficeApp(
    page: Page,
    today: LocalDate,
    hours: List<String>,
    home: HomeView?,
    hour: HourView?,
    ordo: OrdoMonthView?,
    error: String?,
    form: String,
    theme: ThemeChoice,
    textSize: TextSize,
    insets: PaddingValues,
    onOpen: (Page) -> Unit,
    onHome: () -> Unit,
    onForm: (String) -> Unit,
    onTheme: (ThemeChoice) -> Unit,
    onTextSize: (TextSize) -> Unit,
) {
    var menu by remember { mutableStateOf(false) }
    val chrome: @Composable () -> Unit = {
        SiteHeader(onHome = { menu = false; onHome() }, menuOpen = menu, onMenu = { menu = !menu })
        if (menu) {
            val onHourPage = page as? Page.Hour
            MenuPanel(
                currentHour = onHourPage?.hour,
                onHour = onHourPage?.let { h -> { name: String -> menu = false; onOpen(Page.Hour(h.date, name)) } },
                onOrdo = {
                    menu = false
                    val d = when (page) {
                        is Page.Home -> page.date
                        is Page.Hour -> page.date
                        is Page.Ordo -> LocalDate.of(page.year, page.month, 1)
                    }
                    onOpen(Page.Ordo(d.year, d.monthValue))
                },
                onOrdoCurrent = page is Page.Ordo,
                theme = theme,
                onTheme = onTheme,
                textSize = textSize,
                onTextSize = onTextSize,
                onDismiss = { menu = false },
                topOffset = insets.calculateTopPadding() + 52.dp,
            )
        }
    }
    Box(Modifier.fillMaxSize()) {
        PlasterWall()
        when {
            error != null -> Message(error, insets)
            page is Page.Home && home != null -> HomeScreen(
                view = home,
                date = page.date,
                today = today,
                chrome = chrome,
                insets = insets,
                onDate = { onOpen(Page.Home(it)) },
                onHour = { d, h -> onOpen(Page.Hour(d, h)) },
                onOrdoDay = { onOpen(Page.Ordo(page.date.year, page.date.monthValue)) },
            )
            page is Page.Hour && hour != null && hour.hour == page.hour -> HourScreen(
                view = hour,
                date = page.date,
                today = today,
                hours = hours,
                form = form,
                chrome = chrome,
                insets = insets,
                onDate = { onOpen(Page.Hour(it, page.hour)) },
                onForm = onForm,
                onHour = { onOpen(Page.Hour(page.date, it)) },
                onAllHours = { onOpen(Page.Home(page.date)) },
            )
            page is Page.Ordo -> OrdoScreen(
                month = ordo?.takeIf { it.year == page.year && it.month == page.month },
                year = page.year,
                monthNumber = page.month,
                today = today,
                chrome = chrome,
                insets = insets,
                onMonth = { y, m -> onOpen(Page.Ordo(y, m)) },
                onDay = { onOpen(Page.Home(it)) },
            )
            else -> Message("Preparing the office…", insets)
        }
    }
}

@Composable
private fun Message(text: String, insets: PaddingValues) {
    Box(Modifier.fillMaxSize().padding(insets), contentAlignment = Alignment.Center) {
        Text(text, Modifier.padding(24.dp), style = Type.body.copy(color = LocalPalette.current.muted, textAlign = TextAlign.Center))
    }
}
