import SwiftUI
import UIKit

/// The reading measure's side gutter (`--page-gutter` on a phone).
let gutter: CGFloat = 16

/// The lines under the Martyrology and Chant rows, as the web's (`presentation`).
private let prefHints = preferenceHints()

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
/// Drawn as the web's glyph measures beside a label (5.5 by 4.5 at 0.85em, centred on the capitals), since the system face sets `▾` as a speck on the baseline.
struct Caret: View {
    let open: Bool
    /// The label's colour.
    let color: Color
    /// How much of it the caret takes: 0.7, or all of a colour of its own (the picker's gold).
    var share: Double = 0.7
    /// Its size against a label's: the web's `▾` at another size over 0.85em of the labels' 11.52px.
    var scale: CGFloat = 1
    @Environment(\.metrics) private var m

    var body: some View {
        CaretShape(open: open)
            .fill(color.opacity(share))
            .frame(width: 5.5 * scale * m.type, height: 4.5 * scale * m.type)
            .padding(.leading, 5.5 * scale * m.type)
            .accessibilityHidden(true)
    }
}

/// A solid triangle pointing down, or up when open.
struct CaretShape: Shape {
    let open: Bool

    func path(in r: CGRect) -> Path {
        var path = Path()
        if open {
            path.move(to: CGPoint(x: r.minX, y: r.maxY))
            path.addLine(to: CGPoint(x: r.maxX, y: r.maxY))
            path.addLine(to: CGPoint(x: r.midX, y: r.minY))
        } else {
            path.move(to: CGPoint(x: r.minX, y: r.minY))
            path.addLine(to: CGPoint(x: r.maxX, y: r.minY))
            path.addLine(to: CGPoint(x: r.midX, y: r.maxY))
        }
        path.closeSubpath()
        return path
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
            case .reminders, .find, .feast: model.open(.ordo(year: Int(model.today.year), month: Int(model.today.month), day: Int(model.today.day)))
            }
        }
        switch page {
        case .ordo, .year, .find, .feast: ordoCurrent = true
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
        // From 701 to 999pt an hour's links don't fit beside the brand and Settings: left in the
        // row they would crowd it, so they take a centred rank of their own beneath, as a book sets
        // its running head over the page.
        let rank = wide && ranked && nav.onHour != nil
        VStack(spacing: 0) {
            HStack(spacing: 0) {
                Button(action: model.goHome) {
                    HStack(spacing: 0) {
                        // The consecration roundel that ends every hour, 18pt. Centred on the
                        // row it sits on the text box's middle, 0.21em above the baseline
                        // (0.71em ascent, 0.29em descent); the capitals' middle is 0.325em
                        // above it. Raised by the difference, as the web's phone header
                        // raises it, it centres on the caps.
                        PaintedMark(.consecration, size: m.px(18), color: p.lining)
                            .offset(y: -m.px(14.4 * 0.115))
                            .padding(.trailing, m.px(6.08))
                        // A word space follows the mark's margin on a wide screen; the phone's brand is a
                        // flex box, whose words drop the space they begin with.
                        Text(wide ? " DAILY OFFICE" : "DAILY OFFICE").type(Scale.brand).foregroundStyle(p.text)
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
 * The site menu's panel: on an hour, the day's hours (2/3/2 as on home, a hairline between the
 * bands); the Ordo and Reminders, the current page underlined in the lining's terracotta; then
 * the Theme, Text and Martyrology rows, muted, the current choice in the accent over the gilding's
 * rule. `prefsOnly` is the wide header's Settings: the Theme, Text and Martyrology rows alone. Its
 * top edge is the painted line, which follows the season of the page shown.
 */
struct MenuPanel: View {
    var prefsOnly = false
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        let nav = SiteNav(model)
        let style = TextStyle.label(13.12, 0.06)
        VStack(spacing: 0) {
            if let onHour = nav.onHour, !prefsOnly {
                let bands = [["lauds", "prime"], ["terce", "sext", "none"], ["vespers", "compline"]]
                ForEach(Array(bands.enumerated()), id: \.offset) { i, row in
                    HStack(spacing: 0) {
                        ForEach(row, id: \.self) { h in link(hourLabel(h).uppercased(), h == nav.currentHour, style) { onHour(h) } }
                    }
                    // A hairline between the bands, so the grouping reads without labels.
                    .overlay(alignment: .top) { if i > 0 { Hairline(color: p.surfaceEdge) } }
                    .padding(.bottom, m.px(2.4))
                }
                Hairline(color: p.border).padding(.vertical, m.px(4))
            }
            if !prefsOnly {
                HStack(spacing: 0) {
                    link("ORDO", nav.ordoCurrent, style, action: nav.onOrdo)
                    // Habit setup, not an hour: quieter than the Ordo, as on the web.
                    link("REMINDERS", nav.remindersCurrent, .label(11.52, 0.04), action: nav.onReminders)
                }
                Hairline(color: p.border).padding(.top, m.px(6.4)).padding(.bottom, m.px(6.4))
            }
            HStack(spacing: 0) {
                rowLabel("THEME")
                ForEach(ThemeChoice.allCases) { t in
                    cell(t.label.uppercased(), t == model.theme, .label(12, 0.06), spoken: "\(t.label) theme") {
                        guard t != model.theme else { return }
                        chose()
                        withAnimation(restyling) { model.chooseTheme(t) }
                    }
                }
            }
            // The names are the church's, so a line says what they are, as on the web.
            hint(name("Nave") + Text(" is the church by day, and ") + name("Apse") + Text(" is its vault by night. ")
                + name("Default") + Text(" follows your device."))
            HStack(spacing: 0) {
                rowLabel("TEXT")
                ForEach(TextSize.allCases) { s in
                    cell("A", s == model.textSize, .label(s == .small ? 13 : s == .standard ? 16 : 20, 0), spoken: textSizeName(s)) {
                        guard s != model.textSize else { return }
                        chose()
                        withAnimation(restyling) { model.chooseTextSize(s) }
                    }
                }
            }
            .overlay(alignment: .top) { Hairline(color: p.surfaceEdge) }
            // The Martyrology at Prime: Off and On under the last two themes, the longer label
            // taking the first theme's cell, as on the web.
            HStack(spacing: 0) {
                Text("MARTYROLOGY").type(.label(10.56, 0.08)).foregroundStyle(p.muted)
                    .lineLimit(1).fixedSize()
                    .padding(.leading, m.px(10.4))
                    .frame(width: m.px(54), alignment: .leading)
                    .accessibilityHidden(true)
                Color.clear.frame(maxWidth: .infinity, minHeight: 44)
                ForEach([false, true], id: \.self) { on in
                    cell(on ? "ON" : "OFF", on == model.martyrology, .label(12, 0.06), spoken: "Martyrology at Prime \(on ? "on" : "off")") {
                        guard on != model.martyrology else { return }
                        chose()
                        model.chooseMartyrology(on)
                    }
                }
            }
            .overlay(alignment: .top) { Hairline(color: p.surfaceEdge) }
            hint(Text(prefHints.martyrology))
            // Chant, laid out as the Martyrology's row, once a hymn or Marian antiphon has a score.
            if model.offersChant {
                HStack(spacing: 0) {
                    Text("CHANT").type(.label(10.56, 0.08)).foregroundStyle(p.muted)
                        .lineLimit(1).fixedSize()
                        .padding(.leading, m.px(10.4))
                        .frame(width: m.px(54), alignment: .leading)
                        .accessibilityHidden(true)
                    Color.clear.frame(maxWidth: .infinity, minHeight: 44)
                    ForEach([false, true], id: \.self) { on in
                        cell(on ? "ON" : "OFF", on == model.chant, .label(12, 0.06), spoken: "Chant \(on ? "on" : "off")") {
                            guard on != model.chant else { return }
                            chose()
                            model.chooseChant(on)
                        }
                    }
                }
                .overlay(alignment: .top) { Hairline(color: p.surfaceEdge) }
                hint(Text(prefHints.chant))
            }
        }
        .padding(m.px(10.4))
        .frame(maxWidth: m.px(prefsOnly ? 288 : 336))
        .background(p.surface)
        .overlay(Rectangle().stroke(p.border, lineWidth: 1))
        // The panel's top edge is the painted line (`--ornament-line`).
        .overlay(alignment: .top) { Rectangle().fill(o.line).frame(height: 2) }
        // The web's shadow, in both themes: a light catching the edge under the painted line
        // (`inset 0 1px 0 var(--surface-edge)`), and a soft one cast below (`0 0.45rem 1rem`,
        // its blur twice a SwiftUI radius).
        .overlay(alignment: .top) { Rectangle().fill(p.surfaceEdge).frame(height: 1).padding(.top, 2).padding(.horizontal, 0.5) }
        .shadow(color: Color(rgb: 36, 28, 23, 0.09), radius: m.px(8), y: m.px(7.2))
    }

    /// A quiet italic line under a row whose names need saying (the web's `.pref-hint`).
    private func hint(_ t: Text) -> some View {
        t.type(TextStyle(size: 12.8, line: 17.3, italic: true)).foregroundStyle(p.muted)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, m.px(10.4))
            .padding(.top, m.px(7.2))
            .padding(.bottom, m.px(9.6))
    }

    /// A button's name in its hint, upright in small caps as the button is in capitals (the web's `.pref-name`).
    private func name(_ s: String) -> Text {
        Text(s).font(Font(garamond(12.8 * m.type, smallCaps: true))).tracking(0.05 * 12.8 * m.type)
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

    /// A setting's choice: muted, the chosen one in the accent over the gilding's rule (the web's
    /// `[aria-pressed="true"]`), marked as the current hour is on home.
    private func cell(_ label: String, _ current: Bool, _ style: TextStyle, spoken: String? = nil, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Text(label).type(style).foregroundStyle(current ? p.accent : p.muted)
                .frame(maxWidth: .infinity, minHeight: 44)
                .goldUnderline(current, o.flat)
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

/// How a disclosure opens, as on Android: over a quarter second, on Material's standard curve.
let unfolding = Animation.timingCurve(0.4, 0, 0.2, 1, duration: 0.26)

/// How home's date picker folds away, as Android's: a little quicker than it opened.
let folding = Animation.timingCurve(0.4, 0, 0.2, 1, duration: 0.2)

extension AnyTransition {
    /// A disclosure's contents, fading in as they drop a little from their control, and back.
    static var unfold: AnyTransition {
        .asymmetric(
            insertion: .opacity.animation(.easeOut(duration: 0.2).delay(0.06)).combined(with: .offset(y: -6)),
            removal: .opacity.animation(.easeIn(duration: 0.1)).combined(with: .offset(y: -6))
        )
    }
}

/// A small uppercase disclosure label with its caret ("Change date ▾"), muted, in the accent while open; its `value` in ink.
struct Disclosure: View {
    let label: String
    let open: Bool
    var value: String?
    var style: TextStyle = Scale.control
    let toggle: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let ink = open ? p.accent : p.muted
        Button(action: toggle) {
            HStack(spacing: 0) {
                (Text(label.uppercased()) + Text(value.map { " " + $0.uppercased() } ?? "").foregroundColor(p.text))
                    .type(style)
                    .foregroundStyle(ink)
                Caret(open: open, color: ink)
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
 * the choices, a week at most seven thumbs wide. Sundays are red; the chosen day is in the accent
 * over the gold underline, as a chosen prayer form is, and today takes a quiet wash. The title
 * turns the grid into the year's months, and back; both views keep six weeks' height, so turning
 * between them moves nothing below.
 */
struct DayPicker: View {
    let shown: CivilDate
    let today: CivilDate
    /// Previous · Today · Next gathered in the middle (home, and an hour wider than 540pt), or
    /// spread to the edges (an hour on a phone), as the web's `.day-nav`.
    let centred: Bool
    let pick: (CivilDate) -> Void
    @State private var year: Int
    @State private var month: Int
    @State private var months = false
    /// Which way the days last turned: 1 to a later month, -1 to an earlier.
    @State private var turn: CGFloat = 1
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    init(shown: CivilDate, today: CivilDate, centred: Bool, pick: @escaping (CivilDate) -> Void) {
        self.shown = shown
        self.today = today
        self.centred = centred
        self.pick = pick
        let y = Int(shown.year)
        _year = State(initialValue: min(max(y, pickerYears.lowerBound), pickerYears.upperBound))
        _month = State(initialValue: y < pickerYears.lowerBound ? 1 : y > pickerYears.upperBound ? 12 : Int(shown.month))
    }

    var body: some View {
        let link = Scale.body.sized(16, line: 24)
        // A day's cell, a thumb wide and high (`--date-cell`, the web's touch target).
        let cell = max(m.px(44), 44)
        VStack(spacing: 0) {
            Hairline(color: p.border).padding(.horizontal, m.px(24))
            HStack(spacing: centred ? m.px(5.6) : nil) {
                Button { pick(shown.adding(days: -1)) } label: { Text("← Previous").type(link).foregroundStyle(p.accent).padding(.horizontal, m.px(16)).frame(minHeight: 44) }
                    .buttonStyle(Quiet()).accessibilityLabel("Previous day")
                if !centred { Spacer() }
                if shown != today {
                    Button { pick(today) } label: { Text("TODAY").type(Scale.control).foregroundStyle(p.muted).frame(minHeight: 44) }.buttonStyle(Quiet())
                }
                if !centred { Spacer() }
                Button { pick(shown.adding(days: 1)) } label: { Text("Next →").type(link).foregroundStyle(p.accent).padding(.horizontal, m.px(16)).frame(minHeight: 44) }
                    .buttonStyle(Quiet()).accessibilityLabel("Next day")
            }
            .padding(.top, m.px(8))
            // Seven thumb-wide days where they fit, centred; on a narrower column the week narrows
            // to it, the days keeping their height.
            VStack(spacing: 0) {
                HStack(spacing: 0) {
                    // A month's step in the days, a year's in the months; none past the picker's span.
                    step("‹", back, months ? "Previous year" : "Previous month", width: cell)
                    Button { withAnimation(.easeInOut(duration: 0.18)) { months.toggle() } } label: {
                        // Set like the disclosure labels around it, with their gold caret.
                        HStack(spacing: 0) {
                            Text(title.uppercased()).type(.label(11.52, 0.1)).foregroundStyle(p.text)
                            Caret(open: months, color: p.goldLine, share: 1)
                        }
                        .frame(maxWidth: .infinity, minHeight: 44)
                    }
                    .buttonStyle(Quiet())
                    .accessibilityLabel(months ? "\(title), back to days" : "\(title), choose another month")
                    step("›", forward, months ? "Next year" : "Next month", width: cell)
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
                // A swipe across the days turns the month, as the arrows do.
                .simultaneousGesture(
                    DragGesture(minimumDistance: 24).onEnded { drag in
                        guard !months, abs(drag.translation.width) > max(48, abs(drag.translation.height) * 2) else { return }
                        turnTo(drag.translation.width < 0 ? forward : back)
                    }
                )
            }
            .frame(maxWidth: 7 * cell)
            .padding(.horizontal, m.px(8))
        }
        .padding(.top, m.px(8))
    }

    /// Today's wash, in the days and the months: the gold thinned, or by night the vault's blue lifted (`--apse-lift`).
    private var todayWash: Color { p.dark ? Color(hex: 0x84A0CC).opacity(0.13) : p.gold.opacity(0.1) }

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

    /// A step to the next or previous month (or year), a cell wide; at the picker's span's end, in the rules' ink.
    private func step(_ glyph: String, _ to: (Int, Int)?, _ label: String, width: CGFloat) -> some View {
        Button { turnTo(to) } label: {
            Text(glyph).type(Scale.body.sized(22, line: 28)).foregroundStyle(to == nil ? p.border : p.accent)
                .frame(width: width)
                .frame(minHeight: 44)
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
                ForEach(Array(["S", "M", "T", "W", "T", "F", "S"].enumerated()), id: \.offset) { i, d in
                    Text(d).type(.label(10.56, 0.1)).foregroundStyle(i == 0 ? p.rubric : p.muted).frame(maxWidth: .infinity).padding(.vertical, m.px(8))
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
                                // The chosen day in the accent over the gold rule; a Sunday stays red.
                                Text("\(n)").type(TextStyle(size: 18, line: 30, lining: true))
                                    .foregroundStyle(i == 0 ? p.rubric : day == shown ? p.accent : p.text)
                                    .goldUnderline(day == shown, p.goldLine)
                                    .frame(maxWidth: .infinity, minHeight: 44)
                                    .background(day == today ? todayWash : .clear)
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
                            // Back to the days, fading as the title's turn does, the month's days
                            // coming in the way it lies.
                            if mo != month { turn = mo > month ? 1 : -1 }
                            withAnimation(.easeInOut(duration: 0.18)) {
                                month = mo
                                months = false
                            }
                        } label: {
                            Text(monthShort(mo).uppercased()).type(.label(11.5, 0.1)).foregroundStyle(chosen ? p.accent : p.text)
                                .padding(.horizontal, m.px(14.4)).padding(.vertical, m.px(13))
                                .goldUnderline(chosen, p.goldLine)
                                .background(Int(today.year) == year && Int(today.month) == mo ? todayWash : .clear)
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

/// "How are you praying?": the three prayer forms, remembered on this device, as the web's
/// `.leader-selector` sets them: a small-capital legend over a left-aligned list at most 23rem wide,
/// ruled above, below and between; the chosen form's radio filled in the accent within a ring of
/// the ground, its words in the accent over the gold rule.
struct FormChooser: View {
    let form: String
    let choose: (String) -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text("How are you praying?".uppercased()).type(.label(11.2, 0.1)).foregroundStyle(p.muted)
                .padding(.bottom, m.px(10.4))
                .accessibilityLabel("How are you praying?")
                .accessibilityAddTraits(.isHeader)
            Hairline(color: p.border)
            ForEach(Array(prayerForms.enumerated()), id: \.offset) { i, f in
                let on = f.value == form
                if i > 0 { Hairline(color: p.surfaceEdge) }
                Button {
                    if !on { chose() }
                    choose(f.value)
                } label: {
                    HStack(spacing: m.px(12.8)) {
                        ZStack {
                            Circle().fill(on ? p.accent : p.bg)
                            if on { Circle().strokeBorder(p.bg, lineWidth: 3).padding(1) }
                            Circle().strokeBorder(on ? p.accent : p.muted, lineWidth: 1)
                        }
                        .frame(width: m.px(14.4), height: m.px(14.4))
                        Text(f.phrase).type(TextStyle(size: 14.4, line: 20.16))
                            .foregroundStyle(on ? p.accent : p.text)
                            .goldUnderline(on, p.goldLine)
                            .fixedSize(horizontal: false, vertical: true)
                    }
                    .padding(.vertical, m.px(10.4))
                    .padding(.horizontal, m.px(5.6))
                    .frame(maxWidth: .infinity, minHeight: 44, alignment: .leading)
                }
                .buttonStyle(Quiet())
                .accessibilityAddTraits(on ? .isSelected : [])
            }
            Hairline(color: p.border)
            Text("Remembered on this device. Change it any time.").type(TextStyle(size: 12.8, line: 18.56)).foregroundStyle(p.muted)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.top, m.px(11.2))
        }
        .frame(maxWidth: m.px(368), alignment: .leading)
        .frame(maxWidth: .infinity)
        .padding(.top, m.px(12))
        .padding(.bottom, m.px(8))
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
