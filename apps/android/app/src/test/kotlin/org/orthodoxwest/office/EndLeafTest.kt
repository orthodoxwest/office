package org.orthodoxwest.office

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.orthodoxwest.office.core.EndLeaf
import org.orthodoxwest.office.core.endLeafNext
import org.orthodoxwest.office.core.endLeafOpens

/** The cross under the last Amen takes its gold by the core's rule, once it stands wholly on screen. */
class EndLeafTest {
    @Test
    fun theCrossStandsWholeOnlyBetweenTheTopAndTheFoot() {
        // 40 high, the list's foot (above the system bar) at 800.
        assertTrue(whollyShown(0, 40, 800))
        assertTrue(whollyShown(760, 40, 800))
        // Its top 8 peeping in at the foot, where the hour is marked: not whole.
        assertFalse(whollyShown(792, 40, 800))
        // Its foot under the system bar, or its top under the status bar: not whole.
        assertFalse(whollyShown(761, 40, 800))
        assertFalse(whollyShown(-1, 40, 800))
    }

    @Test
    fun theLeafWaitsForTheMarkAndTheWholeCrossAndComesOnce() {
        val open = endLeafOpens(bare = false, prayed = false)
        assertEquals(EndLeaf.PAINT, open)
        assertEquals(EndLeaf.PAINT, endLeafNext(open, bare = false, prayed = true, whollyShown = false))
        assertEquals(EndLeaf.PAINT, endLeafNext(open, bare = false, prayed = false, whollyShown = true))
        assertEquals(EndLeaf.GILDING, endLeafNext(open, bare = false, prayed = true, whollyShown = true))
        assertEquals(EndLeaf.GILDING, endLeafNext(EndLeaf.GILDING, bare = false, prayed = true, whollyShown = false))
        // Prayed before: gilt from the first paint. Bare: never.
        assertEquals(EndLeaf.GILT, endLeafOpens(bare = false, prayed = true))
        assertEquals(EndLeaf.PAINT, endLeafOpens(bare = true, prayed = true))
        assertEquals(EndLeaf.PAINT, endLeafNext(EndLeaf.PAINT, bare = true, prayed = true, whollyShown = true))
    }
}
