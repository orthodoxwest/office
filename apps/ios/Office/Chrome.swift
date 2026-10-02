import SwiftUI
import UIKit

/// The reading measure's side gutter (`--page-gutter` on a phone).
let gutter: CGFloat = 16

/// Wide screens keep the web's prayer measure (`.elements`, 38rem).
let measure: CGFloat = 608

extension View {
    /// The prayer measure, centred, with the page's gutters.
    func measured(_ m: Metrics, max: CGFloat = measure) -> some View {
        frame(maxWidth: m.px(max)).padding(.horizontal, m.px(gutter)).frame(maxWidth: .infinity)
    }

    /// A current control's gold underline.
    func goldUnderline(_ on: Bool, _ color: Color, inset: CGFloat = 0) -> some View {
        overlay(alignment: .bottom) {
            if on { Rectangle().fill(color).frame(height: 1).padding(.horizontal, inset) }
        }
    }
}

/**
 * The web's controls mark state, not touches; a pressed control only dims a little, at once,
 * and comes back over a moment when let go, so even the briefest tap is seen to land.
 */
struct Quiet: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .contentShape(Rectangle())
            .opacity(configuration.isPressed ? 0.55 : 1)
            .animation(configuration.isPressed ? nil : .easeOut(duration: 0.2), value: configuration.isPressed)
    }
}

/// The disclosure caret, the web's `▾`/`▴` in the ink of the words it opens, a little lighter: drawn, not read; the control says expanded or collapsed.
struct Caret: View {
    let open: Bool
    /// The label's colour.
    let color: Color
    @Environment(\.metrics) private var m

    var body: some View {
        Text(open ? " ▴" : " ▾").font(.system(size: 9 * m.type)).foregroundStyle(color.opacity(0.7)).accessibilityHidden(true)
    }
}

/**
 * Where the site's navigation leads, and which of it is the page shown: the day's hours on an
 * hour page, then the Ordo and Reminders. The menu sets it out on a phone, the header inline on
 * a wide screen.
 */
struct SiteNav {
    let hours: [String]
    let currentHour: String?
    let onHour: ((String) -> Void)?
    let onOrdo: () -> Void
    let ordoCurrent: Bool
    let onReminders: () -> Void
    let remindersCurrent: Bool

    init(_ model: AppModel) {
        let page = model.page
        hours = model.hours
        if case let .hour(date, hour) = page {
            currentHour = hour
            onHour = { model.open(.hour(date, $0)) }
        } else {
            currentHour = nil
            onHour = nil
        }
        onOrdo = {
            // The ordo at the day shown, as the web's /calendar opens at today's row.
            switch page {
            case let .home(d), let .hour(d, _): model.open(.ordo(year: Int(d.year), month: Int(d.month), day: Int(d.day)))
            case let .ordo(y, m, _): model.open(.ordo(year: y, month: m, day: 0))
            case let .year(y): model.open(.year(y))
            case .reminders: model.open(.ordo(year: Int(model.today.year), month: Int(model.today.month), day: Int(model.today.day)))
            }
        }
        switch page {
        case .ordo, .year: ordoCurrent = true
        default: ordoCurrent = false
        }
        onReminders = { model.open(.reminders) }
        remindersCurrent = page == .reminders
    }
}

/// The header beam: the brand's roundel and "Daily Office" home, and the menu, or on a wide screen the links themselves.
struct SiteHeader: View {
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.wide) private var wide
    @Environment(\.ranked) private var ranked

    var body: some View {
        let nav = SiteNav(model)
        // From 701 to 959pt an hour's links don't fit beside the brand and Settings: left in the
        // row they would crowd it, so they take a centred rank of their own beneath, as a book sets
        // its running head over the page.
        let rank = wide && ranked && nav.onHour != nil
        VStack(spacing: 0) {
            HStack(spacing: 0) {
                Button(action: model.goHome) {
                    HStack(spacing: 0) {
                        // The consecration roundel that ends every hour, 18pt.
                        PaintedMark(.consecration, size: m.px(18), color: p.lining)
                            .padding(.trailing, m.px(6.08))
                        Text(" DAILY OFFICE").type(Scale.brand).foregroundStyle(p.text)
                    }
                    .padding(.vertical, m.px(10))
                }
                .buttonStyle(Quiet())
                .accessibilityLabel("Daily Office, home")
                Spacer(minLength: m.px(8))
                if rank {
                    InlineNav(nav: nav, links: false)
                } else if wide {
                    InlineNav(nav: nav)
                } else {
                    Button { model.menuOpen.toggle() } label: {
                        HStack(spacing: 0) {
                            Text("MENU").type(Scale.menu).foregroundStyle(p.accent)
                            Caret(open: model.menuOpen, color: p.accent)
                        }
                        .padding(.leading, m.px(12.8))
                        .padding(.trailing, m.px(3.2))
                        .frame(minHeight: 44)
                    }
                    .buttonStyle(Quiet())
                    .accessibilityLabel("Menu")
                    .accessibilityValue(model.menuOpen ? "Expanded" : "Collapsed")
                }
            }
            .padding(.horizontal, m.px(gutter))
            .padding(.top, m.px(6.4))
            .padding(.bottom, rank ? 0 : m.px(5.6))
            .frame(minHeight: 44)
            // The web's nav shell: held to 68rem, so the whole list fits on one line.
            .frame(maxWidth: wide ? m.px(1088) : .infinity)
            .frame(maxWidth: .infinity)
            if rank {
                InlineNav(nav: nav, settings: false)
                    .padding(.top, m.px(2.4))
                    .padding(.bottom, m.px(5.6))
                    .frame(maxWidth: .infinity)
            }
            // The beam: a 4pt course of oak, its upper edge catching the light as a timber's arris does.
            VStack(spacing: 0) {
                Rectangle().fill(p.materialHighlight).frame(height: 1)
                Rectangle().fill(p.oak.opacity(0.55)).frame(height: 4)
            }
            .accessibilityHidden(true)
        }
    }
}

/// The desktop header's links (`.site-menu nav`), muted: the current one in ink over the lining's terracotta; Reminders quieter.
private struct InlineNav: View {
    let nav: SiteNav
    /// The pages' links, and Settings after them; a ranked header sets them apart.
    var links = true
    var settings = true
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        HStack(spacing: 0) {
            if links {
                if let onHour = nav.onHour {
                    ForEach(nav.hours, id: \.self) { h in link(hourLabel(h), h == nav.currentHour) { onHour(h) } }
                    Rectangle().fill(p.border).frame(width: 1, height: m.px(24)).padding(.horizontal, m.px(2.4))
                }
                link("Ordo", nav.ordoCurrent, action: nav.onOrdo)
                link("Reminders", nav.remindersCurrent, secondary: true, action: nav.onReminders)
            }
            // Settings closes the links, quiet as Reminders; its panel holds the theme and text size.
            if settings {
                Button { model.settingsOpen.toggle() } label: {
                    HStack(spacing: 0) {
                        Text("SETTINGS").type(.label(11.52, 0.04)).foregroundStyle(p.muted)
                        Caret(open: model.settingsOpen, color: p.muted)
                    }
                    .padding(.leading, m.px(8.8))
                    .padding(.trailing, m.px(3.2))
                    .frame(minHeight: 44)
                }
                .buttonStyle(Quiet())
                .accessibilityLabel("Settings")
                .accessibilityValue(model.settingsOpen ? "Expanded" : "Collapsed")
            }
        }
    }

    private func link(_ label: String, _ current: Bool, secondary: Bool = false, action: @escaping () -> Void) -> some View {
        let style = secondary ? TextStyle.label(11.52, 0.04) : TextStyle.label(12.48, 0.06)
        return Button(action: action) {
            Text(label.uppercased()).type(style).foregroundStyle(current ? p.text : p.muted)
                .overlay(alignment: .bottom) {
                    if current { Rectangle().fill(p.lining).frame(height: 1.5).offset(y: m.px(4)) }
                }
                .padding(.horizontal, m.px(4.8))
                .frame(minHeight: 44)
        }
        .buttonStyle(Quiet())
        .accessibilityAddTraits(current ? .isSelected : [])
    }
}

/**
 * The site menu's panel: on an hour, the day's hours (2/3/2 as on home); the Ordo and
 * Reminders, the current page underlined in the lining's terracotta; then the Theme and Text
 * rows, the current choice underlined in gold. `prefsOnly` is the wide header's Settings: the
 * Theme and Text rows alone.
 */
struct MenuPanel: View {
    var prefsOnly = false
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let nav = SiteNav(model)
        let style = TextStyle.label(13.12, 0.06)
        VStack(spacing: 0) {
            if let onHour = nav.onHour, !prefsOnly {
                ForEach([["lauds", "prime"], ["terce", "sext", "none"], ["vespers", "compline"]], id: \.self) { row in
                    HStack(spacing: 0) {
                        ForEach(row, id: \.self) { h in link(hourLabel(h).uppercased(), h == nav.currentHour, style) { onHour(h) } }
                    }
                    .padding(.bottom, m.px(2.4))
                }
                Hairline(color: p.border).padding(.vertical, m.px(4))
            }
            if !prefsOnly {
                HStack(spacing: 0) {
                    link("ORDO", nav.ordoCurrent, style, action: nav.onOrdo)
                    // Habit setup, not an hour: quieter than the Ordo, as on the web.
                    link("REMINDERS", nav.remindersCurrent, .label(12, 0.06), action: nav.onReminders)
                }
                Hairline(color: p.border).padding(.top, m.px(6.4)).padding(.bottom, m.px(6.4))
            }
            HStack(spacing: 0) {
                rowLabel("THEME")
                ForEach(ThemeChoice.allCases) { t in
                    cell(t.label.uppercased(), t == model.theme, .label(12, 0.06), p.accent, spoken: "\(t.label) theme") {
                        guard t != model.theme else { return }
                        chose()
                        withAnimation(restyling) { model.chooseTheme(t) }
                    }
                }
            }
            HStack(spacing: 0) {
                rowLabel("TEXT")
                ForEach(TextSize.allCases) { s in
                    cell("A", s == model.textSize, .label(s == .small ? 13 : s == .standard ? 16 : 20, 0), p.muted, spoken: textSizeName(s)) {
                        guard s != model.textSize else { return }
                        chose()
                        withAnimation(restyling) { model.chooseTextSize(s) }
                    }
                }
            }
        }
        .padding(m.px(10.4))
        .frame(maxWidth: m.px(prefsOnly ? 288 : 336))
        .background(p.surface)
        .overlay(Rectangle().stroke(p.border, lineWidth: 1))
        .overlay(alignment: .top) { Rectangle().fill(p.lining).frame(height: 2) }
        .shadow(color: .black.opacity(p.dark ? 0.4 : 0.12), radius: 12, y: 4)
    }

    private func rowLabel(_ s: String) -> some View {
        Text(s).type(.label(10.56, 0.08)).foregroundStyle(p.muted)
            .frame(width: m.px(54), alignment: .leading)
            .padding(.leading, m.px(10.4))
            .accessibilityHidden(true)
    }

    /// A link of the site's navigation: muted, the current page in ink over the lining's terracotta.
    private func link(_ label: String, _ current: Bool, _ style: TextStyle, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(label).type(style).foregroundStyle(current ? p.text : p.muted)
                .frame(maxWidth: .infinity, minHeight: 44)
                .overlay(alignment: .bottom) {
                    if current { Rectangle().fill(p.lining).frame(height: 1.5) }
                }
        }
        .buttonStyle(Quiet())
        .accessibilityLabel(label.capitalized)
        .accessibilityAddTraits(current ? .isSelected : [])
    }

    private func cell(_ label: String, _ current: Bool, _ style: TextStyle, _ color: Color, spoken: String? = nil, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(label).type(style).foregroundStyle(current ? p.text : color)
                .frame(maxWidth: .infinity, minHeight: 44)
                .goldUnderline(current, p.goldLine)
        }
        .buttonStyle(Quiet())
        .accessibilityLabel(spoken ?? label.capitalized)
        .accessibilityAddTraits(current ? .isSelected : [])
    }
}

func textSizeName(_ s: TextSize) -> String {
    switch s {
    case .small: return "Smaller text"
    case .standard: return "Default text size"
    case .large: return "Larger text"
    }
}

/**
 * A light tick under the finger as a choice is made (a setting, a prayer form, a day), as the
 * Android app gives; never for a page opened. The phone's own haptics setting governs it.
 */
func chose() { UISelectionFeedbackGenerator().selectionChanged() }

/// The tap of a checkbox turned on or off.
func toggled() { UIImpactFeedbackGenerator(style: .light).impactOccurred() }

/// How the room changes when the reader changes how it looks: dimming rather than snapping, as the web's theme does.
let restyling = Animation.easeInOut(duration: 0.32)

/**
 * Brings what a disclosure unfolded into view, once it has unfolded, when it opened past the
 * screen's edge: a page's scroll view sets it (`revealing`), and the disclosure names its
 * contents' id.
 */
struct Reveal {
    fileprivate let scroll: (String) -> Void
    func callAsFunction(_ id: String) { scroll(id) }
}

private struct RevealKey: EnvironmentKey {
    static let defaultValue = Reveal { _ in }
}

extension EnvironmentValues {
    var reveal: Reveal {
        get { self[RevealKey.self] }
        set { self[RevealKey.self] = newValue }
    }
}

extension View {
    /// Lets the disclosures inside bring what they unfold into view through `proxy`.
    func revealing(_ proxy: ScrollViewProxy) -> some View {
        environment(\.reveal, Reveal { id in
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.28) {
                withAnimation(.easeInOut(duration: 0.3)) { proxy.scrollTo(id, anchor: .bottom) }
            }
        })
    }
}

/// How a disclosure opens and closes, as on Android: over a quarter second, easing in and out.
let unfolding = Animation.easeInOut(duration: 0.26)

extension AnyTransition {
    /// A disclosure's contents, fading in as they drop a little from their control, and back.
    static var unfold: AnyTransition {
        .asymmetric(
            insertion: .opacity.animation(.easeOut(duration: 0.2).delay(0.06)).combined(with: .offset(y: -6)),
            removal: .opacity.animation(.easeIn(duration: 0.1)).combined(with: .offset(y: -6))
        )
    }
}

/// A small uppercase disclosure label with its caret ("Change date ▾").
struct Disclosure: View {
    let label: String
    let open: Bool
    var value: String?
    let toggle: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Button(action: toggle) {
            HStack(spacing: 0) {
                (Text(label.uppercased()) + Text(value.map { " " + $0.uppercased() } ?? "").foregroundColor(p.text))
                    .type(Scale.control)
                    .foregroundStyle(p.muted)
                Caret(open: open, color: p.muted)
            }
            .padding(.horizontal, m.px(8))
            .frame(minHeight: 44)
        }
        .buttonStyle(Quiet())
        .accessibilityValue(open ? "Expanded" : "Collapsed")
    }
}

/// The picker's span of years, as the web's (app.js PICKER_FIRST_YEAR, PICKER_LAST_YEAR).
private let pickerYears = 1950...2150

/**
 * The hand-set date picker (app.js): Previous · Today · Next, then a month grid whose days are
 * the choices. Sundays are red; the chosen day takes the gold underline, as a chosen prayer form
 * does, and today a quiet wash. The title turns the grid into the year's months, and back; both
 * views keep six weeks' height, so turning between them moves nothing below.
 */
struct DayPicker: View {
    let shown: CivilDate
    let today: CivilDate
    let pick: (CivilDate) -> Void
    @State private var year: Int
    @State private var month: Int
    @State private var months = false
    /// Which way the days last turned: 1 to a later month, -1 to an earlier.
    @State private var turn: CGFloat = 1
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    init(shown: CivilDate, today: CivilDate, pick: @escaping (CivilDate) -> Void) {
        self.shown = shown
        self.today = today
        self.pick = pick
        let y = Int(shown.year)
        _year = State(initialValue: min(max(y, pickerYears.lowerBound), pickerYears.upperBound))
        _month = State(initialValue: y < pickerYears.lowerBound ? 1 : y > pickerYears.upperBound ? 12 : Int(shown.month))
    }

    var body: some View {
        let link = Scale.body.sized(16, line: 24)
        VStack(spacing: 0) {
            Hairline(color: p.border).padding(.horizontal, m.px(24))
            HStack {
                Button { pick(shown.adding(days: -1)) } label: { Text("← Previous").type(link).foregroundStyle(p.accent).padding(.horizontal, m.px(16)).frame(minHeight: 44) }
                    .buttonStyle(Quiet()).accessibilityLabel("Previous day")
                Spacer()
                if shown != today {
                    Button { pick(today) } label: { Text("TODAY").type(Scale.control).foregroundStyle(p.muted).frame(minHeight: 44) }.buttonStyle(Quiet())
                }
                Spacer()
                Button { pick(shown.adding(days: 1)) } label: { Text("Next →").type(link).foregroundStyle(p.accent).padding(.horizontal, m.px(16)).frame(minHeight: 44) }
                    .buttonStyle(Quiet()).accessibilityLabel("Next day")
            }
            .padding(.top, m.px(8))
            HStack(spacing: 0) {
                // A month's step in the days, a year's in the months; none past the picker's span.
                step("‹", back, months ? "Previous year" : "Previous month")
                Button { withAnimation(.easeInOut(duration: 0.18)) { months.toggle() } } label: {
                    HStack(spacing: 0) {
                        Text(title.uppercased()).type(.label(12.8, 0.1)).foregroundStyle(p.text)
                        Caret(open: months, color: p.text)
                    }
                    .frame(maxWidth: .infinity, minHeight: 44)
                }
                .buttonStyle(Quiet())
                .accessibilityLabel(months ? "\(title), back to days" : "\(title), choose another month")
                step("›", forward, months ? "Next year" : "Next month")
            }
            .padding(.top, m.px(12))
            ZStack {
                // The days always lay out, for the height both views share; the months cover them.
                // A new month's days slide in the way it turned.
                days
                    .id(year * 12 + month)
                    .transition(reduceMotion ? .opacity : .asymmetric(
                        insertion: .offset(x: 60 * turn).combined(with: .opacity.animation(.easeOut(duration: 0.18).delay(0.06))),
                        removal: .offset(x: -60 * turn).combined(with: .opacity.animation(.easeIn(duration: 0.09)))
                    ))
                    .opacity(months ? 0 : 1)
                    .accessibilityHidden(months)
                if months { monthGrid.transition(.opacity) }
            }
            .clipped()
            .padding(.horizontal, m.px(8))
            // A swipe across the days turns the month, as the arrows do.
            .simultaneousGesture(
                DragGesture(minimumDistance: 24).onEnded { drag in
                    guard !months, abs(drag.translation.width) > max(48, abs(drag.translation.height) * 2) else { return }
                    turnTo(drag.translation.width < 0 ? forward : back)
                }
            )
        }
        .padding(.top, m.px(8))
    }

    private var title: String { months ? "\(year)" : "\(monthName(month)) \(year)" }

    /// Shows another month's days (or year's months), moving the way it lies.
    private func turnTo(_ to: (Int, Int)?) {
        guard let to else { return }
        turn = to.0 * 12 + to.1 > year * 12 + month ? 1 : -1
        withAnimation(.easeInOut(duration: 0.26)) {
            year = to.0
            month = to.1
        }
    }

    private var back: (Int, Int)? {
        let (y, mo) = months ? (year - 1, month) : month == 1 ? (year - 1, 12) : (year, month - 1)
        return pickerYears.contains(y) ? (y, mo) : nil
    }

    private var forward: (Int, Int)? {
        let (y, mo) = months ? (year + 1, month) : month == 12 ? (year + 1, 1) : (year, month + 1)
        return pickerYears.contains(y) ? (y, mo) : nil
    }

    private func step(_ glyph: String, _ to: (Int, Int)?, _ label: String) -> some View {
        Button { turnTo(to) } label: {
            Text(glyph).type(Scale.body.sized(22, line: 28)).foregroundStyle(to == nil ? p.muted.opacity(0.35) : p.accent)
                .padding(.horizontal, m.px(24)).frame(minHeight: 44)
        }
        .buttonStyle(Quiet())
        .disabled(to == nil)
        .accessibilityLabel(label)
    }

    /// A month's days in six weeks always, blank where the month is not.
    private var days: some View {
        let first = CivilDate(year: Int32(year), month: Int32(month), day: 1)
        let lead = first.weekday - 1
        let length = civil.range(of: .day, in: .month, for: first.date)?.count ?? 30
        return VStack(spacing: 0) {
            HStack(spacing: 0) {
                ForEach(Array(["S", "M", "T", "W", "T", "F", "S"].enumerated()), id: \.offset) { _, d in
                    Text(d).type(.label(11.5, 0.08)).foregroundStyle(p.muted).frame(maxWidth: .infinity).padding(.vertical, m.px(8))
                }
            }
            .accessibilityHidden(true)
            ForEach(0..<6, id: \.self) { week in
                HStack(spacing: 0) {
                    ForEach(0..<7, id: \.self) { i in
                        let n = week * 7 + i - lead + 1
                        if (1...length).contains(n) {
                            let day = CivilDate(year: Int32(year), month: Int32(month), day: Int32(n))
                            Button {
                                chose()
                                pick(day)
                            } label: {
                                Text("\(n)").type(TextStyle(size: 18, line: 30, lining: true))
                                    .foregroundStyle(i == 0 ? p.rubric : p.text)
                                    .goldUnderline(day == shown, p.goldLine)
                                    .frame(maxWidth: .infinity, minHeight: 44)
                                    .background(day == today ? p.pressedWash : .clear)
                            }
                            .buttonStyle(Quiet())
                            .accessibilityLabel(spokenDay(day, today: today))
                            .accessibilityAddTraits(day == shown ? .isSelected : [])
                        } else {
                            Color.clear.frame(maxWidth: .infinity, minHeight: 44)
                        }
                    }
                }
            }
        }
    }

    /// The year's months, three to a row: the chosen day's month underlined in gold, today's washed.
    private var monthGrid: some View {
        VStack(spacing: 0) {
            ForEach(0..<4, id: \.self) { row in
                HStack(spacing: 0) {
                    ForEach(1...3, id: \.self) { col in
                        let mo = row * 3 + col
                        let chosen = Int(shown.year) == year && Int(shown.month) == mo
                        Button {
                            month = mo
                            months = false
                        } label: {
                            Text(monthShort(mo).uppercased()).type(.label(11.5, 0.1)).foregroundStyle(chosen ? p.accent : p.text)
                                .padding(.horizontal, m.px(14.4)).padding(.vertical, m.px(13))
                                .goldUnderline(chosen, p.goldLine)
                                .background(Int(today.year) == year && Int(today.month) == mo ? p.gold.opacity(0.1) : .clear)
                        }
                        .buttonStyle(Quiet())
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                        .accessibilityLabel(monthName(mo) + " " + String(year))
                        .accessibilityAddTraits(chosen ? .isSelected : [])
                    }
                }
            }
        }
    }
}

/// "How are you praying?": the three prayer forms, remembered on this device.
struct FormChooser: View {
    let form: String
    let choose: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(spacing: 0) {
            Text("How are you praying?").type(Scale.body.sized(16, line: 24)).foregroundStyle(p.muted).padding(.bottom, m.px(4))
            ForEach(prayerForms, id: \.value) { f in
                Button {
                    if f.value != form { chose() }
                    choose(f.value)
                } label: {
                    HStack(spacing: m.px(10)) {
                        ZStack {
                            Circle().stroke(f.value == form ? p.gold : p.border, lineWidth: 1).frame(width: m.px(16), height: m.px(16))
                            if f.value == form { Circle().fill(p.gold).frame(width: m.px(8), height: m.px(8)) }
                        }
                        Text(f.phrase).type(Scale.body.sized(17, line: 24)).foregroundStyle(p.text)
                    }
                    .frame(minHeight: 44)
                }
                .buttonStyle(Quiet())
                .accessibilityAddTraits(f.value == form ? .isSelected : [])
            }
            Text("Remembered on this device. Change it any time.").type(Scale.small).foregroundStyle(p.muted).padding(.top, m.px(4))
        }
        .frame(maxWidth: .infinity)
        .padding(.top, m.px(8))
        .padding(.bottom, m.px(4))
    }
}

/**
 * The continuation after a page's content: the previous item, the way back to all of them, and
 * the next item. It carries no ornament of its own; an hour ends on its consecration cross above it.
 */
struct Continuation: View {
    let previousLabel: String
    let previous: String?
    let onPrevious: () -> Void
    let middle: String
    let onMiddle: () -> Void
    let nextLabel: String
    let next: String?
    let onNext: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let small = TextStyle.label(11.2, 0.08)
        let name = Scale.body.sized(16, line: 22)
        HStack(alignment: .center, spacing: 0) {
            Group {
                if let previous {
                    Button(action: onPrevious) {
                        VStack(alignment: .leading, spacing: 0) {
                            Text(previousLabel.uppercased()).type(small).foregroundStyle(p.muted)
                            Text("← \(previous)").type(name).foregroundStyle(p.accent)
                        }
                    }
                    .buttonStyle(Quiet())
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("\(previousLabel): \(previous)")
                    .accessibilityAddTraits(.isButton)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            Button(action: onMiddle) {
                Text(middle.uppercased()).type(.label(12.16, 0.06)).foregroundStyle(p.muted)
                    .padding(.horizontal, m.px(6.4)).padding(.vertical, m.px(12))
            }
            .buttonStyle(Quiet())
            .accessibilityLabel(middle)
            Group {
                if let next {
                    Button(action: onNext) {
                        VStack(alignment: .trailing, spacing: 0) {
                            Text(nextLabel.uppercased()).type(small).foregroundStyle(p.muted)
                            Text("\(next) →").type(name).foregroundStyle(p.accent)
                        }
                    }
                    .buttonStyle(Quiet())
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("\(nextLabel): \(next)")
                    .accessibilityAddTraits(.isButton)
                }
            }
            .frame(maxWidth: .infinity, alignment: .trailing)
        }
    }
}

extension View {
    /**
     * A painter's reserve for an inscription: a feathered clearing of the wall's own colour behind
     * a line of lettering, so the powdering and the vault never run under the letters (the web's
     * `footer > p` on home and the hours: `radial-gradient(closest-side, var(--bg) 58%,
     * transparent)` over 0.6rem × 1.75rem of padding the margin takes back). It stands outside the
     * text's frame, so the foot keeps its height.
     */
    func reserve(_ ground: Color, _ m: Metrics) -> some View {
        background {
            EllipticalGradient(stops: [.init(color: ground, location: 0.58), .init(color: ground.opacity(0), location: 1)], center: .center, startRadiusFraction: 0, endRadiusFraction: 0.5)
                .padding(.horizontal, -m.px(28))
                .padding(.vertical, -m.px(9.6))
                .accessibilityHidden(true)
        }
    }
}

/**
 * The page's foot: the tailpiece that closes every page, the Office's name, and any `matter` a page
 * adds under it (an hour's report line). `gap` stands above it and `bottom` below, in the web's
 * px. On a page with a field on its wall (home, the hours) the name stands on a `reserve`. The
 * preferences are in the menu, or on a wide screen under Settings.
 */
struct Footer<Matter: View>: View {
    let gap: CGFloat
    let bottom: CGFloat
    let reserve: Bool
    let matter: Matter
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    init(gap: CGFloat = 53.6, bottom: CGFloat = 24, reserve: Bool = false, @ViewBuilder matter: () -> Matter) {
        self.gap = gap
        self.bottom = bottom
        self.reserve = reserve
        self.matter = matter()
    }

    var body: some View {
        VStack(spacing: 0) {
            Tailpiece()
            Text("Benedictine Divine Office").type(Scale.small).foregroundStyle(p.muted)
                .background { if reserve { Color.clear.reserve(p.bg, m) } }
                .padding(.top, m.px(8.8))
            matter
        }
        .frame(maxWidth: .infinity)
        .padding(.top, m.px(gap))
        .padding(.bottom, m.px(bottom))
    }
}

extension Footer where Matter == EmptyView {
    init(gap: CGFloat = 53.6, bottom: CGFloat = 24, reserve: Bool = false) {
        self.init(gap: gap, bottom: bottom, reserve: reserve) { EmptyView() }
    }
}

/// A message in place of a page: while it is prepared, or when it cannot be.
struct Message: View {
    let text: String
    @Environment(\.palette) private var p

    var body: some View {
        Text(text).type(Scale.body).foregroundStyle(p.muted).multilineTextAlignment(.center).padding(24)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}
