package org.orthodoxwest.office

import android.app.AlarmManager
import android.app.Application
import android.appwidget.AppWidgetManager
import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.view.View
import android.widget.FrameLayout
import android.widget.TextView
import androidx.test.core.app.ApplicationProvider
import java.io.File
import java.time.LocalDate
import java.time.LocalDateTime
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/** The home-screen widget's words, its refreshes, and how it looks (build/screenshots/widget-*.png). */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class WidgetTest {
    private val app: Application = ApplicationProvider.getApplicationContext()
    private val lent = LocalDate.of(2026, 3, 15)

    private fun choose(theme: ThemeChoice) =
        app.getSharedPreferences("office", Context.MODE_PRIVATE).edit().putString("theme", theme.name).commit()

    /** Inflates `layout` as a launcher would, at a size in dp, and saves it for review. */
    private fun render(layout: Int, theme: ThemeChoice, name: String, widthDp: Int, heightDp: Int, at: Int = 18): View {
        choose(theme)
        val home = Office.core.home(lent.toCivil(), lent.toCivil(), at)
        val views = Widgets.layout(app, layout, home, lent, theme)
        val parent = FrameLayout(app)
        val view = views.apply(app, parent)
        val d = app.resources.displayMetrics.density
        val (w, h) = (widthDp * d).toInt() to (heightDp * d).toInt()
        view.measure(View.MeasureSpec.makeMeasureSpec(w, View.MeasureSpec.EXACTLY), View.MeasureSpec.makeMeasureSpec(h, View.MeasureSpec.EXACTLY))
        view.layout(0, 0, w, h)
        val bitmap = Bitmap.createBitmap(w + 48, h + 48, Bitmap.Config.ARGB_8888)
        Canvas(bitmap).apply {
            drawColor(if (theme == ThemeChoice.APSE) 0xFF30343A.toInt() else 0xFF8A8F96.toInt())
            translate(24f, 24f)
            view.draw(this)
        }
        File("build/screenshots").mkdirs()
        File("build/screenshots/$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
        return view
    }

    @Test
    fun namesTheDayAndInvitesToTheHourNow() {
        val view = render(R.layout.widget_office, ThemeChoice.NAVE, "widget-nave", 330, 170)
        assertEquals("Sunday, March 15", view.findViewById<TextView>(R.id.widget_date).text.toString())
        assertEquals("III Sunday in Lent", view.findViewById<TextView>(R.id.widget_feast).text.toString())
        assertEquals("Pray Vespers", view.findViewById<TextView>(R.id.widget_pray).text.toString())
    }

    @Test
    fun inTheApseAndAsAStrip() {
        render(R.layout.widget_office, ThemeChoice.APSE, "widget-apse", 330, 170, at = 21)
        val strip = render(R.layout.widget_office_strip, ThemeChoice.NAVE, "widget-strip-nave", 330, 60, at = 7)
        assertEquals("Pray Prime", strip.findViewById<TextView>(R.id.widget_pray).text.toString())
    }

    @Test
    fun refreshesAtEachChangeOfHourAndAtMidnight() {
        val at = { h: Int, m: Int -> LocalDateTime.of(2026, 3, 15, h, m) }
        assertEquals(at(17, 0), Widgets.nextChange(at(13, 40)))
        assertEquals(at(20, 0), Widgets.nextChange(at(17, 0)))
        assertEquals(at(0, 0).plusDays(1), Widgets.nextChange(at(21, 15)))
        assertEquals(at(2, 0), Widgets.nextChange(at(0, 5)))
    }

    @Test
    fun onlyAPlacedWidgetSetsItsNextRefresh() {
        val alarms = shadowOf(app.getSystemService(AlarmManager::class.java))
        Widgets.refresh(app, LocalDateTime.of(2026, 3, 15, 13, 40))
        assertEquals(0, alarms.scheduledAlarms.size)
        shadowOf(AppWidgetManager.getInstance(app)).createWidget(OfficeWidget::class.java, R.layout.widget_office)
        // Placing it refreshes in the background; let that finish so it cannot re-set the alarm after stop.
        Widgets.worker.submit {}.get()
        Widgets.refresh(app, LocalDateTime.of(2026, 3, 15, 13, 40))
        // Never waking the phone for it.
        assertEquals(AlarmManager.RTC, alarms.scheduledAlarms.single().getType())
        Widgets.stop(app)
        assertEquals(0, alarms.scheduledAlarms.size)
    }
}
