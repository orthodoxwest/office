import SwiftUI
import UIKit

/**
 * An opening's initial, set as the web's adaptive initial sets it. A painted capital two lines deep
 * stands beside the text as it wraps; a short responsory, or prose that fits on its line, takes a
 * smaller capital raised on that line; a psalm keeps its full capital, raised over a single line
 * (elevated), and may break after its mediant rather than leave a stub of a second line (divided).
 * Hymns and the Marian antiphon always drop.
 */
struct Initial: Equatable {
    /// How an opening chooses its setting.
    enum Adapt: Equatable {
        /// Always raised: a short responsory.
        case raised
        /// Raised if the opening then fits its line, else dropped: a chapter, a collect.
        case prose
        /// Dropped, elevated or divided: a psalm or canticle.
        case psalm
        /// Always dropped: a hymn, the Marian antiphon.
        case dropped
    }

    let letter: String
    /// Two lines deep, as CSS `initial-letter: 2` sizes it: its cap height spans a line pitch and the text's cap height.
    let deep: UIFont
    /// A raised capital as a letter of its line (`.initial-raised`): its face, the brush's edge, the space after it.
    let raised: NSAttributedString
    /// Where a deep capital's glyph starts from the measure's edge: its ink at the margin, moved by its hang.
    let left: CGFloat
    /// How far a deep capital's baseline stands below the first line's: its ink top meets that line's cap height.
    let drop: CGFloat
    /// Where the lines beside a deep capital start: past its ink and its gap.
    let edge: CGFloat
    /// How far the first line's opening word moves from where its line starts.
    let tuck: CGFloat
    /// The lines beside a deep capital: two, or three past a descending tail.
    let rows: Int
    let adapt: Adapt
    /// The gilding's flat ochre, which veils and brightens with the season.
    let color: UIColor
}

/**
 * A block's text as the web sets it: every line box exactly `line` tall with the glyphs centred
 * in it (CSS half-leading), hanging indents, tab-set gutters for verse numbers and sigils, and
 * the painted initial with the text running beside it.
 */
struct ProseSpec: Equatable {
    var text: NSAttributedString
    /// The block's face: its metrics seat the baseline in each line box.
    var font: UIFont
    var line: CGFloat
    var alignment: NSTextAlignment = .left
    /// The indent of each paragraph's first line, and of the lines that run on (a hanging indent).
    var firstIndent: CGFloat = 0
    var restIndent: CGFloat = 0
    var tabs: [NSTextTab] = []
    var initial: Initial?
}

/// A spec laid out at one width: its line fragments, and where the initial stands.
final class ProseLayout: NSObject, NSLayoutManagerDelegate {
    let spec: ProseSpec
    let width: CGFloat
    private let storage = NSTextStorage()
    private let manager = NSLayoutManager()
    private let container: NSTextContainer
    /// The baseline's depth in each line box: the glyphs' ascent and descent centred in it.
    private let baseline: CGFloat
    private(set) var height: CGFloat = 0
    private(set) var usedWidth: CGFloat = 0
    /// Space above the first line, for a raised initial that stands taller than it.
    private var top: CGFloat = 0
    /// Where the initial's baseline starts, in the view.
    private var capOrigin: CGPoint?
    private var capFont: UIFont?

    init(spec: ProseSpec, width: CGFloat) {
        self.spec = spec
        self.width = max(1, width)
        container = NSTextContainer(size: CGSize(width: max(1, width), height: .greatestFiniteMagnitude))
        let font = spec.font
        baseline = (spec.line - (font.ascender - font.descender)) / 2 + font.ascender
        super.init()
        container.lineFragmentPadding = 0
        manager.delegate = self
        manager.usesFontLeading = false
        manager.addTextContainer(container)
        storage.addLayoutManager(manager)
        guard let initial = spec.initial else {
            set(first: spec.firstIndent, rest: spec.restIndent)
            measure()
            return
        }
        // A line beside a dropped capital starts with its word, as CSS drops a space at a line's start
        // (after a lone O); a raised capital keeps the space, sharing its line.
        let lead = spec.text.string.prefix { $0.isWhitespace }.utf16.count
        let body = spec.text.attributedSubstring(from: NSRange(location: lead, length: spec.text.length - lead))
        let divided = ProseLayout.divideAtMediant(body)
        switch initial.adapt {
        case .raised:
            raise(initial)
        case .prose:
            if raise(initial).count > 1 { drop(initial, body) }
        case .dropped:
            drop(initial, body)
        case .psalm:
            let natural = drop(initial, body)
            if natural.count < 2 {
                // Raising can widen the line past the measure: two whole half-verses beside a dropped
                // capital are then better than a new short tail.
                if elevate(initial, body).count > 1, let divided, drop(initial, divided).count != 2 {
                    elevate(initial, body)
                }
            } else if let divided, natural.count == 2, natural[1].words <= 2, natural[1].width < natural[0].width * 0.3 {
                // A one- or two-word tail under less than 30% of the first line: break at the mediant
                // instead, if that keeps two lines and clearly balances them.
                let rows = drop(initial, divided)
                if !(rows.count == 2 && ProseLayout.balance(rows) > ProseLayout.balance(natural) + 0.15) {
                    drop(initial, body)
                }
            }
        }
        measure()
    }

    /// One line of a setting: how wide its words run, and how many there are.
    private struct Row {
        let width: CGFloat
        let words: Int
    }

    /// Sets `text` beside a dropped capital: two lines (three past a descending tail), the remainder
    /// running on at the text edge. As beside the web's float, a line never starts short of the text's
    /// own edge, and the first line's opening word moves by the capital's tuck from where its line starts.
    @discardableResult
    private func drop(_ initial: Initial, _ text: NSAttributedString) -> [Row] {
        let second = max(initial.edge, spec.restIndent)
        let first = max(0, second + initial.tuck)
        container.exclusionPaths = [
            UIBezierPath(rect: CGRect(x: 0, y: 0, width: first, height: spec.line - 0.5)),
            UIBezierPath(rect: CGRect(x: 0, y: spec.line, width: second, height: spec.line * CGFloat(initial.rows - 1) - 0.5)),
        ]
        top = 0
        set(first: 0, rest: spec.restIndent, text: text)
        capOrigin = CGPoint(x: initial.left, y: (lines().first?.minY ?? 0) + baseline + initial.drop)
        capFont = initial.deep
        return rows()
    }

    /// Sets `text` on one line under its full capital, raised into a line pitch above it: the
    /// capital's ink top meets that empty line's cap height, its foot the text's baseline, and the
    /// text starts at its lower contour, untucked.
    @discardableResult
    private func elevate(_ initial: Initial, _ text: NSAttributedString) -> [Row] {
        container.exclusionPaths = []
        top = spec.line
        set(first: max(initial.edge, spec.restIndent), rest: spec.restIndent, text: text)
        capOrigin = CGPoint(x: initial.left, y: baseline + initial.drop)
        capFont = initial.deep
        return rows()
    }

    /// Sets the capital raised as a letter of the first line, its line's height unchanged.
    @discardableResult
    private func raise(_ initial: Initial) -> [Row] {
        container.exclusionPaths = []
        top = 0
        let text = NSMutableAttributedString(attributedString: initial.raised)
        text.append(spec.text)
        set(first: spec.firstIndent, rest: spec.restIndent, text: text)
        capOrigin = nil
        capFont = nil
        return rows()
    }

    private func rows() -> [Row] {
        var out: [Row] = []
        let glyphs = manager.glyphRange(for: container)
        manager.enumerateLineFragments(forGlyphRange: glyphs) { _, used, _, range, _ in
            let chars = self.manager.characterRange(forGlyphRange: range, actualGlyphRange: nil)
            let words = (self.storage.string as NSString).substring(with: chars).split(whereSeparator: \.isWhitespace).count
            out.append(Row(width: used.width, words: words))
        }
        return out
    }

    /// How evenly a two-line setting fills its lines: the shorter over the longer.
    private static func balance(_ rows: [Row]) -> CGFloat {
        guard rows.count == 2, let wide = rows.map(\.width).max(), wide > 0 else { return 0 }
        return rows.map(\.width).min()! / wide
    }

    /// The opening with a break after its mediant (the web's `.initial-divided`), if it has one.
    private static func divideAtMediant(_ text: NSAttributedString) -> NSAttributedString? {
        let s = text.string as NSString
        let at = s.range(of: "*")
        guard at.location != NSNotFound else { return nil }
        let head = at.location + at.length
        let tail = s.substring(from: head)
        let skip = (String(tail.prefix { $0.isWhitespace }) as NSString).length
        let out = NSMutableAttributedString(attributedString: text.attributedSubstring(from: NSRange(location: 0, length: head)))
        out.append(NSAttributedString(string: "\n", attributes: text.attributes(at: max(0, head - 1), effectiveRange: nil)))
        out.append(text.attributedSubstring(from: NSRange(location: head + skip, length: text.length - head - skip)))
        return out
    }

    private func set(first: CGFloat, rest: CGFloat, text source: NSAttributedString? = nil) {
        let text = NSMutableAttributedString(attributedString: source ?? spec.text)
        let style = NSMutableParagraphStyle()
        style.alignment = spec.alignment
        style.firstLineHeadIndent = first
        style.headIndent = rest
        style.minimumLineHeight = spec.line
        style.maximumLineHeight = spec.line
        style.lineBreakMode = .byWordWrapping
        style.tabStops = spec.tabs
        style.defaultTabInterval = 0
        text.addAttribute(.paragraphStyle, value: style, range: NSRange(location: 0, length: text.length))
        storage.setAttributedString(text)
    }

    /// The line fragments, top to bottom.
    private func lines() -> [CGRect] {
        var out: [CGRect] = []
        let glyphs = manager.glyphRange(for: container)
        manager.enumerateLineFragments(forGlyphRange: glyphs) { rect, _, _, _, _ in out.append(rect) }
        return out
    }

    private func measure() {
        let fragments = lines()
        height = top + (fragments.last?.maxY ?? 0)
        var used: CGFloat = 0
        let glyphs = manager.glyphRange(for: container)
        manager.enumerateLineFragments(forGlyphRange: glyphs) { _, usedRect, _, _, _ in used = max(used, usedRect.maxX) }
        usedWidth = used.rounded(.up)
    }

    /// The width a string sets in a face.
    static func advance(_ s: String, _ font: UIFont) -> CGFloat {
        let line = CTLineCreateWithAttributedString(NSAttributedString(string: s, attributes: [.font: font]))
        return CGFloat(CTLineGetTypographicBounds(line, nil, nil, nil))
    }

    // Every line box exactly `line` tall, its baseline where CSS's half-leading puts it.
    func layoutManager(
        _ layoutManager: NSLayoutManager,
        shouldSetLineFragmentRect lineFragmentRect: UnsafeMutablePointer<CGRect>,
        lineFragmentUsedRect: UnsafeMutablePointer<CGRect>,
        baselineOffset: UnsafeMutablePointer<CGFloat>,
        in textContainer: NSTextContainer,
        forGlyphRange glyphRange: NSRange
    ) -> Bool {
        lineFragmentRect.pointee.size.height = spec.line
        lineFragmentUsedRect.pointee.size.height = spec.line
        baselineOffset.pointee = baseline
        return true
    }

    func draw(in ctx: CGContext) {
        let glyphs = manager.glyphRange(for: container)
        manager.drawBackground(forGlyphRange: glyphs, at: CGPoint(x: 0, y: top))
        manager.drawGlyphs(forGlyphRange: glyphs, at: CGPoint(x: 0, y: top))
        guard let initial = spec.initial, let origin = capOrigin, let font = capFont else { return }
        ProseLayout.paint(initial.letter, font, at: origin, color: initial.color, in: ctx)
    }

    /**
     * Draws `letter` flat in `color`, as a painter laid it, its baseline at `origin`, with a hint of
     * the brush's edge a point below it (the web's 1px text-shadow at 30%).
     */
    static func paint(_ letter: String, _ font: UIFont, at origin: CGPoint, color: UIColor, in ctx: CGContext) {
        let line = CTLineCreateWithAttributedString(NSAttributedString(string: letter, attributes: [.font: font]))
        let outline = CGMutablePath()
        for run in CTLineGetGlyphRuns(line) as! [CTRun] {
            let attributes = CTRunGetAttributes(run) as NSDictionary
            guard let runFont = attributes[kCTFontAttributeName as String] else { continue }
            let ctFont = runFont as! CTFont
            let count = CTRunGetGlyphCount(run)
            var glyphs = [CGGlyph](repeating: 0, count: count)
            var positions = [CGPoint](repeating: .zero, count: count)
            CTRunGetGlyphs(run, CFRange(location: 0, length: count), &glyphs)
            CTRunGetPositions(run, CFRange(location: 0, length: count), &positions)
            for i in 0..<count {
                guard let glyph = CTFontCreatePathForGlyph(ctFont, glyphs[i], nil) else { continue }
                // Core Text draws upward from the baseline; the view counts down from its top.
                let t = CGAffineTransform(translationX: origin.x + positions[i].x, y: origin.y).scaledBy(x: 1, y: -1)
                outline.addPath(glyph, transform: t)
            }
        }
        let shadow = CGMutablePath()
        shadow.addPath(outline, transform: CGAffineTransform(translationX: 0, y: 1))
        ctx.saveGState()
        ctx.addPath(shadow)
        ctx.setFillColor(color.withAlphaComponent(0.3).cgColor)
        ctx.fillPath()
        ctx.addPath(outline)
        ctx.setFillColor(color.cgColor)
        ctx.fillPath()
        ctx.restoreGState()
    }
}

/// The view that draws a laid-out spec, keeping the last layout for its width.
final class ProseView: UIView {
    var spec: ProseSpec? {
        didSet { if spec != oldValue { setNeedsDisplay() } }
    }

    private var cache: ProseLayout?

    override init(frame: CGRect) {
        super.init(frame: frame)
        isOpaque = false
        backgroundColor = .clear
        contentMode = .redraw
        isAccessibilityElement = true
    }

    required init?(coder: NSCoder) { fatalError("not used") }

    func layout(_ spec: ProseSpec, width: CGFloat) -> ProseLayout {
        if let cache, cache.width == width, cache.spec == spec { return cache }
        let layout = ProseLayout(spec: spec, width: width)
        cache = layout
        return layout
    }

    override func draw(_ rect: CGRect) {
        guard let spec, let ctx = UIGraphicsGetCurrentContext() else { return }
        layout(spec, width: bounds.width).draw(in: ctx)
    }
}

/// A spec set on the page; `spoken` is what VoiceOver says for it.
struct Prose: UIViewRepresentable {
    let spec: ProseSpec
    var spoken: String?
    var header = false

    func makeUIView(context: Context) -> ProseView { ProseView() }

    func updateUIView(_ view: ProseView, context: Context) {
        view.spec = spec
        view.accessibilityLabel = spoken ?? spec.text.string
        view.accessibilityTraits = header ? [.staticText, .header] : .staticText
    }

    func sizeThatFits(_ proposal: ProposedViewSize, uiView: ProseView, context: Context) -> CGSize? {
        let proposed = proposal.width.flatMap { $0.isFinite ? $0 : nil }
        let layout = uiView.layout(spec, width: proposed ?? 10_000)
        return CGSize(width: proposed ?? layout.usedWidth, height: layout.height)
    }
}
