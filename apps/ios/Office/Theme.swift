import CoreText
import SwiftUI
import UIKit

/// The web's colour tokens for the Nave (style.css `:root`), as the Android app's `Theme.kt` holds them.
struct Palette {
    let text: Color
    let bg: Color
    let accent: Color
    let gold: Color
    let goldLine: Color
    let muted: Color
    let border: Color
    let rubric: Color
    let surface: Color

    static let nave = Palette(
        text: Color(hex: 0x241C17),
        bg: Color(hex: 0xFAF3E9),
        accent: Color(hex: 0x6B3A1F),
        gold: Color(hex: 0x9A7328),
        goldLine: Color(hex: 0xC9AC72),
        muted: Color(hex: 0x6B5D54),
        border: Color(hex: 0xDCCFC3),
        rubric: Color(hex: 0x8B1A1A),
        surface: Color(hex: 0xF6EDDF)
    )
}

extension Color {
    init(hex: UInt32) {
        self.init(
            red: Double((hex >> 16) & 0xFF) / 255,
            green: Double((hex >> 8) & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255
        )
    }
}

/// The day's liturgical colour, as the hour's band and the ordo's rails draw it.
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

/**
 * A text style from the Android app's type scale (`Theme.kt` `Type`), measured from the web at a
 * phone's width: EB Garamond 12, with small capitals, lining figures and tracking where the web
 * sets them.
 */
struct TextStyle {
    var size: CGFloat
    var line: CGFloat
    var tracking: CGFloat = 0
    var smallCaps = false
    var lining = false
    var italic = false

    /// The face, with its OpenType features turned on through the font descriptor.
    var uiFont: UIFont {
        let name = italic ? "EBGaramond12-Italic" : "EBGaramond12-Regular"
        let base = UIFont(name: name, size: size) ?? UIFont.systemFont(ofSize: size)
        var features: [[UIFontDescriptor.FeatureKey: Int]] = []
        if smallCaps {
            features.append([.type: kLowerCaseType, .selector: kLowerCaseSmallCapsSelector])
            features.append([.type: kUpperCaseType, .selector: kUpperCaseSmallCapsSelector])
        }
        if lining {
            features.append([.type: kNumberCaseType, .selector: kUpperCaseNumbersSelector])
        }
        if features.isEmpty { return base }
        let descriptor = base.fontDescriptor.addingAttributes([.featureSettings: features])
        return UIFont(descriptor: descriptor, size: size)
    }

    var font: Font { Font(uiFont) }

    /// The gap SwiftUI leaves between lines for this style's line height.
    var lineSpacing: CGFloat { max(0, line - uiFont.lineHeight) }
}

/// The type scale.
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
}

/// The ✠, from the Noto Sans Symbols subset the web and the Android app use.
func crossFont(size: CGFloat) -> Font {
    Font(UIFont(name: "NotoSansSymbols-Bold", size: size) ?? UIFont.systemFont(ofSize: size))
}

extension Text {
    /// Sets a style from the type scale.
    func style(_ s: TextStyle) -> some View {
        self.font(s.font).tracking(s.tracking).lineSpacing(s.lineSpacing)
    }
}
