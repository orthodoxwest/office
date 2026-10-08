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
        let o = Ornament.of(p, season: view.ornament)
        GeometryReader { geo in
            let screen = geo.size.width
            let niche = wide ? NicheTokens.of(p) : nil
            ZStack(alignment: .top) {
                // The wall's one field, fixed to the screen and phased from its top: by night the
                // vault, clearing the header and thinning toward the foot; by day the powdering,
                // cut square under the beam.
                let top = geo.safeAreaInsets.top
                WallField(seam: top) { h, dark in
                    func at(_ d: CGFloat) -> CGFloat { (top + d) / h }
                    return dark
                        ? [(0, 0), (at(56), 0), (at(112), 1), (max(0.78, at(112)), 1), (1, 0.6)]
                        : [(0, 0), (at(64), 0), (at(64), 1), (1, 1)]
                }
                .ignoresSafeArea()
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
    var dateMeasure: CGFloat? { desk ? nil : min(card * pick(0, 0.233, 0.3) + 139, clear(headPad + 44 - date.line * dyn)) }
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
    /// The invitation's note needs height a short phone lacks, as on the web.
    var prayNote: Bool { !short }
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
 * centred in it or set two parts to three.
 */
private struct FrontispieceLayout: Layout {
    let tier: HomeTier
    let m: Metrics

    /// The head's height, the day's parts (its facts, the versicle when it fits, and "Go to
    /// today"), and the whole. The versicle takes only the height the head has to spare: where it
    /// would make the card taller, the head goes without it.
    private func measure(_ width: CGFloat, _ proposal: ProposedViewSize, _ subviews: Subviews) -> (summary: CGFloat, facts: CGFloat, versicle: CGFloat?, back: CGFloat, height: CGFloat) {
        let free = ProposedViewSize(width: width, height: nil)
        let facts = subviews[2].sizeThatFits(free).height
        let versicle = subviews[3].sizeThatFits(free).height
        let back = subviews[4].sizeThatFits(free).height
        let after = subviews[5].sizeThatFits(free).height
        let foot = m.px(tier.bottom)
        let day = facts + back + m.px(tier.dayGap)
        let least = proposal.height.map { $0.isFinite ? $0 : 0 } ?? 0
        let summary = max(m.px(tier.headPad) + day, tier.arch.rise * width, least - after - foot)
        let fits = versicle > 0 && m.px(tier.headPad) + day + versicle <= summary
        return (summary, facts, fits ? versicle : nil, back, summary + after + foot)
    }

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let width = proposal.width ?? 390
        return CGSize(width: width, height: measure(width, proposal, subviews).height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let (summary, facts, versicle, back, _) = measure(bounds.width, proposal, subviews)
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
    }
}

/**
 * The frontispiece's painted furniture (`.home-hero`), the same at every width: its frame, the
 * rules within, the period cells' wash (the frieze's green earth, thinned), and the panel's own
 * rules, the lining thinned.
 */
private struct FrontispieceInk {
    let frame: Color
    let rule: Color
    let band: Color
    let panelRule: Color

    static func of(_ p: Palette) -> FrontispieceInk {
        p.dark
            ? FrontispieceInk(frame: Color(rgb: 208, 176, 106, 0.34), rule: Color(rgb: 208, 176, 106, 0.24), band: Color(rgb: 208, 176, 106, 0.045), panelRule: p.lining.opacity(0.45))
            : FrontispieceInk(frame: Color(rgb: 87, 52, 33, 0.3), rule: Color(rgb: 107, 58, 31, 0.22), band: p.inscriptionGround.opacity(0.09), panelRule: p.lining.opacity(0.45))
    }
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
    @Environment(\.reveal) private var reveal

    var body: some View {
        let day = dayColor(view.color, p)
        // The lining's inner line is the day's colour, beside the cross on the plaster; a white
        // day's would be tan there, and takes the gold line by day.
        let liningDay = !p.dark && view.color == "white" ? p.goldLine : day
        let ink = FrontispieceInk.of(p)
        let desk = tier.desk
        let side = m.px(tier.side)
        FrontispieceLayout(tier: tier, m: m) {
            // A lining painted round the head on the back wall, down to the inscription band:
            // on a phone 0.75rem inside the panel's edge, in a niche 26pt inside the moulding.
            Lining(arch: tier.arch, inset: m.px(desk ? 26 : 12), hairline: liningDay)
            // The consecration cross in the point of the head, with clear air round it.
            PaintedMark(.consecration, size: m.px(tier.crownSize), color: p.lining)
            // The day.
            VStack(spacing: 0) {
                Button { model.open(.ordo(year: Int(date.year), month: Int(date.month), day: Int(date.day))) } label: {
                    Text(unbroken(view.dateLabel))
                        .type(tier.date)
                        .foregroundStyle(p.text)
                        .multilineTextAlignment(.center)
                        // On a phone a full thumb's height, as the web's, its line at the foot so
                        // the feast stands close under it.
                        .frame(maxWidth: tier.dateMeasure.map { m.px($0) }, minHeight: tier.desk ? nil : m.px(44), alignment: .bottom)
                }
                .buttonStyle(Quiet())
                .accessibilityAddTraits(.isHeader)
                .accessibilityHint("Opens the ordo")
                FeastName(name: view.feastName, alias: view.feastAlias, style: tier.feast)
                    .foregroundStyle(p.accent)
                    .frame(maxWidth: tier.feastMeasure.map { m.px($0) })
                    .padding(.top, m.px(2))
                if !view.octaveNote.isEmpty {
                    Text(view.octaveNote).type(Scale.small).foregroundStyle(p.muted).multilineTextAlignment(.center)
                }
                if !view.penitential.isEmpty {
                    HStack(spacing: m.px(9.6)) {
                        ForEach(view.penitential, id: \.self) { t in
                            Text(t).type(TextStyle(size: 12.48, line: 19.97, tracking: 0.75, smallCaps: true)).foregroundStyle(p.rubric)
                        }
                    }
                    .padding(.top, m.px(12))
                }
                if !view.commemorations.isEmpty {
                    VStack(spacing: 0) {
                        Text("ALSO").type(.label(10.56, 0.1)).foregroundStyle(p.muted)
                        // A second commemoration is joined to the first with "and", as the web's
                        // `.commemorations li + li::before`, so the list reads as a sentence.
                        ForEach(Array(view.commemorations.enumerated()), id: \.offset) { i, c in
                            (i == 0 ? Text(c) : Text("and ").font(Font(garamond(tier.commemoration.size * m.type, italic: true))) + Text(c))
                                .type(tier.commemoration).foregroundStyle(p.text).multilineTextAlignment(.center)
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
                        versicleLine("℣.", view.versicle, style)
                        versicleLine("℟.", view.response, style)
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
                PrayNow(label: view.prayNowLabel, note: tier.prayNote ? view.prayNowNote : "", tier: tier) {
                    model.open(.hour(view.prayNowDate, view.prayNowHour))
                }
                .padding(.top, m.px(tier.bandGap))
                HourDirectory(current: view.currentHour, prayed: model.hoursPrayed(on: date), tier: tier) { h in model.open(.hour(date, h)) }
                    .padding(.top, m.px(tier.prayGap))
                // Season and date control share one line after the invitation.
                Hairline(color: ink.rule).padding(.top, m.px(tier.metaGap))
                if !view.season.isEmpty {
                    Text(view.season).type(Scale.small).foregroundStyle(p.muted).padding(.top, m.px(3.2))
                }
                Disclosure(label: "Change date", open: picking) {
                    withAnimation(unfolding) { picking.toggle() }
                    if picking { reveal("disclosed") }
                }
                if picking {
                    DayPicker(shown: date, today: model.today) { d in
                        picking = false
                        model.open(.home(d))
                    }
                    .id("disclosed")
                    .transition(.unfold)
                }
            }
            .padding(.horizontal, side)
            .frame(maxWidth: .infinity)
        }
        .background {
            if let niche {
                // The niche: a pointed head, the stone moulding, the day's colour as its trim.
                Niche(arch: tier.arch, t: niche, day: day, frame: ink.frame)
            } else {
                // The panel: the day's colour a little toward the frame as a ring at its edge, so
                // a red or green day edges the head without outshouting the cross.
                Panel(arch: tier.arch, ring: mix(day, ink.frame, 0.7), frame: ink.frame)
            }
        }
    }

    /// A line of the versicle: its ℣ or ℟ upright in the rubrics' red, the words in italic, the
    /// text's ink a little withdrawn.
    private func versicleLine(_ sigil: String, _ words: String, _ style: TextStyle) -> some View {
        (Text(sigil).font(Font(garamond(style.size * m.type))).foregroundColor(p.rubric) + Text(" " + words))
            .type(style)
            .foregroundStyle(p.text.opacity(0.84))
            .multilineTextAlignment(.center)
            .fixedSize(horizontal: false, vertical: true)
    }

    /// The inscription band: pale gilt letters on the frieze's green earth, between oxblood rules
    /// each with a gilt fillet inside it, run through to the frame.
    private func inscription(side: CGFloat) -> some View {
        // Gilt lozenges either side, 5pt squares on their points.
        HStack(spacing: m.px(12)) {
            Diamond(size: 7.07, color: o.ink)
            Text("Pray the hours").type(TextStyle(size: tier.band, line: tier.band * 1.6, tracking: tier.band * (tier.desk ? 0.17 : 0.16), smallCaps: true)).foregroundStyle(o.ink)
            Diamond(size: 7.07, color: o.ink)
        }
        .padding(.vertical, m.px(tier.bandPad))
        .frame(maxWidth: .infinity)
        .background(p.inscriptionGround)
        .overlay(alignment: .top) {
            VStack(spacing: 0) {
                Rectangle().fill(p.inscriptionEdge).frame(height: 1)
                Rectangle().fill(o.ink.opacity(0.22)).frame(height: 1)
            }
        }
        .overlay(alignment: .bottom) {
            VStack(spacing: 0) {
                Rectangle().fill(o.ink.opacity(0.22)).frame(height: 1)
                Rectangle().fill(p.inscriptionEdge).frame(height: 1)
            }
        }
        .padding(.horizontal, -side)
        .accessibilityAddTraits(.isHeader)
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
    @Environment(\.metrics) private var m

    private var italic: TextStyle {
        var s = style
        s.italic = true
        return s
    }

    var body: some View {
        if alias.isEmpty {
            Text(name).type(style).multilineTextAlignment(.center)
        } else {
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .firstTextBaseline, spacing: m.px(style.size * 0.25)) {
                    Text(name).type(style)
                    Text(alias).type(italic)
                }
                VStack(spacing: 0) {
                    Text(name).type(style).multilineTextAlignment(.center)
                    Text(alias).type(italic).fixedSize()
                }
            }
            .accessibilityElement(children: .combine)
        }
    }
}

/**
 * The invitation: a painted line of the lining's terracotta with a thinner one 3pt inside it, as a
 * panel's border is ruled twice; neither veils with the season. Its words are the tituli's red
 * ochre, or by night the lining.
 */
private struct PrayNow: View {
    let label: String
    let note: String
    let tier: HomeTier
    let action: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let size = tier.pray.size
        Button(action: action) {
            VStack(spacing: 0) {
                Text(label).type(tier.pray)
                    .foregroundStyle(p.dark ? p.lining : p.titulus)
                // What the hour is, so a newcomer knows what they are opening.
                if !note.isEmpty {
                    let noteSize = max(12.8, size * 0.58)
                    Text(note).type(TextStyle(size: noteSize, line: noteSize * 1.25, italic: true))
                        .foregroundStyle(p.muted)
                        .padding(.top, m.px(size * 0.1))
                }
            }
            .multilineTextAlignment(.center)
            .frame(maxWidth: .infinity)
            .padding(.top, m.px(tier.prayPad))
            // The note takes some of the box's lower air rather than a line of the panel's height.
            .padding(.bottom, m.px(note.isEmpty ? tier.prayPad : max(tier.prayPad - size * 0.3, 2)))
            .padding(.horizontal, m.px(13.8))
            .overlay {
                ZStack {
                    Rectangle().strokeBorder(p.lining, lineWidth: 1)
                    Rectangle().strokeBorder(FrontispieceInk.of(p).panelRule, lineWidth: 1).padding(4)
                }
                .allowsHitTesting(false)
            }
        }
        .buttonStyle(Quiet())
    }
}

/**
 * The hours by period in horizontal bands, the current one underlined in gold: framed in the
 * lining thinned, ruled within in the frontispiece's ink, the period cells in its wash.
 */
private struct HourDirectory: View {
    let current: String
    /// The hours prayed on this day.
    let prayed: Set<String>
    let tier: HomeTier
    let open: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        let desk = tier.desk
        // The desktop's labels are in the accent, their column 5.25rem.
        let label = TextStyle(size: 11.52, line: 18.43, tracking: 11.52 * 0.08, smallCaps: true)
        // One width for the three period labels, widened past the web's 83pt only when the
        // reader's font size needs it, so the hours still line up in columns.
        let font = label.uiFont(m)
        let widest = periods.map { ProseLayout.advance($0.label, font) + label.tracking * m.type * CGFloat($0.label.count) }.max() ?? 0
        let labelWidth = max(widest + m.px(8), m.px(desk ? 84 : 83))
        let ink = FrontispieceInk.of(p)
        VStack(spacing: 0) {
            ForEach(Array(periods.enumerated()), id: \.offset) { i, row in
                if i > 0 { Hairline(color: ink.rule) }
                HStack(spacing: 0) {
                    VStack(spacing: m.px(2)) {
                        PeriodIcon(period: row.period, color: o.flat)
                        Text(row.label).type(label).foregroundStyle(desk ? p.accent : p.muted).lineLimit(1).fixedSize()
                    }
                    .padding(.vertical, m.px(6))
                    .frame(width: labelWidth)
                    .frame(maxHeight: .infinity)
                    .background(ink.band)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(row.label)
                    .accessibilityAddTraits(.isHeader)
                    Rectangle().fill(ink.rule).frame(width: 1)
                    HStack(spacing: 0) {
                        ForEach(Array(row.hours.enumerated()), id: \.offset) { j, h in
                            if j > 0 { VRule(color: ink.rule, height: 16) }
                            let done = prayed.contains(h)
                            Button { open(h) } label: {
                                // Never broken mid-word: at the largest sizes a name steps down to fit its cell.
                                Text(hourLabel(h))
                                    .type(TextStyle(size: tier.hour, line: tier.hour * 1.2, tracking: 0.31))
                                    .foregroundStyle(done ? p.muted : h == current ? p.accent : p.text)
                                    .lineLimit(1)
                                    .minimumScaleFactor(0.7)
                                    .goldUnderline(h == current, p.goldLine)
                                    .padding(.horizontal, m.px(2))
                                    // An hour prayed today withdraws to the muted ink, a small gilt cross
                                    // at its shoulder, as the web's `.is-prayed`; the cross takes no width.
                                    .overlay(alignment: .topTrailing) {
                                        if done {
                                            PaintedMark(.cross, size: m.px(7), color: p.gold)
                                                .offset(x: m.px(9), y: m.px(2))
                                        }
                                    }
                                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                            }
                            .buttonStyle(Quiet())
                            .accessibilityLabel([hourLabel(h), h == current ? "now" : nil, done ? "prayed" : nil].compactMap { $0 }.joined(separator: ", "))
                        }
                    }
                    .frame(minHeight: m.px(tier.row))
                }
                .fixedSize(horizontal: false, vertical: true)
            }
        }
        .overlay(Rectangle().strokeBorder(ink.panelRule, lineWidth: 1).allowsHitTesting(false))
    }
}
