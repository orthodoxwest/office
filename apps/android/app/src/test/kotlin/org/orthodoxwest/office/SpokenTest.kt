package org.orthodoxwest.office

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.orthodoxwest.office.core.BlockKind
import org.orthodoxwest.office.core.RunStyle
import org.orthodoxwest.office.core.hourNames

/** What a screen reader says for the office: every word, and none of the marks meant for the eye. */
class SpokenTest {
    private val blocks = listOf(3 to 15, 4 to 5, 12 to 25).flatMap { (month, day) ->
        hourNames().flatMap { hour -> Office.core.compose(hour, 2026, month, day, "priest").sections.flatMap { it.blocks } }
    }

    @Test
    fun theMarksForTheEyeAreSilent() {
        for (b in blocks) {
            val said = spoken(b)
            for (mark in listOf("*", "†", "℣", "℟", "✠", "")) assertTrue("$mark in: $said", mark !in said)
            assertTrue("empty: ${b.kind}", said.isNotBlank() || b.kind == BlockKind.GAP)
        }
    }

    @Test
    fun sigilsAreNamedAndTheCrossIsSaid() {
        val said = blocks.map(::spoken)
        assertTrue(said.any { it.startsWith("Versicle. ") })
        assertTrue(said.any { it.startsWith("Response. ") })
        assertTrue(said.any { it.startsWith("Antiphon. ") })
        val cross = said.filter { "sign of the cross" in it }
        assertTrue(cross.isNotEmpty())
        for (s in cross) assertTrue(s, ", ," !in s && ",." !in s && !s.endsWith(","))
    }

    @Test
    fun aPointedVerseReadsAsPlainWords() {
        val verse = blocks.first { b -> b.kind == BlockKind.VERSE && b.runs.any { it.text.contains('*') } && b.runs.none { it.style == RunStyle.POSTURE } }
        val printed = verse.runs.joinToString("") { it.text }
        // The mediant reads as the colon it stands for.
        val words = printed.substringBefore('*').trimEnd(' ', '\u00a0').trimEnd(':', ';', ',') + ": " + printed.substringAfter('*').trim()
        assertEquals(words.replace("†", ",").replace(Regex("[\\s\u00a0]+"), " ").replace(" ,", ","), spoken(verse))
    }

    @Test
    fun aPsalmLabelReadsAsTwoPhrases() {
        val label = blocks.first { it.kind == BlockKind.ITEM_LABEL && "·" in it.runs.joinToString("") { r -> r.text } }
        assertTrue(spoken(label), Regex("^Psalm \\d+\\. \\S").containsMatchIn(spoken(label)))
    }

    @Test
    fun aPostureCueIsAnAside() {
        val verse = blocks.first { b -> b.kind == BlockKind.VERSE && b.runs.any { it.style == RunStyle.POSTURE } }
        val cue = verse.runs.first { it.style == RunStyle.POSTURE }.text.trim()
        assertTrue(spoken(verse), "($cue)" in spoken(verse))
    }
}
