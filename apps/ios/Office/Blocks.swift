import SwiftUI
import UIKit

/// The verse gutter (`--verse-gutter`, 1.8rem): verse numbers and ℣/℟ sit in it, text beyond it.
let verseGutter: CGFloat = 28.8

/**
 * Space above `cur`, after `prev`, in the web's rhythm at a phone's width (measured from the
 * rendered hour, as the Android app's `gapBefore`): a clear threshold between psalm groups, air
 * between elements, and the lines of one element close together.
 */
func gapBefore(_ prev: BlockView?, _ cur: BlockView) -> CGFloat {
    guard let prev else { return 0 }
    let heading = { (b: BlockView) in b.kind == .heading || b.kind == .commemorationHeading }
    let antiphon = { (b: BlockView) in b.kind == .antiphon || b.kind == .announcedAntiphon }
    let note = { (b: BlockView) in b.kind == .antiphonNote || b.kind == .announcementNote }
    if heading(cur) { return 38 }
    if heading(prev) { return cur.kind == .chapterRef ? 24 : 27 }
    if cur.kind == .gap || prev.kind == .gap { return 4.8 }
    if prev.kind == .itemLabel { return 9 }
    // A note on the antiphon sits close under it, as the web's `.unrepeated-note`.
    if note(cur) { return 2.4 }
    if cur.kind == .itemLabel { return antiphon(prev) || note(prev) ? 10 : 30 }
    if prev.kind == .chapterRef { return 15 }
    if prev.kind == .latinTitle { return 8 }
    if prev.kind == .speaker { return 3.2 }
    // A closing antiphon, then the next group's opening one: the threshold between groups.
    if antiphon(cur) && prev.kind == .antiphon { return 49 }
    // A closing antiphon sits close under its psalm's last verse or Gloria.
    if cur.kind == .antiphon && (prev.kind == .verse || (prev.kind == .gloriaPatri && !prev.startsElement)) { return 6 }
    if cur.kind == .verse && prev.kind == .verse { return 4.8 }
    // The Gloria Patri after a psalm's last verse.
    if cur.kind == .gloriaPatri && prev.kind == .verse { return 13.6 }
    if cur.kind == .stanza && prev.kind == .stanza { return 12 }
    // A hymn's rubric keeps a stanza's distance from the stanzas around it.
    if cur.kind == .hymnRubric && prev.kind == .stanza { return 12 }
    if cur.kind == .stanza && prev.kind == .hymnRubric { return 12 }
    if cur.startsElement { return 14 }
    return 4.8
}

/// What a block is set in: its style, colour and alignment (the web's class for the same text).
private struct Setting {
    let style: TextStyle
    let color: KeyPath<Palette, Color>
    var alignment: NSTextAlignment = .left
}

private func setting(_ kind: BlockKind) -> Setting {
    switch kind {
    // The tituli, red ochre on the limewash and gilt on the Apse night, clear of the rubrics' red.
    case .heading, .commemorationHeading: return Setting(style: Scale.heading, color: \.titulus, alignment: .center)
    case .itemLabel: return Setting(style: Scale.itemLabel, color: \.titulus, alignment: .center)
    case .latinTitle, .canticleSection: return Setting(style: Scale.bodyItalic, color: \.muted, alignment: .center)
    case .chapterRef, .scriptureRef: return Setting(style: Scale.reference, color: \.rubric, alignment: .center)
    case .rubric, .antiphonNote: return Setting(style: Scale.rubric, color: \.rubric)
    // Under an announcement, centred beneath its words.
    case .announcementNote: return Setting(style: Scale.rubric, color: \.rubric, alignment: .center)
    // A rubric among a hymn's stanzas, centred in its column.
    case .hymnRubric: return Setting(style: Scale.rubric, color: \.rubric, alignment: .center)
    case .speaker: return Setting(style: Scale.speaker, color: \.rubric)
    case .verse, .stanza, .gloriaPatri: return Setting(style: Scale.verse, color: \.text)
    default: return Setting(style: Scale.body, color: \.text)
    }
}

/// A block's runs as styled text in `style`, each run in its web class's colour and face.
func runs(_ block: BlockView, _ style: TextStyle, color: Color, _ p: Palette, _ m: Metrics) -> NSMutableAttributedString {
    let k = m.type
    let size = style.size * k
    let base: [NSAttributedString.Key: Any] = [
        .font: style.uiFont(k),
        .kern: style.tracking * k,
        .foregroundColor: UIColor(color),
    ]
    let out = NSMutableAttributedString()
    for run in block.runs {
        var a = base
        switch run.style {
        case .plain, .break:
            break
        case .prayed:
            a[.foregroundColor] = UIColor(p.text)
        case .mediant:
            // The pointing asterisk is quiet, and lowered 0.25em to the line's optical middle, as the
            // web's `.mediant` (Garamond draws its asterisk high, as a footnote mark).
            a[.foregroundColor] = UIColor(p.muted)
            a[.baselineOffset] = -0.25 * size
        case .secret:
            // Words not said aloud.
            a[.foregroundColor] = UIColor(p.unsaid)
        case .cross:
            a[.foregroundColor] = UIColor(p.rubric)
            a[.font] = crossUIFont(size * 0.8)
        case .latin:
            // Latin within a small-caps label is set in lower case italic and muted, as the web's `.psalm-incipit`.
            a[.font] = garamond(size, italic: true, smallCaps: false, lining: true)
            a[.kern] = 0.4 * k
            a[.foregroundColor] = UIColor(p.muted)
        case .kicker:
            a[.font] = garamond(size * 0.7, italic: style.italic, smallCaps: style.smallCaps, lining: style.lining)
            a[.foregroundColor] = UIColor(p.muted)
            a[.kern] = size * 0.7 * 0.1
        case .posture:
            a[.font] = garamond(size * 0.9, italic: style.italic, smallCaps: style.smallCaps, lining: style.lining)
            a[.foregroundColor] = UIColor(p.rubric)
        }
        out.append(NSAttributedString(string: run.text, attributes: a))
    }
    return out
}

private func isSpace(_ c: unichar) -> Bool {
    guard let s = Unicode.Scalar(c) else { return false }
    return CharacterSet.whitespacesAndNewlines.contains(s)
}

private func isLetter(_ c: unichar) -> Bool {
    guard let s = Unicode.Scalar(c) else { return false }
    return CharacterSet.letters.contains(s)
}

private func inWord(_ c: unichar) -> Bool {
    isLetter(c) || c == 0x27 || c == 0x2019 || c == 0x2D
}

/**
 * The opening letter and the text after it, with the small-caps transition applied: the rest of
 * the first word, or the next word after a lone O or I, as the eye leaves the capital. Only an
 * opening whose first run is ordinary spoken text takes an initial, or a psalm's whose opening
 * words go unsaid after its antiphon: the initial stays gilt, the words after it muted.
 */
func splitInitial(_ block: BlockView, _ text: NSAttributedString) -> (letter: String, rest: NSAttributedString)? {
    let lead = block.runs.first?.style
    guard lead == .plain || (lead == .secret && block.kind == .verse) else { return nil }
    let s = text.string as NSString
    var at = 0
    while at < s.length && isSpace(s.character(at: at)) { at += 1 }
    guard at < s.length, isLetter(s.character(at: at)) else { return nil }
    let letter = s.rangeOfComposedCharacterSequence(at: at)
    let after = NSMaxRange(letter)
    let rest = NSMutableAttributedString(attributedString: text.attributedSubstring(from: NSRange(location: after, length: s.length - after)))
    let r = rest.string as NSString
    func first(from: Int, _ test: (unichar) -> Bool) -> Int {
        var i = from
        while i < r.length && !test(r.character(at: i)) { i += 1 }
        return i
    }
    var start = 0
    var end = first(from: 0) { !inWord($0) }
    if end == 0 {
        start = first(from: 0) { !isSpace($0) }
        end = first(from: start) { !inWord($0) }
    }
    if end > start {
        let range = NSRange(location: start, length: end - start)
        rest.enumerateAttribute(.font, in: range) { value, sub, _ in
            guard let font = value as? UIFont else { return }
            rest.addAttribute(.font, value: garamond(font.pointSize, italic: font.fontName.contains("Italic"), smallCaps: true), range: sub)
            rest.addAttribute(.kern, value: font.pointSize * 0.04, range: sub)
        }
    }
    return (s.substring(with: letter), rest)
}

/**
 * The initial for an opening in `style`, painted in the gilding: two lines deep as the web's
 * `initial-letter: 2` sets it, fitted by its ink and the capital's optical profile (see
 * `render_blocks::initials`), or raised as a letter of its line.
 */
private func initial(_ letter: String, _ style: TextStyle, _ o: Ornament, _ m: Metrics, adapt: Initial.Adapt) -> Initial {
    let size = style.size * m.type
    let fit = initialFit(letter: letter)
    let em = size * CGFloat(initialSize(lineHeightEm: Float(style.line / style.size)))
    let deep = garamond(em)
    let ink = CTLineGetBoundsWithOptions(CTLineCreateWithAttributedString(NSAttributedString(string: letter, attributes: [.font: deep])), .useGlyphPathBounds)
    // The profiles are measured in the web's declared initial, not the size it is drawn at.
    let profile = size * CGFloat(initialProfileEm())
    let left = CGFloat(fit.hang) * profile
    // Raised: the brush's edge a point below (the web's 1px text-shadow at 30%), and the space after.
    let shadow = NSShadow()
    shadow.shadowOffset = CGSize(width: 0, height: 1)
    shadow.shadowBlurRadius = 0
    shadow.shadowColor = UIColor(o.flat).withAlphaComponent(0.3)
    let raisedSize = size * CGFloat(raisedInitialSize())
    let raised = NSAttributedString(string: letter, attributes: [
        .font: garamond(raisedSize),
        .foregroundColor: UIColor(o.flat),
        .shadow: shadow,
        .kern: CGFloat(raisedInitialGap()) * raisedSize + CGFloat(fit.raisedTuck) * size,
    ])
    return Initial(
        letter: letter,
        deep: deep,
        raised: raised,
        left: left - ink.minX,
        drop: ink.maxY - CGFloat(capHeight()) * size,
        edge: left + ink.width + CGFloat(fit.gap) * profile,
        tuck: CGFloat(fit.tuck) * size,
        rows: fit.depth > 0 ? 3 : 2,
        adapt: adapt,
        color: UIColor(o.flat)
    )
}

/// How a block is typeset: its text, face, measure and any initial, as the Android app's `Block`.
func proseSpec(_ block: BlockView, _ p: Palette, _ o: Ornament, _ m: Metrics) -> ProseSpec {
    let look = setting(block.kind)
    let style = look.style
    let color = p[keyPath: look.color]
    let font = style.uiFont(m)
    let line = style.line * m.type
    let text = runs(block, style, color: color, p, m)
    var spec = ProseSpec(text: text, font: font, line: line, alignment: look.alignment)
    let gutter = m.px(verseGutter)

    /// An opening with its initial, the lines after the capital running on at `textStart`.
    func opening(textStart: CGFloat, raised: Bool = false) -> ProseSpec {
        guard let split = splitInitial(block, text) else {
            var s = spec
            s.firstIndent = textStart
            s.restIndent = textStart
            return s
        }
        var s = spec
        s.text = split.rest
        s.restIndent = max(s.restIndent, textStart)
        let adapt: Initial.Adapt
        switch block.kind {
        case .verse: adapt = .psalm
        case .stanza, .chantLine: adapt = .dropped
        default: adapt = raised ? .raised : .prose
        }
        s.initial = initial(split.letter, style, o, m, adapt: adapt)
        return s
    }

    func marked(_ marker: String, _ markerStyle: TextStyle, _ markerColor: Color) -> NSMutableAttributedString {
        NSMutableAttributedString(string: marker, attributes: [
            .font: markerStyle.uiFont(m),
            .kern: markerStyle.tracking * m.type,
            .foregroundColor: UIColor(markerColor),
        ])
    }

    switch block.kind {
    case .antiphon, .announcedAntiphon:
        // Body antiphons hang left: the sigil opens the line, wrapped lines clear it. An announcement's
        // opening words stand centred over the psalm's label, as the web's `.antiphon-announce`.
        var sigil = Scale.body
        sigil.smallCaps = true
        sigil.tracking = 1.4
        let t = marked(block.marker, sigil, p.titulus)
        t.append(NSAttributedString(string: " ", attributes: [.font: font]))
        t.append(text)
        spec.text = t
        if block.kind == .announcedAntiphon {
            spec.alignment = .center
        } else {
            spec.restIndent = 21.6 * m.type
        }
    case .antiphonNote:
        // Under the antiphon's words, past its "Ant." (the antiphon's hanging indent).
        spec.firstIndent = 21.6 * m.type
        spec.restIndent = 21.6 * m.type
    case .verse:
        if block.dropCap { return opening(textStart: gutter) }
        spec.restIndent = gutter
        if block.marker.isEmpty {
            spec.firstIndent = gutter
        } else {
            // The number set right in the gutter, the verse from its edge.
            let t = NSMutableAttributedString(string: "\t", attributes: [.font: font])
            t.append(marked(block.marker, Scale.verseNumber, p.muted))
            t.append(NSAttributedString(string: "\t", attributes: [.font: font]))
            t.append(text)
            spec.text = t
            spec.tabs = [NSTextTab(textAlignment: .right, location: gutter - m.px(8)), NSTextTab(textAlignment: .left, location: gutter)]
        }
    case .gloriaPatri:
        // The Gloria Patri keeps the verses' edge; each line's wrap steps in (the web's `.source-line`, 1.1rem).
        spec.firstIndent = gutter
        spec.restIndent = gutter + 17.6 * m.type
    case .versicle, .response, .all:
        if block.dropCap { return opening(textStart: 0, raised: true) }
        if block.marker.isEmpty { break }
        if block.marker.count > 2 {
            // A spelled-out sigil ("Blessing.", "All:") stands above the text.
            let t = marked(block.marker, Scale.body, p.rubric)
            t.append(NSAttributedString(string: "\u{2028}", attributes: [.font: font]))
            t.append(text)
            spec.text = t
            spec.restIndent = gutter
        } else {
            // The sigil set right in its gutter; the gutter keeps its stop at any size.
            let t = NSMutableAttributedString(string: "\t", attributes: [.font: font])
            t.append(marked(block.marker, Scale.body, p.rubric))
            t.append(NSAttributedString(string: "\t", attributes: [.font: font]))
            t.append(text)
            spec.text = t
            let sigil = m.px(22)
            spec.tabs = [NSTextTab(textAlignment: .right, location: sigil), NSTextTab(textAlignment: .left, location: sigil + m.px(6.4))]
            spec.restIndent = sigil + m.px(6.4)
        }
    case .stanza:
        // A wrapped line hangs beneath its own start (`.hymn-line`, 1.1rem).
        spec.restIndent = 17.6 * m.type
        if block.dropCap { return opening(textStart: 0) }
    case .paragraph, .chantLine:
        if block.dropCap { return opening(textStart: 0) }
    default:
        break
    }
    return spec
}

/**
 * One block of a composed hour, styled after the web's classes for the same text. A hymn's
 * stanzas and rubrics are set in `column`, the width of the hymn's longest line (see `hymnColumns`).
 */
struct BlockRow: View {
    let block: BlockView
    var column: CGFloat?
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        if block.kind == .gap {
            Color.clear.frame(height: m.px(8)).accessibilityHidden(true)
        } else {
            let prose = Prose(spec: proseSpec(block, p, o, m), spoken: spoken(block), header: block.kind == .heading || block.kind == .commemorationHeading)
            if block.kind == .stanza || block.kind == .hymnRubric {
                // The hymn's column, centred: the rag balanced by an equal indent on the left,
                // as the web's fit-content `.hymn-verses`.
                prose.frame(maxWidth: column ?? .infinity).frame(maxWidth: .infinity)
            } else {
                prose
            }
        }
    }
}

/// The web's `.hymn-verses` max-width, 28rem.
private let hymnMax: CGFloat = 448

/// Slack on a hymn's column, so its longest line does not wrap on rounding. The web's `.hymn-line`
/// hang adds nothing to the column: its padding and negative text-indent cancel in `fit-content`.
private let hymnSlack: CGFloat = 1

/// Where a block is in an hour: its section, and its place there.
struct BlockAt: Hashable {
    let section: Int
    let block: Int
}

/**
 * Each hymn's column width, by the place of its stanzas: its longest metrical line, the opening
 * initial included, capped at 28rem. The web centres `.hymn-verses` on the same measure
 * (`width: fit-content`). A hymn is a run of stanzas, with any rubric or gap among them.
 */
func hymnColumns(_ sections: [SectionView], _ p: Palette, _ o: Ornament, _ m: Metrics) -> [BlockAt: CGFloat] {
    func width(_ t: NSAttributedString) -> CGFloat {
        CGFloat(CTLineGetTypographicBounds(CTLineCreateWithAttributedString(t), nil, nil, nil))
    }
    let style = Scale.verse
    func stanzaWidth(_ block: BlockView) -> CGFloat {
        let text = runs(block, style, color: p.text, p, m)
        let s = text.string as NSString
        var widest: CGFloat = 0
        var start = 0
        var i = 0
        for line in text.string.components(separatedBy: "\n") {
            let length = (line as NSString).length
            let piece = text.attributedSubstring(from: NSRange(location: start, length: length))
            var w = width(piece)
            if block.dropCap && i <= 1 && !line.trimmingCharacters(in: .whitespaces).isEmpty {
                // The initial stands beside the first two lines, which start at its fitted edges.
                let letter = String(line.trimmingCharacters(in: .whitespaces).prefix(1))
                let cap = initial(letter, style, o, m, adapt: .dropped)
                if i == 0 {
                    let after = line.drop { $0.isWhitespace }.dropFirst().drop { $0.isWhitespace }
                    let from = (line as NSString).length - (String(after) as NSString).length
                    w = width(piece.attributedSubstring(from: NSRange(location: from, length: length - from))) + cap.edge + cap.tuck
                } else {
                    w += cap.edge
                }
            }
            widest = max(widest, w)
            start += length + 1
            i += 1
            if start > s.length { break }
        }
        return widest
    }
    var out: [BlockAt: CGFloat] = [:]
    for (si, section) in sections.enumerated() {
        var run: [Int] = []
        var widest: CGFloat = 0
        func close() {
            let column = min((widest + m.px(hymnSlack)).rounded(.up), m.px(hymnMax))
            if widest > 0 { for b in run { out[BlockAt(section: si, block: b)] = column } }
            run = []
            widest = 0
        }
        for (bi, b) in section.blocks.enumerated() {
            switch b.kind {
            case .stanza:
                run.append(bi)
                widest = max(widest, stanzaWidth(b))
            // A hymn's rubric takes its column without widening it (the web's width: 0; min-width: 100%).
            case .hymnRubric:
                run.append(bi)
            case .rubric, .gap:
                break
            default:
                close()
            }
        }
        close()
    }
    return out
}

private let crossMark = "\u{E000}"

private extension String {
    func replacing(pattern: String, with template: String) -> String {
        replacingOccurrences(of: pattern, with: template, options: .regularExpression)
    }
}

/**
 * A block as VoiceOver says it: ℣ and ℟ named, the pointing marks (the mediant's * and the
 * flex †) turned to the pauses they mark, printed verse numbers left silent, and ✠ said as the
 * sign of the cross.
 */
func spoken(_ block: BlockView) -> String {
    var words = block.runs.map { run -> String in
        switch run.style {
        // The pause between half-verses, as the printed psalter's colon.
        case .mediant: return ": "
        case .cross: return crossMark
        case .break: return " "
        // A posture cue within a verse ("Sit.") is an aside to the words around it.
        case .posture: return " (\(run.text.trimmingCharacters(in: .whitespacesAndNewlines))) "
        default: return run.text
        }
    }.joined()
    words = words
        .replacingOccurrences(of: "†", with: ", ")
        .replacingOccurrences(of: "·", with: ".")
        .replacingOccurrences(of: "℣.", with: "Versicle.").replacingOccurrences(of: "℟.", with: "Response.")
        .replacingOccurrences(of: "℣", with: "Versicle").replacingOccurrences(of: "℟", with: "Response")
        .replacing(pattern: "[\\s\\u00a0]+", with: " ")
        .replacing(pattern: "[,;:]?\\s*\(crossMark)\\s*", with: ", sign of the cross, ")
        // No pause doubled: a mark after punctuation, or a comma before it, gives way.
        .replacing(pattern: "([,.;:!?])\\s*[,:]", with: "$1")
        .replacing(pattern: ",\\s*([,.;:!?])", with: "$1")
        .replacing(pattern: " ([,.;:!?])", with: "$1")
        .trimmingCharacters(in: .whitespaces)
    if words.hasPrefix(", ") { words.removeFirst(2) }
    if words.hasSuffix(",") { words.removeLast() }
    if let first = words.first { words = first.uppercased() + words.dropFirst() }
    let marker: String
    switch block.kind {
    case .antiphon, .announcedAntiphon: marker = "Antiphon."
    case .versicle, .response: marker = block.marker.replacingOccurrences(of: "℣.", with: "Versicle.").replacingOccurrences(of: "℟.", with: "Response.")
    case .verse: marker = ""
    default: marker = block.marker
    }
    return [marker, words].filter { !$0.isEmpty }.joined(separator: " ")
}
