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
                // Apse: one fixed field, anchored top centre, clearing the header.
                VaultField(fade: [(0, 0), (0.09, 0), (0.16, 0.9), (0.6, 0.7), (1, 0.3)]).ignoresSafeArea()
                // A wide screen sets the frontispiece in a niche, and lights the room toward it.
                if let niche { ChapelLight(t: niche, niche: nicheFrame).ignoresSafeArea() }
                ScrollViewReader { scroll in
                ScrollView {
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
                        } else {
                            Frontispiece(view: view, date: date, niche: nil, head: 0)
                                .frame(maxWidth: m.px(576))
                                .padding(.horizontal, m.px(gutter))
                                .padding(.top, m.px(16))
                        }
                        Footer(diamond: false)
                    }
                    .frame(minHeight: niche == nil ? nil : geo.size.height)
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
        let day = dayColor(view.color)
        let desk = niche != nil
        let side = m.px(desk ? 28 : 16)
        VStack(spacing: 0) {
            // The consecration cross, the mark that ends every hour, opens the card; a niche
            // carries it at its crown instead.
            if !desk {
                PaintedMark(.consecration, size: m.px(30.4), color: p.lining)
            }
            Button { model.open(.ordo(year: Int(date.year), month: Int(date.month), day: Int(date.day))) } label: {
                Text(view.dateLabel)
                    .type(desk ? Scale.body.sized(25.92, line: 31.1).tracked(0.39) : Scale.body.sized(22.08, line: 26.5).tracked(0.22))
                    .foregroundStyle(p.text)
                    .multilineTextAlignment(.center)
            }
            .buttonStyle(Quiet())
            .accessibilityAddTraits(.isHeader)
            .accessibilityHint("Opens the ordo")
            .padding(.top, desk ? 0 : m.px(8.8))
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
                .padding(.top, m.px(8.8 + 2.4))
            PrayNow(label: view.prayNowLabel, desk: desk) {
                model.open(.hour(view.prayNowDate, view.prayNowHour))
            }
            .padding(.top, m.px(13.6))
            HourDirectory(current: view.currentHour, desk: desk) { h in model.open(.hour(date, h)) }
                .padding(.top, m.px(12.8))
            // Season and date control share one line after the invitation.
            Hairline(color: p.border).padding(.top, m.px(11.2))
            if !view.season.isEmpty {
                Text(view.season).type(Scale.small).foregroundStyle(p.muted).padding(.top, m.px(4.8))
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
        // A niche leaves room under its head for the crown's cross and the lining's arch.
        .padding(.top, desk ? head * 0.62 + m.px(33.6) : m.px(18.2))
        .padding(.bottom, m.px(desk ? 20 : 16))
        .frame(maxWidth: .infinity)
        .backgroundPreferenceValue(InscriptionKey.self, alignment: .topLeading) { band in
            GeometryReader { g in
                // A border painted on the niche's back wall, round its head, down to the inscription band.
                if desk, let band { NicheLining(head: head, bottom: g[band].minY) }
            }
        }
        .background {
            if let niche {
                // The niche: a low round head, the stone moulding, the day's colour as its trim.
                let frame = p.dark ? Color(rgb: 208, 176, 106, 0.34) : Color(rgb: 87, 52, 33, 0.3)
                Niche(t: niche, day: day, head: head, frame: frame)
            } else {
                ZStack(alignment: .top) {
                    p.surface
                    // The book-cover tooling, 0.35rem inside the frame.
                    Rectangle().strokeBorder(o.flat.opacity(0.18), lineWidth: 1)
                        .padding(EdgeInsets(top: 3 + m.px(5.6), leading: 1 + m.px(5.6), bottom: 1 + m.px(5.6), trailing: 1 + m.px(5.6)))
                    // The day's colour as the frame's top edge, like a vestment's trim.
                    Rectangle().fill(day).frame(height: 3)
                }
                .overlay(Rectangle().stroke(p.border, lineWidth: 1))
            }
        }
        .overlay(alignment: .top) {
            // The consecration cross at the niche's crown, as heavy as the one that ends each hour.
            if desk {
                PaintedMark(.consecration, size: m.px(36), color: p.lining).padding(.top, head * 0.36 - 2)
            }
        }
    }

    /// The inscription band: pale gilt letters on the frieze's green earth, between oxblood rules, run through to the frame.
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
        .overlay(alignment: .top) { Rectangle().fill(p.inscriptionEdge).frame(height: 1) }
        .overlay(alignment: .bottom) { Rectangle().fill(p.inscriptionEdge).frame(height: 1) }
        .padding(.horizontal, -side)
        .accessibilityAddTraits(.isHeader)
    }
}

/**
 * The invitation: a single painted line of the lining's terracotta, which does not veil with the
 * season. On the Apse its words are ivory, so the frame carries the colour.
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
                .foregroundStyle(p.dark ? p.text : p.accent)
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity)
                .padding(.vertical, m.px(desk ? 12 : 11.9))
                .padding(.horizontal, m.px(13.8))
                .overlay {
                    Rectangle().strokeBorder(p.lining, lineWidth: 1).allowsHitTesting(false)
                }
        }
        .buttonStyle(Quiet())
    }
}

/// The hours by period in horizontal bands, the current one underlined in gold.
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
        VStack(spacing: 0) {
            ForEach(Array(periods.enumerated()), id: \.offset) { i, row in
                if i > 0 { Hairline(color: p.border) }
                HStack(spacing: 0) {
                    VStack(spacing: m.px(2)) {
                        PeriodIcon(period: row.period, color: o.flat)
                        Text(row.label).type(label).foregroundStyle(desk ? p.accent : p.muted).lineLimit(1).fixedSize()
                    }
                    .padding(.vertical, m.px(6))
                    .frame(width: labelWidth)
                    .frame(maxHeight: .infinity)
                    .background(p.inscriptionWash)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel(row.label)
                    .accessibilityAddTraits(.isHeader)
                    Rectangle().fill(p.border).frame(width: 1)
                    HStack(spacing: 0) {
                        ForEach(Array(row.hours.enumerated()), id: \.offset) { j, h in
                            if j > 0 { VRule(color: p.border) }
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
        .overlay(Rectangle().stroke(p.border, lineWidth: 1))
    }
}
