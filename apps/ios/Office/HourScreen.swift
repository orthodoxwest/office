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

/// An hour of a day, in the reader's prayer form.
struct HourPage: View {
    let date: CivilDate
    let hour: String
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let form = model.form
        Loaded(key: "\(date.iso) \(hour) \(form)", motion: model.motion(to: .hour(date, hour))) {
            try Office.core.get().compose(hour: hour, year: date.year, month: date.month, day: date.day, form: form)
        } content: { view in
            HourScreen(view: view, date: date)
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

    init(view: HourView, date: CivilDate) {
        self.view = view
        self.date = date
        var open: [Int: Bool] = [:]
        for (i, s) in view.sections.enumerated() where s.collapsible { open[i] = openPreparation.contains(view.hour) }
        _open = State(initialValue: open)
    }

    var body: some View {
        let o = Ornament.of(p, season: view.ornament)
        let columns = hymnColumns(view.sections, p, o, m)
        let index = model.hours.firstIndex(of: view.hour) ?? 0
        let rows = self.rows
        ScrollViewReader { scroll in
            ScrollView {
                LazyVStack(spacing: 0) {
                    Rectangle().fill(dayColor(view.color)).frame(height: 3).accessibilityHidden(true)
                    SiteHeader()
                    HourTitle(view: view, date: date)
                    ForEach(rows) { row in
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
                        case let .block(at, block, gap, cross):
                            VStack(spacing: 0) {
                                if cross {
                                    PaintedMark(.cross, size: m.px(9.92), color: p.lining).padding(.bottom, m.px(11.2))
                                }
                                BlockRow(block: block, column: columns[at])
                            }
                            .padding(.top, m.px(gap))
                            .measured(m)
                        }
                    }
                    Epilogue(
                        previous: index > 0 ? model.hours[index - 1] : nil,
                        next: index + 1 < model.hours.count ? model.hours[index + 1] : nil,
                        reportUrl: view.reportUrl,
                        date: date
                    )
                }
            }
            .onAppear {
                // For review screenshots: `-anchor hymn` or `-anchor psalm` opens at the first one.
                guard let id = anchor(rows) else { return }
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { scroll.scrollTo(id, anchor: .top) }
            }
        }
        .environment(\.ornament, o)
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
                // office's own (the web's `.elements > .section-heading`) but the page's first. The
                // Hymn's and the Chapter's headings open their element, its blocks following them.
                let next = section.blocks.indices.contains(j + 1) ? section.blocks[j + 1] : nil
                let part = (heading || block.kind == .commemorationHeading) && !section.collapsible
                    && (!block.startsElement || next.map { $0.startsElement } ?? true)
                out.append(.block(BlockAt(section: i, block: j), block, gap: gap, cross: part && !out.isEmpty))
                prev = block
            }
        }
        return out
    }
}

/// The hour's title, framed; the day; and the date and prayer-form controls.
private struct HourTitle: View {
    let view: HourView
    let date: CivilDate
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @State private var picking = false
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
                ZStack {
                    DoubleRule(gap: 40, heavy: true)
                    TitleSign(sign: HourSign(hour: view.hour))
                }
                Text(title).type(style).foregroundStyle(p.text)
                    .multilineTextAlignment(.center)
                    .padding(.horizontal, m.px(20.8))
                    .accessibilityLabel(view.title)
                    .accessibilityAddTraits(.isHeader)
                DoubleRule(lozengeColor: dayColor(view.color))
            }
            .frame(maxWidth: max(m.px(384), titleWidth))
            Text(meta.joined(separator: "\u{00a0}· ")).type(Scale.meta).foregroundStyle(p.muted)
                .multilineTextAlignment(.center)
                .padding(.top, m.px(2))
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
                .transition(.unfold)
            }
            if choosing {
                FormChooser(form: model.form) { f in
                    withAnimation(unfolding) { choosing = false }
                    model.chooseForm(f)
                }
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
        }
        Disclosure(label: "Prayer form:", open: choosing, value: prayerForms.first { $0.value == model.form }?.label) {
            withAnimation(unfolding) {
                choosing.toggle()
                picking = false
            }
        }
    }
}

/**
 * After the prayer: the consecration cross the hour ends on, the other hours, the report link,
 * and the foot. In Apse the vault fades in here.
 */
private struct Epilogue: View {
    let previous: String?
    let next: String?
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
                nextLabel: "Next hour",
                next: next.map(hourLabel),
                onNext: { if let next { model.open(.hour(date, next)) } }
            )
            .measured(m)
            .padding(.top, m.px(83.2))
            Button {
                if let url = URL(string: reportUrl) { openURL(url) }
            } label: {
                (Text("Spotted an error on this page? ") + Text("Report a problem").foregroundColor(p.accent).underline())
                    .type(Scale.small)
                    .foregroundStyle(p.muted)
                    .multilineTextAlignment(.center)
            }
            .buttonStyle(Quiet())
            .accessibilityHint("Opens the report form in your browser")
            .measured(m)
            .padding(.top, m.px(40))
            Footer(diamond: false)
        }
        .background(VaultField(fade: [(0, 0), (0.35, 0), (0.7, 0.8), (1, 1)]))
    }
}
