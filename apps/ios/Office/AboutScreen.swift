import SwiftUI
import UIKit

/// The About page's words, as the web's /about sets them (presentation::about): read once, as they never change.
let aboutPage: AboutView = about()

/// Where a link on the About page leads: a page of the app, or the browser.
enum AboutLink: Equatable {
    case ordo
    case reminders
    case web(URL)
}

/**
 * The page an About link's target opens: the site's own paths map to the app's pages, except
 * the privacy policy, which lives only on the site the beacons go to.
 */
func aboutLink(_ target: String, defaults: UserDefaults = .standard) -> AboutLink? {
    switch target {
    case "/calendar": return .ordo
    case "/reminders": return .reminders
    case "/privacy": return Usage.siteURL("/privacy", defaults: defaults).map(AboutLink.web)
    default: return target.hasPrefix("https://") ? URL(string: target).map(AboutLink.web) : nil
    }
}

/// Width from which About sets the Horologium, hours and invitation as a right rail beside the prose.
private let aboutRailFrom: CGFloat = 840

struct AboutPage: View {
    @EnvironmentObject private var model: AppModel
    @State private var course: (year: Int32, months: [CourseMonthView])?

    var body: some View {
        let year = model.today.year
        AboutScreen(view: aboutPage, course: course?.year == year ? course?.months : nil)
            .screenshotReady(.about, ready: course?.year == year)
            .background(PlasterWall())
            .toolbar(.hidden, for: .navigationBar)
            .task(id: year) {
                if case let .success(months) = await compose({ try Office.core.get().ordoCourse(year: year) }), !Task.isCancelled {
                    course = (year, months)
                }
            }
    }
}

/**
 * About the Office, set as the web's `/about`: the feast page's painted frontispiece head, an
 * illuminated drop-cap introduction, the interactive Horologium over an open hand-ruled hours
 * directory, a live miniature office specimen and ruled key in Reading the page, chronological
 * marginalia, and the current year's liturgical color ribbon in The Ordo.
 */
struct AboutScreen: View {
    let view: AboutView
    var course: [CourseMonthView]? = nil
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.openURL) private var openURL

    @State private var pinnedHour: String?
    @State private var specimenForm = "private"
    @State private var litKey: String?
    @State private var selectedColor: String?

    /// The hour it is now, as home has it (`presentation::about`): nil only if the core refuses the date.
    private var now: AboutNow? { try? aboutNow(today: model.today, clockHour: Int32(model.clockHour)) }

    var body: some View {
        let now = self.now
        let currentHour = now?.current ?? ""
        let verse = view.blocks.first { $0.kind == "verse" }
        let intro = view.blocks.first { $0.kind == "intro" }?.text ?? ""
        let parsed = partitionAboutBlocks(view.blocks)

        GeometryReader { geo in
            let wide = geo.size.width >= aboutRailFrom
            ScrollView {
                VStack(spacing: 0) {
                    SiteHeader()
                    Group {
                        if wide {
                            VStack(alignment: .leading, spacing: 0) {
                                AboutHead(
                                    kicker: view.kicker,
                                    title: view.title,
                                    verseText: verse?.text ?? "",
                                    verseCite: verse?.cite ?? "",
                                    wide: true
                                )
                                HStack(alignment: .top, spacing: 51) {
                                    VStack(alignment: .leading, spacing: 0) {
                                        introBlock(intro)
                                        proseSections(parsed.sections)
                                    }
                                    .frame(maxWidth: .infinity, alignment: .leading)
                                    VStack(alignment: .leading, spacing: 0) {
                                        hoursColumn(parsed: parsed, current: currentHour)
                                        doorBlock(now: now, fillWidth: true)
                                    }
                                    .frame(width: min(376, geo.size.width * 0.36), alignment: .leading)
                                }
                                .padding(.top, m.px(8))
                            }
                            .frame(maxWidth: 1024)
                            .padding(.horizontal, 24)
                            .frame(maxWidth: .infinity)
                        } else {
                            VStack(alignment: .leading, spacing: 0) {
                                AboutHead(
                                    kicker: view.kicker,
                                    title: view.title,
                                    verseText: verse?.text ?? "",
                                    verseCite: verse?.cite ?? "",
                                    wide: false
                                )
                                introBlock(intro)
                                hoursColumn(parsed: parsed, current: currentHour)
                                    .padding(.top, m.px(22))
                                proseSections(parsed.sections)
                                doorBlock(now: now, fillWidth: false)
                            }
                            .measured(m)
                        }
                    }
                    .padding(.top, m.px(18))
                    Footer()
                }
            }
        }
        // A paragraph's links open the app's own pages, or the browser.
        .environment(\.openURL, OpenURLAction { url in
            switch aboutLink(url.absoluteString) {
            case .ordo?:
                let t = model.today
                model.open(.ordo(year: Int(t.year), month: Int(t.month), day: Int(t.day)))
                return .handled
            case .reminders?:
                model.open(.reminders)
                return .handled
            case let .web(site)?:
                openURL(site)
                return .handled
            case nil:
                return .discarded
            }
        })
    }

    @ViewBuilder private func introBlock(_ text: String) -> some View {
        if !text.isEmpty {
            BlockRow(block: BlockView(
                kind: .paragraph,
                marker: "",
                dropCap: true,
                startsElement: true,
                runs: [RunView(text: text, style: .plain, gloss: nil)],
                score: "",
                phrases: false,
                space: .none
            ), flushDropCap: true)
            .padding(.top, m.px(4))
        }
    }

    @ViewBuilder private func hoursColumn(parsed: AboutPartition, current: String) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            if !parsed.hoursHeading.isEmpty {
                AboutSectionHeading(text: parsed.hoursHeading, seed: 110)
            }
            AboutHorologium(
                periods: view.periods,
                current: current,
                pinned: pinnedHour,
                onSelect: { slug in pinnedHour = pinnedHour == slug ? nil : slug }
            )
            AboutHoursDirectory(
                periods: view.periods,
                current: current,
                preview: pinnedHour != current ? pinnedHour : nil
            )
            if !parsed.hoursNote.isEmpty {
                Text(parsed.hoursNote)
                    .type(Scale.bodyItalic.sized(15.04, line: 23))
                    .foregroundStyle(p.muted)
                    .padding(.top, m.px(10.4))
            }
        }
    }

    @ViewBuilder private func doorBlock(now: AboutNow?, fillWidth: Bool) -> some View {
        if let now {
            PrayNow(
                label: now.prayNowLabel, style: Scale.body.sized(19.2, line: 24.96), pad: 11.5,
                frontal: p.lining, wash: HomeInk.of(p).tabletWash, lifted: nil
            ) {
                model.open(.hour(now.prayNowDate, now.prayNowHour))
            }
            .frame(maxWidth: fillWidth ? .infinity : m.px(280))
            .frame(maxWidth: .infinity)
            .padding(.top, m.px(fillWidth ? 21.6 : 33.6))
            .padding(.bottom, m.px(6.4))
        }
    }

    @ViewBuilder private func proseSections(_ sections: [AboutSectionData]) -> some View {
        ForEach(Array(sections.enumerated()), id: \.offset) { idx, sec in
            VStack(alignment: .leading, spacing: 0) {
                AboutSectionHeading(text: sec.heading, seed: UInt32(111 + idx))
                let groups = groupSectionBlocks(sec.blocks)
                ForEach(Array(groups.enumerated()), id: \.offset) { gi, group in
                    switch group {
                    case let .paragraph(runs):
                        Text(linked(runs))
                            .type(Scale.body.sized(20, line: 31))
                            .foregroundStyle(p.text)
                            .padding(.bottom, gi + 1 < groups.count ? m.px(14.4) : 0)
                    case let .note(text):
                        Text(text)
                            .type(Scale.bodyItalic.sized(15.04, line: 23))
                            .foregroundStyle(p.muted)
                            .padding(.bottom, gi + 1 < groups.count ? m.px(14.4) : 0)
                    case .specimen:
                        AboutSpecimenLeaf(
                            specimen: view.specimen,
                            form: specimenForm,
                            litKey: litKey,
                            onForm: { specimenForm = $0 }
                        )
                        .padding(.bottom, m.px(16))
                    case let .keys(items):
                        AboutRuledKeyList(keys: items, litKey: litKey) { key in
                            litKey = litKey == key ? nil : key
                        }
                    case let .eras(items):
                        AboutLineageList(eras: items)
                    case .ordoRibbon:
                        AboutOrdoRibbon(
                            course: course,
                            today: model.today,
                            colors: view.ordoColors,
                            label: view.ribbonLabel,
                            selected: selectedColor,
                            onSelect: { slug in selectedColor = selectedColor == slug ? nil : slug }
                        )
                    }
                }
            }
            .padding(.top, m.px(33.6))
        }
    }

    /// A paragraph's runs, its links in the accent (the app's tint) over a gold hairline, as the web's `.about a`.
    private func linked(_ runs: [AboutRunView]) -> AttributedString {
        runs.reduce(into: AttributedString()) { out, run in
            var s = AttributedString(run.text)
            if !run.link.isEmpty, let url = URL(string: run.link) {
                s.link = url
                s[AttributeScopes.SwiftUIAttributes.UnderlineStyleAttribute.self] = Text.LineStyle(pattern: .solid, color: p.goldLine)
            }
            out += s
        }
    }
}

private struct AboutSectionData {
    let heading: String
    let blocks: [AboutBlockView]
}

private struct AboutPartition {
    let hoursHeading: String
    let hoursNote: String
    let sections: [AboutSectionData]
}

private enum AboutGroup {
    case paragraph([AboutRunView])
    case note(String)
    case specimen
    case keys([AboutBlockView])
    case eras([AboutBlockView])
    case ordoRibbon
}

private func partitionAboutBlocks(_ blocks: [AboutBlockView]) -> AboutPartition {
    var hoursHeading = ""
    var hoursNote = ""
    var inHours = false
    var sections: [AboutSectionData] = []
    var curHeading: String?
    var curBlocks: [AboutBlockView] = []
    func flush() {
        if let h = curHeading { sections.append(AboutSectionData(heading: h, blocks: curBlocks)) }
        curHeading = nil
        curBlocks = []
    }
    for (i, b) in blocks.enumerated() {
        switch b.kind {
        case "verse", "intro", "invitation": break
        case "heading":
            // The heading over the hours heads their column, whatever its words.
            if i + 1 < blocks.count && blocks[i + 1].kind == "hours" {
                flush()
                hoursHeading = b.text
                inHours = true
            } else {
                flush()
                inHours = false
                curHeading = b.text
            }
        case "hours": inHours = true
        case "note":
            if inHours { hoursNote = b.text } else { curBlocks.append(b) }
        default:
            curBlocks.append(b)
        }
    }
    flush()
    return AboutPartition(hoursHeading: hoursHeading, hoursNote: hoursNote, sections: sections)
}

private func groupSectionBlocks(_ blocks: [AboutBlockView]) -> [AboutGroup] {
    var groups: [AboutGroup] = []
    var i = 0
    while i < blocks.count {
        let b = blocks[i]
        switch b.kind {
        case "paragraph":
            groups.append(.paragraph(b.runs))
            i += 1
        case "note":
            groups.append(.note(b.text))
            i += 1
        case "specimen":
            groups.append(.specimen)
            i += 1
        case "key":
            var items: [AboutBlockView] = []
            while i < blocks.count && blocks[i].kind == "key" {
                items.append(blocks[i])
                i += 1
            }
            groups.append(.keys(items))
        case "era":
            var items: [AboutBlockView] = []
            while i < blocks.count && blocks[i].kind == "era" {
                items.append(blocks[i])
                i += 1
            }
            groups.append(.eras(items))
        case "ordo_ribbon":
            groups.append(.ordoRibbon)
            i += 1
        default:
            i += 1
        }
    }
    return groups
}

/**
 * The painted frontispiece head (echoing `FeastHead`): a full-width hand-ruled terracotta course
 * broken at the centre by the consecration cross, a soft wall glow behind it, the small-caps
 * kicker, the display title, and the red Psalm epigraph.
 */
private struct AboutHead: View {
    let kicker: String
    let title: String
    let verseText: String
    let verseCite: String
    let wide: Bool
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let crown = m.px(34.4)
        let gap = m.px(24)
        let titleSize: CGFloat = wide ? 40 : 33.6
        let light = p.dark ? Color(rgb: 150, 178, 220, 0.16) : Color(rgb: 255, 253, 248, 0.92)

        VStack(spacing: 0) {
            if !kicker.isEmpty {
                Text(kicker)
                    .type(TextStyle(size: 14.08, line: 19.7, tracking: 2.25, smallCaps: true))
                    .foregroundStyle(p.titulus)
                    .multilineTextAlignment(.center)
            }
            Text(title)
                .type(TextStyle(size: titleSize, line: titleSize * 1.12))
                .foregroundStyle(p.text)
                .multilineTextAlignment(.center)
                .padding(.top, m.px(2.4))
                .accessibilityAddTraits(.isHeader)
            if !verseText.isEmpty {
                Text(verseText)
                    .type(Scale.bodyItalic.sized(wide ? 18.6 : 17.6, line: 25))
                    .foregroundStyle(p.rubric)
                    .multilineTextAlignment(.center)
                    .padding(.top, m.px(11))
                Text(verseCite)
                    .type(TextStyle(size: 12.48, line: 18, tracking: 1.25, smallCaps: true))
                    .foregroundStyle(p.rubric.opacity(0.92))
                    .multilineTextAlignment(.center)
                    .padding(.top, m.px(2))
            }
        }
        .padding(.horizontal, m.px(8))
        .padding(.top, m.px(28))
        .padding(.bottom, m.px(wide ? 28 : 21))
        .frame(maxWidth: .infinity)
        .background {
            Canvas { ctx, size in
                let rx = size.width * 0.52
                let ry = size.height * 0.48
                guard rx > 0, ry > 0 else { return }
                var pool = ctx
                pool.translateBy(x: size.width / 2, y: size.height * 0.42)
                pool.scaleBy(x: 1, y: ry / rx)
                pool.fill(
                    Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)),
                    with: .radialGradient(Gradient(colors: [light, .clear]), center: .zero, startRadius: 0, endRadius: rx)
                )
            }
            .padding(.horizontal, -m.px(16))
            .padding(.top, -m.px(35))
            .padding(.bottom, -m.px(20))
            .allowsHitTesting(false)
            .accessibilityHidden(true)
        }
        .overlay(alignment: .top) {
            ZStack(alignment: .top) {
                Canvas { ctx, s in
                    let mid = s.width / 2
                    ctx.fill(ruledLine(0, 0.5, mid - gap, 0.5, 1.25, seed: 101), with: .color(p.lining))
                    ctx.fill(ruledLine(mid + gap, 0.5, s.width, 0.5, 1.25, seed: 102), with: .color(p.lining))
                }
                .frame(height: 6)
                Crown(size: crown, ink: p.lining).offset(y: -crown / 2)
            }
            .allowsHitTesting(false)
            .accessibilityHidden(true)
        }
        .padding(.top, m.px(18))
    }
}

/// A section's heading: a titulus in small capitals over one hand-ruled stroke of the lining.
private struct AboutSectionHeading: View {
    let text: String
    let seed: UInt32
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Text(text)
            .type(TextStyle(size: 15.68, line: 20.4, tracking: 2.35, smallCaps: true))
            .foregroundStyle(p.titulus)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.bottom, m.px(6))
            .overlay(alignment: .bottom) {
                RuledDivider(color: p.lining.opacity(0.78), weight: 1.25, seed: seed)
            }
            .padding(.bottom, m.px(13.6))
            .accessibilityAddTraits(.isHeader)
    }
}

/**
 * The painted Horologium (Arc of the Day): a semicircular solar path with seven tappable stations
 * from Dawn to Night. Tapping any station glides the sun/moon smoothly along the arc and shifts
 * the theme-faithful wash (honest warm plaster light in Nave; vaulted night sky with subtle
 * gold-leaf stars and a travelling lantern halo in Apse).
 */
private struct AboutHorologium: View {
    let periods: [AboutPeriodView]
    let current: String
    let pinned: String?
    let onSelect: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        let hours = periods.flatMap(\.hours)
        let activeSlug = pinned ?? (current.isEmpty ? "compline" : current)
        if let active = hours.first(where: { $0.hour == activeSlug }) ?? hours.last {
            let isNow = active.hour == current && !current.isEmpty
            let largeType = m.type > 1.35
            let readoutSpoken = "\(isNow ? "Now, " : "")\(active.time). \(active.name), \(active.gloss)"
            let readout = VStack(spacing: m.px(1)) {
                Text((isNow ? "Now · " : "") + active.time)
                    .type(TextStyle(size: 11.2, line: 14.5, tracking: 1.5, smallCaps: true))
                    .foregroundStyle(p.titulus)
                    .multilineTextAlignment(.center)
                (Text(active.name).foregroundColor(p.text)
                    + Text(" · " + active.gloss).italic().foregroundColor(p.muted))
                    .type(Scale.body.sized(18, line: 22))
                    .multilineTextAlignment(.center)
            }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(readoutSpoken)
            VStack(spacing: 0) {
                ZStack {
                    HorologiumCanvas(
                        hours: hours,
                        activeSlug: active.hour,
                        activeNight: active.night,
                        angle: active.angle,
                        p: p,
                        o: o
                    )
                    .animation(reduceMotion ? nil : .easeOut(duration: 0.42), value: active.angle)

                    GeometryReader { geo in
                        let w = geo.size.width
                        let h = geo.size.height
                        let cardinal = TextStyle(size: 8.5, line: 11, tracking: 1.2, smallCaps: true)

                        Text("Dawn").type(cardinal).foregroundStyle(p.muted)
                            .position(x: w * (42.0 / 320.0) + 14, y: h - 6)
                            .allowsHitTesting(false).accessibilityHidden(true)
                        Text("Noon").type(cardinal).foregroundStyle(p.muted)
                            .position(x: w / 2, y: 8)
                            .allowsHitTesting(false).accessibilityHidden(true)
                        Text("Night").type(cardinal).foregroundStyle(p.muted)
                            .position(x: w * (278.0 / 320.0) - 14, y: h - 6)
                            .allowsHitTesting(false).accessibilityHidden(true)

                        // Centered readout inside the dome belly at standard Dynamic Type
                        if !largeType {
                            readout
                                .frame(width: w * 0.72)
                                .position(x: w / 2, y: h * 0.67)
                                .allowsHitTesting(false)
                        }

                        // 44pt tap targets over each of the seven stations (Apple's minimum)
                        ForEach(Array(hours.enumerated()), id: \.offset) { _, hr in
                            let rad = hr.angle * .pi / 180.0
                            let px = w * CGFloat((160.0 + cos(rad) * 118.0) / 320.0)
                            let py = h * CGFloat((128.0 - sin(rad) * 104.0) / 144.0)
                            Button { onSelect(hr.hour) } label: {
                                Color.clear.frame(width: 44, height: 44).contentShape(Rectangle())
                            }
                            .buttonStyle(.plain)
                            .position(x: px, y: py)
                            .accessibilityLabel("\(hr.name): \(hr.gloss), \(hr.time)")
                            .accessibilityAddTraits(hr.hour == active.hour ? .isSelected : [])
                        }
                    }
                }
                .aspectRatio(320.0 / 144.0, contentMode: .fit)

                if largeType {
                    readout
                        .frame(maxWidth: .infinity)
                        .padding(.top, m.px(8))
                }
            }
            .padding(.top, m.px(3))
            .padding(.bottom, m.px(13.6))
        }
    }
}

private struct HorologiumCanvas: View, Animatable {
    let hours: [AboutHourView]
    let activeSlug: String
    let activeNight: Bool
    var angle: Double
    let p: Palette
    let o: Ornament

    var animatableData: Double {
        get { angle }
        set { angle = newValue }
    }

    private func washColors() -> (Color, Color, Color) {
        if p.dark {
            switch activeSlug {
            case "lauds", "vespers": return (Color(rgb: 208, 134, 102, 0.22), Color(rgb: 46, 72, 108, 0.28), Color(rgb: 18, 28, 40, 0))
            case "sext": return (Color(rgb: 232, 198, 122, 0.26), Color(rgb: 46, 72, 108, 0.26), Color(rgb: 18, 28, 40, 0))
            case "compline": return (Color(rgb: 154, 182, 224, 0.22), Color(rgb: 34, 56, 88, 0.32), Color(rgb: 18, 28, 40, 0))
            default: return (Color(rgb: 214, 178, 108, 0.22), Color(rgb: 46, 72, 108, 0.28), Color(rgb: 18, 28, 40, 0))
            }
        } else {
            switch activeSlug {
            case "lauds", "vespers": return (Color(rgb: 196, 118, 92, 0.20), Color(rgb: 216, 166, 86, 0.08), Color(rgb: 250, 243, 233, 0))
            case "sext": return (Color(rgb: 224, 178, 86, 0.26), Color(rgb: 224, 178, 86, 0.09), Color(rgb: 250, 243, 233, 0))
            case "compline": return (Color(rgb: 146, 134, 122, 0.16), Color(rgb: 146, 134, 122, 0.05), Color(rgb: 250, 243, 233, 0))
            default: return (Color(rgb: 216, 166, 86, 0.22), Color(rgb: 216, 166, 86, 0.07), Color(rgb: 250, 243, 233, 0))
            }
        }
    }

    private func ellipseArc(cx: CGFloat, cy: CGFloat, rx: CGFloat, ry: CGFloat, fromDeg: Double, toDeg: Double, steps: Int = 56, close: Bool = false) -> Path {
        var path = Path()
        for i in 0...steps {
            let t = Double(i) / Double(steps)
            let rad = (fromDeg + (toDeg - fromDeg) * t) * .pi / 180.0
            let pt = CGPoint(x: cx + CGFloat(cos(rad)) * rx, y: cy - CGFloat(sin(rad)) * ry)
            if i == 0 { path.move(to: pt) } else { path.addLine(to: pt) }
        }
        if close { path.closeSubpath() }
        return path
    }

    var body: some View {
        Canvas { ctx, size in
            let sx = size.width / 320
            let sy = size.height / 144
            let cx = 160 * sx
            let cy = 128 * sy
            let rx = 118 * sx
            let ry = 104 * sy

            func arcPt(_ deg: Double, factor: CGFloat = 1) -> CGPoint {
                let rad = deg * .pi / 180.0
                return CGPoint(x: cx + CGFloat(cos(rad)) * rx * factor, y: cy - CGFloat(sin(rad)) * ry * factor)
            }

            let dome = ellipseArc(cx: cx, cy: cy, rx: rx, ry: ry, fromDeg: 180, toDeg: 0, close: true)
            let orb = arcPt(angle)
            let (w0, w1, w2) = washColors()
            let washR = max(size.width, size.height) * 0.68
            ctx.fill(
                dome,
                with: .radialGradient(
                    Gradient(stops: [.init(color: w0, location: 0), .init(color: w1, location: 0.52), .init(color: w2, location: 1)]),
                    center: orb,
                    startRadius: 0,
                    endRadius: washR
                )
            )

            if p.dark {
                var clipped = ctx
                clipped.clip(to: dome)
                let stars: [(CGFloat, CGFloat, CGFloat)] = [
                    (102, 56, 1.3), (134, 38, 1.6), (176, 35, 1.2), (218, 52, 1.5),
                    (82, 84, 1.1), (154, 54, 1.0), (238, 82, 1.2), (192, 62, 1.0),
                ]
                for (x, y, r) in stars {
                    let rr = r * sx
                    clipped.fill(Path(ellipseIn: CGRect(x: x * sx - rr, y: y * sy - rr, width: 2 * rr, height: 2 * rr)), with: .color(o.hi.opacity(0.62)))
                }
            }

            // Station ticks
            for hr in hours {
                var tick = Path()
                tick.move(to: arcPt(hr.angle, factor: 0.91))
                tick.addLine(to: arcPt(hr.angle, factor: 1.0))
                ctx.stroke(tick, with: .color(p.lining.opacity(0.32)), lineWidth: 0.7 * sx)
            }

            // Inner dashed arc
            let inner = ellipseArc(cx: cx, cy: cy, rx: rx * 0.93, ry: ry * 0.93, fromDeg: 180, toDeg: 0)
            ctx.stroke(inner, with: .color(p.lining.opacity(0.32)), style: StrokeStyle(lineWidth: 0.6 * sx, dash: [2 * sx, 4 * sx]))

            // Main solar arc
            let main = ellipseArc(cx: cx, cy: cy, rx: rx, ry: ry, fromDeg: 180, toDeg: 0)
            ctx.stroke(main, with: .color(p.lining.opacity(0.52)), lineWidth: 1.15 * sx)

            // Lit arc from Lauds (168°) to orb
            let lit = ellipseArc(cx: cx, cy: cy, rx: rx, ry: ry, fromDeg: 168, toDeg: min(167.5, angle), steps: 40)
            ctx.stroke(lit, with: .color(o.flat.opacity(0.78)), lineWidth: 1.5 * sx)

            // Horizon line
            var horizon = Path()
            horizon.move(to: CGPoint(x: 16 * sx, y: cy))
            horizon.addLine(to: CGPoint(x: 304 * sx, y: cy))
            ctx.stroke(horizon, with: .color(p.lining.opacity(0.38)), lineWidth: 0.85 * sx)

            // Non-active diamond stations
            for hr in hours where hr.hour != activeSlug {
                let pt = arcPt(hr.angle)
                let h = 3.8 * sx
                var d = Path()
                d.move(to: CGPoint(x: pt.x, y: pt.y - h))
                d.addLine(to: CGPoint(x: pt.x + h, y: pt.y))
                d.addLine(to: CGPoint(x: pt.x, y: pt.y + h))
                d.addLine(to: CGPoint(x: pt.x - h, y: pt.y))
                d.closeSubpath()
                ctx.fill(d, with: .color(p.bg))
                ctx.stroke(d, with: .color(p.lining.opacity(0.85)), lineWidth: 1.2 * sx)
            }

            // Active Sun or Moon orb with radial halo
            let haloR = 15 * sx
            ctx.fill(
                Path(ellipseIn: CGRect(x: orb.x - haloR, y: orb.y - haloR, width: 2 * haloR, height: 2 * haloR)),
                with: .radialGradient(
                    Gradient(stops: [
                        .init(color: o.hi.opacity(p.dark ? 0.42 : 0.36), location: 0),
                        .init(color: o.flat.opacity(0.12), location: 0.55),
                        .init(color: .clear, location: 1),
                    ]),
                    center: orb,
                    startRadius: 0,
                    endRadius: haloR
                )
            )
            let gScale = (activeNight ? 0.80 : 0.875) * sx
            let half = 12 * gScale
            var glyph = ctx
            glyph.translateBy(x: orb.x - half, y: orb.y - half)
            glyph.scaleBy(x: gScale, y: gScale)
            glyph.fill(activeNight ? moonPath : sunPath, with: .color(o.flat))
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * The seven hours in home's three periods, ruled as home's painted page: firm hand-ruled terracotta
 * frame above and below, medium dividers between periods, faint hairlines between hours inside a
 * period, and zero vertical rules or shaded cells.
 */
private struct AboutHoursDirectory: View {
    let periods: [AboutPeriodView]
    let current: String
    let preview: String?
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    private var nameStyle: TextStyle { TextStyle(size: 18.56, line: 23.2) }

    var body: some View {
        let frameInk = p.lining.opacity(0.78)
        let innerInk = p.lining.opacity(0.36)
        let hairInk = p.lining.opacity(0.19)
        let periodInk = mix(p.accent, p.muted, 0.45)
        let label = TextStyle(size: 11.2, line: 13.4, tracking: 11.2 * 0.12, smallCaps: true)
        let font = nameStyle.uiFont(m)
        let nameWidth = ceil(periods.flatMap(\.hours).map { ProseLayout.advance($0.name, font) }.max() ?? 0) + 1

        VStack(spacing: 0) {
            ForEach(Array(periods.enumerated()), id: \.offset) { i, period in
                RuledDivider(color: i == 0 ? frameInk : innerInk, weight: i == 0 ? 1.25 : 0.9, seed: UInt32(130 + i))
                HStack(spacing: 0) {
                    VStack(spacing: m.px(2.56)) {
                        PeriodIcon(period: Period.named(period.label), color: o.flat.opacity(0.9))
                        Text(period.label).type(label).foregroundStyle(periodInk).lineLimit(1).fixedSize()
                    }
                    .padding(.horizontal, m.px(4))
                    .frame(minWidth: m.px(78))
                    .frame(maxHeight: .infinity)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(period.label)
                    .accessibilityAddTraits(.isHeader)

                    VStack(spacing: 0) {
                        ForEach(Array(period.hours.enumerated()), id: \.offset) { j, h in
                            if j > 0 { RuledDivider(color: hairInk, weight: 0.75, seed: UInt32(140 + i * 5 + j)) }
                            row(h, nameWidth: nameWidth)
                        }
                    }
                }
                .fixedSize(horizontal: false, vertical: true)
            }
            RuledDivider(color: frameInk, weight: 1.25, seed: 139)
        }
    }

    private func row(_ h: AboutHourView, nameWidth: CGFloat) -> some View {
        let isNow = h.hour == current
        let isPreview = h.hour == preview
        return Button { model.open(.hour(model.today, h.hour)) } label: {
            HStack(alignment: .firstTextBaseline, spacing: m.px(10.4)) {
                Text(h.name).type(nameStyle).foregroundStyle(isNow ? titulusNow(p, season: model.season) : p.text)
                    .lineLimit(1)
                    .fixedSize()
                    .frame(width: nameWidth, alignment: .leading)
                VStack(alignment: .leading, spacing: 0) {
                    Text(h.gloss).type(TextStyle(size: 16.32, line: 20.9, italic: true)).foregroundStyle(p.text)
                    Text(h.time).type(TextStyle(size: 13.44, line: 17.5)).foregroundStyle(p.muted)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(.vertical, m.px(7.7))
            .padding(.leading, m.px(7))
            .padding(.trailing, m.px(9))
            .background {
                if isNow {
                    PoolAlongRow(alpha: 1)
                } else if isPreview {
                    PoolAlongRow(alpha: 0.65)
                }
            }
        }
        .buttonStyle(Lit { PoolOfLight(at: UnitPoint(x: 0.3, y: 0.5), size: CGSize(width: 0.88, height: 1.16)) })
        .accessibilityElement(children: .ignore)
        .accessibilityLabel([h.name, h.gloss, h.time, isNow ? "now" : nil].compactMap { $0 }.joined(separator: ", "))
        .accessibilityAddTraits(.isButton)
    }
}

/**
 * The live miniature office specimen in "Reading the page": framed in home's double hand-ruled
 * course over the feathered tablet wash, with the hour's Private / Deacon / Priest switch and
 * soft pool-of-light illumination tied to the ruled key below.
 */
private struct AboutSpecimenLeaf: View {
    let specimen: AboutSpecimenView
    let form: String
    let litKey: String?
    let onForm: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    private var lineStyle: TextStyle { Scale.body.sized(16.5, line: 24) }

    var body: some View {
        let wash = HomeInk.of(p).tabletWash
        VStack(alignment: .leading, spacing: 0) {
            let label = Text(specimen.label)
                .type(TextStyle(size: 11.84, line: 16, tracking: 1.65, smallCaps: true))
                .foregroundStyle(p.titulus)
                .fixedSize(horizontal: true, vertical: false)
            let toggle = HStack(spacing: m.px(8)) {
                formTab(specimen.aloneLabel, selected: form == "private") { onForm("private") }
                Rectangle().fill(o.flat.opacity(0.72)).frame(width: 3.5, height: 3.5).rotationEffect(.degrees(45))
                formTab(specimen.deaconLabel, selected: form == "deacon") { onForm("deacon") }
                Rectangle().fill(o.flat.opacity(0.72)).frame(width: 3.5, height: 3.5).rotationEffect(.degrees(45))
                formTab(specimen.priestLabel, selected: form == "priest") { onForm("priest") }
            }
            .fixedSize(horizontal: true, vertical: false)

            ViewThatFits(in: .horizontal) {
                HStack(alignment: .firstTextBaseline) {
                    label
                    Spacer(minLength: 8)
                    toggle
                }
                VStack(alignment: .leading, spacing: m.px(4)) {
                    label
                    toggle
                }
            }
            .padding(.bottom, m.px(8))
            .background { if litKey == "form" { SpecimenLitPool() } }

            RuledDivider(color: p.lining.opacity(0.36), weight: 0.85, seed: 169)
                .padding(.bottom, m.px(10))

            // Lauds' opening as the hour prints it: its rubric, the secret Our Father, the ✠ where it falls.
            Text(specimen.rubric)
                .type(Scale.bodyItalic.sized(15, line: 22))
                .foregroundStyle(p.rubric)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.bottom, m.px(6))
                .background { if litKey == "rubric" { SpecimenLitPool() } }

            (Text(specimen.secretVoiced + " ").foregroundColor(p.text)
                + Text(specimen.secretSilent).foregroundColor(p.dark ? p.unsaid : p.muted))
                .type(lineStyle)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.bottom, m.px(8))
                .background { if litKey == "secret" { SpecimenLitPool() } }

            RuledDivider(color: p.lining.opacity(0.19), weight: 0.75, seed: 171)
                .padding(.bottom, m.px(8))

            HStack(alignment: .firstTextBaseline, spacing: 0) {
                Text("℣.").type(lineStyle).foregroundStyle(p.rubric).frame(width: m.px(27), alignment: .leading)
                openingVersicle
                    .type(lineStyle)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(.bottom, m.px(4))
            .background { if litKey == "vr" || litKey == "cross" { SpecimenLitPool() } }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("Versicle. " + specimen.openingVersicle.replacingOccurrences(of: "✠", with: "sign of the cross,"))

            HStack(alignment: .firstTextBaseline, spacing: 0) {
                Text("℟.").type(lineStyle).foregroundStyle(p.rubric).frame(width: m.px(27), alignment: .leading)
                Text(specimen.openingResponse).type(lineStyle).foregroundStyle(p.text)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(.bottom, m.px(8))
            .background { if litKey == "vr" { SpecimenLitPool() } }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("Response. \(specimen.openingResponse)")

            // The close's greeting, as the chosen form gives it: a deacon greets as a priest does.
            RuledDivider(color: p.lining.opacity(0.19), weight: 0.75, seed: 170)

            let clergy = form != "private"
            let formV = clergy ? specimen.priestVersicle : specimen.aloneVersicle
            let formR = clergy ? specimen.priestResponse : specimen.aloneResponse
            VStack(alignment: .leading, spacing: m.px(4)) {
                HStack(alignment: .firstTextBaseline, spacing: 0) {
                    Text("℣.").type(lineStyle).foregroundStyle(p.rubric).frame(width: m.px(27), alignment: .leading)
                    Text(formV).type(lineStyle).foregroundStyle(p.text)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                HStack(alignment: .firstTextBaseline, spacing: 0) {
                    Text("℟.").type(lineStyle).foregroundStyle(p.rubric).frame(width: m.px(27), alignment: .leading)
                    Text(formR).type(lineStyle).foregroundStyle(p.text)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
            .padding(.top, m.px(7))
            .background { if litKey == "form" { SpecimenLitPool() } }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("Versicle. \(formV) Response. \(formR)")
        }
        .padding(.horizontal, m.px(18))
        .padding(.vertical, m.px(15))
        .frame(maxWidth: .infinity, alignment: .leading)
        .background {
            if let tablet = UIImage(named: "tablet.png") {
                Image(uiImage: tablet).renderingMode(.template).resizable().foregroundStyle(wash).padding(1)
                    .accessibilityHidden(true)
            }
        }
        .overlay {
            Canvas { ctx, size in invitationRules(ctx, size, lining: p.lining, frontal: o.flat, seed: 161, share: 0.58) }
                .allowsHitTesting(false)
                .accessibilityHidden(true)
        }
        .onChange(of: form) { _, next in
            let clergy = next != "private"
            let v = clergy ? specimen.priestVersicle : specimen.aloneVersicle
            let r = clergy ? specimen.priestResponse : specimen.aloneResponse
            UIAccessibility.post(notification: .announcement, argument: "Versicle. \(v) Response. \(r)")
        }
    }

    /// The opening versicle with its ✠ in the rubrics' red, where the hour prints it.
    private var openingVersicle: Text {
        let parts = specimen.openingVersicle.components(separatedBy: "✠")
        guard parts.count == 2 else { return Text(specimen.openingVersicle).foregroundColor(p.text) }
        return Text(parts[0]).foregroundColor(p.text) + Text("✠").foregroundColor(p.rubric) + Text(parts[1]).foregroundColor(p.text)
    }

    private func formTab(_ label: String, selected: Bool, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(label)
                .type(TextStyle(size: 12.48, line: 16, tracking: 1.25, smallCaps: true))
                .foregroundStyle(selected ? p.text : p.muted)
                .padding(.vertical, 3)
                .overlay(alignment: .bottom) {
                    if selected { Rectangle().fill(p.gold).frame(height: 1.5) }
                }
        }
        .buttonStyle(Quiet())
        .accessibilityAddTraits(selected ? .isSelected : [])
    }
}

private struct SpecimenLitPool: View {
    @Environment(\.palette) private var p

    var body: some View {
        let color = HomeInk.of(p).nowPool
        Canvas { ctx, size in
            let rx = size.width * 0.68
            let ry = size.height * 0.85
            guard rx > 0, ry > 0 else { return }
            var pool = ctx
            pool.translateBy(x: size.width * 0.32, y: size.height * 0.5)
            pool.scaleBy(x: 1, y: ry / rx)
            pool.fill(
                Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)),
                with: .radialGradient(poolGradient(color), center: .zero, startRadius: 0, endRadius: rx)
            )
        }
        .padding(.horizontal, -8)
        .padding(.vertical, -2)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/// The open hand-ruled 5-row typographic key beneath the specimen leaf.
private struct AboutRuledKeyList: View {
    let keys: [AboutBlockView]
    let litKey: String?
    let onToggle: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let desc = Scale.body.sized(15.5, line: 23)
        VStack(spacing: 0) {
            ForEach(Array(keys.enumerated()), id: \.offset) { i, k in
                if i > 0 { RuledDivider(color: p.lining.opacity(0.19), weight: 0.8, seed: UInt32(180 + i)) }
                let active = litKey == k.id
                Button { onToggle(k.id) } label: {
                    HStack(alignment: .firstTextBaseline, spacing: m.px(10)) {
                        keyMark(k, desc: desc)
                            .frame(width: m.px(104), alignment: .leading)
                        Text(k.text).type(desc).foregroundStyle(p.text)
                            .multilineTextAlignment(.leading)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                    .padding(.vertical, m.px(8.3))
                    .padding(.horizontal, m.px(3))
                    .background { if active { PoolAlongRow(alpha: 0.85) } }
                }
                .buttonStyle(Quiet())
                .accessibilityLabel("\(spokenMark(k)): \(k.text)")
                .accessibilityAddTraits(active ? .isSelected : [])
            }
            RuledDivider(color: p.lining.opacity(0.36), weight: 0.9, seed: 189)
        }
    }

    private func spokenMark(_ k: AboutBlockView) -> String {
        switch k.ink {
        case "symbol": return "Versicle and response"
        case "cross": return "Cross"
        case "secret": return [k.mark, k.tail].filter { !$0.isEmpty }.joined(separator: " ")
        default: return k.mark
        }
    }

    @ViewBuilder private func keyMark(_ k: AboutBlockView, desc: TextStyle) -> some View {
        switch k.ink {
        case "rubric":
            Text(k.mark).type(Scale.bodyItalic.sized(15.5, line: 23)).foregroundStyle(p.rubric)
        case "symbol":
            Text(k.mark).type(TextStyle(size: 15.5, line: 23, tracking: 1.3)).foregroundStyle(p.rubric)
        case "cross":
            Text(k.mark).type(Scale.body.sized(16.8, line: 23)).foregroundStyle(p.rubric)
        case "secret":
            (Text(k.mark + " ").foregroundColor(p.text) + Text(k.tail).foregroundColor(p.dark ? p.unsaid : p.muted)).type(desc)
        case "form":
            Text(k.mark).type(TextStyle(size: 13.44, line: 20, tracking: 1.35, smallCaps: true)).foregroundStyle(p.titulus)
        default:
            Text(k.mark).type(desc).foregroundStyle(p.text)
        }
    }
}

/// The three chronological beats in "An ancient prayer", along a hand-ruled marginal thread.
private struct AboutLineageList: View {
    let eras: [AboutBlockView]
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(Array(eras.enumerated()), id: \.offset) { i, era in
                HStack(alignment: .top, spacing: m.px(14)) {
                    Rectangle().fill(o.flat).frame(width: 6, height: 6).rotationEffect(.degrees(45))
                        .padding(.top, m.px(7))
                        .accessibilityHidden(true)
                    VStack(alignment: .leading, spacing: m.px(4)) {
                        Text(era.period)
                            .type(TextStyle(size: 13.12, line: 18, tracking: 1.45, smallCaps: true))
                            .foregroundStyle(p.titulus)
                            .accessibilityAddTraits(.isHeader)
                        Text(era.text)
                            .type(Scale.body.sized(20, line: 31))
                            .foregroundStyle(p.text)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
                .padding(.bottom, i == eras.count - 1 ? 0 : m.px(16.8))
            }
        }
        .padding(.top, m.px(4))
        .background(alignment: .leading) {
            Canvas { ctx, size in
                let y0 = m.px(10)
                let y1 = max(y0 + 1, size.height - m.px(24))
                ctx.fill(ruledLine(3, y0, 3, y1, 0.9, seed: 195), with: .color(p.lining.opacity(0.36)))
            }
            .frame(width: 6)
            .allowsHitTesting(false)
            .accessibilityHidden(true)
        }
    }
}

/// The current year's liturgical color ribbon and interactive 6-color diamond legend in "The Ordo".
private struct AboutOrdoRibbon: View {
    let course: [CourseMonthView]?
    let today: CivilDate
    let colors: [AboutOrdoColorView]
    /// What VoiceOver says for the drawn ribbon.
    let label: String
    let selected: String?
    let onSelect: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    private func ribbonColor(_ slug: String) -> Color {
        if p.dark && slug == "black" { return Color(hex: 0x6A7380) }
        return dayColor(slug, p)
    }

    var body: some View {
        let span = Int(courseSpan())
        let gap = 3
        let total = 12 * span + 11 * gap

        VStack(alignment: .leading, spacing: 0) {
            if let course, !course.isEmpty {
                let day = min(max(Int(today.day), 1), span)
                let todayFrac = CGFloat((Int(today.month) - 1) * (span + gap) + day - 1) + 0.5
                let frac = todayFrac / CGFloat(total)

                Canvas { ctx, size in
                    let unit = size.width / CGFloat(total)
                    for (mi, month) in course.enumerated() {
                        let ox = mi * (span + gap)
                        var end = 0
                        for r in month.runs {
                            let col = ribbonColor(r.color)
                            let alpha = (selected == nil || selected == r.color) ? 1.0 : 0.16
                            ctx.fill(
                                Path(CGRect(x: CGFloat(ox + Int(r.start)) * unit, y: 0, width: CGFloat(r.days) * unit, height: 5)),
                                with: .color(col.opacity(alpha))
                            )
                            end = max(end, Int(r.start + r.days))
                        }
                        if end < span {
                            ctx.fill(
                                Path(CGRect(x: CGFloat(ox + end) * unit, y: 2, width: CGFloat(span - end) * unit, height: 1)),
                                with: .color(p.lining.opacity(0.22))
                            )
                        }
                    }
                    // Today's gilt diamond hanging beneath today's place
                    let x = size.width * frac
                    let r: CGFloat = 2.8
                    var lozenge = Path()
                    lozenge.move(to: CGPoint(x: x, y: 8))
                    lozenge.addLine(to: CGPoint(x: x + r, y: 8 + r))
                    lozenge.addLine(to: CGPoint(x: x, y: 8 + 2 * r))
                    lozenge.addLine(to: CGPoint(x: x - r, y: 8 + r))
                    lozenge.closeSubpath()
                    ctx.fill(lozenge, with: .color(o.flat))
                }
                .frame(height: 15)
                .accessibilityElement()
                .accessibilityLabel(label)

                // Each name stands at its own month's start, as the web's (view.rs AboutRibbonLabel).
                GeometryReader { geo in
                    ForEach(Array(["Jan", "Mar", "May", "Jul", "Sep", "Nov"].enumerated()), id: \.offset) { i, label in
                        Text(label).type(TextStyle(size: 10.88, line: 14, tracking: 0.9, smallCaps: true)).foregroundStyle(p.muted)
                            .fixedSize()
                            .offset(x: geo.size.width * CGFloat(2 * i * (span + gap)) / CGFloat(total))
                    }
                }
                .frame(height: m.px(14))
                .padding(.top, 2)
                .accessibilityHidden(true)
            }

            MetaLine(space: m.px(14)) {
                ForEach(Array(colors.enumerated()), id: \.offset) { _, c in
                    let isSelected = selected == c.slug
                    Button { onSelect(c.slug) } label: {
                        HStack(spacing: 5.5) {
                            Rectangle().fill(ribbonColor(c.slug)).frame(width: 5.5, height: 5.5).rotationEffect(.degrees(45))
                            Text(c.name).type(Scale.body.sized(13.76, line: 18)).foregroundStyle(isSelected ? p.text : p.muted)
                        }
                        .padding(.vertical, 3)
                        .overlay(alignment: .bottom) {
                            if isSelected { Rectangle().fill(p.gold).frame(height: 1.5) }
                        }
                    }
                    .buttonStyle(Quiet())
                    .accessibilityLabel("\(c.name): \(c.meaning)")
                    .accessibilityAddTraits(isSelected ? .isSelected : [])
                }
            }
            .padding(.top, m.px(10))

            if let active = colors.first(where: { $0.slug == selected }) {
                Text("\(active.name) — \(active.meaning)")
                    .type(Scale.bodyItalic.sized(14.4, line: 20))
                    .foregroundStyle(p.muted)
                    .padding(.top, m.px(6))
            }
        }
        .padding(.top, m.px(14))
        .padding(.bottom, m.px(4))
    }
}

/// One horizontal hand-ruled stroke (`ruledLine`) across the full width.
private struct RuledDivider: View {
    let color: Color
    let weight: CGFloat
    let seed: UInt32

    var body: some View {
        Canvas { ctx, size in
            ctx.fill(ruledLine(0, size.height / 2, size.width, size.height / 2, weight, seed: seed), with: .color(color))
        }
        .frame(height: max(1, weight))
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * Home's pool of light (`poolGradient`), laid along an About row as the web's
 * `.about-hour.is-current::before`: an ellipse half the row wide and nearly its height, brightest
 * behind the name (29% across, a little under the middle), feathered in from the row's start so
 * no edge of it lies on a rule.
 */
private struct PoolAlongRow: View {
    var alpha: Double = 1
    @Environment(\.palette) private var p

    var body: some View {
        let color = HomeInk.of(p).nowPool.opacity(alpha)
        Canvas { ctx, size in
            let rx = size.width * 0.5
            let ry = size.height * 0.46
            guard rx > 0, ry > 0 else { return }
            var pool = ctx
            pool.translateBy(x: size.width * 0.29, y: size.height * 0.46)
            pool.scaleBy(x: 1, y: ry / rx)
            pool.fill(
                Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)),
                with: .radialGradient(poolGradient(color), center: .zero, startRadius: 0, endRadius: rx)
            )
        }
        .mask(LinearGradient(stops: [.init(color: .clear, location: 0), .init(color: .black, location: 0.14)], startPoint: .leading, endPoint: .trailing))
        .padding(.vertical, 1)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

extension Period {
    /// A period by its table's label, as the web's `period_ornament`: the moon for any but morning and day.
    static func named(_ label: String) -> Period {
        switch label.lowercased() {
        case "morning": return .morning
        case "day": return .day
        default: return .evening
        }
    }
}
