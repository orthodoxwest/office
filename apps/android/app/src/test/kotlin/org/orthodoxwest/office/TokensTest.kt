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

    /** The system's font scale reaches the type no further than iOS lets Dynamic Type (Theme.swift). */
    @Test
    fun fontScaleStopsWhereIosDoes() {
        val swift = File("../../ios/Office/Theme.swift").readText()
        val cap = Regex("""return min\(body / 17, ([0-9.]+)\)""").find(swift)?.groupValues?.get(1)?.toFloat() ?: error("no cap in Theme.swift")
        assertEquals(cap, MaxFontScale, 0f)
    }

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
        for ((season, css) in listOf("quiet" to "body.season-quiet", "passiontide" to "body.season-passiontide", "eastertide" to "body.season-eastertide")) {
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
            // Apse's gilt tituli take the season's gilding; Nave's painted ones stay. Nave's gold
            // moon follows the season; Apse's silver goes quiet and veils with the gilding, and is
            // restated at Eastertide.
            check(dark, "titulus", seasoned(Apse, season).titulus)
            assertEquals(Nave.titulus, seasoned(Nave, season).titulus)
            alias(block, "moon-ink", "ornament")
            assertEquals(o.flat, seasoned(Nave, season).moonInk)
            if (season == "eastertide") check(dark, "moon-ink", seasoned(Apse, season).moonInk)
            else assertEquals(d.flat, seasoned(Apse, season).moonInk)
        }
    }

    /** Home's painted page (HomePaint.kt) takes the web's "Home: the painted page" tokens. */
    @Test
    fun homeInk() {
        check(root, "ochre", Ochre)
        check(root, "leaf-matte-hi", LeafMatteHi)
        check(root, "leaf-matte", LeafMatte)
        check(root, "leaf-matte-lo", LeafMatteLo)
        check(root, "leaf-shadow", LeafShadow)
        // Nave's wash and pool are thin ochre: 19% and 34% (`color-mix(... var(--ochre) N%, transparent)`).
        assertEquals("color-mix(in srgb, var(--ochre) 19%, transparent)", value(root, "tablet-wash")?.trim())
        assertEquals("color-mix(in srgb, var(--ochre) 34%, transparent)", value(root, "now-pool")?.trim())
        assertEquals(Ochre.copy(alpha = 0.19f), homeInk(Nave).tabletWash)
        assertEquals(Ochre.copy(alpha = 0.34f), homeInk(Nave).nowPool)
        val dark = rule(":root[data-theme=\"dark\"]")
        val ink = homeInk(Apse)
        check(dark, "tablet-wash", ink.tabletWash)
        check(dark, "now-pool", ink.nowPool)
        check(dark, "now-gloss", ink.nowGloss)
        check(dark, "sky-deep", ink.skyDeep)
        check(dark, "sky", ink.sky)
        check(dark, "sky-low", ink.skyLow)
        check(dark, "sky-rim", ink.skyRim)
        check(dark, "lime-white", ink.limeWhite)
        check(dark, "sky-muted", ink.skyMuted)
        check(dark, "sky-rubric", ink.skyRubric)
        check(dark, "lining-lifted", ink.liningLifted)
        // The current hour's name is lifted by night in the quiet seasons and Passiontide.
        for (season in listOf("quiet", "passiontide")) {
            check(rule(":root[data-theme=\"dark\"] body.season-$season"), "titulus-now", titulusNow(Apse, season))
        }
        assertEquals(Apse.titulus, titulusNow(Apse, "eastertide"))
        assertEquals(Nave.titulus, titulusNow(Nave, "quiet"))
    }

    /** Home's pointed heads are the web's, as tools/genarch.py writes them for both, in its order. */
    @Test
    fun arches() {
        val arches = listOf(PhoneArch, TallArch, TallerArch, NicheArch, UprightArch)
        val rises = Regex("--arch-rise: ([0-9.]+);").findAll(css).map { it.groupValues[1].toFloat() }.toList()
        assertEquals(rises, arches.map { it.rise })
        for (a in arches) {
            // Each upper arc leaves its haunch at a tangent and passes through the point.
            assertEquals(a.radius, a.haunch + hypot(a.centre - a.haunch, a.depth), 1e-5f)
            assertEquals(a.radius, hypot(a.centre - 0.5f, a.rise + a.depth), 1e-5f)
        }
    }

    private val swift = File("../../ios/Office/Niche.swift").readText()

    /**
     * The niche's zoom figures are the web's: each `NicheFigures.<name>` rule in style.css, the
     * Kotlin figures and Niche.swift's, and the unzoomed header and colophon round them.
     */
    @Test
    fun nicheFigures() {
        val kotlin = mapOf(
            "landscape" to NicheFigures.Landscape,
            "landscapeList" to NicheFigures.LandscapeList,
            "upright" to NicheFigures.Upright,
            "uprightList" to NicheFigures.UprightList,
            "short" to NicheFigures.Short,
        )
        fun number(block: String, token: String) = (value(block, token) ?: error("--$token missing")).trim().removeSuffix("rem").toFloat()
        val swiftFigures = Regex("static let (\\w+) = NicheFigures\\(card: ([0-9.]+), fit: ([0-9.]+), air: ([0-9.]+), least: ([0-9.]+), share: ([0-9.]+)\\)")
            .findAll(swift).associate { m -> m.groupValues[1] to m.groupValues.drop(2).map { it.toFloat() } }
        assertEquals(kotlin.keys, swiftFigures.keys)
        for ((name, f) in kotlin) {
            val at = css.indexOf("/* NicheFigures.$name */").also { assertTrue("no NicheFigures.$name rule", it >= 0) }
            val block = css.substring(at, css.indexOf("}", at))
            val web = listOf(number(block, "card-w"), number(block, "niche-fit"), number(block, "niche-air"), number(block, "niche-least"), number(block, "niche-share"))
            val app = listOf(f.card, f.fit, f.air, f.least, f.share)
            assertEquals("NicheFigures.$name", web, app)
            assertEquals("Niche.swift's $name", web, swiftFigures[name])
        }
        val main = css.substring(css.indexOf("--niche-above:") - 40, css.indexOf("/* NicheFigures.landscape */"))
        assertEquals(NicheFigures.ABOVE, number(main, "niche-above"))
        assertEquals(NicheFigures.BELOW, number(main, "niche-below"))
        assertTrue(swift.contains("static let above: CGFloat = ${NicheFigures.ABOVE}"))
        assertTrue(swift.contains("static let below: CGFloat = ${NicheFigures.BELOW}"))
        // The list on a screen on its side, and the short screen, from the same heights.
        assertTrue(css.contains("(min-width: 701px) and (min-height: ${NicheFigures.LIST_LANDSCAPE.toInt()}px)"))
        assertTrue(css.contains("(min-width: 701px) and (max-height: ${NicheFigures.SHORT.toInt()}px)"))
        assertTrue(swift.contains("static let listLandscape: CGFloat = ${NicheFigures.LIST_LANDSCAPE.toInt()}"))
        assertTrue(swift.contains("static let shortHeight: CGFloat = ${NicheFigures.SHORT.toInt()}"))
    }

    /** The niche's reveal by day is the web's `--reveal-*`, in Kotlin and in Niche.swift. */
    @Test
    fun reveal() {
        val block = css.substring(css.indexOf("--reveal-shade:") - 200, css.indexOf("--reveal-cast:") + 60)
        val t = nicheTokens(Nave)
        val tokens = mapOf(
            "reveal-shade" to ("revealShade" to t.revealShade),
            "reveal-mid" to ("revealMid" to t.revealMid),
            "reveal-lit" to ("revealLit" to t.revealLit),
            "reveal-sill" to ("revealSill" to t.revealSill),
            "reveal-sill-edge" to ("revealSillEdge" to t.revealSillEdge),
            "reveal-soffit" to ("revealSoffit" to t.revealSoffit),
            "reveal-arris" to ("arris" to t.arris),
            "reveal-arris-turn" to ("arrisTurn" to t.arrisTurn),
            "reveal-arris-shade" to ("arrisShade" to t.arrisShade),
            "reveal-cast" to ("cast" to t.cast),
        )
        for ((token, pair) in tokens) {
            val (swiftName, color) = pair
            check(block, token, color)
            // Niche.swift: `var revealShade = Color(hex: 0xCBBCAA)` or `Color(rgb: 96, 66, 44, 0.3)`.
            val decl = Regex("var $swiftName = Color\\((hex: 0x([0-9A-Fa-f]{6})|rgb: ([0-9]+), ([0-9]+), ([0-9]+), ([0-9.]+))\\)").find(swift)
                ?: error("Niche.swift: no $swiftName")
            val g = decl.groupValues
            val got = if (g[2].isNotEmpty()) {
                val hex = g[2].toInt(16)
                floatArrayOf((hex shr 16 and 255).toFloat(), (hex shr 8 and 255).toFloat(), (hex and 255).toFloat(), 1f)
            } else {
                floatArrayOf(g[3].toFloat(), g[4].toFloat(), g[5].toFloat(), g[6].toFloat())
            }
            val want = parse(value(block, token)!!)
            for (i in 0..3) assertEquals("Niche.swift $swiftName vs --$token", want[i], got[i], 0.01f)
        }
    }
}
