package org.orthodoxwest.office

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLayoutResult
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withLink
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.orthodoxwest.office.core.AboutPeriodView
import org.orthodoxwest.office.core.AboutRunView
import org.orthodoxwest.office.core.AboutView
import org.orthodoxwest.office.core.aboutNow
import java.time.LocalDate
import kotlin.math.max
import kotlin.math.min

/**
 * Each block's margins above and below, as style.css sets `.about`'s (the prose 0.8rem apart, the
 * tituli 1.8rem above). Between two blocks the larger stands, as CSS margins collapse.
 */
private val MARGINS: Map<String, Pair<Dp, Dp>> = mapOf(
    "intro" to (0.dp to 12.8.dp),
    "verse" to (19.2.dp to 0.dp),
    "heading" to (28.8.dp to 7.2.dp),
    "paragraph" to (0.dp to 12.8.dp),
    "hours" to (0.dp to 0.dp),
    "note" to (8.8.dp to 12.8.dp),
    "key" to (0.dp to 8.8.dp),
    "invitation" to (33.6.dp to 6.4.dp),
)

/**
 * About the Office, set as the web's /about: the reminders page's measure and headpiece, the
 * tituli in small capitals, the verse as red work, and the seven hours as home's table of
 * periods. The words are the core's (presentation::about), so every front says the same. A link
 * opens the app's own page where it has one (the ordo at today, the reminders), and the site in
 * the browser otherwise. The table lights the hour it is now as home's directory does, and the page
 * ends with home's invitation to it; both follow `clockHour`, which home's minute check keeps.
 */
@Composable
fun AboutScreen(
    view: AboutView,
    today: LocalDate,
    clockHour: Int,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onOpen: (Page) -> Unit,
) {
    val p = LocalPalette.current
    val context = LocalContext.current
    val site = remember { Usage.site(context.getSharedPreferences("office", Context.MODE_PRIVATE)) }
    val follow: (String) -> Unit = { target ->
        when (target) {
            "/calendar" -> onOpen(Page.Ordo(today.year, today.monthValue, today.dayOfMonth))
            "/reminders" -> onOpen(Page.Reminders)
            // The privacy policy and the issue tracker are the site's alone.
            else -> runCatching {
                context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(if (target.startsWith("/")) site + target else target)))
            }
        }
    }
    val body = Type.body.copy(lineHeight = 31.sp, color = p.text)
    // The hour it is now, as home has it (presentation::about): null only if the core refuses the date.
    val now = remember(today, clockHour) { runCatching { aboutNow(today.toCivil(), clockHour) }.getOrNull() }
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        chrome()
        Column(Modifier.measure().padding(top = 24.dp)) {
            PlainHeadpiece()
            Text(view.title, Modifier.padding(top = 8.8.dp).semantics { heading() }, style = body.copy(fontSize = 28.8.sp, lineHeight = 34.56.sp))
            var below = 8.8.dp
            view.blocks.forEach { b ->
                val (above, after) = MARGINS[b.kind] ?: (0.dp to 0.dp)
                Spacer(Modifier.height(maxOf(below, above)))
                below = after
                when (b.kind) {
                    "intro" -> Text(b.text, style = body)
                    "verse" -> Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(b.text, style = body.copy(color = p.rubric, fontStyle = FontStyle.Italic, textAlign = TextAlign.Center))
                        Text(
                            b.cite,
                            style = body.copy(
                                fontSize = 13.12.sp,
                                lineHeight = 20.34.sp,
                                letterSpacing = 0.79.sp,
                                color = p.rubric,
                                fontFeatureSettings = ALL_SMALL_CAPS,
                                textAlign = TextAlign.Center,
                            ),
                        )
                    }
                    // A titulus: small capitals, tracked, in the tituli's ink.
                    "heading" -> Text(
                        b.text,
                        Modifier.semantics { heading() },
                        style = body.copy(fontSize = 15.2.sp, lineHeight = 23.56.sp, letterSpacing = 1.22.sp, color = p.titulus, fontFeatureSettings = ALL_SMALL_CAPS),
                    )
                    "paragraph" -> LinkedText(b.runs, body, follow)
                    "hours" -> HoursTable(view.periods, now?.current.orEmpty()) { onOpen(Page.Hour(today, it)) }
                    // Home's invitation, still, a size down by width only (the web's `.about-door`):
                    // with no day of its own, its frontal is home's for a day without a colour, the lining.
                    "invitation" -> if (now != null) {
                        Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.TopCenter) {
                            Box(Modifier.widthIn(max = 280.dp)) {
                                PrayNow(now.prayNowLabel, 19.2f, 11.9.dp, 12.5.dp, p.lining, homeInk(p).tabletWash, null) {
                                    val d = now.prayNowDate
                                    onOpen(Page.Hour(LocalDate.of(d.year, d.month, d.day), now.prayNowHour))
                                }
                            }
                        }
                    }
                    "note" -> Text(b.text, style = body.copy(fontSize = 16.8.sp, lineHeight = 26.04.sp, color = p.muted))
                    // The mark first, in the rubrics' red where the page prints it so.
                    "key" -> Text(
                        buildAnnotatedString {
                            withStyle(SpanStyle(color = when (b.ink) { "rubric" -> p.rubric; "unsaid" -> p.unsaid; else -> p.text })) { append(b.mark) }
                            append(" ")
                            append(b.text)
                        },
                        style = body,
                    )
                }
            }
        }
        Footer()
    }
}

/**
 * A paragraph with its links in the accent, each underlined in the gold line a fifth of the type
 * below its baseline, as the web's `.about a` (a span's own underline takes only its text's colour).
 */
@Composable
private fun LinkedText(runs: List<AboutRunView>, style: TextStyle, onLink: (String) -> Unit) {
    val p = LocalPalette.current
    var layout by remember { mutableStateOf<TextLayoutResult?>(null) }
    val text = buildAnnotatedString {
        runs.forEach { run ->
            if (run.link.isEmpty()) {
                append(run.text)
            } else {
                withLink(LinkAnnotation.Clickable(run.link, TextLinkStyles(SpanStyle(color = p.accent))) { onLink(run.link) }) { append(run.text) }
            }
        }
    }
    Text(
        text,
        Modifier.drawBehind {
            val l = layout ?: return@drawBehind
            val drop = style.fontSize.toPx() * 0.2f
            val w = 1.dp.toPx()
            text.getLinkAnnotations(0, text.length).forEach { link ->
                if (link.end <= link.start) return@forEach
                val first = l.getLineForOffset(link.start)
                val last = l.getLineForOffset(link.end - 1)
                for (line in first..last) {
                    val x0 = if (line == first) l.getBoundingBox(link.start).left else l.getLineLeft(line)
                    val x1 = if (line == last) l.getBoundingBox(link.end - 1).right else l.getLineRight(line)
                    val y = l.getLineBaseline(line) + drop
                    drawLine(p.goldLine, Offset(min(x0, x1), y), Offset(max(x0, x1), y), w)
                }
            }
        },
        style = style,
        onTextLayout = { layout = it },
    )
}

/** A period's ornament, by the label the core gives it. */
private fun periodOf(label: String): Period = when (label.lowercase()) {
    "morning" -> Period.MORNING
    "day" -> Period.DAY
    else -> Period.EVENING
}

/**
 * The hours table's ink, as home's frontispiece first ruled it (`.about` in style.css): the rules
 * within, the period cells' wash (the frieze's green earth, thinned) and the frame, the lining
 * thinned.
 */
private class HoursInk(val rule: Color, val band: Color, val panelRule: Color)

private fun hoursInk(p: Palette): HoursInk = if (p.dark) {
    HoursInk(Color(208, 176, 106).copy(alpha = 0.24f), Color(208, 176, 106).copy(alpha = 0.045f), p.lining.copy(alpha = 0.45f))
} else {
    HoursInk(Color(107, 58, 31).copy(alpha = 0.22f), p.inscriptionGround.copy(alpha = 0.09f), p.lining.copy(alpha = 0.45f))
}

/**
 * The seven hours in home's three periods: each period's ornament and label in the frontispiece's
 * wash, then its hours, each name opening that hour today, with what it is in italic and when it
 * is said beneath. Framed at the sides and ends in the lining thinned, ruled within in the
 * frontispiece's ink, as home's directory. The hour it is now (`current`, empty from midnight to
 * 2 am as on home) lies under home's pool of light laid along its row, its name in the tituli's ink.
 */
@Composable
private fun HoursTable(periods: List<AboutPeriodView>, current: String, onHour: (String) -> Unit) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val ink = hoursInk(p)
    val labelStyle = Type.label(11.52f, 0.08f).copy(lineHeight = 13.82.sp, color = p.muted, fontFeatureSettings = ALL_SMALL_CAPS)
    val nameStyle = Type.body.copy(fontSize = 19.2.sp, lineHeight = 24.96.sp, color = p.text)
    val glossStyle = Type.body.copy(fontSize = 17.6.sp, lineHeight = 22.88.sp, color = p.text, fontStyle = FontStyle.Italic)
    val timeStyle = Type.body.copy(fontSize = 14.4.sp, lineHeight = 18.72.sp, color = p.muted)
    // The label column is the web's 5.2rem, widened only when the reader's font size needs it; the
    // names stand in one column as wide as the widest of the seven, measured, as on every front, so
    // the rows line up and Prime's gloss keeps one line.
    val measurer = rememberTextMeasurer()
    val density = LocalDensity.current
    val labelWidth = remember(periods, labelStyle, density) {
        with(density) { periods.maxOf { measurer.measure(it.label, labelStyle).size.width }.toDp() + 12.8.dp }.coerceAtLeast(83.2.dp)
    }
    val nameWidth = remember(periods, nameStyle, density) {
        with(density) { periods.flatMap { it.hours }.maxOf { measurer.measure(it.name, nameStyle).size.width }.toDp() + 1.dp }
    }
    val pool = homeInk(p).nowPool
    // About keeps no season of its own.
    val nameNow = titulusNow(p, "")
    Column(
        Modifier.fillMaxWidth().drawBehind {
            val w = 1.dp.toPx()
            drawLine(ink.panelRule, Offset(w / 2f, 0f), Offset(w / 2f, size.height), w)
            drawLine(ink.panelRule, Offset(size.width - w / 2f, 0f), Offset(size.width - w / 2f, size.height), w)
        },
    ) {
        periods.forEachIndexed { i, period ->
            Hairline(if (i == 0) ink.panelRule else ink.rule)
            Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min)) {
                Column(
                    Modifier.width(labelWidth).fillMaxHeight().background(ink.band).padding(horizontal = 6.4.dp, vertical = 6.dp).semantics(mergeDescendants = true) { heading() },
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center,
                ) {
                    PeriodIcon(periodOf(period.label), o.flat)
                    Text(period.label, Modifier.padding(top = 2.56.dp), softWrap = false, style = labelStyle)
                }
                Box(Modifier.width(1.dp).fillMaxHeight().background(ink.rule))
                Column(Modifier.weight(1f)) {
                    period.hours.forEachIndexed { j, h ->
                        if (j > 0) Hairline(ink.rule)
                        // The whole row opens the hour: a thumb's target, read as one. Pressed, it
                        // lies under home's pool of light, a long lozenge under its words.
                        val isNow = h.hour == current
                        Row(
                            Modifier.fillMaxWidth()
                                .then(if (isNow) Modifier.poolAlongRow(pool).semantics { stateDescription = "now" } else Modifier)
                                .tap(action = "open ${h.name}", lit = Lit(pool, PoolShape.Row)) { onHour(h.hour) }
                                .padding(horizontal = 13.6.dp, vertical = 8.dp),
                        ) {
                            Text(h.name, Modifier.width(nameWidth).alignByBaseline(), softWrap = false, style = if (isNow) nameStyle.copy(color = nameNow) else nameStyle)
                            Spacer(Modifier.width(12.dp))
                            Column(Modifier.weight(1f).alignByBaseline()) {
                                Text(h.gloss, style = glossStyle)
                                Text(h.time, style = timeStyle)
                            }
                        }
                    }
                }
            }
        }
        Hairline(ink.panelRule)
    }
}

/**
 * Home's pool of light ([poolBrush]) laid along an About row, as the web's
 * `.about-hour.is-current::before`: an ellipse half the row wide and nearly its height, brightest
 * behind the name (29% across, a little under the middle), feathered in from the row's start so
 * no edge of it lies on a rule.
 */
private fun Modifier.poolAlongRow(pool: Color): Modifier = drawBehind {
    val rx = size.width * 0.5f
    val ry = size.height * 0.46f
    if (rx <= 0f || ry <= 0f) return@drawBehind
    val c = Offset(size.width * 0.29f, size.height * 0.46f)
    drawContext.canvas.saveLayer(Rect(Offset.Zero, size), Paint())
    scale(1f, ry / rx, pivot = c) { drawCircle(poolBrush(pool, c, rx), rx, c) }
    drawRect(Brush.horizontalGradient(0f to Color.Transparent, 0.14f to Color.Black, 1f to Color.Black), blendMode = BlendMode.DstIn)
    drawContext.canvas.restore()
}
