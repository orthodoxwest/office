//! The app's motion, as [`crate::Space`] is its rhythm: five named motions and three curves,
//! which every front times by. The web reads them as `--tempo-*` and `--curve-*` custom
//! properties (style.css), the apps as constants (`Tempo.swift`, `Tempo.kt`); the tests below
//! read all three and fail when a front drifts from this table. The apps keep their own literals
//! rather than reading the table through the bindings, so a designer's tweak is one line on each
//! front and this table, and no Rust build stands between them.
//!
//! A motion's way in and way out are each a [`Beat`]: how long, after how long, on which curve.
//! A page never shows two pages at once (the turn's way in waits for its way out), a press is
//! seen at once and lets go slowly, and a panel leaves faster than it came.
//!
//! Under reduced motion the web cuts every one (style.css's `prefers-reduced-motion` block); the
//! apps keep the touch and the dim, which do not move anything, and cut the rest.

/// One of the three curves every motion takes, as a CSS `cubic-bezier`'s four figures. No front
/// reaches for its platform's own `ease`: CSS's `ease-out` and SwiftUI's `easeOut` are cousins of
/// [`Curve::Arrive`], not twins.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Curve {
    /// Something that changes in place: it starts briskly and settles (Material's standard,
    /// Compose's `FastOutSlowInEasing`).
    Standard,
    /// Something coming into view or let go: no start, a long settle (Material's decelerate,
    /// `LinearOutSlowInEasing`).
    Arrive,
    /// Something going away: a slow start, then gone (Material's accelerate,
    /// `FastOutLinearInEasing`).
    Leave,
}

impl Curve {
    pub const ALL: [Curve; 3] = [Curve::Standard, Curve::Arrive, Curve::Leave];

    /// The curve's control points, x1, y1, x2, y2.
    pub fn points(self) -> [f32; 4] {
        match self {
            Curve::Standard => [0.4, 0.0, 0.2, 1.0],
            Curve::Arrive => [0.0, 0.0, 0.2, 1.0],
            Curve::Leave => [0.4, 0.0, 1.0, 1.0],
        }
    }

    /// The curve's name, as the web's `--curve-*` property names it.
    pub fn name(self) -> &'static str {
        match self {
            Curve::Standard => "standard",
            Curve::Arrive => "arrive",
            Curve::Leave => "leave",
        }
    }
}

/// One way of a motion: its length and the wait before it, in milliseconds, and its curve.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub struct Beat {
    pub ms: u32,
    pub delay: u32,
    pub curve: Curve,
}

const fn beat(ms: u32, delay: u32, curve: Curve) -> Beat {
    Beat { ms, delay, curve }
}

/// The five things that move.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Tempo {
    /// A press: the finger is answered at once, and the answer is let go over a fifth of a
    /// second, so even the briefest tap is seen to land.
    Touch,
    /// A small panel (the menu, Settings) settling under the beam it hangs from; it leaves
    /// faster than it came. The note a glossed word or an ordo mark opens comes and goes on it
    /// too, by fading alone.
    Settle,
    /// A disclosure unfolding (its way in) and folding (its way out), and home's turn to the
    /// calendar.
    Unfold,
    /// Page to page: the old page goes, then the new one comes, so two pages of prayer are never
    /// seen over each other. The way in waits for the way out.
    Turn,
    /// The room changing as a whole: theme and text size, one beat.
    Dim,
}

impl Tempo {
    pub const ALL: [Tempo; 5] = [Tempo::Touch, Tempo::Settle, Tempo::Unfold, Tempo::Turn, Tempo::Dim];

    /// How far bare words (a link, a disclosure's summary, a step of the picker) dim under the
    /// finger, as an opacity: the touch's answer where there is no pool of light to lie under.
    /// The hour that is now withdraws its words as far when pressed.
    pub const TOUCH_QUIET: f32 = 0.7;

    /// What a press moves rather than lights (the ordo day's rail running the row's full height)
    /// reaches its place over this, so the stretch is seen; it goes back over the touch's way out.
    pub const TOUCH_REACH: Beat = beat(120, 0, Curve::Arrive);

    /// The motion's name, as the web's `--tempo-*` properties name it.
    pub fn name(self) -> &'static str {
        match self {
            Tempo::Touch => "touch",
            Tempo::Settle => "settle",
            Tempo::Unfold => "unfold",
            Tempo::Turn => "turn",
            Tempo::Dim => "dim",
        }
    }

    /// The way in: a press, a panel opening, an unfold, the new page, the new room.
    pub fn enter(self) -> Beat {
        match self {
            Tempo::Touch => beat(0, 0, Curve::Arrive),
            Tempo::Settle => beat(160, 0, Curve::Arrive),
            Tempo::Unfold => beat(260, 0, Curve::Standard),
            Tempo::Turn => beat(210, 90, Curve::Arrive),
            Tempo::Dim => beat(320, 0, Curve::Standard),
        }
    }

    /// The way out: a release, a panel closing, a fold, the old page.
    pub fn leave(self) -> Beat {
        match self {
            Tempo::Touch => beat(200, 0, Curve::Arrive),
            Tempo::Settle => beat(120, 0, Curve::Leave),
            Tempo::Unfold => beat(200, 0, Curve::Standard),
            Tempo::Turn => beat(90, 0, Curve::Leave),
            Tempo::Dim => beat(320, 0, Curve::Standard),
        }
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;

    fn read(path: &str) -> String {
        std::fs::read_to_string(Path::new(env!("CARGO_MANIFEST_DIR")).join("../..").join(path)).unwrap_or_else(|e| panic!("{path}: {e}"))
    }

    /// Each way of each motion, by the names the fronts give it: `touch`/`in`, `touch`/`out`.
    fn beats() -> impl Iterator<Item = (Tempo, &'static str, Beat)> {
        Tempo::ALL.into_iter().flat_map(|t| [(t, "in", t.enter()), (t, "out", t.leave())])
    }

    fn camel(t: Tempo, way: &str) -> String {
        format!("{}{}", t.name(), if way == "in" { "In" } else { "Out" })
    }

    /// The turn never shows two pages: its way in starts when its way out has ended.
    #[test]
    fn a_turn_waits_for_the_page_to_go() {
        assert_eq!(Tempo::Turn.enter().delay, Tempo::Turn.leave().ms);
        assert_eq!(Tempo::Touch.enter().ms, 0, "a press is seen at once");
        assert!(Tempo::Settle.leave().ms < Tempo::Settle.enter().ms, "a panel leaves faster than it came");
    }

    /// The web times by the same table: `--tempo-turn-in: 210ms`, `--tempo-turn-in-delay: 90ms`,
    /// `--tempo-turn-in-curve: var(--curve-arrive)`, and each `--curve-*` is its cubic-bezier.
    #[test]
    fn the_web_keeps_tempo() {
        let css = read("apps/office-web/static/style.css");
        let value = |property: &str| -> String {
            let decl = format!("{property}:");
            let at = css.find(&decl).unwrap_or_else(|| panic!("{property} in style.css"));
            css[at + decl.len()..].split(';').next().unwrap().trim().to_string()
        };
        for c in Curve::ALL {
            let [x1, y1, x2, y2] = c.points();
            assert_eq!(value(&format!("--curve-{}", c.name())), format!("cubic-bezier({x1}, {y1}, {x2}, {y2})"), "--curve-{}", c.name());
        }
        for (t, way, b) in beats() {
            let base = format!("--tempo-{}-{way}", t.name());
            assert_eq!(value(&base), format!("{}ms", b.ms), "{base}");
            assert_eq!(value(&format!("{base}-delay")), format!("{}ms", b.delay), "{base}-delay");
            assert_eq!(value(&format!("{base}-curve")), format!("var(--curve-{})", b.curve.name()), "{base}-curve");
        }
        assert_eq!(value("--tempo-touch-quiet"), Tempo::TOUCH_QUIET.to_string());
        let reach = Tempo::TOUCH_REACH;
        assert_eq!(value("--tempo-touch-reach"), format!("{}ms", reach.ms));
        assert_eq!(value("--tempo-touch-reach-delay"), format!("{}ms", reach.delay));
        assert_eq!(value("--tempo-touch-reach-curve"), format!("var(--curve-{})", reach.curve.name()));
    }

    /// The iOS app's table (`Tempo.swift`): `static let turnIn = Beat(ms: 210, delay: 90, curve: .arrive)`
    /// and `case .arrive: return (0, 0, 0.2, 1)`.
    #[test]
    fn the_ios_app_keeps_tempo() {
        let swift = read("apps/ios/Office/Tempo.swift");
        for c in Curve::ALL {
            let [x1, y1, x2, y2] = c.points();
            let line = format!("case .{}: return ({x1}, {y1}, {x2}, {y2})", c.name());
            assert!(swift.contains(&line), "Tempo.swift lacks `{line}`");
        }
        for (t, way, b) in beats() {
            let line = format!("static let {} = Beat(ms: {}, delay: {}, curve: .{})", camel(t, way), b.ms, b.delay, b.curve.name());
            assert!(swift.contains(&line), "Tempo.swift lacks `{line}`");
        }
        let r = Tempo::TOUCH_REACH;
        for line in [
            format!("static let touchQuiet = {}", Tempo::TOUCH_QUIET),
            format!("static let touchReach = Beat(ms: {}, delay: {}, curve: .{})", r.ms, r.delay, r.curve.name()),
        ] {
            assert!(swift.contains(&line), "Tempo.swift lacks `{line}`");
        }
    }

    /// The Android app's table (`Tempo.kt`): `val turnIn = Beat(ms = 210, delay = 90, curve = Curve.Arrive)`
    /// and `Arrive(0f, 0f, 0.2f, 1f)`.
    #[test]
    fn the_android_app_keeps_tempo() {
        let kt = read("apps/android/app/src/main/kotlin/org/orthodoxwest/office/Tempo.kt");
        let title = |s: &str| s[..1].to_uppercase() + &s[1..];
        for c in Curve::ALL {
            let [x1, y1, x2, y2] = c.points();
            let line = format!("{}({x1}f, {y1}f, {x2}f, {y2}f)", title(c.name()));
            assert!(kt.contains(&line), "Tempo.kt lacks `{line}`");
        }
        for (t, way, b) in beats() {
            let line = format!("val {} = Beat(ms = {}, delay = {}, curve = Curve.{})", camel(t, way), b.ms, b.delay, title(b.curve.name()));
            assert!(kt.contains(&line), "Tempo.kt lacks `{line}`");
        }
        let r = Tempo::TOUCH_REACH;
        for line in [
            format!("const val TOUCH_QUIET = {}f", Tempo::TOUCH_QUIET),
            format!("val touchReach = Beat(ms = {}, delay = {}, curve = Curve.{})", r.ms, r.delay, title(r.curve.name())),
        ] {
            assert!(kt.contains(&line), "Tempo.kt lacks `{line}`");
        }
    }
}
