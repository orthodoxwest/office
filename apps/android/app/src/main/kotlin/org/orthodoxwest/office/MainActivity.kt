package org.orthodoxwest.office

import android.Manifest
import android.content.Intent
import android.content.res.Configuration
import android.content.res.Resources
import android.graphics.Color
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.SystemBarStyle
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.asPaddingValues
import androidx.compose.foundation.layout.calculateEndPadding
import androidx.compose.foundation.layout.calculateStartPadding
import androidx.compose.foundation.layout.displayCutout
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.systemBars
import androidx.compose.foundation.layout.union
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.saveable.rememberSaveableStateHolder
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalLayoutDirection
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import java.time.LocalDate
import org.orthodoxwest.office.core.HomeView
import org.orthodoxwest.office.core.HourView
import org.orthodoxwest.office.core.OrdoMonthView
import org.orthodoxwest.office.core.ordoYear

/** The scrims `enableEdgeToEdge` gives a three-button navigation bar by default (Android 8–9). */
private val LIGHT_SCRIM = Color.argb(0xe6, 0xff, 0xff, 0xff)
private val DARK_SCRIM = Color.argb(0x80, 0x1b, 0x1b, 0x1b)

class MainActivity : ComponentActivity() {
    private val vm: OfficeViewModel by viewModels()

    // Turning reminders on asks for notifications first (Android 13+); on a refusal they are
    // still scheduled, and the page says how to let them through.
    private val askNotifications = registerForActivityResult(ActivityResultContracts.RequestPermission()) {
        vm.setRemindersOn(true)
        vm.refreshReminderStatus()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        // The reader's theme from the first frame, not the phone's light or dark.
        val theme = ThemeChoice.saved(this)
        setTheme(theme.window)
        dress(theme, theme.dark((resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES))
        super.onCreate(savedInstanceState)
        // A restored activity still holds the intent that first opened it; the saved way back already includes it.
        if (savedInstanceState == null) openFrom(intent)
        Shortcuts.publish(this)
        setContent {
            val vm = vm
            val dark = vm.theme.dark(isSystemInDarkTheme())
            LaunchedEffect(vm.theme, dark) { dress(vm.theme, dark) }
            BackHandler(enabled = vm.canGoBack) { vm.back() }
            OfficeTheme(choice = vm.theme, textSize = vm.textSize, season = vm.season) {
                OfficeApp(
                    page = vm.page,
                    entry = vm.entry.id,
                    entries = vm.entries.map { it.id },
                    today = vm.today,
                    hours = vm.hours,
                    home = vm.home,
                    hour = vm.hour,
                    ordo = vm.ordo,
                    error = vm.error,
                    form = vm.form,
                    theme = vm.theme,
                    textSize = vm.textSize,
                    // The bars and any camera cutout: above and below on a phone upright, at the sides on its side.
                    insets = WindowInsets.systemBars.union(WindowInsets.displayCutout).asPaddingValues(),
                    onOpen = vm::open,
                    onHome = vm::goHome,
                    onForm = vm::chooseForm,
                    onTheme = vm::chooseTheme,
                    onTextSize = vm::chooseTextSize,
                    reminders = vm.reminders,
                    reminderStatus = vm.reminderStatus,
                    onReminders = vm::changeReminders,
                    onTurnOn = ::turnOnReminders,
                    onTurnOff = { vm.setRemindersOn(false) },
                    onAllowNotifications = ::openNotificationSettings,
                    onAllowExact = ::openExactAlarmSettings,
                )
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        openFrom(intent)
    }

    override fun onResume() {
        super.onResume()
        vm.refreshReminderStatus()
        // Every visit keeps the alarm window running ahead, and the widget current.
        vm.syncReminders()
        vm.refreshWidgets()
        // A return on a new day is that day's visit.
        vm.countVisit()
    }

    /**
     * The status and navigation bars' icons, light over the Apse and dark over the Nave, whatever
     * the phone's own mode; and the launch screen of the next start in the same theme.
     */
    private fun dress(choice: ThemeChoice, dark: Boolean) {
        enableEdgeToEdge(
            statusBarStyle = SystemBarStyle.auto(Color.TRANSPARENT, Color.TRANSPARENT) { dark },
            navigationBarStyle = SystemBarStyle.auto(LIGHT_SCRIM, DARK_SCRIM) { dark },
        )
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            splashScreen.setSplashScreenTheme(if (choice == ThemeChoice.DEFAULT) Resources.ID_NULL else choice.window)
        }
    }

    /**
     * A tapped reminder opens its hour, a launcher shortcut its hour or the ordo, today, and the
     * widget its hour or home.
     */
    private fun openFrom(intent: Intent?) {
        if (intent?.action == Widgets.HOME) return vm.goHome()
        val hour = intent?.getStringExtra(EXTRA_HOUR) ?: return
        if (intent.action == Shortcuts.OPEN) {
            Shortcuts.page(hour)?.let(vm::open)
            return
        }
        val date = intent.getStringExtra(EXTRA_DATE)?.let { runCatching { LocalDate.parse(it) }.getOrNull() } ?: return
        vm.open(Page.Hour(date, hour))
    }

    private fun turnOnReminders() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU && !Notifications.allowed(this)) {
            askNotifications.launch(Manifest.permission.POST_NOTIFICATIONS)
        } else {
            vm.setRemindersOn(true)
        }
    }

    private fun openNotificationSettings() {
        startActivity(Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(Settings.EXTRA_APP_PACKAGE, packageName))
    }

    private fun openExactAlarmSettings() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            startActivity(Intent(Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM, Uri.parse("package:$packageName")))
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
    reminders: ReminderSettings,
    reminderStatus: ReminderStatus,
    onReminders: (ReminderSettings) -> Unit,
    onTurnOn: () -> Unit,
    onTurnOff: () -> Unit,
    onAllowNotifications: () -> Unit,
    onAllowExact: () -> Unit,
    entry: Long = 0,
    entries: List<Long> = listOf(entry),
) {
    var menu by remember { mutableStateOf(false) }
    // The wide header's Settings panel: the theme and text size.
    var settings by remember { mutableStateOf(false) }
    // Each visit keeps its own scroll position and open sections, for Back and for a return
    // after Android has closed the app; a visit's state goes when it leaves the way back.
    val visits = rememberSaveableStateHolder()
    var kept by rememberSaveable { mutableStateOf(entries) }
    LaunchedEffect(entries) {
        (kept - entries.toSet()).forEach(visits::removeState)
        kept = entries
    }
    val onHourPage = page as? Page.Hour
    val nav = SiteNav(
        hours = hours,
        currentHour = onHourPage?.hour,
        onHour = onHourPage?.let { h -> { name: String -> menu = false; onOpen(Page.Hour(h.date, name)) } },
        onOrdo = {
            menu = false
            // The ordo at the day shown, as the web's /calendar opens at today's row.
            onOpen(
                when (page) {
                    is Page.Home -> Page.Ordo(page.date.year, page.date.monthValue, page.date.dayOfMonth)
                    is Page.Hour -> Page.Ordo(page.date.year, page.date.monthValue, page.date.dayOfMonth)
                    is Page.Ordo -> Page.Ordo(page.year, page.month)
                    is Page.Year -> Page.Year(page.year)
                    Page.Reminders -> Page.Ordo(today.year, today.monthValue, today.dayOfMonth)
                },
            )
        },
        ordoCurrent = page is Page.Ordo || page is Page.Year,
        onReminders = { menu = false; onOpen(Page.Reminders) },
        remindersCurrent = page is Page.Reminders,
        settingsOpen = settings,
        onSettings = { settings = !settings },
    )
    val chrome: @Composable () -> Unit = {
        SiteHeader(onHome = { menu = false; settings = false; onHome() }, menuOpen = menu, onMenu = { menu = !menu }, nav = nav)
        if (menu && !LocalWide.current) {
            MenuPanel(
                currentHour = nav.currentHour,
                onHour = nav.onHour,
                onOrdo = nav.onOrdo,
                onOrdoCurrent = nav.ordoCurrent,
                onReminders = nav.onReminders,
                onRemindersCurrent = nav.remindersCurrent,
                theme = theme,
                onTheme = onTheme,
                textSize = textSize,
                onTextSize = onTextSize,
                onDismiss = { menu = false },
                topOffset = insets.calculateTopPadding() + 52.dp,
            )
        }
        if (settings && LocalWide.current) {
            // Under the header's end: the nav shell is held to 68rem and centred.
            val screen = LocalConfiguration.current.screenWidthDp.dp
            MenuPanel(
                currentHour = null,
                onHour = null,
                onOrdo = nav.onOrdo,
                onOrdoCurrent = nav.ordoCurrent,
                onReminders = nav.onReminders,
                onRemindersCurrent = nav.remindersCurrent,
                theme = theme,
                onTheme = onTheme,
                textSize = textSize,
                onTextSize = onTextSize,
                onDismiss = { settings = false },
                topOffset = insets.calculateTopPadding() + 52.dp,
                prefsOnly = true,
                end = ((screen - 1088.dp) / 2).coerceAtLeast(0.dp) + Gutter,
            )
        }
    }
    // The web's desktop composition from its breakpoint up: a tablet, or a phone on its side.
    BoxWithConstraints(Modifier.fillMaxSize()) {
        CompositionLocalProvider(LocalWide provides (maxWidth >= WideFrom), LocalPrefs provides Prefs(theme, onTheme, textSize, onTextSize)) {
            PlasterWall()
            // The wall runs under the bars; the page keeps clear of them at the sides (the screens
            // take the top and bottom themselves, so their backgrounds run under the bars).
            val direction = LocalLayoutDirection.current
            Box(Modifier.padding(start = insets.calculateStartPadding(direction), end = insets.calculateEndPadding(direction))) {
                visits.SaveableStateProvider(entry) {
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
                            onOrdoDay = { onOpen(Page.Ordo(page.date.year, page.date.monthValue, page.date.dayOfMonth)) },
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
                            onToday = { onOpen(Page.Ordo(today.year, today.monthValue, today.dayOfMonth)) },
                            onYear = { onOpen(Page.Year(it)) },
                            onDay = { onOpen(Page.Home(it)) },
                            focusDay = page.day,
                        )
                        page is Page.Year -> OrdoYearScreen(
                            view = remember(page.year) { ordoYear(page.year) },
                            today = today,
                            chrome = chrome,
                            insets = insets,
                            onMonth = { y, m -> onOpen(Page.Ordo(y, m)) },
                            onToday = { onOpen(Page.Ordo(today.year, today.monthValue, today.dayOfMonth)) },
                            onYear = { onOpen(Page.Year(it)) },
                            onDay = { onOpen(Page.Ordo(it.year, it.monthValue, it.dayOfMonth)) },
                        )
                        page is Page.Reminders -> RemindersScreen(
                            settings = reminders,
                            status = reminderStatus,
                            chrome = chrome,
                            insets = insets,
                            onChange = onReminders,
                            onTurnOn = onTurnOn,
                            onTurnOff = onTurnOff,
                            onAllowNotifications = onAllowNotifications,
                            onAllowExact = onAllowExact,
                        )
                        else -> Message("Preparing the office…", insets)
                    }
                }
            }
        }
    }
}

@Composable
private fun Message(text: String, insets: PaddingValues) {
    Box(Modifier.fillMaxSize().padding(insets), contentAlignment = Alignment.Center) {
        Text(text, Modifier.padding(24.dp), style = Type.body.copy(color = LocalPalette.current.muted, textAlign = TextAlign.Center))
    }
}
