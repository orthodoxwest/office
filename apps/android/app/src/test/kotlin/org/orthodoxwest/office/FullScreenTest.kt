package org.orthodoxwest.office

import android.content.Context
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The status bar is put away only when the reader has chosen Full screen. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class FullScreenTest {
    private fun choose(on: Boolean) {
        ApplicationProvider.getApplicationContext<Context>().getSharedPreferences("office", Context.MODE_PRIVATE)
            .edit().putBoolean("full-screen", on).commit()
    }

    private fun statusBarShown(): Boolean {
        var shown = true
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            scenario.onActivity { a ->
                shown = ViewCompat.getRootWindowInsets(a.window.decorView)!!.isVisible(WindowInsetsCompat.Type.statusBars())
            }
        }
        return shown
    }

    @Test
    fun shownByDefault() {
        assertEquals(true, statusBarShown())
    }

    @Test
    fun hiddenWhenChosen() {
        choose(true)
        assertEquals(false, statusBarShown())
    }
}
