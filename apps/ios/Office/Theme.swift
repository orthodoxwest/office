import CoreText
import SwiftUI
import UIKit

/**
 * The web's design tokens (apps/office-web/static/style.css), as the Android app's `Theme.kt`
 * holds them: Nave is the stylesheet's `:root`, Apse its `[data-theme="dark"]` block.
 */
struct Palette: Equatable {
    let dark: Bool
    let text: Color
    let bg: Color
    let accent: Color
    let gold: Color
    let goldLine: Color
    let muted: Color
    /** Words not said aloud: secret prayer, a psalm's opening its antiphon has just said. */
    let unsaid: Color
    let border: Color
    let rubric: Color
    // The painted ornament, in the earth pigments of English wall painting: terracotta for rules,
    // linings and crosses, opaque and the same in every season; red ochre for the tituli (section
    // headings, psalm numbers, "Ant."), gilt on the Apse; slate blue for the ordo's doubles.
    let lining: Color
    let titulus: Color
    let kalendarBlue: Color
    /// The evening hours' moon: the gold on the Nave, silver on the Apse night.
    let moonInk: Color
    let surface: Color
    let surfaceEdge: Color
    let pressedWash: Color
    /// The header's beam, in both themes, and the light catching a timber's upper edge.
    let oak: Color
    let materialHighlight: Color
    let inscriptionGround: Color
    let inscriptionEdge: Color
    let inscriptionWash: Color
    /** The plaster wall behind every page, baked from the web's layers (apps/android/tools/bake-plaster.py). */
    let plaster: String

    static let nave = Palette(
        dark: false,
        text: Color(hex: 0x241C17),
        bg: Color(hex: 0xFAF3E9),
        accent: Color(hex: 0x6B3A1F),
        gold: Color(hex: 0x9A7328),
        goldLine: Color(hex: 0xC9AC72),
        muted: Color(hex: 0x6B5D54),
        unsaid: Color(hex: 0x786B62),
        border: Color(hex: 0xDCCFC3),
        rubric: Color(hex: 0x8B1A1A),
        lining: Color(hex: 0xA85A48),
        titulus: Color(hex: 0x93412C),
        kalendarBlue: Color(hex: 0x34507A),
        moonInk: Color(hex: 0x9A7328),
        surface: Color(hex: 0xF6EDDF),
        surfaceEdge: Color(rgb: 107, 58, 31, 0.12),
        pressedWash: Color(rgb: 107, 58, 31, 0.09),
        oak: Color(hex: 0x3F372F),
        materialHighlight: Color(rgb: 255, 255, 255, 0.38),
        inscriptionGround: Color(hex: 0x545F54),
        inscriptionEdge: Color(hex: 0x5E2A27),
        inscriptionWash: Color(rgb: 87, 94, 65, 0.055),
        plaster: "plaster_nave.jpg"
    )

    static let apse = Palette(
        dark: true,
        text: Color(hex: 0xDDD6C3),
        bg: Color(hex: 0x121C28),
        accent: Color(hex: 0xD0B06A),
        gold: Color(hex: 0xD8BC74),
        goldLine: Color(hex: 0x6A5C3A),
        muted: Color(hex: 0x9AA4B0),
        unsaid: Color(hex: 0x9AA4B0),
        border: Color(hex: 0x2A3648),
        rubric: Color(hex: 0xD47070),
        lining: Color(hex: 0xCF8C6A),
        titulus: Color(hex: 0xD8BC74),
        kalendarBlue: Color(hex: 0xA9BEDF),
        moonInk: Color(hex: 0xC9D0D9),
        surface: Color(hex: 0x172232),
        surfaceEdge: Color(rgb: 208, 176, 106, 0.18),
        pressedWash: Color(rgb: 208, 176, 106, 0.12),
        oak: Color(hex: 0x3F372F),
        materialHighlight: Color(rgb: 208, 176, 106, 0.065),
        inscriptionGround: Color(hex: 0x263431),
        inscriptionEdge: Color(hex: 0x5C2B35),
        inscriptionWash: Color(rgb: 208, 176, 106, 0.04),
        plaster: "plaster_apse.jpg"
    )
}

extension Color {
    init(hex: UInt32) {
        self.init(.sRGB, red: Double((hex >> 16) & 0xFF) / 255, green: Double((hex >> 8) & 0xFF) / 255, blue: Double(hex & 0xFF) / 255)
    }

    init(rgb r: Double, _ g: Double, _ b: Double, _ alpha: Double) {
        self.init(.sRGB, red: r / 255, green: g / 255, blue: b / 255, opacity: alpha)
    }
}

/**
 * The gilding, which alone follows the season ("Seasonal ornament" in style.css): gold leaf
 * through most of the year, veiled in Passiontide, warmed at Eastertide. Functional gold
 * (controls, selections) stays `Palette.gold`. The painted rules are the lining's terracotta,
 * so `line` is `Palette.lining` except where a season gilds or veils it; the church veils its
 * images, not its walls.
 */
struct Ornament: Equatable {
    let flat: Color
    let line: Color
    let hi: Color
    let lo: Color
    let ink: Color

    static func of(_ p: Palette, season: String) -> Ornament {
        switch (season, p.dark) {
        case ("passiontide", true): return Ornament(flat: Color(hex: 0xB0A4C2), line: Color(hex: 0x565070), hi: Color(hex: 0xCAC1D7), lo: Color(hex: 0x988AAD), ink: Color(hex: 0xDDD6E8))
        case ("passiontide", false): return Ornament(flat: Color(hex: 0x756A7E), line: Color(hex: 0xB5AABD), hi: Color(hex: 0x8D8395), lo: Color(hex: 0x605469), ink: Color(hex: 0xDDD6E8))
        case ("eastertide", true): return Ornament(flat: Color(hex: 0xE6CF8C), line: Color(hex: 0x7A6A44), hi: Color(hex: 0xF2E2B1), lo: Color(hex: 0xD3B86F), ink: Color(hex: 0xF2E2B1))
        case ("eastertide", false): return Ornament(flat: Color(hex: 0xA4731A), line: Color(hex: 0xD0B06C), hi: Color(hex: 0xC8922D), lo: Color(hex: 0x886011), ink: Color(hex: 0xF2E2B1))
        case (_, true): return Ornament(flat: p.gold, line: p.lining, hi: Color(hex: 0xE7D295), lo: Color(hex: 0xC0A25A), ink: Color(hex: 0xECD9A0))
        default: return Ornament(flat: p.gold, line: p.lining, hi: Color(hex: 0xB98D3C), lo: Color(hex: 0x7D5C1C), ink: Color(hex: 0xECD9A0))
        }
    }
}

/// The liturgical colours of the ordo rails and the band across an hour's top (`.day-color-*`).
func dayColor(_ name: String) -> Color {
    switch name {
    case "red": return Color(hex: 0xB02A24)
    case "green": return Color(hex: 0x3A6B3A)
    case "violet": return Color(hex: 0x6A3A8A)
    case "black": return Color(hex: 0x3A3A3A)
    case "rose": return Color(hex: 0xC4607A)
    default: return Color(hex: 0xC9B896)
    }
}

/// A day's colour as home sets it: on the Apse night white and black are lifted, as `.day-color-*` is.
func dayColor(_ name: String, _ p: Palette) -> Color {
    switch (name, p.dark) {
    case ("white", true): return Color(hex: 0xD0B06A)
    case ("black", true): return Color(hex: 0x8A94A0)
    default: return dayColor(name)
    }
}

/// The menu's Theme row: Default follows the device; Nave and Apse are the web's names.
enum ThemeChoice: String, CaseIterable, Identifiable {
    case system = "default", nave, apse

    var id: String { rawValue }
    var label: String { self == .system ? "Default" : rawValue.capitalized }

    func dark(_ system: ColorScheme) -> Bool {
        switch self {
        case .system: return system == .dark
        case .nave: return false
        case .apse: return true
        }
    }

    /// The window's own light or dark, so the status bar and system controls match the page.
    var scheme: ColorScheme? {
        switch self {
        case .system: return nil
        case .nave: return .light
        case .apse: return .dark
        }
    }
}

/// The menu's Text row, scaling the whole page as the web scales its root (93%, 100%, 110%).
enum TextSize: String, CaseIterable, Identifiable {
    case small, standard = "default", large

    var id: String { rawValue }
    var scale: CGFloat {
        switch self {
        case .small: return 0.93
        case .standard: return 1
        case .large: return 1.1
        }
    }
}

/**
 * How large the page is set. `layout` scales every measure, as the web's root size and the menu's
 * Text row do; `type` scales the text alone, adding the reader's Dynamic Type size on top, as the
 * Android app keeps the phone's font scale apart from its own.
 */
struct Metrics: Equatable {
    var layout: CGFloat = 1
    var type: CGFloat = 1

    /// A length in the web's CSS pixels at a phone's width.
    func px(_ v: CGFloat) -> CGFloat { v * layout }

    init(textSize: TextSize = .standard, dynamicType: DynamicTypeSize = .large) {
        layout = textSize.scale
        type = textSize.scale * Metrics.factor(dynamicType)
    }

    /// The system body size at each Dynamic Type setting, over its default (Large, 17pt).
    static func factor(_ size: DynamicTypeSize) -> CGFloat {
        let body: CGFloat
        switch size {
        case .xSmall: body = 14
        case .small: body = 15
        case .medium: body = 16
        case .large: body = 17
        case .xLarge: body = 19
        case .xxLarge: body = 21
        case .xxxLarge: body = 23
        case .accessibility1: body = 28
        case .accessibility2: body = 33
        case .accessibility3: body = 40
        case .accessibility4: body = 47
        case .accessibility5: body = 53
        @unknown default: body = 17
        }
        // Garamond at 20px is already a large face; past twice that the measure breaks every line.
        return min(body / 17, 2)
    }
}

/**
 * A text style from the type scale, in the web's CSS pixels at a phone's width: EB Garamond 12,
 * with small capitals, lining figures and tracking where the web sets them.
 */
struct TextStyle: Equatable {
    var size: CGFloat
    var line: CGFloat
    var tracking: CGFloat = 0
    var smallCaps = false
    var lining = false
    var italic = false

    /// The face at `k` times its size, with its OpenType features turned on.
    func uiFont(_ k: CGFloat = 1) -> UIFont {
        garamond(size * k, italic: italic, smallCaps: smallCaps, lining: lining)
    }

    func uiFont(_ m: Metrics) -> UIFont { uiFont(m.type) }

    func sized(_ size: CGFloat, line: CGFloat? = nil) -> TextStyle {
        var s = self
        s.size = size
        s.line = line ?? self.line * size / self.size
        return s
    }

    /// This style with `tracking` px between letters.
    func tracked(_ tracking: CGFloat) -> TextStyle {
        var s = self
        s.tracking = tracking
        return s
    }

    /// Uppercase working labels: "Change date", "Menu", continuation labels.
    static func label(_ size: CGFloat, _ tracking: CGFloat) -> TextStyle {
        TextStyle(size: size, line: size * 1.6, tracking: size * tracking)
    }
}

/// EB Garamond 12 at `size`. Regular and Italic only: its Bold was never finished, and the Office sets nothing bold.
func garamond(_ size: CGFloat, italic: Bool = false, smallCaps: Bool = false, lining: Bool = false) -> UIFont {
    let base = UIFont(name: italic ? "EBGaramond12-Italic" : "EBGaramond12-Regular", size: size) ?? UIFont.systemFont(ofSize: size)
    var features: [[UIFontDescriptor.FeatureKey: Int]] = []
    if smallCaps {
        features.append([.type: kLowerCaseType, .selector: kLowerCaseSmallCapsSelector])
        features.append([.type: kUpperCaseType, .selector: kUpperCaseSmallCapsSelector])
    }
    if lining {
        features.append([.type: kNumberCaseType, .selector: kUpperCaseNumbersSelector])
    }
    if features.isEmpty { return base }
    return UIFont(descriptor: base.fontDescriptor.addingAttributes([.featureSettings: features]), size: size)
}

/// The ✠, from the Noto Sans Symbols subset the web and the Android app use: the Garamond cut has no cross.
func crossUIFont(_ size: CGFloat) -> UIFont {
    UIFont(name: "NotoSansSymbols-Bold", size: size) ?? UIFont.systemFont(ofSize: size)
}

/// The type scale (the Android app's `Type`).
enum Scale {
    static let body = TextStyle(size: 20, line: 32)
    static let bodyItalic = TextStyle(size: 20, line: 32, italic: true)
    static let verse = TextStyle(size: 20, line: 33)
    static let rubric = TextStyle(size: 18, line: 27)
    static let heading = TextStyle(size: 19.2, line: 24.96, tracking: 2.112, smallCaps: true)
    static let itemLabel = TextStyle(size: 20, line: 27, tracking: 1.4, smallCaps: true, lining: true)
    static let reference = TextStyle(size: 17, line: 23.8)
    static let verseNumber = TextStyle(size: 15.6, line: 25.74)
    static let speaker = TextStyle(size: 17, line: 22.1, tracking: 0.94, smallCaps: true)
    static let hourTitle = TextStyle(size: 24.8, line: 39.68, tracking: 1.984)
    static let meta = TextStyle(size: 14.4, line: 20.88)
    static let small = TextStyle(size: 12.48, line: 19.97)
    static let brand = TextStyle.label(13.12, 0.08)
    static let menu = TextStyle.label(12.48, 0.08)
    static let control = TextStyle.label(11.52, 0.1)
}

// The page's palette, gilding, measures and composition, as the Android app's composition locals.
private struct PaletteKey: EnvironmentKey { static let defaultValue = Palette.nave }
private struct OrnamentKey: EnvironmentKey { static let defaultValue = Ornament.of(.nave, season: "") }
private struct MetricsKey: EnvironmentKey { static let defaultValue = Metrics() }
private struct WideKey: EnvironmentKey { static let defaultValue = false }
private struct RankedKey: EnvironmentKey { static let defaultValue = false }

extension EnvironmentValues {
    var palette: Palette {
        get { self[PaletteKey.self] }
        set { self[PaletteKey.self] = newValue }
    }

    var ornament: Ornament {
        get { self[OrnamentKey.self] }
        set { self[OrnamentKey.self] = newValue }
    }

    var metrics: Metrics {
        get { self[MetricsKey.self] }
        set { self[MetricsKey.self] = newValue }
    }

    /// Whether the page is laid out at the web's desktop widths (701pt up): an iPad, or a large phone on its side.
    var wide: Bool {
        get { self[WideKey.self] }
        set { self[WideKey.self] = newValue }
    }

    /// Whether a wide page is narrower than the hours' header holds on one line (701–959pt): there
    /// an hour's links take a rank of their own under the brand and Settings.
    var ranked: Bool {
        get { self[RankedKey.self] }
        set { self[RankedKey.self] = newValue }
    }
}

/// The web's breakpoint (style.css `min-width: 701px`).
let wideFrom: CGFloat = 701

/// Where the hours' header holds its links beside the brand and Settings (style.css `max-width: 959px`).
let rankedBelow: CGFloat = 960

/// Sets a style from the type scale, at the page's size.
private struct Typeface: ViewModifier {
    let style: TextStyle
    @Environment(\.metrics) private var m

    func body(content: Content) -> some View {
        let font = style.uiFont(m)
        content
            .font(Font(font))
            .tracking(style.tracking * m.type)
            .lineSpacing(max(0, style.line * m.type - font.lineHeight))
    }
}

extension View {
    func type(_ style: TextStyle) -> some View { modifier(Typeface(style: style)) }
}
