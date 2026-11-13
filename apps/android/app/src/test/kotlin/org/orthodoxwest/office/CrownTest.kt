package org.orthodoxwest.office

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.test.core.app.ApplicationProvider
import com.github.takahirom.roborazzi.captureRoboImage
import java.time.LocalDate
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.HomeView
import org.orthodoxwest.office.core.crownPlan
import org.orthodoxwest.office.core.hourNames
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * Home's crown: the tap rule from the core, the play landing on the right look and leaving the
 * plain themed screen, the tap back kept for the settle, a touch elsewhere passing through, and
 * the plain path for a black day. Renders the play's middle into build/screenshots/ for review
 * beside the web's strips.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class CrownTest {
    @get:Rule
    val compose = createComposeRule()

    private val lent = LocalDate.of(2026, 3, 15)

    private inner class Stage(start: ThemeChoice, val deviceDark: Boolean = false) {
        var theme by mutableStateOf(start)
        var saved: ThemeChoice? = null
        var plain = 0
        lateinit var crown: Crown
        lateinit var view: HomeView
    }

    private fun show(stage: Stage, date: LocalDate = lent) {
        val view = Office.core.home(date.toCivil(), lent.toCivil(), 18)
        stage.view = view
        compose.setContent {
            val crown = rememberCrown()
            stage.crown = crown
            crown.choice = { stage.theme }
            crown.deviceDark = stage.deviceDark
            crown.animationScale = { 1f }
            crown.save = { stage.saved = it }
            crown.apply = { stage.theme = it }
            crown.plain = { stage.plain++; stage.theme = it }
            Box(Modifier.fillMaxSize().crowning(crown)) {
                OfficeTheme(choice = stage.theme, season = view.ornament) {
                    CompositionLocalProvider(LocalCrown provides crown) {
                        OfficeApp(
                            Page.Home(date), lent, hourNames(), view, null, null, null, "private", stage.theme, TextSize.DEFAULT, PaddingValues(),
                            onOpen = {}, onHome = {}, onForm = {}, onTheme = {}, onTextSize = {},
                            reminders = ReminderStore(ApplicationProvider.getApplicationContext()).load(),
                            reminderStatus = ReminderStatus(notificationsAllowed = true, exactAllowed = true),
                            onReminders = {}, onTurnOn = {}, onTurnOff = {}, onAllowNotifications = {}, onAllowExact = {},
                        )
                    }
                }
            }
        }
        compose.waitForIdle()
        compose.mainClock.autoAdvance = false
    }

    private fun advance(ms: Long) = compose.mainClock.advanceTimeBy(ms)

    private fun shoot(name: String) = compose.onRoot().captureRoboImage("build/screenshots/$name.png")

    @Test
    fun theTapRuleIsTheCoresWithTheTapBackKept() {
        val stage = Stage(ThemeChoice.APSE, deviceDark = true)
        show(stage)
        val crown = stage.crown
        // An Apse chosen on a dark phone: the look turns light, saved as Nave, and the tap back
        // brings Apse back, not Default.
        assertEquals(ThemeChoice.NAVE, crown.next())
        stage.theme = ThemeChoice.NAVE
        assertEquals(ThemeChoice.APSE, crown.next())
        stage.theme = ThemeChoice.APSE
        // Default on a light phone turns dark as Apse; a look matching the device is Default.
        val light = Stage(ThemeChoice.DEFAULT)
        crown.choice = { light.theme }
        crown.deviceDark = false
        assertEquals(ThemeChoice.APSE, crown.next())
        light.theme = ThemeChoice.APSE
        assertEquals(ThemeChoice.DEFAULT, crown.next())
    }

    @Test
    fun duskPlaysAndSettlesOnTheApse() {
        val stage = Stage(ThemeChoice.NAVE)
        show(stage)
        compose.onNodeWithContentDescription("Switch to Apse").performClick()
        advance(200)
        // Saved at the tap, shown at the turn.
        assertTrue(stage.crown.playing)
        assertEquals(ThemeChoice.APSE, stage.saved)
        assertEquals(ThemeChoice.NAVE, stage.theme)
        advance(1000)
        assertEquals(ThemeChoice.APSE, stage.theme)
        assertEquals("took the plain change: ${stage.crown.plainBecause}", 0, stage.plain)
        shoot("crown-dusk-1200")
        advance(1400)
        shoot("crown-dusk-2600")
        advance(700)
        shoot("crown-dusk-3300")
        advance(crownPlan(true).end.toLong() + 1500)
        compose.waitForIdle()
        assertFalse(stage.crown.playing)
        assertTrue(stage.crown.lifted.isEmpty())
        assertEquals(ThemeChoice.APSE, stage.theme)
        // The button names what it will do now.
        compose.onNodeWithContentDescription("Switch to Nave").assertExists()
        shoot("crown-dusk-settled")
    }

    @Test
    fun dawnPlaysAndSettlesOnTheNave() {
        val stage = Stage(ThemeChoice.APSE)
        show(stage)
        compose.onNodeWithContentDescription("Switch to Nave").performClick()
        advance(1000)
        assertEquals("took the plain change: ${stage.crown.plainBecause}", 0, stage.plain)
        assertEquals(ThemeChoice.NAVE, stage.theme)
        shoot("crown-dawn-1000")
        advance(1300)
        shoot("crown-dawn-2300")
        advance(700)
        shoot("crown-dawn-3000")
        advance(crownPlan(false).end.toLong() + 1500)
        compose.waitForIdle()
        assertFalse(stage.crown.playing)
        // Nave on a light phone is the device's look: saved as Default.
        assertEquals(ThemeChoice.DEFAULT, stage.theme)
        assertEquals("took the plain change: ${stage.crown.plainBecause}", 0, stage.plain)
    }

    @Test
    fun aSecondTapDuringThePlayPlaysTheWayBackAtTheSettle() {
        val stage = Stage(ThemeChoice.NAVE)
        show(stage)
        compose.onNodeWithContentDescription("Switch to Apse").performClick()
        advance(1500)
        compose.onNodeWithContentDescription("Switch to Nave").performClick()
        advance(crownPlan(true).end.toLong() + 500)
        // The way back is under way, or done.
        advance(crownPlan(false).end.toLong() + 1500)
        compose.waitForIdle()
        assertFalse(stage.crown.playing)
        assertEquals(ThemeChoice.NAVE, stage.theme)
        assertEquals(ThemeChoice.NAVE, stage.saved)
    }

    @Test
    fun aTouchElsewhereRunsThePlayOutAndPassesThrough() {
        val stage = Stage(ThemeChoice.NAVE)
        show(stage)
        compose.onNodeWithContentDescription("Switch to Apse").performClick()
        advance(1500)
        assertTrue("not playing: ${stage.crown.plainBecause}", stage.crown.playing)
        compose.onNodeWithText(stage.view.prayNowLabel).performClick()
        advance(crownPlan(true).rush.toLong() + 200)
        compose.waitForIdle()
        assertFalse(stage.crown.playing)
        assertEquals(ThemeChoice.APSE, stage.theme)
    }

    @Test
    fun aTouchBeforeTheTurnTakesTheMenusChange() {
        val stage = Stage(ThemeChoice.NAVE)
        show(stage)
        compose.onNodeWithContentDescription("Switch to Apse").performClick()
        advance(200)
        compose.onNodeWithText(stage.view.prayNowLabel).performClick()
        advance(100)
        assertFalse(stage.crown.playing)
        assertEquals(1, stage.plain)
        assertEquals(ThemeChoice.APSE, stage.theme)
    }

    @Test
    fun aBlackDayTakesTheMenusChange() {
        val stage = Stage(ThemeChoice.NAVE)
        // All Souls.
        show(stage, LocalDate.of(2026, 11, 2))
        assertEquals("black", stage.view.color)
        compose.onNodeWithContentDescription("Switch to Apse").performClick()
        advance(100)
        assertFalse(stage.crown.playing)
        assertEquals(1, stage.plain)
        assertEquals(ThemeChoice.APSE, stage.theme)
        assertNull(stage.crown.play)
    }
}
