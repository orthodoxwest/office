import Observation
import SwiftUI
import UIKit

/**
 * Whether an hour's prayers before or after the Office open, as the web's `office-prayers-open`:
 * the core's default for the hour (Lauds, Prime and Vespers open, the rest closed) until the reader
 * opens or closes them, then that choice, kept on this device for the hour's section
 * ("sext/Prayers Before the Office"). A section a day leaves out (the Triduum's prayers after the
 * Office) keeps its choice for when it returns.
 */
private enum SessionPrayers {
    private static func key(_ hour: String, _ label: String) -> String { "prayers-open.\(hour)/\(label)" }

    static func opens(_ hour: String, _ label: String) -> Bool {
        UserDefaults.standard.object(forKey: key(hour, label)) as? Bool ?? prayersOpenByDefault(hour: hour)
    }

    static func choose(_ hour: String, _ label: String, open: Bool) {
        UserDefaults.standard.set(open, forKey: key(hour, label))
    }
}

/**
 * What a page shows once its content is composed. Composing reads the corpus, so it runs off
 * the main thread; the page keeps what it had until the new content is ready, then moves to it
 * as `motion` says, from the top of the new page. Content composed again for the same `place`
 * (an hour in another prayer form) is the same page: it changes where it stands, keeping the
 * reader's place and what they had opened, as the web's prayer form does.
 */
struct Loaded<Value, Content: View>: View {
    let key: String
    let place: String
    let motion: Motion
    let load: () throws -> Value
    let content: (Value) -> Content
    @State private var shown: (place: String, value: Value)?
    @State private var failure: String?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// `key` names what is composed; `place`, the page it is (by default the same).
    init(key: String, place: String? = nil, motion: Motion = .fade, load: @escaping () throws -> Value, @ViewBuilder content: @escaping (Value) -> Content) {
        self.key = key
        self.place = place ?? key
        self.motion = motion
        self.load = load
        self.content = content
    }

    var body: some View {
        ZStack {
            if let shown {
                content(shown.value).id(shown.place).transition(transition)
            } else if let failure {
                Message(text: failure)
            } else {
                Message(text: "Preparing the office…")
            }
        }
        .task(id: key) {
            let place = self.place
            let result = await compose(load)
            // Another key was asked for while this one composed: what it asked for is coming.
            guard !Task.isCancelled else { return }
            switch result {
            case let .success(v):
                failure = nil
                // The first content simply appears (the stack's push brings it), and the same
                // page composed again changes in place; another page replaces it in motion.
                if shown == nil || shown?.place == place {
                    shown = (place, v)
                } else {
                    withAnimation(.easeInOut(duration: 0.3)) { shown = (place, v) }
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
        let chant = model.chant
        let composition = "\(form) \(martyrology) \(chant)"
        // Another prayer form, or the Martyrology or Chant turned on or off, is the same page
        // composed again: the reader keeps their place and the sections they opened.
        Loaded(key: "\(date.iso) \(hour) \(composition)", place: "\(date.iso) \(hour)", motion: model.motion(to: .hour(date, hour))) { () throws -> ComposedHour in
            let view = try Office.core.get().compose(hour: hour, year: date.year, month: date.month, day: date.day, form: form, martyrology: martyrology, chant: chant)
            return ComposedHour(view: view, composition: composition)
        } content: { (composed: ComposedHour) in
            let view = composed.view
            HourScreen(view: view, date: date, composition: composed.composition)
                // Reported whenever the page comes forward (an earlier Prime gone back to
                // included), and again when its Martyrology or Chant is turned on or off.
                .onAppear { model.composed(view, page: .hour(date, hour)) }
                .onChange(of: view.martyrology) { model.composed(view, page: .hour(date, hour)) }
                .onChange(of: view.chant) { model.composed(view, page: .hour(date, hour)) }
                .onChange(of: view.ornament, initial: true) { _, season in model.composed(season: season, page: .hour(date, hour)) }
        }
        .background(PlasterWall())
        .toolbar(.hidden, for: .navigationBar)
        // Like the web's wake lock: the screen stays on while an hour is open.
        .onAppear { UIApplication.shared.isIdleTimerDisabled = true }
        .onDisappear { UIApplication.shared.isIdleTimerDisabled = false }
    }
}

/// An hour as composed, and the prayer form and Martyrology setting it was composed in.
private struct ComposedHour {
    let view: HourView
    let composition: String
}

/**
 * One row of an hour's page: a collapsible section's toggle, or a block with the space above it
 * and, before a heading between the office's parts, the small painted cross. A row's id is its
 * content's, the same in every prayer form where the row is: its section's key, then the block's
 * kind, marker and words, and which of the blocks so made in that section it is.
 */
private enum Row: Identifiable {
    case toggle(section: String, label: String, open: Bool, atTop: Bool)
    case block(BlockAt, BlockView, id: String, space: Space, cross: Bool)

    var id: String {
        switch self {
        case let .toggle(key, _, _, _): return "toggle \(key)"
        case let .block(_, _, id, _, _): return id
        }
    }
}

/// Each section's key, the same in every prayer form: its label, and which of the sections so
/// labelled it is.
private func sectionKeys(_ sections: [SectionView]) -> [String] {
    var seen: [String: Int] = [:]
    return sections.map { section in
        let n = seen[section.label, default: 0]
        seen[section.label] = n + 1
        return "\(section.label)#\(n)"
    }
}

/**
 * Where the reader was when the prayer form or the Martyrology changed, as the web's prayer form
 * notes it (leader.js): the office's rows then on screen, top-most first, each with the anchor
 * that would stand it where it stood; and every row's id, to tell whether the office changed.
 */
private struct Held {
    let marks: [(id: String, anchor: UnitPoint)]
    let ids: [String]
}

/// How far the hour has been prayed. Another prayer form is the same hour, so it carries over, as
/// the Android app's does; another hour or day begins again (`page`, the one it was prayed on).
private final class PrayedSoFar {
    var page = ""
    var reading = PrayedReading(cursor: 0, read: 0)
    var done = false
}

/// An hour, as the web's hour page: colour band, header, title, controls, the office, its epilogue.
struct HourScreen: View {
    let view: HourView
    let date: CivilDate
    /// The prayer form and Martyrology setting `view` was composed in.
    let composition: String
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    /// The collapsible sections, open or not, by their keys, so they stay as the reader left them
    /// in another prayer form.
    @State private var open: [String: Bool]
    /// How far through the office, for the hairline across the top.
    @State private var reading = Reading()
    /// The page's width, and the heights of what stands above and below a first-class day's frame.
    @State private var width: CGFloat = 0
    @State private var headerHeight: CGFloat = 0
    @State private var epilogueHeight: CGFloat = 0
    /// Where the reader was when the prayer form or the Martyrology changed, until the hour is
    /// composed again.
    @State private var held: Held?
    @State private var prayed = PrayedSoFar()

    init(view: HourView, date: CivilDate, composition: String) {
        self.view = view
        self.date = date
        self.composition = composition
        var open: [String: Bool] = [:]
        for (key, s) in zip(sectionKeys(view.sections), view.sections) where s.collapsible { open[key] = SessionPrayers.opens(view.hour, s.label) }
        _open = State(initialValue: open)
    }

    var body: some View {
        let p = self.p.seasoned(view.ornament)
        let o = Ornament.of(p, season: view.ornament)
        // From 920pt the prayer's text is a step larger, 21px for 20 (the web's `.elements` from
        // 920px, where it read a touch small on a laptop); the header, the title and the epilogue
        // keep their type. `type` carries the step, and the hour's rhythm, set in body lines, grows with it
        // (the web's `--line: 1.6em`).
        let prayer: Metrics = {
            var scaled = m
            if width >= m.px(widePrayerFrom) { scaled.type *= widePrayerStep }
            return scaled
        }()
        let columns = hymnColumns(view.sections, p, o, prayer)
        let index = model.hours.firstIndex(of: view.hour) ?? -1
        let rows = self.rows
        // A first-class day's frame takes a little of a phone's measure (the web's 12px side
        // columns under 920px); where the lines stand outside the measure, the text keeps it.
        let inset: CGFloat = view.firstClass && width < m.px(frameClear) ? m.px(12) : 0
        ScrollViewReader { scroll in
            ScrollView {
                LazyVStack(spacing: 0) {
                    Rectangle().fill(bandColor(view.color)).frame(height: 3).accessibilityHidden(true)
                    SiteHeader()
                        .onGeometryChange(for: CGFloat.self) { $0.size.height } action: { headerHeight = $0 }
                    HourTitle(view: view, date: date, pageWidth: width)
                        .padding(.horizontal, inset)
                    ForEach(Array(rows.enumerated()), id: \.element.id) { n, row in
                        Group {
                            switch row {
                            case let .toggle(key, label, expanded, atTop):
                                Button {
                                    withAnimation(unfolding) { open[key] = !expanded }
                                    SessionPrayers.choose(view.hour, label, open: !expanded)
                                } label: {
                                    HStack(spacing: 0) {
                                        Text(label).type(Scale.heading).foregroundStyle(p.titulus)
                                        // A section's caret is 0.75em of its heading (`.session-prayers summary::after`).
                                        Caret(open: expanded, color: p.titulus, scale: 0.75 * Scale.heading.size / (0.85 * 11.52))
                                    }
                                    .frame(maxWidth: .infinity, minHeight: 44)
                                }
                                .buttonStyle(Quiet())
                                .accessibilityAddTraits(.isHeader)
                                .accessibilityValue(expanded ? "Expanded" : "Collapsed")
                                // A section's label stands as a part's heading does: a section's
                                // space above it, unless it opens the hour, and a bind's below.
                                .padding(.top, atTop ? 0 : prayer.space(.section))
                                .padding(.bottom, expanded ? prayer.space(.bind) : 0)
                                .measured(m)
                                .padding(.horizontal, inset)
                                .environment(\.metrics, prayer)
                            case let .block(at, block, _, space, cross):
                                VStack(spacing: 0) {
                                    if cross {
                                        PaintedMark(.cross, size: m.px(9.92), color: p.lining).padding(.bottom, prayer.space(.bind))
                                    }
                                    BlockRow(block: block, column: columns[at])
                                }
                                .padding(.top, prayer.space(space))
                                .measured(m)
                                .padding(.horizontal, inset)
                                .environment(\.metrics, prayer)
                            }
                        }
                        .read(row.id, at: n, into: reading)
                    }
                    Epilogue(
                        previous: index > 0 ? model.hours[index - 1] : nil,
                        next: index >= 0 && index + 1 < model.hours.count ? model.hours[index + 1] : nil,
                        // Compline's next is the morrow's Lauds, as on the web.
                        morrow: index >= 0 && index + 1 == model.hours.count ? model.hours.first : nil,
                        reportUrl: view.reportUrl,
                        date: date
                    )
                    .read(Reading.ending, at: rows.count, into: reading)
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
            // Begun again on the hour composed in another form, so it reads the rows now shown.
            .task(id: "\(date.iso)/\(view.hour)/\(composition)") { await watchPrayed() }
            // The row the reader is at stays where it stood while the office changes around it, as
            // the web's prayer form keeps the first element on screen in its place.
            .onChange(of: model.form) { hold() }
            .onChange(of: model.martyrology) { hold() }
            .onChange(of: model.chant) { hold() }
            .onChange(of: composition) { restore(scroll) }
            .onAppear {
                // For review screenshots: `-anchor hymn` or `-anchor psalm` opens at the first one.
                guard let id = anchor(rows) else { return }
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { scroll.scrollTo(id, anchor: .top) }
            }
        }
        .environment(\.ornament, o)
        .environment(\.palette, p)
    }

    /**
     * Marks the hour prayed once its end is reached after its words were read on screen at a
     * praying pace (the core's `PrayedReading`, as the web's): a quick scroll to the end, or a
     * wait there, does not.
     */
    @MainActor private func watchPrayed() async {
        // Reached in place from another hour, the page keeps its state: that hour's reading is not this one's.
        let page = "\(date.iso) \(view.hour)"
        if prayed.page != page {
            prayed.page = page
            prayed.reading = PrayedReading(cursor: 0, read: 0)
            prayed.done = false
        }
        var last = Date()
        while !Task.isCancelled && !prayed.done {
            try? await Task.sleep(for: .seconds(1))
            let now = Date()
            let seconds = now.timeIntervalSince(last)
            last = now
            guard UIApplication.shared.applicationState == .active else { continue }
            var spans: [(from: Double, to: Double)] = []
            var total = 0.0
            for row in rows {
                var words = 0.0
                if case let .block(_, block, _, _, _) = row { words = Double(blockWords(block: block)) }
                spans.append((total, total + words))
                total += words
            }
            let seen = reading.onScreen()
            let shown = seen.rows.filter { $0 < spans.count }.map { spans[$0] }
            let first = shown.map(\.from).min() ?? (seen.ending ? total : 0)
            let through = shown.map(\.to).max() ?? first
            prayed.reading = prayedReadingAdvance(reading: prayed.reading, first: first, through: through, seconds: seconds)
            if prayedReadingDone(reading: prayed.reading, total: total, atEnd: seen.ending) {
                prayed.done = true
                model.markPrayed(date, view.hour)
                return
            }
        }
    }

    private func anchor(_ rows: [Row]) -> String? {
        let asked = UserDefaults.standard.string(forKey: "anchor")
        let at = rows.firstIndex { row in
            guard case let .block(_, block, _, _, _) = row else { return false }
            switch asked {
            case "hymn": return block.kind == .stanza
            case "psalm": return block.kind == .verse && block.dropCap
            default: return false
            }
        }
        // Two rows before it, so its label shows below the status bar.
        return at.map { rows[max(0, $0 - 2)].id }
    }

    /**
     * Notes the rows on screen as the prayer form or the Martyrology changes, while the hour shown
     * is still the one composed before.
     */
    private func hold() {
        let height = reading.viewport.height
        let marks = reading.shownRows().compactMap { shown -> (id: String, anchor: UnitPoint)? in
            // `scrollTo` stands the row's point at the anchor's height on the screen's: for its top
            // to stay where it is, that height is its top over the room the screen has beside it.
            let room = height - shown.frame.height
            guard room > 0, shown.frame.minY >= 0, shown.frame.minY <= room else { return nil }
            return (id: shown.id, anchor: UnitPoint(x: 0.5, y: shown.frame.minY / room))
        }
        held = Held(marks: marks, ids: rows.map(\.id))
    }

    /// Once the hour is composed again, stands the top-most row held that is still in it where it stood.
    private func restore(_ scroll: ScrollViewProxy) {
        guard let was = held else { return }
        held = nil
        let ids = rows.map(\.id)
        // The office reads the same in this form (the Martyrology away from Prime): nothing moved.
        guard ids != was.ids else { return }
        let present = Set(ids)
        guard let mark = was.marks.first(where: { present.contains($0.id) }) else { return }
        // Once the new rows are laid out.
        DispatchQueue.main.async { scroll.scrollTo(mark.id, anchor: mark.anchor) }
    }

    /// The page's rows. Each block's space depends on the one before it, across sections.
    private var rows: [Row] {
        var out: [Row] = []
        var prev: BlockView?
        var afterClosed = false
        var afterOpen = false
        for ((i, section), key) in zip(view.sections.enumerated(), sectionKeys(view.sections)) {
            if section.collapsible {
                // A section new to this form opens as the hour's sections open.
                let expanded = open[key] ?? SessionPrayers.opens(view.hour, section.label)
                out.append(.toggle(section: key, label: section.label, open: expanded, atTop: out.isEmpty))
                prev = nil
                afterClosed = !expanded
                afterOpen = expanded
                if !expanded { continue }
            }
            // How many blocks of each make this section has had, so that two alike differ.
            var made: [String: Int] = [:]
            for (j, block) in section.blocks.enumerated() {
                let heading = block.kind == .heading
                let space: Space
                if prev != nil {
                    space = block.space
                } else if afterClosed {
                    // Under a closed section's label: its own space, at most a group's.
                    space = spaceLines(space: block.space) < spaceLines(space: .group) ? block.space : .group
                } else if afterOpen {
                    // An open section's label holds the space below itself.
                    space = .none
                } else {
                    space = heading ? .bind : .none
                }
                afterClosed = false
                afterOpen = false
                // Between the office's parts, one small painted cross: before each heading of the
                // office's own (the web's `.elements > .section-heading`), the Hymn's and the
                // Chapter's included, but the page's first.
                let part = (heading || block.kind == .commemorationHeading) && !section.collapsible
                let make = "\(key)|\(block.kind)|\(block.marker)|\(block.runs.map(\.text).joined())"
                let n = made[make, default: 0]
                made[make] = n + 1
                out.append(.block(BlockAt(section: i, block: j), block, id: "\(make)#\(n)", space: space, cross: part && !out.isEmpty))
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
    /// The hour's ending's id among the rows' (theirs hold a `|`, or begin "toggle ").
    static let ending = "ending"
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
    /// Each row's height as last laid out, by index; and where those on screen now stand, by the
    /// row's id, so a row leaving never clears the place of one that has taken its index.
    @ObservationIgnored private var heights: [Int: CGFloat] = [:]
    @ObservationIgnored private var frames: [String: (row: Int, frame: CGRect)] = [:]

    func place(_ id: String, at row: Int, _ frame: CGRect?) {
        if let frame {
            frames[id] = (row: row, frame: frame)
            heights[row] = frame.height
        } else {
            frames[id] = nil
        }
        update()
    }

    /// The office's rows on screen now, by id, and where each stands, top-most first.
    func shownRows() -> [(id: String, frame: CGRect)] {
        frames.filter { $0.value.row < rows && $0.value.frame.maxY > 0 && $0.value.frame.minY < viewport.height }
            .sorted { $0.value.frame.minY < $1.value.frame.minY }
            .map { (id: $0.key, frame: $0.value.frame) }
    }

    /// The rows on screen now, and whether the office's ending is.
    func onScreen() -> (rows: [Int], ending: Bool) {
        let shown = frames.values.filter { $0.frame.maxY > 0 && $0.frame.minY < viewport.height }.map { $0.row }
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
        let ending = frames[Reading.ending]?.frame
        if let ending, ending.minY <= bottom { return 1 }
        guard let top = frames.values.filter({ $0.frame.minY <= 0 }).max(by: { $0.row < $1.row }), top.row < end else { return 0 }
        let known = (0..<end).compactMap { heights[$0] }
        let average = known.isEmpty ? 1 : known.reduce(0, +) / CGFloat(known.count)
        func height(_ i: Int) -> CGFloat { heights[i] ?? average }
        let passed = (0..<top.row).reduce(0) { $0 + height($1) } - top.frame.minY
        // Once the ending is in sight its distance is known exactly.
        let left = ending.map { $0.minY - bottom } ?? ((0..<end).reduce(0) { $0 + height($1) } - bottom - passed)
        return passed + left <= 0 ? 1 : min(1, max(0, passed / (passed + left)))
    }
}

private extension View {
    /// Reports the place on screen of this row, `id`, the `row`th, to `reading`, and its leaving.
    func read(_ id: String, at row: Int, into reading: Reading) -> some View {
        onGeometryChange(for: CGRect.self) { $0.frame(in: .named(Reading.space)) } action: { reading.place(id, at: row, $0) }
            .onDisappear { reading.place(id, at: row, nil) }
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
 * it the line would strike through the text passing under the bar. On a screen with rounded
 * corners it runs between them, so its first and last stretches are not lost behind the curve
 * (as the Android app's, which reads the corners from the system).
 */
private struct ProgressHairline: View {
    let reading: Reading
    @Environment(\.ornament) private var o

    var body: some View {
        GeometryReader { geo in
            let clear = cornerReach(at: 1)
            let span = max(0, geo.size.width - 2 * clear)
            Rectangle().fill(o.flat).frame(width: span * reading.ratio).offset(x: span > 0 ? clear : 0)
        }
        .frame(height: 2)
        .allowsHitTesting(false)
        .accessibilityElement()
        .accessibilityLabel("Progress through the prayer text")
        .accessibilityValue("\(Int((reading.ratio * 100).rounded())) percent")
    }
}

/**
 * How far in from each side a screen's rounded top corner reaches, `y` points below its top edge.
 * iOS does not give the corners' radius, but on every iPhone and iPad with rounded corners it is
 * close to the deepest of the window's safe-area insets (the status bar's in portrait, the
 * sensor housing's in landscape), from 39pt to 62pt on a phone; a screen whose insets are no
 * deeper than the old 20pt status bar has square corners.
 */
@MainActor
private func cornerReach(at y: CGFloat) -> CGFloat {
    let window = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first?.windows.first
    guard let insets = window?.safeAreaInsets else { return 0 }
    let r = max(insets.top, insets.left, insets.right)
    guard r > 20, y < r else { return 0 }
    return r - (r * r - (r - y) * (r - y)).squareRoot()
}

/// The hour's title, framed; the day; and the date and prayer-form controls.
private struct HourTitle: View {
    let view: HourView
    let date: CivilDate
    /// The page's width, for the web's 540px step in the date picker's navigation.
    let pageWidth: CGFloat
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
                DoubleRule(lozengeColor: titleColor(view.color, p))
            }
            .frame(maxWidth: max(m.px(384), titleWidth))
            // On a phone the date stands on its own line above the day's name, so a line never
            // ends on the separator (the web's `.hour-meta-part:first-child` to 700px); a wide
            // screen keeps one line. Each part is kept whole (`.hour-meta-part`), its separator
            // bound to it by the no-break space, so a line breaks between parts, not inside one.
            let metaLines: [[String]] = wide || meta.count < 2 ? [meta] : [[meta[0]], Array(meta.dropFirst())]
            let space = ProseLayout.advance(" ", Scale.meta.uiFont(m))
            VStack(spacing: 0) {
                ForEach(Array(metaLines.enumerated()), id: \.offset) { _, parts in
                    MetaLine(space: space) {
                        ForEach(Array(parts.enumerated()), id: \.offset) { i, part in
                            Text(i < parts.count - 1 ? part + "\u{00a0}·" : part)
                                .type(Scale.meta).foregroundStyle(p.muted).multilineTextAlignment(.center)
                        }
                    }
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
                DayPicker(shown: date, today: model.today, centred: pageWidth > 540) { d in
                    picking = false
                    model.open(.hour(d, view.hour))
                }
                .id("disclosed")
                .transition(.unfold)
            }
            if choosing {
                FormChooser(form: model.form) { f in
                    let changed = f != model.form
                    // The prayers change once the chooser has folded away, so the reader's place is
                    // held where the rows then stand, not where they stood below the open chooser.
                    withAnimation(unfolding) { choosing = false } completion: { model.chooseForm(f) }
                    // VoiceOver hears that the prayers changed, as the web's status line says it;
                    // a moment later, so the chooser folding away does not cut it short.
                    if changed, let phrase = prayerForms.first(where: { $0.value == f })?.phrase {
                        DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) {
                            UIAccessibility.post(notification: .announcement, argument: "Prayers updated. \(phrase).")
                        }
                    }
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
 * A line of the hour's date, day and season (`.hour-meta-part`), each kept whole: as many to a line
 * as fit, centred, so a line breaks between parts and never inside one, unless a part alone is
 * wider than the line, which then wraps within itself. `space` follows each part's separator.
 */
private struct MetaLine: Layout {
    let space: CGFloat

    private typealias Row = (items: [(index: Int, size: CGSize)], width: CGFloat, height: CGFloat)

    private func arrange(_ width: CGFloat, _ subviews: Subviews) -> [Row] {
        var rows: [Row] = []
        var row: Row = (items: [], width: 0, height: 0)
        for (i, view) in subviews.enumerated() {
            var size = view.sizeThatFits(.unspecified)
            if size.width > width { size = view.sizeThatFits(ProposedViewSize(width: width, height: nil)) }
            let next = row.items.isEmpty ? size.width : row.width + space + size.width
            if next > width && !row.items.isEmpty {
                rows.append(row)
                row = (items: [(index: i, size: size)], width: size.width, height: size.height)
            } else {
                row = (items: row.items + [(index: i, size: size)], width: next, height: max(row.height, size.height))
            }
        }
        if !row.items.isEmpty { rows.append(row) }
        return rows
    }

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let rows = arrange(proposal.width ?? .infinity, subviews)
        return CGSize(width: rows.map { $0.width }.max() ?? 0, height: rows.reduce(0) { $0 + $1.height })
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        var y = bounds.minY
        // Arranged at the width it was measured at, so the lines break where they were measured.
        for row in arrange(proposal.width ?? bounds.width, subviews) {
            var x = bounds.midX - row.width / 2
            for item in row.items {
                subviews[item.index].place(at: CGPoint(x: x, y: y), proposal: ProposedViewSize(item.size))
                x += item.size.width + space
            }
            y += row.height
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
    @Environment(\.wide) private var wide
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
            // A full thumb's height to tap the report line on a phone, the line centred in it: the
            // web's link grows its line box so, and the foot with it.
            let reach = wide ? 0 : max(0, (44 - 18.9 * m.type) / 2)
            Footer(gap: 52 + 53.6, reserve: true) {
                Button {
                    if let url = URL(string: reportUrl) { openURL(url) }
                } label: {
                    (Text("Spotted an error on this page? ") + Text("Report a problem").foregroundColor(p.accent).underline())
                        .type(Scale.small.sized(11.84, line: 18.9))
                        .foregroundStyle(p.muted)
                        .multilineTextAlignment(.center)
                        .padding(.vertical, reach)
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
