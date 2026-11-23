package org.orthodoxwest.office

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
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
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.BlendMode
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.Paint
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.clipPath
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.drawscope.translate
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.imageResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
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
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.orthodoxwest.office.core.AboutBlockView
import org.orthodoxwest.office.core.AboutHourView
import org.orthodoxwest.office.core.AboutOrdoColorView
import org.orthodoxwest.office.core.AboutPeriodView
import org.orthodoxwest.office.core.AboutRunView
import org.orthodoxwest.office.core.AboutSpecimenView
import org.orthodoxwest.office.core.AboutView
import org.orthodoxwest.office.core.BlockKind
import org.orthodoxwest.office.core.BlockView
import org.orthodoxwest.office.core.CourseMonthView
import org.orthodoxwest.office.core.RunStyle
import org.orthodoxwest.office.core.RunView
import org.orthodoxwest.office.core.Space
import org.orthodoxwest.office.core.aboutNow
import org.orthodoxwest.office.core.colorDaysNote
import org.orthodoxwest.office.core.courseSpan
import org.orthodoxwest.office.core.rareColorMaxDays
import java.time.LocalDate
import kotlin.math.atan2
import kotlin.math.cos
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.math.sin

/** Width from which About sets the Horologium, hours and invitation as a right rail beside the prose. */
private val AboutRailFrom = 840.dp

/**
 * About the Office, set as the web's `/about`: the feast page's painted frontispiece head, an
 * illuminated drop-cap introduction, the interactive Horologium over an open hand-ruled hours
 * directory, a live miniature office specimen and ruled key in Reading the page, chronological
 * marginalia, and the current year's liturgical color ribbon in The Ordo.
 */
@Composable
fun AboutScreen(
    view: AboutView,
    today: LocalDate,
    clockHour: Int,
    chrome: @Composable () -> Unit,
    insets: PaddingValues,
    onOpen: (Page) -> Unit,
    course: List<CourseMonthView>? = null,
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
    val now = remember(today, clockHour) { runCatching { aboutNow(today.toCivil(), clockHour) }.getOrNull() }
    val currentHour = now?.current.orEmpty()

    // Extract head epigraph, intro, hours heading/note, and remaining prose sections from shared blocks.
    val verse = remember(view) { view.blocks.firstOrNull { it.kind == "verse" } }
    val intro = remember(view) { view.blocks.firstOrNull { it.kind == "intro" }?.text.orEmpty() }
    val parsed = remember(view) { partitionAboutBlocks(view.blocks) }

    // Interactive states shared across the Horologium + hours list, specimen + key, and Ordo ribbon.
    var pinnedHour by remember { mutableStateOf<String?>(null) }
    var specimenForm by remember { mutableStateOf("private") }
    var litKey by remember { mutableStateOf<String?>(null) }
    var selectedColor by remember { mutableStateOf<String?>(null) }

    val hoursColumn: @Composable (Modifier) -> Unit = { mod ->
        Column(mod) {
            if (parsed.hoursHeading.isNotEmpty()) {
                AboutSectionHeading(parsed.hoursHeading, seed = 110)
            }
            Horologium(
                periods = view.periods,
                current = currentHour,
                pinned = pinnedHour,
                onSelect = { slug -> pinnedHour = if (pinnedHour == slug) null else slug },
                onPin = { slug -> pinnedHour = slug },
            )
            HoursDirectory(
                periods = view.periods,
                current = currentHour,
                preview = pinnedHour?.takeIf { it != currentHour },
                onHour = { onOpen(Page.Hour(today, it)) },
            )
            if (parsed.hoursNote.isNotEmpty()) {
                Text(
                    parsed.hoursNote,
                    Modifier.padding(top = 10.4.dp),
                    style = body.copy(fontSize = 15.04.sp, lineHeight = 23.sp, color = p.muted, fontStyle = FontStyle.Italic),
                )
            }
        }
    }

    val doorBlock: @Composable (Boolean) -> Unit = { fillWidth ->
        if (now != null) {
            Box(
                Modifier.fillMaxWidth().padding(top = if (fillWidth) 21.6.dp else 33.6.dp, bottom = 6.4.dp),
                contentAlignment = Alignment.TopCenter,
            ) {
                Box(if (fillWidth) Modifier.fillMaxWidth() else Modifier.widthIn(max = 280.dp)) {
                    PrayNow(now.prayNowLabel, 19.2f, 11.9.dp, 12.5.dp, p.lining, homeInk(p).tabletWash, null) {
                        val d = now.prayNowDate
                        onOpen(Page.Hour(LocalDate.of(d.year, d.month, d.day), now.prayNowHour))
                    }
                }
            }
        }
    }

    val introBlock: @Composable () -> Unit = {
        if (intro.isNotEmpty()) {
            val dropCapBlock = remember(intro) {
                BlockView(
                    kind = BlockKind.PARAGRAPH,
                    marker = "",
                    dropCap = true,
                    startsElement = true,
                    runs = listOf(RunView(text = intro, style = RunStyle.PLAIN, gloss = null)),
                    score = "",
                    phrases = false,
                    space = Space.NONE,
                )
            }
            Opening(
                dropCapBlock,
                body,
                Modifier.padding(top = 4.dp).clearAndSetSemantics { contentDescription = intro },
                textStart = 0.dp,
            )
        }
    }

    val proseSections: @Composable () -> Unit = {
        parsed.sections.forEachIndexed { idx, sec ->
            Column(Modifier.padding(top = 33.6.dp)) {
                AboutSectionHeading(sec.heading, seed = 111 + idx)
                var i = 0
                while (i < sec.blocks.size) {
                    val b = sec.blocks[i]
                    when (b.kind) {
                        "paragraph" -> {
                            LinkedText(b.runs, body, follow)
                            if (i + 1 < sec.blocks.size) Spacer(Modifier.height(14.4.dp))
                            i++
                        }
                        "note" -> {
                            Text(b.text, style = body.copy(fontSize = 15.04.sp, lineHeight = 23.sp, color = p.muted, fontStyle = FontStyle.Italic))
                            if (i + 1 < sec.blocks.size) Spacer(Modifier.height(14.4.dp))
                            i++
                        }
                        "specimen" -> {
                            SpecimenLeaf(
                                specimen = view.specimen,
                                form = specimenForm,
                                litKey = litKey,
                                onForm = { specimenForm = it },
                                onToggleKey = { key -> litKey = if (litKey == key) null else key },
                            )
                            Spacer(Modifier.height(16.dp))
                            i++
                        }
                        "key" -> {
                            val keys = mutableListOf<AboutBlockView>()
                            while (i < sec.blocks.size && sec.blocks[i].kind == "key") {
                                keys.add(sec.blocks[i])
                                i++
                            }
                            RuledKeyList(keys, litKey) { key -> litKey = if (litKey == key) null else key }
                        }
                        "era" -> {
                            val eras = mutableListOf<AboutBlockView>()
                            while (i < sec.blocks.size && sec.blocks[i].kind == "era") {
                                eras.add(sec.blocks[i])
                                i++
                            }
                            LineageList(eras, body)
                        }
                        "ordo_ribbon" -> {
                            OrdoRibbon(
                                course = course,
                                today = today,
                                colors = view.ordoColors,
                                label = view.ribbonLabel,
                                selected = selectedColor,
                                onSelect = { slug -> selectedColor = if (selectedColor == slug) null else slug },
                                onMonth = { y, m -> onOpen(Page.Ordo(y, m, 0)) },
                            )
                            i++
                        }
                        else -> i++
                    }
                }
            }
        }
    }

    BoxWithConstraints(Modifier.fillMaxSize()) {
        val wide = maxWidth >= AboutRailFrom
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState())
                .padding(top = insets.calculateTopPadding(), bottom = insets.calculateBottomPadding()),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            chrome()
            val containerMod = if (wide) {
                Modifier.widthIn(max = 1024.dp).fillMaxWidth().padding(horizontal = 24.dp)
            } else {
                Modifier.measure()
            }
            Column(containerMod.padding(top = 18.dp)) {
                AboutHead(view.kicker, view.title, verse?.text.orEmpty(), verse?.cite.orEmpty(), wide = wide)
                if (wide) {
                    Row(
                        Modifier.fillMaxWidth().padding(top = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(51.dp),
                        verticalAlignment = Alignment.Top,
                    ) {
                        Column(Modifier.weight(1.45f)) {
                            introBlock()
                            proseSections()
                        }
                        Column(Modifier.weight(1f)) {
                            hoursColumn(Modifier.fillMaxWidth())
                            doorBlock(true)
                        }
                    }
                } else {
                    introBlock()
                    Spacer(Modifier.height(22.dp))
                    hoursColumn(Modifier.fillMaxWidth())
                    proseSections()
                    doorBlock(false)
                }
            }
            Footer()
        }
    }
}

private data class AboutSectionData(val heading: String, val blocks: List<AboutBlockView>)
private data class AboutPartition(val hoursHeading: String, val hoursNote: String, val sections: List<AboutSectionData>)

private fun partitionAboutBlocks(blocks: List<AboutBlockView>): AboutPartition {
    var hoursHeading = ""
    var hoursNote = ""
    var inHours = false
    val sections = mutableListOf<AboutSectionData>()
    var curHeading: String? = null
    var curBlocks = mutableListOf<AboutBlockView>()
    fun flush() {
        curHeading?.let { sections.add(AboutSectionData(it, curBlocks.toList())) }
        curHeading = null
        curBlocks = mutableListOf()
    }
    blocks.forEachIndexed { i, b ->
        when (b.kind) {
            "verse", "intro", "invitation" -> Unit
            "heading" -> {
                // The heading over the hours heads their column, whatever its words.
                if (blocks.getOrNull(i + 1)?.kind == "hours") {
                    flush()
                    hoursHeading = b.text
                    inHours = true
                } else {
                    flush()
                    inHours = false
                    curHeading = b.text
                }
            }
            "hours" -> inHours = true
            "note" -> if (inHours) hoursNote = b.text else curBlocks.add(b)
            else -> curBlocks.add(b)
        }
    }
    flush()
    return AboutPartition(hoursHeading, hoursNote, sections)
}

/**
 * The painted frontispiece head (echoing `FeastHead`): a full-width hand-ruled terracotta course
 * broken at the centre by the consecration cross, a soft wall glow behind it, the small-caps
 * kicker, the display title, and the red Psalm epigraph.
 */
@Composable
private fun AboutHead(kicker: String, title: String, verseText: String, verseCite: String, wide: Boolean) {
    val p = LocalPalette.current
    val pigment = ImageBitmap.imageResource(R.drawable.pigment)
    val crown = 34.4.dp
    val titleSize = if (wide) 40f else 33.6f
    val pool = if (p.dark) Color(150, 178, 220).copy(alpha = 0.16f) else Color(255, 253, 248).copy(alpha = 0.92f)

    Box(Modifier.fillMaxWidth().padding(top = 18.dp)) {
        Column(
            Modifier.fillMaxWidth().drawBehind {
                val u = 1.dp.toPx()
                val w = size.width
                val box = Rect(-16f * u, -35f * u, w + 16f * u, size.height + 20f * u)
                val centre = Offset(box.center.x, box.top + 0.42f * box.height)
                val rx = 0.52f * box.width
                val ry = 0.48f * box.height
                if (rx > 0f && ry > 0f) {
                    scale(1f, ry / rx, pivot = centre) {
                        drawCircle(Brush.radialGradient(0f to pool, 1f to Color.Transparent, center = centre, radius = rx), rx, centre)
                    }
                }
                val mid = w / 2f
                val gap = 24f * u
                drawPath(ruledLine(0f, 0.5f * u, mid - gap, 0.5f * u, 1.25f * u, 101, u), p.lining)
                drawPath(ruledLine(mid + gap, 0.5f * u, w, 0.5f * u, 1.25f * u, 102, u), p.lining)
            }.padding(start = 8.dp, end = 8.dp, top = 28.dp, bottom = if (wide) 28.dp else 21.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            if (kicker.isNotEmpty()) {
                Text(
                    kicker,
                    style = Type.body.copy(
                        fontSize = 14.08.sp,
                        lineHeight = 19.7.sp,
                        letterSpacing = 2.25.sp,
                        color = p.titulus,
                        fontFeatureSettings = ALL_SMALL_CAPS,
                        textAlign = TextAlign.Center,
                    ),
                )
            }
            Text(
                title,
                Modifier.padding(top = 2.4.dp).semantics { heading() },
                style = Type.body.copy(
                    fontSize = titleSize.sp,
                    lineHeight = (titleSize * 1.12f).sp,
                    color = p.text,
                    textAlign = TextAlign.Center,
                ),
            )
            if (verseText.isNotEmpty()) {
                Text(
                    verseText,
                    Modifier.padding(top = 11.dp),
                    style = Type.body.copy(
                        fontSize = if (wide) 18.6.sp else 17.6.sp,
                        lineHeight = 25.sp,
                        color = p.rubric,
                        fontStyle = FontStyle.Italic,
                        textAlign = TextAlign.Center,
                    ),
                )
                Text(
                    verseCite,
                    Modifier.padding(top = 2.dp),
                    style = Type.body.copy(
                        fontSize = 12.48.sp,
                        lineHeight = 18.sp,
                        letterSpacing = 1.25.sp,
                        color = p.rubric.copy(alpha = 0.92f),
                        fontFeatureSettings = ALL_SMALL_CAPS,
                        textAlign = TextAlign.Center,
                    ),
                )
            }
        }
        ConsecrationCross(
            Modifier.align(Alignment.TopCenter).offset(y = -crown / 2).size(crown).clearAndSetSemantics {},
            pigment = pigment,
        )
    }
}

/** A section's heading: a titulus in small capitals over one hand-ruled stroke of the lining. */
@Composable
private fun AboutSectionHeading(text: String, seed: Int) {
    val p = LocalPalette.current
    Text(
        text,
        Modifier.fillMaxWidth().padding(bottom = 13.6.dp).semantics { heading() }.drawBehind {
            val u = 1.dp.toPx()
            drawPath(ruledLine(0f, size.height - 0.5f * u, size.width, size.height - 0.5f * u, 1.25f * u, seed, u), p.lining.copy(alpha = 0.78f))
        }.padding(bottom = 6.dp),
        style = Type.body.copy(
            fontSize = 15.68.sp,
            lineHeight = 20.4.sp,
            letterSpacing = 2.35.sp,
            color = p.titulus,
            fontFeatureSettings = ALL_SMALL_CAPS,
        ),
    )
}

/**
 * The painted Horologium (Arc of the Day): a semicircular solar path with seven tappable stations
 * from Dawn to Night. Tapping any station smoothly glides the sun (or crescent moon at Vespers and
 * Compline) along the arc and shifts the theme-faithful wash (honest warm plaster light in Nave;
 * quiet vaulted night sky with subtle gold-leaf stars and a travelling lantern halo in Apse).
 */
private data class VaultStar(val x: Float, val y: Float, val r: Float, val eightRay: Boolean, val weight: Float)

private val HOROLOGIUM_STARS = listOf(
    VaultStar(102f, 56f, 3.4f, true, 0.88f),
    VaultStar(134f, 38f, 4.2f, true, 0.96f),
    VaultStar(176f, 35f, 2.9f, false, 0.78f),
    VaultStar(218f, 52f, 3.9f, true, 0.92f),
    VaultStar(82f, 84f, 2.6f, false, 0.74f),
    VaultStar(154f, 54f, 2.4f, false, 0.68f),
    VaultStar(238f, 82f, 3.1f, true, 0.82f),
    VaultStar(192f, 62f, 2.4f, false, 0.72f),
)

@Composable
private fun Horologium(
    periods: List<AboutPeriodView>,
    current: String,
    pinned: String?,
    onSelect: (String) -> Unit,
    onPin: (String) -> Unit,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val hours = remember(periods) { periods.flatMap { it.hours } }
    val activeSlug = pinned ?: current.ifEmpty { "compline" }
    val active = hours.firstOrNull { it.hour == activeSlug } ?: hours.lastOrNull() ?: return
    val isNow = active.hour == current && current.isNotEmpty()
    var dragAngle by remember { mutableStateOf<Float?>(null) }
    val animatedAngle by animateFloatAsState(
        targetValue = active.angle.toFloat(),
        animationSpec = tween(durationMillis = 420, easing = FastOutSlowInEasing),
        label = "horologium-angle",
    )
    val displayAngle = dragAngle ?: animatedAngle
    val starTargetAlpha = when (active.hour) {
        "sext" -> 0.24f
        "terce", "none" -> 0.38f
        "prime" -> 0.50f
        "lauds", "vespers" -> 0.68f
        "compline" -> 0.88f
        else -> 0.62f
    }
    val starAlpha by animateFloatAsState(
        targetValue = starTargetAlpha,
        animationSpec = tween(durationMillis = 450, easing = FastOutSlowInEasing),
        label = "horologium-stars",
    )

    // Theme-faithful dome wash stops: Nave stays warm plaster (never dark sky or stars);
    // Apse stays vaulted night sky with subtle stars and shifts the orb's lantern glow.
    val (wash0, wash1, wash2) = if (p.dark) {
        when (active.hour) {
            "lauds", "vespers" -> Triple(Color(208, 134, 102, 56), Color(46, 72, 108, 72), Color(18, 28, 40, 0))
            "sext" -> Triple(Color(232, 198, 122, 66), Color(46, 72, 108, 66), Color(18, 28, 40, 0))
            "compline" -> Triple(Color(154, 182, 224, 56), Color(34, 56, 88, 82), Color(18, 28, 40, 0))
            else -> Triple(Color(214, 178, 108, 56), Color(46, 72, 108, 72), Color(18, 28, 40, 0))
        }
    } else {
        when (active.hour) {
            "lauds", "vespers" -> Triple(Color(196, 118, 92, 51), Color(216, 166, 86, 20), Color(250, 243, 233, 0))
            "sext" -> Triple(Color(224, 178, 86, 66), Color(224, 178, 86, 23), Color(250, 243, 233, 0))
            "compline" -> Triple(Color(146, 134, 122, 41), Color(146, 134, 122, 13), Color(250, 243, 233, 0))
            else -> Triple(Color(216, 166, 86, 56), Color(216, 166, 86, 18), Color(250, 243, 233, 0))
        }
    }

    val density = LocalDensity.current
    val largeFont = density.fontScale > 1.35f
    val readout: @Composable (Modifier) -> Unit = { mod ->
        Column(
            // Not a live region, as on the web and iOS: each stop already names its hour, and a
            // readout spoken at every pass would talk over the page.
            mod.clearAndSetSemantics {
                contentDescription = "${if (isNow) "Now, " else ""}${active.time}. ${active.name}, ${active.gloss}"
            },
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text(
                (if (isNow) "Now · " else "") + active.time,
                style = Type.body.copy(
                    fontSize = 11.2.sp,
                    lineHeight = 14.5.sp,
                    letterSpacing = 1.5.sp,
                    color = p.titulus,
                    fontFeatureSettings = ALL_SMALL_CAPS,
                    textAlign = TextAlign.Center,
                ),
            )
            Text(
                buildAnnotatedString {
                    append(active.name)
                    withStyle(SpanStyle(color = p.muted, fontStyle = FontStyle.Italic, fontSize = 15.sp)) {
                        append(" · ")
                        append(active.gloss)
                    }
                },
                style = Type.body.copy(fontSize = 18.4.sp, lineHeight = 22.5.sp, color = p.text, textAlign = TextAlign.Center),
            )
        }
    }

    Column(Modifier.fillMaxWidth().padding(top = 3.dp, bottom = 13.6.dp)) {
        BoxWithConstraints(
            Modifier.fillMaxWidth().aspectRatio(320f / 144f)
                .pointerInput(hours) {
                    fun updateFromPosition(pos: Offset) {
                        if (size.width <= 0 || size.height <= 0) return
                        val svgX = (pos.x / size.width) * 320f
                        val svgY = (pos.y / size.height) * 144f
                        val deg = Math.toDegrees(atan2((128f - svgY).toDouble(), (svgX - 160f).toDouble())).toFloat().coerceIn(12f, 168f)
                        dragAngle = deg
                        val nearest = hours.minByOrNull { kotlin.math.abs(it.angle.toFloat() - deg) }
                        if (nearest != null) onPin(nearest.hour)
                    }
                    detectHorizontalDragGestures(
                        onDragStart = { updateFromPosition(it) },
                        onHorizontalDrag = { change, _ ->
                            change.consume()
                            updateFromPosition(change.position)
                        },
                        onDragEnd = { dragAngle = null },
                        onDragCancel = { dragAngle = null },
                    )
                },
        ) {
            val boxW = maxWidth
            val boxH = maxHeight

            Canvas(Modifier.fillMaxSize()) {
                val sx = size.width / 320f
                val sy = size.height / 144f
                val cx = 160f * sx
                val cy = 128f * sy
                val rx = 118f * sx
                val ry = 104f * sy

                fun arcPt(deg: Float, factor: Float = 1f): Offset {
                    val rad = Math.toRadians(deg.toDouble())
                    return Offset(cx + cos(rad).toFloat() * rx * factor, cy - sin(rad).toFloat() * ry * factor)
                }

                val dome = Path().apply {
                    moveTo(cx - rx, cy)
                    arcTo(Rect(cx - rx, cy - ry, cx + rx, cy + ry), 180f, 180f, false)
                    close()
                }
                val orb = arcPt(displayAngle)
                val washRadius = max(size.width, size.height) * 0.68f
                drawPath(dome, Brush.radialGradient(0f to wash0, 0.52f to wash1, 1f to wash2, center = orb, radius = washRadius))

                if (p.dark) {
                    clipPath(dome) {
                        // Parish apse 8-ray principal stars and 4-ray minor stars, matching about.html.
                        HOROLOGIUM_STARS.forEach { s ->
                            val r = s.r * sx
                            val px = s.x * sx
                            val py = s.y * sy
                            val starPath = Path().apply {
                                if (s.eightRay) {
                                    val w = r * 0.19f
                                    val c = r * 0.25f
                                    val d = r * 0.45f
                                    moveTo(px, py - r)
                                    lineTo(px + w, py - c)
                                    lineTo(px + d, py - d)
                                    lineTo(px + c, py - w)
                                    lineTo(px + r, py)
                                    lineTo(px + c, py + w)
                                    lineTo(px + d, py + d)
                                    lineTo(px + w, py + c)
                                    lineTo(px, py + r)
                                    lineTo(px - w, py + c)
                                    lineTo(px - d, py + d)
                                    lineTo(px - c, py + w)
                                    lineTo(px - r, py)
                                    lineTo(px - c, py - w)
                                    lineTo(px - d, py - d)
                                    lineTo(px - w, py - c)
                                    close()
                                } else {
                                    val w = r * 0.21f
                                    moveTo(px, py - r)
                                    lineTo(px + w, py - w)
                                    lineTo(px + r, py)
                                    lineTo(px + w, py + w)
                                    lineTo(px, py + r)
                                    lineTo(px - w, py + w)
                                    lineTo(px - r, py)
                                    lineTo(px - w, py - w)
                                    close()
                                }
                            }
                            drawPath(starPath, o.hi.copy(alpha = (starAlpha * s.weight).coerceIn(0f, 1f)))
                        }
                    }
                }

                // Station ticks
                hours.forEach { h ->
                    val a = h.angle.toFloat()
                    drawLine(p.lining.copy(alpha = 0.32f), arcPt(a, 0.91f), arcPt(a, 1f), 0.7f * sx)
                }

                // Inner dashed arc
                val innerArc = Path().apply {
                    arcTo(Rect(cx - rx * 0.93f, cy - ry * 0.93f, cx + rx * 0.93f, cy + ry * 0.93f), 180f, 180f, false)
                }
                drawPath(
                    innerArc,
                    p.lining.copy(alpha = 0.32f),
                    style = Stroke(width = 0.6f * sx, pathEffect = PathEffect.dashPathEffect(floatArrayOf(2f * sx, 4f * sx))),
                )

                // Main solar arc
                val mainArc = Path().apply {
                    arcTo(Rect(cx - rx, cy - ry, cx + rx, cy + ry), 180f, 180f, false)
                }
                drawPath(mainArc, p.lining.copy(alpha = 0.52f), style = Stroke(width = 1.15f * sx))

                // Lit arc from Lauds (168°) to the current orb angle
                val startSweep = 180f + (180f - 168f)
                val sweep = (168f - displayAngle).coerceAtLeast(0.5f)
                val litArc = Path().apply {
                    arcTo(Rect(cx - rx, cy - ry, cx + rx, cy + ry), startSweep, sweep, false)
                }
                drawPath(litArc, o.flat.copy(alpha = 0.78f), style = Stroke(width = 1.5f * sx))

                // Horizon line
                drawLine(p.lining.copy(alpha = 0.38f), Offset(16f * sx, cy), Offset(304f * sx, cy), 0.85f * sx)

                // Non-active diamond stations
                hours.forEach { h ->
                    if (h.hour == active.hour) return@forEach
                    val pt = arcPt(h.angle.toFloat())
                    val side = 5.4f * sx
                    val half = side / 2f
                    val d = Path().apply {
                        moveTo(pt.x, pt.y - half * 1.414f)
                        lineTo(pt.x + half * 1.414f, pt.y)
                        lineTo(pt.x, pt.y + half * 1.414f)
                        lineTo(pt.x - half * 1.414f, pt.y)
                        close()
                    }
                    drawPath(d, p.bg)
                    drawPath(d, p.lining.copy(alpha = 0.85f), style = Stroke(width = 1.2f * sx))
                }

                // Active Sun or Moon orb with radial halo
                val haloR = 15f * sx
                drawCircle(
                    Brush.radialGradient(
                        0f to o.hi.copy(alpha = if (p.dark) 0.42f else 0.36f),
                        0.55f to o.flat.copy(alpha = 0.12f),
                        1f to Color.Transparent,
                        center = orb,
                        radius = haloR,
                    ),
                    haloR,
                    orb,
                )
                val glyphScale = (if (active.night) 0.80f else 0.875f) * sx
                val glyphHalf = 12f * glyphScale
                translate(orb.x - glyphHalf, orb.y - glyphHalf) {
                    scale(glyphScale, glyphScale, pivot = Offset.Zero) {
                        drawPath(if (active.night) Moon else Sun, o.flat)
                    }
                }
            }

            // Cardinal labels (Dawn / Noon / Night)
            val cardinalSp = with(density) { 8.5.dp.toSp() }
            val cardinalStyle = Type.body.copy(
                fontSize = cardinalSp,
                lineHeight = cardinalSp * 1.25f,
                letterSpacing = cardinalSp * 0.14f,
                color = p.muted,
                fontFeatureSettings = ALL_SMALL_CAPS,
            )
            Text("Dawn", Modifier.align(Alignment.BottomStart).padding(start = boxW * (42f / 320f)).clearAndSetSemantics {}, style = cardinalStyle)
            Text("Noon", Modifier.align(Alignment.TopCenter).padding(top = 2.dp).clearAndSetSemantics {}, style = cardinalStyle)
            Text("Night", Modifier.align(Alignment.BottomEnd).padding(end = boxW * (42f / 320f)).clearAndSetSemantics {}, style = cardinalStyle)

            // Centered readout inside the dome belly at standard font scale
            if (!largeFont) {
                readout(Modifier.align(Alignment.BottomCenter).padding(bottom = boxH * 0.22f, start = boxW * 0.14f, end = boxW * 0.14f))
            }

            // 44dp tap targets over each of the seven stations (Lauds and Prime sit about 48dp apart on a phone)
            val tapPx = with(density) { 44.dp.toPx() }
            hours.forEach { h ->
                val rad = Math.toRadians(h.angle)
                val fracX = (160.0 + cos(rad) * 118.0) / 320.0
                val fracY = (128.0 - sin(rad) * 104.0) / 144.0
                val xPx = with(density) { (boxW.toPx() * fracX - tapPx / 2f).roundToInt() }
                val yPx = with(density) { (boxH.toPx() * fracY - tapPx / 2f).roundToInt() }
                Box(
                    Modifier.offset { IntOffset(xPx, yPx) }
                        .size(44.dp)
                        .tap(label = "${h.name}: ${h.gloss}, ${h.time}", selected = h.hour == active.hour) { onSelect(h.hour) },
                )
            }
        }
        // At large accessibility font scales, set the readout cleanly beneath the arc.
        if (largeFont) {
            readout(Modifier.fillMaxWidth().padding(top = 8.dp))
        }
    }
}

/**
 * The seven hours in home's three periods, ruled as home's painted page: firm hand-ruled terracotta
 * frame above and below, medium dividers between periods, faint hairlines between hours inside a
 * period, and zero vertical rules or shaded cells.
 */
@Composable
private fun HoursDirectory(
    periods: List<AboutPeriodView>,
    current: String,
    preview: String?,
    onHour: (String) -> Unit,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val frameInk = p.lining.copy(alpha = 0.78f)
    val innerInk = p.lining.copy(alpha = 0.36f)
    val hairInk = p.lining.copy(alpha = 0.19f)
    val periodInk = mix(p.accent, p.muted, 0.45f)

    val labelStyle = Type.label(11.2f, 0.12f).copy(lineHeight = 13.4.sp, color = periodInk, fontFeatureSettings = ALL_SMALL_CAPS)
    val nameStyle = Type.body.copy(fontSize = 18.56.sp, lineHeight = 23.2.sp, color = p.text)
    val glossStyle = Type.body.copy(fontSize = 16.32.sp, lineHeight = 20.9.sp, color = p.text, fontStyle = FontStyle.Italic)
    val timeStyle = Type.body.copy(fontSize = 13.44.sp, lineHeight = 17.5.sp, color = p.muted)

    val measurer = rememberTextMeasurer()
    val density = LocalDensity.current
    val labelWidth = remember(periods, labelStyle, density) {
        with(density) { periods.maxOf { measurer.measure(it.label, labelStyle).size.width }.toDp() + 8.dp }.coerceAtLeast(78.dp)
    }
    val nameWidth = remember(periods, nameStyle, density) {
        with(density) { periods.flatMap { it.hours }.maxOf { measurer.measure(it.name, nameStyle).size.width }.toDp() + 1.dp }
    }
    val pool = homeInk(p).nowPool
    val nameNow = titulusNow(p, "")

    Column(Modifier.fillMaxWidth()) {
        periods.forEachIndexed { i, period ->
            RuledDivider(if (i == 0) frameInk else innerInk, if (i == 0) 1.25f else 0.9f, seed = 130 + i)
            Row(Modifier.fillMaxWidth().height(IntrinsicSize.Min)) {
                Column(
                    Modifier.width(labelWidth).fillMaxHeight().padding(horizontal = 4.dp, vertical = 7.dp)
                        .semantics(mergeDescendants = true) { heading() },
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.Center,
                ) {
                    PeriodIcon(periodOf(period.label), o.flat.copy(alpha = 0.9f))
                    Text(period.label, Modifier.padding(top = 2.56.dp), softWrap = false, style = labelStyle)
                }
                Column(Modifier.weight(1f)) {
                    period.hours.forEachIndexed { j, h ->
                        if (j > 0) RuledDivider(hairInk, 0.75f, seed = 140 + i * 5 + j)
                        val isNow = h.hour == current
                        val isPreview = h.hour == preview
                        Row(
                            Modifier.fillMaxWidth()
                                .then(
                                    when {
                                        isNow -> Modifier.poolAlongRow(pool).semantics { stateDescription = "now" }
                                        isPreview -> Modifier.poolAlongRow(pool.copy(alpha = pool.alpha * 0.65f))
                                        else -> Modifier
                                    },
                                )
                                .tap(action = "open ${h.name}", lit = Lit(pool, PoolShape.Row)) { onHour(h.hour) }
                                .padding(start = 7.dp, end = 9.dp, top = 7.7.dp, bottom = 7.7.dp),
                        ) {
                            Text(
                                h.name,
                                Modifier.width(nameWidth).alignByBaseline(),
                                softWrap = false,
                                style = if (isNow) nameStyle.copy(color = nameNow) else nameStyle,
                            )
                            Spacer(Modifier.width(10.4.dp))
                            Column(Modifier.weight(1f).alignByBaseline()) {
                                Text(h.gloss, style = glossStyle)
                                Text(h.time, style = timeStyle)
                            }
                        }
                    }
                }
            }
        }
        RuledDivider(frameInk, 1.25f, seed = 139)
    }
}

/**
 * The live miniature office specimen in "Reading the page": framed in home's double hand-ruled
 * course over the feathered tablet wash, with the hour's Private / Deacon / Priest switch and
 * soft pool-of-light illumination tied to the ruled key below.
 */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun SpecimenLeaf(
    specimen: AboutSpecimenView,
    form: String,
    litKey: String?,
    onForm: (String) -> Unit,
    onToggleKey: (String) -> Unit,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val tablet = ImageBitmap.imageResource(R.drawable.tablet)
    val wash = homeInk(p).tabletWash
    val pool = homeInk(p).nowPool
    val lineStyle = Type.body.copy(fontSize = 16.5.sp, lineHeight = 24.sp, color = p.text)
    var openingLayout by remember { mutableStateOf<TextLayoutResult?>(null) }
    val crossIndex = remember(specimen.openingVersicle) { specimen.openingVersicle.indexOf('✠') }

    Column(
        Modifier.fillMaxWidth().drawBehind {
            tabletWash(tablet, wash)
            invitationRules(p.lining, o.flat, seed = 161, day = 0.58f)
        }.padding(horizontal = 18.dp, vertical = 15.dp),
    ) {
        // Header bar: "From an hour" + Private ◆ Deacon ◆ Priest (wraps cleanly at 200% font scale)
        FlowRow(
            Modifier.fillMaxWidth().litPool(litKey == "form", pool).padding(bottom = 8.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalArrangement = Arrangement.spacedBy(4.dp),
            itemVerticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                specimen.label,
                Modifier.padding(end = 10.dp).pointerInput(Unit) { detectTapGestures { onToggleKey("form") } },
                softWrap = false,
                style = Type.body.copy(
                    fontSize = 11.84.sp,
                    lineHeight = 16.sp,
                    letterSpacing = 1.65.sp,
                    color = p.titulus,
                    fontFeatureSettings = ALL_SMALL_CAPS,
                ),
            )
            FlowRow(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalArrangement = Arrangement.spacedBy(2.dp),
                itemVerticalAlignment = Alignment.CenterVertically,
            ) {
                SpecimenFormTab(specimen.aloneLabel, form == "private") { onForm("private") }
                Box(Modifier.size(3.5.dp).rotate(45f).background(o.flat.copy(alpha = 0.72f)))
                SpecimenFormTab(specimen.deaconLabel, form == "deacon") { onForm("deacon") }
                Box(Modifier.size(3.5.dp).rotate(45f).background(o.flat.copy(alpha = 0.72f)))
                SpecimenFormTab(specimen.priestLabel, form == "priest") { onForm("priest") }
            }
        }
        RuledDivider(p.lining.copy(alpha = 0.36f), 0.85f, seed = 169)
        Spacer(Modifier.height(10.dp))

        // Lauds' opening as the hour prints it: its rubric, the secret Our Father, the ✠ where it falls.
        Text(
            specimen.rubric,
            Modifier.fillMaxWidth().litPool(litKey == "rubric", pool)
                .pointerInput(Unit) { detectTapGestures { onToggleKey("rubric") } }
                .padding(bottom = 6.dp),
            style = lineStyle.copy(fontSize = 15.sp, color = p.rubric, fontStyle = FontStyle.Italic),
        )
        Text(
            buildAnnotatedString {
                append(specimen.secretVoiced)
                append(" ")
                withStyle(SpanStyle(color = if (p.dark) p.unsaid else p.muted)) { append(specimen.secretSilent) }
            },
            Modifier.fillMaxWidth().litPool(litKey == "secret", pool)
                .pointerInput(Unit) { detectTapGestures { onToggleKey("secret") } }
                .padding(bottom = 8.dp),
            style = lineStyle,
        )

        RuledDivider(p.lining.copy(alpha = 0.19f), 0.75f, seed = 171)
        Spacer(Modifier.height(8.dp))
        Row(
            Modifier.fillMaxWidth().litPool(litKey == "vr" || litKey == "cross", pool).padding(bottom = 4.dp)
                .clearAndSetSemantics {
                    contentDescription = "Versicle. " + specimen.openingVersicle.replace("✠", "sign of the cross,")
                },
        ) {
            Text("℣.", Modifier.width(27.dp).pointerInput(Unit) { detectTapGestures { onToggleKey("vr") } }, style = lineStyle.copy(color = p.rubric))
            Text(
                buildAnnotatedString {
                    val parts = specimen.openingVersicle.split("✠", limit = 2)
                    append(parts[0])
                    if (parts.size == 2) {
                        withStyle(SpanStyle(color = p.rubric)) { append("✠") }
                        append(parts[1])
                    }
                },
                Modifier.pointerInput(crossIndex) {
                    detectTapGestures { pos ->
                        val offset = openingLayout?.getOffsetForPosition(pos) ?: -1
                        if (crossIndex >= 0 && offset in (crossIndex - 1)..(crossIndex + 1)) onToggleKey("cross")
                        else onToggleKey("vr")
                    }
                },
                style = lineStyle,
                onTextLayout = { openingLayout = it },
            )
        }
        Row(
            Modifier.fillMaxWidth().litPool(litKey == "vr", pool)
                .pointerInput(Unit) { detectTapGestures { onToggleKey("vr") } }
                .padding(bottom = 8.dp)
                .clearAndSetSemantics { contentDescription = "Response. ${specimen.openingResponse}" },
        ) {
            Text("℟.", Modifier.width(27.dp), style = lineStyle.copy(color = p.rubric))
            Text(specimen.openingResponse, style = lineStyle)
        }

        // The close's greeting, as the chosen form gives it: a deacon greets as a priest does,
        // while a quiet note names how Deacon and Priest differ across the hours.
        RuledDivider(p.lining.copy(alpha = 0.19f), 0.75f, seed = 170)
        val clergy = form != "private"
        val formV = if (clergy) specimen.priestVersicle else specimen.aloneVersicle
        val formR = if (clergy) specimen.priestResponse else specimen.aloneResponse
        val formNote = when (form) {
            "deacon" -> specimen.deaconNote
            "priest" -> specimen.priestNote
            else -> ""
        }
        Column(
            Modifier.fillMaxWidth().litPool(litKey == "form", pool)
                .pointerInput(Unit) { detectTapGestures { onToggleKey("form") } }
                .padding(top = 7.dp)
                .clearAndSetSemantics {
                    liveRegion = LiveRegionMode.Polite
                    contentDescription = "Versicle. $formV Response. $formR" + if (formNote.isNotEmpty()) " $formNote" else ""
                },
        ) {
            Row(Modifier.fillMaxWidth().padding(bottom = 4.dp)) {
                Text("℣.", Modifier.width(27.dp), style = lineStyle.copy(color = p.rubric))
                Text(formV, style = lineStyle)
            }
            Row(Modifier.fillMaxWidth()) {
                Text("℟.", Modifier.width(27.dp), style = lineStyle.copy(color = p.rubric))
                Text(formR, style = lineStyle)
            }
            if (formNote.isNotEmpty()) {
                Text(
                    formNote,
                    Modifier.padding(top = 5.dp),
                    style = Type.body.copy(fontSize = 13.76.sp, lineHeight = 19.sp, color = p.muted, fontStyle = FontStyle.Italic),
                )
            }
        }
    }
}

private fun Modifier.litPool(active: Boolean, pool: Color): Modifier = if (active) {
    drawBehind {
        val c = Offset(size.width * 0.32f, size.height * 0.5f)
        val rx = size.width * 0.68f
        val ry = size.height * 0.85f
        if (rx > 0f && ry > 0f) {
            scale(1f, ry / rx, pivot = c) { drawCircle(poolBrush(pool, c, rx), rx, c) }
        }
    }
} else {
    this
}

@Composable
private fun SpecimenFormTab(label: String, selected: Boolean, onClick: () -> Unit) {
    val p = LocalPalette.current
    Text(
        label,
        Modifier
            .tap(selected = selected, label = label) { onClick() }
            .drawBehind {
                if (selected) {
                    val y = size.height - 0.75.dp.toPx()
                    drawLine(p.gold, Offset(0f, y), Offset(size.width, y), 1.5.dp.toPx())
                }
            }
            .padding(vertical = 3.dp),
        softWrap = false,
        style = Type.body.copy(
            fontSize = 12.48.sp,
            lineHeight = 16.sp,
            letterSpacing = 1.25.sp,
            color = if (selected) p.text else p.muted,
            fontFeatureSettings = ALL_SMALL_CAPS,
        ),
    )
}

/** The open hand-ruled 5-row typographic key beneath the specimen leaf. */
@Composable
private fun RuledKeyList(keys: List<AboutBlockView>, litKey: String?, onToggle: (String) -> Unit) {
    val p = LocalPalette.current
    val pool = homeInk(p).nowPool
    val descStyle = Type.body.copy(fontSize = 15.5.sp, lineHeight = 23.sp, color = p.text)
    Column(Modifier.fillMaxWidth()) {
        keys.forEachIndexed { i, k ->
            if (i > 0) RuledDivider(p.lining.copy(alpha = 0.19f), 0.8f, seed = 180 + i)
            val active = litKey == k.id
            Row(
                Modifier.fillMaxWidth()
                    .then(if (active) Modifier.poolAlongRow(pool.copy(alpha = pool.alpha * 0.85f)) else Modifier)
                    .tap(selected = active, label = "${k.spoken}: ${k.text}") { onToggle(k.id) }
                    .padding(vertical = 8.3.dp, horizontal = 3.dp),
                verticalAlignment = Alignment.Top,
            ) {
                val markMod = Modifier.width(108.dp).padding(end = 10.dp)
                when (k.ink) {
                    "rubric" -> Text(k.mark, markMod, style = descStyle.copy(color = p.rubric, fontStyle = FontStyle.Italic))
                    "symbol" -> Text(k.mark, markMod, style = descStyle.copy(color = p.rubric, letterSpacing = 1.3.sp))
                    "cross" -> Text(k.mark, markMod, style = descStyle.copy(fontSize = 16.8.sp, color = p.rubric))
                    "secret" -> Text(
                        buildAnnotatedString {
                            append(k.mark)
                            append(" ")
                            withStyle(SpanStyle(color = if (p.dark) p.unsaid else p.muted)) { append(k.tail) }
                        },
                        markMod,
                        style = descStyle,
                    )
                    "form" -> Text(
                        k.mark,
                        markMod,
                        style = descStyle.copy(
                            fontSize = 13.44.sp,
                            letterSpacing = 1.35.sp,
                            color = p.titulus,
                            fontFeatureSettings = ALL_SMALL_CAPS,
                        ),
                    )
                    else -> Text(k.mark, markMod, style = descStyle)
                }
                Text(k.text, Modifier.weight(1f), style = descStyle)
            }
        }
        RuledDivider(p.lining.copy(alpha = 0.36f), 0.9f, seed = 189)
    }
}

/** The three chronological beats in "An ancient prayer", along a hand-ruled marginal thread. */
@Composable
private fun LineageList(eras: List<AboutBlockView>, body: TextStyle) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    Column(
        Modifier.fillMaxWidth().padding(top = 4.dp).drawBehind {
            val u = 1.dp.toPx()
            val x = 3.dp.toPx()
            val y0 = 10.dp.toPx()
            val y1 = (size.height - 24.dp.toPx()).coerceAtLeast(y0 + 1f)
            drawPath(ruledLine(x, y0, x, y1, 0.9f * u, 195, u), p.lining.copy(alpha = 0.36f))
        },
    ) {
        eras.forEachIndexed { i, era ->
            Row(
                Modifier.fillMaxWidth().padding(bottom = if (i == eras.lastIndex) 0.dp else 16.8.dp),
                verticalAlignment = Alignment.Top,
            ) {
                Box(Modifier.padding(top = 7.dp, end = 14.dp).size(6.dp).rotate(45f).background(o.flat))
                Column(Modifier.weight(1f)) {
                    Text(
                        era.period,
                        Modifier.padding(bottom = 4.dp).semantics { heading() },
                        style = Type.body.copy(
                            fontSize = 13.12.sp,
                            lineHeight = 18.sp,
                            letterSpacing = 1.45.sp,
                            color = p.titulus,
                            fontFeatureSettings = ALL_SMALL_CAPS,
                        ),
                    )
                    Text(era.text, style = body)
                }
            }
        }
    }
}

/** The current year's liturgical color ribbon and interactive 6-color diamond legend in "The Ordo". */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun OrdoRibbon(
    course: List<CourseMonthView>?,
    today: LocalDate,
    colors: List<AboutOrdoColorView>,
    label: String,
    selected: String?,
    onSelect: (String) -> Unit,
    onMonth: (Int, Int) -> Unit,
) {
    val p = LocalPalette.current
    val o = LocalOrnament.current
    val span = courseSpan().toInt()
    val gap = 3
    val total = 12 * span + 11 * gap
    val rareColors = remember(course) {
        if (course == null) emptySet()
        else {
            val maxDays = rareColorMaxDays().toInt()
            val totals = mutableMapOf<String, Int>()
            course.forEach { m -> m.runs.forEach { r -> totals[r.color] = (totals[r.color] ?: 0) + r.days } }
            totals.filterValues { it in 1..maxDays }.keys
        }
    }

    Column(Modifier.fillMaxWidth().padding(top = 14.dp, bottom = 4.dp)) {
        if (!course.isNullOrEmpty()) {
            val todayFrac = ((today.monthValue - 1) * (span + gap) + today.dayOfMonth.coerceIn(1, span) - 0.5f) / total.toFloat()
            Box(
                Modifier.fillMaxWidth().height(23.dp)
                    .pointerInput(today.year, total) {
                        detectTapGestures { offset ->
                            if (size.width > 0) {
                                val x = (offset.x / size.width) * total
                                val mi = (x / (span + gap)).toInt().coerceIn(0, 11)
                                onMonth(today.year, mi + 1)
                            }
                        }
                    }
                    .clearAndSetSemantics { contentDescription = label },
            ) {
                Canvas(Modifier.fillMaxWidth().fillMaxHeight()) {
                    val unit = size.width / total.toFloat()
                    val barTop = 8.dp.toPx()
                    val barH = 5.dp.toPx()
                    course.forEachIndexed { mi, month ->
                        val ox = mi * (span + gap)
                        var end = 0
                        month.runs.forEach { r ->
                            val col = ribbonColor(r.color, p)
                            val alpha = if (selected == null || selected == r.color) 1f else 0.16f
                            drawRect(
                                color = col.copy(alpha = alpha),
                                topLeft = Offset((ox + r.start) * unit, barTop),
                                size = Size(r.days * unit, barH),
                            )
                            if (selected == r.color && r.color in rareColors) {
                                val midX = (ox + r.start + r.days / 2f) * unit
                                val pr = 2.4.dp.toPx()
                                val pinY = barTop - 4.5.dp.toPx()
                                lozenge(Offset(midX, pinY), pr, col, null)
                            }
                            end = max(end, r.start + r.days)
                        }
                        if (end < span) {
                            drawRect(
                                color = p.lining.copy(alpha = 0.22f),
                                topLeft = Offset((ox + end) * unit, barTop + barH * 0.4f),
                                size = Size((span - end) * unit, barH * 0.2f),
                            )
                        }
                    }
                    // Today's gilt diamond hanging beneath today's place on the ribbon
                    val x = size.width * todayFrac
                    val r = 2.8.dp.toPx()
                    lozenge(Offset(x, barTop + barH + 3.dp.toPx() + r), r, o.flat, null)
                }
            }
            // Each name stands at its own month's start, as the web's (view.rs AboutRibbonLabel).
            BoxWithConstraints(Modifier.fillMaxWidth().padding(top = 2.dp).clearAndSetSemantics {}) {
                listOf("Jan", "Mar", "May", "Jul", "Sep", "Nov").forEachIndexed { i, m ->
                    Text(
                        m,
                        Modifier.offset(x = maxWidth * (2 * i * (span + gap)).toFloat() / total.toFloat()),
                        softWrap = false,
                        style = Type.body.copy(
                            fontSize = 10.88.sp,
                            lineHeight = 14.sp,
                            letterSpacing = 0.9.sp,
                            color = p.muted,
                            fontFeatureSettings = ALL_SMALL_CAPS,
                        ),
                    )
                }
            }
        }

        FlowRow(
            Modifier.fillMaxWidth().padding(top = 10.dp),
            horizontalArrangement = Arrangement.spacedBy(14.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            colors.forEach { c ->
                val isSelected = selected == c.slug
                Row(
                    Modifier
                        .tap(selected = isSelected, label = "${c.name}: ${c.meaning}") { onSelect(c.slug) }
                        .drawBehind {
                            if (isSelected) {
                                val y = size.height - 0.75.dp.toPx()
                                drawLine(p.gold, Offset(0f, y), Offset(size.width, y), 1.5.dp.toPx())
                            }
                        }
                        .padding(vertical = 3.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(5.5.dp),
                ) {
                    Box(Modifier.size(5.5.dp).rotate(45f).background(ribbonColor(c.slug, p)))
                    Text(
                        c.name,
                        style = Type.body.copy(fontSize = 13.76.sp, lineHeight = 18.sp, color = if (isSelected) p.text else p.muted),
                    )
                }
            }
        }

        val active = colors.firstOrNull { it.slug == selected }
        if (active != null) {
            val note = remember(course, active.slug, today.year) {
                if (!course.isNullOrEmpty()) colorDaysNote(course, active.slug, today.year) else ""
            }
            val line = "${active.name} — ${active.meaning}" + if (note.isNotEmpty()) " ($note)" else ""
            Text(
                line,
                Modifier.padding(top = 6.dp).semantics { liveRegion = LiveRegionMode.Polite },
                style = Type.body.copy(fontSize = 14.4.sp, lineHeight = 20.sp, color = p.muted, fontStyle = FontStyle.Italic),
            )
        }
    }
}

internal fun ribbonColor(slug: String, p: Palette): Color = when {
    p.dark && slug == "black" -> Color(0xFF6A7380)
    else -> dayColor(slug, p)
}

/** One horizontal hand-ruled stroke (`ruledLine`) across the full width. */
@Composable
private fun RuledDivider(color: Color, weightDp: Float, seed: Int) {
    Canvas(Modifier.fillMaxWidth().height(weightDp.dp.coerceAtLeast(1.dp))) {
        val u = 1.dp.toPx()
        drawPath(ruledLine(0f, size.height / 2f, size.width, size.height / 2f, weightDp * u, seed, u), color)
    }
}

/**
 * A paragraph with its links in the accent, each underlined in the gold line a fifth of the type
 * below its baseline, as the web's `.about a`.
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
internal fun periodOf(label: String): Period = when (label.lowercase()) {
    "morning" -> Period.MORNING
    "day" -> Period.DAY
    else -> Period.EVENING
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
