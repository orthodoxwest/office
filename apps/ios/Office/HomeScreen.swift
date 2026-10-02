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
                    // Home is at least a screen tall, its colophon at the foot.
                    VStack(spacing: 0) {
                        SiteHeader()
                        if let niche {
                            // Centred between the header and the foot, as the web's desktop home.
                            Spacer(minLength: 0)
                            // The moulding stands 0.75rem out from the card; room for it below the header.
                            Frontispiece(view: view, date: date, niche: niche, head: nicheHead(screen))
                                .background(GeometryReader { card in
                                    Color.clear.preference(key: NicheFrameKey.self, value: card.frame(in: .global))
                                })
                                .frame(maxWidth: nicheWidth(screen))
                                .padding(.horizontal, 24)
                                .padding(.top, 40)
                                .padding(.bottom, 12)
                            Spacer(minLength: 0)
                            Footer()
                        } else {
                            Frontispiece(view: view, date: date, niche: nil, head: m.px(screen < 375 ? 56 : 68))
                                .frame(maxWidth: m.px(576))
                                .padding(.horizontal, m.px(gutter))
                                .padding(.top, m.px(13.6))
                            Spacer(minLength: 0)
                            // The phone's home fits its screen with nothing to spare: the head is
                            // paid for in the footer's gap and padding.
                            Footer(gap: 29.6, bottom: 25.6)
                        }
                    }
                    .frame(minHeight: geo.size.height)
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

/// The inscription band's bounds, where the niche's painted lining ends.
private struct InscriptionKey: PreferenceKey {
    static let defaultValue: Anchor<CGRect>? = nil
    static func reduce(value: inout Anchor<CGRect>?, nextValue: () -> Anchor<CGRect>?) { value = nextValue() ?? value }
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
    let niche: NicheTokens?
    let head: CGFloat
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
        let desk = niche != nil
        let side = m.px(desk ? 28 : 16)
        // On a phone the lining stands this far inside the panel's edge, and the cross below it.
        let panelInset = m.px(9.6)
        let crown = panelInset + m.px(13.6)
        // Room under the head for the crown's cross and the lining's arch: on a phone the
        // lining's inset, air, the cross, and its clearance before the date.
        let top = desk ? head * 0.62 + m.px(33.6) : crown + m.px(30.4 + 20)
        VStack(spacing: 0) {
            Button { model.open(.ordo(year: Int(date.year), month: Int(date.month), day: Int(date.day))) } label: {
                Text(view.dateLabel)
                    .type(desk ? Scale.body.sized(25.92, line: 31.1).tracked(0.39) : Scale.body.sized(22.08, line: 26.5).tracked(0.22))
                    .foregroundStyle(p.text)
                    .multilineTextAlignment(.center)
            }
            .buttonStyle(Quiet())
            .accessibilityAddTraits(.isHeader)
            .accessibilityHint("Opens the ordo")
            Text(view.feast).type(desk ? Scale.body.sized(18.72, line: 23.4) : Scale.body.sized(17.28, line: 21.6))
                .foregroundStyle(p.accent)
                .multilineTextAlignment(.center)
                .padding(.top, m.px(2))
            if !view.octaveNote.isEmpty {
                Text(view.octaveNote).type(Scale.small).foregroundStyle(p.muted).multilineTextAlignment(.center)
            }
            if !view.isToday {
                Button { model.open(.home(model.today)) } label: {
                    Text("GO TO TODAY").type(Scale.menu).foregroundStyle(p.accent)
                        .goldUnderline(true, p.goldLine)
                        .padding(.vertical, m.px(12))
                }
                .buttonStyle(Quiet())
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
                    ForEach(view.commemorations, id: \.self) { c in
                        Text(c).type(TextStyle(size: 14, line: 20)).foregroundStyle(p.text).multilineTextAlignment(.center)
                    }
                }
                .padding(.top, m.px(8))
            }
            inscription(side: side, desk: desk)
                .anchorPreference(key: InscriptionKey.self, value: .bounds) { $0 }
                .padding(.top, m.px(9.6))
            PrayNow(label: view.prayNowLabel, desk: desk) {
                model.open(.hour(view.prayNowDate, view.prayNowHour))
            }
            .padding(.top, m.px(12))
            HourDirectory(current: view.currentHour, desk: desk) { h in model.open(.hour(date, h)) }
                .padding(.top, m.px(11.2))
            // Season and date control share one line after the invitation.
            Hairline(color: ink.rule).padding(.top, m.px(8.8))
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
        .padding(.top, top)
        .padding(.bottom, m.px(desk ? 20 : 12))
        .frame(maxWidth: .infinity)
        .backgroundPreferenceValue(InscriptionKey.self, alignment: .topLeading) { band in
            GeometryReader { g in
                // A border painted round the head on the back wall, down to the inscription band:
                // on a phone its curve springs 8pt below the head's; on a wide screen it stands
                // 26pt inside the moulding.
                if let band {
                    if desk {
                        NicheLining(inset: 2 + 26, ry: head - 26, bottom: g[band].minY, hairline: liningDay)
                    } else {
                        NicheLining(inset: panelInset, ry: head - panelInset + m.px(8), bottom: g[band].minY, hairline: liningDay)
                    }
                }
            }
        }
        .background {
            if let niche {
                // The niche: a low round head, the stone moulding, the day's colour as its trim.
                Niche(t: niche, day: day, head: head, frame: ink.frame)
            } else {
                // The panel: a segmental head, the day's colour as a ring at its edge.
                Panel(day: day, frame: ink.frame, head: head)
            }
        }
        .overlay(alignment: .top) {
            // The consecration cross at the crown of the head, with clear air round it.
            PaintedMark(.consecration, size: m.px(desk ? 36 : 30.4), color: p.lining).padding(.top, desk ? head * 0.36 - 2 : crown)
        }
    }

    /// The inscription band: pale gilt letters on the frieze's green earth, between oxblood rules
    /// each with a gilt fillet inside it, run through to the frame.
    private func inscription(side: CGFloat, desk: Bool) -> some View {
        // Gilt lozenges either side, 5pt squares on their points.
        HStack(spacing: m.px(12)) {
            Diamond(size: 7.07, color: o.ink)
            Text("Pray the hours").type(TextStyle(size: 12.8, line: 20.48, tracking: 12.8 * (desk ? 0.17 : 0.16), smallCaps: true)).foregroundStyle(o.ink)
            Diamond(size: 7.07, color: o.ink)
        }
        .padding(.vertical, m.px(3.2))
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
 * The invitation: a painted line of the lining's terracotta with a thinner one 3pt inside it, as a
 * panel's border is ruled twice; neither veils with the season. Its words are the tituli's red
 * ochre, or by night the lining.
 */
private struct PrayNow: View {
    let label: String
    let desk: Bool
    let action: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Button(action: action) {
            Text(label).type((desk ? Scale.body.sized(20, line: 26) : Scale.body.sized(19.2, line: 24.96)).tracked(0.38))
                .foregroundStyle(p.dark ? p.lining : p.titulus)
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity)
                .padding(.vertical, m.px(desk ? 12 : 11.9))
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
    let desk: Bool
    let open: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
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
                            Button { open(h) } label: {
                                // Never broken mid-word: at the largest sizes a name steps down to fit its cell.
                                Text(hourLabel(h))
                                    .type(TextStyle(size: desk ? 16 : 15.68, line: 18.8, tracking: 0.31))
                                    .foregroundStyle(h == current ? p.accent : p.text)
                                    .lineLimit(1)
                                    .minimumScaleFactor(0.7)
                                    .goldUnderline(h == current, p.goldLine)
                                    .padding(.horizontal, m.px(2))
                                    .frame(maxWidth: .infinity, maxHeight: .infinity)
                            }
                            .buttonStyle(Quiet())
                            .accessibilityLabel(h == current ? "\(hourLabel(h)), now" : hourLabel(h))
                        }
                    }
                    .frame(minHeight: m.px(desk ? 46 : 44))
                }
                .fixedSize(horizontal: false, vertical: true)
            }
        }
        .overlay(Rectangle().strokeBorder(ink.panelRule, lineWidth: 1).allowsHitTesting(false))
    }
}
