import Observation
import SwiftUI
import UIKit

/// The Lauds, Prime and Vespers preparation opens, as on the web; the other hours' stays closed.
private let openPreparation: Set<String> = ["lauds", "prime", "vespers"]

/**
 * What a page shows once its content is composed. Composing reads the corpus, so it runs off
 * the main thread; the page keeps what it had until the new content is ready, then moves to it
 * as `motion` says, from the top of the new page.
 */
struct Loaded<Value, Content: View>: View {
    let key: String
    let motion: Motion
    let load: () throws -> Value
    let content: (Value) -> Content
    @State private var shown: (key: String, value: Value)?
    @State private var failure: String?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    init(key: String, motion: Motion = .fade, load: @escaping () throws -> Value, @ViewBuilder content: @escaping (Value) -> Content) {
        self.key = key
        self.motion = motion
        self.load = load
        self.content = content
    }

    var body: some View {
        ZStack {
            if let shown {
                content(shown.value).id(shown.key).transition(transition)
            } else if let failure {
                Message(text: failure)
            } else {
                Message(text: "Preparing the office…")
            }
        }
        .task(id: key) {
            let result = await compose(load)
            switch result {
            case let .success(v):
                failure = nil
                // The first content simply appears (the stack's push brings it); later
                // content replaces it in motion.
                if shown == nil {
                    shown = (key, v)
                } else {
                    withAnimation(.easeInOut(duration: 0.3)) { shown = (key, v) }
                }
            case let .failure(e):
                shown = nil
                failure = "\(e)"
            }
        }
    }

    private var transition: AnyTransition { inPlace(motion, reduceMotion: reduceMotion) }
}

/**
 * How a page replaced in place moves, as the Android app's do along Material's shared axis:
 * later from the trailing side, earlier from the leading, the leaving page gone before the
 * coming one settles; a fade where there is no order, or the reader has asked for less motion.
 */
func inPlace(_ motion: Motion, reduceMotion: Bool) -> AnyTransition {
    let fadeIn = AnyTransition.opacity.animation(.easeOut(duration: 0.21).delay(0.09))
    let fadeOut = AnyTransition.opacity.animation(.easeIn(duration: 0.09))
    guard motion != .fade, !reduceMotion else { return .asymmetric(insertion: fadeIn, removal: fadeOut) }
    let on: CGFloat = motion == .next ? 1 : -1
    return .asymmetric(
        insertion: AnyTransition.offset(x: 30 * on).combined(with: fadeIn),
        removal: AnyTransition.offset(x: -30 * on).combined(with: fadeOut)
    )
}

/// Runs `load` on a background queue.
func compose<V>(_ load: @escaping () throws -> V) async -> Result<V, Error> {
    await withCheckedContinuation { done in
        DispatchQueue.global(qos: .userInitiated).async {
            done.resume(returning: Result { try load() })
        }
    }
}

/// An hour of a day, in the reader's prayer form, and Prime with the Martyrology when the reader reads it.
struct HourPage: View {
    let date: CivilDate
    let hour: String
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let form = model.form
        let martyrology = model.martyrology
        Loaded(key: "\(date.iso) \(hour) \(form) \(martyrology)", motion: model.motion(to: .hour(date, hour))) {
            try Office.core.get().compose(hour: hour, year: date.year, month: date.month, day: date.day, form: form, martyrology: martyrology)
        } content: { view in
            HourScreen(view: view, date: date)
                .onAppear { model.composed(view, page: .hour(date, hour)) }
        }
        .background(PlasterWall())
        .toolbar(.hidden, for: .navigationBar)
        // Like the web's wake lock: the screen stays on while an hour is open.
        .onAppear { UIApplication.shared.isIdleTimerDisabled = true }
        .onDisappear { UIApplication.shared.isIdleTimerDisabled = false }
    }
}

/**
 * One row of an hour's page: a collapsible section's toggle, or a block with the space above it
 * and, before a heading between the office's parts, the small painted cross.
 */
private enum Row: Identifiable {
    case toggle(section: Int, label: String, open: Bool)
    case block(BlockAt, BlockView, gap: CGFloat, cross: Bool)

    var id: String {
        switch self {
        case let .toggle(i, _, _): return "toggle-\(i)"
        case let .block(at, _, _, _): return "\(at.section)-\(at.block)"
        }
    }
}

/// An hour, as the web's hour page: colour band, header, title, controls, the office, its epilogue.
struct HourScreen: View {
    let view: HourView
    let date: CivilDate
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    /// The collapsible sections, open or not.
    @State private var open: [Int: Bool]
    /// How far through the office, for the hairline across the top.
    @State private var reading = Reading()
    /// The page's width, and the heights of what stands above and below a first-class day's frame.
    @State private var width: CGFloat = 0
    @State private var headerHeight: CGFloat = 0
    @State private var epilogueHeight: CGFloat = 0

    init(view: HourView, date: CivilDate) {
        self.view = view
        self.date = date
        var open: [Int: Bool] = [:]
        for (i, s) in view.sections.enumerated() where s.collapsible { open[i] = openPreparation.contains(view.hour) }
        _open = State(initialValue: open)
    }

    var body: some View {
        let o = Ornament.of(p, season: view.ornament)
        // From 920pt the prayer's text is a step larger, 21px for 20 (the web's `.elements` from
        // 920px, where it read a touch small on a laptop); the header, the title and the epilogue
        // keep their type. Only `type` steps, so the office's own spacing stands.
        let prayer: Metrics = {
            var scaled = m
            if width >= m.px(widePrayerFrom) { scaled.type *= widePrayerStep }
            return scaled
        }()
        let columns = hymnColumns(view.sections, p, o, prayer)
        let index = model.hours.firstIndex(of: view.hour) ?? 0
        let rows = self.rows
        // A first-class day's frame takes a little of a phone's measure (the web's 12px side
        // columns under 920px); where the lines stand outside the measure, the text keeps it.
        let inset: CGFloat = view.firstClass && width < m.px(frameClear) ? m.px(12) : 0
        ScrollViewReader { scroll in
            ScrollView {
                LazyVStack(spacing: 0) {
                    Rectangle().fill(dayColor(view.color)).frame(height: 3).accessibilityHidden(true)
                    SiteHeader()
                        .onGeometryChange(for: CGFloat.self) { $0.size.height } action: { headerHeight = $0 }
                    HourTitle(view: view, date: date)
                        .padding(.horizontal, inset)
                    ForEach(Array(rows.enumerated()), id: \.element.id) { n, row in
                        Group {
                            switch row {
                            case let .toggle(i, label, expanded):
                                Button { withAnimation(unfolding) { open[i] = !expanded } } label: {
                                    HStack(spacing: 0) {
                                        Text(label).type(Scale.heading).foregroundStyle(p.titulus)
                                        Caret(open: expanded, color: p.titulus)
                                    }
                                    .frame(maxWidth: .infinity, minHeight: 44)
                                }
                                .buttonStyle(Quiet())
                                .accessibilityAddTraits(.isHeader)
                                .accessibilityValue(expanded ? "Expanded" : "Collapsed")
                                .padding(.top, m.px(5.6))
                                .padding(.bottom, expanded ? m.px(12.8) : 0)
                                .measured(m)
                                .padding(.horizontal, inset)
                                .environment(\.metrics, prayer)
                            case let .block(at, block, gap, cross):
                                VStack(spacing: 0) {
                                    if cross {
                                        PaintedMark(.cross, size: m.px(9.92), color: p.lining).padding(.bottom, m.px(11.2))
                                    }
                                    BlockRow(block: block, column: columns[at])
                                }
                                .padding(.top, m.px(gap))
                                .measured(m)
                                .padding(.horizontal, inset)
                                .environment(\.metrics, prayer)
                            }
                        }
                        .read(n, into: reading)
                    }
                    Epilogue(
                        previous: index > 0 ? model.hours[index - 1] : nil,
                        next: index + 1 < model.hours.count ? model.hours[index + 1] : nil,
                        // Compline's next is the morrow's Lauds, as on the web.
                        morrow: index + 1 == model.hours.count ? model.hours.first : nil,
                        reportUrl: view.reportUrl,
                        date: date
                    )
                    .read(rows.count, into: reading)
                    .onGeometryChange(for: CGFloat.self) { $0.size.height } action: { epilogueHeight = $0 }
                }
                .background(alignment: .top) {
                    if view.firstClass {
                        FirstClassFrame(top: 3 + headerHeight, bottom: epilogueHeight + m.px(20))
                    }
                }
            }
            .coordinateSpace(name: Reading.space)
            .onGeometryChange(for: CGSize.self) { $0.size } action: { reading.viewport = $0; width = $0.width }
            .onChange(of: rows.count, initial: true) { reading.rows = rows.count }
            .overlay(alignment: .top) { ProgressHairline(reading: reading).ignoresSafeArea(edges: .top) }
            .revealing(scroll)
            .task(id: "\(date.iso)/\(view.hour)") { await watchPrayed() }
            .onAppear {
                // For review screenshots: `-anchor hymn` or `-anchor psalm` opens at the first one.
                guard let id = anchor(rows) else { return }
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { scroll.scrollTo(id, anchor: .top) }
            }
        }
        .environment(\.ornament, o)
    }

    /**
     * Marks the hour prayed once its end is reached after its words were read on screen at a
     * praying pace (the core's `PrayedReading`, as the web's): a quick scroll to the end, or a
     * wait there, does not.
     */
    @MainActor private func watchPrayed() async {
        var sofar = PrayedReading(cursor: 0, read: 0)
        var last = Date()
        while !Task.isCancelled {
            try? await Task.sleep(for: .seconds(1))
            let now = Date()
            let seconds = now.timeIntervalSince(last)
            last = now
            guard UIApplication.shared.applicationState == .active else { continue }
            var spans: [(from: Double, to: Double)] = []
            var total = 0.0
            for row in rows {
                var words = 0.0
                if case let .block(_, block, _, _) = row { words = Double(blockWords(block: block)) }
                spans.append((total, total + words))
                total += words
            }
            let seen = reading.onScreen()
            let shown = seen.rows.filter { $0 < spans.count }.map { spans[$0] }
            let first = shown.map(\.from).min() ?? (seen.ending ? total : 0)
            let through = shown.map(\.to).max() ?? first
            sofar = prayedReadingAdvance(reading: sofar, first: first, through: through, seconds: seconds)
            if prayedReadingDone(reading: sofar, total: total, atEnd: seen.ending) {
                model.markPrayed(date, view.hour)
                return
            }
        }
    }

    private func anchor(_ rows: [Row]) -> String? {
        let asked = UserDefaults.standard.string(forKey: "anchor")
        let at = rows.firstIndex { row in
            guard case let .block(_, block, _, _) = row else { return false }
            switch asked {
            case "hymn": return block.kind == .stanza
            case "psalm": return block.kind == .verse && block.dropCap
            default: return false
            }
        }
        // Two rows before it, so its label shows below the status bar.
        return at.map { rows[max(0, $0 - 2)].id }
    }

    /// The page's rows. Each block's space depends on the one before it, across sections.
    private var rows: [Row] {
        var out: [Row] = []
        var prev: BlockView?
        var afterClosed = false
        for (i, section) in view.sections.enumerated() {
            if section.collapsible {
                let expanded = open[i] ?? false
                out.append(.toggle(section: i, label: section.label, open: expanded))
                prev = nil
                afterClosed = !expanded
                if !expanded { continue }
            }
            for (j, block) in section.blocks.enumerated() {
                let heading = block.kind == .heading
                let gap: CGFloat
                if prev != nil {
                    gap = gapBefore(prev, block)
                } else if afterClosed {
                    gap = heading ? 23.2 : 14
                } else {
                    gap = heading ? 12 : 0
                }
                afterClosed = false
                // Between the office's parts, one small painted cross: before each heading of the
                // office's own (the web's `.elements > .section-heading`), the Hymn's and the
                // Chapter's included, but the page's first.
                let part = (heading || block.kind == .commemorationHeading) && !section.collapsible
                out.append(.block(BlockAt(section: i, block: j), block, gap: gap, cross: part && !out.isEmpty))
                prev = block
            }
        }
        return out
    }
}

/**
 * How far the reader has come through the office, as the web's `.hour-scroll-progress`: nothing
 * while the band, header and title pass, then the office as it reaches the top of the screen,
 * and all of it once the office's end reaches the bottom; the hour's ending keeps it full. Each
 * row reports where it stands as it is laid out; rows not yet laid out are taken at the average
 * height of those that have been.
 */
@Observable
final class Reading {
    /// The scroll view's coordinates, from the top of the screen below the status bar.
    static let space = "hour"
    /// How much of the office has passed, from 0 to 1; the only thing the hairline watches.
    private(set) var ratio: CGFloat = 0
    /// The office's rows in number; the ending stands at this index. A section opened or closed
    /// moves every row after it, so the heights are learned again.
    @ObservationIgnored var rows = 0 {
        didSet { if rows != oldValue { heights = [:] } }
    }
    @ObservationIgnored var viewport: CGSize = .zero {
        didSet { if viewport.width != oldValue.width { heights = [:] }; update() }
    }
    /// Each row's height as last laid out, by index, and where those on screen now stand.
    @ObservationIgnored private var heights: [Int: CGFloat] = [:]
    @ObservationIgnored private var frames: [Int: CGRect] = [:]

    func place(_ row: Int, _ frame: CGRect?) {
        if let frame {
            frames[row] = frame
            heights[row] = frame.height
        } else {
            frames[row] = nil
        }
        update()
    }

    /// The rows on screen now, and whether the office's ending is.
    func onScreen() -> (rows: [Int], ending: Bool) {
        let shown = frames.filter { $0.value.maxY > 0 && $0.value.minY < viewport.height }.map(\.key)
        return (shown.filter { $0 < rows }, shown.contains(rows))
    }

    private func update() {
        let now = read()
        if abs(now - ratio) > 0.0005 || (now != ratio && (now == 0 || now == 1)) { ratio = now }
    }

    private func read() -> CGFloat {
        let end = rows
        guard end > 0 else { return 0 }
        let bottom = viewport.height
        let ending = frames[end]
        if let ending, ending.minY <= bottom { return 1 }
        guard let top = frames.filter({ $0.value.minY <= 0 }).max(by: { $0.key < $1.key }), top.key < end else { return 0 }
        let known = (0..<end).compactMap { heights[$0] }
        let average = known.isEmpty ? 1 : known.reduce(0, +) / CGFloat(known.count)
        func height(_ i: Int) -> CGFloat { heights[i] ?? average }
        let passed = (0..<top.key).reduce(0) { $0 + height($1) } - top.value.minY
        // Once the ending is in sight its distance is known exactly.
        let left = ending.map { $0.minY - bottom } ?? ((0..<end).reduce(0) { $0 + height($1) } - bottom - passed)
        return passed + left <= 0 ? 1 : min(1, max(0, passed / (passed + left)))
    }
}

private extension View {
    /// Reports this row's place on screen to `reading`, and its leaving.
    func read(_ row: Int, into reading: Reading) -> some View {
        onGeometryChange(for: CGRect.self) { $0.frame(in: .named(Reading.space)) } action: { reading.place(row, $0) }
            .onDisappear { reading.place(row, nil) }
    }
}

/**
 * The frame of a first-class day's hour, as the web's `.rank-first-class`: a thin double line in
 * the lining down each side, from the hour's title (`top` below the page's head) to 20pt above
 * the epilogue (`bottom` from the page's foot), where the consecration cross closes the hour. On
 * a phone the lines stand 5pt from the screen's edges and the text gives them 12pt; on a wide
 * screen they stand 40pt outside the measure, as the web's from 920px.
 */
private struct FirstClassFrame: View {
    let top: CGFloat
    let bottom: CGFloat
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            let height = size.height - top - bottom
            guard height > 0 else { return }
            let side = size.width < m.px(frameClear) ? m.px(5) : (size.width - m.px(measure)) / 2 - m.px(24)
            let ink = GraphicsContext.Shading.color(p.lining.opacity(0.6))
            for x in [side, size.width - side - 3] {
                ctx.fill(Path(CGRect(x: x, y: top, width: 1, height: height)), with: ink)
                ctx.fill(Path(CGRect(x: x + 2, y: top, width: 1, height: height)), with: ink)
            }
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/// Where the prayer's text steps up to 21px, and by how much (the web's `.elements` from 920px).
private let widePrayerFrom: CGFloat = 920
private let widePrayerStep: CGFloat = 1.05

/// The width from which a first-class frame stands clear of the measure: room for 40pt outside the text each side.
private let frameClear: CGFloat = measure + 80

/**
 * The gold hairline across the top, in the season's ornament gold. The band scrolls away with
 * the page here, so the line runs along the screen's top edge, above the status bar: just below
 * it the line would strike through the text passing under the bar.
 */
private struct ProgressHairline: View {
    let reading: Reading
    @Environment(\.ornament) private var o

    var body: some View {
        GeometryReader { geo in
            Rectangle().fill(o.flat).frame(width: geo.size.width * reading.ratio)
        }
        .frame(height: 2)
        .allowsHitTesting(false)
        .accessibilityElement()
        .accessibilityLabel("Progress through the prayer text")
        .accessibilityValue("\(Int((reading.ratio * 100).rounded())) percent")
    }
}

/// The hour's title, framed; the day; and the date and prayer-form controls.
private struct HourTitle: View {
    let view: HourView
    let date: CivilDate
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide
    @State private var picking = false
    @Environment(\.reveal) private var reveal
    @State private var choosing = false

    var body: some View {
        let title = view.title.uppercased()
        let style = Scale.hourTitle
        let font = style.uiFont(m)
        // The rules take the title's width, at least 24rem (the measure, on a phone), as the web's h1.
        let titleWidth = ProseLayout.advance(title, font) + style.tracking * m.type * CGFloat(title.count) + 2 * m.px(20.8)
        let meta = [view.dateLabel, view.feast, view.seasonLabel].filter { !$0.isEmpty }
        VStack(spacing: 0) {
            VStack(spacing: 0) {
                // The hour's sign is set into the title's upper rule, the rule itself its line; the
                // day's colour reaches the lower one's lozenge.
                // One height whatever the sign (the web's 1.05rem headpiece), so the title stands
                // at the same place on every hour.
                ZStack {
                    DoubleRule(gap: 40, heavy: true)
                    TitleSign(sign: HourSign(hour: view.hour))
                }
                .frame(height: m.px(16.8))
                Text(title).type(style).foregroundStyle(p.text)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, m.px(20.8))
                    .accessibilityLabel(view.title)
                    .accessibilityAddTraits(.isHeader)
                DoubleRule(lozengeColor: dayColor(view.color))
            }
            .frame(maxWidth: max(m.px(384), titleWidth))
            // On a phone the date stands on its own line above the day's name, so a line never
            // ends on the separator (the web's `.hour-meta-part:first-child` to 700px); a wide
            // screen keeps one line.
            let metaLines = wide || meta.count < 2
                ? [meta.joined(separator: "\u{00a0}· ")]
                : [meta[0], meta.dropFirst().joined(separator: "\u{00a0}· ")]
            VStack(spacing: 0) {
                ForEach(metaLines, id: \.self) { line in
                    Text(line).type(Scale.meta).foregroundStyle(p.muted).multilineTextAlignment(.center)
                }
            }
            .padding(.top, m.px(2))
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(meta.joined(separator: ". "))
            if date != model.today {
                Button { model.open(.hour(model.today, view.hour)) } label: {
                    Text("GO TO TODAY").type(Scale.menu).foregroundStyle(p.accent)
                        .goldUnderline(true, p.goldLine)
                        .padding(.vertical, m.px(12))
                }
                .buttonStyle(Quiet())
                .padding(.top, m.px(5.6))
            }
            // Side by side, or one above the other when the reader's font size leaves no room.
            ViewThatFits(in: .horizontal) {
                HStack(spacing: m.px(12)) { disclosures }
                VStack(spacing: 0) { disclosures }
            }
            .padding(.top, m.px(4))
            if picking {
                DayPicker(shown: date, today: model.today) { d in
                    picking = false
                    model.open(.hour(d, view.hour))
                }
                .id("disclosed")
                .transition(.unfold)
            }
            if choosing {
                FormChooser(form: model.form) { f in
                    withAnimation(unfolding) { choosing = false }
                    model.chooseForm(f)
                }
                .id("disclosed")
                .transition(.unfold)
            }
            Hairline(color: p.lining.opacity(0.3)).padding(.top, m.px(7.2))
        }
        .padding(.top, m.px(17.6))
        .measured(m)
        .accessibilityElement(children: .contain)
    }

    @ViewBuilder private var disclosures: some View {
        Disclosure(label: "Change date", open: picking) {
            withAnimation(unfolding) {
                picking.toggle()
                choosing = false
            }
            if picking { reveal("disclosed") }
        }
        Disclosure(label: "Prayer form:", open: choosing, value: prayerForms.first { $0.value == model.form }?.label) {
            withAnimation(unfolding) {
                choosing.toggle()
                picking = false
            }
            if choosing { reveal("disclosed") }
        }
    }
}

/**
 * After the prayer: the consecration cross the hour ends on, the other hours, and the foot, with
 * the report line under its colophon. The wall's field shows only below the hour navigation: the
 * Apse vault by night, the Nave's powdering by day.
 */
private struct Epilogue: View {
    let previous: String?
    let next: String?
    let morrow: String?
    let reportUrl: String
    let date: CivilDate
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.openURL) private var openURL

    var body: some View {
        VStack(spacing: 0) {
            // One mark, as painted where the bishop anointed the walls: the links and foot below carry none.
            PaintedMark(.consecration, size: m.px(40), color: p.lining)
                .padding(.top, m.px(26.4))
            Continuation(
                previousLabel: "Previous hour",
                previous: previous.map(hourLabel),
                onPrevious: { if let previous { model.open(.hour(date, previous)) } },
                middle: "All hours",
                onMiddle: { model.open(.home(date)) },
                nextLabel: morrow == nil ? "Next hour" : "Tomorrow",
                next: (next ?? morrow).map(hourLabel),
                onNext: {
                    if let next { model.open(.hour(date, next)) } else if let morrow { model.open(.hour(date.adding(days: 1), morrow)) }
                }
            )
            .measured(m)
            .padding(.top, m.px(83.2))
            // The field is phased from the seam where the ending's air (3.25rem) meets the footer.
            // By night it fades in over the first rem and thins down the footer.
            let air = m.px(52)
            Footer(gap: 52 + 53.6, reserve: true) {
                Button {
                    if let url = URL(string: reportUrl) { openURL(url) }
                } label: {
                    (Text("Spotted an error on this page? ") + Text("Report a problem").foregroundColor(p.accent).underline())
                        .type(Scale.small.sized(11.84, line: 18.9))
                        .foregroundStyle(p.muted)
                        .multilineTextAlignment(.center)
                }
                .buttonStyle(Quiet())
                .accessibilityHint("Opens the report form in your browser")
                .measured(m)
                .reserve(p.bg, m)
                .padding(.top, m.px(4.8))
            }
            .background(
                WallField(seam: air) { h, dark in
                    let seam = air / h
                    return dark
                        ? [(0, 0), (m.px(16) / h, 1), (seam + 0.45 * (1 - seam), 1), (seam + 0.8 * (1 - seam), 0.5), (1, 0.25)]
                        : [(0, 1), (1, 1)]
                }
            )
        }
    }
}
