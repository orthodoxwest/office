import SwiftUI

/// The search and feast page's fixed words, as every front sets them.
let feastWordsShared = feastWords()

/// The search for a feast or saint, as the web's /calendar/find.
struct FindPage: View {
    let query: String
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let year = Int32(model.today.year)
        // One place however the words change, so the field keeps its focus while the results follow.
        Loaded(key: "\(query)|\(year)", place: "find") {
            try Office.core.get().findFeasts(query: query, year: year)
        } content: { view in
            FindScreen(view: view, query: query)
        }
        .background(PlasterWall())
        .toolbar(.hidden, for: .navigationBar)
    }
}

/// A feast's page, as the web's /feast/{id}.
struct FeastPage: View {
    let id: String
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let today = model.today
        Loaded(key: "\(id)|\(today.iso)", place: id, motion: model.motion(to: .feast(id))) { () throws -> FeastPageView in
            guard let view = try Office.core.get().feastPage(id: id, today: today) else { throw NoFeast() }
            return view
        } content: { view in
            FeastScreen(view: view)
        }
        .background(PlasterWall())
        .toolbar(.hidden, for: .navigationBar)
    }
}

private struct NoFeast: Error, CustomStringConvertible {
    var description: String { "No feast by that name." }
}

extension View {
    /// The feast pages' column: the prayer measure on a phone, the web's 44rem on a wide screen.
    func feastColumn(_ m: Metrics, wide: Bool) -> some View {
        Group {
            if wide {
                frame(maxWidth: 704).padding(.horizontal, 24).frame(maxWidth: .infinity)
            } else {
                measured(m)
            }
        }
    }
}

/**
 * The search for a feast or saint: the field, then each feast found with how it is kept this year
 * and next. The field searches as the reader types.
 */
struct FindScreen: View {
    let view: FindView
    let query: String
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide

    var body: some View {
        let w = feastWordsShared
        ScrollView {
            LazyVStack(spacing: 0) {
                SiteHeader()
                FeastHeader(title: w.findTitle, subtitle: view.subtitle, ink: p.text).feastColumn(m, wide: wide).padding(.top, m.px(17.6))
                SearchField(query: query) { model.search($0) }.feastColumn(m, wide: wide).padding(.top, m.px(16))
                if !view.count.isEmpty {
                    Text(view.count).type(Scale.small.sized(14.4, line: 21)).foregroundStyle(p.muted)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .feastColumn(m, wide: wide).padding(.top, m.px(14))
                }
                ForEach(view.results, id: \.id) { r in
                    result(r).feastColumn(m, wide: wide).padding(.top, m.px(16))
                }
                Footer()
            }
        }
        .scrollDismissesKeyboard(.interactively)
        // Said as it changes, as the web's `role="status"`; on the page, which stays while the
        // count comes and goes.
        .onChange(of: view.count) { _, count in
            if !count.isEmpty { AccessibilityNotification.Announcement(count).post() }
        }
    }

    private func result(_ r: FeastResultView) -> some View {
        let ink = rankTone(r.rankTone, p)
        return VStack(alignment: .leading, spacing: 0) {
            Button { model.open(.feast(r.id)) } label: {
                VStack(alignment: .leading, spacing: 0) {
                    HStack(alignment: .top, spacing: 0) {
                        Text(r.name).type(Scale.body.sized(21.6, line: 27)).foregroundStyle(ink)
                            .multilineTextAlignment(.leading)
                            .frame(maxWidth: .infinity, alignment: .leading)
                        if !r.rank.isEmpty {
                            Text(r.rank).type(Scale.small.sized(15.2, line: 20)).foregroundStyle(ink)
                                .padding(.leading, m.px(12)).padding(.top, m.px(4))
                        }
                    }
                    (Text(r.meta + " · ") + Text(feastWordsShared.everyYear).foregroundColor(p.accent))
                        .type(Scale.small.sized(14.7, line: 21)).foregroundStyle(p.muted)
                        .multilineTextAlignment(.leading)
                        .padding(.top, m.px(3)).padding(.bottom, m.px(6))
                }
            }
            .buttonStyle(Quiet())
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("\(r.name), \(r.rankFull). \(r.meta)")
            .accessibilityAddTraits(.isHeader)
            .accessibilityHint("Opens the feast's page")
            ForEach(Array(r.years.enumerated()), id: \.offset) { i, y in YearRow(y: y, first: i == 0) }
            Hairline(color: p.border).padding(.top, m.px(12))
        }
    }
}

/// The width from which the feast page sets its standing and years as a rail beside its texts.
private let feastRailFrom: CGFloat = 960

/**
 * A feast's page: its rank, then its standing (its date and the next day it is kept, leading to
 * that day), its texts with the collect first, and how it is kept in the next eight years. On a wide
 * screen the standing and the years make a rail beside the texts (style.css `.feast-body`).
 */
struct FeastScreen: View {
    let view: FeastPageView
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide

    private var note: TextStyle { Scale.small.sized(14.7, line: 21) }

    var body: some View {
        GeometryReader { geo in
            let rail = geo.size.width >= feastRailFrom && !view.texts.isEmpty
            ScrollView {
                LazyVStack(spacing: 0) {
                    SiteHeader()
                    Group {
                        back
                        FeastHeader(title: view.name, subtitle: view.subtitle, ink: p.rubric)
                    }
                    .feastPageColumn(m, wide: wide, rail: rail)
                    if rail {
                        HStack(alignment: .top, spacing: 48) {
                            texts.frame(maxWidth: .infinity, alignment: .leading)
                            VStack(alignment: .leading, spacing: 0) {
                                StandingBox(view: view)
                                years.padding(.top, m.px(22))
                            }
                            .frame(width: 320)
                        }
                        .feastPageColumn(m, wide: wide, rail: true)
                        .padding(.top, m.px(22))
                    } else {
                        StandingBox(view: view).feastColumn(m, wide: wide).padding(.top, m.px(22))
                        if !view.texts.isEmpty {
                            texts.feastColumn(m, wide: wide).padding(.top, m.px(22))
                        }
                        years.feastColumn(m, wide: wide).padding(.top, m.px(28))
                    }
                    Footer()
                }
            }
        }
    }

    private var back: some View {
        Button { model.findAgain() } label: {
            Text("‹ \(feastWordsShared.findTitle)").type(Scale.small.sized(15.2, line: 20)).foregroundStyle(p.accent)
                .padding(.vertical, m.px(12))
                .frame(minHeight: 44)
        }
        .buttonStyle(Quiet())
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// The next eight years, a compact reference: a row says its status only where it differs from the usual.
    private var years: some View {
        VStack(alignment: .leading, spacing: 0) {
            SectionHeading(text: view.yearsHeading, small: true)
            ForEach(Array(view.years.enumerated()), id: \.offset) { i, y in YearRow(y: y, first: i == 0, compact: true) }
        }
    }

    private var texts: some View {
        let w = feastWordsShared
        return VStack(alignment: .leading, spacing: 0) {
            SectionHeading(text: w.textsHeading)
            Text(view.textsSource).type(TextStyle(size: 14.7, line: 21, italic: true)).foregroundStyle(p.muted).padding(.top, m.px(6))
            ForEach(Array(view.texts.enumerated()), id: \.offset) { _, t in
                TextLabel(label: t.label, common: t.common ? w.commonMark : "")
                    .padding(.top, m.px(18)).padding(.bottom, m.px(2))
                // Each text keeps the hour's rhythm between its blocks, a source's blank lines included.
                ForEach(Array(t.blocks.enumerated()), id: \.offset) { i, b in
                    BlockRow(block: b).padding(.top, i == 0 ? 0 : m.space(b.space))
                }
            }
            if let whole = view.wholeOffice {
                Button { model.open(.home(whole.date)) } label: {
                    (Text(w.wholeOffice + " ") + Text(whole.label).foregroundColor(p.accent))
                        .type(note).foregroundStyle(p.muted).multilineTextAlignment(.leading)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .buttonStyle(Quiet())
                .padding(.top, m.px(18))
                .accessibilityHint("Opens the day")
            }
        }
    }
}

extension View {
    /// The feast page's column, or on the rail the wider one its texts and rail share (the web's 62rem).
    func feastPageColumn(_ m: Metrics, wide: Bool, rail: Bool) -> some View {
        Group {
            if rail {
                frame(maxWidth: 992).padding(.horizontal, 24).frame(maxWidth: .infinity)
            } else {
                feastColumn(m, wide: wide)
            }
        }
    }
}

/**
 * The feast's standing, framed as the invitation is (the lining ruled twice): for a monastic
 * observance its heading, the feast everyone keeps and the note; then its date, and the next day it
 * is kept, which a tap opens (`.feast-standing`).
 */
private struct StandingBox: View {
    let view: FeastPageView
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let w = feastWordsShared
        let monastic = !view.monasticNote.isEmpty
        VStack(alignment: .leading, spacing: m.px(8)) {
            if monastic {
                Text(w.monasticHeading).type(TextStyle(size: 16, line: 22, tracking: 1, smallCaps: true)).foregroundStyle(p.titulus)
                if let all = view.keptByAll {
                    Button { model.open(.feast(all.id)) } label: {
                        (Text(w.everyoneKeeps + " ") + Text(all.name).foregroundColor(p.accent) + Text(" " + view.keptByAllNote))
                            .type(Scale.body.sized(16.8, line: 24)).foregroundStyle(p.text).multilineTextAlignment(.leading)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                    .buttonStyle(Quiet())
                    .accessibilityHint("Opens the feast's page")
                }
                Text(view.monasticNote).type(Scale.small.sized(14.7, line: 20.6)).foregroundStyle(p.muted)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            Group {
                if !view.dateLead.isEmpty {
                    Text(view.dateLead + " ") + Text(view.date).foregroundColor(p.text)
                } else {
                    Text(view.date)
                }
            }
            .type(Scale.body.sized(monastic ? 16 : 18.4, line: 26)).foregroundStyle(p.muted)
            .frame(maxWidth: .infinity, alignment: .leading)
            if let next = view.next {
                Button { model.open(.home(next.date)) } label: { nextRow(next) }
                    .buttonStyle(Quiet())
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("\(next.label): \(next.dateLabel)")
                    .accessibilityHint("Opens the day")
            }
        }
        .padding(.leading, m.px(17.6)).padding(.trailing, m.px(16))
        .padding(.top, m.px(13.6)).padding(.bottom, m.px(15))
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(p.surface)
        .overlay { Rectangle().strokeBorder(p.lining, lineWidth: 1) }
        .padding(3)
        .overlay { Rectangle().strokeBorder(p.lining, lineWidth: 1).padding(-0.5) }
    }

    /// The next keeping under a dotted rule: its label, then its date with the way to the day, which
    /// keeps to the date's line while it fits and takes the right of its own when not.
    private func nextRow(_ next: NextKeepingView) -> some View {
        let date = Text(next.dateLabel).type(Scale.body.sized(16.8, line: 25)).foregroundStyle(p.text).fixedSize()
        let go = Text("\(feastWordsShared.daysHours) ›").type(Scale.body.sized(15.7, line: 25)).foregroundStyle(p.rubric).fixedSize()
        return VStack(alignment: .leading, spacing: 0) {
            DottedRule(color: p.border)
            Text(next.label).type(TextStyle(size: 13.6, line: 18, tracking: 1.1, smallCaps: true)).foregroundStyle(p.muted)
                .padding(.top, m.px(8.8))
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .lastTextBaseline, spacing: m.px(12.8)) {
                    date
                    Spacer(minLength: 0)
                    go
                }
                VStack(alignment: .trailing, spacing: 0) {
                    date.frame(maxWidth: .infinity, alignment: .leading)
                    go
                }
            }
            .padding(.top, m.px(2))
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .contentShape(Rectangle())
    }
}

/// A text's label in the rubric's small capitals, and "Common" after it when the text is the Common's.
private struct TextLabel: View {
    let label: String
    let common: String
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let caps = TextStyle(size: 16, line: 22, tracking: 0.8, smallCaps: true)
        let name = Text(label).font(Font(caps.uiFont(m))).foregroundColor(p.rubric).tracking(0.8 * m.type)
        let mark = Text(common.isEmpty ? "" : "  " + common).font(Font(TextStyle(size: 13.6, line: 22, italic: true).uiFont(m))).foregroundColor(p.muted)
        return (name + mark)
            .lineSpacing(max(0, caps.line * m.type - caps.uiFont(m).lineHeight))
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityAddTraits(.isHeader)
    }
}

/// A page's headpiece, its title in `ink`, and its subtitle, over a hairline (the ordo's header).
private struct FeastHeader: View {
    let title: String
    let subtitle: String
    let ink: Color
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(spacing: 0) {
            Headpiece()
            Text(title).type(TextStyle(size: 28, line: 34, lining: true)).foregroundStyle(ink)
                .multilineTextAlignment(.center)
                .padding(.top, m.px(2))
                .accessibilityAddTraits(.isHeader)
            Text(subtitle).type(Scale.small).foregroundStyle(p.muted).multilineTextAlignment(.center).padding(.top, m.px(4.8))
        }
        .frame(maxWidth: .infinity)
        .padding(.top, m.px(13.4))
        .padding(.bottom, m.px(16))
        .overlay(alignment: .bottom) { Hairline(color: p.border) }
    }
}

/// A section's heading in the tituli's small capitals over the lining (`.feast-section h2`).
private struct SectionHeading: View {
    let text: String
    var small = false
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Text(text).type(small ? TextStyle(size: 16.8, line: 22, tracking: 1.3, smallCaps: true) : TextStyle(size: 20, line: 26, tracking: 1.6, smallCaps: true)).foregroundStyle(p.titulus)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.bottom, m.px(5))
            .overlay(alignment: .bottom) { Hairline(color: p.lining) }
            .accessibilityAddTraits(.isHeader)
    }
}

/// A year's keeping: its year, date and status on the day's colour rail, leading to that day. A year
/// kept as the feast usually is leaves its status unsaid; `compact` is the feast page's tighter row.
private struct YearRow: View {
    let y: FeastYearView
    let first: Bool
    var compact = false
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    private var spoken: String {
        ["\(y.year)", y.dateLabel, y.label, y.note].filter { !$0.isEmpty }.joined(separator: ", ")
    }

    var body: some View {
        VStack(spacing: 0) {
            if !first { DottedRule(color: p.border) }
            if let date = y.date {
                Button { model.open(.home(date)) } label: { row(open: true) }
                    .buttonStyle(Quiet())
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(spoken)
                    .accessibilityHint("Opens the day")
            } else {
                row(open: false)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(spoken)
            }
        }
    }

    private func row(open: Bool) -> some View {
        let quiet = y.kind == "monastic" || y.kind == "not-kept"
        return HStack(alignment: .top, spacing: 0) {
            Rectangle().fill(titleColor(y.color, p)).frame(width: 3)
            Text(String(y.year)).type(Scale.body.sized(16, line: 24)).foregroundStyle(p.muted)
                .frame(width: m.px(44), alignment: .leading)
                .padding(.leading, m.px(10))
            VStack(alignment: .leading, spacing: 0) {
                Text(y.dateLabel).type(Scale.body.sized(17.6, line: 24)).foregroundStyle(p.text)
                if !y.usual { Text(y.label).type(TextStyle(size: 15.2, line: 21, italic: quiet)).foregroundStyle(keepingInk(y.kind, p)) }
                if !y.note.isEmpty { Text(y.note).type(Scale.small.sized(13.8, line: 18)).foregroundStyle(p.muted) }
            }
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            if open {
                Text("›").type(Scale.body.sized(16)).foregroundStyle(p.goldLine)
                    .padding(.leading, m.px(8))
                    .frame(maxHeight: .infinity)
            }
        }
        .fixedSize(horizontal: false, vertical: true)
        .padding(.vertical, m.px(compact ? 5 : 8))
        .frame(minHeight: 44)
    }
}

/// A dotted hairline between a feast's years.
private struct DottedRule: View {
    let color: Color

    var body: some View {
        Canvas { ctx, size in
            var x: CGFloat = 0
            while x < size.width {
                ctx.fill(Path(CGRect(x: x, y: 0, width: 1, height: 1)), with: .color(color))
                x += 3
            }
        }
        .frame(height: 1)
        .frame(maxWidth: .infinity)
        .accessibilityHidden(true)
    }
}

/// A year's status in its ink: commemorated red, transferred slate blue, the rest the text's or muted.
private func keepingInk(_ kind: String, _ p: Palette) -> Color {
    switch kind {
    case "commemorated": return p.rubric
    case "transferred": return p.kalendarBlue
    case "monastic": return p.titulus
    case "not-kept": return p.muted
    default: return p.text
    }
}

/// A feast's name in its rank's ink, as the ordo marks ranks.
private func rankTone(_ tone: String, _ p: Palette) -> Color {
    switch tone {
    case "greater": return p.rubric
    case "double": return p.kalendarBlue
    default: return p.text
    }
}

/**
 * The search field, as the web's: a magnifier, the words, and a gold rule beneath. It searches as
 * the reader types, a moment after the last key, and puts the cursor in itself when empty.
 */
private struct SearchField: View {
    let query: String
    let onQuery: (String) -> Void
    @State private var text: String
    @FocusState private var focused: Bool
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    init(query: String, onQuery: @escaping (String) -> Void) {
        self.query = query
        self.onQuery = onQuery
        _text = State(initialValue: query)
    }

    var body: some View {
        HStack(spacing: m.px(10)) {
            Magnifier(ink: p.goldLine)
            TextField("", text: $text, prompt: Text(feastWordsShared.findPrompt).italic().foregroundColor(p.muted))
                .type(Scale.body.sized(17, line: 24))
                .foregroundStyle(p.text)
                .focused($focused)
                .textInputAutocapitalization(.words)
                .autocorrectionDisabled()
                .submitLabel(.search)
                .onSubmit {
                    onQuery(text)
                    focused = false
                }
                .accessibilityLabel(feastWordsShared.findPrompt)
                .frame(maxWidth: .infinity)
            if !text.isEmpty {
                Button {
                    text = ""
                    onQuery("")
                } label: {
                    Text("Clear").type(Scale.small.sized(15.2)).foregroundStyle(p.accent)
                        .padding(.leading, m.px(12)).frame(minHeight: 44)
                }
                .buttonStyle(Quiet())
                .accessibilityLabel("Clear the search")
            }
        }
        .frame(minHeight: 48)
        .overlay(alignment: .bottom) { Hairline(color: p.goldLine) }
        .onChange(of: text) { _, now in
            if now.count > 100 { text = String(now.prefix(100)) }
        }
        // Searches a moment after the last key.
        .task(id: text) {
            guard text != query else { return }
            try? await Task.sleep(for: .milliseconds(250))
            guard !Task.isCancelled else { return }
            onQuery(text)
        }
        .onAppear { if query.isEmpty { focused = true } }
    }
}

/**
 * The ordo's way into the search: the field as the web draws it on the ordo, which opens the
 * search page with the cursor in its field.
 */
struct SearchEntry: View {
    let action: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Button(action: action) {
            HStack(spacing: m.px(10)) {
                Magnifier(ink: p.goldLine)
                Text(feastWordsShared.findPrompt).type(TextStyle(size: 17, line: 24, italic: true)).foregroundStyle(p.muted)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .frame(minHeight: 48)
            .overlay(alignment: .bottom) { Hairline(color: p.goldLine) }
        }
        .buttonStyle(Quiet())
        .accessibilityLabel(feastWordsShared.findPrompt)
        .accessibilityAddTraits(.isSearchField)
    }
}

/// The search's magnifier (macros.html `feast_search`), viewBox 20×20.
private struct Magnifier: View {
    let ink: Color
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            let u = size.width / 20
            let style = StrokeStyle(lineWidth: 1.6 * u, lineCap: .round)
            ctx.stroke(Path(ellipseIn: CGRect(x: 3 * u, y: 3 * u, width: 11 * u, height: 11 * u)), with: .color(ink), style: style)
            var handle = Path()
            handle.move(to: CGPoint(x: 12.6 * u, y: 12.6 * u))
            handle.addLine(to: CGPoint(x: 17 * u, y: 17 * u))
            ctx.stroke(handle, with: .color(ink), style: style)
        }
        .frame(width: m.px(17), height: m.px(17))
        .accessibilityHidden(true)
    }
}
