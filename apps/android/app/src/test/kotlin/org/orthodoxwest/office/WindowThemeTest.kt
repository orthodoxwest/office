package org.orthodoxwest.office

import android.content.Context
import android.graphics.drawable.ColorDrawable
import androidx.core.view.WindowCompat
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The window and its bars follow the reader's theme, not the phone's light or dark. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class WindowThemeTest {
    private fun choose(theme: ThemeChoice) {
        ApplicationProvider.getApplicationContext<Context>().getSharedPreferences("office", Context.MODE_PRIVATE)
            .edit().putString("theme", theme.name).commit()
    }

    /** The window's background colour and whether the status bar's icons are dark. */
    private fun dressed(): Pair<Int, Boolean> {
        var got = 0 to false
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            scenario.onActivity { a ->
                val bars = WindowCompat.getInsetsController(a.window, a.window.decorView)
                got = (a.window.decorView.background as ColorDrawable).color to bars.isAppearanceLightStatusBars
            }
        }
        return got
    }

    @Test
    @Config(qualifiers = "night")
    fun naveOnAPhoneInDarkMode() {
        choose(ThemeChoice.NAVE)
        assertEquals(0xFFFAF3E9.toInt() to true, dressed())
    }

    @Test
    @Config(qualifiers = "notnight")
    fun apseOnAPhoneInLightMode() {
        choose(ThemeChoice.APSE)
        assertEquals(0xFF121C28.toInt() to false, dressed())
    }

    @Test
    @Config(qualifiers = "night")
    fun defaultFollowsThePhone() {
        choose(ThemeChoice.DEFAULT)
        assertEquals(0xFF121C28.toInt() to false, dressed())
    }
}
