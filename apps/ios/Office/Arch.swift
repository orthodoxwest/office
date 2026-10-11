import SwiftUI

/**
 * Home's pointed head, the web's (tools/genarch.py): a steep four-centred arch, short haunch arcs
 * centred on the springing line so the head leaves the jambs without a kink, then long upper arcs
 * centred below it, meeting at the point. Its figures are in card widths: the head rises `rise`
 * from the springing to the point; each haunch is an arc of radius `haunch` centred on the
 * springing line `haunch` in from its side; each upper arc, of radius `radius`, is centred
 * `centre` in from its side and `depth` below the springing.
 */
struct Arch {
    let rise: CGFloat
    let haunch: CGFloat
    let centre: CGFloat
    let depth: CGFloat
    let radius: CGFloat
}

// genarch:begin: tools/genarch.py writes this block; edit the script, not the figures.
extension Arch {
    /// The phone head, for a phone: a 144° point.
    static let phone = Arch(rise: 0.44, haunch: 0.3, centre: 0.9, depth: 0.781098, radius: 1.284944)
    /// The tall head, for a phone from 800 high: a 131° point.
    static let tall = Arch(rise: 0.56, haunch: 0.4, centre: 1, depth: 0.535519, radius: 1.204227)
    /// The taller head, for a phone from 880 high: a 123° point.
    static let taller = Arch(rise: 0.64, haunch: 0.4, centre: 1.05, depth: 0.364522, radius: 1.145236)
    /// The niche head, for a wide screen's niche: a 146° point.
    static let niche = Arch(rise: 0.4, haunch: 0.24, centre: 0.86, depth: 0.795773, radius: 1.248789)
}
// genarch:end

/**
 * The head of a card `width` wide, `d` beyond its edge (inside it when negative), offset as a
 * moulding's lines are: each arc keeps its centre and gains `d` on its radius, and the upper arcs
 * meet on the centre line. The card's top is the point of the head itself, its springing `rise`
 * widths below; the jambs run down to `foot`. `origin` is the card's top left. `open` leaves the
 * foot unclosed, for a line painted round the head.
 */
func archPath(_ a: Arch, width: CGFloat, outset d: CGFloat, foot: CGFloat, at origin: CGPoint = .zero, open: Bool = false) -> Path {
    let spring = a.rise * width
    let haunch = a.haunch * width
    let centre = a.centre * width
    let below = spring + a.depth * width
    let radius = a.radius * width + d
    // Angles clockwise from three o'clock, as the screen's y runs down: the haunch meets the upper
    // arc on the line through their centres, and the upper arcs meet on the centre line.
    let tangent = atan2(-a.depth, a.haunch - a.centre) + 2 * .pi
    let across = width / 2 - centre
    let point = atan2(-(radius * radius - across * across).squareRoot(), across) + 2 * .pi
    func at(_ x: CGFloat, _ y: CGFloat) -> CGPoint { CGPoint(x: origin.x + x, y: origin.y + y) }
    func arc(_ x: CGFloat, _ y: CGFloat, _ r: CGFloat, from start: CGFloat, by sweep: CGFloat, in p: inout Path) {
        p.addArc(center: at(x, y), radius: r, startAngle: .radians(start), endAngle: .radians(start + sweep), clockwise: false)
    }
    var p = Path()
    p.move(to: at(-d, foot))
    p.addLine(to: at(-d, spring))
    arc(haunch, spring, haunch + d, from: .pi, by: tangent - .pi, in: &p)
    arc(centre, below, radius, from: tangent, by: point - tangent, in: &p)
    arc(width - centre, below, radius, from: 3 * .pi - point, by: point - tangent, in: &p)
    arc(width - haunch, spring, haunch + d, from: 3 * .pi - tangent, by: tangent - .pi, in: &p)
    p.addLine(to: at(width + d, foot))
    if !open { p.closeSubpath() }
    return p
}

/**
 * How wide the head of a card `width` wide stands `y` below the card's top, between lines `d`
 * beyond its edges (inside them when negative) as `archPath` draws them: the jambs below the
 * springing, the haunches above it, then the upper arcs, and nothing above the point.
 */
func archChord(_ a: Arch, width: CGFloat, outset d: CGFloat, at y: CGFloat) -> CGFloat {
    let spring = a.rise * width
    let haunch = a.haunch * width
    let centre = a.centre * width
    let below = spring + a.depth * width
    // The haunch gives way to the upper arc on the line through their centres.
    let join = spring - (haunch + d) * a.depth / (a.depth * a.depth + (a.centre - a.haunch) * (a.centre - a.haunch)).squareRoot()
    let left: CGFloat
    if y >= spring {
        left = -d
    } else if y >= join {
        left = haunch - ((haunch + d) * (haunch + d) - (spring - y) * (spring - y)).squareRoot()
    } else {
        let radius = a.radius * width + d
        let up = below - y
        if up >= radius { return 0 }
        left = centre - (radius * radius - up * up).squareRoot()
    }
    return max(0, width - 2 * left)
}
