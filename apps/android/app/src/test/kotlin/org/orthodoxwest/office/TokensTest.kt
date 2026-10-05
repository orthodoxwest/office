package org.orthodoxwest.office

import androidx.compose.ui.graphics.Color
import java.io.File
import kotlin.math.abs
import kotlin.math.hypot
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

    private fun value(block: String, token: String): String? =
        Regex("(?m)^\\s*--" + Regex.escape(token) + ":\\s*([^;]+);").find(block)?.groupValues?.get(1)

    private fun check(block: String, token: String, color: Color) {
        val value = value(block, token) ?: error("--$token missing")
        val want = parse(value)
        val got = floatArrayOf(color.red * 255f, color.green * 255f, color.blue * 255f, color.alpha)
        for (i in 0..2) assertTrue("--$token: $value vs $color", abs(want[i] - got[i]) < 0.6f)
        assertEquals("--$token alpha", want[3], got[3], 0.01f)
    }

    /** A token the stylesheet declares as another's value (`--moon-ink: var(--gold)`). */
    private fun alias(block: String, token: String, target: String) {
        assertEquals("--$token", "var(--$target)", value(block, token)?.trim() ?: error("--$token missing"))
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
        check(block, "material-highlight", p.materialHighlight)
        check(block, "inscription-ground", p.inscriptionGround)
        check(block, "inscription-edge", p.inscriptionEdge)
        check(block, "inscription-wash", p.inscriptionWash)
        check(block, "lining", p.lining)
        check(block, "titulus", p.titulus)
        check(block, "kalendar-blue", p.kalendarBlue)
    }

    private val root get() = rule(":root").takeIf { it.contains("--text:") } ?: error(":root tokens")

    @Test
    fun naveIsTheRoot() {
        checkPalette(root, Nave)
        check(root, "oak", Nave.oak)
        // The moon is Nave's gold.
        alias(root, "moon-ink", "gold")
        assertEquals(Nave.gold, Nave.moonInk)
        val o = ornament(Nave, "")
        // Out of season the gilding is the gold, and the ornament's line the painted lining.
        alias(root, "ornament", "gold")
        assertEquals(Nave.gold, o.flat)
        alias(root, "ornament-line", "lining")
        assertEquals(Nave.lining, o.line)
        check(root, "ornament-hi", o.hi)
        check(root, "ornament-lo", o.lo)
        check(root, "inscription-ink", o.ink)
    }

    @Test
    fun apseIsTheDarkTheme() {
        val dark = rule(":root[data-theme=\"dark\"]")
        checkPalette(dark, Apse)
        // The beam is the root's oak in both themes.
        assertEquals(null, value(dark, "oak"))
        check(root, "oak", Apse.oak)
        check(dark, "moon-ink", Apse.moonInk)
        val o = ornament(Apse, "")
        // The root's aliases resolve against Apse's own gold and lining.
        assertEquals(null, value(dark, "ornament"))
        assertEquals(Apse.gold, o.flat)
        assertEquals(null, value(dark, "ornament-line"))
        assertEquals(Apse.lining, o.line)
        check(dark, "ornament-hi", o.hi)
        check(dark, "ornament-lo", o.lo)
        // Apse declares no inscription ink of its own: its band is lettered in the root's.
        assertEquals(null, value(dark, "inscription-ink"))
        check(root, "inscription-ink", o.ink)
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
            // The season's inscription ink is set on the body in both themes.
            assertEquals(null, value(dark, "inscription-ink"))
            check(block, "inscription-ink", d.ink)
        }
    }

    /** Home's pointed heads are the web's, as tools/genarch.py writes them for both, in its order. */
    @Test
    fun arches() {
        val arches = listOf(PhoneArch, TallArch, TallerArch, NicheArch)
        val rises = Regex("--arch-rise: ([0-9.]+);").findAll(css).map { it.groupValues[1].toFloat() }.toList()
        assertEquals(rises, arches.map { it.rise })
        for (a in arches) {
            // Each upper arc leaves its haunch at a tangent and passes through the point.
            assertEquals(a.radius, a.haunch + hypot(a.centre - a.haunch, a.depth), 1e-5f)
            assertEquals(a.radius, hypot(a.centre - 0.5f, a.rise + a.depth), 1e-5f)
        }
    }
}
