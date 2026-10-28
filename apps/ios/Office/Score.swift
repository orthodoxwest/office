import CoreText
import Neuma
import SwiftUI
import UIKit

/// Staff spaces per lyric em: neuma's default lyric size, as the web's render-html/src/chant.rs.
private let lyricSpaces: Float = 2.45

/**
 * A chant score (a `.score` block, with the Chant setting on): neuma engraves its GABC at the
 * column's width with its lyrics at the hymn's text size, as the web's chant.js lays it out at its
 * column's, and the staff takes a tint of the theme's lining, as the web's `.chant-score` does. The
 * score keeps to the hymn's measure, as the web's `.hymn-verses`. The opening score carries its
 * mode above it (`.chant-mode`); a whole hymn's stanzas after the first carry their number (the
 * marker), in the rubric's red above the score; and a tune shown once carries a strip of its
 * hymn's verses under it (`.chant-strip`), a tap setting that verse's words under the notes.
 */
struct ScoreRow: View {
    let block: BlockView
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m
    @State private var width: CGFloat = 0
    @State private var verse = 0

    var body: some View {
        let lyric = Float(Scale.verse.size * m.type)
        let shown = block.stanzas.indices.contains(verse) ? verse : 0
        let gabc = block.stanzas.isEmpty ? block.score : block.stanzas[shown]
        let page = width > 0
            ? Engraving.page(gabc, initial: block.dropCap && shown == 0, phrases: block.phrases, width: Float(width.rounded(.down)), lyric: lyric)
            : nil
        VStack(alignment: .leading, spacing: m.px(2)) {
            VStack(alignment: .leading, spacing: m.px(2)) {
                if !block.marker.isEmpty {
                    Text(block.marker).font(Font(garamond(Scale.verse.size * 0.85 * m.type))).foregroundStyle(p.rubric)
                }
                if !block.mode.isEmpty {
                    Text(block.mode).font(Font(garamond(Scale.verse.size * 0.8 * m.type, smallCaps: true)))
                        .tracking(Scale.verse.size * 0.06 * m.type)
                        .foregroundStyle(p.rubric)
                        .padding(.bottom, m.px(1))
                }
                // A custos may reach past the measure: the whole score is fitted to the column.
                let fit = page.map { $0.width > Float(width) ? width / CGFloat($0.width) : 1 } ?? 1
                Canvas { context, _ in
                    context.scaleBy(x: fit, y: fit)
                    if let page { draw(page, in: &context, fit: fit * displayScale) }
                }
                .frame(maxWidth: .infinity)
                .frame(height: page.map { CGFloat($0.height) * fit } ?? 0)
                .onGeometryChange(for: CGFloat.self) { $0.size.width } action: { width = $0 }
            }
            .accessibilityElement(children: .ignore)
            .accessibilityLabel(spoken(block))
            if block.stanzas.count > 1 {
                verseStrip(chosen: shown)
            }
        }
        .frame(maxWidth: m.px(hymnMax))
        .frame(maxWidth: .infinity)
        .onChange(of: block.score) { verse = 0 }
    }

    @Environment(\.displayScale) private var displayScale

    /// The verses a tune can carry, numbered: the one under the notes ringed in the rubric's red (`.chant-strip`).
    private func verseStrip(chosen: Int) -> some View {
        HStack(spacing: m.px(4)) {
            ForEach(block.stanzas.indices, id: \.self) { i in
                let on = i == chosen
                Button { verse = i } label: {
                    Text("\(i + 1)").font(Font(garamond(Scale.verse.size * 0.85 * m.type)))
                        .foregroundStyle(on ? p.rubric : p.muted)
                        .frame(minWidth: m.px(32), minHeight: m.px(32))
                        .overlay(Circle().strokeBorder(on ? p.rubric : p.surfaceEdge, lineWidth: 1))
                        .contentShape(Circle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("Verse \(i + 1)")
                .accessibilityAddTraits(on ? .isSelected : [])
            }
        }
        .padding(.top, m.px(2))
        .padding(.bottom, m.px(10))
        .accessibilityElement(children: .contain)
        .accessibilityLabel("The verse under the notes")
    }

    /// neuma's display list in the theme's inks: glyphs and rectangles, then lyrics in EB Garamond with ligatures off,
    /// the initial gilt as every dropped initial is. Drawn at `fit` device pixels to its unit: staff and ledger lines are
    /// snapped to whole pixels, so every line of a staff is as crisp and as heavy as the others.
    private func draw(_ page: Neuma.Page, in context: inout GraphicsContext, fit: CGFloat) {
        // The web's --chant-* tokens (style.css): neumes in the text's ink, staves a tint of the
        // lining (fainter on a night ground).
        let (note, staff) = (p.text, mix(p.lining, p.bg, p.dark ? 0.5 : 0.72))
        func ink(_ role: Ink) -> Color { role == .staff || role == .ledger ? staff : note }
        for item in page.items {
            switch item {
            case let .glyph(glyph, x, y, scale, role, _):
                guard let path = Engraving.outline(glyph) else { continue }
                let placed = CGAffineTransform(translationX: CGFloat(x), y: CGFloat(y)).scaledBy(x: CGFloat(scale), y: CGFloat(scale))
                context.fill(path.applying(placed), with: .color(ink(role)))
            case let .rect(x, y, w, h, role, _):
                var (top, height) = (CGFloat(y), CGFloat(h))
                if role == .staff || role == .ledger, fit > 0 {
                    top = (top * fit).rounded() / fit
                    height = max(1, (height * fit).rounded()) / fit
                }
                context.fill(Path(CGRect(x: CGFloat(x), y: top, width: CGFloat(w), height: height)), with: .color(ink(role)))
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

    /// The score laid out `width` points wide with its lyrics `lyric` points high; the first stanza alone takes the
    /// initial, and a hymn's phrases each take a staff.
    static func page(_ gabc: String, initial: Bool, phrases: Bool, width: Float, lyric: Float) -> Neuma.Page {
        let key = "\(initial) \(phrases) \(width) \(lyric)\n\(gabc)"
        lock.lock()
        defer { lock.unlock() }
        if let page = pages[key] { return page }
        // Each rotation, Split View resize or text size engraves anew; keep the cache from growing without end.
        if pages.count >= 64 { pages.removeAll() }
        // Set as the web sets it (`page_options` in render-html's chant.rs): lyrics measured in the app's
        // own Garamond (the table the core sets, `.custom`); a hyphen between every two syllables; the
        // initial half the Solesmes size and flush with the margin, the mode above the score rather than
        // over it; the last staff ending at the final bar; and a hymn's phrases each on a staff of their own.
        let options = ChantOptions(
            initial: initial ? 1 : 0, annotation: false, font: .custom, everyHyphen: true, initialScale: 0.5, initialMargin: 0,
            shortLastStaff: true, phraseLines: phrases
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
