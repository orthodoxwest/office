import SwiftUI

/**
 * Home's crown: the figures of its play and the sums made from them (presentation::crown, which
 * the web's app.js and style.css mirror). A tap on the consecration cross at the head of home's
 * arch turns the look over, Nave to Apse as dusk and Apse to Nave as dawn: the cross answers, the
 * sky in the head acts the change out, and then the page takes the new light, spreading out of the
 * arch in the arch's own shape. `CrownPlay.swift` plays it; this file holds every figure the play
 * reads, so the Rust core is its one source.
 */
struct CrownFigures {
    /// A stretch of the play: when it begins, in ms from the tap, and how long it runs.
    struct Span {
        let at: Double
        let len: Double
        var end: Double { at + len }
    }

    /// When the look itself changes under the play (the turn).
    let turn: Double
    /// The front crossing the head of the arch, eased by `frontEase`.
    let front: Span
    /// The page beyond the arch taking the new look, its shape eased by `spreadEase`.
    let spread: Span
    /// The veil over the turning wall, from its first trace to its last.
    let veilFrom: Double
    let veilTo: Double
    /// The play's length; later only where the page's far corner is not yet covered.
    let end: Double
    /// The veil's colour and its strength at its height.
    let veilInk: Pigment
    let veilPeak: Double
    /// The share of the new look under a line at which its ink is handed over.
    let inkAt: Double
    /// How the page's own opacity rises through the spread.
    let glowEase: CubicEase
    let frontEase: CubicEase
    let spreadEase: CubicEase
    /// How much longer the spread runs on a screen wider than 700, ms.
    let spreadWide: Double
    /// The feather of the spread's edge, in card widths where the spread begins.
    let spreadFeather: Double
    /// The share of the spread by which its shape has reached every place, and its bend.
    let spreadShapePart: Double
    let spreadBend: Double
    /// The share of the spread over which the page's opacity rises.
    let spreadGlowPart: Double
    /// How far the veil's shape runs ahead of the page's, and by what share it is done.
    let veilLead: Double
    let veilPart: Double
    /// A line's ink changes hands in this long, ms.
    let inkHandover: Double
    /// The sky's pigments.
    let skyOchre: Pigment
    let skyRedOchre: Pigment
    let skyEmber: Pigment
    let skyLime: Pigment
    let skyRoseLilac: Pigment
    /// A second tap closer than this to the last is the same tap's flurry, ms.
    let tapApart: Double
    /// A touch elsewhere runs what is left out in this long, ms.
    let rush: Double
    /// How many times slower than its timetable the play is performed (the clock is divided by it).
    let pace: Double
    /// The frieze's own change: dark first at dusk, light last at dawn.
    let frieze: Span
    /// The veil is multiplied over the page: its strength rises to `veilPeak` by `veilTop` of its
    /// span (`veilIn`) and is gone by `veilGone` (`veilOut`); it thins to `veilFar` of itself at
    /// the screen's edge.
    let veilTop: Double
    let veilGone: Double
    let veilIn: CubicEase
    let veilOut: CubicEase
    let veilFar: Double
    /// Dawn's first warmth, the night's teal lifted, and the cross's ember gold.
    let skyPeach: Pigment
    let skyTeal: Pigment
    let crossEmber: Pigment
    /// Each star kindles in `starKindle`, to `starOver` of its size and back, and goes out in
    /// `starSnuff`.
    let starKindle: Double
    let starOver: Double
    let starSnuff: Double
    /// Dusk's evening star stands alone this long; then the brightest, one at a time, apart.
    let eveningStarAlone: Double
    let starsBright: Int
    let starsBrightApart: Double
    /// Dawn's field is out by this share of the way to the settle; none lingers longer than this
    /// after the plaster's front has reached it.
    let dawnFieldOut: Double
    let dawnStarLinger: Double
    /// The ending beat: the page still for `beatHold`, then one thing, in `beatLen`.
    let beatHold: Double
    let beatLen: Double
}

extension CrownFigures {
    /// The Rust core's plan (`crownPlan(toDark:)`, presentation::crown), for a dusk or a dawn.
    static func of(dusk: Bool) -> CrownFigures {
        let p = crownPlan(toDark: dusk)
        return CrownFigures(
            turn: Double(p.turn),
            front: Span(at: Double(p.front.at), len: Double(p.front.len)),
            spread: Span(at: Double(p.spread.at), len: Double(p.spread.len)),
            veilFrom: Double(p.veilFrom),
            veilTo: Double(p.veilTo),
            end: Double(p.end),
            veilInk: Pigment(p.veilInk),
            veilPeak: Double(p.veilPeak),
            inkAt: Double(p.inkAt),
            glowEase: CubicEase(p.glowEase),
            frontEase: CubicEase(p.frontEase),
            spreadEase: CubicEase(p.spreadEase),
            spreadWide: Double(p.spreadWide),
            spreadFeather: Double(p.spreadFeather),
            spreadShapePart: Double(p.spreadShapePart),
            spreadBend: Double(p.spreadBend),
            spreadGlowPart: Double(p.spreadGlowPart),
            veilLead: Double(p.veilLead),
            veilPart: Double(p.veilPart),
            inkHandover: Double(p.inkHandover),
            skyOchre: Pigment(p.skyOchre),
            skyRedOchre: Pigment(p.skyRedOchre),
            skyEmber: Pigment(p.skyEmber),
            skyLime: Pigment(p.skyLime),
            skyRoseLilac: Pigment(p.skyRoseLilac),
            tapApart: Double(p.tapApart),
            rush: Double(p.rush),
            pace: Double(p.pace),
            frieze: Span(at: Double(p.frieze.at), len: Double(p.frieze.len)),
            veilTop: Double(p.veilTop),
            veilGone: Double(p.veilGone),
            veilIn: CubicEase(p.veilIn),
            veilOut: CubicEase(p.veilOut),
            veilFar: Double(p.veilFar),
            skyPeach: Pigment(p.skyPeach),
            skyTeal: Pigment(p.skyTeal),
            crossEmber: Pigment(p.crossEmber),
            starKindle: Double(p.starKindle),
            starOver: Double(p.starOver),
            starSnuff: Double(p.starSnuff),
            eveningStarAlone: Double(p.eveningStarAlone),
            starsBright: Int(p.starsBright),
            starsBrightApart: Double(p.starsBrightApart),
            dawnFieldOut: Double(p.dawnFieldOut),
            dawnStarLinger: Double(p.dawnStarLinger),
            beatHold: Double(p.beatHold),
            beatLen: Double(p.beatLen)
        )
    }
}

extension Pigment {
    init(_ v: CrownRgb) { self.init(Double(v.r), Double(v.g), Double(v.b)) }
}

extension CubicEase {
    init(_ v: CrownEase) { self.init(Double(v.x1), Double(v.y1), Double(v.x2), Double(v.y2)) }
}

/// A pigment, as red, green and blue from 0 to 255.
struct Pigment: Equatable {
    let r: Double
    let g: Double
    let b: Double

    init(_ r: Double, _ g: Double, _ b: Double) {
        self.r = r
        self.g = g
        self.b = b
    }

    func color(_ alpha: Double = 1) -> Color { Color(rgb: r, g, b, alpha) }

    /// This pigment `share` of the way to `other`.
    func toward(_ other: Pigment, _ share: Double) -> Pigment {
        let k = min(max(share, 0), 1)
        return Pigment(r + (other.r - r) * k, g + (other.g - g) * k, b + (other.b - b) * k)
    }
}

/// A cubic-bezier ease, as CSS writes it.
struct CubicEase: Equatable {
    let x1: Double
    let y1: Double
    let x2: Double
    let y2: Double

    init(_ x1: Double, _ y1: Double, _ x2: Double, _ y2: Double) {
        self.x1 = x1
        self.y1 = y1
        self.x2 = x2
        self.y2 = y2
    }

    static let linear = CubicEase(0, 0, 1, 1)
    /// CSS's `ease-out`.
    static let easeOut = CubicEase(0, 0, 0.58, 1)

    private static func axis(_ a: Double, _ b: Double, _ t: Double) -> Double {
        3 * a * t * (1 - t) * (1 - t) + 3 * b * t * t * (1 - t) + t * t * t
    }

    /// The eased progress at time `x` (0...1), as app.js's `easeAt`.
    func at(_ x: Double) -> Double {
        if x <= 0 { return 0 }
        if x >= 1 { return 1 }
        var lo = 0.0
        var hi = 1.0
        for _ in 0..<24 {
            let mid = (lo + hi) / 2
            if CubicEase.axis(x1, x2, mid) < x { lo = mid } else { hi = mid }
        }
        return CubicEase.axis(y1, y2, (lo + hi) / 2)
    }

    /// The time (0...1) at which the eased progress is `p`, as app.js's `timeAt`.
    func time(at p: Double) -> Double {
        if p <= 0 { return 0 }
        if p >= 1 { return 1 }
        var lo = 0.0
        var hi = 1.0
        for _ in 0..<24 {
            let mid = (lo + hi) / 2
            if CubicEase.axis(y1, y2, mid) < p { lo = mid } else { hi = mid }
        }
        return CubicEase.axis(x1, x2, (lo + hi) / 2)
    }
}

/// How far (0...1) a stretch of `len` ms beginning at `at` has run by `t`.
func crownRun(_ t: Double, _ at: Double, _ len: Double) -> Double {
    len <= 0 ? (t >= at ? 1 : 0) : min(max((t - at) / len, 0), 1)
}

/// A CSS keyframe track: each stop's place (0...1), its value, and the ease to the next stop.
struct CrownTrack {
    let stops: [(at: Double, value: Double, ease: CubicEase)]

    func value(_ u: Double) -> Double {
        guard let first = stops.first, let last = stops.last else { return 0 }
        if u <= first.at { return first.value }
        if u >= last.at { return last.value }
        for i in 0..<(stops.count - 1) where u < stops[i + 1].at {
            let a = stops[i]
            let b = stops[i + 1]
            let k = a.ease.at((u - a.at) / (b.at - a.at))
            return a.value + (b.value - a.value) * k
        }
        return last.value
    }
}

/// CSS's smoothstep.
func crownSmooth(_ a: Double, _ b: Double, _ x: Double) -> Double {
    let t = min(max((x - a) / (b - a), 0), 1)
    return t * t * (3 - 2 * t)
}

extension Arch {
    /// How far inside the head a point is, in card widths (negative: outside); x across from the
    /// left springing, y up from the springing line (app.js's `archNow().inside`).
    func inside(_ x: Double, _ y: Double) -> Double {
        let h = Double(haunch)
        let c = Double(centre)
        let cy = -Double(depth)
        let r = Double(radius)
        let len = hypot(h - c, -cy)
        let turn = atan2(cy + (-cy) * r / len, c + (h - c) * r / len - h)
        func left(_ x: Double, _ y: Double) -> Double {
            if y <= 0 { return x }
            if atan2(y, x - h) >= turn { return h - hypot(x - h, y) }
            return r - hypot(x - c, y - cy)
        }
        return min(left(x, y), left(1 - x, y))
    }
}

/// One of the sky's stars as the play sees it: its centre on the screen, its size, its turn.
struct CrownStar {
    let index: Int
    let center: CGPoint
    let size: Double
}

/**
 * The sums for one play, in screen points and ms from the tap, made once at the tap from where
 * home's parts stand (app.js's `compute()`): the front's ellipse in the head, the spread's shape
 * and its sizes, when each line's ink hands over, each star's turn, and when the page has settled.
 */
struct CrownScene {
    let dusk: Bool
    let fig: CrownFigures
    let arch: Arch
    /// The card (the arch's full width, its top the point of the head) and the head's box inside
    /// it, down to the inscription band.
    let card: CGRect
    let vault: CGRect
    /// The head inside the slate course: the sky's own clip.
    let vaultPath: Path
    let screen: CGSize

    // The front: an ellipse `a` times as wide as tall about (cx, cy), its vertical radius
    // growing from r0 to ra (where its middle reaches the head's far side) and on to r.
    let a: Double
    let cx: Double
    let cy: Double
    let r0: Double
    let ra: Double
    let r: Double

    // The spread: the arch's shape, feathered, grown by ratio about the middle of the springing.
    let width: Double
    let ax: Double
    let ys: Double
    let feather: Double
    let s0: Double
    let s1: Double
    let footDepth: Double
    let margin: Double
    let top: Double
    let deep: Double
    let nodes: [Double]
    let veilNodes: [Double]
    let dur: Double
    let shapeDur: Double
    let veilDur: Double
    /// The veil's whole span, from `fig.veilFrom` (longer on a wide screen, as the spread is).
    let veilSpan: Double

    /// The light on the horizon: its height and (at dawn) its half-width.
    let washHeight: Double
    let washRadius: Double

    /// When each of the head's words and the page's rows hands its ink over, ms from the tap.
    let words: [String: Double]
    let rows: [String: Double]
    let wordRects: [String: CGRect]
    let rowRects: [String: CGRect]

    let stars: [CrownStar]
    /// Each star's turn, ms from the tap: at dusk when it kindles, at dawn when it goes out.
    let starTurns: [Double]
    /// The star nearest the cross: the evening star, and the morning star; and (dawn) when the
    /// plaster's front nears it and its gold deepens.
    let evening: Int?
    let morningAt: Double?

    /// When the page has turned everywhere, when the stills go, the ending beat (the page still
    /// for `beatHold`, then one thing), and the play's end.
    let covered: Double
    let settle: Double
    let beat: Double
    let end: Double

    /// Where a front's feather is half through, and where it begins (style.css's stops).
    static let mid = 0.89
    static let core = 0.78
    static let frontStops: [(Double, Double)] = [(0, 1), (0.78, 1), (0.835, 0.844), (0.89, 0.5), (0.945, 0.156), (1, 0)]

    /**
     * `card` and `headHeight` place the arch, `words` and `rows` are the frames of the head's
     * lines and the page's rows, `crown` the cross's, `stars` the sky's (centres on the screen).
     */
    init(dusk: Bool, fig: CrownFigures, arch: Arch, card: CGRect, headHeight: CGFloat, screen: CGSize,
         words: [String: CGRect], rows: [String: CGRect], crown: CGRect, stars: [CrownStar]) {
        self.dusk = dusk
        self.fig = fig
        self.arch = arch
        self.card = card
        self.screen = screen
        let v = CGRect(x: card.minX, y: card.minY, width: card.width, height: headHeight)
        vault = v
        let path = archPath(arch, width: card.width, outset: -5, foot: headHeight + 2, at: card.origin)
        vaultPath = path
        let w = Double(v.width)
        let vh = Double(v.height)
        let apex = Double(path.boundingRect.minY - v.minY)
        let hc = max(1, vh - apex)

        // The front.
        var a: Double, cyLocal: Double, r0: Double, ra: Double, r: Double
        if dusk {
            // About a point 0.6 of the head above its own, twice as wide as tall.
            a = 2
            cyLocal = apex - 0.6 * hc
            r0 = 0.6 * hc
            ra = (vh - cyLocal) / CrownScene.mid
            r = (w * w / (4 * a * a) + 1.6 * hc * 1.6 * hc).squareRoot() / CrownScene.core + 4
        } else {
            // A low dome about a point 0.3 of the head under the horizon.
            a = 2.5
            cyLocal = vh + 0.3 * hc
            r0 = 0.3 * hc
            ra = (cyLocal - apex) / CrownScene.mid
            r = 1.3 * hc / CrownScene.core + 4
        }
        r = max(r, ra + 1)
        self.a = a
        cx = Double(v.midX)
        cy = Double(v.minY) + cyLocal
        self.r0 = r0
        self.ra = ra
        self.r = r
        let frontCy = Double(v.minY) + cyLocal
        let frontAt = { (y: Double) -> Double in
            let need = abs(y - frontCy) / CrownScene.mid
            return fig.front.at + 0.95 * fig.front.len * fig.frontEase.time(at: (need - r0) / (ra - r0))
        }

        // The spread.
        let W = w
        let ax = Double(card.midX)
        let ys = Double(card.minY) + Double(arch.rise) * W
        let yc = Double(v.maxY)
        let fw = fig.spreadFeather
        let F = 0.5
        let s0 = 1 / (1 + fw)
        let D = max(0.05, (yc - ys) / (W * s0) - F * (1 + fw))
        let shape = { (x: Double, y: Double) -> Double in
            y >= -D ? arch.inside(x, y) : 0.5 * (1 - hypot((x - 0.5) / 0.5, (y + D) / F))
        }
        let corners = [(0.0, 0.0), (Double(screen.width), 0.0), (0.0, Double(screen.height)), (Double(screen.width), Double(screen.height))]
        let cornerAt = { (sc: Double) -> Double in
            corners.map { shape(($0.0 - ax) / (W * sc) + 0.5, (ys - $0.1) / (W * sc)) }.min() ?? 0
        }
        var s1 = 1.0
        while cornerAt(s1) < fw / 2 && s1 < 60 { s1 += 0.05 }
        s1 *= 1.02
        let wide = Double(screen.width) > 700 ? fig.spreadWide : 0
        let dur = fig.spread.len + wide
        let m0 = fw / 2 + 0.012
        width = W
        self.ax = ax
        self.ys = ys
        feather = fw
        self.s0 = s0
        self.s1 = s1
        footDepth = D
        margin = m0
        top = Double(arch.rise) + 2.4 * m0
        deep = D + F * (1 + fw) + 0.02
        self.dur = dur
        let sizeAt = { (tau: Double) -> Double in
            let e = fig.spreadEase.at(tau / dur)
            return 0.5 * s0 * pow(s1 / s0, e) + 0.5 * (s0 + e * (s1 - s0))
        }
        let vDur = (dur * fig.veilPart).rounded()
        let shapeDur = (dur * fig.spreadShapePart).rounded()
        veilDur = vDur
        veilSpan = fig.veilTo - fig.veilFrom + wide
        self.shapeDur = shapeDur
        var nodes: [Double] = []
        var veilNodes: [Double] = []
        for i in 0...8 {
            // The veil's shape is the spread's whole course run through in `veilPart` of it: over
            // the whole page while the veil is still next to nothing, so no edge of it crosses.
            veilNodes.append(sizeAt(dur * Double(i) / 8))
            nodes.append(s0 * pow(s1 / s0, pow(Double(i) / 8, fig.spreadBend)))
        }
        self.nodes = nodes
        self.veilNodes = veilNodes
        let alphaAt = { (X: Double, Y: Double, time: Double) -> Double in
            let u = min(max((time - fig.spread.at) / dur, 0), 1)
            let f = min(1, (time - fig.spread.at) / shapeDur) * 8
            let i = max(0, min(7, Int(f.rounded(.down))))
            let sc = nodes[i] + (max(0, f) - Double(i)) * (nodes[i + 1] - nodes[i])
            return crownSmooth(-fw / 2, fw / 2, shape((X - ax) / (W * sc) + 0.5, (ys - Y) / (W * sc)))
                * fig.glowEase.at(u / fig.spreadGlowPart)
        }
        let when = { (X: Double, Y: Double, level: Double) -> Double in
            for i in 0...120 {
                let time = fig.spread.at + dur * Double(i) / 120
                if alphaAt(X, Y, time) >= level { return time }
            }
            return fig.spread.at + dur
        }
        let sw = Double(screen.width)
        let sh = Double(screen.height)
        covered = ([(0.0, 0.0), (sw, 0), (0, sh), (sw, sh), (sw / 2, sh), (sw / 2, 0)].map { when($0.0, $0.1, 0.96) }.max() ?? 0) + 40

        // The light on the horizon stops under the last line of the day.
        let last = words.values.filter { $0.height > 0 }.map { Double($0.maxY) }.max() ?? Double(v.minY) + apex + 0.5 * hc
        let room = max(0.13 * hc, min(0.5 * hc, Double(v.maxY) - last - 4))
        washHeight = room
        washRadius = min(0.48 * w, 2.6 * room)

        // The day's lines hand over as the head's front reaches them.
        let shownWords = words.filter { $0.value.height > 0 && $0.value.width > 0 }
        wordRects = shownWords
        self.words = shownWords.mapValues { frontAt(Double($0.minY) + Double($0.height) * 0.6) }

        // The rows change ink one after another, each as the page under it is turned far enough
        // to carry the new ink, dealt out 14ms apart about the middle one, none further than
        // 60ms from its own moment and never two within 10ms: the wall is shaded to the new
        // look's value before it is crossed, so the rows come back together, in one breath.
        let shownRows = rows.filter { $0.value.height > 0 && $0.value.width > 0 }
        rowRects = shownRows
        var order = shownRows.map { (key: $0.key, at: when(Double($0.value.midX), Double($0.value.midY), fig.inkAt), y: Double($0.value.minY)) }
        order.sort { $0.at != $1.at ? $0.at < $1.at : $0.y < $1.y }
        var dealt: [String: Double] = [:]
        if !order.isEmpty {
            let middle = (order.count - 1) / 2
            let base = order[middle].at
            var previous = -Double.infinity
            for (n, row) in order.enumerated() {
                var t = max(row.at - 60, min(row.at + 60, base + Double(n - middle) * 14))
                if n > 0 && t < previous + 10 { t = previous + 10 }
                previous = t
                dealt[row.key] = t
            }
        }
        self.rows = dealt
        let lastDip = dealt.values.max() ?? 0
        let settle = max(covered, lastDip + 220, fig.spread.at + 1100, fig.veilTo - 80, fig.spread.at + dur * fig.spreadGlowPart - 50) + 80
        self.settle = settle
        // The ending beat: the page is still for `beatHold`, then one thing (the last star, or
        // the morning star and the cross), then the end.
        let beat = settle + fig.beatHold
        self.beat = beat
        end = max(fig.end, beat + fig.beatLen + 30)

        // Each star's turn, by magnitude, from the change of look (app.js's `starTurns`; the sums
        // are made in ms from the turn, as the web's are, and given from the tap). Dusk: the
        // evening star (the one nearest the cross) as the dark first reaches it, alone; then the
        // brightest, one at a time; then the field, brightest to faintest, until the page is
        // still; and after the held breath the last and faintest. None before the dark has
        // reached it. Dawn: the field goes out faintest first, then the brightest one at a time,
        // none long after the plaster's front; the morning star stays for the ending beat.
        self.stars = stars
        let crossAt = CGPoint(x: crown.midX, y: crown.midY)
        let nearest = stars.indices.min { hypot(stars[$0].center.x - crossAt.x, stars[$0].center.y - crossAt.y) < hypot(stars[$1].center.x - crossAt.x, stars[$1].center.y - crossAt.y) }
        evening = nearest
        let turn = fig.turn
        let settleIn = settle - turn
        let beatIn = beat - turn
        let dark = { (i: Int) -> Double in frontAt(Double(stars[i].center.y)) - turn }
        let seedOf = { (i: Int) -> Double in Double((i * 7919) % 97) / 97 }
        var out = [Double](repeating: beatIn, count: stars.count)
        // Brightest first; equal stars keep their order.
        let rest = stars.indices.filter { $0 != nearest }.sorted { stars[$0].size != stars[$1].size ? stars[$0].size > stars[$1].size : $0 < $1 }
        let bright = Array(rest.prefix(fig.starsBright))
        var field = Array(rest.dropFirst(fig.starsBright))
        let apart = fig.starsBrightApart
        var morning: Double?
        if dusk {
            let first = nearest.map { max(150, dark($0) + 60) } ?? 150
            if let nearest { out[nearest] = first }
            let lastOne: Int? = field.count > 1 ? field.removeLast() : nil
            for (n, i) in bright.enumerated() {
                out[i] = max(first + fig.eveningStarAlone + Double(n) * apart, dark(i) + 100)
            }
            let from = first + fig.eveningStarAlone + Double(bright.count) * apart
            let to = max(from, settleIn - 0.3 * fig.starKindle)
            for (n, i) in field.enumerated() {
                let star = stars[i]
                let sy = Double(star.center.y)
                var t = from + (to - from) * (Double(n) + 0.6 * (seedOf(i) - 0.5)) / Double(max(1, field.count - 1))
                t = max(from, min(to, t), dark(i) + 100)
                // A star out in the moulding's margin waits for the wall's night.
                if arch.inside((Double(star.center.x) - Double(card.minX)) / W, (ys - sy) / W) < (5 + 9) / W {
                    t = max(t, when(Double(star.center.x), sy, 0.5) - turn + 200)
                }
                out[i] = min(t, settleIn - 0.3 * fig.starKindle)
            }
            if let lastOne { out[lastOne] = beatIn }
        } else {
            let until = fig.dawnFieldOut * settleIn - Double(bright.count) * apart
            for (n, i) in field.reversed().enumerated() {
                let t = 40 + (until - 40) * (Double(n) + 0.6 * (seedOf(i) - 0.5)) / Double(max(1, field.count - 1))
                out[i] = max(0, min(t, until, dark(i) + fig.dawnStarLinger))
            }
            for (n, i) in bright.reversed().enumerated() {
                out[i] = max(0, min(until + Double(n + 1) * apart, dark(i) + fig.dawnStarLinger))
            }
            if let nearest {
                out[nearest] = beatIn
                // When the plaster's front nears the morning star: its gold deepens there, to
                // stand on the light wall.
                morning = (dark(nearest) - 300).rounded() + turn
            }
        }
        morningAt = morning
        starTurns = out.map { $0.rounded() + turn }
    }

    // MARK: The front

    /// The front's vertical radius at `t`: eased to `ra` by 95% of its span, then on to `r`.
    func frontRadius(_ t: Double) -> Double {
        let u = crownRun(t, fig.front.at, fig.front.len)
        if u <= 0.95 { return r0 + (ra - r0) * fig.frontEase.at(u / 0.95) }
        return ra + (r - ra) * (u - 0.95) / 0.05
    }

    /// The front's ellipse at `t`, `scale` times its size: beyond it the front has not reached.
    func frontEllipse(_ t: Double, scale: Double = 1) -> Path {
        let R = frontRadius(t) * scale
        return Path(ellipseIn: CGRect(x: cx - a * R, y: cy - R, width: 2 * a * R, height: 2 * R))
    }

    /// How far the front has turned the head at a point (0...1).
    func frontAlpha(_ p: CGPoint, _ t: Double) -> Double {
        let R = frontRadius(t)
        let d = hypot((Double(p.x) - cx) / a, Double(p.y) - cy) / R
        let stops = CrownScene.frontStops
        if d <= 0 { return 1 }
        for i in 1..<stops.count where d <= stops[i].0 {
            let (x0, y0) = stops[i - 1]
            let (x1, y1) = stops[i]
            return y0 + (y1 - y0) * (d - x0) / (x1 - x0)
        }
        return 0
    }

    // MARK: The spread

    /// The size of the page's shape at `t`, in card widths of its first size.
    func spreadScale(_ t: Double) -> Double {
        let f = min(1, max(0, t - fig.spread.at) / shapeDur) * 8
        let i = max(0, min(7, Int(f.rounded(.down))))
        return nodes[i] + (f - Double(i)) * (nodes[i + 1] - nodes[i])
    }

    /// The size of the veil's shape at `t`.
    func veilScale(_ t: Double) -> Double {
        let f = min(1, max(0, t - fig.spread.at) / veilDur) * 8
        let i = max(0, min(7, Int(f.rounded(.down))))
        return (veilNodes[i] + (f - Double(i)) * (veilNodes[i + 1] - veilNodes[i])) * fig.veilLead
    }

    /// The page's own opacity at `t`.
    func glow(_ t: Double) -> Double {
        fig.glowEase.at(crownRun(t, fig.spread.at, dur) / fig.spreadGlowPart)
    }

    /// Where the shape's image lies at size `sc`.
    func maskRect(_ sc: Double) -> CGRect {
        let w = (1 + 2 * margin) * width * sc
        return CGRect(x: ax - w / 2, y: ys - top * width * sc, width: w, height: (top + deep) * width * sc)
    }

    /// The last 250ms before the settle bring the shape up to cover everything (0...1).
    func topUp(_ t: Double) -> Double {
        CubicEase.easeOut.at(crownRun(t, settle - 250, 250))
    }

    /// The shape, feathered, as an image: the arch's head and its jambs, closed by a round foot as
    /// deep as the page stands below the springing (app.js's `maskFor`). White, its alpha the
    /// share of the new look.
    func maskImage() -> CGImage? {
        let nx = 120
        let q = Double(nx) / (1 + 2 * margin)
        let ny = Int(((top + deep) * q).rounded(.up))
        guard ny > 0 else { return nil }
        var bytes = [UInt8](repeating: 0, count: nx * ny * 4)
        let fw = feather
        let D = footDepth
        for row in 0..<ny {
            let y = top - (Double(row) + 0.5) / q
            for i in 0..<nx {
                let x = (Double(i) + 0.5) / q - margin
                let s = y >= -D ? arch.inside(x, y) : 0.5 * (1 - hypot((x - 0.5) / 0.5, (y + D) / 0.5))
                let alpha = UInt8((255 * crownSmooth(-fw / 2, fw / 2, s)).rounded())
                let k = (row * nx + i) * 4
                // Premultiplied white.
                bytes[k] = alpha
                bytes[k + 1] = alpha
                bytes[k + 2] = alpha
                bytes[k + 3] = alpha
            }
        }
        guard let provider = CGDataProvider(data: Data(bytes) as CFData) else { return nil }
        return CGImage(
            width: nx, height: ny, bitsPerComponent: 8, bitsPerPixel: 32, bytesPerRow: nx * 4,
            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGBitmapInfo(rawValue: CGImageAlphaInfo.premultipliedLast.rawValue),
            provider: provider, decode: nil, shouldInterpolate: true, intent: .defaultIntent
        )
    }

    // MARK: The veil

    /// The veil's strength at `t` (style.css's `crown-veil-dusk` and `crown-veil-dawn`): from
    /// rest to `veilPeak` at `veilTop` of its span, by `veilIn`, and gone by `veilGone`, by
    /// `veilOut`. Dusk deepens while the new page has barely begun and leaves on a long tail;
    /// dawn is at its height while the wall is still dark and lifted as the plaster comes.
    func veilOpacity(_ t: Double) -> Double {
        guard t >= fig.veilFrom else { return 0 }
        let u = crownRun(t, fig.veilFrom, veilSpan)
        if u <= fig.veilTop { return fig.veilPeak * fig.veilIn.at(u / fig.veilTop) }
        if u < fig.veilGone { return fig.veilPeak * (1 - fig.veilOut.at((u - fig.veilTop) / (fig.veilGone - fig.veilTop))) }
        return 0
    }

    // MARK: The frieze

    /// How far the frieze has taken the new look at `t` (0...1): the land, dark first at dusk, a
    /// silhouette against the afterglow, and light last at dawn.
    func friezeTurn(_ t: Double) -> Double {
        CubicEase(0.4, 0, 0.3, 1).at(crownRun(t, fig.frieze.at, fig.frieze.len))
    }

    // MARK: Ink

    /// How far a line's old ink has gone at `t` (0...1).
    func inkOut(_ key: String, _ t: Double) -> Double {
        if let w = words[key] { return CubicEase(0.4, 0, 0.7, 1).at(crownRun(t, w - 230, 240)) }
        if let r = rows[key] { return crownRun(t, r - fig.inkHandover / 2, fig.inkHandover) }
        return 0
    }

    /// How far a line's new ink has come at `t` (0...1).
    func inkIn(_ key: String, _ t: Double) -> Double {
        if let w = words[key] { return CubicEase(0.3, 0, 0.4, 1).at(crownRun(t, w - 80, 360)) }
        if let r = rows[key] { return crownRun(t, r - fig.inkHandover / 2, fig.inkHandover) }
        return 1
    }
}
