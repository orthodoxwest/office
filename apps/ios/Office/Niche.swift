import SwiftUI

/**
 * The desktop home's niche and chapel light ("Home niche" in style.css): on a wide screen the
 * frontispiece is set into the wall under a low round head, with a stone moulding, the day's
 * colour as a trim, and the room lit toward it. Phones keep the plain card.
 */
struct NicheTokens {
    let stone: Color
    let edge: Color
    let recess: Color
    let sheen: Color
    let warm: Color
    /// The ground the niche is cleared to; the Nave's is none.
    let clear: Color?
    let pool: Color
    let shade: Color
    let beam: Color

    static func of(_ p: Palette) -> NicheTokens {
        if !p.dark {
            return NicheTokens(
                stone: Color(hex: 0xE5DAC9),
                edge: Color(rgb: 63, 55, 47, 0.3),
                recess: Color(rgb: 70, 40, 20, 0.2),
                sheen: Color(rgb: 255, 255, 255, 0.14),
                warm: Color(rgb: 232, 176, 84, 0.2),
                clear: nil,
                pool: Color(rgb: 240, 188, 96, 0.2),
                shade: Color(rgb: 88, 60, 38, 0.36),
                beam: Color(rgb: 255, 251, 238, 0.62)
            )
        }
        return NicheTokens(
            stone: Color(hex: 0x243040),
            edge: Color(rgb: 208, 176, 106, 0.3),
            recess: Color(rgb: 0, 0, 0, 0.42),
            sheen: .clear,
            warm: Color(rgb: 208, 176, 106, 0.09),
            clear: p.bg,
            pool: Color(rgb: 214, 168, 90, 0.08),
            shade: Color(rgb: 3, 7, 12, 0.62),
            beam: Color(rgb: 200, 214, 234, 0.08)
        )
    }
}

/// The niche's card width at a screen width: clamp(38rem, 10rem + 38vw, 48rem).
func nicheWidth(_ screen: CGFloat) -> CGFloat { min(max(160 + screen * 0.38, 608), 768) }

/// The round head's height: clamp(5rem, 2rem + 6vw, 8rem).
func nicheHead(_ screen: CGFloat) -> CGFloat { min(max(32 + screen * 0.06, 80), 128) }

/**
 * The niche's outline for a card of `size`, `outset` beyond it: a low elliptical head across the
 * whole width (border-radius: 50% 50% 0 0 / head head 0 0), square below.
 */
func nichePath(_ size: CGSize, head: CGFloat, outset d: CGFloat, at origin: CGPoint = .zero) -> Path {
    let ry = max(0, head + d)
    let rx = size.width / 2 + d
    var p = Path()
    let left = origin.x - d, right = origin.x + size.width + d, top = origin.y - d, bottom = origin.y + size.height + d
    p.move(to: CGPoint(x: left, y: bottom))
    p.addLine(to: CGPoint(x: left, y: top + ry))
    // The head: half an ellipse from the left side over to the right.
    p.addArc(center: .zero, radius: 1, startAngle: .degrees(180), endAngle: .degrees(360), clockwise: false,
             transform: CGAffineTransform(translationX: (left + right) / 2, y: top + ry).scaledBy(x: rx, y: max(ry, 0.001)))
    p.addLine(to: CGPoint(x: right, y: bottom))
    p.closeSubpath()
    return p
}

/// How far the niche's drawing runs beyond the card: the clearing, the moulding and their shadows.
private let reach: CGFloat = 96

/**
 * The niche behind the frontispiece, in the order the web's box-shadows stack, bottom first: a
 * clearing of the Apse's ground, the shadow under the head, the moulding's edge and stone, the
 * day's colour; then the lit recess, its shadow under the head, the frame and the tooling.
 */
struct Niche: View {
    let t: NicheTokens
    let day: Color
    let head: CGFloat
    let frame: Color
    let tooling: Color
    @Environment(\.palette) private var p

    var body: some View {
        Canvas { ctx, full in
            let rem: CGFloat = 16
            let o = CGPoint(x: reach, y: reach)
            let size = CGSize(width: full.width - 2 * reach, height: full.height - 2 * reach)
            func shape(_ outset: CGFloat, dy: CGFloat = 0) -> Path {
                nichePath(size, head: head, outset: outset, at: CGPoint(x: o.x, y: o.y + dy))
            }
            // CSS's blur radius is twice the Gaussian's deviation.
            func blurred(_ path: Path, _ color: Color, _ blur: CGFloat) {
                ctx.drawLayer { layer in
                    layer.addFilter(.blur(radius: blur / 2))
                    layer.fill(path, with: .color(color))
                }
            }
            if let clear = t.clear {
                blurred(shape(1.4 * rem), clear, 1.8 * rem)
                ctx.fill(shape(1.25 * rem), with: .color(clear))
            }
            // 0 1.5rem 3rem -0.75rem: the head casts its shade down the wall.
            blurred(shape(-0.75 * rem, dy: 1.5 * rem), t.shade, 3 * rem)
            ctx.fill(shape(0.75 * rem + 1), with: .color(t.edge))
            ctx.fill(shape(0.75 * rem), with: .color(t.stone))
            ctx.fill(shape(2), with: .color(day))
            let recess = shape(0)
            ctx.fill(recess, with: .color(p.surface))
            ctx.drawLayer { inside in
                inside.clip(to: recess)
                // A warm pool under the head, and a sheen falling from it.
                let rx = size.width * 0.7, ry = size.height * 0.5
                var pool = inside
                pool.translateBy(x: o.x + size.width / 2, y: o.y)
                pool.scaleBy(x: 1, y: ry / rx)
                pool.fill(
                    Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)),
                    with: .radialGradient(Gradient(stops: [.init(color: t.warm, location: 0), .init(color: t.warm.opacity(0), location: 0.72)]), center: .zero, startRadius: 0, endRadius: rx)
                )
                inside.fill(
                    Path(CGRect(x: o.x, y: o.y, width: size.width, height: size.height)),
                    with: .linearGradient(Gradient(stops: [.init(color: t.sheen, location: 0), .init(color: t.sheen.opacity(0), location: 0.3)]), startPoint: CGPoint(x: 0, y: o.y), endPoint: CGPoint(x: 0, y: o.y + size.height))
                )
                // inset 0 2.6rem 2.6rem -2rem: everything outside the shape, spread 2rem and
                // dropped 2.6rem, blurred, and seen through the shape: a recess in shadow under the head.
                var outside = Path(CGRect(origin: .zero, size: full))
                outside.addPath(shape(2 * rem, dy: 2.6 * rem))
                inside.drawLayer { shadow in
                    shadow.addFilter(.blur(radius: 1.3 * rem))
                    shadow.fill(outside, with: .color(t.recess), style: FillStyle(eoFill: true))
                }
            }
            ctx.stroke(shape(-1), with: .color(frame), lineWidth: 2)
            // The book-cover tooling, 0.35rem inside, its head following the arch.
            let inset = 0.35 * rem
            let tool = nichePath(CGSize(width: size.width - 2 * inset, height: size.height - 2 * inset), head: head - inset, outset: 0, at: CGPoint(x: o.x + inset, y: o.y + inset))
            ctx.stroke(tool, with: .color(tooling), lineWidth: 1)
        }
        .padding(-reach)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * The chapel's light over the wall, fixed to the screen: a warm pool where the niche stands, the
 * room's edges in shade, and a shaft from a high window spending itself before the floor.
 */
struct ChapelLight: View {
    let t: NicheTokens

    var body: some View {
        Canvas { ctx, size in
            // CSS's radial ellipse: a circle of radius rx, squeezed to ry about its centre, over the whole screen.
            func ellipse(_ cx: CGFloat, _ cy: CGFloat, _ rx: CGFloat, _ ry: CGFloat, _ stops: [Gradient.Stop]) {
                var e = ctx
                e.translateBy(x: cx, y: cy)
                e.scaleBy(x: 1, y: ry / rx)
                let k = rx / ry
                e.fill(
                    Path(CGRect(x: -cx, y: -cy * k, width: size.width, height: size.height * k)),
                    with: .radialGradient(Gradient(stops: stops), center: .zero, startRadius: 0, endRadius: rx)
                )
            }
            // The shaft: linear-gradient(100deg, transparent 43%, beam 49%, transparent 57%), masked toward the floor.
            ctx.drawLayer { shaft in
                let a = 100.0 * Double.pi / 180
                let dir = CGPoint(x: sin(a), y: -cos(a))
                let len = abs(size.width * dir.x) + abs(size.height * dir.y)
                let c = CGPoint(x: size.width / 2, y: size.height / 2)
                shaft.fill(
                    Path(CGRect(origin: .zero, size: size)),
                    with: .linearGradient(
                        Gradient(stops: [.init(color: t.beam.opacity(0), location: 0.43), .init(color: t.beam, location: 0.49), .init(color: t.beam.opacity(0), location: 0.57)]),
                        startPoint: CGPoint(x: c.x - dir.x * len / 2, y: c.y - dir.y * len / 2),
                        endPoint: CGPoint(x: c.x + dir.x * len / 2, y: c.y + dir.y * len / 2)
                    )
                )
                shaft.blendMode = .destinationIn
                shaft.fill(
                    Path(CGRect(origin: .zero, size: size)),
                    with: .linearGradient(
                        Gradient(stops: [.init(color: .black, location: 0), .init(color: .black.opacity(0.55), location: 0.38), .init(color: .black.opacity(0), location: 0.74)]),
                        startPoint: .zero,
                        endPoint: CGPoint(x: 0, y: size.height)
                    )
                )
            }
            ellipse(size.width * 0.5, size.height * 0.54, size.width * 0.4, size.height * 0.46, [.init(color: t.pool, location: 0), .init(color: t.pool.opacity(0), location: 0.7)])
            ellipse(size.width * 0.5, size.height * 0.52, size.width * 0.74, size.height * 0.8, [.init(color: t.shade.opacity(0), location: 0.34), .init(color: t.shade, location: 1)])
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}
