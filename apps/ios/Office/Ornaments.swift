import SwiftUI
import UIKit

// Headpiece sprig (macros.html `headpiece`), viewBox 56×20.
private let sprigRule = svg("M2 10h52")
private let sprigStem = svg("M2 15C20 15 34 11 54 5")
private let sprigLeaves = svg(
    "M14 14C8 14 7 9 6 6c6 0 9 3 8 8Zm12-2c-6-1-7-6-7-9 6 1 9 4 7 9Zm11-3c-5-2-5-6-4-9 5 3 7 5 4 9Z" +
        "M16 14c-3 4-7 5-10 4 2-4 6-5 10-4Zm14-3c-2 5-6 7-10 7 1-4 5-7 10-7Zm14-4c-1 5-5 7-9 7 1-4 5-7 9-7Z"
)

// Home frame corner (home.html), viewBox 32×32.
private let cornerRule = svg("M3 29V3h26")
private let cornerLeaf = svg("M7 7c7 0 12 4 13 11C13 18 8 14 7 7Zm0 0 9 8")

/// Home's period engravings (home.html), viewBox 24×16, stroked.
enum Period: CaseIterable {
    case morning, day, evening

    var paths: [Path] {
        switch self {
        case .morning: return [svg("M2 13h20M7 13a5 5 0 0 1 10 0M12 2v3M4.2 5.2l2.1 2.1M19.8 5.2l-2.1 2.1")]
        case .day: return [
            svg("M8.5 8a3.5 3.5 0 1 0 7 0a3.5 3.5 0 1 0 -7 0"),
            svg("M12 0.75v1.5M12 13.75v1.5M4.75 8h1.5M17.75 8h1.5M6.9 2.9l1.05 1.05M16.05 12.05l1.05 1.05M6.9 13.1l1.05-1.05M16.05 3.95l1.05-1.05"),
        ]
        case .evening: return [svg("m12 1 1.8 4.7L19 7.5l-5.2 1.8L12 14l-1.8-4.7L5 7.5l5.2-1.8Z")]
        }
    }
}

// The ordo's abstinence fish (calendar.html `icon-fish`), viewBox 24×12.
private let fish = svg("M1 6 C5 1.2 13 1.2 17 6 C13 10.8 5 10.8 1 6 Z M16.5 6 L23 1.5 L21.2 6 L23 10.5 Z")

// The Apse vault tile (style.css `--apse-star-tile`), viewBox 132×132.
private let vaultRibs = svg("M0 0L132 132M0 66L66 0M66 132L132 66")
private let vaultStars = svg(
    "M33 25.5L33.52 31.75L36.39 29.61L34.25 32.48L40.5 33L34.25 33.52L36.39 36.39L33.52 34.25L33 40.5L32.48 34.25L29.61 36.39L31.75 33.52L25.5 33L31.75 32.48L29.61 29.61L32.48 31.75Z" +
        "M99 91.5L99.52 97.75L102.39 95.61L100.25 98.48L106.5 99L100.25 99.52L102.39 102.39L99.52 100.25L99 106.5L98.48 100.25L95.61 102.39L97.75 99.52L91.5 99L97.75 98.48L95.61 95.61L98.48 97.75Z"
)
private let vaultSparks = svg("M99 28.2L99.64 32.36L102.2 33L99.64 33.64L99 37.8L98.36 33.64L95.8 33L98.36 32.36ZM33 94.2L33.64 98.36L36.2 99L33.64 99.64L33 103.8L32.36 99.64L29.8 99L32.36 98.36Z")

/// The scale at which a `vw`×`vh` viewBox meets `size`.
private func meet(_ vw: CGFloat, _ vh: CGFloat, _ size: CGSize) -> CGFloat { min(size.width / vw, size.height / vh) }

private struct Sprig: View {
    let mirror: Bool
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            let rect = CGRect(origin: .zero, size: size)
            let k = meet(56, 20, size)
            ctx.stroke(sprigRule.fitted(56, 20, in: rect, mirror: mirror), with: .color(o.flat), lineWidth: 0.8 * k)
            ctx.stroke(sprigStem.fitted(56, 20, in: rect, mirror: mirror), with: .color(o.flat), lineWidth: 0.8 * k)
            ctx.fill(sprigLeaves.fitted(56, 20, in: rect, mirror: mirror), with: .color(o.flat))
        }
        .frame(width: m.px(53), height: m.px(14.4))
        .opacity(0.55)
    }
}

/// The ✠ in the cross face, in the gilding or `color`.
struct Cross: View {
    var size: CGFloat = 11
    var color: Color?
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        Text("✠").font(Font(crossUIFont(size * m.type))).foregroundStyle(color ?? o.flat)
    }
}

/// The headpiece above a page's title: sprig, cross, sprig.
struct Headpiece: View {
    @Environment(\.metrics) private var m

    var body: some View {
        HStack(spacing: m.px(10)) {
            Sprig(mirror: false)
            Cross()
            Sprig(mirror: true)
        }
        .accessibilityHidden(true)
    }
}

/// The web's plain headpiece: a short rule either side of the cross, set to the left.
struct PlainHeadpiece: View {
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        HStack(spacing: m.px(8)) {
            Rectangle().fill(o.line).frame(width: m.px(22), height: 1)
            Cross()
            Rectangle().fill(o.line).frame(width: m.px(22), height: 1)
        }
        .accessibilityHidden(true)
    }
}

/// A diamond about `center`, the lorica boards' lozenge.
func lozenge(_ center: CGPoint, _ r: CGFloat) -> Path {
    var p = Path()
    p.move(to: CGPoint(x: center.x, y: center.y - r))
    p.addLine(to: CGPoint(x: center.x + r, y: center.y))
    p.addLine(to: CGPoint(x: center.x, y: center.y + r))
    p.addLine(to: CGPoint(x: center.x - r, y: center.y))
    p.closeSubpath()
    return p
}

/**
 * The double gold hairline that frames a title, broken by `gap` at its centre (where the
 * headpiece sits) and optionally interrupted by a lozenge in the day's colour.
 */
struct DoubleRule: View {
    var gap: CGFloat = 0
    var lozengeColor: Color?
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        let half = m.px(gap) / 2
        let offset = m.px(1.5)
        let r = m.px(4.5)
        Canvas { ctx, size in
            let mid = size.height / 2
            for y in [mid - offset, mid + offset] {
                var p = Path()
                if half > 0 {
                    p.move(to: CGPoint(x: 0, y: y))
                    p.addLine(to: CGPoint(x: size.width / 2 - half, y: y))
                    p.move(to: CGPoint(x: size.width / 2 + half, y: y))
                    p.addLine(to: CGPoint(x: size.width, y: y))
                } else {
                    p.move(to: CGPoint(x: 0, y: y))
                    p.addLine(to: CGPoint(x: size.width, y: y))
                }
                ctx.stroke(p, with: .color(o.line), lineWidth: 1)
            }
            if let lozengeColor {
                let l = lozenge(CGPoint(x: size.width / 2, y: mid), r)
                ctx.fill(l, with: .color(lozengeColor))
                ctx.stroke(l, with: .color(o.flat), lineWidth: 0.8)
            }
        }
        .frame(height: m.px(9))
        .frame(maxWidth: .infinity)
        .accessibilityHidden(true)
    }
}

/// A small free-standing gilt lozenge, as the ✦ that closes a page.
struct Diamond: View {
    var size: CGFloat = 7
    var color: Color?
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, s in
            ctx.fill(lozenge(CGPoint(x: s.width / 2, y: s.height / 2), min(s.width, s.height) / 2), with: .color(color ?? o.flat))
        }
        .frame(width: m.px(size), height: m.px(size))
        .accessibilityHidden(true)
    }
}

/// One of the four tooled corners of home's frontispiece.
struct FrameCorner: View {
    let mirror: Bool
    let flip: Bool
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            let rect = CGRect(origin: .zero, size: size)
            let k = meet(32, 32, size)
            ctx.stroke(cornerRule.fitted(32, 32, in: rect, mirror: mirror, flip: flip), with: .color(o.flat), lineWidth: k)
            ctx.stroke(cornerLeaf.fitted(32, 32, in: rect, mirror: mirror, flip: flip), with: .color(o.flat), lineWidth: k)
        }
        .frame(width: m.px(26), height: m.px(26))
        .opacity(0.5)
        .accessibilityHidden(true)
    }
}

struct PeriodIcon: View {
    let period: Period
    let color: Color
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            let rect = CGRect(origin: .zero, size: size)
            let k = meet(24, 16, size)
            for p in period.paths {
                ctx.stroke(p.fitted(24, 16, in: rect), with: .color(color), style: StrokeStyle(lineWidth: 1.2 * k, lineCap: .butt))
            }
        }
        .frame(width: m.px(19), height: m.px(13))
        .opacity(0.7)
        .accessibilityHidden(true)
    }
}

struct FishIcon: View {
    let color: Color
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            ctx.fill(fish.fitted(24, 12, in: CGRect(origin: .zero, size: size)), with: .color(color))
        }
        .frame(width: m.px(18), height: m.px(9))
        .accessibilityHidden(true)
    }
}

/// The plaster wall behind every page, baked from the web's layers (apps/android/tools/bake-plaster.py).
struct PlasterWall: View {
    @Environment(\.palette) private var p

    var body: some View {
        GeometryReader { geo in
            if let image = UIImage(named: p.plaster) {
                Image(uiImage: image)
                    .resizable()
                    .scaledToFill()
                    .frame(width: geo.size.width, height: geo.size.height)
                    .clipped()
            } else {
                p.bg
            }
        }
        .ignoresSafeArea()
        .accessibilityHidden(true)
    }
}

/**
 * A field of the Apse vault (apse-vault.md): a diaper of hairline ribs with eight-ray stars,
 * their soft halos, and four-ray sparks, in the gilding, faded down the field by stops of
 * (fraction, opacity). The Nave has none.
 */
struct VaultField: View {
    let fade: [(CGFloat, Double)]
    @Environment(\.palette) private var p
    @Environment(\.ornament) private var o

    var body: some View {
        if p.dark {
            let ink = o.flat
            Canvas { ctx, size in
                let tile: CGFloat = 132
                // Phase from the top centre, as the web anchors home's field.
                let x0 = (size.width / 2).truncatingRemainder(dividingBy: tile) - tile
                var y: CGFloat = 0
                while y < size.height {
                    var x = x0
                    while x < size.width {
                        var t = ctx
                        t.translateBy(x: x, y: y)
                        for c in [CGPoint(x: 33, y: 33), CGPoint(x: 99, y: 99)] {
                            let halo = Path(ellipseIn: CGRect(x: c.x - 12, y: c.y - 12, width: 24, height: 24))
                            t.fill(halo, with: .radialGradient(
                                Gradient(stops: [
                                    .init(color: ink.opacity(0.28), location: 0),
                                    .init(color: ink.opacity(0.09), location: 0.3),
                                    .init(color: ink.opacity(0), location: 1),
                                ]),
                                center: c, startRadius: 0, endRadius: 12
                            ))
                            t.fill(Path(ellipseIn: CGRect(x: c.x - 1.7, y: c.y - 1.7, width: 3.4, height: 3.4)), with: .color(ink.opacity(0.78)))
                        }
                        t.stroke(vaultRibs, with: .color(ink.opacity(0.07)), lineWidth: 1.2)
                        t.fill(vaultStars, with: .color(ink.opacity(0.78)))
                        t.fill(vaultSparks, with: .color(ink.opacity(0.5)))
                        x += tile
                    }
                    y += tile
                }
            }
            .mask(
                LinearGradient(
                    stops: fade.map { Gradient.Stop(color: .black.opacity($0.1), location: $0.0) },
                    startPoint: .top,
                    endPoint: .bottom
                )
            )
            .allowsHitTesting(false)
            .accessibilityHidden(true)
        }
    }
}

/// A hairline across the measure.
struct Hairline: View {
    let color: Color

    var body: some View {
        Rectangle().fill(color).frame(height: 1).frame(maxWidth: .infinity).accessibilityHidden(true)
    }
}

/// A short vertical hairline between neighbouring items.
struct VRule: View {
    let color: Color
    var height: CGFloat = 14
    @Environment(\.metrics) private var m

    var body: some View {
        Rectangle().fill(color).frame(width: 1, height: m.px(height)).accessibilityHidden(true)
    }
}
