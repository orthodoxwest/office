import SwiftUI
import UIKit

/// The verse gutter (`--verse-gutter`, 1.8rem): verse numbers and ℣/℟ sit in it, text beyond it.
let verseGutter: CGFloat = 28.8

/// How far "All:" may hang into the page's margin (`.sigil-all`, 2.25rem less ℣'s 1.4rem).
private let allHang: CGFloat = 13.6

/// How far "Blessing." may hang into a wide page's margin (`.sigil-word`, 4.25rem less ℣'s 1.4rem).
private let wordHang: CGFloat = 45.6

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
    // A whole hymn's scores stand apart, room above each for its number (the web's `.chant-stanza`,
    // 1.4rem); the stanzas after a tune keep a stanza's distance from it.
    if cur.kind == .score && prev.kind == .score { return 22.4 }
    if cur.kind == .stanza && prev.kind == .score { return 12 }
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
    case .heading: return Setting(style: Scale.heading, color: \.titulus, alignment: .center)
    case .commemorationHeading: return Setting(style: Scale.commemorationName, color: \.titulus, alignment: .center)
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

extension NSAttributedString.Key {
    /// Marks a posture cue's run, so a divided opening can keep the cue with its mediant.
    static let posture = NSAttributedString.Key("org.orthodoxwest.office.posture")
    /// Marks a glossed rubric term's run ("secretly") with its note, as the web's `.gloss-term`.
    static let gloss = NSAttributedString.Key("org.orthodoxwest.office.gloss")
}

/// A rubric term's note (presentation::gloss), carried on its run; equal by its words, so a
/// spec compares equal across renders.
final class GlossNote: NSObject {
    let title: String
    let note: String

    init(_ gloss: GlossView) {
        title = gloss.title
        note = gloss.note
    }

    override func isEqual(_ object: Any?) -> Bool {
        guard let other = object as? GlossNote else { return false }
        return title == other.title && note == other.note
    }

    override var hash: Int { title.hashValue ^ note.hashValue }
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
            // "Commemoration of", on its own line in the heading's small caps and tracking, as the
            // web's `.commemoration-kicker`; the name beneath stays in mixed case.
            a[.font] = garamond(size * 0.78, italic: style.italic, smallCaps: true, lining: style.lining)
            a[.foregroundColor] = UIColor(p.muted)
            a[.kern] = Scale.heading.tracking * k
        case .posture:
            a[.font] = garamond(size * 0.9, italic: style.italic, smallCaps: style.smallCaps, lining: style.lining)
            a[.foregroundColor] = UIColor(p.rubric)
            a[.posture] = true
        case .numeral:
            // A psalm's numbers ("66 (67)") out of its label's small caps, as the web's `.psalm-num`:
            // a small-cap "1" is a bare stem that reads as the section numeral "i".
            a[.font] = garamond(size * 0.9, italic: style.italic, smallCaps: false, lining: true)
            a[.kern] = 0.02 * size * 0.9
        case .hebrewNumeral:
            // The Hebrew number beside it, "(67)", muted and a little smaller, as `.psalm-heb`
            // (0.86 of the numeral's 0.9).
            a[.font] = garamond(size * 0.774, italic: style.italic, smallCaps: false, lining: true)
            a[.kern] = 0.02 * size * 0.774
            a[.foregroundColor] = UIColor(p.muted)
        }
        if let gloss = run.gloss { a[.gloss] = GlossNote(gloss) }
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
/// `hangsWords`: the page is 1000pt wide or more, where "Blessing." hangs into the margin as "All:" does.
func proseSpec(_ block: BlockView, _ p: Palette, _ o: Ornament, _ m: Metrics, hangsWords: Bool = false) -> ProseSpec {
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
        let word = block.marker.count > 2 && block.kind != .all
        if word && !hangsWords {
            // "Blessing." is too wide to hang in the page's gutter: below 1000pt it stands above its
            // words (the web's `.sigil-word` to 999px).
            let t = marked(block.marker, Scale.body, p.rubric)
            t.append(NSAttributedString(string: "\u{2028}", attributes: [.font: font]))
            t.append(text)
            spec.text = t
            spec.restIndent = gutter
        } else {
            // The sigil set right in its gutter; the gutter keeps its stop at any size. "All:" is
            // wider than ℣./℟. and hangs its extra width into the page's gutter, so its words keep
            // the edge the ℣/℟ lines' words share (the web's `.sigil-all`): the block reaches out
            // by `hang` and its stops move in by as much. From 1000pt "Blessing." hangs so too.
            let mark = marked(block.marker, Scale.body, p.rubric)
            let t = NSMutableAttributedString(string: "\t", attributes: [.font: font])
            t.append(mark)
            t.append(NSAttributedString(string: "\t", attributes: [.font: font]))
            t.append(text)
            spec.text = t
            let sigil = m.px(22)
            let width = CGFloat(CTLineGetTypographicBounds(CTLineCreateWithAttributedString(NSAttributedString(attributedString: mark)), nil, nil, nil))
            let hang = block.kind == .all ? min(m.px(allHang), max(0, width - sigil)) : word ? min(m.px(wordHang), max(0, width - sigil)) : 0
            let stop = max(sigil, width - hang) + hang
            spec.tabs = [NSTextTab(textAlignment: .right, location: stop), NSTextTab(textAlignment: .left, location: stop + m.px(6.4))]
            spec.restIndent = stop + m.px(6.4)
            spec.hang = hang
        }
    case .stanza:
        // A wrapped line hangs beneath its own start (`.hymn-line`, 1.1rem).
        spec.restIndent = 17.6 * m.type
        if block.dropCap { return opening(textStart: 0) }
    case .paragraph:
        // A prose opening (a chapter, a collect, the Our Father) turns over below its initial at the
        // edge its ℟'s words share, the initial hung in the gutter, as a psalm's first verse does.
        if block.dropCap { return opening(textStart: gutter) }
    case .chantLine:
        // A sung line's wrap hangs beneath its own start (`.chant-line`, 1.2rem).
        spec.restIndent = 19.2 * m.type
        if block.dropCap { return opening(textStart: 0) }
    default:
        break
    }
    return spec
}

/// A psalm label's part (its title, or its Latin incipit) as a block of its own.
private func labelPart(_ block: BlockView, _ runs: ArraySlice<RunView>) -> BlockView {
    var part = block
    part.runs = Array(runs)
    return part
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
    @Environment(\.wide) private var wide
    @Environment(\.ranked) private var ranked
    /// A rubric's glossed term whose note is open, and that term's box.
    @State private var openGloss: Int?
    @State private var glossRect: CGRect = .zero
    /// The last term opened, kept while its note closes so the card does not empty as it goes.
    @State private var shownGloss = 0

    var body: some View {
        if block.kind == .gap {
            Color.clear.frame(height: m.px(8)).accessibilityHidden(true)
        } else if block.kind == .score {
            ScoreRow(block: block)
        } else {
            let spec = proseSpec(block, p, o, m, hangsWords: wide && !ranked)
            let glosses = block.runs.compactMap(\.gloss)
            let prose = Prose(
                spec: spec,
                spoken: spoken(block),
                header: block.kind == .heading || block.kind == .commemorationHeading,
                openGloss: openGloss,
                onGloss: glosses.isEmpty ? nil : { i, rect in
                    glossRect = rect
                    if openGloss != i { shownGloss = i }
                    openGloss = openGloss == i ? nil : i
                }
            )
            if block.kind == .itemLabel, let latin = block.runs.firstIndex(where: { $0.style == .latin }), latin > 0 {
                // A psalm's number and its Latin incipit share a line, a dot between them. When the
                // line cannot hold both, the incipit stands on its own line beneath and the dot goes
                // (the web's `.item-label.stacked`): a separator ending a line alone separates nothing.
                let title = labelPart(block, block.runs[..<(latin - 1)])
                let incipit = labelPart(block, block.runs[latin...])
                ViewThatFits(in: .horizontal) {
                    prose.fixedSize(horizontal: true, vertical: false)
                    VStack(spacing: 0) {
                        Prose(spec: proseSpec(title, p, o, m))
                        Prose(spec: proseSpec(incipit, p, o, m))
                    }
                }
                .frame(maxWidth: .infinity)
                .accessibilityElement(children: .ignore)
                .accessibilityLabel(spoken(block))
            } else if block.kind == .stanza && !block.marker.isEmpty {
                // A verse after a tune shown once: its number hangs in the rubric's red before its
                // first line, as a hymnal numbers its verses (the web's `.hymn-stanza-number`), in an
                // indent of its own so it stays inside a phone's margin.
                prose.frame(maxWidth: column ?? .infinity)
                    .padding(.leading, m.px(stanzaNumberHang))
                    .overlay(alignment: .topLeading) {
                        Text(block.marker).font(Font(garamond(Scale.verse.size * 0.85 * m.type)))
                            .foregroundStyle(p.rubric)
                            .frame(width: m.px(stanzaNumberHang - 4), alignment: .trailing)
                            .offset(y: m.px(3))
                            .accessibilityHidden(true)
                    }
                    .frame(maxWidth: .infinity)
            } else if block.kind == .stanza || block.kind == .hymnRubric {
                // The hymn's column, centred: the rag balanced by an equal indent on the left,
                // as the web's fit-content `.hymn-verses`.
                prose.frame(maxWidth: column ?? .infinity).frame(maxWidth: .infinity)
            } else if !glosses.isEmpty {
                // A rubric term's note opens over the word, so it never covers the prayer it
                // explains (the web's `.gloss-note`); a tap elsewhere puts it away.
                prose
                    // The arrow on the card's bottom edge, pointing down to the word; iOS may still
                    // place the card itself where the room above is short.
                    .popover(
                        isPresented: Binding(get: { openGloss != nil }, set: { if !$0 { openGloss = nil } }),
                        attachmentAnchor: .rect(.rect(glossRect)),
                        arrowEdge: .bottom
                    ) {
                        if shownGloss < glosses.count { GlossCard(gloss: glosses[shownGloss]) }
                    }
                    .padding(.leading, -spec.hang)
            } else {
                // A sigil hung into the margin ("All:") takes the room it hangs into.
                prose.padding(.leading, -spec.hang)
            }
        }
    }
}

/**
 * A rubric term's note, as the web's `.gloss-note`: a card of the surface, its heading in the
 * rubric's red small caps over the note in the text's ink, 19rem wide.
 */
private struct GlossCard: View {
    let gloss: GlossView
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        VStack(alignment: .leading, spacing: m.px(3.2)) {
            Text(gloss.title)
                .font(Font(garamond(14.44 * m.type, smallCaps: true)))
                .kerning(0.72 * m.type)
                .foregroundStyle(p.rubric)
            Text(gloss.note)
                .font(Font(garamond(15.2 * m.type)))
                .lineSpacing(3 * m.type)
                .foregroundStyle(p.text)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(.horizontal, m.px(14.4))
        .padding(.top, m.px(11.2))
        .padding(.bottom, m.px(12.8))
        .frame(width: m.px(304), alignment: .leading)
        .presentationCompactAdaptation(.popover)
        .presentationBackground(p.surface)
        .accessibilityElement(children: .combine)
    }
}

/// The web's `.hymn-verses` max-width, 28rem.
let hymnMax: CGFloat = 448
/// How far a verse's number hangs before its column (the web's `.hymn-stanza-number`, 1.5rem).
private let stanzaNumberHang: CGFloat = 24

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
