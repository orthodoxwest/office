import SwiftUI
import UIKit

/**
 * The desktop home's niche and chapel light ("Home niche" in style.css): on a wide screen the
 * frontispiece is set into the wall under a pointed head, with a stone moulding, the day's colour
 * as a trim, and the room lit toward it. Phones set it in a painted panel (`Panel`).
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

/// How far the niche's drawing runs beyond the card: the clearing, the moulding and their shadows.
private let reach: CGFloat = 96

/// The canvas's card, `reach` inside it: its pointed head offset by `d`, dropped `dy`.
private struct Card {
    let arch: Arch
    let full: CGSize

    var origin: CGPoint { CGPoint(x: reach, y: reach) }
    var size: CGSize { CGSize(width: full.width - 2 * reach, height: full.height - 2 * reach) }

    func shape(_ d: CGFloat, dy: CGFloat = 0) -> Path {
        archPath(arch, width: size.width, outset: d, foot: size.height + d, at: CGPoint(x: origin.x, y: origin.y + dy))
    }

    /// Everything on the canvas outside the head offset by `d` and dropped `dy`: the wall, drawn
    /// only for the shade its edge casts inside the shape.
    func outside(_ d: CGFloat, dy: CGFloat) -> Path {
        var wall = Path(CGRect(origin: .zero, size: full))
        wall.addPath(shape(d, dy: dy))
        return wall
    }
}

/// Draws `path` blurred, for a CSS blur of `blur`: its Gaussian's deviation is half the radius.
private func blurred(_ ctx: GraphicsContext, _ path: Path, _ color: Color, _ blur: CGFloat, eoFill: Bool = false) {
    ctx.drawLayer { layer in
        layer.addFilter(.blur(radius: blur / 2))
        layer.fill(path, with: .color(color), style: FillStyle(eoFill: eoFill))
    }
}

/**
 * The niche behind the frontispiece, under the pointed head `arch`, in the order the web's
 * courses stack, outermost first: the room's shade under the niche and a field of the Apse's
 * ground round both, the clearing, the moulding's edge and stone, the day's colour as a trim;
 * then the frame and the lit recess, with the shade its head casts. Each is the arch offset by
 * its own distance.
 */
struct Niche: View {
    let arch: Arch
    let t: NicheTokens
    let day: Color
    let frame: Color
    @Environment(\.palette) private var p

    var body: some View {
        Canvas { ctx, full in
            let rem: CGFloat = 16
            let card = Card(arch: arch, full: full)
            let o = card.origin, size = card.size
            // The moulding's silhouette, the source of its halo: the clearing, or with none the
            // stone's edge.
            let outer = t.clear != nil ? 1.25 * rem + 1 : 0.75 * rem + 1
            if let clear = t.clear {
                // drop-shadow(0 0 0.9rem clear) round the moulding and its shadow (as deep as the
                // night's shade), as the second filter.
                blurred(ctx, card.shape(outer, dy: 1.25 * rem), clear.opacity(0.62), 1.54 * rem)
                blurred(ctx, card.shape(outer), clear, 0.9 * rem)
            }
            // drop-shadow(0 1.25rem 1.25rem chapel-shade): the niche's shade on the wall below it.
            blurred(ctx, card.shape(outer, dy: 1.25 * rem), t.shade, 1.25 * rem)
            if let clear = t.clear { ctx.fill(card.shape(1.25 * rem + 1), with: .color(clear)) }
            ctx.fill(card.shape(0.75 * rem + 1), with: .color(t.edge))
            ctx.fill(card.shape(0.75 * rem), with: .color(t.stone))
            // The day's colour is a hint at the niche's edge, not a second frame.
            ctx.fill(card.shape(1.5), with: .color(day))
            // The frame, the card's edge drawn in its rule, and the recess 2pt inside it.
            ctx.fill(card.shape(0), with: .color(p.surface))
            ctx.fill(card.shape(0), with: .color(frame))
            let recess = card.shape(-2)
            ctx.fill(recess, with: .color(p.surface))
            ctx.drawLayer { inside in
                inside.clip(to: recess)
                // A warm pool under the head, and a sheen falling from it, laid from 2rem above
                // the card as the web's courses are.
                let top = o.y - 2 * rem
                let height = size.height + 2 * rem
                let rx = size.width * 0.7, ry = height * 0.5
                var pool = inside
                pool.translateBy(x: o.x + size.width / 2, y: top)
                pool.scaleBy(x: 1, y: ry / rx)
                pool.fill(
                    Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)),
                    with: .radialGradient(Gradient(stops: [.init(color: t.warm, location: 0), .init(color: t.warm.opacity(0), location: 0.72)]), center: .zero, startRadius: 0, endRadius: rx)
                )
                inside.fill(
                    Path(CGRect(x: o.x, y: top, width: size.width, height: height)),
                    with: .linearGradient(Gradient(stops: [.init(color: t.sheen, location: 0), .init(color: t.sheen.opacity(0), location: 0.3)]), startPoint: CGPoint(x: 0, y: top), endPoint: CGPoint(x: 0, y: top + height))
                )
                // drop-shadow(0 0.55rem 0.9rem recess): the wall outside the arch casts its shade
                // in, and the recess lies in shadow under the head.
                blurred(inside, card.outside(-2, dy: 0.55 * rem), t.recess, 0.9 * rem, eoFill: true)
            }
        }
        .padding(-reach)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * A phone's frontispiece: a painted panel under the pointed head `arch`, in the niche's family
 * (`.home-hero`), its courses outermost first: a soft halo of the wall's ground that keeps the
 * field off it, the day's colour as a ring at its edge (`ring`), the frame, then the panel with
 * the shade its head casts and, by day, the light caught under the head's edge.
 */
struct Panel: View {
    let arch: Arch
    let ring: Color
    let frame: Color
    @Environment(\.palette) private var p

    var body: some View {
        Canvas { ctx, full in
            let rem: CGFloat = 16
            let card = Card(arch: arch, full: full)
            let outer = card.shape(1.5)
            // drop-shadow(0 0 0.5rem bg) drop-shadow(0 0 0.75rem bg), by night 0.35rem and
            // 0.6rem: the second blurs the first again, so it reaches as far as the two in
            // quadrature.
            let (near, far): (CGFloat, CGFloat) = p.dark ? (0.35, 0.6) : (0.5, 0.75)
            blurred(ctx, outer, p.bg, (near * near + far * far).squareRoot() * rem)
            blurred(ctx, outer, p.bg, near * rem)
            ctx.fill(outer, with: .color(ring))
            ctx.fill(card.shape(0), with: .color(p.surface))
            ctx.fill(card.shape(0), with: .color(frame))
            let face = card.shape(-1)
            ctx.fill(face, with: .color(p.surface))
            ctx.drawLayer { inside in
                inside.clip(to: face)
                // The plaster under the day's words, cover-fitted to the panel as the web's `.home-arch-fill`.
                if let plaster = UIImage(named: p.panel) {
                    let image = inside.resolve(Image(uiImage: plaster))
                    let scale = max(card.size.width / image.size.width, card.size.height / image.size.height)
                    let fitted = CGSize(width: image.size.width * scale, height: image.size.height * scale)
                    let at = CGPoint(x: card.origin.x + (card.size.width - fitted.width) / 2, y: card.origin.y + (card.size.height - fitted.height) / 2)
                    inside.draw(image, in: CGRect(origin: at, size: fitted))
                }
                // drop-shadow(0 0.3rem 0.4rem recess): the shade the head casts on the panel.
                blurred(inside, card.outside(-1, dy: 0.3 * rem), NicheTokens.of(p).recess, 0.4 * rem, eoFill: true)
                if !p.dark {
                    // drop-shadow(0 1px 0 white 45%), by day: the light caught under the head's edge.
                    inside.fill(card.outside(-1, dy: 1), with: .color(.white.opacity(0.45)), style: FillStyle(eoFill: true))
                }
            }
        }
        .padding(-reach)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * The lining painted round the head on a panel's or niche's back wall (`.home-lining`): the arch
 * again, `inset` inside the card's edge, as a 2pt band of the lining and the day's colour
 * (`hairline`) as a line 8pt inside the band's outer edge; round the head only and open below,
 * down to the foot of this view (the inscription band), so it reads as paint on the wall rather
 * than another edge of the arch. The view is the card's width, its top the card's.
 */
struct Lining: View {
    let arch: Arch
    let inset: CGFloat
    let hairline: Color
    @Environment(\.palette) private var p

    var body: some View {
        Canvas { ctx, size in
            // A miter limit of 2 keeps the point sharp, and bevels the hair's-breadth step that
            // may join the upper arcs there rather than drawing it out into a spike.
            func line(_ d: CGFloat, _ color: Color, width: CGFloat) {
                let path = archPath(arch, width: size.width, outset: d - width / 2, foot: size.height, open: true)
                ctx.stroke(path, with: .color(color), style: StrokeStyle(lineWidth: width, lineJoin: .miter, miterLimit: 2))
            }
            line(-inset, p.lining, width: 2)
            line(-inset - 8, hairline, width: 1)
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * Where the pool of light falls on the niche: two thirds of the way down, where the web's
 * viewport-centred pool meets its vertically centred niche at desktop sizes.
 */
let poolDepth: CGFloat = 0.66

/**
 * The chapel's light over the wall, fixed to the screen: a warm pool on the niche (`niche`, its
 * frame on the screen; the web's 50% 54% until it is placed), the room's edges in shade, and a
 * shaft from a high window spending itself before the floor.
 */
struct ChapelLight: View {
    let t: NicheTokens
    var niche: CGRect?

    var body: some View {
        GeometryReader { geo in
            let origin = geo.frame(in: .global).origin
            light(pool: niche.map { CGPoint(x: $0.midX - origin.x, y: $0.minY + $0.height * poolDepth - origin.y) })
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }

    private func light(pool: CGPoint?) -> some View {
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
            let at = pool ?? CGPoint(x: size.width * 0.5, y: size.height * 0.54)
            ellipse(at.x, at.y, size.width * 0.4, size.height * 0.46, [.init(color: t.pool, location: 0), .init(color: t.pool.opacity(0), location: 0.7)])
            ellipse(size.width * 0.5, size.height * 0.52, size.width * 0.74, size.height * 0.8, [.init(color: t.shade.opacity(0), location: 0.34), .init(color: t.shade, location: 1)])
        }
    }
}
