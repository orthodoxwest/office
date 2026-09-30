import SwiftUI

/// The verse gutter (`--verse-gutter`, 1.8rem): verse numbers and ℣/℟ sit in it, text beyond it.
let verseGutter: CGFloat = 28.8

/**
 * Space above `cur`, after `prev`, in the web's rhythm at a phone's width: the Android app's
 * `gapBefore`, which was measured from the rendered hour.
 */
func gapBefore(_ prev: BlockView?, _ cur: BlockView) -> CGFloat {
    guard let prev else { return 0 }
    let heading = { (b: BlockView) in b.kind == .heading || b.kind == .commemorationHeading }
    if heading(cur) { return 38 }
    if heading(prev) { return cur.kind == .chapterRef ? 24 : 27 }
    if cur.kind == .gap || prev.kind == .gap { return 4.8 }
    if prev.kind == .itemLabel { return 9 }
    if cur.kind == .itemLabel { return prev.kind == .antiphon ? 10 : 30 }
    if prev.kind == .chapterRef { return 15 }
    if prev.kind == .latinTitle { return 8 }
    if prev.kind == .speaker { return 3.2 }
    // A closing antiphon, then the next group's opening one: the threshold between groups.
    if cur.kind == .antiphon && prev.kind == .antiphon { return 49 }
    // A closing antiphon sits close under its psalm's last verse or Gloria.
    if cur.kind == .antiphon && (prev.kind == .verse || (prev.kind == .paragraph && !prev.startsElement)) { return 6 }
    if cur.kind == .verse && prev.kind == .verse { return 4.8 }
    // The Gloria Patri after a psalm's last verse.
    if cur.kind == .paragraph && prev.kind == .verse { return 13.6 }
    if cur.kind == .stanza && prev.kind == .stanza { return 12 }
    if cur.startsElement { return 14 }
    return 4.8
}

/// A block's runs as styled text, each run in its web class's colour and face.
func runs(_ block: BlockView, _ p: Palette, base: TextStyle) -> AttributedString {
    var out = AttributedString()
    for run in block.runs {
        var piece = AttributedString(run.text)
        switch run.style {
        case .plain, .break, .prayed:
            break
        case .mediant, .secret:
            // The pointing asterisk is quiet, as are words said silently.
            piece.foregroundColor = p.muted
        case .cross:
            piece.foregroundColor = p.rubric
            piece.font = crossFont(size: base.size * 0.8)
        case .latin:
            var italic = base
            italic.italic = true
            italic.smallCaps = false
            piece.font = italic.font
        case .kicker:
            var small = base
            small.size = base.size * 0.7
            piece.font = small.font
            piece.foregroundColor = p.muted
        case .posture:
            var cue = base
            cue.size = base.size * 0.9
            piece.font = cue.font
            piece.foregroundColor = p.rubric
        }
        out.append(piece)
    }
    return out
}

/// One block of a composed hour, styled after the web's classes for the same text.
struct BlockRow: View {
    let block: BlockView
    let p: Palette

    var body: some View {
        switch block.kind {
        case .gap:
            Color.clear.frame(height: 8)
        case .heading, .commemorationHeading:
            centred(Scale.heading, color: p.text)
        case .itemLabel:
            centred(Scale.itemLabel, color: p.muted)
        case .latinTitle, .canticleSection:
            centred(Scale.bodyItalic, color: p.muted)
        case .chapterRef, .scriptureRef:
            centred(Scale.reference, color: p.rubric)
        case .rubric:
            text(Scale.rubric, color: p.rubric)
        case .speaker:
            text(Scale.speaker, color: p.rubric)
        case .antiphon:
            // Body antiphons hang left: the sigil opens the line.
            let sigil = Text(block.marker).font(TextStyle(size: 20, line: 32, tracking: 1.4, smallCaps: true).font).foregroundStyle(p.rubric)
            (sigil + Text(" ") + Text(runs(block, p, base: Scale.body)).foregroundStyle(p.text))
                .style(Scale.body)
                .frame(maxWidth: .infinity, alignment: .leading)
        case .verse:
            HStack(alignment: .firstTextBaseline, spacing: 0) {
                Text(block.marker)
                    .style(Scale.verseNumber)
                    .foregroundStyle(p.muted)
                    .frame(width: verseGutter - 8, alignment: .trailing)
                    .padding(.trailing, 8)
                Text(runs(block, p, base: Scale.verse)).style(Scale.verse).foregroundStyle(p.text)
                Spacer(minLength: 0)
            }
        case .versicle, .response, .all:
            if block.marker.isEmpty {
                text(Scale.body, color: p.text)
            } else if block.marker.count > 2 {
                // A spelled-out sigil ("Blessing.", "All:") stands above the text.
                VStack(alignment: .leading, spacing: 0) {
                    Text(block.marker).style(Scale.body).foregroundStyle(p.rubric)
                    Text(runs(block, p, base: Scale.body)).style(Scale.body).foregroundStyle(p.text).padding(.leading, verseGutter)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            } else {
                HStack(alignment: .firstTextBaseline, spacing: 6.4) {
                    Text(block.marker).style(Scale.body).foregroundStyle(p.rubric).frame(minWidth: 22, alignment: .trailing)
                    Text(runs(block, p, base: Scale.body)).style(Scale.body).foregroundStyle(p.text)
                    Spacer(minLength: 0)
                }
            }
        case .stanza:
            text(Scale.verse, color: p.text)
        case .paragraph, .chantLine:
            text(Scale.body, color: p.text)
        }
    }

    private func text(_ s: TextStyle, color: Color) -> some View {
        Text(runs(block, p, base: s)).style(s).foregroundStyle(color).frame(maxWidth: .infinity, alignment: .leading)
    }

    private func centred(_ s: TextStyle, color: Color) -> some View {
        Text(runs(block, p, base: s)).style(s).foregroundStyle(color).multilineTextAlignment(.center).frame(maxWidth: .infinity)
    }
}
