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
 * The web's controls mark state, not touches; a pressed control only dims a little (Tempo's
 * `touchQuiet`, the web's bare words), at once, and comes back over Tempo's touch when let go, so
 * even the briefest tap is seen to land.
 */
struct Quiet: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .contentShape(Rectangle())
            .opacity(configuration.isPressed ? Tempo.touchQuiet : 1)
            .animation(configuration.isPressed ? nil : Tempo.touchOut.animation, value: configuration.isPressed)
    }
}

/**
 * Light under the finger: a pressed row (home's hours, the ordo's days, the picker's days, About's
 * hours, the menu's cells) lies under home's pool of light, the edgeless pool that marks the hour
 * that is now, at once, and lets it go over Tempo's touch. Bare words keep `Quiet`. Nothing
 * scales. `pool` is the row's pool at full strength, drawn behind the label; `strength` how much of
 * it a press shows (home's hours take 70% by day, the moonlit pool being faint by night). The hour
 * that is now is lit already (`current`): a press deepens its pool by half again and its words
 * withdraw to 70% (the web's `.home-hour-link.is-current:active`).
 */
struct Lit<Pool: View>: ButtonStyle {
    var strength: Double = 1
    var current = false
    /// The pool's light at rest, when it is neither the hour that is now's nor out: home's light
    /// moving with the hours sets it frame by frame (`HourLightNow`).
    var resting: Double? = nil
    @ViewBuilder var pool: () -> Pool

    func makeBody(configuration: Configuration) -> some View {
        let held = configuration.isPressed
        let rest = resting ?? (current ? 1 : 0)
        return configuration.label
            .opacity(current && held ? Tempo.touchQuiet : 1)
            .background {
                ZStack {
                    pool().opacity(held ? max(rest, current ? 1 : strength) : rest)
                    if current { pool().opacity(held ? 0.5 : 0) }
                }
                .allowsHitTesting(false)
                .accessibilityHidden(true)
            }
            .contentShape(Rectangle())
            .animation(held ? nil : Tempo.touchOut.animation, value: held)
    }
}

extension Lit where Pool == PoolOfLight {
    /// A row whose pool fills it, centred, with no edge.
    init(strength: Double = 1, current: Bool = false) {
        self.init(strength: strength, current: current) { PoolOfLight() }
    }
}

/**
 * A part of a row of several targets (the ordo's date and feast) that lights the whole row: it
 * tells the row it is held, and the row draws its pool (`RowPool`) as `Lit` would. It does not dim.
 */
struct Holding: ButtonStyle {
    @Binding var held: Bool

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .contentShape(Rectangle())
            .onChange(of: configuration.isPressed, initial: false) { _, now in held = now }
    }
}

extension View {
    /// A row lit while `held`, at once, let go over Tempo's touch: a long lozenge of home's pool
    /// under its words (the web's `ellipse 44% 58%`), centred at `x` across it.
    func rowPool(_ held: Bool, x: CGFloat) -> some View {
        background {
            PoolOfLight(at: UnitPoint(x: x, y: 0.5), size: CGSize(width: 0.88, height: 1.16))
                .opacity(held ? 1 : 0)
                .animation(held ? nil : Tempo.touchOut.animation, value: held)
                .allowsHitTesting(false)
                .accessibilityHidden(true)
        }
    }
}

/**
 * Home's pool of light, the web's `radial-gradient(closest-side, pool, pool 55% at 52%,
 * transparent)`: an ellipse with no edge filling its frame, ochre by day, moonlit blue by night.
 * `at` and `size` place a long lozenge in a row (the web's `ellipse 44% 58% at 30% 50%`).
 */
struct PoolOfLight: View {
    var at: UnitPoint = .center
    /// The ellipse's width and height as fractions of the frame's.
    var size: CGSize = CGSize(width: 1, height: 1)
    @Environment(\.palette) private var p

    var body: some View {
        let pool = HomeInk.of(p).nowPool
        GeometryReader { g in
            let w = g.size.width * size.width
            let h = g.size.height * size.height
            Ellipse()
                .fill(EllipticalGradient(
                    stops: [.init(color: pool, location: 0), .init(color: pool.opacity(0.55), location: 0.52), .init(color: pool.opacity(0), location: 1)],
                    center: .center, startRadiusFraction: 0, endRadiusFraction: 0.5
                ))
                .frame(width: w, height: h)
                .position(x: g.size.width * at.x, y: g.size.height * at.y)
        }
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
 * hour page, then the Ordo, Reminders and About. The menu sets it out on a phone, the header
 * inline on a wide screen.
 */
struct SiteNav {
    let hours: [String]
    let currentHour: String?
    let onHour: ((String) -> Void)?
    let onOrdo: () -> Void
    let ordoCurrent: Bool
    let onReminders: () -> Void
    let remindersCurrent: Bool
    let onAbout: () -> Void
    let aboutCurrent: Bool

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
            case .reminders, .about, .find, .feast: model.open(.ordo(year: Int(model.today.year), month: Int(model.today.month), day: Int(model.today.day)))
            }
        }
        switch page {
        case .ordo, .year, .find, .feast: ordoCurrent = true
        default: ordoCurrent = false
        }
        onReminders = { model.open(.reminders) }
        remindersCurrent = page == .reminders
        onAbout = { model.open(.about) }
        aboutCurrent = page == .about
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
                    Button { model.toggle(.menu) } label: {
                        HStack(spacing: 0) {
                            Text("MENU").type(Scale.menu).foregroundStyle(p.accent)
                            Caret(open: model.overlay == .menu, color: p.accent)
                        }
                        .padding(.leading, m.px(12.8))
                        .padding(.trailing, m.px(3.2))
                        .frame(minHeight: 44)
                    }
                    .buttonStyle(Quiet())
                    .accessibilityLabel("Menu")
                    .accessibilityValue(model.overlay == .menu ? "Expanded" : "Collapsed")
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

/// The desktop header's links (`.site-menu nav`), muted: the current one in ink over the lining's terracotta; Reminders and About quieter.
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
                // An hour's header is already full with the seven hours, so About stays out of
                // it, as on the web; the phone's menu keeps it everywhere.
                if nav.onHour == nil {
                    link(aboutPage.menu, nav.aboutCurrent, secondary: true, action: nav.onAbout)
                }
            }
            // Settings closes the links, quiet as Reminders; its panel holds the theme and text size.
            if settings {
                Button { model.toggle(.settings) } label: {
                    HStack(spacing: 0) {
                        Text("SETTINGS").type(.label(11.52, 0.04)).foregroundStyle(p.muted)
                        Caret(open: model.overlay == .settings, color: p.muted)
                    }
                    .padding(.leading, m.px(8.8))
                    .padding(.trailing, m.px(3.2))
                    .frame(minHeight: 44)
                }
                .buttonStyle(Quiet())
                .accessibilityLabel("Settings")
                .accessibilityValue(model.overlay == .settings ? "Expanded" : "Collapsed")
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
 * bands); the Ordo, Reminders and About, the current page underlined in the lining's terracotta;
 * then the Theme, Text and Martyrology rows, muted, the current choice in the accent over the
 * gilding's rule. `prefsOnly` is the wide header's Settings: the Theme, Text and Martyrology rows
 * alone. Its top edge is the painted line, which follows the season of the page shown.
 */
struct MenuPanel: View {
    var prefsOnly = false
    @EnvironmentObject private var model: AppModel
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

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
                    // Habit setup and the page for newcomers, not hours: quieter than the Ordo, as on the web.
                    link("REMINDERS", nav.remindersCurrent, .label(11.52, 0.04), action: nav.onReminders)
                }
                // About in the first half of a row of its own, as the web's grid and Android set it.
                HStack(spacing: 0) {
                    link(aboutPage.menu.uppercased(), nav.aboutCurrent, .label(11.52, 0.04), action: nav.onAbout)
                    Color.clear.frame(maxWidth: .infinity, maxHeight: 0)
                }
                .padding(.top, m.px(2.4))
                Hairline(color: p.border).padding(.top, m.px(6.4)).padding(.bottom, m.px(6.4))
            }
            HStack(spacing: 0) {
                rowLabel("THEME")
                ForEach(ThemeChoice.allCases) { t in
                    cell(t.label.uppercased(), t == model.theme, .label(12, 0.06), spoken: "\(t.label) theme") {
                        guard t != model.theme else { return }
                        chose()
                        withAnimation(reduceMotion ? nil : restyling) { model.chooseTheme(t) }
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
                        // At once, as on the web: the prayer is set again at its new size in one
                        // frame, never seen re-wrapping, and the hour holds the reader's row.
                        model.chooseTextSize(s)
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
        .background {
            // By night the menu opens on home's blue plaster (the phone panel's baked Apse plaster),
            // not a flat slab of the surface: the web's `.site-menu-panel` by night.
            if p.dark, let plaster = UIImage(named: p.panel) {
                Image(uiImage: plaster).resizable().scaledToFill()
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
                    .clipped()
            } else {
                p.surface
            }
        }
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
        .buttonStyle(Lit())
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
        .buttonStyle(Lit())
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
 * A light tick under the finger as a choice is made (a setting, a prayer form, a day, a month
 * swiped), as the Android app gives; never for a page opened. The phone's own haptics setting
 * governs it.
 */
func chose() {
    Haptics.selection.selectionChanged()
    Haptics.selection.prepare()
}

/// The tap of a checkbox turned on, firm, or off, softer: Android's ToggleOn and ToggleOff.
func toggled(_ on: Bool) {
    if on {
        Haptics.on.impactOccurred()
        Haptics.on.prepare()
    } else {
        Haptics.off.impactOccurred(intensity: 0.7)
        Haptics.off.prepare()
    }
}

/**
 * The generators, made once and kept: the Taptic Engine sleeps until a generator is prepared, so
 * one made for each tap could land its tick after the touch it answers. Each is prepared again
 * after it plays, for the next choice in the same breath.
 */
private enum Haptics {
    static let selection: UISelectionFeedbackGenerator = {
        let g = UISelectionFeedbackGenerator()
        g.prepare()
        return g
    }()
    static let on = UIImpactFeedbackGenerator(style: .light)
    static let off = UIImpactFeedbackGenerator(style: .soft)
}

/// How the room changes when the reader changes how it looks: dimming rather than snapping, as the web's theme does
/// (and, under Reduce Motion, at once, as the web's and Android's then do).
let restyling = Tempo.dimIn.animation

/**
 * Brings what a disclosure unfolded into view when it opened past the screen's edge: a page's
 * scroll view sets it (`revealing`), and the disclosure names its contents' id, calling it once
 * the unfolding has finished (`withAnimation`'s completion), as Android waits on the
 * transition's own state rather than a clock. `alongside` scrolls there in the caller's own
 * transaction instead, so a disclosure that unfolds under `withAnimation` and scrolls to its foot
 * moves the page once, on the unfolding's curve, as home's "Change date" does on the web and
 * Android.
 */
struct Reveal {
    fileprivate let scroll: (String) -> Void
    fileprivate let now: (String) -> Void
    func callAsFunction(_ id: String) { scroll(id) }
    func alongside(_ id: String) { now(id) }
}

private struct RevealKey: EnvironmentKey {
    static let defaultValue = Reveal(scroll: { _ in }, now: { _ in })
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
        environment(\.reveal, Reveal(
            scroll: { id in
                // Under Reduce Motion the page is simply there, as the web's and Android's are.
                withAnimation(UIAccessibility.isReduceMotionEnabled ? nil : .easeInOut(duration: 0.3)) {
                    proxy.scrollTo(id, anchor: .bottom)
                }
            },
            now: { id in proxy.scrollTo(id, anchor: .bottom) }
        ))
    }
}

/// How a disclosure opens, as on Android: over a quarter second, on the standard curve.
let unfolding = Tempo.unfoldIn.animation

/// How a disclosure folds away, as Android's and the web's: a little quicker than it opened.
let folding = Tempo.unfoldOut.animation

/// A disclosure's turn: `unfolding` as it opens and `folding` as it closes, the web's and
/// Android's one curve; under Reduce Motion none, so it is simply open or shut, as the web's and
/// Android's then are.
func disclosing(_ opening: Bool) -> Animation? {
    UIAccessibility.isReduceMotionEnabled ? nil : (opening ? unfolding : folding)
}

extension AnyTransition {
    /// A disclosure's contents, fading in over 200ms after 60 as they drop a little from their
    /// control, and out over 100ms as they fold back (the web's and Android's fades); under Reduce
    /// Motion they are simply there or gone, as the web's and Android's folds then cut.
    static var unfold: AnyTransition {
        if UIAccessibility.isReduceMotionEnabled { return .identity }
        return .asymmetric(
            insertion: .opacity.animation(Beat(ms: Tempo.unfoldIn.ms - 60, delay: 60, curve: .arrive).animation).combined(with: .offset(y: -6)),
            removal: .opacity.animation(Beat(ms: Tempo.unfoldOut.ms / 2, delay: 0, curve: .leave).animation).combined(with: .offset(y: -6))
        )
    }
}

/// A small uppercase disclosure label with its caret ("Change date ▾"), muted, in the accent while open; its `value` in ink.
struct Disclosure: View {
    let label: String
    let open: Bool
    var value: String?
    var style: TextStyle = Scale.control
    /// Set in capitals, as the controls are; home's foot sets it in small caps instead.
    var upper = true
    let toggle: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let ink = open ? p.accent : p.muted
        let cased = { (s: String) in upper ? s.uppercased() : s }
        Button(action: toggle) {
            HStack(spacing: 0) {
                (Text(cased(label)) + Text(value.map { " " + cased($0) } ?? "").foregroundColor(p.text))
                    .type(style)
                    .foregroundStyle(ink)
                    // Home's and an hour's small-caps disclosures keep one line.
                    .fixedSize(horizontal: !upper, vertical: false)
                Caret(open: open, color: ink)
            }
            .padding(.horizontal, m.px(8))
            .frame(minHeight: 44)
        }
        .buttonStyle(Quiet())
        .accessibilityValue(open ? "Expanded" : "Collapsed")
    }
}

/// Change date's day links, worded as the web's.
private let dayNav = dayNavWords()

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
    /// The days' width, of which a turning month travels a fifth, as Android's does.
    @State private var daysWidth: CGFloat = 0
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
            // "Previous day" and "Next day", worded (`presentation::PREVIOUS_DAY`), so they read as
            // the day's turn and the month's arrows below as the month's; gathered, they stand at
            // the grid's edges.
            HStack(spacing: 0) {
                Button { pick(shown.adding(days: -1)) } label: { Text("← \(dayNav.previous)").type(link).foregroundStyle(p.accent).padding(.horizontal, m.px(centred ? 4 : 16)).frame(minHeight: 44) }
                    .buttonStyle(Quiet()).accessibilityLabel(dayNav.previous)
                Spacer(minLength: 0)
                if shown != today {
                    Button { pick(today) } label: { Text("TODAY").type(Scale.control).foregroundStyle(p.muted).frame(minHeight: 44) }.buttonStyle(Quiet())
                }
                Spacer(minLength: 0)
                Button { pick(shown.adding(days: 1)) } label: { Text("\(dayNav.next) →").type(link).foregroundStyle(p.accent).padding(.horizontal, m.px(centred ? 4 : 16)).frame(minHeight: 44) }
                    .buttonStyle(Quiet()).accessibilityLabel(dayNav.next)
            }
            .frame(maxWidth: centred ? 7 * cell : .infinity)
            .padding(.top, m.px(8))
            // A hairline parts the day's turn above from the month's below.
            Hairline(color: p.border.opacity(0.58)).frame(maxWidth: 7 * cell).padding(.top, m.px(4))
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
                    // A new month's days slide in the way it turned, a fifth of their width, as
                    // Android's do (`it / 5`).
                    let travel = daysWidth > 0 ? daysWidth / 5 : 60
                    days
                        .id(year * 12 + month)
                        .transition(reduceMotion ? .opacity : .asymmetric(
                            insertion: .offset(x: travel * turn).combined(with: .opacity.animation(.easeOut(duration: 0.18).delay(0.06))),
                            removal: .offset(x: -travel * turn).combined(with: .opacity.animation(.easeIn(duration: 0.09)))
                        ))
                        .opacity(months ? 0 : 1)
                        .accessibilityHidden(months)
                    if months { monthGrid.transition(.opacity) }
                }
                .clipped()
                .onGeometryChange(for: CGFloat.self) { $0.size.width } action: { daysWidth = $0 }
                // A swipe across the days turns the month, as the arrows do, with a tick as it
                // commits: the one turn with no control under the finger (Android's too).
                .simultaneousGesture(
                    DragGesture(minimumDistance: 24).onEnded { drag in
                        guard !months, abs(drag.translation.width) > max(48, abs(drag.translation.height) * 2),
                              let to = drag.translation.width < 0 ? forward : back else { return }
                        chose()
                        turnTo(to)
                    }
                )
            }
            .frame(maxWidth: 7 * cell)
            .padding(.horizontal, m.px(8))
        }
        .padding(.top, m.px(8))
    }

    /// Today, in the days and the months, lies under a pool of light with no edge, as the hour
    /// that is now does on home (`--now-pool`): ochre by day, moonlit blue by night.
    private var todayPool: some View {
        let pool = HomeInk.of(p).nowPool
        return RadialGradient(stops: [.init(color: pool, location: 0), .init(color: pool.opacity(0.55), location: 0.52), .init(color: pool.opacity(0), location: 1)], center: .center, startRadius: 0, endRadius: 22)
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
                                    .background { if day == today { todayPool } }
                            }
                            .buttonStyle(Lit())
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
                                .background { if Int(today.year) == year && Int(today.month) == mo { todayPool } }
                        }
                        .buttonStyle(Lit())
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
 * the next item. `ruled` sets it as home sets its hours (an hour's end, the web's
 * `.page-hour .hour-continuation`): labels in true small capitals, the names a size up, home's gilt
 * points between the three, and one rule struck under them with home's pen where the wall's field
 * begins.
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
    var ruled = false
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    private func label(_ text: String) -> String { ruled ? text : text.uppercased() }

    var body: some View {
        let small = ruled ? TextStyle(size: 13.76, line: 18, tracking: 1.1, smallCaps: true) : TextStyle.label(11.2, 0.08)
        let name = Scale.body.sized(ruled ? 16.96 : 16, line: 22)
        HStack(alignment: .center, spacing: 0) {
            Group {
                if let previous {
                    Button(action: onPrevious) {
                        VStack(alignment: .leading, spacing: 0) {
                            Text(label(previousLabel)).type(small).foregroundStyle(p.muted)
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
            if ruled && previous != nil { GiltPoint() }
            Button(action: onMiddle) {
                Text(label(middle)).type(ruled ? small : .label(12.16, 0.06)).foregroundStyle(p.muted)
                    .padding(.horizontal, m.px(ruled ? 12 : 6.4)).padding(.vertical, m.px(12))
            }
            .buttonStyle(Quiet())
            .accessibilityLabel(middle)
            if ruled && next != nil { GiltPoint() }
            Group {
                if let next {
                    Button(action: onNext) {
                        VStack(alignment: .trailing, spacing: 0) {
                            Text(label(nextLabel)).type(small).foregroundStyle(p.muted)
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
        .padding(.top, ruled ? m.px(8.8) : 0).padding(.bottom, ruled ? m.px(9.6) : 0)
        .background {
            if ruled {
                Canvas { ctx, size in
                    ctx.fill(ruledLine(0, size.height - 0.5, size.width, size.height - 0.5, 1.2, seed: 51), with: .color(p.lining.opacity(0.55)))
                }
                .accessibilityHidden(true)
            }
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

/// "Preparing the office…", only when composing takes long enough to be noticed: it waits 400ms
/// and then fades in over 200, so a quick open or cold start never flashes it (Android's `Preparing`).
struct Preparing: View {
    @State private var shown = false

    var body: some View {
        Message(text: "Preparing the office…")
            .opacity(shown ? 1 : 0)
            .accessibilityHidden(!shown)
            .task {
                try? await Task.sleep(for: .milliseconds(400))
                guard !Task.isCancelled else { return }
                withAnimation(.easeOut(duration: 0.2)) { shown = true }
            }
    }
}
