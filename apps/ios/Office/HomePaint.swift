import SwiftUI
import UIKit

/**
 * Home's painted page, the web's "Home: the painted page" (style.css) and Android's HomePaint.kt:
 * the colours the page's furniture takes beyond the palette.
 *
 * By day the invitation lies on a thin wash of yellow ochre and the hour that is now under a pool
 * of the same light; by night both are moonlit blue. The apse's sky, set by night in the head of a
 * phone's arch, is the parish vault's slate blue (`sky*`), ringed by the slate course's outer ring
 * in shade (`skyRim`); a white day's line on it is lime white, so the leaf fillet stays the one
 * gold line; the day's words on it take a lifted muted ink and rubric, and the invitation a lifted
 * lining. The leaf is the season's gilding matted toward the soft, rosy leaf of the parish's
 * photographs (`leafMatte*`).
 */
struct HomeInk {
    let tabletWash: Color
    let nowPool: Color
    let nowGloss: Color
    /// Today's band in the ordo: the wash thinner, a band being ten times the tablet's area (`--today-wash`).
    let todayWash: Color
    var skyDeep = Color(hex: 0x172231)
    var sky = Color(hex: 0x233440)
    var skyLow = Color(hex: 0x2F474D)
    var skyRim = Color(hex: 0x0C131D)
    var limeWhite = Color(hex: 0xE9E3D0)
    var skyMuted = Color(hex: 0xB9C3C8)
    var skyRubric = Color(hex: 0xF0A39C)
    var liningLifted = Color(hex: 0xE0A686)

    static func of(_ p: Palette) -> HomeInk {
        p.dark
            ? HomeInk(tabletWash: Color(rgb: 132, 160, 204, 0.12), nowPool: Color(rgb: 140, 170, 216, 0.24), nowGloss: Color(hex: 0xC3CCD8), todayWash: Color(rgb: 132, 160, 204, 0.11))
            : HomeInk(tabletWash: ochre.opacity(0.19), nowPool: ochre.opacity(0.34), nowGloss: mix(p.text, p.muted, 0.55), todayWash: ochre.opacity(0.12))
    }
}

/// `--ochre`, whose thin washes are Nave's tablet and pool.
let ochre = Color(hex: 0xC9973F)
let leafMatteHi = Color(hex: 0xC9A182)
let leafMatte = Color(hex: 0xA98765)
let leafMatteLo = Color(hex: 0x7D6248)

/// The current hour's name on its pool: the tituli's ink, lifted by night in the quiet seasons and Passiontide (`--titulus-now`).
func titulusNow(_ p: Palette, season: String) -> Color {
    switch (p.dark, season) {
    case (true, "quiet"): return Color(hex: 0xCFC39F)
    case (true, "passiontide"): return Color(hex: 0xCDC4DC)
    default: return p.titulus
    }
}

/// How much light the leaf catches: the quiet seasons leave it unlit, Passiontide veils it.
func leafCatch(_ season: String) -> CGFloat {
    switch season {
    case "quiet": return 0.6
    case "passiontide": return 0.7
    default: return 1
    }
}

/// The apse's leaf: the season's gilding pair matted toward the parish's own (`--sky-leaf*`).
struct Leaf {
    let hi: Color
    let mid: Color
    let lo: Color

    init(_ o: Ornament) {
        hi = mix(o.hi, leafMatteHi, 0.74)
        mid = mix(o.flat, leafMatte, 0.7)
        lo = mix(o.lo, leafMatteLo, 0.62)
    }
}

/**
 * A ruled line, as the web's `rule()` in app.js and Android's `ruledLine`: one stroke of a pen
 * from (x1, y1) to (x2, y2), horizontal or vertical, `w` wide, its weight breathing with the hand
 * (seeded, so a line is the same every time) and lifting to a point over the last 4.5pt at each
 * end. The same seeds as the web's, so the fronts rule alike.
 */
func ruledLine(_ x1: CGFloat, _ y1: CGFloat, _ x2: CGFloat, _ y2: CGFloat, _ w: CGFloat, seed: UInt32) -> Path {
    var s = UInt64(seed)
    func next() -> CGFloat {
        s = (s &* 1_664_525 &+ 1_013_904_223) & 0xFFFF_FFFF
        return CGFloat(Double(s) / 4_294_967_296)
    }
    let horizontal = abs(x2 - x1) >= abs(y2 - y1)
    let length = horizontal ? x2 - x1 : y2 - y1
    let n = max(6, Int((length / 14).rounded()))
    let phase1 = next() * 6.28
    let phase2 = next() * 6.28
    let drift = (next() - 0.5) * 0.5
    var above: [(CGFloat, CGFloat)] = []
    var below: [(CGFloat, CGFloat)] = []
    for i in 0...n {
        let t = CGFloat(i) / CGFloat(n)
        let d = t * length
        let end = min(1, min(d, length - d) / 4.5)
        let weight = w * (0.86 + 0.16 * sin(phase1 + t * 5.1) + 0.08 * sin(phase2 + t * 17)) * (0.35 + 0.65 * end)
        let c = drift * (t - 0.5) + 0.12 * sin(phase2 + t * 3.3)
        above.append((d, c - weight / 2))
        below.append((d, c + weight / 2))
    }
    var path = Path()
    for (i, (d, o)) in (above + below.reversed()).enumerated() {
        let p = horizontal ? CGPoint(x: x1 + d, y: y1 + o) : CGPoint(x: x1 + o, y: y1 + d)
        if i == 0 { path.move(to: p) } else { path.addLine(to: p) }
    }
    path.closeSubpath()
    return path
}

/// The invitation's double frame (the web's `.pray-now` rules): the lining, then the frontal inside it, short of the corners.
func invitationRules(_ ctx: GraphicsContext, _ size: CGSize, lining: Color, frontal: Color) {
    let w = size.width
    let h = size.height
    let g: CGFloat = 3.5
    ctx.fill(ruledLine(0, 0.5, w, 0.5, 1.25, seed: 21), with: .color(lining))
    ctx.fill(ruledLine(0, h - 0.5, w, h - 0.5, 1.25, seed: 22), with: .color(lining))
    ctx.fill(ruledLine(0.5, 0, 0.5, h, 1.25, seed: 23), with: .color(lining))
    ctx.fill(ruledLine(w - 0.5, 0, w - 0.5, h, 1.25, seed: 24), with: .color(lining))
    let day = frontal.opacity(0.5)
    ctx.fill(ruledLine(g + 1.5, g, w - g - 1.5, g, 0.7, seed: 25), with: .color(day))
    ctx.fill(ruledLine(g + 1.5, h - g, w - g - 1.5, h - g, 0.7, seed: 26), with: .color(day))
    ctx.fill(ruledLine(g, g + 1.5, g, h - g - 1.5, 0.7, seed: 27), with: .color(day))
    ctx.fill(ruledLine(w - g, g + 1.5, w - g, h - g - 1.5, 0.7, seed: 28), with: .color(day))
}

/**
 * Fills `path` in `ink` through the pigment mask (`pigment.png`, tools/genpaint.py), its 256px
 * tile drawn `tile` points across and repeated, so the colour stays the token and only the
 * pigment's unevenness comes through. Without the image, flat.
 */
func throughPigment(_ ctx: GraphicsContext, _ path: Path, ink: Color, tile: CGFloat, style: FillStyle = FillStyle(), stroke: StrokeStyle? = nil) {
    ctx.drawLayer { layer in
        if let stroke { layer.stroke(path, with: .color(ink), style: stroke) } else { layer.fill(path, with: .color(ink), style: style) }
        guard let image = UIImage(named: "pigment.png") else { return }
        layer.blendMode = .destinationIn
        let bounds = path.boundingRect.insetBy(dx: -4, dy: -4)
        layer.fill(Path(bounds), with: .tiledImage(Image(uiImage: image), scale: tile / image.size.width))
    }
}

/// Home's consecration cross, as Android's `ConsecrationCross` with a brush: painted through the pigment, or filled with `shading` (gilt or leaf).
struct Crown: View {
    let size: CGFloat
    var ink: Color
    var shading: GraphicsContext.Shading?
    var alpha: CGFloat = 1

    var body: some View {
        Canvas { ctx, s in
            ctx.opacity = alpha
            let (box, shapes) = Mark.consecration.drawing
            for shape in shapes {
                let path = shape.path.fitted(box, box, in: CGRect(origin: .zero, size: s))
                let style = FillStyle(eoFill: shape.evenOdd)
                if let shading {
                    ctx.fill(path, with: shading, style: style)
                } else {
                    throughPigment(ctx, path, ink: ink, tile: 96 * 40 / s.width, style: style)
                }
            }
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }
}

/// One of the sky's gold-leaf stars: its centre in card widths, its size and outline in points.
struct SkyStar {
    let x: CGFloat
    let y: CGFloat
    let r: CGFloat
    let leaf: CGFloat
    let outline: [CGFloat]

    static func parse(_ field: String) -> [SkyStar] {
        field.split(separator: ";").map { star in
            let v = star.split(separator: " ").map { CGFloat(Double($0) ?? 0) }
            return SkyStar(x: v[0], y: v[1], r: v[2], leaf: v[3], outline: Array(v.dropFirst(4)))
        }
    }

    /// The ending tile's edge, points: never scaled (`ornaments/sky-tile.svg`).
    static let tileSize: CGFloat = 384

    /// The hour's ending by night (tools/genstars.py's tile): each star as a path in points from
    /// the tile's top left, with its leaf, and again across each edge it crosses, so the tile
    /// repeats without a seam as the web's mask does.
    static let tile: [(path: Path, leaf: Double)] = parse(StarFields.tile).flatMap { s -> [(path: Path, leaf: Double)] in
        var out: [(path: Path, leaf: Double)] = []
        for ox in [-tileSize, 0, tileSize] {
            for oy in [-tileSize, 0, tileSize] {
                let cx = s.x * tileSize + ox
                let cy = s.y * tileSize + oy
                guard cx > -12, cx < tileSize + 12, cy > -12, cy < tileSize + 12 else { continue }
                var path = Path()
                for i in stride(from: 0, to: s.outline.count - 1, by: 2) {
                    let pt = CGPoint(x: cx + s.outline[i], y: cy + s.outline[i + 1])
                    if i == 0 { path.move(to: pt) } else { path.addLine(to: pt) }
                }
                path.closeSubpath()
                out.append((path: path, leaf: Double(s.leaf)))
            }
        }
        return out
    }

    private static let phone = parse(StarFields.phone)
    private static let tall = parse(StarFields.tall)
    private static let taller = parse(StarFields.taller)
    private static let niche = parse(StarFields.niche)
    private static let upright = parse(StarFields.upright)

    /// The stars of a head's field (tools/genstars.py), or none for a head with no sky.
    static func of(_ a: Arch) -> [SkyStar] {
        switch a.rise {
        case Arch.phone.rise: return phone
        case Arch.tall.rise: return tall
        case Arch.taller.rise: return taller
        case Arch.niche.rise: return niche
        case Arch.upright.rise: return upright
        default: return []
        }
    }
}

/**
 * The apse's sky inside the head by night, a phone's or the wide niche's (the web's `.home-sky`, Android's `sky`): the
 * slate from deep to light, a lighter and greener glow toward the frieze, and the vault's
 * brushwork (`sky_wash.jpg`) laid over it in soft light, clipped to the head inside the slate
 * course and down to the frieze (the view's foot). Then its gold-leaf stars, leaving out every
 * one that would touch the painter's reserve (`reserve`, the day's words padded 14 by 6, and the
 * cross padded 11) or come within 9pt of the frieze; on a first-class feast a size larger and
 * fully lit. Its fillet and hairline (`skyLining`) stand where the lining does by day.
 */
struct Sky: View {
    let arch: Arch
    let ink: HomeInk
    let leaf: Leaf
    let lit: CGFloat
    let stars: Bool
    let first: Bool
    let hairline: Color?
    let reserve: [CGRect]

    var body: some View {
        Canvas { ctx, size in
            let w = size.width
            let band = size.height
            let head = archPath(arch, width: w, outset: -5, foot: band + 2)
            ctx.drawLayer { sky in
                sky.clip(to: head)
                let top: CGFloat = -32
                sky.fill(
                    Path(CGRect(x: 0, y: top, width: w, height: band - top + 2)),
                    with: .linearGradient(Gradient(stops: [.init(color: ink.skyDeep, location: 0.1), .init(color: ink.sky, location: 0.9)]), startPoint: CGPoint(x: 0, y: top), endPoint: CGPoint(x: 0, y: band))
                )
                // radial-gradient(ellipse 85% 60% at 50% 100%, sky-low, transparent)
                let rx = 0.85 * w
                let ry = 0.6 * (band - top)
                var glow = sky
                glow.translateBy(x: w / 2, y: band)
                glow.scaleBy(x: 1, y: ry / rx)
                glow.fill(Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)), with: .radialGradient(Gradient(colors: [ink.skyLow, ink.skyLow.opacity(0)]), center: .zero, startRadius: 0, endRadius: rx))
                // The brushwork: at least 23rem wide, run down from the top, in soft light.
                if let wash = UIImage(named: "sky_wash.jpg") {
                    var brush = sky
                    brush.blendMode = .softLight
                    let tileW = max(w, 23 * 16)
                    let tileH = tileW * wash.size.height / wash.size.width
                    let image = Image(uiImage: wash)
                    var y = top
                    while y < band {
                        brush.draw(image, in: CGRect(x: (w - tileW) / 2, y: y, width: tileW, height: tileH))
                        y += tileH
                    }
                }
                guard stars else { return }
                let scale: CGFloat = first ? 1.14 : 1
                let light = GraphicsContext.Shading.radialGradient(
                    Gradient(stops: [.init(color: leaf.hi, location: 0), .init(color: leaf.mid, location: 0.6), .init(color: leaf.lo, location: 1)]),
                    center: CGPoint(x: w / 2, y: band * 0.62), startRadius: 0, endRadius: w * 0.72
                )
                for s in SkyStar.of(arch) {
                    let cx = s.x * w
                    let cy = s.y * w
                    let r = s.r * scale
                    if cy + r > band - 9 { continue }
                    let box = CGRect(x: cx - r, y: cy - r, width: 2 * r, height: 2 * r)
                    if reserve.contains(where: { $0.intersects(box) }) { continue }
                    var path = Path()
                    for i in stride(from: 0, to: s.outline.count - 1, by: 2) {
                        let p = CGPoint(x: cx + s.outline[i] * scale, y: cy + s.outline[i + 1] * scale)
                        if i == 0 { path.move(to: p) } else { path.addLine(to: p) }
                    }
                    path.closeSubpath()
                    var star = sky
                    star.opacity = lit * (first ? min(1, s.leaf + 0.25) : s.leaf)
                    star.fill(path, with: light)
                }
            }
            if let hairline { skyLining(ctx, size, hairline: hairline) }
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }

    /// The sky's two lines, where the lining stands by day: one matte leaf fillet against the slate
    /// (1.5pt, 2pt on a first-class feast), then the day's colour as a hairline 6pt inside it; open at the foot.
    private func skyLining(_ ctx: GraphicsContext, _ size: CGSize, hairline: Color) {
        let t: CGFloat = first ? 2 : 1.5
        var fillet = ctx
        fillet.opacity = first ? 1 : 0.9 * lit
        fillet.stroke(
            archPath(arch, width: size.width, outset: -5 - t / 2, foot: size.height, open: true),
            with: .linearGradient(Gradient(stops: [.init(color: leaf.lo, location: 0), .init(color: leaf.hi, location: 0.38), .init(color: leaf.mid, location: 0.7), .init(color: leaf.lo, location: 1)]), startPoint: CGPoint(x: 0, y: -32), endPoint: CGPoint(x: 0, y: size.height)),
            style: StrokeStyle(lineWidth: t, lineJoin: .miter, miterLimit: 2)
        )
        ctx.stroke(archPath(arch, width: size.width, outset: -11.5, foot: size.height, open: true), with: .color(hairline.opacity(0.85)), style: StrokeStyle(lineWidth: 1, lineJoin: .miter, miterLimit: 2))
    }
}

/**
 * The painter's reserve round the day's words: each text's frame in the frontispiece's space
 * (`frontispiece`), by key. The sky's stars keep clear of them. iOS gives a text's frame, not its
 * lines' boxes, so a centred line narrower than its text's widest keeps a little more clear air
 * than on the web and Android.
 */
struct ReserveKey: PreferenceKey {
    static let defaultValue: [String: CGRect] = [:]
    static func reduce(value: inout [String: CGRect], nextValue: () -> [String: CGRect]) {
        value.merge(nextValue()) { $1 }
    }
}

extension View {
    /// Enters this text in the painter's reserve under `key`.
    func reserved(_ key: String) -> some View {
        background(GeometryReader { g in
            Color.clear.preference(key: ReserveKey.self, value: [key: g.frame(in: .named("frontispiece"))])
        })
    }
}

/// A gilt point, as the web's `.home-hour-link + .home-hour-link::before`: the ornament's gold at 70%, 3.5pt on its corner.
struct GiltPoint: View {
    @Environment(\.ornament) private var o

    var body: some View {
        Canvas { ctx, s in ctx.fill(lozenge(CGPoint(x: s.width / 2, y: s.height / 2), 2.5), with: .color(o.flat.opacity(0.7))) }
            .frame(width: 5, height: 5)
            .accessibilityHidden(true)
    }
}

/**
 * An action set as home's invitation, not a filled slab (the web's `.reminder-subscribe`): the
 * tablet's thin wash inside the lining's rule with a finer one 4pt inside it, its words in the
 * lining, lifted by night. Reminders' "Turn on reminders".
 */
struct TabletButton: View {
    let label: String
    let action: () -> Void
    @Environment(\.palette) private var p
    @Environment(\.metrics) private var m

    var body: some View {
        let home = HomeInk.of(p)
        Button(action: action) {
            Text(label).type(Scale.body.sized(17.92, line: 24).tracked(0.21))
                .foregroundStyle(p.dark ? home.liningLifted : p.lining)
                .frame(maxWidth: .infinity, minHeight: 46)
                .padding(.horizontal, m.px(17.6))
                .background {
                    if let tablet = UIImage(named: "tablet.png") {
                        Image(uiImage: tablet).renderingMode(.template).resizable().foregroundStyle(home.tabletWash).padding(1)
                    }
                }
                .overlay {
                    ZStack {
                        Rectangle().strokeBorder(p.lining.opacity(0.78), lineWidth: 1)
                        Rectangle().strokeBorder(p.lining.opacity(0.36), lineWidth: 1).padding(4)
                    }
                    .allowsHitTesting(false)
                }
        }
        .buttonStyle(Quiet())
    }
}
