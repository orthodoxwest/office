import SwiftUI

/**
 * The app's motion, `render_blocks::Tempo`: five named motions and three curves, each way of a
 * motion a [Beat]. The web times by the same table (style.css's `--tempo-*` and `--curve-*`) and
 * Android by Tempo.kt; a test in render-blocks reads this file and fails when a figure here drifts
 * from the table, so change one there first.
 */
enum Tempo {
    /// A press: answered at once, let go over a fifth of a second.
    static let touchIn = Beat(ms: 0, delay: 0, curve: .arrive)
    static let touchOut = Beat(ms: 200, delay: 0, curve: .arrive)
    /// How far bare words dim under the finger (`Quiet`), and the hour that is now's words when
    /// pressed: the web's `--tempo-touch-quiet`.
    static let touchQuiet = 0.7
    /// What a press moves rather than lights (the ordo day's rail running the row's height)
    /// reaches its place over this; it goes back over `touchOut`.
    static let touchReach = Beat(ms: 120, delay: 0, curve: .arrive)
    /// The consecration cross under the last Amen taking its gold, once the hour is marked prayed
    /// and the cross stands wholly on screen: evenly, by opacity alone (`presentation::EndLeaf`).
    static let gild = Beat(ms: 2000, delay: 0, curve: .standard)
    /// A small panel settling under the beam; it leaves faster than it came.
    static let settleIn = Beat(ms: 160, delay: 0, curve: .arrive)
    static let settleOut = Beat(ms: 120, delay: 0, curve: .leave)
    /// A disclosure unfolding, and folding.
    static let unfoldIn = Beat(ms: 260, delay: 0, curve: .standard)
    static let unfoldOut = Beat(ms: 200, delay: 0, curve: .standard)
    /// Page to page: the old page goes, then the new one comes; never two at once.
    static let turnIn = Beat(ms: 210, delay: 90, curve: .arrive)
    static let turnOut = Beat(ms: 90, delay: 0, curve: .leave)
    /// The room changing as a whole: theme and text size.
    static let dimIn = Beat(ms: 320, delay: 0, curve: .standard)
    static let dimOut = Beat(ms: 320, delay: 0, curve: .standard)
}

/// One way of a motion: its length and the wait before it, in milliseconds, on one of the curves.
struct Beat {
    let ms: Int
    let delay: Int
    let curve: TempoCurve

    var seconds: Double { Double(ms) / 1000 }

    /// The beat as an animation, its wait included.
    var animation: Animation {
        let (x1, y1, x2, y2) = curve.points
        return Animation.timingCurve(x1, y1, x2, y2, duration: seconds).delay(Double(delay) / 1000)
    }
}

/// The three curves, as CSS cubic-beziers: no motion reaches for SwiftUI's own `easeOut`.
enum TempoCurve {
    /// Something that changes in place.
    case standard
    /// Something coming into view, or let go.
    case arrive
    /// Something going away.
    case leave

    var points: (Double, Double, Double, Double) {
        switch self {
        case .standard: return (0.4, 0, 0.2, 1)
        case .arrive: return (0, 0, 0.2, 1)
        case .leave: return (0.4, 0, 1, 1)
        }
    }
}
