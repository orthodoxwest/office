import SwiftUI

/**
 * An SVG path's `d`, as the web's templates draw its ornaments: moves, lines, curves and arcs,
 * absolute and relative, with repeated coordinates continuing the command before them.
 */
func svg(_ d: String) -> Path {
    var path = Path()
    var tokens = SVGTokens(d)
    var command: Character = "M"
    var current = CGPoint.zero
    var start = CGPoint.zero
    while let next = tokens.peek() {
        if case let .command(c) = next {
            tokens.advance()
            command = c
            if c == "Z" || c == "z" {
                path.closeSubpath()
                current = start
                continue
            }
        }
        let relative = command.isLowercase
        let origin = relative ? current : .zero
        func point() -> CGPoint? {
            guard let x = tokens.number(), let y = tokens.number() else { return nil }
            return CGPoint(x: origin.x + x, y: origin.y + y)
        }
        switch command {
        case "M", "m":
            guard let p = point() else { return path }
            path.move(to: p)
            current = p
            start = p
            // Further pairs after a move are lines.
            command = relative ? "l" : "L"
        case "L", "l":
            guard let p = point() else { return path }
            path.addLine(to: p)
            current = p
        case "H", "h":
            guard let x = tokens.number() else { return path }
            current = CGPoint(x: (relative ? current.x : 0) + x, y: current.y)
            path.addLine(to: current)
        case "V", "v":
            guard let y = tokens.number() else { return path }
            current = CGPoint(x: current.x, y: (relative ? current.y : 0) + y)
            path.addLine(to: current)
        case "C", "c":
            guard let c1 = point(), let c2 = point(), let p = point() else { return path }
            path.addCurve(to: p, control1: c1, control2: c2)
            current = p
        case "A", "a":
            guard let rx = tokens.number(), let ry = tokens.number(), let rotation = tokens.number(),
                  let large = tokens.number(), let sweep = tokens.number(), let p = point() else { return path }
            addArc(&path, from: current, to: p, rx: rx, ry: ry, rotation: rotation, large: large != 0, sweep: sweep != 0)
            current = p
        default:
            return path
        }
    }
    return path
}

private enum SVGToken {
    case command(Character)
    case number(CGFloat)
}

private struct SVGTokens {
    private var tokens: [SVGToken] = []
    private var at = 0

    init(_ d: String) {
        var number = ""
        func flush() {
            if let v = Double(number) { tokens.append(.number(CGFloat(v))) }
            number = ""
        }
        for ch in d {
            if ch.isLetter && ch != "e" && ch != "E" {
                flush()
                tokens.append(.command(ch))
            } else if ch == "-" || ch == "+" {
                // A sign starts a number, unless it follows an exponent.
                if let last = number.last, last == "e" || last == "E" {
                    number.append(ch)
                } else {
                    flush()
                    number.append(ch)
                }
            } else if ch == "." {
                // A second point starts the next number: "0.5.5" is 0.5 and .5.
                if number.contains(".") && !number.contains("e") { flush() }
                number.append(ch)
            } else if ch.isNumber || ch == "e" || ch == "E" {
                number.append(ch)
            } else {
                flush()
            }
        }
        flush()
    }

    func peek() -> SVGToken? { at < tokens.count ? tokens[at] : nil }
    mutating func advance() { at += 1 }

    mutating func number() -> CGFloat? {
        guard case let .number(v)? = peek() else { return nil }
        at += 1
        return v
    }
}

/// An elliptical arc as cubic Béziers of at most a quarter turn each (SVG 1.1 §F.6.5).
private func addArc(_ path: inout Path, from p0: CGPoint, to p1: CGPoint, rx: CGFloat, ry: CGFloat, rotation: CGFloat, large: Bool, sweep: Bool) {
    var rx = abs(rx), ry = abs(ry)
    guard rx > 0, ry > 0, p0 != p1 else {
        path.addLine(to: p1)
        return
    }
    let phi = rotation * .pi / 180
    let (c, s) = (cos(phi), sin(phi))
    let dx = (p0.x - p1.x) / 2, dy = (p0.y - p1.y) / 2
    let x1 = c * dx + s * dy, y1 = -s * dx + c * dy
    let lambda = (x1 * x1) / (rx * rx) + (y1 * y1) / (ry * ry)
    if lambda > 1 {
        rx *= sqrt(lambda)
        ry *= sqrt(lambda)
    }
    let num = rx * rx * ry * ry - rx * rx * y1 * y1 - ry * ry * x1 * x1
    let den = rx * rx * y1 * y1 + ry * ry * x1 * x1
    var k = sqrt(max(0, num / den))
    if large == sweep { k = -k }
    let cx1 = k * rx * y1 / ry, cy1 = -k * ry * x1 / rx
    let cx = c * cx1 - s * cy1 + (p0.x + p1.x) / 2
    let cy = s * cx1 + c * cy1 + (p0.y + p1.y) / 2
    func angle(_ ux: CGFloat, _ uy: CGFloat, _ vx: CGFloat, _ vy: CGFloat) -> CGFloat {
        let a = atan2(ux * vy - uy * vx, ux * vx + uy * vy)
        return a
    }
    let theta = angle(1, 0, (x1 - cx1) / rx, (y1 - cy1) / ry)
    var delta = angle((x1 - cx1) / rx, (y1 - cy1) / ry, (-x1 - cx1) / rx, (-y1 - cy1) / ry)
    if !sweep && delta > 0 { delta -= 2 * .pi }
    if sweep && delta < 0 { delta += 2 * .pi }
    let segments = max(1, Int(ceil(abs(delta) / (.pi / 2))))
    let step = delta / CGFloat(segments)
    let t = 4 / 3 * tan(step / 4)
    func onEllipse(_ a: CGFloat) -> (CGPoint, CGPoint) {
        // The point at angle `a`, and its derivative.
        let (ca, sa) = (cos(a), sin(a))
        let point = CGPoint(x: cx + rx * ca * c - ry * sa * s, y: cy + rx * ca * s + ry * sa * c)
        let slope = CGPoint(x: -rx * sa * c - ry * ca * s, y: -rx * sa * s + ry * ca * c)
        return (point, slope)
    }
    var a = theta
    for i in 0..<segments {
        let (from, d0) = onEllipse(a)
        let (to, d1) = onEllipse(a + step)
        let end = i == segments - 1 ? p1 : to
        path.addCurve(
            to: end,
            control1: CGPoint(x: from.x + t * d0.x, y: from.y + t * d0.y),
            control2: CGPoint(x: to.x - t * d1.x, y: to.y - t * d1.y)
        )
        a += step
    }
}

extension Path {
    /**
     * This path, drawn in a `vw`×`vh` viewBox, fitted and centred in `rect` as SVG's default
     * `meet`; mirrored left to right, or flipped top to bottom, within the box.
     */
    func fitted(_ vw: CGFloat, _ vh: CGFloat, in rect: CGRect, mirror: Bool = false, flip: Bool = false) -> Path {
        let k = min(rect.width / vw, rect.height / vh)
        var t = CGAffineTransform(translationX: rect.minX + (rect.width - vw * k) / 2, y: rect.minY + (rect.height - vh * k) / 2)
        t = t.scaledBy(x: k, y: k)
        if mirror { t = t.translatedBy(x: vw, y: 0).scaledBy(x: -1, y: 1) }
        if flip { t = t.translatedBy(x: 0, y: vh).scaledBy(x: 1, y: -1) }
        return applying(t)
    }
}
