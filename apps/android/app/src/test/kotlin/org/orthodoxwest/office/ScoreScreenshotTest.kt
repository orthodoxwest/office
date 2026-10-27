package org.orthodoxwest.office

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.unit.dp
import com.github.takahirom.roborazzi.captureRoboImage
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.orthodoxwest.office.core.BlockKind
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

/**
 * The Compline hymn with a tune over its first stanza, as a `BlockKind.SCORE` block draws it
 * through neuma's bindings in the same library, for review beside the web's `.chant-score` in
 * both themes. The melody is a placeholder; no score is in the corpus yet.
 */
@RunWith(RobolectricTestRunner::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w390dp-h844dp-xxhdpi")
class ScoreScreenshotTest {
    @get:Rule
    val compose = createComposeRule()

    private val gabc = "name: Te lucis;\n%%\n(c4) To(e) thee,(f) be(h)fore(g) the(h) close(i) of(h) day,(g.) (;) " +
        "Cre(h)a(i)tor(j) of(i) the(h) world,(ih) we(g) pray(h.) (;) That(h) with(hg) thy(f) won(g)ted(h) fa(g)vour,(f) thou(e.) (;) " +
        "Wouldst(e) be(f) our(g) guard(h) and(gf) keep(e)er(f) now.(e.) (::)"

    private fun hymn(theme: ThemeChoice, name: String) {
        val blocks = Office.core.compose("compline", 2026, 10, 7, "private").sections.flatMap { it.blocks }
        val first = blocks.indexOfFirst { it.kind == BlockKind.STANZA }
        val shown = blocks.subList(first - 1, first + 2).toMutableList()
        shown[1] = shown[1].copy(kind = BlockKind.SCORE, score = gabc)
        compose.setContent {
            OfficeTheme(choice = theme) {
                Column(Modifier.padding(16.dp)) {
                    shown.forEachIndexed { i, b -> Block(b, Modifier.padding(top = gapBefore(shown.getOrNull(i - 1), b))) }
                }
            }
        }
        compose.waitForIdle()
        compose.onRoot().captureRoboImage("build/screenshots/score-$name.png")
    }

    @Test fun nave() = hymn(ThemeChoice.NAVE, "nave")
    @Test fun apse() = hymn(ThemeChoice.APSE, "apse")

    /** neuma engraves the score through the app's own library: staves, notes and the words. */
    @Test
    fun engraves() {
        val page = engrave(gabc, initial = true, width = 1000f, lyric = 60f)
        assertTrue(page.height > 0f && page.items.size > 40)
        assertTrue(page.altText, page.altText.startsWith("To thee, before the close of day"))
    }
}
