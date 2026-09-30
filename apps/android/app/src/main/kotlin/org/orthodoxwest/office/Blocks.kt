package org.orthodoxwest.office

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextIndent
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp
import org.orthodoxwest.office.core.BlockKind
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.RunStyle

/** Width of the gutter that holds verse numbers and ℣/℟ sigils (the web's 1.8rem). */
private val Gutter: Dp = 30.dp

private const val SMALL_CAPS = "smcp, c2sc"

/** One block of a composed hour, styled after the web's classes for the same text. */
@Composable
fun Block(block: BlockView, modifier: Modifier = Modifier) {
    val p = LocalPalette.current
    val body = MaterialTheme.typography.bodyLarge
    val small = body.copy(fontSize = 16.sp, lineHeight = 22.sp)
    when (block.kind) {
        BlockKind.GAP -> Spacer(modifier.height(8.dp))
        BlockKind.HEADING -> Text(
            runs(block),
            modifier.fillMaxWidth().padding(top = 22.dp, bottom = 8.dp),
            style = body.copy(color = p.rubric, fontFeatureSettings = SMALL_CAPS, letterSpacing = 0.06.em, textAlign = TextAlign.Center),
        )
        BlockKind.COMMEMORATION_HEADING -> Text(
            runs(block),
            modifier.fillMaxWidth().padding(top = 22.dp, bottom = 8.dp),
            style = body.copy(color = p.rubric, textAlign = TextAlign.Center),
        )
        BlockKind.ITEM_LABEL -> Text(
            runs(block),
            modifier.fillMaxWidth().padding(top = 14.dp, bottom = 4.dp),
            // Lining figures: Garamond's old-style "111" reads as the numeral III.
            style = body.copy(color = p.rubric, fontFeatureSettings = "lnum", textAlign = TextAlign.Center),
        )
        BlockKind.LATIN_TITLE -> Text(
            runs(block),
            modifier.fillMaxWidth().padding(bottom = 8.dp),
            style = body.copy(color = p.muted, fontStyle = FontStyle.Italic, textAlign = TextAlign.Center),
        )
        BlockKind.CHAPTER_REF, BlockKind.SCRIPTURE_REF -> Text(
            runs(block),
            modifier.fillMaxWidth().padding(bottom = 6.dp),
            style = small.copy(color = p.rubric, fontFeatureSettings = "lnum", textAlign = TextAlign.Center),
        )
        BlockKind.CANTICLE_SECTION -> Text(
            runs(block),
            modifier.fillMaxWidth().padding(vertical = 8.dp),
            style = body.copy(color = p.muted, fontStyle = FontStyle.Italic, textAlign = TextAlign.Center),
        )
        BlockKind.RUBRIC -> Text(
            runs(block),
            modifier.fillMaxWidth().padding(vertical = 4.dp),
            style = body.copy(color = p.rubric, fontSize = 17.sp, lineHeight = 24.sp),
        )
        BlockKind.SPEAKER -> Text(
            runs(block),
            modifier.fillMaxWidth().padding(top = 6.dp),
            style = small.copy(color = p.rubric, fontFeatureSettings = SMALL_CAPS, letterSpacing = 0.055.em),
        )
        BlockKind.ANTIPHON -> Text(
            buildAnnotatedString {
                withStyle(SpanStyle(color = p.rubric, fontStyle = FontStyle.Italic)) { append(block.marker) }
                append(" ")
                append(runs(block))
            },
            modifier.fillMaxWidth().padding(vertical = 4.dp),
            style = body,
        )
        BlockKind.VERSE -> Gutter(block.marker, markerStyle = small.copy(color = p.muted, fontSize = 14.sp), modifier = modifier) {
            Text(runs(block), style = body.copy(textIndent = TextIndent(restLine = 18.sp)))
        }
        BlockKind.VERSICLE, BlockKind.RESPONSE, BlockKind.ALL -> Gutter(block.marker, markerStyle = body.copy(color = p.rubric), modifier = modifier) {
            Text(runs(block), style = body)
        }
        BlockKind.STANZA -> Text(runs(block), modifier.fillMaxWidth().padding(bottom = 12.dp), style = body)
        BlockKind.PARAGRAPH, BlockKind.CHANT_LINE -> Text(runs(block), modifier.fillMaxWidth().padding(vertical = 2.dp), style = body)
    }
}

/** Text beside a right-aligned marker in the gutter; a spelled-out marker ("Blessing.") sits above instead. */
@Composable
private fun Gutter(marker: String, markerStyle: TextStyle, modifier: Modifier, content: @Composable () -> Unit) {
    if (marker.length > 3) {
        Column(modifier.fillMaxWidth().padding(vertical = 2.dp)) {
            Text(marker, style = markerStyle)
            Row { Spacer(Modifier.width(Gutter)); content() }
        }
        return
    }
    Row(modifier.fillMaxWidth().padding(vertical = 2.dp), horizontalArrangement = Arrangement.Start) {
        Text(marker, Modifier.width(Gutter).padding(end = 6.dp).alignByBaseline(), style = markerStyle.copy(textAlign = TextAlign.End))
        Box(Modifier.alignByBaseline()) { content() }
    }
}

/** A block's runs as styled text; a drop cap enlarges and reddens the first letter. */
@Composable
private fun runs(block: BlockView): AnnotatedString {
    val p = LocalPalette.current
    return buildAnnotatedString {
        var capPending = block.dropCap
        for (run in block.runs) {
            val style = when (run.style) {
                RunStyle.PLAIN, RunStyle.BREAK -> null
                RunStyle.MEDIANT -> SpanStyle(color = p.rubric)
                RunStyle.CROSS -> SpanStyle(color = p.rubric, fontFamily = CrossFont)
                RunStyle.PRAYED -> SpanStyle(color = p.text)
                RunStyle.SECRET -> SpanStyle(color = p.muted)
                RunStyle.LATIN -> SpanStyle(fontStyle = FontStyle.Italic, color = p.muted)
                RunStyle.KICKER -> SpanStyle(fontSize = 0.8.em, fontStyle = FontStyle.Italic, color = p.muted)
            }
            var text = run.text
            if (capPending && run.style == RunStyle.PLAIN && text.isNotBlank()) {
                capPending = false
                val lead = text.indexOfFirst { it.isLetter() }.coerceAtLeast(0)
                append(text.substring(0, lead))
                withStyle(SpanStyle(color = p.rubric, fontSize = 1.6.em)) { append(text[lead]) }
                text = text.substring(lead + 1)
            }
            if (style == null) append(text) else withStyle(style) { append(text) }
        }
    }
}
