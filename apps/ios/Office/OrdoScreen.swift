import SwiftUI

/// One month of the ordo, brought to `day` when one is asked for (the web's #d-date).
struct OrdoPage: View {
    let year: Int
    let month: Int
    let day: Int
    @EnvironmentObject private var model: AppModel

    var body: some View {
        let (y, mo) = (year, month)
        Loaded(key: "\(y)-\(mo)", motion: model.motion(to: .ordo(year: y, month: mo, day: day))) {
            try Office.core.get().ordoMonth(year: Int32(y), month: Int32(mo))
        } content: { view in
            OrdoScreen(month: view, focusDay: day)
        }
        .background(PlasterWall())
        .toolbar(.hidden, for: .navigationBar)
    }
}

/// A year's frontispiece: arithmetic, drawn at once.
struct YearPage: View {
    let year: Int
    @EnvironmentObject private var model: AppModel
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        // Drawn at once, so another year moves in as soon as it is asked for.
        ZStack {
            OrdoYearScreen(view: ordoYear(year: Int32(year)))
                .id(year)
                .transition(inPlace(model.motion(to: .year(year)), reduceMotion: reduceMotion))
        }
        .animation(.easeInOut(duration: 0.3), value: year)
        .background(PlasterWall())
        .toolbar(.hidden, for: .navigationBar)
    }
}

extension View {
    /// The ordo's column: the prayer measure on a phone, the web's 56rem calendar on a wide screen.
    func ordoColumn(_ m: Metrics, wide: Bool) -> some View {
        Group {
            if wide {
                frame(maxWidth: 896).padding(.horizontal, 24).frame(maxWidth: .infinity)
            } else {
                measured(m)
            }
        }
    }
}

/// One month of the ordo, as the web's month page: year and month navigation, then the days.
struct OrdoScreen: View {
    let month: OrdoMonthView
    let focusDay: Int
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide
    @State private var allDetails = false
    @State private var focused = false

    var body: some View {
        let year = Int(month.year), number = Int(month.month)
        ScrollViewReader { scroll in
            ScrollView {
                LazyVStack(spacing: 0) {
                    SiteHeader().id("top")
                    OrdoHeader(year: year, month: number, roman: nil).ordoColumn(m, wide: wide).padding(.top, m.px(17.6))
                    tools.ordoColumn(m, wide: wide).padding(.top, m.px(8))
                    MonthHeading(name: monthName(number), isTodaysMonth: Int(model.today.year) == year && Int(model.today.month) == number) {
                        withAnimation { scroll.scrollTo("top", anchor: .top) }
                    }
                    .ordoColumn(m, wide: wide)
                    .padding(.top, m.px(10))
                    if wide { DayColumns().ordoColumn(m, wide: wide) }
                    ForEach(month.days, id: \.date.day) { d in
                        DayRow(d: d, isToday: d.date == model.today, allDetails: allDetails)
                            .ordoColumn(m, wide: wide)
                            .id("d-\(d.date.day)")
                    }
                    let prev = number == 1 ? (year - 1, 12) : (year, number - 1)
                    let next = number == 12 ? (year + 1, 1) : (year, number + 1)
                    Continuation(
                        previousLabel: "Previous month",
                        previous: monthName(prev.1),
                        onPrevious: { model.open(.ordo(year: prev.0, month: prev.1, day: 0)) },
                        middle: "\(year) Ordo",
                        onMiddle: { model.open(.year(year)) },
                        nextLabel: "Next month",
                        next: monthName(next.1),
                        onNext: { model.open(.ordo(year: next.0, month: next.1, day: 0)) }
                    )
                    .ordoColumn(m, wide: wide)
                    .padding(.top, m.px(44))
                    Footer()
                }
            }
            .onAppear {
                // A day asked for is brought into view once; after that the reader's own place stands.
                guard !focused else { return }
                focused = true
                if focusDay > 0 {
                    DispatchQueue.main.async { scroll.scrollTo("d-\(focusDay)", anchor: .top) }
                }
            }
        }
    }

    private var tools: some View {
        let small = Scale.small.sized(12, line: 17.4)
        return HStack(spacing: 0) {
            HStack(spacing: 0) {
                Text("§ Fasting · ").type(small).foregroundStyle(p.muted)
                FishIcon(color: p.muted).padding(.trailing, m.px(3))
                Text("Abstinence").type(small).foregroundStyle(p.muted)
            }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel("Section mark: fasting. Fish: abstinence.")
            Spacer()
            Button { withAnimation(unfolding) { allDetails.toggle() } } label: {
                Text(allDetails ? "Hide office details" : "Show office details").type(Scale.small).foregroundStyle(p.accent).underline()
                    .padding(.vertical, m.px(12))
            }
            .buttonStyle(Quiet())
        }
        .frame(minHeight: 44)
    }
}

/**
 * The ordo's title, year navigation and month strip. `month` is the month shown, or nil on the
 * year's frontispiece, which takes "Anno Domini" and `roman` for its subtitle. A wide screen
 * sets the title beside the year's navigation, and the twelve months on one line.
 */
private struct OrdoHeader: View {
    let year: Int
    let month: Int?
    let roman: String?
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide

    var body: some View {
        if wide {
            // The web's desktop header: the title beside the year's navigation, then the months on one line.
            VStack(spacing: 0) {
                HStack {
                    title
                    Spacer(minLength: 32)
                    yearNav
                }
                .padding(.top, m.px(10.4))
                .padding(.bottom, m.px(16))
                .background(rules)
                strip.overlay(alignment: .bottom) { Rectangle().fill(p.border).frame(height: 1) }
            }
        } else {
            VStack(spacing: 0) {
                title
                yearNav.padding(.top, m.px(8))
                Hairline(color: p.border).padding(.top, m.px(8))
                strip
            }
            .padding(.top, m.px(10.4))
            .padding(.bottom, m.px(16))
            .background(rules)
        }
    }

    /// The frontispiece's hairline below; it opens as the hours do, with its headpiece alone and no rule above.
    private var rules: some View {
        VStack(spacing: 0) {
            Spacer()
            Rectangle().fill(p.border).frame(height: 1)
        }
    }

    private var title: some View {
        VStack(spacing: 0) {
            Headpiece()
            Text("\(String(year)) Ordo").type(TextStyle(size: wide ? 34.4 : 28, line: wide ? 39.6 : 34, lining: true)).foregroundStyle(p.text)
                .padding(.top, m.px(2))
                .accessibilityAddTraits(.isHeader)
            // The frontispiece reads as the printed ordo's title page.
            let subtitle = month == nil && !(roman ?? "").isEmpty ? "Anno Domini \(roman ?? "")" : "Feasts & daily observances"
            Text(subtitle).type(wide ? Scale.small.sized(13.6) : Scale.small).foregroundStyle(p.muted).padding(.top, m.px(4.8))
        }
    }

    /// Previous and next year keep the page's kind: a month's, or the frontispiece.
    private func sameView(_ y: Int) {
        if let month { model.open(.ordo(year: y, month: month, day: 0)) } else { model.open(.year(y)) }
    }

    private var yearNav: some View {
        let nav = TextStyle(size: 12.8, line: 20.5, lining: true)
        let t = model.today
        return HStack(spacing: 0) {
            Button { sameView(year - 1) } label: { cell("‹ \(String(year - 1))", nav) }
                .buttonStyle(Quiet()).accessibilityLabel("Previous year, \(String(year - 1))")
            VRule(color: p.border)
            Button { model.open(.ordo(year: Int(t.year), month: Int(t.month), day: Int(t.day))) } label: {
                // The rule spans the cell, less its inset, as the web's current-year mark.
                Text("Today").type(nav).foregroundStyle(p.accent)
                    .padding(m.px(12))
                    .frame(maxWidth: wide ? nil : .infinity)
                    .goldUnderline(Int(t.year) == year, p.goldLine, inset: m.px(wide ? 12 : 24))
            }
            .buttonStyle(Quiet())
            .accessibilityHint("Opens today in the ordo")
            VRule(color: p.border)
            Button { sameView(year + 1) } label: { cell("\(String(year + 1)) ›", nav) }
                .buttonStyle(Quiet()).accessibilityLabel("Next year, \(String(year + 1))")
        }
        .frame(minHeight: 44)
    }

    private func cell(_ s: String, _ style: TextStyle) -> some View {
        Text(s).type(style).foregroundStyle(p.accent).padding(m.px(12)).frame(maxWidth: wide ? nil : .infinity)
    }

    // The month strip: the shown month underlined in gold, and today's month marked with the
    // lozenge, on its underline or, on a wide screen, on the strip's hairline beneath.
    private var strip: some View {
        let t = model.today
        let rows: [[Int]] = wide ? [Array(1...12)] : [Array(1...6), Array(7...12)]
        return VStack(spacing: 0) {
            ForEach(rows, id: \.self) { row in
                HStack(spacing: 0) {
                    ForEach(row, id: \.self) { mo in
                        let current = mo == month
                        let todays = Int(t.year) == year && Int(t.month) == mo
                        Button { model.open(.ordo(year: year, month: mo, day: 0)) } label: {
                            Text(monthShort(mo).uppercased()).type(.label(12.48, 0.06))
                                .foregroundStyle(current ? (wide ? p.accent : p.text) : p.muted)
                                .overlay(alignment: .bottom) {
                                    if !wide {
                                        ZStack {
                                            if current { Rectangle().fill(p.goldLine).frame(height: 2).padding(.horizontal, -m.px(14)) }
                                            if todays { Diamond(size: 6) }
                                        }
                                        .offset(y: m.px(5))
                                    }
                                }
                                .frame(maxWidth: .infinity, minHeight: 44)
                                .overlay(alignment: .bottom) {
                                    if wide {
                                        ZStack {
                                            if current { Rectangle().fill(p.goldLine).frame(height: 2) }
                                            if todays { Diamond(size: 6) }
                                        }
                                        .frame(height: 6)
                                        .offset(y: 3)
                                    }
                                }
                        }
                        .buttonStyle(Quiet())
                        .accessibilityLabel(monthName(mo))
                        .accessibilityAddTraits(current ? .isSelected : [])
                    }
                }
            }
        }
    }
}

/// A Kalendar's month, its rubricated heading in the titulus over the ornament's double rule; the way back to the top.
private struct MonthHeading: View {
    let name: String
    let isTodaysMonth: Bool
    let top: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        HStack(spacing: 0) {
            Text(name).type(TextStyle(size: 21.6, line: 28, tracking: 1.3, smallCaps: true)).foregroundStyle(p.titulus)
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityAddTraits(.isHeader)
            Button(action: top) {
                Text("↑").type(Scale.body.sized(16, line: 20)).foregroundStyle(p.muted).padding(.horizontal, m.px(14)).padding(.vertical, m.px(6))
            }
            .buttonStyle(Quiet())
            .accessibilityLabel("Back to the top")
        }
        .padding(.vertical, m.px(6.4))
        // Today's month takes the lozenge: a 6pt square of the lining's terracotta, ringed in the ground.
        .overlay(alignment: .bottom) { OrnamentRule(lozengeSize: isTodaysMonth ? 4.24 : nil, ring: p.bg, ink: p.lining) }
    }
}

/**
 * The ornament's double rule along a heading's foot, with a lozenge at its centre in the gilding
 * or `ink`, ringed in the ground where `ring`.
 */
private struct OrnamentRule: View {
    let lozengeSize: CGFloat?
    let ring: Color?
    var ink: Color?
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            for y in [size.height - 0.5, size.height - 2.5] {
                var line = Path()
                line.move(to: CGPoint(x: 0, y: y))
                line.addLine(to: CGPoint(x: size.width, y: y))
                ctx.stroke(line, with: .color(o.line), lineWidth: 1)
            }
            let c = CGPoint(x: size.width / 2, y: size.height - 1.5)
            if let lozengeSize {
                if let ring { ctx.fill(lozenge(c, m.px(lozengeSize + 2)), with: .color(ring)) }
                ctx.fill(lozenge(c, m.px(lozengeSize)), with: .color(ink ?? o.flat))
            }
        }
        .frame(height: 12)
        .offset(y: 4.5)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/// A day of the month: its colour as a rail, the date, the feast and its marks, and its disclosures.
private struct DayRow: View {
    let d: OrdoDayView
    let isToday: Bool
    let allDetails: Bool
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide
    @State private var details: Bool?
    @State private var comms = false

    private var hasLauds: Bool { !d.benedictusAntiphon.isEmpty || d.laudsPreces || d.laudsSuffrage || !d.laudsComms.isEmpty }
    private var hasVespers: Bool { !d.magnificatAntiphon.isEmpty || d.vespersPreces || d.vespersSuffrage || !d.vespersComms.isEmpty || !d.vespersNote.isEmpty }
    private var hasDetails: Bool { hasLauds || d.hoursPreces || hasVespers }

    /// One stop for VoiceOver: the date, its colour and observances, and the feast.
    private var spoken: String {
        [
            spokenDay(d.date, today: model.today),
            d.feast,
            "liturgical color: \(d.color)",
            d.fast ? "fasting" : nil,
            d.abstinence ? "abstinence" : nil,
            d.rank.isEmpty ? nil : "rank \(d.rank)",
        ].compactMap { $0 }.joined(separator: ". ")
    }

    var body: some View {
        if wide { table } else { card }
    }

    private func openDay() { model.open(.home(d.date)) }

    /// A day's rank is its ink, as in a Book of Hours: the great feasts red-letter, doubles slate blue, lesser days black.
    private var rankInk: Color? {
        switch d.rank {
        case "1cl", "2cl", "gd": return p.rubric
        case "d": return p.kalendarBlue
        default: return nil
        }
    }

    /// The feast's name in its rank's ink, a first-class feast's with a small painted cross before it;
    /// `fill` takes the column's width, else the name's own, wrapped to what it is given.
    private func feastName(_ style: TextStyle, fill: Bool = true) -> some View {
        let em = style.size * m.type
        return HStack(alignment: .firstTextBaseline, spacing: 0) {
            if d.rank == "1cl" {
                PaintedMark(.cross, size: em * 0.62, color: p.lining).padding(.trailing, em * 0.32)
            }
            Text(d.feast).type(style).foregroundStyle(rankInk ?? p.text).multilineTextAlignment(.leading)
        }
        .frame(maxWidth: fill ? .infinity : nil, alignment: .leading)
    }

    private var anyOpen: Bool { comms || ((details ?? allDetails) && hasDetails) }

    /// The disclosures' buttons: the commemorations' count and "Office details".
    private var disclosureButtons: some View {
        let open = details ?? allDetails
        return HStack(spacing: m.px(9.6)) {
            if !d.commemorations.isEmpty {
                let n = d.commemorations.count
                small("\(n) commemoration\(n > 1 ? "s" : "")", comms) { withAnimation(unfolding) { comms.toggle() } }
            }
            if hasDetails { small("Office details", open) { withAnimation(unfolding) { details = !open } } }
        }
    }

    @ViewBuilder private var disclosures: some View {
        disclosureButtons
        disclosed
    }

    /// What the open disclosures unfold: the commemorations, and the office digest.
    @ViewBuilder private var disclosed: some View {
        let open = details ?? allDetails
        if comms {
            ForEach(d.commemorations, id: \.self) { c in
                Text(c).type(TextStyle(size: 13.6, line: 20.4, italic: true)).foregroundStyle(p.muted)
            }
            .transition(.unfold)
        }
        if open && hasDetails { Digest(d: d, hasLauds: hasLauds, hasVespers: hasVespers).transition(.unfold) }
    }

    private func small(_ label: String, _ open: Bool, toggle: @escaping () -> Void) -> some View {
        Button(action: toggle) {
            HStack(spacing: 0) {
                Text(label).type(Scale.small.sized(12, line: 16.8)).foregroundStyle(p.muted)
                Caret(open: open, color: p.muted)
            }
            .frame(minHeight: 44)
        }
        .buttonStyle(Quiet())
        .accessibilityValue(open ? "Expanded" : "Collapsed")
    }

    private var card: some View {
        VStack(spacing: 0) {
            HStack(alignment: .top, spacing: 0) {
                // The day's liturgical colour as a rail beside its date.
                Rectangle().fill(dayColor(d.color)).frame(width: 3, height: m.px(40))
                Button(action: openDay) {
                    VStack(spacing: 0) {
                        Text("\(d.date.day)").type(TextStyle(size: 21.6, line: 23.76, lining: true)).foregroundStyle(isToday ? p.gold : p.accent)
                        Text(d.weekday.uppercased()).type(TextStyle.label(12, 0.06).sized(12, line: 16.8)).foregroundStyle(p.muted)
                        if isToday { Text("Today").type(Scale.small.sized(11, line: 14)).foregroundStyle(p.accent) }
                    }
                    .frame(width: m.px(59))
                }
                .buttonStyle(Quiet())
                .accessibilityLabel(spoken)
                .accessibilityHint("Opens the day")
                VStack(alignment: .leading, spacing: 0) {
                    HStack(alignment: .top, spacing: 0) {
                        // The date's stop already says the feast; the feast is a second target for the eye only.
                        Button(action: openDay) {
                            feastName(Scale.body.sized(16, line: 21.6))
                        }
                        .buttonStyle(Quiet())
                        .accessibilityHidden(true)
                        // The card's marks stay quiet, as the web's: red in a row means rank.
                        HStack(spacing: m.px(6)) {
                            if d.fast { Text("§").type(Scale.small.sized(12)).foregroundStyle(p.muted) }
                            if d.abstinence { FishIcon(color: p.muted) }
                            if !d.rank.isEmpty {
                                Text(d.rank).type(Scale.small.sized(12, line: 16.8)).foregroundStyle(p.muted)
                            }
                        }
                        .padding(.leading, m.px(8))
                        .padding(.top, m.px(2))
                        .accessibilityHidden(true)
                    }
                    disclosures
                }
            }
            .padding(.top, m.px(12.8))
            .padding(.bottom, m.px(6.4))
            Hairline(color: p.border)
        }
        // Today is painted, not selected: inset 8pt from the gutters.
        .background { if isToday { TodayBand(inset: 8) } }
    }

    private var table: some View {
        VStack(spacing: 0) {
            HStack(alignment: .top, spacing: 0) {
                Button(action: openDay) {
                    VStack(spacing: 0) {
                        Text("\(d.date.day)").type(TextStyle(size: 19.2, line: 23, lining: true)).foregroundStyle(isToday ? p.gold : p.accent)
                        if isToday { Text("Today").type(Scale.small.sized(11.2, line: 15.7)).foregroundStyle(p.accent) }
                    }
                    .padding(.leading, dayColumnPad)
                    .frame(width: dayColumn, alignment: .center)
                    .frame(maxHeight: .infinity, alignment: .top)
                    .overlay(alignment: .leading) { Rectangle().fill(dayColor(d.color)).frame(width: 3).padding(.top, 1.6) }
                }
                .buttonStyle(Quiet())
                .accessibilityLabel(spoken)
                .accessibilityHint("Opens the day")
                Text(d.weekday).type(Scale.small.sized(12.8)).foregroundStyle(p.muted)
                    .padding(.horizontal, 7.2).padding(.vertical, 3)
                    .frame(width: weekColumn, alignment: .leading)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 0) {
                    // The disclosures share the feast's line, 1.25rem after the name (the web's
                    // inline `.day-disclosures` from 701px), so a month reads as one line per
                    // day; one that is open drops below the name at the column's full width.
                    let name = Scale.body.sized(16, line: 22.4)
                    if anyOpen {
                        Button(action: openDay) { feastName(name) }
                            .buttonStyle(Quiet())
                            .accessibilityHidden(true)
                        disclosureButtons
                    } else {
                        HStack(alignment: .firstTextBaseline, spacing: m.px(20)) {
                            Button(action: openDay) { feastName(name, fill: false) }
                                .buttonStyle(Quiet())
                                .accessibilityHidden(true)
                                .layoutPriority(1)
                            disclosureButtons.fixedSize()
                            Spacer(minLength: 0)
                        }
                    }
                    disclosed
                }
                .padding(.horizontal, 7.2)
                // Fasting and abstinence stay quiet, so red in a row means rank.
                Group {
                    if d.fast { Text("§").type(Scale.body.sized(16)).foregroundStyle(p.muted) }
                }
                .frame(width: flagColumn)
                .accessibilityHidden(true)
                Group {
                    if d.abstinence { FishIcon(color: p.muted).padding(.top, 6) }
                }
                .frame(width: flagColumn)
                .accessibilityHidden(true)
                // A Kalendar has no hyperlinks: the rank keeps its ink and loses the rule.
                Group {
                    if !d.rank.isEmpty { Text(d.rank).type(Scale.small.sized(12.48, line: 17.5)).foregroundStyle(rankInk ?? p.text) }
                }
                .padding(.trailing, 7.2).padding(.top, 3)
                .frame(width: rankColumn, alignment: .trailing)
                .accessibilityHidden(true)
            }
            .fixedSize(horizontal: false, vertical: true)
            // The web's `.month-table td`: 0.5rem above and below, from 701px.
            .padding(.vertical, 8)
            Hairline(color: p.border)
        }
        // Today is painted, not selected: the band runs on across the row.
        .background { if isToday { TodayBand(inset: 0) } }
    }
}

/**
 * Today in the ordo: a ground of the frieze's wash ruled top and bottom in the gold line, above the
 * row's hairline, `inset` from its sides.
 */
private struct TodayBand: View {
    let inset: CGFloat
    @Environment(\.palette) private var p

    var body: some View {
        VStack(spacing: 0) {
            Rectangle().fill(p.goldLine).frame(height: 1)
            Rectangle().fill(p.inscriptionWash)
            Rectangle().fill(p.goldLine).frame(height: 1)
        }
        .padding(.horizontal, inset)
        .padding(.bottom, 1)
        .accessibilityHidden(true)
    }
}

// The desktop table's column widths (`.month-table`): day, weekday, the feast, fasting, abstinence, rank.
private let dayColumn: CGFloat = 44.8
private let dayColumnPad: CGFloat = 10.4
private let weekColumn: CGFloat = 48
private let flagColumn: CGFloat = 41.6
private let rankColumn: CGFloat = 51.2

/// The table's column headings, over a hairline.
private struct DayColumns: View {
    @Environment(\.palette) private var p

    var body: some View {
        let th = TextStyle.label(11.2, 0.06)
        VStack(spacing: 0) {
            HStack(spacing: 0) {
                Text("DAY").type(th).padding(.leading, dayColumnPad).frame(width: dayColumn)
                Text("WK").type(th).padding(.horizontal, 7.2).frame(width: weekColumn, alignment: .leading)
                Text("FEAST / OBSERVANCE").type(th).padding(.horizontal, 7.2).frame(maxWidth: .infinity, alignment: .leading)
                Text("FAST").type(th).frame(width: flagColumn)
                Text("ABST.").type(th).frame(width: flagColumn)
                Text("RANK").type(th).padding(.trailing, 7.2).frame(width: rankColumn, alignment: .trailing)
            }
            .foregroundStyle(p.muted)
            .padding(.top, 8)
            .padding(.bottom, 5.6)
            Hairline(color: p.border)
        }
        .accessibilityHidden(true)
    }
}

/// The office digest: each hour's gospel antiphon, preces, suffrage, and commemorations.
private struct Digest: View {
    let d: OrdoDayView
    let hasLauds: Bool
    let hasVespers: Bool
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            if hasLauds {
                hour("Lauds", notes: [], antiphon: d.benedictusAntiphon.isEmpty ? nil : "Ben. “\(d.benedictusAntiphon)”", preces: d.laudsPreces, suffrage: d.laudsSuffrage, comms: d.laudsComms)
            }
            if d.hoursPreces { hour("Hours", notes: [], antiphon: nil, preces: true, suffrage: false, comms: []) }
            if hasVespers {
                hour("Vespers", notes: d.vespersNote.isEmpty ? [] : [d.vespersNote], antiphon: d.magnificatAntiphon.isEmpty ? nil : "Mag. “\(d.magnificatAntiphon)”", preces: d.vespersPreces, suffrage: d.vespersSuffrage, comms: d.vespersComms)
            }
        }
        .padding(m.px(8))
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(alignment: .leading) { Rectangle().fill(p.goldLine).frame(width: 1) }
        .padding(.bottom, m.px(8))
    }

    private func hour(_ name: String, notes: [String], antiphon: String?, preces: Bool, suffrage: Bool, comms: [CommemorationView]) -> some View {
        let line = TextStyle(size: 13.6, line: 20.4, italic: true)
        var marks = Text("")
        if let antiphon { marks = marks + Text(antiphon) }
        var rubric = [String]()
        if preces { rubric.append("Preces") }
        if suffrage { rubric.append("Suffrage") }
        if !rubric.isEmpty {
            let lead = antiphon == nil ? "" : "  "
            marks = marks + Text(lead + rubric.joined(separator: "  ")).font(Font(garamond(13.6 * m.type))).foregroundColor(p.rubric)
        }
        return VStack(alignment: .leading, spacing: 0) {
            Text(name.uppercased()).type(TextStyle.label(12, 0.06).sized(12, line: 18)).foregroundStyle(p.accent).padding(.bottom, m.px(2.4))
            ForEach(notes, id: \.self) { Text($0).type(line).foregroundStyle(p.muted) }
            if antiphon != nil || preces || suffrage { marks.type(line).foregroundStyle(p.muted) }
            ForEach(comms, id: \.self) { c in
                Text("Com. \(c.name)" + (c.incipit.isEmpty ? "" : " “\(c.incipit)”")).type(line).foregroundStyle(p.muted)
            }
        }
        .padding(.bottom, m.px(4))
    }
}

/**
 * A year's frontispiece, as the web's /calendar/{year}: the title page, the month strip, and the
 * Tabula Temporaria. Each date leads to its day in the ordo.
 */
struct OrdoYearScreen: View {
    let view: OrdoYearView
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide

    var body: some View {
        ScrollView {
            VStack(spacing: 0) {
                SiteHeader()
                OrdoHeader(year: Int(view.year), month: nil, roman: view.roman).ordoColumn(m, wide: wide).padding(.top, m.px(17.6))
                VStack(spacing: 0) {
                    TabulaHeading()
                    if wide {
                        // The web's desktop tabula: the figures in one line, the two tables side by side.
                        Figures(figures: view.figures, perRow: 4).padding(.top, m.px(24))
                        HStack(alignment: .top, spacing: 40) {
                            TabulaTable(title: "Moveable feasts", rows: view.moveable)
                            TabulaTable(title: "Ember days", rows: view.ember)
                        }
                        .padding(.top, m.px(24))
                    } else {
                        Figures(figures: view.figures, perRow: 2).padding(.top, m.px(24))
                        TabulaTable(title: "Moveable feasts", rows: view.moveable).padding(.top, m.px(24))
                        TabulaTable(title: "Ember days", rows: view.ember).padding(.top, m.px(24))
                    }
                }
                .ordoColumn(m, wide: wide)
                .padding(.top, m.px(40))
                Footer()
            }
        }
    }
}

/// "Tabula Temporaria", a titulus in small capitals over the ornament's double rule, its lozenge at the centre.
private struct TabulaHeading: View {
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Text("Tabula Temporaria").type(TextStyle(size: 21.6, line: 28, tracking: 1.3, smallCaps: true)).foregroundStyle(p.titulus)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.vertical, m.px(6.4))
            // Ringed in the page's ground, as the web's box-shadow parts the rules around it.
            .overlay(alignment: .bottom) { OrnamentRule(lozengeSize: 2.5, ring: p.bg) }
            .accessibilityAddTraits(.isHeader)
    }
}

/**
 * The year's four figures, `perRow` to a row, on a painted tablet: the surface thinned so the wall
 * shows through, framed in the lining ruled twice (a thinner line 3pt inside the first), with a
 * quatrefoil knop at each corner where the rules stop short. Each numeral large in the accent, its
 * name beneath.
 */
private struct Figures: View {
    let figures: [TabulaRowView]
    let perRow: Int
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let rows = stride(from: 0, to: figures.count, by: perRow).map { Array(figures[$0..<min($0 + perRow, figures.count)]) }
        VStack(spacing: m.px(20)) {
            ForEach(Array(rows.enumerated()), id: \.offset) { _, row in
                HStack(alignment: .top, spacing: m.px(16)) {
                    ForEach(row, id: \.label) { f in
                        VStack(spacing: 0) {
                            // Lining figures stand 4 and 25 level with XIII and D.
                            Text(f.value).type(TextStyle(size: 25.6, line: 30.7, lining: true)).foregroundStyle(p.accent)
                            Text(f.label.uppercased()).type(.label(11.2, 0.06)).foregroundStyle(p.muted).multilineTextAlignment(.center).padding(.top, m.px(3))
                        }
                        .frame(maxWidth: .infinity)
                        .accessibilityElement(children: .combine)
                    }
                }
            }
        }
        .padding(.horizontal, m.px(6.4))
        .padding(.top, m.px(17.6))
        .padding(.bottom, m.px(16))
        .background(p.surface.opacity(0.6))
        .overlay { TabletFrame() }
    }
}

/// The tablet's frame: two rules each side, stopping short of the corners, where the knops sit.
private struct TabletFrame: View {
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let knop = m.px(14)
        ZStack {
            Canvas { ctx, size in
                for (inset, ink) in [(CGFloat(0), p.lining), (3, p.lining.opacity(0.4))] {
                    let a = inset + 0.5
                    var rule = Path()
                    rule.move(to: CGPoint(x: knop, y: a)); rule.addLine(to: CGPoint(x: size.width - knop, y: a))
                    rule.move(to: CGPoint(x: knop, y: size.height - a)); rule.addLine(to: CGPoint(x: size.width - knop, y: size.height - a))
                    rule.move(to: CGPoint(x: a, y: knop)); rule.addLine(to: CGPoint(x: a, y: size.height - knop))
                    rule.move(to: CGPoint(x: size.width - a, y: knop)); rule.addLine(to: CGPoint(x: size.width - a, y: size.height - knop))
                    ctx.stroke(rule, with: .color(ink), lineWidth: 1)
                }
            }
            let corners: [Alignment] = [.topLeading, .topTrailing, .bottomLeading, .bottomTrailing]
            ForEach(0..<corners.count, id: \.self) { i in
                PaintedMark(.quatrefoil, size: knop, color: p.lining).frame(maxWidth: .infinity, maxHeight: .infinity, alignment: corners[i])
            }
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/// A titled list of dates, ruled between rows; a date that cannot share its row takes the next line whole.
private struct TabulaTable: View {
    let title: String
    let rows: [TabulaRowView]
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(title.uppercased()).type(.label(11.2, 0.06)).foregroundStyle(p.muted).padding(.vertical, m.px(5.6)).accessibilityAddTraits(.isHeader)
            Hairline(color: p.border)
            ForEach(rows, id: \.label) { r in
                Button {
                    if let date = r.date { model.open(.ordo(year: Int(date.year), month: Int(date.month), day: Int(date.day))) }
                } label: {
                    ViewThatFits(in: .horizontal) {
                        HStack(spacing: m.px(16)) {
                            Text(r.label).type(Scale.body).foregroundStyle(p.text).fixedSize()
                            Spacer(minLength: 0)
                            Text(r.value).type(Scale.body).foregroundStyle(p.accent).fixedSize()
                        }
                        VStack(alignment: .leading, spacing: 0) {
                            Text(r.label).type(Scale.body).foregroundStyle(p.text)
                            Text(r.value).type(Scale.body).foregroundStyle(p.accent).frame(maxWidth: .infinity, alignment: .trailing)
                        }
                    }
                    .padding(.vertical, m.px(8.8))
                    .frame(minHeight: 44)
                }
                .buttonStyle(Quiet())
                .disabled(r.date == nil)
                .accessibilityElement(children: .ignore)
                .accessibilityLabel("\(r.label), \(r.value)")
                .accessibilityHint(r.date == nil ? "" : "Opens the day in the ordo")
                Hairline(color: p.border)
            }
        }
        .frame(maxWidth: .infinity)
    }
}
