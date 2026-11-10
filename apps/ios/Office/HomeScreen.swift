import SwiftUI

private let periods: [(period: Period, label: String, hours: [String])] = [
    (.morning, "Morning", ["lauds", "prime"]),
    (.day, "Day", ["terce", "sext", "none"]),
    (.evening, "Evening", ["vespers", "compline"]),
]

/// Home for a day: its frontispiece, the invitation to pray, and the hours of the day.
struct HomePage: View {
    let date: CivilDate
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let today = model.today
        let clock = model.clockHour
        Loaded(key: "\(date.iso) \(today.iso) \(clock)", motion: model.motion(to: .home(date))) {
            try Office.core.get().home(date: date, today: today, clockHour: Int32(clock))
        } content: { view in
            HomeScreen(view: view, date: date)
                .onChange(of: view.ornament, initial: true) { _, season in model.composed(season: season, page: .home(date)) }
        }
        .background(PlasterWall())
        .toolbar(.hidden, for: .navigationBar)
    }
}

struct HomeScreen: View {
    let view: HomeView
    let date: CivilDate
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide
    /// Where the niche stands on the screen, which the chapel light follows.
    @State private var nicheFrame: CGRect?

    var body: some View {
        let p = self.p.seasoned(view.ornament)
        let o = Ornament.of(p, season: view.ornament)
        GeometryReader { geo in
            let screen = geo.size.width
            let niche = wide ? NicheTokens.of(p) : nil
            ZStack(alignment: .top) {
                // The wall's one field, fixed to the screen and phased from its top: by night the
                // vault, clearing the header and thinning toward the foot; by day the powdering,
                // cut square under the beam.
                let top = geo.safeAreaInsets.top
                // On a phone by night the stars are the apse's own, inside the arch (`Sky`): the
                // wall outside it is plain dark plaster.
                if !(p.dark && !wide) {
                    WallField(seam: top) { h, dark in
                        func at(_ d: CGFloat) -> CGFloat { (top + d) / h }
                        return dark
                            ? [(0, 0), (at(56), 0), (at(112), 1), (max(0.78, at(112)), 1), (1, 0.6)]
                            : [(0, 0), (at(64), 0), (at(64), 1), (1, 1)]
                    }
                    .ignoresSafeArea()
                }
                // A wide screen sets the frontispiece in a niche, and lights the room toward it.
                if let niche { ChapelLight(t: niche, niche: nicheFrame).ignoresSafeArea() }
                ScrollViewReader { scroll in
                ScrollView {
                    // Home is at least a screen tall, its colophon at the foot. The tiers go by
                    // the height home has and the screen's width, in the web's pixels.
                    let tall = geo.size.height
                    if let niche {
                        let card = nicheWidth(screen)
                        VStack(spacing: 0) {
                            SiteHeader()
                            // Centred between the header and the foot, as the web's desktop home.
                            Spacer(minLength: 0)
                            // The moulding stands 0.75rem out from the card; room for it below the header.
                            Frontispiece(view: view, date: date, tier: HomeTier(desk: true, screen: screen / m.layout, tall: tall / m.layout, card: card / m.layout, dyn: m.type / m.layout), niche: niche)
                                .background(GeometryReader { card in
                                    Color.clear.preference(key: NicheFrameKey.self, value: card.frame(in: .global))
                                })
                                .frame(maxWidth: card)
                                .padding(.horizontal, 24)
                                .padding(.top, 40)
                                .padding(.bottom, 12)
                            Spacer(minLength: 0)
                            Footer(reserve: true)
                        }
                        .frame(minHeight: tall)
                    } else {
                        // A phone's card stands from the header to a little above the footer:
                        // whatever height home has beyond its own goes to the panel, above the band.
                        let card = min(m.px(576), screen - 2 * m.px(gutter))
                        PhoneHome(height: tall) {
                            SiteHeader()
                            Frontispiece(view: view, date: date, tier: HomeTier(desk: false, screen: screen / m.layout, tall: tall / m.layout, card: card / m.layout, dyn: m.type / m.layout), niche: nil)
                                .frame(maxWidth: m.px(576))
                                .padding(.horizontal, m.px(gutter))
                                .padding(.top, m.px(13.6))
                            Footer(gap: 29.6, bottom: 25.6, reserve: true)
                        }
                    }
                }
                .revealing(scroll)
                }
            }
        }
        .onPreferenceChange(NicheFrameKey.self) { nicheFrame = $0 }
        .environment(\.ornament, o)
        .environment(\.palette, p)
    }
}

/// The niche's frame on the screen.
private struct NicheFrameKey: PreferenceKey {
    static let defaultValue: CGRect? = nil
    static func reduce(value: inout CGRect?, nextValue: () -> CGRect?) { value = nextValue() ?? value }
}

/**
 * A phone's home: the header, the card and the footer, at least `height` tall, the card offered
 * the height left over as its least, so it stands from the header to a little above the footer.
 */
private struct PhoneHome: Layout {
    let height: CGFloat

    private func heights(_ width: CGFloat, _ subviews: Subviews) -> (header: CGFloat, card: CGFloat, footer: CGFloat, spare: CGFloat) {
        let free = ProposedViewSize(width: width, height: nil)
        let header = subviews[0].sizeThatFits(free).height
        let footer = subviews[2].sizeThatFits(free).height
        let spare = max(0, height - header - footer)
        let card = subviews[1].sizeThatFits(ProposedViewSize(width: width, height: spare)).height
        return (header, card, footer, spare)
    }

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? 390
        let h = heights(width, subviews)
        return CGSize(width: width, height: max(h.header + h.card + h.footer, height))
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let h = heights(bounds.width, subviews)
        let free = ProposedViewSize(width: bounds.width, height: nil)
        subviews[0].place(at: bounds.origin, proposal: free)
        subviews[1].place(at: CGPoint(x: bounds.minX, y: bounds.minY + h.header), proposal: ProposedViewSize(width: bounds.width, height: h.spare))
        // The colophon at the foot of the screen.
        subviews[2].place(at: CGPoint(x: bounds.minX, y: bounds.maxY - h.footer), proposal: free)
    }
}

/**
 * Home's measures at a size of screen, in the web's pixels, as style.css's home tiers set them.
 * A phone's head, its cross and the room above the date go by the height home has (`tall`): from
 * 800 high the card has height to spare, so the head rises further to a sharper point, the cross
 * and the date stand lower in it, and the spare height is parted two to three above and below the
 * day rather than centred about it; from 880 more so. Its larger rows and controls go by width as
 * well, from 375 wide and 830 or 880 high: a narrower or shorter phone needs the height for the
 * lines its day wraps to. A wide screen's niche (`desk`) has its own. `card` is the card's width;
 * `dyn` the reader's Dynamic Type size over the text size's own scale, which `Metrics.type` carries.
 */
struct HomeTier {
    let desk: Bool
    private let step: Int
    private let type: Int
    private let short: Bool
    private let card: CGFloat
    private let tall: CGFloat
    private let dyn: CGFloat

    init(desk: Bool, screen: CGFloat, tall: CGFloat, card: CGFloat, dyn: CGFloat = 1) {
        self.desk = desk
        step = desk ? 0 : tall >= 880 ? 2 : tall >= 800 ? 1 : 0
        type = desk || screen < 375 ? 0 : tall >= 880 ? 2 : tall >= 830 ? 1 : 0
        short = !desk && tall <= 700
        self.card = card
        self.tall = tall
        self.dyn = dyn
    }

    /**
     * The head's type is fitted to the panel, not to Dynamic Type (the web's `--date-size`). The
     * head is as tall as the arch's rise, a share of the card's width, plus whatever height home
     * has to spare, so type set by the reader's size floats in it set small and crowds it set
     * large. The date is 7% of the card's inline size (its width inside 16pt of padding) or 2.92%
     * of home's height, whichever is more, within bounds that keep Dynamic Type; the feast and the
     * commemorations follow in proportion. A style's size is scaled by `Metrics.type` when set, so
     * Dynamic Type is divided out of the fitted size here. The usual size at 375×667 gives the
     * sizes the head had (22, 17 and 14px), a tall phone the sizes its tier had (25 and 27px for
     * the date). A niche keeps its own sizes.
     */
    private func fitted(_ inline: CGFloat, _ height: CGFloat, _ lo: CGFloat, _ hi: CGFloat) -> CGFloat {
        min(max(max(inline, height) / dyn, lo), hi)
    }
    private var inline: CGFloat { card - 32 }

    private func pick<T>(_ niche: T, _ phone: T...) -> T { desk ? niche : phone[min(type, phone.count - 1)] }
    private func rise<T>(_ niche: T, _ phone: T...) -> T { desk ? niche : phone[step] }

    var arch: Arch { rise(Arch.niche, Arch.phone, Arch.tall, Arch.taller) }
    /**
     * The room under the point for the cross and air, before the date. On a phone the date's tap
     * box is a thumb's height with its line at the foot, so the room gives back the box's slack
     * above the line: the date stands where it stood centred in the box, the feast closer under it.
     */
    var headPad: CGFloat { rise(120, 86.4, 105.6, 118.4) - (desk ? 0 : (44 - date.line * dyn) / 2) }
    var crownTop: CGFloat { rise(48, 33.6, 49.6, 54.4) }
    var crownSize: CGFloat { rise(36, 30.4, 35.2, 40) }
    /// Spare height under the head is parted 2:3 above and below the day, else the day is centred in it.
    var split: Bool { !desk && step > 0 }
    var side: CGFloat { desk ? 28 : 16 }
    private var lining: CGFloat { desk ? 26 : 12 }
    /**
     * How far in from the card's edge the day's words stand: inside the lining's hairline (its
     * inset, then 9pt) with 12pt of clear air, so a long feast name breaks rather than running
     * over the lining.
     */
    var dayClear: CGFloat { lining + 9 + 12 }
    var bottom: CGFloat { desk ? 20 : 12 }
    var date: TextStyle {
        if desk { return Scale.body.sized(25.92, line: 31.1).tracked(0.39) }
        let size = fitted(0.07 * inline, 0.0292 * tall, 19.2, 32)
        return Scale.body.sized(size, line: size * 1.2).tracked(size * 0.01)
    }
    /**
     * The head's width `y` below the card's top inside the lining's hairline, less the day's 12pt
     * of air each side: a line of the day set there clears the lining.
     */
    private func clear(_ y: CGFloat) -> CGFloat { archChord(arch, width: card, outset: -(lining + 9), at: y) - 24 }
    /**
     * The date's measure, the head's width where its first line stands, and no wider than a short
     * phone's: a date too long for it breaks after the weekday, its second line lower where the
     * head is wider.
     */
    var dateMeasure: CGFloat? { desk ? nil : clear(headPad + 44 - date.line * dyn) }
    /**
     * On a phone the weekday is a titulus in spaced small caps over the rest of the date set 10%
     * larger (6% on a short phone), as the web's `.home-date-weekday` and `.home-date-rest`. A day
     * with no room to give (two commemorations, or one opened from another day) closes the title
     * up on a taller phone.
     */
    func weekday(heavy: Bool) -> TextStyle {
        let size = max(0.66 * date.size, 15.2 / dyn)
        return TextStyle(size: size, line: size * (short ? 1.18 : heavy ? 1.06 : 1.5), tracking: size * 0.15, smallCaps: true)
    }
    func rest(heavy: Bool) -> TextStyle {
        let size = date.size * (short ? 1.06 : heavy ? 1.02 : 1.1)
        return Scale.body.sized(size, line: size * (short ? 1.08 : heavy ? 1 : 1.12)).tracked(size * 0.005)
    }
    var feast: TextStyle {
        if desk { return Scale.body.sized(18.72, line: 23.4) }
        let size = fitted(0.055 * inline, 0.0215 * tall, 15.2, 24)
        return Scale.body.sized(size, line: size * 1.25)
    }
    /**
     * The feast's measure, the head's width at its first line, when the day stands at the head's
     * room: a long name breaks there rather than running over the lining further up the arch.
     */
    var feastMeasure: CGFloat? { desk ? nil : clear(headPad + 44 + 2) }
    /// A short phone's commemorations give way, so a past date with them still fits.
    var commemoration: TextStyle {
        if short { return TextStyle(size: 13.6, line: 17.68) }
        if desk { return TextStyle(size: 14, line: 20) }
        let size = fitted(0.046 * inline, 0.0175 * tall, 12.8, 17.6)
        return TextStyle(size: size, line: size * 1.4)
    }
    /// The day's versicle, set only where a phone is over 700 high, a measure at most 19rem wide
    /// and inside the day's clearance of the lining.
    var versicle: TextStyle? { short ? nil : TextStyle(size: 15.36, line: 20.28, italic: true) }
    var versicleMeasure: CGFloat { min(304, card - 2 * dayClear) }
    /// Below the day, to the band.
    var dayGap: CGFloat { pick(16.8, 5.6, 12) }
    var band: CGFloat { pick(12.8, 12.8, 12.8, 13.6) }
    var bandPad: CGFloat { pick(3.2, 3.2, 3.2, 3.84) }
    var bandGap: CGFloat { pick(15.2, 12, 16, 20) }
    var pray: TextStyle { pick(Scale.body.sized(23.2, line: 30.16), Scale.body.sized(19.2, line: 24.96), Scale.body.sized(22.4, line: 29.12), Scale.body.sized(24, line: 31.2)).tracked(0.38) }
    var prayPad: CGFloat { pick(13.8, 11.9, 13.5, 15.9) }
    var prayGap: CGFloat { pick(15.2, 11.2, 20, 24) }
    var row: CGFloat { pick(46, 44, 54.4, 62.4) }
    var hour: CGFloat { pick(17.6, 15.68) }
    var metaGap: CGFloat { pick(14.4, 8.8, 16, 19.2) }
}

/// The date with its weekday and the rest each kept whole, so a break never parts a month from its day.
private func unbroken(_ date: String) -> String {
    guard let comma = date.range(of: ", ") else { return date }
    let keep = { (s: Substring) in s.replacingOccurrences(of: " ", with: "\u{00A0}") }
    return keep(date[..<comma.lowerBound]) + ", " + keep(date[comma.upperBound...])
}

/**
 * The frontispiece's courses (`.home-hero`): the lining round the head, the cross in its point,
 * the day under the head's room for them, then from the inscription band the invitation and the
 * hours. The day's part (`.home-summary`) is never shorter than the head, so the band never
 * crosses the arch, and takes whatever height the card is offered beyond its own, the day
 * centred in it or set two parts to three. "Change date"'s picker stands under the hours, apart
 * from them so the head is measured without it.
 */
private struct FrontispieceLayout: Layout {
    let tier: HomeTier
    let m: Metrics
    /// How far the picker has unfolded, from 0 to 1, eased frame by frame with the disclosure.
    var open: CGFloat

    var animatableData: CGFloat {
        get { open }
        set { open = newValue }
    }

    /// The head's height, the day's parts (its facts, the versicle when it fits, and "Go to
    /// today"), the hours, the picker as far as it has unfolded, and the whole. The versicle takes
    /// only the height the head has to spare: where it would make the card taller, the head goes
    /// without it. It is fitted to the card with the picker shut, as the web's is, so opening the
    /// picker never takes it away.
    private func measure(_ width: CGFloat, _ proposal: ProposedViewSize, _ subviews: Subviews) -> (summary: CGFloat, facts: CGFloat, versicle: CGFloat?, back: CGFloat, after: CGFloat, picker: CGFloat, height: CGFloat) {
        let free = ProposedViewSize(width: width, height: nil)
        let facts = subviews[2].sizeThatFits(free).height
        let versicle = subviews[3].sizeThatFits(free).height
        let back = subviews[4].sizeThatFits(free).height
        let after = subviews[5].sizeThatFits(free).height
        // The picker's full height; none once it has folded away.
        let full = subviews[6].sizeThatFits(free).height
        let unfolded = min(max(open, 0), 1)
        let foot = m.px(tier.bottom)
        let pad = m.px(tier.headPad)
        let day = facts + back + m.px(tier.dayGap)
        let rise = tier.arch.rise * width
        let least = proposal.height.map { $0.isFinite ? $0 : 0 } ?? 0
        let shut = max(pad + day, rise, least - after - foot)
        let fits = versicle > 0 && pad + day + versicle <= shut
        // The picker takes the head's spare height before it lengthens the card. Taken as it
        // unfolded, that height went in the first frames: the day leapt up, then stood while the
        // card grew under the picker. Instead the head gives it up in step with the unfolding, so
        // the day rises as the picker opens and settles with it.
        let opened = max(pad + day + (fits ? versicle : 0), rise, least - after - full - foot)
        let summary = full > 0 ? shut - (shut - opened) * unfolded : shut
        let picker = full * unfolded
        return (summary, facts, fits ? versicle : nil, back, after, picker, summary + after + picker + foot)
    }

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? 390
        return CGSize(width: width, height: measure(width, proposal, subviews).height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let (summary, facts, versicle, back, after, picker, _) = measure(bounds.width, proposal, subviews)
        let free = ProposedViewSize(width: bounds.width, height: nil)
        let pad = m.px(tier.headPad)
        let spare = summary - pad - facts - (versicle ?? 0) - back - m.px(tier.dayGap)
        subviews[0].place(at: bounds.origin, proposal: ProposedViewSize(width: bounds.width, height: summary))
        subviews[1].place(at: CGPoint(x: bounds.midX, y: bounds.minY + m.px(tier.crownTop)), anchor: .top, proposal: .unspecified)
        var y = bounds.minY + pad + (tier.split ? spare * 2 / 5 : spare / 2)
        subviews[2].place(at: CGPoint(x: bounds.minX, y: y), proposal: free)
        y += facts
        // Proposed no height, the versicle gives way to nothing; a point's grace keeps rounding
        // from refusing it the height it asked for.
        subviews[3].place(at: CGPoint(x: bounds.minX, y: y), proposal: ProposedViewSize(width: bounds.width, height: versicle.map { $0 + 1 } ?? 0))
        y += versicle ?? 0
        subviews[4].place(at: CGPoint(x: bounds.minX, y: y), proposal: free)
        subviews[5].place(at: CGPoint(x: bounds.minX, y: bounds.minY + summary), proposal: free)
        // Revealed from its top as far as it has unfolded.
        subviews[6].place(at: CGPoint(x: bounds.minX, y: bounds.minY + summary + after), proposal: ProposedViewSize(width: bounds.width, height: picker))
    }
}

/**
 * The frontispiece's frame (`.home-hero`), the same at every width; the rules within are ruled by
 * hand in the lining (`HourDirectory`, `PrayNow`).
 */
private func frontispieceFrame(_ p: Palette) -> Color {
    p.dark ? Color(rgb: 208, 176, 106, 0.34) : Color(rgb: 87, 52, 33, 0.3)
}

private struct Frontispiece: View {
    let view: HomeView
    let date: CivilDate
    let tier: HomeTier
    let niche: NicheTokens?
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @State private var picking = false
    /// Whether the picker is laid out: from its opening until it has folded away.
    @State private var unfolded = false
    /// The painter's reserve round the day's words, which the sky's stars keep clear of.
    @State private var reserve: [String: CGRect] = [:]
    @Environment(\.reveal) private var reveal

    /// By night a phone's head is the apse's sky (the web's `.home-sky`).
    private var night: Bool { p.dark && !tier.desk }
    private var home: HomeInk { HomeInk.of(p) }
    /// The day's red work and muted ink, lifted on the sky to keep their contrast.
    private var rubric: Color { night ? home.skyRubric : p.rubric }
    private var muted: Color { night ? home.skyMuted : p.muted }

    var body: some View {
        let day = dayColor(view.color, p)
        // The lining's inner line is the day's colour, beside the cross on the plaster; a white
        // day's would be tan there, and takes the gold line by day.
        let liningDay = !p.dark && view.color == "white" ? p.goldLine : day
        let frame = frontispieceFrame(p)
        let desk = tier.desk
        let side = m.px(tier.side)
        let home = self.home
        let catchLight = leafCatch(view.ornament)
        let leaf = Leaf(o)
        let black = view.color == "black"
        let first = view.firstClass
        // The frontal: the day's colour inside the invitation's frame; on a white day gold by day
        // and lime white by night on the sky, where the fillet is the one gold line; the lining
        // thinned on a bare black day.
        let frontal = black ? p.lining.opacity(0.8) : view.color == "white" ? (night ? home.limeWhite : p.goldLine) : day
        let comma = view.dateLabel.range(of: ", ")
        let heavy = view.commemorations.count > 1 || !view.isToday
        FrontispieceLayout(tier: tier, m: m, open: picking ? 1 : 0) {
            // A lining painted round the head on the back wall, down to the inscription band:
            // on a phone 0.75rem inside the panel's edge, in a niche 26pt inside the moulding. By
            // night on a phone, the apse's sky with its stars, ringed by a leaf fillet and the
            // day's hairline.
            if night {
                let crown = m.px(tier.crownSize)
                let top = m.px(tier.crownTop)
                GeometryReader { g in
                    let w = g.size.width
                    let clear = reserve.values.map { $0.insetBy(dx: -14, dy: -6) }
                        + [CGRect(x: (w - crown) / 2 - 11, y: top - 11, width: crown + 22, height: crown + 22)]
                    Sky(arch: tier.arch, ink: home, leaf: leaf, lit: catchLight, stars: !black, first: first, hairline: black ? nil : view.color == "white" ? home.limeWhite : day, reserve: clear)
                }
            } else {
                Lining(arch: tier.arch, inset: m.px(desk ? 26 : 12), hairline: liningDay, pigmented: true)
            }
            // The consecration cross in the point of the head, with clear air round it: painted
            // through the pigment, gilt on a first-class feast by day, leaf by night on the sky.
            Group {
                let size = m.px(tier.crownSize)
                if night && black {
                    Crown(size: size, ink: muted.opacity(0.7), shading: .color(muted.opacity(0.7)))
                } else if night {
                    Crown(size: size, ink: leaf.mid, shading: .linearGradient(Gradient(stops: [.init(color: leaf.hi, location: 0), .init(color: leaf.mid, location: 0.55), .init(color: leaf.lo, location: 1)]), startPoint: .zero, endPoint: CGPoint(x: size, y: size)), alpha: 0.35 + 0.65 * catchLight)
                } else if first {
                    Crown(size: size, ink: o.flat, shading: .linearGradient(Gradient(stops: [.init(color: o.hi, location: 0.08), .init(color: mix(o.hi, .white, 0.55), location: 0.3), .init(color: o.flat, location: 0.52), .init(color: o.lo, location: 0.92)]), startPoint: .zero, endPoint: CGPoint(x: size, y: size)))
                } else {
                    Crown(size: size, ink: p.lining)
                }
            }
            // The day.
            VStack(spacing: 0) {
                Button { model.open(.ordo(year: Int(date.year), month: Int(date.month), day: Int(date.day))) } label: {
                    if let comma, !desk {
                        // On a phone the weekday is a titulus over the rest of the date, the two a
                        // full thumb's height at least, as the web's, the rest at the foot so the
                        // feast stands close under it. A reader hears "Tuesday, November 10, 2026".
                        VStack(spacing: 0) {
                            Text(view.dateLabel[..<comma.lowerBound])
                                .type(tier.weekday(heavy: heavy))
                                .foregroundStyle(p.titulus)
                                .reserved("weekday")
                            Text(unbroken(String(view.dateLabel[comma.upperBound...])))
                                .type(tier.rest(heavy: heavy))
                                .foregroundStyle(p.text)
                                .multilineTextAlignment(.center)
                                .reserved("date")
                        }
                        .frame(minHeight: m.px(44), alignment: .bottom)
                        .accessibilityElement(children: .ignore)
                        .accessibilityLabel(view.dateLabel)
                    } else {
                        Text(unbroken(view.dateLabel))
                            .type(tier.date)
                            .foregroundStyle(p.text)
                            .multilineTextAlignment(.center)
                            .reserved("date")
                            .frame(maxWidth: tier.dateMeasure.map { m.px($0) }, minHeight: tier.desk ? nil : m.px(44), alignment: .bottom)
                    }
                }
                .buttonStyle(Quiet())
                .accessibilityAddTraits(.isHeader)
                .accessibilityHint("Opens the ordo")
                // The day's feast leads to its page, underlined as the web's `.feast-link`.
                Group {
                    if view.feastId.isEmpty {
                        FeastName(name: view.feastName, alias: view.feastAlias, style: tier.feast)
                    } else {
                        Button { model.open(.feast(view.feastId)) } label: {
                            FeastName(name: view.feastName, alias: view.feastAlias, style: tier.feast, linked: true)
                        }
                        .buttonStyle(Quiet())
                        .accessibilityHint("Opens the feast's page")
                    }
                }
                .foregroundStyle(p.accent)
                .reserved("feast")
                .frame(maxWidth: tier.feastMeasure.map { m.px($0) })
                .padding(.top, m.px(2))
                if !view.octaveNote.isEmpty {
                    Text(view.octaveNote).type(Scale.small).foregroundStyle(muted).multilineTextAlignment(.center).reserved("octave")
                }
                if !view.penitential.isEmpty {
                    HStack(spacing: m.px(9.6)) {
                        ForEach(view.penitential, id: \.self) { t in
                            Text(t).type(TextStyle(size: 12.48, line: 19.97, tracking: 0.75, smallCaps: true)).foregroundStyle(rubric).reserved("fast " + t)
                        }
                    }
                    .padding(.top, m.px(12))
                }
                if !view.commemorations.isEmpty {
                    VStack(spacing: 0) {
                        Text("ALSO").type(.label(10.56, 0.1)).foregroundStyle(muted).reserved("also")
                        // A second commemoration is joined to the first with "and", as the web's
                        // `.commemorations li + li::before`, so the list reads as a sentence.
                        // Each leads to its feast's page.
                        ForEach(Array(view.commemorations.enumerated()), id: \.offset) { i, c in
                            let id = i < view.commemorationIds.count ? view.commemorationIds[i] : ""
                            let line = (i == 0 ? Text("") : Text("and ").font(Font(garamond(tier.commemoration.size * m.type, italic: true)))) + Text(c).underline(!id.isEmpty, color: p.goldLine)
                            if id.isEmpty {
                                line.type(tier.commemoration).foregroundStyle(p.text).multilineTextAlignment(.center).reserved("also \(i)")
                            } else {
                                Button { model.open(.feast(id)) } label: {
                                    line.type(tier.commemoration).foregroundStyle(p.text).multilineTextAlignment(.center).reserved("also \(i)")
                                }
                                .buttonStyle(Quiet())
                                .accessibilityHint("Opens the feast's page")
                            }
                        }
                    }
                    .padding(.top, m.px(8))
                }
            }
            .padding(.horizontal, m.px(tier.dayClear))
            .frame(maxWidth: .infinity)
            // The day's versicle, when the head has the height to spare for it: the layout
            // proposes it no height where it would make the card taller, and it gives way.
            ViewThatFits(in: .vertical) {
                if let style = tier.versicle, !view.versicle.isEmpty {
                    VStack(spacing: 0) {
                        versicleLine("℣.", view.versicle, style).reserved("versicle")
                        versicleLine("℟.", view.response, style).reserved("response")
                    }
                    .frame(maxWidth: m.px(tier.versicleMeasure))
                    .padding(.top, m.px(12))
                }
                Color.clear.frame(height: 0)
            }
            .frame(maxWidth: .infinity)
            // After the day's facts, so they read together.
            VStack(spacing: 0) {
                if !view.isToday {
                    Button { model.open(.home(model.today)) } label: {
                        Text("GO TO TODAY").type(Scale.menu).foregroundStyle(p.accent)
                            .reserved("today")
                            .goldUnderline(true, p.goldLine)
                            .padding(.vertical, m.px(12))
                    }
                    .buttonStyle(Quiet())
                }
            }
            .frame(maxWidth: .infinity)
            // The invitation and the hours, from the inscription band.
            VStack(spacing: 0) {
                inscription(side: side)
                PrayNow(label: view.prayNowLabel, tier: tier, frontal: frontal, wash: black ? .clear : home.tabletWash, lifted: night ? home.liningLifted : nil) {
                    model.open(.hour(view.prayNowDate, view.prayNowHour))
                }
                .padding(.top, m.px(tier.bandGap))
                HourDirectory(current: view.currentHour, note: view.currentHourNote, prayed: model.hoursPrayed(on: date), tier: tier, now: titulusNow(p, season: view.ornament), home: home) { h in model.open(.hour(date, h)) }
                    .padding(.top, m.px(tier.prayGap))
                // The season and "Change date" share one line after the hours, centred (the web's
                // `.home-day-meta`), in small caps, a gilt point between them. Open, the date
                // control takes a line of its own beneath the season, and the picker the card's width.
                let foot = TextStyle(size: 13.44, line: 20.16, tracking: 13.44 * 0.09, smallCaps: true)
                let change = Disclosure(label: "Change date", open: picking, style: foot, upper: false) { togglePicker() }
                if view.season.isEmpty {
                    change.padding(.top, m.px(tier.metaGap + 1))
                } else {
                    let season = HStack(spacing: 0) {
                        Text(view.season).type(foot).foregroundStyle(p.muted)
                            .multilineTextAlignment(.center)
                        if !picking {
                            Canvas { ctx, s in ctx.fill(lozenge(CGPoint(x: s.width / 2, y: s.height / 2), 2.5), with: .color(o.flat.opacity(0.7))) }
                                .frame(width: 5, height: 5)
                                .padding(.leading, m.px(14.4))
                                .accessibilityHidden(true)
                        }
                    }
                    // One layout either way, so "Change date" stays the control VoiceOver is on
                    // as it opens.
                    SeasonLine(spacing: m.px(8.8), above: m.px(3.2), stacked: picking) {
                        season
                        change
                    }
                    .padding(.top, m.px(tier.metaGap + 1))
                }
            }
            .padding(.horizontal, side)
            .frame(maxWidth: .infinity)
            // "Change date"'s picker, revealed from its top as the layout gives it its height
            // (the disclosure's own unfolding, Android's `Unfold`), and laid out only while shown.
            VStack(spacing: 0) {
                if unfolded {
                    DayPicker(shown: date, today: model.today, centred: true) { d in
                        picking = false
                        unfolded = false
                        model.open(.home(d))
                    }
                    .transition(.identity)
                }
            }
            .fixedSize(horizontal: false, vertical: true)
            .frame(minHeight: 0, maxHeight: .infinity, alignment: .top)
            .clipped()
            .opacity(picking ? 1 : 0)
            .accessibilityHidden(!picking)
            .padding(.horizontal, side)
            .frame(maxWidth: .infinity)
            .id("disclosed")
        }
        .coordinateSpace(name: "frontispiece")
        .onPreferenceChange(ReserveKey.self) { reserve = $0 }
        .background {
            if let niche {
                // The niche: a pointed head, the stone moulding, the day's colour as its trim.
                Niche(arch: tier.arch, t: niche, day: day, frame: frame)
            } else {
                // The panel: the day's colour a little toward the frame as a ring at its edge, so
                // a red or green day edges the head without outshouting the cross; by night the
                // slate course's outer ring in shade, round the sky.
                Panel(arch: tier.arch, ring: night ? home.skyRim : mix(day, frame, 0.7), frame: frame)
            }
        }
    }

    /// Opens "Change date"'s picker, or folds it away and then lets it go, on the disclosures' curves.
    private func togglePicker() {
        if picking {
            withAnimation(folding) { picking = false } completion: {
                // Opened again while it folded, it stays.
                if !picking { unfolded = false }
            }
        } else {
            unfolded = true
            withAnimation(unfolding) { picking = true }
            reveal("disclosed")
        }
    }

    /// A line of the versicle: its ℣ or ℟ upright in the rubrics' red, the words in italic, the
    /// text's ink a little withdrawn.
    private func versicleLine(_ sigil: String, _ words: String, _ style: TextStyle) -> some View {
        (Text(sigil).font(Font(garamond(style.size * m.type))).foregroundColor(rubric) + Text(" " + words))
            .type(style)
            .foregroundStyle(p.text.opacity(0.84))
            .multilineTextAlignment(.center)
            .fixedSize(horizontal: false, vertical: true)
    }

    /// The inscription band: flat green earth between oxblood edges and leaf fillets, ruled by
    /// hand (the web's `rule()`), its letters leaf that catches the light at their heads, run
    /// through to the frame.
    private func inscription(side: CGFloat) -> some View {
        // Gilt lozenges either side, 5pt squares on their points.
        Lozenged(
            text: "Pray the hours",
            style: TextStyle(size: tier.band, line: tier.band * 1.6, tracking: tier.band * (tier.desk ? 0.17 : 0.15), smallCaps: true),
            ink: o.ink,
            letters: LinearGradient(stops: [.init(color: mix(o.ink, .white, 0.7), location: 0.2), .init(color: o.ink, location: 0.58)], startPoint: .top, endPoint: .bottom)
        )
        .padding(.vertical, m.px(tier.bandPad))
        .frame(maxWidth: .infinity)
        .background(p.inscriptionGround)
        .overlay {
            Canvas { ctx, size in
                let w = size.width
                let h = size.height
                ctx.fill(ruledLine(0, 0.6, w, 0.6, 1.3, seed: 11), with: .color(p.inscriptionEdge))
                ctx.fill(ruledLine(0, h - 0.6, w, h - 0.6, 1.3, seed: 12), with: .color(p.inscriptionEdge))
                let fillet = o.ink.opacity(0.3)
                ctx.fill(ruledLine(0, 2.2, w, 2.2, 0.9, seed: 13), with: .color(fillet))
                ctx.fill(ruledLine(0, h - 2.2, w, h - 2.2, 0.9, seed: 14), with: .color(fillet))
            }
            .allowsHitTesting(false)
        }
        .padding(.horizontal, -side)
        .accessibilityAddTraits(.isHeader)
    }
}

/**
 * The season and "Change date" (the web's `.home-day-meta`): side by side where the line has room
 * for both, else, and always while the picker is open, the control on a line of its own under the
 * season, which then stands `above` below the rule.
 */
private struct SeasonLine: Layout {
    let spacing: CGFloat
    let above: CGFloat
    let stacked: Bool

    /// Whether the two stand side by side in `width`, at their ideal widths.
    private func beside(_ width: CGFloat?, _ subviews: Subviews) -> Bool {
        guard !stacked, subviews.count == 2 else { return false }
        guard let width, width.isFinite else { return true }
        let ideal = subviews.map { $0.sizeThatFits(.unspecified).width }
        return ideal[0] + spacing + ideal[1] <= width
    }

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        if beside(proposal.width, subviews) {
            let sizes = subviews.map { $0.sizeThatFits(.unspecified) }
            return CGSize(width: sizes[0].width + spacing + sizes[1].width, height: max(sizes[0].height, sizes[1].height))
        }
        let free = ProposedViewSize(width: proposal.width, height: nil)
        let sizes = subviews.map { $0.sizeThatFits(free) }
        let width = sizes.map(\.width).max() ?? 0
        return CGSize(width: width, height: above + sizes.map(\.height).reduce(0, +))
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        if beside(bounds.width, subviews) {
            let sizes = subviews.map { $0.sizeThatFits(.unspecified) }
            var x = bounds.midX - (sizes[0].width + spacing + sizes[1].width) / 2
            for (subview, size) in zip(subviews, sizes) {
                subview.place(at: CGPoint(x: x, y: bounds.midY), anchor: .leading, proposal: ProposedViewSize(size))
                x += size.width + spacing
            }
            return
        }
        let free = ProposedViewSize(width: bounds.width, height: nil)
        var y = bounds.minY + above
        for subview in subviews {
            let size = subview.sizeThatFits(free)
            subview.place(at: CGPoint(x: bounds.midX, y: y), anchor: .top, proposal: ProposedViewSize(width: bounds.width, height: size.height))
            y += size.height
        }
    }
}

/**
 * The feast's name, and its familiar name in italic: beside it where the line has room, else on a
 * line of its own, never broken. A reader hears the two together.
 */
private struct FeastName: View {
    let name: String
    let alias: String
    let style: TextStyle
    /// Underlined in the gold line, as a link to the feast's page (the web's `.feast-link`).
    var linked = false
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    private var italic: TextStyle {
        var s = style
        s.italic = true
        return s
    }

    var body: some View {
        if alias.isEmpty {
            Text(name).underline(linked, color: p.goldLine).type(style).multilineTextAlignment(.center)
        } else {
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .firstTextBaseline, spacing: m.px(style.size * 0.25)) {
                    Text(name).underline(linked, color: p.goldLine).type(style)
                    Text(alias).underline(linked, color: p.goldLine).type(italic)
                }
                VStack(spacing: 0) {
                    Text(name).underline(linked, color: p.goldLine).type(style).multilineTextAlignment(.center)
                    Text(alias).underline(linked, color: p.goldLine).type(italic).fixedSize()
                }
            }
            .accessibilityElement(children: .combine)
        }
    }
}

/**
 * The invitation: a thin wash (`tablet.png`, feathered so no edge of it lies on a rule) in a frame
 * ruled by hand twice, the lining and inside it the day's colour (`frontal`) stopping short of
 * the corners; neither veils with the season, and a bare black day has no wash. Its words are the
 * tituli's red ochre, or by night the lining, lifted on a phone (`lifted`).
 */
private struct PrayNow: View {
    let label: String
    let tier: HomeTier
    let frontal: Color
    let wash: Color
    let lifted: Color?
    let action: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Button(action: action) {
            Text(label).type(tier.pray.tracked(tier.pray.size * 0.012))
                .foregroundStyle(lifted ?? (p.dark ? p.lining : p.titulus))
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity, minHeight: 44)
                .padding(.vertical, m.px(tier.prayPad))
                .padding(.horizontal, m.px(13.8))
                .background {
                    if let tablet = UIImage(named: "tablet.png") {
                        Image(uiImage: tablet).renderingMode(.template).resizable()
                            .foregroundStyle(wash)
                            .padding(1)
                    }
                }
                .overlay {
                    Canvas { ctx, size in invitationRules(ctx, size, lining: p.lining, frontal: frontal) }
                        .allowsHitTesting(false)
                }
        }
        .buttonStyle(Quiet())
    }
}

/**
 * The hours by period in horizontal bands, ruled as a fine book rules a table (the web's
 * `.home-hour-links`): a firmer rule above and below and finer ones between the periods, by hand,
 * in the lining, and none upright. The periods are the margin's index in the quiet ink, with no
 * cell of their own; gilt points part the hours in a band. The hour that is now lies under a pool
 * of light with no edge, its name rubricated (`now`) and its gloss under it.
 */
private struct HourDirectory: View {
    let current: String
    /// What the current hour is, set under its name for a newcomer.
    let note: String
    /// The hours prayed on this day.
    let prayed: Set<String>
    let tier: HomeTier
    let now: Color
    let home: HomeInk
    let open: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        let desk = tier.desk
        let label = TextStyle(size: 11.52, line: 18.43, tracking: 11.52 * 0.12, smallCaps: true)
        // One width for the three period labels, widened past the web's 83pt only when the
        // reader's font size needs it, so the hours still line up in columns.
        let font = label.uiFont(m)
        let widest = periods.map { ProseLayout.advance($0.label, font) + label.tracking * m.type * CGFloat($0.label.count) }.max() ?? 0
        let labelWidth = max(widest + m.px(8), m.px(desk ? 84 : 83))
        VStack(spacing: 0) {
            ForEach(Array(periods.enumerated()), id: \.offset) { i, row in
                HStack(spacing: 0) {
                    VStack(spacing: m.px(2)) {
                        PeriodIcon(period: row.period, color: o.flat).opacity(0.9 / 0.7)
                        Text(row.label).type(label).foregroundStyle(mix(p.accent, p.muted, 0.45)).lineLimit(1).fixedSize()
                    }
                    .padding(.vertical, m.px(6))
                    .padding(.leading, m.px(1.4))
                    .frame(width: labelWidth)
                    .frame(maxHeight: .infinity)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(row.label)
                    .accessibilityAddTraits(.isHeader)
                    Color.clear.frame(width: 1)
                    HStack(spacing: 0) {
                        ForEach(Array(row.hours.enumerated()), id: \.offset) { j, h in
                            let done = prayed.contains(h)
                            let isNow = h == current
                            let gloss = isNow ? note : ""
                            Button { open(h) } label: {
                                VStack(spacing: 0) {
                                    // Never broken mid-word: at the largest sizes a name steps down to fit its cell.
                                    Text(hourLabel(h))
                                        .type(TextStyle(size: tier.hour, line: tier.hour * 1.2, tracking: 0.31))
                                        .foregroundStyle(done ? p.muted : isNow ? now : p.text)
                                        .lineLimit(1)
                                        .minimumScaleFactor(0.7)
                                        // An hour prayed today withdraws to the muted ink, a small gilt cross
                                        // at its shoulder, as the web's `.is-prayed`; the cross takes no width.
                                        .overlay(alignment: .topTrailing) {
                                            if done {
                                                PaintedMark(.cross, size: m.px(7), color: p.gold)
                                                    .offset(x: m.px(9), y: m.px(2))
                                            }
                                        }
                                    // What the current hour is, for a newcomer, as the web's
                                    // `.home-hour-link-note`; one line, which may overhang its cell a little.
                                    if !gloss.isEmpty {
                                        Text(gloss)
                                            .type(TextStyle(size: 12.48, line: 14.35, italic: true))
                                            .foregroundStyle(home.nowGloss)
                                            .lineLimit(1)
                                            .fixedSize()
                                            .frame(width: 0)
                                    }
                                }
                                .padding(.horizontal, m.px(2))
                                .frame(maxWidth: .infinity, maxHeight: .infinity)
                                // The light rests on the hour that is now: a pool with no edge, at most 9.5rem across.
                                .background {
                                    if isNow {
                                        Canvas { ctx, size in
                                            let r = min(size.width + 8, 152) / 2
                                            let ry = (size.height + 4) / 2
                                            var pool = ctx
                                            pool.translateBy(x: size.width / 2, y: size.height / 2)
                                            pool.scaleBy(x: 1, y: ry / r)
                                            pool.fill(
                                                Path(ellipseIn: CGRect(x: -r, y: -r, width: 2 * r, height: 2 * r)),
                                                with: .radialGradient(Gradient(stops: [.init(color: home.nowPool, location: 0), .init(color: home.nowPool.opacity(0.55), location: 0.52), .init(color: home.nowPool.opacity(0), location: 1)]), center: .zero, startRadius: 0, endRadius: r)
                                            )
                                        }
                                        .padding(.horizontal, -8)
                                        .padding(.vertical, -4)
                                        .allowsHitTesting(false)
                                    }
                                }
                                // A gilt point before every hour but the first of its band.
                                .overlay(alignment: .leading) {
                                    if j > 0 {
                                        Rectangle().fill(o.flat.opacity(0.7)).frame(width: 3.5, height: 3.5)
                                            .rotationEffect(.degrees(45))
                                            .offset(x: -1.75)
                                            .accessibilityHidden(true)
                                    }
                                }
                            }
                            .buttonStyle(Quiet())
                            .accessibilityLabel([hourLabel(h), gloss.isEmpty ? nil : gloss, isNow ? "now" : nil, done ? "prayed" : nil].compactMap { $0 }.joined(separator: ", "))
                        }
                    }
                    .frame(minHeight: m.px(tier.row))
                }
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, 1)
                // The rule above each band: the frame's above the first, finer ones between.
                .overlay(alignment: .top) {
                    Canvas { ctx, size in
                        let rule = i == 0
                            ? ruledLine(0, 0.5, size.width, 0.5, 1.3, seed: 31)
                            : ruledLine(0, 0.5, size.width, 0.5, 0.9, seed: UInt32(41 + i - 1))
                        ctx.fill(rule, with: .color(p.lining.opacity(i == 0 ? 0.78 : 0.36)))
                    }
                    .frame(height: 2)
                    .allowsHitTesting(false)
                }
            }
        }
        .padding(.bottom, 1)
        .overlay(alignment: .bottom) {
            Canvas { ctx, size in
                ctx.fill(ruledLine(0, size.height - 0.5, size.width, size.height - 0.5, 1.3, seed: 32), with: .color(p.lining.opacity(0.78)))
            }
            .frame(height: 2)
            .allowsHitTesting(false)
        }
    }
}
