import CoreText
import Neuma
import SwiftUI
import UIKit

/// Staff spaces per lyric em: neuma's default lyric size, as the web's render-html/src/chant.rs.
private let lyricSpaces: Float = 2.45

/**
 * A chant score (a `.score` block, with the Chant setting on): neuma engraves its GABC at the
 * column's width with its lyrics at the hymn's text size, as the web's container query picks the
 * engraving that fits, and the staff takes the theme's lining, as the web's `.chant-score` does.
 * A whole hymn's stanzas after the first carry their number (the marker), in the rubric's red
 * above the score.
 */
struct ScoreRow: View {
    let block: BlockView
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @State private var width: CGFloat = 0

    var body: some View {
        let lyric = Float(Scale.verse.size * m.type)
        let page = width > 0 ? Engraving.page(block.score, initial: block.dropCap, width: Float(width.rounded(.down)), lyric: lyric) : nil
        VStack(alignment: .leading, spacing: m.px(2)) {
            if !block.marker.isEmpty {
                Text(block.marker).font(Font(garamond(Scale.verse.size * 0.85 * m.type))).foregroundStyle(p.rubric)
                    .accessibilityHidden(true)
            }
            // A custos may reach past the measure: the whole score is fitted to the column.
            let fit = page.map { $0.width > Float(width) ? width / CGFloat($0.width) : 1 } ?? 1
            Canvas { context, _ in
                context.scaleBy(x: fit, y: fit)
                if let page { draw(page, in: &context) }
            }
            .frame(maxWidth: .infinity)
            .frame(height: page.map { CGFloat($0.height) * fit } ?? 0)
            .onGeometryChange(for: CGFloat.self) { $0.size.width } action: { width = $0 }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(spoken(block))
    }

    /// neuma's display list in the theme's inks: glyphs and rectangles, then lyrics in EB Garamond with ligatures off,
    /// the initial gilt as every dropped initial is and the mode above it in the rubric's red.
    private func draw(_ page: Neuma.Page, in context: inout GraphicsContext) {
        // The web's --chant-* tokens (style.css): neumes a shade off the text's ink, staves in the
        // lining (thinned on a night ground).
        let (note, staff) = (mix(p.text, p.bg, 0.86), p.dark ? mix(p.lining, p.bg, 0.62) : p.lining)
        func ink(_ role: Ink) -> Color { role == .staff || role == .ledger ? staff : note }
        for item in page.items {
            switch item {
            case let .glyph(glyph, x, y, scale, role, _):
                guard let path = Engraving.outline(glyph) else { continue }
                let placed = CGAffineTransform(translationX: CGFloat(x), y: CGFloat(y)).scaledBy(x: CGFloat(scale), y: CGFloat(scale))
                context.fill(path.applying(placed), with: .color(ink(role)))
            case let .rect(x, y, w, h, role, _):
                context.fill(Path(CGRect(x: CGFloat(x), y: CGFloat(y), width: CGFloat(w), height: CGFloat(h))), with: .color(ink(role)))
            case let .text(x, baseline, size, runs, role, _):
                let line = NSMutableAttributedString()
                for run in runs {
                    line.append(NSAttributedString(string: run.text, attributes: [
                        .font: garamond(CGFloat(size), italic: run.italic, smallCaps: run.smallCaps),
                        .ligature: 0,
                        .foregroundColor: UIColor(role == .initial ? o.flat : run.rubric || role == .rubric || role == .annotation ? p.rubric : p.text),
                    ]))
                }
                context.withCGContext { cg in
                    // Core Text sets type upright in a y-up space; the canvas runs y down.
                    cg.textMatrix = CGAffineTransform(scaleX: 1, y: -1)
                    cg.textPosition = CGPoint(x: CGFloat(x), y: CGFloat(baseline))
                    CTLineDraw(CTLineCreateWithAttributedString(line), cg)
                }
            }
        }
    }
}

/// Scores engraved once per width and text size, and neuma's glyph outlines fetched once.
private enum Engraving {
    private static let lock = NSLock()
    private static var pages: [String: Neuma.Page] = [:]
    private static var outlines: [Int32: Path] = [:]

    /// The score laid out `width` points wide with its lyrics `lyric` points high; the first stanza alone takes the initial.
    static func page(_ gabc: String, initial: Bool, width: Float, lyric: Float) -> Neuma.Page {
        let key = "\(initial) \(width) \(lyric)\n\(gabc)"
        lock.lock()
        defer { lock.unlock() }
        if let page = pages[key] { return page }
        // Each rotation, Split View resize or text size engraves anew; keep the cache from growing without end.
        if pages.count >= 64 { pages.removeAll() }
        // Set as the web sets it (`page_options` in render-html's chant.rs): a hyphen between every two
        // syllables, the initial three quarters of the Solesmes size and flush with the margin, and the
        // last staff ending at the final bar.
        let options = ChantOptions(
            initial: initial ? 1 : 0, annotation: initial, everyHyphen: true, initialScale: 0.75, initialMargin: 0,
            shortLastStaff: true
        )
        let chant = Chant(gabc: gabc, options: options)
        let page = chant.layout(width: width, options: LayoutOptions(scale: lyric / lyricSpaces)).page()
        pages[key] = page
        return page
    }

    static func outline(_ id: Int32) -> Path? {
        lock.lock()
        defer { lock.unlock() }
        if let path = outlines[id] { return path }
        guard let d = glyphOutline(id: id)?.path else { return nil }
        let path = svg(d)
        outlines[id] = path
        return path
    }
}
