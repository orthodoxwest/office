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

/// The width from which the feast page sets its standing and years as a rail beside its texts
/// (style.css `min-width: 760px`).
private let feastRailFrom: CGFloat = 760

/// The width from which the rail is wider and the texts keep their own measure (`min-width: 960px`).
private let feastDeskFrom: CGFloat = 960

/// `--frontal`: a day's colour as the page's finer rules and points take it: the ornament's gold
/// on a white day, the lining thinned on a black one.
private func frontalInk(_ color: String?, _ p: Palette, _ o: Ornament) -> Color {
    switch color {
    case nil: return p.lining
    case "white": return o.flat
    case "black": return p.lining.opacity(0.8)
    case let c?: return titleColor(c, p)
    }
}

/// One stroke of home's pen along a view's top, or its foot: the feast page's rules (app.js's
/// feast-page block, the same seeds).
private struct Stroke: View {
    let weight: CGFloat
    let seed: UInt32
    let color: Color
    var foot = false

    var body: some View {
        Canvas { ctx, size in
            let y = foot ? size.height - 0.5 : 0.5
            ctx.fill(ruledLine(0, y, size.width, y, weight, seed: seed), with: .color(color))
        }
        .frame(height: 2)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * A feast's page, a leaf of home's painted wall (style.css "The feast page"): its head, then its
 * standing (the next day it is kept, leading to that day), its texts with the collect first, and
 * how it is kept in the next eight years, ruled as home's hours. From 760pt the standing and the
 * years make a rail beside the texts, and on a first-class feast an upright course of the lining
 * and its colour runs between them (`.feast-body`).
 */
struct FeastScreen: View {
    let view: FeastPageView
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide

    private var note: TextStyle { Scale.small.sized(14.7, line: 21) }

    /// Whether the standing has anything to say: a monastic observance's note, or a next keeping.
    private var stands: Bool { !view.monasticNote.isEmpty || view.next != nil }

    var body: some View {
        GeometryReader { geo in
            let width = geo.size.width
            let railed = width >= feastRailFrom
            ScrollView {
                LazyVStack(spacing: 0) {
                    SiteHeader()
                    Group {
                        back
                        FeastHead(view: view, width: width)
                    }
                    .feastPageColumn(m, wide: wide, rail: railed)
                    if railed && !view.texts.isEmpty {
                        rail(inner: min(992, width - 48), desk: width >= feastDeskFrom)
                            .feastPageColumn(m, wide: wide, rail: true)
                            .padding(.top, m.px(6.4))
                    } else {
                        VStack(spacing: m.px(22.4)) {
                            if stands { StandingBox(view: view) }
                            if !view.texts.isEmpty { texts }
                            years
                        }
                        // With no texts a wide page keeps one column, the web's 36rem.
                        .frame(maxWidth: railed ? 576 : .infinity)
                        .feastColumn(m, wide: wide)
                        .padding(.top, railed ? m.px(6.4) : 0)
                    }
                    Footer()
                }
            }
        }
    }

    /// The texts with the standing and the years as a rail to their right, in a column `inner`
    /// wide; on a first-class feast an upright ruled course midway between them.
    private func rail(inner: CGFloat, desk: Bool) -> some View {
        let side: CGFloat = desk ? 304 : 272
        let gap: CGFloat = desk ? 48 : 40
        let measure: CGFloat = desk ? min(560, inner - side - gap) : inner - side - gap
        let frontal = frontalInk(view.color, p, o)
        return HStack(alignment: .top, spacing: 0) {
            texts.frame(width: measure, alignment: .leading)
            Spacer(minLength: 0)
            VStack(spacing: m.px(22.4)) {
                if stands { StandingBox(view: view, close: !desk) }
                years
            }
            .frame(width: side)
        }
        .overlay {
            if view.register == "first" {
                Canvas { ctx, size in
                    let x = (measure + size.width - side) / 2
                    ctx.fill(ruledLine(x, 4, x, size.height - 4, 1.25, seed: 57), with: .color(p.lining))
                    ctx.fill(ruledLine(x + 3.5, 6, x + 3.5, size.height - 6, 0.8, seed: 58), with: .color(frontal.opacity(0.62)))
                }
                .allowsHitTesting(false)
                .accessibilityHidden(true)
            }
        }
    }

    private var back: some View {
        Button { model.findAgain() } label: {
            Text("‹ \(feastWordsShared.findTitle)").type(TextStyle(size: 15.2, line: 20, tracking: 1.37, smallCaps: true)).foregroundStyle(p.muted)
                .padding(.vertical, m.px(12))
                .frame(minHeight: 44)
        }
        .buttonStyle(Quiet())
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    /// The next eight years, ruled as home's hours: a row says its status only where it differs from the usual.
    private var years: some View {
        VStack(alignment: .leading, spacing: 0) {
            // The headings' strokes are seeded in the page's order: the texts', then the years'.
            SectionHeading(text: view.yearsHeading, seed: view.texts.isEmpty ? 71 : 72)
            FeastYears(years: view.years)
        }
    }

    /// A text's blocks, the collect's opening on the hours' dropped initial (`.feast-text.collect`).
    private func blocks(_ t: FeastTextView, first: Bool) -> [BlockView] {
        guard first, t.prayer, t.blocks.first?.kind == .paragraph else { return t.blocks }
        var blocks = t.blocks
        blocks[0].dropCap = true
        return blocks
    }

    private var texts: some View {
        let w = feastWordsShared
        return VStack(alignment: .leading, spacing: 0) {
            SectionHeading(text: w.textsHeading, seed: 71).padding(.bottom, m.px(4.8))
            Text(view.textsSource).type(TextStyle(size: 14.4, line: 21, italic: true)).foregroundStyle(p.muted).padding(.top, m.px(6.4))
            ForEach(Array(view.texts.enumerated()), id: \.offset) { n, t in
                TextLabel(label: t.label, common: t.common ? w.commonMark : "")
                    .padding(.top, m.px(n == 0 ? 20.8 : 16)).padding(.bottom, m.px(2.4))
                // Each text keeps the hour's rhythm between its blocks, a source's blank lines included.
                ForEach(Array(blocks(t, first: n == 0).enumerated()), id: \.offset) { i, b in
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
                .padding(.top, m.px(17.6))
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
 * The feast page's head, home's day block turned about (`.feast-head`): the feast leads and its
 * usual date is the titulus over it, under one ruled course of the lining with the consecration
 * cross set in it. The register follows the rank: a first-class feast's cross is gilt and its
 * course doubled with a finer rule in its colour; another feast's cross is the lining through the
 * pigment; a commemoration or a monastic observance has a bare head and its name in the text's ink,
 * a size down. The rank and colour stay written under the name, parted by a point of the colour.
 * The head stands on a lit place with no edge, except a plain or a black one. A reader hears the
 * name, then the date, then the rank.
 */
private struct FeastHead: View {
    let view: FeastPageView
    /// The page's width, which sets the name's size as the web's breakpoints do.
    let width: CGFloat
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    private var nameSize: CGFloat {
        let sizes: [CGFloat]
        switch view.register {
        case "first": sizes = [33.6, 40, 43.2]
        case "plain": sizes = [27.2, 31.2, 33.6]
        default: sizes = [31.2, 36.8, 40]
        }
        return sizes[width >= feastDeskFrom ? 2 : width >= feastRailFrom ? 1 : 0]
    }

    var body: some View {
        let first = view.register == "first"
        let plain = view.register == "plain"
        let frontal = frontalInk(view.color, p, o)
        let size = nameSize
        let crown = m.px(first ? 37.6 : 33.6)
        let gap = m.px(first ? 27 : 24)
        let rank = TextStyle(size: 15.68, line: 22, italic: true)
        let parts = view.subtitle.components(separatedBy: " · ")
        VStack(spacing: 0) {
            Text(view.titulus).type(TextStyle(size: 16, line: 24, tracking: 2.4, smallCaps: true)).foregroundStyle(p.titulus)
                .multilineTextAlignment(.center)
                .accessibilitySortPriority(1)
            // By night the name is in the text's ink, as home's date is.
            Text(view.name).type(TextStyle(size: size, line: size * 1.14)).foregroundStyle(plain || p.dark ? p.text : p.rubric)
                .multilineTextAlignment(.center)
                .padding(.top, m.px(1.6))
                .accessibilityAddTraits(.isHeader)
                .accessibilitySortPriority(2)
            MetaLine(space: m.px(9.6)) {
                ForEach(Array(parts.enumerated()), id: \.offset) { i, part in
                    if i > 0 {
                        Rectangle().fill(frontal).frame(width: 5.5, height: 5.5)
                            .rotationEffect(.degrees(45))
                            .frame(height: rank.uiFont(m).lineHeight)
                    }
                    Text(part).type(rank).foregroundStyle(p.muted).multilineTextAlignment(.center)
                }
            }
            .padding(.top, m.px(8.8))
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(view.subtitle)
        }
        .accessibilityElement(children: .contain)
        .padding(.horizontal, m.px(8))
        .padding(.top, m.px(plain ? 3.2 : 25.6))
        .padding(.bottom, m.px(plain ? 16 : width >= feastDeskFrom ? 25.6 : 18.4))
        .frame(maxWidth: .infinity)
        .background {
            if !plain && view.color != "black" { pool }
        }
        .overlay(alignment: .top) {
            if !plain {
                ZStack(alignment: .top) {
                    Canvas { ctx, s in
                        let mid = s.width / 2
                        ctx.fill(ruledLine(0, 0.5, mid - gap, 0.5, 1.25, seed: 51), with: .color(p.lining))
                        ctx.fill(ruledLine(mid + gap, 0.5, s.width, 0.5, 1.25, seed: 52), with: .color(p.lining))
                        if first {
                            ctx.fill(ruledLine(1.5, 4, mid - gap - 3, 4, 0.8, seed: 53), with: .color(frontal.opacity(0.62)))
                            ctx.fill(ruledLine(mid + gap + 3, 4, s.width - 1.5, 4, 0.8, seed: 54), with: .color(frontal.opacity(0.62)))
                        }
                    }
                    .frame(height: 6)
                    cross(crown, gilt: first).offset(y: -crown / 2)
                }
                .allowsHitTesting(false)
                .accessibilityHidden(true)
            }
        }
        .padding(.top, m.px(plain ? 6.4 : 20.8))
    }

    /// The consecration cross on the course: gilt on a first-class feast, as home's, else the lining through the pigment.
    @ViewBuilder
    private func cross(_ size: CGFloat, gilt: Bool) -> some View {
        if gilt {
            Crown(size: size, ink: o.flat, shading: .linearGradient(Gradient(stops: [.init(color: o.hi, location: 0.08), .init(color: mix(o.hi, .white, 0.55), location: 0.3), .init(color: o.flat, location: 0.52), .init(color: o.lo, location: 0.92)]), startPoint: .zero, endPoint: CGPoint(x: size, y: size)))
        } else {
            Crown(size: size, ink: p.lining)
        }
    }

    /// A lit place on the wall under the cross, with no edge: limewash by day, moonlight by night
    /// (`.feast-head::before`), reaching past the head on every side.
    private var pool: some View {
        let light = p.dark ? Color(rgb: 150, 178, 220, 0.17) : Color(rgb: 255, 253, 248, 0.9)
        let tall: CGFloat = p.dark ? 0.48 : 0.46
        return Canvas { ctx, size in
            let rx = size.width * 0.5
            let ry = size.height * tall
            var pool = ctx
            pool.translateBy(x: size.width / 2, y: size.height * 0.46)
            pool.scaleBy(x: 1, y: ry / rx)
            pool.fill(
                Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)),
                with: .radialGradient(Gradient(colors: [light, light.opacity(0)]), center: .zero, startRadius: 0, endRadius: rx)
            )
        }
        .padding(.top, -m.px(40)).padding(.horizontal, -m.px(16)).padding(.bottom, -m.px(24))
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * The feast's standing, drawn as home's invitation (`.feast-standing`): the wash feathered by the
 * tablet mask, the lining's frame, and inside it a finer frame in the colour of the day it is next
 * kept, stopping short of the corners. It says one thing, centred: when the feast is next kept, and
 * the way to that day, which a tap opens. A monastic observance's keeps its explanation as prose,
 * with a ruled hairline before the next keeping. `close` is the narrower rail's smaller date.
 */
private struct StandingBox: View {
    let view: FeastPageView
    var close = false
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        let w = feastWordsShared
        let monastic = !view.monasticNote.isEmpty
        let frontal = frontalInk(view.next?.color, p, o)
        // A black day's is bare, as on home.
        let wash = view.next?.color == "black" ? Color.clear : HomeInk.of(p).tabletWash
        VStack(spacing: m.px(8)) {
            if monastic {
                Text(w.monasticHeading).type(TextStyle(size: 15.2, line: 22, tracking: 2.28, smallCaps: true)).foregroundStyle(p.titulus)
                    .multilineTextAlignment(.center)
                    .accessibilityAddTraits(.isHeader)
                if let all = view.keptByAll {
                    Button { model.open(.feast(all.id)) } label: {
                        (Text(w.everyoneKeeps + " ") + Text(all.name).foregroundColor(p.accent) + Text(" " + view.keptByAllNote))
                            .type(Scale.body.sized(16.8, line: 24)).foregroundStyle(p.text).multilineTextAlignment(.leading)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                    .buttonStyle(Quiet())
                    .accessibilityHint("Opens the feast's page")
                }
                Text(view.monasticNote).type(Scale.small.sized(14.72, line: 20.6)).foregroundStyle(p.muted)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            if let next = view.next {
                let go = next.today ? w.todaysHours : w.daysHours
                Button { model.open(.home(next.date)) } label: { keeping(next, go: go, ruled: monastic) }
                    .buttonStyle(Quiet())
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("\(next.label): \(next.dateLabel). \(go)")
                    .accessibilityHint("Opens the day")
            }
        }
        .padding(.horizontal, m.px(monastic ? 18.4 : 17.6))
        .padding(.top, m.px(monastic ? 16 : 13.6)).padding(.bottom, m.px(monastic ? 14.4 : 15.2))
        .frame(maxWidth: .infinity)
        .background {
            if let tablet = UIImage(named: "tablet.png") {
                Image(uiImage: tablet).renderingMode(.template).resizable().foregroundStyle(wash).padding(1)
            }
        }
        .overlay {
            Canvas { ctx, size in invitationRules(ctx, size, lining: p.lining, frontal: frontal, seed: 61, share: 0.62) }
                .allowsHitTesting(false)
                .accessibilityHidden(true)
        }
    }

    /// The next keeping: its label as a titulus, its date, and the way to the day, in the rubric's
    /// red by day and the lining by night, as home's invitation is.
    private func keeping(_ next: NextKeepingView, go: String, ruled: Bool) -> some View {
        let date: CGFloat = close ? 19.2 : 20.8
        return VStack(spacing: 0) {
            Text(next.label).type(TextStyle(size: 13.76, line: 19, tracking: 2.06, smallCaps: true)).foregroundStyle(p.titulus)
                .multilineTextAlignment(.center)
            Text(next.dateLabel).type(TextStyle(size: date, line: date * 1.3)).foregroundStyle(p.text)
                .multilineTextAlignment(.center)
            Text("\(go) ›").type(TextStyle(size: 19.2, line: 26, tracking: 0.23)).foregroundStyle(p.dark ? p.lining : p.rubric)
                .multilineTextAlignment(.center)
                .padding(.top, m.px(4))
        }
        .frame(maxWidth: .infinity)
        .padding(.top, ruled ? m.px(11.2) : 0)
        .overlay(alignment: .top) {
            if ruled { Stroke(weight: 0.9, seed: 69, color: p.lining.opacity(0.36)) }
        }
        .padding(.top, ruled ? m.px(4.8) : 0)
        .contentShape(Rectangle())
    }
}

/// A text's label in the tituli's small capitals and ink, and "Common" after it when the text is the Common's.
private struct TextLabel: View {
    let label: String
    let common: String
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let caps = TextStyle(size: 16, line: 22, tracking: 0.8, smallCaps: true)
        let name = Text(label).font(Font(caps.uiFont(m))).foregroundColor(p.titulus).tracking(0.8 * m.type)
        let mark = Text(common.isEmpty ? "" : "  " + common).font(Font(TextStyle(size: 13.6, line: 22, italic: true).uiFont(m))).foregroundColor(p.muted)
        return (name + mark)
            .lineSpacing(max(0, caps.line * m.type - caps.uiFont(m).lineHeight))
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityAddTraits(.isHeader)
    }
}

/// A page's headpiece, its title in `ink`, and its subtitle, over a hairline (the ordo's header): the search's.
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

/// A section's heading, a titulus in spaced small capitals over one pen stroke (`.feast-section h2`).
private struct SectionHeading: View {
    let text: String
    let seed: UInt32
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Text(text).type(TextStyle(size: 17.28, line: 24, tracking: 2.6, smallCaps: true)).foregroundStyle(p.titulus)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.bottom, m.px(6.4))
            .overlay(alignment: .bottom) { Stroke(weight: 1.3, seed: seed, color: p.lining.opacity(0.78), foot: true) }
            .accessibilityAddTraits(.isHeader)
    }
}

/**
 * The feast page's years, ruled as home's hour table (`.feast-years`): a firmer rule above and
 * below and finer ones between, never upright, with no colour bars or chevrons. Each row has its
 * year in old-style figures, a point in that year's day colour, its date, and its status only
 * where it differs from the feast's usual; it is one touch target tall, and pressed it lies under
 * home's pool of light.
 */
private struct FeastYears: View {
    let years: [FeastYearView]
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(spacing: 0) {
            ForEach(Array(years.enumerated()), id: \.offset) { i, y in
                entry(y)
                    .overlay(alignment: .top) {
                        Stroke(weight: i == 0 ? 1.3 : 0.9, seed: UInt32(81 + i), color: p.lining.opacity(i == 0 ? 0.78 : 0.36))
                    }
            }
        }
        .overlay(alignment: .bottom) {
            if !years.isEmpty { Stroke(weight: 1.3, seed: UInt32(81 + years.count), color: p.lining.opacity(0.78), foot: true) }
        }
    }

    @ViewBuilder
    private func entry(_ y: FeastYearView) -> some View {
        let spoken = ["\(y.year)", y.dateLabel, y.usual ? "" : y.label, y.note].filter { !$0.isEmpty }.joined(separator: ", ")
        if let date = y.date {
            Button { model.open(.home(date)) } label: { row(y) }
                .buttonStyle(Pooled(pool: HomeInk.of(p).nowPool))
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(spoken)
                .accessibilityHint("Opens the day")
        } else {
            row(y)
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(spoken)
        }
    }

    private func row(_ y: FeastYearView) -> some View {
        let quiet = y.kind == "monastic" || y.kind == "not-kept"
        let point = y.color == "white" ? o.flat : titleColor(y.color, p)
        return HStack(alignment: .firstTextBaseline, spacing: m.px(8.8)) {
            Text(String(y.year)).type(TextStyle(size: 16, line: 24, tracking: 0.64)).foregroundStyle(mix(p.titulus, p.muted, 0.55))
                .frame(width: 46.4 * max(m.layout, m.type), alignment: .leading)
            // The point stands at the middle of the date's small letters.
            Rectangle().fill(point.opacity(0.85)).frame(width: 4.5, height: 4.5)
                .rotationEffect(.degrees(45))
                .frame(width: m.px(11.2))
                .alignmentGuide(.firstTextBaseline) { $0[VerticalAlignment.center] + 4 * m.type }
            VStack(alignment: .leading, spacing: 0) {
                Text(y.dateLabel).type(TextStyle(size: 17.28, line: 24)).foregroundStyle(p.text)
                if !y.usual { Text(y.label).type(TextStyle(size: 14.72, line: 21, italic: quiet)).foregroundStyle(keepingInk(y.kind, p)) }
                if !y.note.isEmpty { Text(y.note).type(TextStyle(size: 13.76, line: 18)).foregroundStyle(p.muted) }
            }
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(.horizontal, m.px(4.8))
        .padding(.top, m.px(6.7)).padding(.bottom, m.px(6.1))
        .frame(minHeight: 44)
    }
}

/// A year's row pressed: under home's pool of light, with no edge (`.feast-years a.find-year:active`).
private struct Pooled: ButtonStyle {
    let pool: Color

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .contentShape(Rectangle())
            .background {
                if configuration.isPressed {
                    Canvas { ctx, size in
                        // radial-gradient(ellipse 60% 90% at 30% 50%, pool, transparent 75%)
                        let rx = size.width * 0.6
                        let ry = size.height * 0.9
                        var light = ctx
                        light.translateBy(x: size.width * 0.3, y: size.height / 2)
                        light.scaleBy(x: 1, y: ry / rx)
                        light.fill(
                            Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)),
                            with: .radialGradient(Gradient(stops: [.init(color: pool, location: 0), .init(color: pool.opacity(0), location: 0.75)]), center: .zero, startRadius: 0, endRadius: rx)
                        )
                    }
                    .allowsHitTesting(false)
                }
            }
    }
}

/// A year's keeping in the search's results: its year, date and status on the day's colour rail,
/// leading to that day.
private struct YearRow: View {
    let y: FeastYearView
    let first: Bool
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
        .padding(.vertical, m.px(8))
        .frame(minHeight: 44)
    }
}

/// A dotted hairline between a feast's years in the search's results.
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
