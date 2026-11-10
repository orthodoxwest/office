import SwiftUI
import UIKit

/**
 * The wide home's niche and chapel light ("Home niche" in style.css): on a wide screen the
 * frontispiece is one lancet niche cut into the wall, its head genarch's landscape `Arch.niche` or
 * `Arch.upright`, its size one scale (`nicheZoom`), and the room lit toward it. By day its depth is
 * told by light on the reveal (`Niche`); by night it is the phone's panel and apse sky (`Panel`,
 * `Sky`) on a plain wall, so the phone and the wide home are one building.
 */
struct NicheTokens {
    let recess: Color
    let sheen: Color
    let warm: Color
    /// The ground the niche is cleared to; the Nave's is none.
    let clear: Color?
    let pool: Color
    let shade: Color
    let beam: Color
    /// The reveal by day (`--reveal-*`): shade on the left jamb, light on the right and the sill.
    var revealShade = Color(hex: 0xCBBCAA)
    var revealMid = Color(hex: 0xE6DCCD)
    var revealLit = Color(hex: 0xFFFDF6)
    var revealSill = Color(hex: 0xFBF6EA)
    var revealSillEdge = Color(hex: 0xD9CCBA)
    var revealSoffit = Color(rgb: 96, 66, 44, 0.3)
    var arris = Color(rgb: 255, 253, 246, 0.9)
    var arrisTurn = Color(rgb: 255, 253, 246, 0.2)
    var arrisShade = Color(rgb: 96, 66, 44, 0.34)
    var cast = Color(rgb: 84, 52, 30, 0.3)

    static func of(_ p: Palette) -> NicheTokens {
        if !p.dark {
            return NicheTokens(
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

/**
 * One layout's figures, in rem (16pt), as style.css's `.page-home main` gives them for it (each
 * rule marked `NicheFigures.<name>`; Android's TokensTest holds the stylesheet, Niche.kt and this
 * file to each other): the card's width before the zoom, the niche's height at 1 on its heaviest
 * day with the colophon's zoomed margin (on a short screen, down to the invitation), the air above
 * it, the least zoom and the share of the screen's width it may take.
 */
struct NicheFigures {
    let card: CGFloat
    let fit: CGFloat
    let air: CGFloat
    let least: CGFloat
    let share: CGFloat

    static let landscape = NicheFigures(card: 37.5, fit: 47.25, air: 1.6, least: 0.96, share: 0.62)
    static let landscapeList = NicheFigures(card: 37.5, fit: 61, air: 1.6, least: 0.96, share: 0.62)
    static let upright = NicheFigures(card: 34.5, fit: 48.75, air: 2.4, least: 0.9, share: 0.8)
    static let uprightList = NicheFigures(card: 34.5, fit: 62.5, air: 2.4, least: 0.9, share: 0.8)
    static let short = NicheFigures(card: 37.5, fit: 35.25, air: 0.5, least: 0.5, share: 0.62)

    /// What is not zoomed above the niche (the header) and below it (the colophon), in rem.
    static let above: CGFloat = 5.0625
    static let below: CGFloat = 6.375
    /// The list of seven on a screen on its side, from this high (an upright one's from 1000).
    static let listLandscape: CGFloat = 1146
    /// A short screen, to this high: the niche is fitted down to the invitation.
    static let shortHeight: CGFloat = 560
}

/**
 * The niche's layout at a size of screen (`width` by `height`, the height home has, in the web's
 * pixels), as style.css's "Home niche" sets it: a landscape or upright lancet, the glossed table or
 * the list of seven, and one scale for the whole niche. Android's `NicheLayout`.
 */
struct NicheLayout {
    /// An upright (or square) screen takes the taller head and a narrower lancet.
    let upright: Bool
    /// A short screen on its side (a phone turned) is fitted down to the invitation.
    let short: Bool
    /// Where the screen has the height, the hours stand one to a line.
    let list: Bool
    let figures: NicheFigures
    /// The one scale (`--niche-zoom`).
    let zoom: CGFloat
    /// Whether the heaviest day fits with the colophon (`--niche-fits`); where it does not, the
    /// colophon waits under the fold rather than being cut through by it.
    let fits: Bool

    init(width: CGFloat, height: CGFloat) {
        upright = height >= width
        short = !upright && height <= NicheFigures.shortHeight
        list = upright ? height >= 1000 : height >= NicheFigures.listLandscape
        figures = upright ? (list ? .uprightList : .upright) : list ? .landscapeList : short ? .short : .landscape
        let below = short ? 0 : NicheFigures.below
        let raw = (height - 16 * (NicheFigures.above + below)) / (16 * (figures.fit + figures.air))
        zoom = nicheZoom(raw: raw, width: width, figures: figures)
        fits = !short && raw >= figures.least
    }

    var arch: Arch { upright ? .upright : .niche }
    /// The card's width before the zoom (`--card-w`).
    var card: CGFloat { figures.card * 16 }
    /// The air above the niche before the zoom (`--niche-air`).
    var air: CGFloat { figures.air * 16 }
    /// The list of seven's middle column (its label stands left of it), before the zoom.
    var listMiddle: CGFloat { upright ? 118.4 : 108 }
}

/// `clamp(least, min(raw, share * width / card), 1.5)`, `raw` the height's ratio: style.css's
/// `--niche-zoom`, and Niche.kt's `nicheZoom`.
func nicheZoom(raw: CGFloat, width: CGFloat, figures f: NicheFigures) -> CGFloat {
    max(f.least, min(raw, f.share * width / (16 * f.card), 1.5))
}

extension Metrics {
    /// These metrics at the niche's scale: every length and every size inside it, as the web's `zoom`.
    func zoomed(_ z: CGFloat) -> Metrics {
        var m = self
        m.layout *= z
        m.type *= z
        return m
    }
}

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

    /// The head's springing, where its curves meet the sides.
    var spring: CGFloat { origin.y + arch.rise * size.width }

    /// The wall outside the head offset by `d` (inward, so negative) and dropped `dy`, inside the
    /// head's own sides and above its springing (the web's `.home-arch-shade::before`).
    func head(_ d: CGFloat, dy: CGFloat) -> Path {
        outside(d, dy: dy).intersection(Path(CGRect(x: origin.x - d, y: 0, width: size.width + 2 * d, height: spring + dy)), eoFill: true)
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
 * The niche by day behind the frontispiece, under the pointed head `arch`, in the order the web's
 * courses stack, outermost first: the arris where the wall turns into the recess, catching the
 * light on the right and lost in shade on the left; the reveal (`--reveal`, 1.2rem), the left jamb
 * in shade and the right lit, the soffit turning from the light toward the point, the sill between
 * the jambs mitred at its corners and facing the light; the day's colour as one fillet at the back
 * wall's edge (`fillet`); then the back wall, the phone's plaster under a warm glow and sheen, with
 * the shade the head and left jamb cast on it. Each is the arch offset by its own distance. By
 * night the niche is the phone's panel (`Panel`) and its sky.
 */
struct Niche: View {
    let arch: Arch
    let t: NicheTokens
    let fillet: Color
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, full in
            let rem = m.px(16)
            let card = Card(arch: arch, full: full)
            let o = card.origin, size = card.size
            let w = size.width, h = size.height
            let r = 1.2 * rem
            func across(_ stops: [Gradient.Stop]) -> GraphicsContext.Shading {
                .linearGradient(Gradient(stops: stops), startPoint: CGPoint(x: o.x - r, y: 0), endPoint: CGPoint(x: o.x + w + r, y: 0))
            }
            // linear-gradient(100deg, ...) across the courses: near enough level, left to right.
            ctx.fill(card.shape(r + m.px(1.5)), with: across([.init(color: t.arrisShade, location: 0), .init(color: t.arrisShade, location: 0.36), .init(color: t.arrisTurn, location: 0.52), .init(color: t.arris, location: 0.7)]))
            let reveal = card.shape(r)
            ctx.fill(reveal, with: across([.init(color: t.revealShade, location: 0), .init(color: t.revealShade, location: 0.34), .init(color: t.revealMid, location: 0.56), .init(color: t.revealLit, location: 0.8)]))
            ctx.drawLayer { inside in
                inside.clip(to: reveal)
                inside.fill(
                    Path(CGRect(x: o.x - r, y: o.y - r, width: w + 2 * r, height: 0.5 * w + r)),
                    with: .linearGradient(Gradient(colors: [t.revealSoffit, t.revealSoffit.opacity(0)]), startPoint: CGPoint(x: 0, y: o.y - r), endPoint: CGPoint(x: 0, y: o.y + 0.5 * w))
                )
                // The sill, mitred: from the jambs' outer corners in to the back wall's.
                var sill = Path()
                sill.move(to: CGPoint(x: o.x - r, y: o.y + h + r))
                sill.addLine(to: CGPoint(x: o.x + w + r, y: o.y + h + r))
                sill.addLine(to: CGPoint(x: o.x + w, y: o.y + h))
                sill.addLine(to: CGPoint(x: o.x, y: o.y + h))
                sill.closeSubpath()
                inside.fill(sill, with: .linearGradient(Gradient(stops: [.init(color: t.revealSillEdge, location: 0), .init(color: t.revealSill, location: 0.35), .init(color: t.revealSill, location: 1)]), startPoint: CGPoint(x: 0, y: o.y + h), endPoint: CGPoint(x: 0, y: o.y + h + r)))
            }
            ctx.fill(card.shape(m.px(1.5)), with: .color(fillet))
            let face = card.shape(0)
            ctx.fill(face, with: .color(p.surface))
            ctx.drawLayer { inside in
                inside.clip(to: face)
                // The plaster, cover-fitted, then the warm pool under the head and the sheen falling
                // from it, laid from 2rem above the card as the web's courses are.
                if let plaster = UIImage(named: p.panel) {
                    let image = inside.resolve(Image(uiImage: plaster))
                    let scale = max(w / image.size.width, h / image.size.height)
                    let fitted = CGSize(width: image.size.width * scale, height: image.size.height * scale)
                    inside.draw(image, in: CGRect(origin: CGPoint(x: o.x + (w - fitted.width) / 2, y: o.y), size: fitted))
                }
                let top = o.y - 2 * rem
                let height = h + 2 * rem
                let rx = w * 0.7, ry = height * 0.5
                var pool = inside
                pool.translateBy(x: o.x + w / 2, y: top)
                pool.scaleBy(x: 1, y: ry / rx)
                pool.fill(
                    Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)),
                    with: .radialGradient(Gradient(stops: [.init(color: t.warm, location: 0), .init(color: t.warm.opacity(0), location: 0.72)]), center: .zero, startRadius: 0, endRadius: rx)
                )
                inside.fill(
                    Path(CGRect(x: o.x, y: top, width: w, height: height)),
                    with: .linearGradient(Gradient(stops: [.init(color: t.sheen, location: 0), .init(color: t.sheen.opacity(0), location: 0.3)]), startPoint: CGPoint(x: 0, y: top), endPoint: CGPoint(x: 0, y: top + height))
                )
                // drop-shadow(0.5rem 0.7rem 0.8rem cast) drop-shadow(0.1rem 0.15rem 0.12rem cast):
                // the head and the left jamb throw their shade on the back wall.
                var deep = inside
                deep.translateBy(x: 0.5 * rem, y: 0)
                blurred(deep, card.outside(0, dy: 0.7 * rem), t.cast, 0.8 * rem, eoFill: true)
                var near = inside
                near.translateBy(x: 0.1 * rem, y: 0)
                blurred(near, card.outside(0, dy: 0.15 * rem), t.cast, 0.12 * rem, eoFill: true)
            }
        }
        .padding(-reach)
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/// Where the first-class fillet's leaf catches the light most (`--gilt-glint`): the reveal's lit plaster.
let giltGlint = Color(hex: 0xFFFDF6)

/**
 * A first-class day's fillet of matte leaf round the niche's head by day (`.home-summary::before`),
 * 3pt over the back wall's edge, open at the foot and down to this view's foot (the frieze), as the
 * parish's chancel arch is gilt; the jambs and the sill keep the day's fillet. The view is the
 * card's width, its top the card's.
 */
struct GiltHead: View {
    let arch: Arch
    @Environment(\.ornament) private var o
    @Environment(\.metrics) private var m

    var body: some View {
        Canvas { ctx, size in
            let gilt = Gradient(stops: [
                .init(color: mix(o.lo, o.hi, 0.6), location: 0), .init(color: mix(o.lo, o.hi, 0.6), location: 0.3),
                .init(color: o.hi, location: 0.62), .init(color: mix(o.hi, giltGlint, 0.7), location: 0.82), .init(color: o.hi, location: 1),
            ])
            let path = archPath(arch, width: size.width, outset: 0, foot: size.height, open: true)
            ctx.stroke(path, with: .linearGradient(gilt, startPoint: .zero, endPoint: CGPoint(x: size.width, y: 0)), style: StrokeStyle(lineWidth: m.px(3), lineJoin: .miter, miterLimit: 2))
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/**
 * A phone's frontispiece: a painted panel under the pointed head `arch`, in the niche's family
 * (`.home-hero`), its courses outermost first: a soft halo of the wall's ground that keeps the
 * field off it, the day's colour as a ring at its edge (`ring`), the frame, then the panel with
 * the shade its head casts and, by day, the light caught under the head's edge. By night (the
 * web's Apse phone panel) the frame is a course of slate lit from above, the panel stands a step
 * above the sky with moonlight under the cross, its shade is soft and wide, and the card casts a
 * shadow on the sky instead of the halo.
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
            if p.dark {
                // drop-shadow(0 0 0.35rem bg) drop-shadow(0 0.5rem 1rem black 70%): a clearing
                // of the sky's own colour that keeps the stars off the card's edge, over the
                // shadow the card casts on the sky. The second shadow is cast by the card and its
                // clearing, so it reaches as far as the two in quadrature.
                let spread: CGFloat = (0.35 * 0.35 + 1).squareRoot()
                blurred(ctx, card.shape(1.5, dy: 0.5 * rem), .black.opacity(0.7), spread * rem)
                blurred(ctx, outer, p.bg, 0.35 * rem)
            } else {
                // drop-shadow(0 0 0.5rem bg) drop-shadow(0 0 0.75rem bg): the second blurs the
                // first again, so it reaches as far as the two in quadrature.
                let spread: CGFloat = (0.5 * 0.5 + 0.75 * 0.75).squareRoot()
                blurred(ctx, outer, p.bg, spread * rem)
                blurred(ctx, outer, p.bg, 0.5 * rem)
            }
            ctx.fill(outer, with: .color(ring))
            ctx.fill(card.shape(0), with: .color(p.surface))
            if p.dark {
                // --arch-stone: the slate course, lit from above, from the courses' top (2rem
                // above the point) to the springing, and level below it.
                let top = card.origin.y - 2 * rem
                let stone = Gradient(colors: [Color(rgb: 58, 72, 94, 1), Color(rgb: 42, 54, 72, 1)])
                ctx.fill(card.shape(0), with: .linearGradient(stone, startPoint: CGPoint(x: 0, y: top), endPoint: CGPoint(x: 0, y: card.spring)))
            } else {
                ctx.fill(card.shape(0), with: .color(frame))
            }
            let inset: CGFloat = p.dark ? -5 : -1
            let face = card.shape(inset)
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
                if p.dark {
                    // --panel-glow: moonlight under the cross, radii the panel's width and 1.06
                    // of it, centred 0.39 of it below the courses' top (2rem above the point).
                    // The web lays it under the plaster and veils both by half; laid over the
                    // baked plaster at half its strength it lifts the panel as much and keeps
                    // the trowel marks.
                    let w = card.size.width
                    let center = CGPoint(x: card.origin.x + w / 2, y: card.origin.y + 0.39 * w - 2 * rem)
                    var glow = inside
                    glow.translateBy(x: center.x, y: center.y)
                    glow.scaleBy(x: 1, y: 1.06)
                    let moon = Gradient(colors: [Color(rgb: 150, 178, 220, 0.1), Color(rgb: 150, 178, 220, 0)])
                    glow.fill(Path(ellipseIn: CGRect(x: -w, y: -w, width: 2 * w, height: 2 * w)), with: .radialGradient(moon, center: .zero, startRadius: 0, endRadius: w))
                    // drop-shadow(0 1px 0 moonlit 14%) drop-shadow(0 0.25rem 1rem black 30%):
                    // the head's shade soft and wide, and over it the light under the head's edge.
                    // The course casts them from round the head only: the web paints it down the
                    // sides and foot on the panel, under no shade.
                    blurred(inside, card.head(inset, dy: 0.25 * rem), .black.opacity(0.3), rem)
                    inside.fill(card.head(inset, dy: 1), with: .color(Color(rgb: 200, 214, 234, 0.14)))
                } else {
                    // drop-shadow(0 0.3rem 0.4rem recess): the shade the head casts on the panel.
                    blurred(inside, card.outside(-1, dy: 0.3 * rem), NicheTokens.of(p).recess, 0.4 * rem, eoFill: true)
                    // drop-shadow(0 1px 0 white 45%): the light caught under the head's edge.
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
    /// Home's lining lies unevenly on the limewash, through the pigment (`pigment.png`, as the web's mask).
    var pigmented = false
    @Environment(\.palette) private var p

    var body: some View {
        Canvas { ctx, size in
            // A miter limit of 2 keeps the point sharp, and bevels the hair's-breadth step that
            // may join the upper arcs there rather than drawing it out into a spike.
            func line(_ d: CGFloat, _ color: Color, width: CGFloat) {
                let path = archPath(arch, width: size.width, outset: d - width / 2, foot: size.height, open: true)
                let style = StrokeStyle(lineWidth: width, lineJoin: .miter, miterLimit: 2)
                if pigmented {
                    throughPigment(ctx, path, ink: color, tile: 128, stroke: style)
                } else {
                    ctx.stroke(path, with: .color(color), style: style)
                }
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
