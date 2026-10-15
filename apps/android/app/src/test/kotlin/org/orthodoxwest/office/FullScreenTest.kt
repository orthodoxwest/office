package org.orthodoxwest.office

import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.test.core.app.ActivityScenario
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** The status bar is put away while the app is open. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [35])
class FullScreenTest {
    @Test
    fun statusBarHidden() {
        ActivityScenario.launch(MainActivity::class.java).use { scenario ->
            scenario.onActivity { a ->
                val insets = ViewCompat.getRootWindowInsets(a.window.decorView)!!
                assertEquals(false, insets.isVisible(WindowInsetsCompat.Type.statusBars()))
            }
        }
    }
}
