import SwiftUI
import UIKit
import UIKit.UIGestureRecognizerSubclass

/*
 * Home's crown: the consecration cross at the head of home's arch is a button, never announced.
 * A tap turns the look over (presentation::crown::after_crown_tap, saved as the menu's Theme row
 * saves it) and plays the change as dusk or dawn, to the core's timetable (CrownPlan.swift), as
 * the web's "Home: the crown's play" does with a view transition:
 *
 * - Act one is drawn over the page from the tap, in a layer of the window above the app's own
 *   views that takes no touches: the cross's two looks, the light on the horizon, and the stars.
 * - At the turn the page is photographed (the old look), then again with its rows' and the day's
 *   words' ink taken off (its ground), and the theme is changed under the photographs. The new
 *   look is live from then on: the old one is let go of through the front crossing the head and
 *   then through the arch's own shape spreading over the page, feathered wide, under a veil of
 *   the hour's colour multiplied over both (a layer of its own, blended, so it only shades). Each
 *   row and line of the day hands its ink over at its own moment: the old ink goes from the
 *   photograph and the live line's new ink comes in. The frieze holds its old look over all of
 *   it until its own span. A soft tick marks the turn.
 * - At the settle the photographs go, and the page is its live self; after a held breath, one
 *   last thing (the last star, or the morning star and the cross) ends the play.
 *
 * A touch anywhere else runs what is left out quickly and goes on to the control touched; a
 * scroll, a turn of the phone, opening the menu or another page, or the app leaving the screen
 * ends it at once. Reduce Motion, Increase Contrast and a black day take the menu's own change.
 */

/// Where the play's parts stand on the screen, read as home lays them out.
final class CrownFrames {
    var value: [String: CGRect] = [:]
}

/// Frames of home's parts by name, in the window's space: "card" (the frontispiece), "head" (the
/// day's part, under the arch), "crown" and "tap", the day's lines ("w…") and the page's rows ("r…").
struct CrownFrameKey: PreferenceKey {
    static let defaultValue: [String: CGRect] = [:]
    static func reduce(value: inout [String: CGRect], nextValue: () -> [String: CGRect]) {
        value.merge(nextValue()) { $1 }
    }
}

extension View {
    /// Enters this view's frame among the crown's parts under `key`.
    func crownFrame(_ key: String) -> some View {
        background(GeometryReader { g in
            Color.clear.preference(key: CrownFrameKey.self, value: [key: g.frame(in: .global)])
        })
    }

    /// A row of the page ("r…") or a line of the day ("w…") whose ink the crown's play hands over
    /// in one move; at rest it is untouched.
    func crownInk(_ key: String) -> some View {
        modifier(CrownInk(key: key)).crownFrame(key)
    }
}

private struct CrownInk: ViewModifier {
    let key: String
    @ObservedObject private var player = CrownPlayer.shared

    func body(content: Content) -> some View {
        TimelineView(.animation(paused: !player.inking)) { _ in
            // Never quite nothing, so the line still takes its taps while its ink is away.
            content.opacity(player.inking ? max(0.002, player.ink(key)) : 1)
        }
    }
}

/// A button that shows nothing of being one: the crown is never announced.
struct Unmarked: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label.contentShape(Rectangle())
    }
}

/// What the crown is on a day: what its looks are made from, whichever way it turns.
struct CrownHead {
    let arch: Arch
    let first: Bool
    let black: Bool
    let season: String
    let crownSize: CGFloat
    let crownTop: CGFloat
    /// The day's words, in the card's own space, which the sky's stars keep clear of.
    let reserve: [CGRect]
}

/**
 * The look of home's consecration cross on a page dark or light: leaf on the night sky, gilt on a
 * first-class feast by day, painted otherwise; thin on a bare black night.
 */
func crownMark(dark: Bool, season: String, first: Bool, black: Bool, size: CGFloat) -> Crown {
    let p = (dark ? Palette.apse : Palette.nave).seasoned(season)
    let o = Ornament.of(p, season: season)
    if dark && black {
        let muted = HomeInk.of(p).skyMuted.opacity(0.7)
        return Crown(size: size, ink: muted, shading: .color(muted))
    }
    if dark {
        let leaf = Leaf(o)
        return Crown(size: size, ink: leaf.mid, shading: .linearGradient(Gradient(stops: [.init(color: leaf.hi, location: 0), .init(color: leaf.mid, location: 0.55), .init(color: leaf.lo, location: 1)]), startPoint: .zero, endPoint: CGPoint(x: size, y: size)), alpha: 0.35 + 0.65 * leafCatch(season))
    }
    if first {
        return Crown(size: size, ink: o.flat, shading: .linearGradient(Gradient(stops: [.init(color: o.hi, location: 0.08), .init(color: mix(o.hi, .white, 0.55), location: 0.3), .init(color: o.flat, location: 0.52), .init(color: o.lo, location: 0.92)]), startPoint: .zero, endPoint: CGPoint(x: size, y: size)))
    }
    return Crown(size: size, ink: p.lining)
}

/**
 * The tap's memory: the choice the last tap turned away from, kept only while the saved choice is
 * still the one that tap wrote, and never saved (presentation::crown's `restore`).
 */
struct CrownMemory {
    private(set) var turned: (from: ThemeChoice, wrote: ThemeChoice)?

    /// The choice a tap on the crown saves, `stored` being the saved one.
    mutating func next(stored: ThemeChoice, deviceDark: Bool) -> ThemeChoice {
        let restore = turned.flatMap { $0.wrote == stored ? $0.from : nil }
        let after = afterCrownTap(choice: stored.rawValue, deviceDark: deviceDark, restore: restore?.rawValue)
        let choice = ThemeChoice(rawValue: after.choice) ?? .system
        turned = after.restore.flatMap(ThemeChoice.init(rawValue:)).map { (from: $0, wrote: choice) }
        return choice
    }
}

/// How a play ends: run out, cut short where it stands, or before the turn as the menu's change.
enum CrownEnding { case end, cut, crossfade }

/// The crown's play: one at a time, and one way out of it.
final class CrownPlayer: NSObject, ObservableObject {
    static let shared = CrownPlayer()

    /// The play under way, if any: while there is one, home's own cross and stars stand down
    /// and the play draws them.
    @Published private(set) var run: CrownRun?
    /// Whether the rows' and the day's ink is the play's to hand over.
    @Published private(set) var inking = false

    private var memory = CrownMemory()
    private var lastTap = -Double.infinity
    private var link: CADisplayLink?
    private var watcher: CrownTouches?

    private override init() {
        super.init()
        let center = NotificationCenter.default
        center.addObserver(forName: UIApplication.didEnterBackgroundNotification, object: nil, queue: .main) { [weak self] _ in self?.cut() }
        center.addObserver(forName: UIAccessibility.reduceMotionStatusDidChangeNotification, object: nil, queue: .main) { [weak self] _ in
            if UIAccessibility.isReduceMotionEnabled { self?.cut() }
        }
        center.addObserver(forName: UIAccessibility.darkerSystemColorsStatusDidChangeNotification, object: nil, queue: .main) { [weak self] _ in
            if UIAccessibility.isDarkerSystemColorsEnabled { self?.cut() }
        }
    }

    static func now() -> Double { CACurrentMediaTime() * 1000 }

    /// The new ink of a row or line of the day, at this moment.
    func ink(_ key: String) -> Double { run?.ink(key) ?? 1 }

    /**
     * A tap on the crown. While the page is turning a second tap is kept, once, and plays the way
     * back at the settle (not the taps of a double tap: only one a moment after the last); after
     * the settle it is the way back at once.
     */
    func tap(model: AppModel, head: CrownHead, frames: CrownFrames, plain: Bool = false) {
        let now = CrownPlayer.now()
        if let run {
            if !run.settled {
                let apart = run.fig.tapApart
                if (run.ready || now - lastTap > 2 * apart) && now - lastTap > apart { run.again.toggle() }
                lastTap = now
                return
            }
            teardown(.cut)
        }
        lastTap = now
        turn(model: model, head: head, frames: frames, plain: plain)
    }

    private func turn(model: AppModel, head: CrownHead, frames: CrownFrames, plain: Bool) {
        let window = CrownPlayer.keyWindow()
        let deviceDark = CrownPlayer.deviceDark(window)
        let choice = memory.next(stored: model.theme, deviceDark: deviceDark)
        let toDark = choice.dark(deviceDark ? .dark : .light)
        // The menu's own change, crossfade and all: for a reader who asked for stillness or
        // contrast, a black day (no sky is played over Good Friday or All Souls), a page with
        // the menu open over it, or anything the play cannot stand on.
        let still = plain || UIAccessibility.isReduceMotionEnabled || UIAccessibility.isDarkerSystemColorsEnabled
            || head.black || model.overlay != .none
        guard !still, let window, let root = window.rootViewController?.view,
              let run = CrownRun(model: model, choice: choice, dusk: toDark, head: head, frames: frames, window: window, root: root, deviceDark: deviceDark)
        else {
            withAnimation(restyling) { model.turnTheme(choice) }
            return
        }
        self.run = run
        // The stage goes up in the same frame as home's own cross and stars stand down.
        let host = UIHostingController(rootView: CrownStageView(player: self))
        host.view.backgroundColor = .clear
        host.view.isUserInteractionEnabled = false
        host.view.frame = window.bounds
        host.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        host.safeAreaRegions = []
        window.addSubview(host.view)
        host.view.layoutIfNeeded()
        run.host = host
        crownTurning()
        let touches = CrownTouches { [weak self] point in self?.touched(point) }
        window.addGestureRecognizer(touches)
        watcher = touches
        let link = CADisplayLink(target: self, selector: #selector(tick))
        link.preferredFrameRateRange = CAFrameRateRange(minimum: 60, maximum: 120, preferred: 120)
        link.add(to: .main, forMode: .common)
        self.link = link
    }

    @objc private func tick() {
        guard let run else { return }
        guard let window = run.window, window.bounds.size == run.scene.screen else {
            // A turn of the phone or a new size: a new layout, and the photographs are stale.
            teardown(run.switched ? .cut : .crossfade)
            return
        }
        // The device's own look changed under a Default choice: what is left runs out.
        if CrownPlayer.deviceDark(window) != run.deviceDark { rush(run.fig.rush) }
        let t = run.clock()
        switch run.phase {
        case .act:
            if t >= run.fig.turn {
                // The old look, as it stands: the stage shows it over the page from now on.
                run.hold()
                guard let old = CrownPlayer.photograph(run.root, after: false) else {
                    teardown(.crossfade)
                    return
                }
                run.oldShot = Image(uiImage: old)
                run.phase = .photographed
                // The rows' and the day's ink stand down under it, in the same frame.
                inking = true
            }
        case .photographed:
            // The page's ground without its ink, then the new look under both.
            guard let ground = CrownPlayer.photograph(run.root, after: true) else {
                teardown(.crossfade)
                return
            }
            run.groundShot = Image(uiImage: ground)
            var quiet = Transaction()
            quiet.disablesAnimations = true
            withTransaction(quiet) { run.model.turnTheme(run.choice) }
            run.switched = true
            run.phase = .turned
            // One soft tick as the look turns under the reader's eyes.
            crownTurned()
        case .turned:
            // One clock: everything from here is timed from the first frame of the new look,
            // however long the photographs took.
            run.resume()
            run.ready = true
            run.phase = .playing
        case .playing:
            if t >= run.scene.settle { settle(run) }
        case .settled:
            if t >= run.scene.end { teardown(.end) }
        }
    }

    /// The page has visibly turned: the photographs go and the page is live; a tap kept while
    /// it turned plays the way back now.
    private func settle(_ run: CrownRun) {
        run.oldShot = nil
        run.groundShot = nil
        run.settled = true
        run.phase = .settled
        inking = false
        if run.again && !run.rushing {
            let tapped = lastTap
            teardown(.cut)
            turn(model: run.model, head: run.head, frames: run.frames, plain: false)
            lastTap = tapped
        }
    }

    /// Ends the play now, leaving the page in the look it was turning to.
    func teardown(_ how: CrownEnding) {
        guard let run else { return }
        self.run = nil
        link?.invalidate()
        link = nil
        if let watcher { watcher.view?.removeGestureRecognizer(watcher) }
        watcher = nil
        run.host?.view.removeFromSuperview()
        run.host = nil
        inking = false
        if !run.switched {
            if how == .crossfade {
                withAnimation(restyling) { run.model.turnTheme(run.choice) }
            } else {
                var quiet = Transaction()
                quiet.disablesAnimations = true
                withTransaction(quiet) { run.model.turnTheme(run.choice) }
            }
        }
    }

    /// Something else has happened to the page (another page, the menu, a new theme): the play
    /// ends where it stands. `owner` limits it to the play of one home.
    func cut(owner: CrownFrames? = nil) {
        guard let run, owner == nil || owner === run.frames else { return }
        teardown(run.switched ? .cut : .crossfade)
    }

    /// Home's parts have moved (a scroll, a new layout): the photographs cannot move with them.
    func moved(_ frames: CrownFrames) {
        guard let run, frames === run.frames, let card = frames.value["card"] else { return }
        let was = run.scene.card
        if abs(card.minX - was.minX) > 0.5 || abs(card.minY - was.minY) > 0.5 || abs(card.width - was.width) > 0.5 {
            teardown(run.switched ? .cut : .crossfade)
        }
    }

    /// A touch elsewhere does not cut the play: what is left runs out in `ms`. Before the turn
    /// there is nothing to run out, and the menu's crossfade takes it.
    func rush(_ ms: Double) {
        guard let run, !run.rushing else { return }
        guard run.switched else {
            teardown(.crossfade)
            return
        }
        run.rushing = true
        run.speed(max(1, (run.scene.end - run.clock()) * run.fig.pace / ms))
    }

    private func touched(_ point: CGPoint) {
        guard let run else { return }
        if let tap = run.frames.value["tap"], tap.contains(point) { return }
        rush(run.fig.rush)
    }

    static func keyWindow() -> UIWindow? {
        let windows = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.flatMap(\.windows)
        return windows.first(where: \.isKeyWindow) ?? windows.first
    }

    /// Whether the device itself is dark, whatever the app's theme makes its window.
    static func deviceDark(_ window: UIWindow?) -> Bool {
        let traits = window?.windowScene?.screen.traitCollection ?? UIScreen.main.traitCollection
        return traits.userInterfaceStyle == .dark
    }

    /// The app's own views as they stand (`after`: once the changes pending are drawn); the
    /// stage, a view of the window beside them, is not in it.
    static func photograph(_ view: UIView, after: Bool) -> UIImage? {
        let format = UIGraphicsImageRendererFormat(for: view.traitCollection)
        format.opaque = true
        var drawn = false
        let image = UIGraphicsImageRenderer(bounds: view.bounds, format: format).image { _ in
            drawn = view.drawHierarchy(in: view.bounds, afterScreenUpdates: after)
        }
        return drawn ? image : nil
    }
}

/// Watches the window's touches without taking them: a touch anywhere but the crown runs the
/// play out, and goes on to what it touched.
private final class CrownTouches: UIGestureRecognizer, UIGestureRecognizerDelegate {
    private let began: (CGPoint) -> Void

    init(_ began: @escaping (CGPoint) -> Void) {
        self.began = began
        super.init(target: nil, action: nil)
        cancelsTouchesInView = false
        delaysTouchesBegan = false
        delaysTouchesEnded = false
        delegate = self
    }

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent) {
        if let touch = touches.first { began(touch.location(in: view)) }
        state = .failed
    }

    func gestureRecognizer(_ gestureRecognizer: UIGestureRecognizer, shouldRecognizeSimultaneouslyWith other: UIGestureRecognizer) -> Bool { true }
}

/// One play, from the tap to its end: its sums, its clock, its photographs, and its drawing.
final class CrownRun {
    enum Phase { case act, photographed, turned, playing, settled }

    let model: AppModel
    let choice: ThemeChoice
    let dusk: Bool
    let head: CrownHead
    let frames: CrownFrames
    let fig: CrownFigures
    let scene: CrownScene
    let deviceDark: Bool
    weak var window: UIWindow?
    let root: UIView
    var host: UIHostingController<CrownStageView>?

    var phase = Phase.act
    /// The new look has been put under the photographs.
    var switched = false
    /// The clock runs again after the photographs.
    var ready = false
    var settled = false
    /// A second tap was kept: the way back plays at the settle.
    var again = false
    var rushing = false
    var oldShot: Image?
    var groundShot: Image?

    private let mask: Image?
    private let starPaths: [Path]
    private let starLeaf: [Double]
    private let starLight: GraphicsContext.Shading
    private let lit: Double
    private let oldCross: Crown
    private let newCross: Crown
    /// The cross's ember gold: the last light it keeps at dusk, the first it takes at dawn.
    private let emberCross: Crown
    private let catchInk: Color
    /// A gilt cross by day answers dusk with a catch of light at once.
    private let gilt: Bool
    private let crown: CGRect
    /// The frieze, the land under the sky: it changes on its own span, over everything else.
    private let frieze: CGRect?
    private let pigment: UIImage?

    // The clock: ms from the tap, held while the photographs are taken, sped by a rush.
    private var base = 0.0
    private var since: Double?
    private var rate = 1.0

    init?(model: AppModel, choice: ThemeChoice, dusk: Bool, head: CrownHead, frames: CrownFrames, window: UIWindow, root: UIView, deviceDark: Bool) {
        let f = frames.value
        guard let card = f["card"], let headFrame = f["head"], let crown = f["crown"], card.width > 0, headFrame.height > 0 else { return nil }
        self.model = model
        self.choice = choice
        self.dusk = dusk
        self.head = head
        self.frames = frames
        self.deviceDark = deviceDark
        self.window = window
        self.root = root
        self.crown = crown
        // On whole points, so the band's cut and its redrawing meet on the screen's pixels.
        frieze = f["frieze"].flatMap { $0.width > 0 && $0.height > 0 ? $0.integral : nil }
        let fig = CrownFigures.of(dusk: dusk)
        self.fig = fig
        let band = headFrame.height
        let placed = SkyStar.placed(head.arch, width: card.width, band: band, first: head.first,
                                    reserve: Sky.starReserve(head.reserve, width: card.width, crown: head.crownSize, top: head.crownTop))
        let stars = placed.enumerated().map { i, s in
            CrownStar(index: i, center: CGPoint(x: card.minX + s.center.x, y: card.minY + s.center.y), size: Double(s.size))
        }
        let scene = CrownScene(
            dusk: dusk, fig: fig, arch: head.arch, card: card, headHeight: band, screen: window.bounds.size,
            words: f.filter { $0.key.hasPrefix("w") }, rows: f.filter { $0.key.hasPrefix("r") }, crown: crown, stars: stars
        )
        self.scene = scene
        mask = scene.maskImage().map { Image(decorative: $0, scale: 1) }
        // The stars, in the screen's space, with the night's own leaf.
        let move = CGAffineTransform(translationX: card.minX, y: card.minY)
        starPaths = placed.map { $0.path.applying(move) }
        starLeaf = placed.map(\.leaf)
        let night = Palette.apse.seasoned(head.season)
        let leaf = Leaf(Ornament.of(night, season: head.season))
        starLight = .radialGradient(
            Gradient(stops: [.init(color: leaf.hi, location: 0), .init(color: leaf.mid, location: 0.6), .init(color: leaf.lo, location: 1)]),
            center: CGPoint(x: card.midX, y: card.minY + band * 0.62), startRadius: 0, endRadius: card.width * 0.72
        )
        lit = Double(leafCatch(head.season))
        oldCross = crownMark(dark: !dusk, season: head.season, first: head.first, black: head.black, size: crown.width)
        newCross = crownMark(dark: dusk, season: head.season, first: head.first, black: head.black, size: crown.width)
        let ember = fig.crossEmber.color()
        emberCross = Crown(size: crown.width, ink: ember, shading: .color(ember))
        catchInk = mix(leaf.hi, .white, 0.62)
        gilt = dusk && head.first
        pigment = UIImage(named: "pigment.png")
        since = CrownPlayer.now()
    }

    // MARK: The clock

    func clock() -> Double {
        guard let since else { return base }
        return base + rate * (CrownPlayer.now() - since) / fig.pace
    }

    func hold() {
        base = clock()
        since = nil
    }

    func resume() {
        since = CrownPlayer.now()
    }

    func speed(_ r: Double) {
        base = clock()
        if since != nil { since = CrownPlayer.now() }
        rate = r
    }

    // MARK: Ink

    /// The live line's new ink: away while the photographs are taken, then in at its moment.
    func ink(_ key: String) -> Double {
        guard switched else { return 0 }
        return scene.inkIn(key, clock())
    }

    // MARK: The cross

    /// The cross's two looks, its ember gold and the leaf's catch of light, at `t`.
    func crosses(_ t: Double) -> some View {
        let old: Double
        let new: Double
        let gleam: Double
        var ember = 0.0
        let rise = CubicEase(0.3, 0, 0.3, 1)
        if dusk {
            // The leaf is laid over the paint first, then the paint goes; a cross already gilt
            // answers with a catch of light at once. Then it keeps the last of the light: an
            // ember gold that comes with the afterglow and stays on it after the wall is dark,
            // going only as the page comes to rest.
            let ease = CubicEase(0.4, 0, 0.4, 1)
            new = ease.at(crownRun(t, 80, 760))
            old = 1 - ease.at(crownRun(t, 780, 300))
            let flash = CrownTrack(stops: [(0, 0, CubicEase(0.4, 0, 0.6, 1)), (0.4, 0.6, CubicEase(0.4, 0, 0.6, 1)), (1, 0, CubicEase.linear)])
            gleam = gilt ? flash.value(crownRun(t, 80, 760)) : 0
            if switched {
                ember = 0.85 * rise.at(crownRun(t, fig.turn + 500, 900)) * (1 - rise.at(crownRun(t, scene.beat - 970, 600)))
            }
        } else {
            // The leaf lifts at once and settles. As the plaster nears it the cross deepens to
            // the ember gold, which stands on cream where leaf alone is lost; and it is the last
            // thing to change: at the ending beat, with the morning star, it turns from gold to
            // its red.
            let ease = CubicEase(0.4, 0, 0.4, 1)
            gleam = CrownTrack(stops: [(0, 0, CubicEase(0.4, 0, 0.4, 1)), (0.34, 0.55, CubicEase(0.4, 0, 0.6, 1)), (1, 0, CubicEase.linear)]).value(crownRun(t, 60, 1300))
            let beat = ease.at(crownRun(t, scene.beat, fig.beatLen))
            new = switched ? beat : 0
            old = switched ? 1 - beat : 1
            if switched {
                ember = 0.85 * rise.at(crownRun(t, fig.turn + 900, 800)) * (1 - beat)
            }
        }
        return ZStack {
            oldCross.opacity(old)
            newCross.opacity(new)
            emberCross.opacity(ember)
            Crown(size: crown.width, ink: catchInk, shading: .color(catchInk)).opacity(gleam)
        }
        .frame(width: crown.width, height: crown.height)
        .position(x: crown.midX, y: crown.midY)
    }

    // MARK: The picture

    func paint(_ ctx: inout GraphicsContext, size: CGSize, t: Double) {
        let s = scene
        let whole = CGRect(origin: .zero, size: size)
        let topUp = s.topUp(t)

        // The page that is leaving, with the first act's light on it, let go of through the
        // front in the head and the spread over the page.
        ctx.drawLayer { old in
            if let oldShot {
                old.draw(oldShot, in: whole)
                if let groundShot {
                    // Each line's old ink goes at its moment, leaving its ground.
                    for (key, rect) in s.wordRects.merging(s.rowRects, uniquingKeysWith: { a, _ in a }) {
                        let gone = s.inkOut(key, t)
                        guard gone > 0 else { continue }
                        var g = old
                        g.clip(to: Path(rect))
                        g.opacity = gone
                        g.draw(groundShot, in: whole)
                    }
                }
            }
            paintBelts(&old, t)
            paintFirstLight(&old, t)
            if switched && t > fig.front.at {
                var front = old
                front.clip(to: s.vaultPath)
                front.blendMode = .destinationOut
                fillFront(&front, t)
            }
            if switched, t >= fig.spread.at, let mask {
                var page = old
                page.blendMode = .destinationOut
                page.opacity = s.glow(t)
                let sc = s.spreadScale(t)
                page.draw(mask, in: s.maskRect(sc + (1.3 * s.s1 - sc) * topUp))
            }
            // The frieze is drawn on its own over all of it (below).
            if switched, oldShot != nil, let frieze {
                var band = old
                band.blendMode = .destinationOut
                band.fill(Path(frieze), with: .color(.black))
            }
        }

        // What the light leaves on the new ground in the head, shown as the front passes.
        if switched { paintEmber(&ctx, t) }

        // The frieze is the horizon, and the one thing that holds: the old band stays whole
        // over the turning page until its own span, and gives way to the live one.
        if switched, let oldShot, let frieze {
            let left = 1 - s.friezeTurn(t)
            if left > 0 {
                var band = ctx
                band.clip(to: Path(frieze))
                band.opacity = left
                band.draw(oldShot, in: whole)
            }
        }

        paintStars(&ctx, t)
    }

    /**
     * What is multiplied over the page beneath (the stage's own layer, blended): it can only
     * shade, so the plaster keeps its texture and every line its ink. The veil over the turning
     * wall, in the page's growing shape and ahead of it; and at dawn the warm shade the head's
     * new plaster comes up under, where the front has let it in.
     */
    func paintShade(_ ctx: inout GraphicsContext, size: CGSize, t: Double) {
        guard switched else { return }
        let s = scene
        let whole = CGRect(origin: .zero, size: size)

        // The veil, over the turning wall (not the head, the frieze or the cross, which ride
        // above it).
        if t >= fig.spread.at, let mask {
            let strength = s.veilOpacity(t) * (1 - s.topUp(t))
            if strength > 0 {
                ctx.drawLayer { veil in
                    veil.opacity = strength
                    let rect = s.maskRect(s.veilScale(t))
                    veil.clip(to: Path(rect))
                    veil.clip(to: s.vaultPath, options: .inverse)
                    if let frieze { veil.clip(to: Path(frieze), options: .inverse) }
                    veil.clip(to: Path(crown), options: .inverse)
                    paintVeil(&veil, whole)
                    veil.blendMode = .destinationIn
                    veil.draw(mask, in: rect)
                }
            }
        }

        // Dawn's plaster does not arrive white: it comes up under a warm shade, peach low and
        // rose above, which lifts on a long tail.
        if !dusk {
            let shade = 1 - CubicEase(0.3, 0, 0.3, 1).at(crownRun(t, fig.turn + 250, 2300))
            if shade > 0, t > fig.front.at {
                let v = s.vault
                ctx.drawLayer { warm in
                    warm.clip(to: s.vaultPath)
                    warm.clip(to: s.frontEllipse(t))
                    warm.opacity = shade
                    warm.fill(Path(v), with: .linearGradient(
                        Gradient(stops: [.init(color: fig.skyPeach.color(), location: 0), .init(color: Color(rgb: 206, 158, 140, 1), location: 0.55), .init(color: fig.skyRoseLilac.color(), location: 1)]),
                        startPoint: CGPoint(x: v.midX, y: v.maxY), endPoint: CGPoint(x: v.midX, y: v.minY)
                    ))
                    // Only as far as the front has let the new ground in.
                    var reach = warm
                    reach.blendMode = .destinationIn
                    fillFront(&reach, t)
                }
            }
        }
    }

    /// The front's ellipse at `t`, its feather as style.css's stops, filled in `ctx`'s blend.
    private func fillFront(_ ctx: inout GraphicsContext, _ t: Double) {
        let R = scene.frontRadius(t)
        var c = ctx
        c.translateBy(x: scene.cx, y: scene.cy)
        c.scaleBy(x: scene.a, y: 1)
        let stops = CrownScene.frontStops.map { Gradient.Stop(color: .black.opacity($0.1), location: $0.0) }
        c.fill(Path(ellipseIn: CGRect(x: -R, y: -R, width: 2 * R, height: 2 * R)), with: .radialGradient(Gradient(stops: stops), center: .zero, startRadius: 0, endRadius: R))
    }

    /**
     * The belt. Ahead of the front, on the old sky it has not reached, rides a band of rose-lilac
     * (the Belt of Venus over the earth's shadow at dusk; the first rose over the night at dawn),
     * and at dawn a wider one of the night's teal lifted ahead of that: the front is three
     * pigments deep. Each is a wash through a ring on the front's own ellipse, grown with it.
     */
    private func paintBelts(_ ctx: inout GraphicsContext, _ t: Double) {
        guard switched else { return }
        var c = ctx
        c.clip(to: scene.vaultPath)
        if !dusk {
            belt(&c, t, scale: 1.5, pigment: fig.skyTeal, alpha: 0.6, ring: [(0, 0.52), (0.6, 0.66), (1, 0.76), (0.4, 0.9), (0, 1)])
        }
        belt(&c, t, scale: 1.22, pigment: fig.skyRoseLilac, alpha: dusk ? 0.62 : 0.5, ring: [(0, 0.6), (0.5, 0.69), (1, 0.79), (0.45, 0.9), (0, 1)])
    }

    /// One belt: `pigment` at `alpha` through a ring whose strength runs by `ring` (strength,
    /// place) out to the front's ellipse `scale` times its size.
    private func belt(_ ctx: inout GraphicsContext, _ t: Double, scale: Double, pigment: Pigment, alpha: Double, ring: [(Double, Double)]) {
        let R = scene.frontRadius(t) * scale
        guard R > 0 else { return }
        var c = ctx
        c.translateBy(x: scene.cx, y: scene.cy)
        c.scaleBy(x: scene.a, y: 1)
        let stops = ring.map { Gradient.Stop(color: pigment.color(alpha * $0.0), location: $0.1) }
        c.fill(Path(ellipseIn: CGRect(x: -R, y: -R, width: 2 * R, height: 2 * R)), with: .radialGradient(Gradient(stops: stops), center: .zero, startRadius: 0, endRadius: R))
    }

    /// Act one in the head, over the old ground: at dusk the raking light on the whole niche and
    /// the afterglow rising off the horizon, and a warmth on the plaster as the night comes down
    /// over it; at dawn the night lifting to teal, rose and peach from the horizon, and then a
    /// thin lime, low.
    private func paintFirstLight(_ ctx: inout GraphicsContext, _ t: Double) {
        let v = scene.vault
        let f = fig
        var c = ctx
        c.clip(to: scene.vaultPath)
        if dusk {
            if switched {
                let warm = CubicEase(0.3, 0, 0.5, 1).at(crownRun(t, f.turn + 100, 900))
                if warm > 0 {
                    var w = c
                    w.opacity = warm
                    w.fill(Path(v), with: .linearGradient(
                        Gradient(stops: [.init(color: f.skyRedOchre.color(0.2), location: 0), .init(color: f.skyOchre.color(0.15), location: 0.45), .init(color: f.skyOchre.color(0.1), location: 1)]),
                        startPoint: CGPoint(x: v.midX, y: v.maxY), endPoint: CGPoint(x: v.midX, y: v.minY)
                    ))
                }
            }
            // Golden hour on the plaster: an ochre laid thin over the whole niche, deeper toward
            // the horizon, before the dark comes down; the night covers it from above.
            let gone = 1 - crownRun(t, f.turn + 1750, 500)
            let raking = CubicEase(0.3, 0, 0.3, 1).at(crownRun(t, 120, 1300)) * gone
            if raking > 0 {
                var w = c
                w.opacity = raking
                w.fill(Path(v), with: .linearGradient(
                    Gradient(stops: [.init(color: f.skyOchre.color(0.4), location: 0), .init(color: f.skyOchre.color(0.28), location: 0.4), .init(color: f.skyOchre.color(0.16), location: 1)]),
                    startPoint: CGPoint(x: v.midX, y: v.maxY), endPoint: CGPoint(x: v.midX, y: v.minY)
                ))
            }
            let rise = CubicEase(0.3, 0, 0.3, 1).at(crownRun(t, 250, 1250))
            let shown = rise * gone
            guard shown > 0 else { return }
            // On a phone the afterglow is the cue that something has begun: denser.
            let phone = scene.screen.width <= 700
            let red: [Gradient.Stop] = phone
                ? [.init(color: f.skyRedOchre.color(0.5), location: 0), .init(color: f.skyRedOchre.color(0.22), location: 0.14), .init(color: f.skyRedOchre.color(0), location: 0.32)]
                : [.init(color: f.skyRedOchre.color(0.24), location: 0), .init(color: f.skyRedOchre.color(0.1), location: 0.14), .init(color: f.skyRedOchre.color(0), location: 0.3)]
            let ochre: [Gradient.Stop] = phone
                ? [.init(color: f.skyOchre.color(0.62), location: 0), .init(color: f.skyOchre.color(0.52), location: 0.3), .init(color: f.skyOchre.color(0.2), location: 0.68), .init(color: f.skyOchre.color(0), location: 1)]
                : [.init(color: f.skyOchre.color(0.3), location: 0), .init(color: f.skyOchre.color(0.26), location: 0.3), .init(color: f.skyOchre.color(0.1), location: 0.68), .init(color: f.skyOchre.color(0), location: 1)]
            let box = washBox(scene.washHeight, scale: 0.3 + 0.7 * rise)
            wash(&c, box: box, opacity: shown, layers: [red, ochre], radius: CGSize(width: 1.6 * box.width, height: box.height), mask: [(0, 1), (0.34, 0.85), (0.66, 0.4), (1, 0)], pigmented: true)
        } else {
            // The night first lifts to its teal from the horizon, then a rose-lilac, then a peach
            // of red ochre and lime: one wash, three pigments deep, rising, the teal leading. The
            // lime comes last, thin and only low. Both are gone once the plaster has come.
            let gone = 1 - crownRun(t, f.turn + 1100, 700)
            let rx = scene.washRadius
            let mask: [(Double, Double)] = [(0, 1), (0.3, 0.9), (0.64, 0.45), (1, 0)]
            let first = CubicEase(0.3, 0, 0.3, 1).at(crownRun(t, 150, 1100))
            if first * gone > 0 {
                let box = washBox(1.5 * scene.washHeight, scale: 0.3 + 0.7 * first)
                let dawn: [Gradient.Stop] = [
                    .init(color: f.skyPeach.color(0.92), location: 0), .init(color: Color(rgb: 206, 150, 124, 0.8), location: 0.16),
                    .init(color: f.skyRoseLilac.color(0.5), location: 0.38), .init(color: f.skyTeal.color(0.5), location: 0.62),
                    .init(color: f.skyTeal.color(0.22), location: 0.82), .init(color: f.skyTeal.color(0), location: 1),
                ]
                wash(&c, box: box, opacity: first * gone, layers: [dawn], radius: CGSize(width: 1.3 * rx, height: box.height), mask: mask, pigmented: false)
            }
            let lime = CubicEase(0.3, 0, 0.3, 1).at(crownRun(t, 700, 900))
            if lime * gone > 0 {
                let box = washBox(0.5 * scene.washHeight, scale: 0.3 + 0.7 * lime)
                let stops: [Gradient.Stop] = [
                    .init(color: f.skyLime.color(0.45), location: 0), .init(color: f.skyLime.color(0.3), location: 0.4),
                    .init(color: f.skyLime.color(0), location: 1),
                ]
                wash(&c, box: box, opacity: lime * gone, layers: [stops], radius: CGSize(width: rx, height: box.height), mask: mask, pigmented: false)
            }
        }
    }

    /// A light on the horizon's box, `height` tall when risen, grown from the horizon to `scale`
    /// of it.
    private func washBox(_ height: Double, scale: Double) -> CGRect {
        let v = scene.vault
        let h = height * scale
        return CGRect(x: v.minX, y: v.maxY - h, width: v.width, height: h)
    }

    /// A wash in `box`: its gradients run up from the horizon, laid through an ellipse about the
    /// horizon's middle of `radius` whose strength falls by `mask`, and (dusk) through the pigment.
    private func wash(_ ctx: inout GraphicsContext, box: CGRect, opacity: Double, layers: [[Gradient.Stop]], radius: CGSize, mask: [(Double, Double)], pigmented: Bool) {
        guard box.height > 0.5, radius.width > 0 else { return }
        let centre = CGPoint(x: box.midX, y: box.maxY)
        let oval = Path(ellipseIn: CGRect(x: centre.x - radius.width, y: centre.y - radius.height, width: 2 * radius.width, height: 2 * radius.height))
        ctx.drawLayer { w in
            w.opacity = opacity
            w.clip(to: Path(box))
            w.clip(to: oval)
            for stops in layers {
                w.fill(Path(box), with: .linearGradient(Gradient(stops: stops), startPoint: CGPoint(x: box.midX, y: box.maxY), endPoint: CGPoint(x: box.midX, y: box.minY)))
            }
            var fall = w
            fall.blendMode = .destinationIn
            fall.translateBy(x: centre.x, y: centre.y)
            fall.scaleBy(x: 1, y: radius.height / radius.width)
            let r = radius.width
            fall.fill(Path(ellipseIn: CGRect(x: -r, y: -r, width: 2 * r, height: 2 * r)), with: .radialGradient(
                Gradient(stops: mask.map { Gradient.Stop(color: .black.opacity($0.1), location: $0.0) }), center: .zero, startRadius: 0, endRadius: r
            ))
            if pigmented, let pigment {
                var grain = w
                grain.blendMode = .destinationIn
                grain.fill(Path(box), with: .tiledImage(Image(uiImage: pigment), scale: 128 / pigment.size.width))
            }
        }
    }

    /// What the light leaves on the new ground in the head, a low mound on the horizon that
    /// sinks and goes out: an ember on the night sky at dusk, a warmth on the plaster at dawn.
    private func paintEmber(_ ctx: inout GraphicsContext, _ t: Double) {
        let f = fig
        let v = scene.vault
        let sink = dusk ? CubicEase(0.4, 0, 0.3, 1).at(crownRun(t, f.turn + 1900, 1500)) : CubicEase(0.4, 0, 0.3, 1).at(crownRun(t, f.turn + 900, 1600))
        guard sink < 1, t > f.front.at else { return }
        let h = 0.22 * v.height * (1 - 0.6 * sink)
        let box = CGRect(x: v.minX, y: v.maxY - h, width: v.width, height: h)
        let stops: [Gradient.Stop] = dusk
            ? [.init(color: f.skyEmber.color(0.64), location: 0), .init(color: f.skyEmber.color(0.32), location: 0.45), .init(color: f.skyEmber.color(0), location: 1)]
            : [.init(color: f.skyOchre.color(0.2), location: 0), .init(color: f.skyOchre.color(0.08), location: 0.5), .init(color: f.skyOchre.color(0), location: 1)]
        // Shown as far as the front has turned the head over the horizon's middle.
        let shown = (1 - sink) * scene.frontAlpha(CGPoint(x: box.midX, y: box.midY), t)
        guard shown > 0.001 else { return }
        var e = ctx
        e.clip(to: scene.vaultPath)
        wash(&e, box: box, opacity: shown, layers: [stops], radius: CGSize(width: 0.58 * box.width, height: box.height), mask: [(0, 1), (0.26, 0.85), (0.52, 0.4), (0.7, 0.12), (0.84, 0)], pigmented: false)
    }

    /// The veil's ink, thinning only a little outward from the springing's middle (style.css's
    /// `.crown-veil`).
    private func paintVeil(_ ctx: inout GraphicsContext, _ whole: CGRect) {
        let ink = fig.veilInk
        let shares: [Double] = [1, 0.94, 0.82, fig.veilFar]
        let stops = zip(shares, [0, 0.28, 0.58, 1.0]).map { Gradient.Stop(color: ink.color($0.0), location: $0.1) }
        let rx = 1.5 * scene.width
        let ry = 1.9 * scene.width
        let oval = Path(ellipseIn: CGRect(x: scene.ax - rx, y: scene.ys - ry, width: 2 * rx, height: 2 * ry))
        // Beyond the ellipse the last stop holds.
        var beyond = Path(whole)
        beyond.addPath(oval)
        ctx.fill(beyond, with: .color(ink.color(shares[3])), style: FillStyle(eoFill: true))
        var c = ctx
        c.translateBy(x: scene.ax, y: scene.ys)
        c.scaleBy(x: 1, y: ry / rx)
        c.fill(Path(ellipseIn: CGRect(x: -rx, y: -rx, width: 2 * rx, height: 2 * rx)), with: .radialGradient(Gradient(stops: stops), center: .zero, startRadius: 0, endRadius: rx))
    }

    /// The stars, by magnitude, over the head's new ground both ways. Dusk: unlit until each
    /// one's turn; they swell a little past their size as they kindle and settle, and the
    /// leaf's light carries them. Dawn: out faintest first, and the morning star, its gold
    /// deepened to stand on the light wall, at the ending beat.
    private func paintStars(_ ctx: inout GraphicsContext, _ t: Double) {
        guard !starPaths.isEmpty else { return }
        var sky = ctx
        sky.clip(to: scene.vaultPath)
        let rise = CubicEase(0.3, 0.2, 0.3, 1)
        let kindle = CrownTrack(stops: [(0, 0, rise), (0.55, 1, rise), (1, 1, CubicEase.linear)])
        let swell = CrownTrack(stops: [(0, 0.86, rise), (0.55, fig.starOver, rise), (1, 1, CubicEase.linear)])
        for (i, path) in starPaths.enumerated() {
            let turn = scene.starTurns[i]
            let centre = scene.stars[i].center
            let morning = !dusk && i == scene.evening
            var shown: Double
            var scale = 1.0
            if dusk {
                guard switched else { continue }
                let u = crownRun(t, turn, fig.starKindle)
                shown = kindle.value(u)
                scale = swell.value(u)
            } else if morning {
                shown = 1 - CubicEase(0.4, 0, 0.4, 1).at(crownRun(t, turn, fig.beatLen))
            } else {
                shown = 1 - CubicEase(0.4, 0, 0.3, 1).at(crownRun(t, turn, fig.starSnuff))
            }
            guard shown > 0.001 else { continue }
            var star = sky
            star.opacity = lit * starLeaf[i] * shown
            if morning, switched, let at = scene.morningAt {
                let deep = CubicEase(0.4, 0, 0.4, 1).at(crownRun(t, at, 700))
                if deep > 0 {
                    // style.css's brightness(0.7) saturate(1.8), reached by degrees.
                    star.addFilter(.colorMultiply(Color(white: 1 - 0.3 * deep)))
                    star.addFilter(.saturation(1 + 0.8 * deep))
                }
            }
            if scale != 1 {
                star.translateBy(x: centre.x, y: centre.y)
                star.scaleBy(x: scale, y: scale)
                star.translateBy(x: -centre.x, y: -centre.y)
            }
            star.fill(path, with: starLight)
        }
    }
}

/// The stage over the window: the play's picture, which takes no touches.
struct CrownStageView: View {
    @ObservedObject var player: CrownPlayer

    var body: some View {
        ZStack(alignment: .topLeading) {
            if let run = player.run {
                TimelineView(.animation) { _ in
                    let t = run.clock()
                    ZStack(alignment: .topLeading) {
                        Canvas { ctx, size in run.paint(&ctx, size: size, t: t) }
                        // Multiplied over everything under it, the live page and the leaving
                        // one's picture alike: it can only shade.
                        Canvas { ctx, size in run.paintShade(&ctx, size: size, t: t) }
                            .blendMode(.multiply)
                        run.crosses(t)
                    }
                }
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .ignoresSafeArea()
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}
