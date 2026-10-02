package org.orthodoxwest.office

import androidx.compose.ui.graphics.Color
import java.io.File
import kotlin.math.abs
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

/** The app's palette is the web's: each Kotlin token matches its value in style.css. */
class TokensTest {
    private val css = File("../../office-web/static/style.css").readText()

    /** The declarations of the first rule whose selector is exactly `selector`. */
    private fun rule(selector: String): String {
        val start = Regex("(?m)^" + Regex.escape(selector) + " \\{").find(css)?.range?.last ?: error("no $selector rule")
        return css.substring(start, css.indexOf("\n}", start))
    }

    private fun parse(value: String): FloatArray {
        val v = value.trim()
        if (v.startsWith("#")) return floatArrayOf(v.substring(1, 3).toInt(16).toFloat(), v.substring(3, 5).toInt(16).toFloat(), v.substring(5, 7).toInt(16).toFloat(), 1f)
        val parts = v.substringAfter("(").substringBefore(")").split(",", " ", "/").filter { it.isNotBlank() }.map { it.trim().toFloat() }
        return floatArrayOf(parts[0], parts[1], parts[2], parts.getOrElse(3) { 1f })
    }

    private fun check(block: String, token: String, color: Color) {
        val value = Regex("(?m)^\\s*--" + Regex.escape(token) + ":\\s*([^;]+);").find(block)?.groupValues?.get(1) ?: error("--$token missing")
        val want = parse(value)
        val got = floatArrayOf(color.red * 255f, color.green * 255f, color.blue * 255f, color.alpha)
        for (i in 0..2) assertTrue("--$token: $value vs $color", abs(want[i] - got[i]) < 0.6f)
        assertEquals("--$token alpha", want[3], got[3], 0.01f)
    }

    private fun checkPalette(block: String, p: Palette) {
        check(block, "text", p.text)
        check(block, "bg", p.bg)
        check(block, "accent", p.accent)
        check(block, "gold", p.gold)
        check(block, "gold-line", p.goldLine)
        check(block, "muted", p.muted)
        check(block, "unsaid", p.unsaid)
        check(block, "border", p.border)
        check(block, "rubric", p.rubric)
        check(block, "surface", p.surface)
        check(block, "surface-edge", p.surfaceEdge)
        check(block, "pressed-wash", p.pressedWash)
        check(block, "oak-line", p.oakLine)
        check(block, "inscription-ground", p.inscriptionGround)
        check(block, "inscription-edge", p.inscriptionEdge)
        check(block, "inscription-wash", p.inscriptionWash)
    }

    @Test
    fun naveIsTheRoot() {
        val root = rule(":root").takeIf { it.contains("--text:") } ?: error(":root tokens")
        checkPalette(root, Nave)
        val o = ornament(Nave, "")
        check(root, "ornament-hi", o.hi)
        check(root, "ornament-lo", o.lo)
        check(root, "inscription-ink", o.ink)
    }

    @Test
    fun apseIsTheDarkTheme() {
        val dark = rule(":root[data-theme=\"dark\"]")
        checkPalette(dark, Apse)
        val o = ornament(Apse, "")
        check(dark, "ornament-hi", o.hi)
        check(dark, "ornament-lo", o.lo)
    }

    @Test
    fun seasonsRetintTheGilding() {
        for ((season, css) in listOf("passiontide" to "body.season-passiontide", "eastertide" to "body.season-eastertide")) {
            val block = rule(css)
            val o = ornament(Nave, season)
            check(block, "ornament", o.flat)
            check(block, "ornament-line", o.line)
            check(block, "ornament-hi", o.hi)
            check(block, "ornament-lo", o.lo)
            check(block, "inscription-ink", o.ink)
            val dark = rule(":root[data-theme=\"dark\"] $css")
            val d = ornament(Apse, season)
            check(dark, "ornament", d.flat)
            check(dark, "ornament-line", d.line)
            check(dark, "ornament-hi", d.hi)
            check(dark, "ornament-lo", d.lo)
        }
    }
}
