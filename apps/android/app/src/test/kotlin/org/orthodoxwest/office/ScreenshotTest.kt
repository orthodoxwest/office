package org.orthodoxwest.office

import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import com.github.takahirom.roborazzi.captureRoboImage
import java.time.LocalDate
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.OfficeCore
import org.orthodoxwest.office.core.hourNames
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Renders real hours from the Rust core into build/screenshots/, for review without a device.
 * The tall captures show most of an hour in one image.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w411dp-h891dp-xxhdpi")
class ScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private fun shoot(name: String, hour: String, date: LocalDate, form: String = "private", dark: Boolean = false) {
        val view = core.compose(hour, date.year, date.monthValue, date.dayOfMonth, form)
        compose.setContent {
            OfficeTheme(dark = dark) {
                OfficeScreen(date, hour, hourNames(), form, view, null, onShow = { _, _ -> }, onForm = {}, onNow = {})
            }
        }
        compose.onRoot().captureRoboImage("build/screenshots/$name.png")
    }

    @Test
    fun laudsOfChristmas() = shoot("lauds-christmas", "lauds", LocalDate.of(2026, 12, 25))

    @Test
    @Config(qualifiers = "w411dp-h3000dp-xxhdpi")
    fun laudsOfChristmasTall() = shoot("lauds-christmas-tall", "lauds", LocalDate.of(2026, 12, 25))

    @Test
    @Config(qualifiers = "w411dp-h3000dp-xxhdpi")
    fun complineLedByAPriestTall() = shoot("compline-priest-tall", "compline", LocalDate.of(2026, 9, 30), form = "priest")

    @Test
    @Config(qualifiers = "w411dp-h3000dp-xxhdpi")
    fun vespersInTheDarkTall() = shoot("vespers-dark-tall", "vespers", LocalDate.of(2026, 3, 11), dark = true)

    @Test
    @Config(qualifiers = "w411dp-h3000dp-xxhdpi")
    fun primeTall() = shoot("prime-tall", "prime", LocalDate.of(2026, 9, 30))

    companion object {
        private val core by lazy { OfficeCore() }
    }
}
