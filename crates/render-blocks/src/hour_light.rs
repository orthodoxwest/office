//! The light moving with the hours on home. When the hour that is now changes while home is in
//! front (never on arrival, never when a page or app is corrected on its return), the pool of light
//! under the old hour dies as one gathers under the new, the rubric ink follows, the gloss changes,
//! and the invitation's words ("Pray Sext", "Pray None") change in place. Every front plays the one
//! pass on one clock: the web as Web Animations (app.js's `HOUR_LIGHT`, which a test holds to these
//! figures), the apps by asking [`light_at`] for each frame.
//!
//! A word is never slid. A name whose seat changes (on a phone the hour that is now stands higher,
//! to make room for its gloss; in the wide niche it is also larger) goes out where it stood and
//! comes back where it belongs, behind the invitation's own 120ms out and 180ms in, at the moment
//! the invitation's words change: the layout of the new hour takes over at [`HourLight::SEAT`],
//! when they are out.
//!
//! These are motion figures, like the tempo of the other passes the fronts share; they can join a
//! common set of timings when one exists.

/// The pass's figures, in milliseconds from its start.
pub struct HourLight;

impl HourLight {
    /// The whole pass: the old pool dies and the new gathers over it.
    pub const TOTAL: u32 = 1400;
    /// The old gloss is gone by here, before its name changes seat.
    pub const GLOSS_OUT: u32 = 500;
    /// The names' ink, rubric to plain and plain to rubric, from here …
    pub const INK_FROM: u32 = 300;
    /// … for this long.
    pub const INK_FOR: u32 = 600;
    /// The words that change seat or say (the names, the invitation) start out here …
    pub const WORDS_OUT: u32 = 500;
    /// … are out here, where the new hour's layout takes over …
    pub const SEAT: u32 = 620;
    /// … and are back here.
    pub const WORDS_BACK: u32 = 800;
    /// The new gloss comes in from here to the end, after its name is seated.
    pub const GLOSS_IN: u32 = 800;

    /// The pool, the ink and the glosses: a gentle ease in and out.
    pub const EASE: Curve = Curve(0.45, 0.0, 0.55, 1.0);
    /// The words going out (CSS's `ease-in`).
    pub const OUT: Curve = Curve(0.42, 0.0, 1.0, 1.0);
    /// The words coming back (CSS's `ease-out`).
    pub const BACK: Curve = Curve(0.0, 0.0, 0.58, 1.0);
}

/// A cubic Bézier timing curve through (0, 0) and (1, 1), as CSS's `cubic-bezier(x1, y1, x2, y2)`.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Curve(pub f32, pub f32, pub f32, pub f32);

impl Curve {
    /// The curve's progress at `t` of its time (clamped to 0–1).
    pub fn at(self, t: f32) -> f32 {
        // Exact at the ends, so a pass begins and ends on the page at rest.
        if t <= 0.0 {
            return 0.0;
        }
        if t >= 1.0 {
            return 1.0;
        }
        let Curve(x1, y1, x2, y2) = self;
        let bez = |a: f32, b: f32, s: f32| {
            let u = 1.0 - s;
            3.0 * u * u * s * a + 3.0 * u * s * s * b + s * s * s
        };
        // Solve x(s) = t by bisection: monotone in s for 0 ≤ x1, x2 ≤ 1.
        let (mut lo, mut hi) = (0.0f32, 1.0f32);
        for _ in 0..32 {
            let mid = (lo + hi) / 2.0;
            if bez(x1, x2, mid) < t {
                lo = mid;
            } else {
                hi = mid;
            }
        }
        bez(y1, y2, (lo + hi) / 2.0)
    }

    /// As CSS writes it: `cubic-bezier(0.45, 0, 0.55, 1)`.
    pub fn css(self) -> String {
        let Curve(a, b, c, d) = self;
        format!("cubic-bezier({a}, {b}, {c}, {d})")
    }
}

/// The pass at one instant. Each figure is an opacity or a progress from 0 to 1.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct LightFrame {
    /// The pool under the hour the light leaves.
    pub leaving_pool: f32,
    /// The pool under the hour the light comes to.
    pub coming_pool: f32,
    /// How far the ink has turned: the leaving name from the rubric toward its plain ink, the
    /// coming name from its plain ink toward the rubric (and in the wide niche, where every hour
    /// keeps its gloss, the glosses' inks with them).
    pub ink: f32,
    /// The old gloss, where it is shown only under the hour that is now.
    pub leaving_gloss: f32,
    /// The new gloss, likewise.
    pub coming_gloss: f32,
    /// The words that change seat or say: a name re-seated, the invitation's words.
    pub words: f32,
    /// Whether the new hour's layout has taken over (its seat, its gloss, the invitation's new
    /// words).
    pub seated: bool,
    /// Whether the pass is over: the page is drawn at rest, exactly as it would be fresh.
    pub done: bool,
}

/// The pass `ms` milliseconds after it began.
pub fn light_at(ms: f32) -> LightFrame {
    let h = |v: u32| v as f32;
    let span = |from: u32, len: u32, curve: Curve| curve.at((ms - h(from)) / h(len));
    let pool = span(0, HourLight::TOTAL, HourLight::EASE);
    let words = if ms < h(HourLight::WORDS_OUT) {
        1.0
    } else if ms < h(HourLight::SEAT) {
        1.0 - span(HourLight::WORDS_OUT, HourLight::SEAT - HourLight::WORDS_OUT, HourLight::OUT)
    } else {
        span(HourLight::SEAT, HourLight::WORDS_BACK - HourLight::SEAT, HourLight::BACK)
    };
    let done = ms >= h(HourLight::TOTAL);
    LightFrame {
        leaving_pool: 1.0 - pool,
        coming_pool: pool,
        ink: span(HourLight::INK_FROM, HourLight::INK_FOR, HourLight::EASE),
        leaving_gloss: 1.0 - span(0, HourLight::GLOSS_OUT, HourLight::EASE),
        coming_gloss: span(HourLight::GLOSS_IN, HourLight::TOTAL - HourLight::GLOSS_IN, HourLight::EASE),
        words: if done { 1.0 } else { words },
        seated: ms >= h(HourLight::SEAT),
        done,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

    #[test]
    fn the_pass_begins_as_the_page_was_and_ends_as_a_fresh_one() {
        let start = light_at(0.0);
        assert_eq!((start.leaving_pool, start.coming_pool, start.ink, start.words), (1.0, 0.0, 0.0, 1.0));
        assert_eq!((start.leaving_gloss, start.coming_gloss), (1.0, 0.0));
        assert!(!start.seated && !start.done);
        let end = light_at(HourLight::TOTAL as f32);
        assert_eq!(
            end,
            LightFrame {
                leaving_pool: 0.0,
                coming_pool: 1.0,
                ink: 1.0,
                leaving_gloss: 0.0,
                coming_gloss: 1.0,
                words: 1.0,
                seated: true,
                done: true
            }
        );
        assert_eq!(light_at(5000.0), end);
    }

    // The layout changes only while the words that move are out of sight, the old gloss gone and
    // the new not yet come.
    #[test]
    fn the_seat_changes_while_the_moving_words_are_out() {
        let seat = HourLight::SEAT as f32;
        let before = light_at(seat - 0.01);
        let after = light_at(seat);
        assert!(!before.seated && after.seated);
        assert!(before.words < 0.01 && after.words < 0.01, "{} {}", before.words, after.words);
        assert_eq!((before.leaving_gloss, after.coming_gloss), (0.0, 0.0));
        assert_eq!(light_at(HourLight::WORDS_OUT as f32).words, 1.0);
        assert!((light_at(HourLight::WORDS_BACK as f32).words - 1.0).abs() < 1e-4);
    }

    // The pools are equal at the middle of the pass (the curve is symmetric), and the old still
    // holds most of the light at a quarter.
    #[test]
    fn the_pools_cross_at_the_middle() {
        let mid = light_at(HourLight::TOTAL as f32 / 2.0);
        assert!((mid.leaving_pool - 0.5).abs() < 1e-3 && (mid.coming_pool - 0.5).abs() < 1e-3);
        let quarter = light_at(350.0);
        assert!(quarter.leaving_pool > 0.85 && quarter.coming_pool < 0.15, "{quarter:?}");
    }

    #[test]
    fn the_curves_are_css_s() {
        assert_eq!(HourLight::EASE.css(), "cubic-bezier(0.45, 0, 0.55, 1)");
        assert_eq!(HourLight::OUT.css(), "cubic-bezier(0.42, 0, 1, 1)");
        assert_eq!(HourLight::BACK.css(), "cubic-bezier(0, 0, 0.58, 1)");
        for c in [HourLight::EASE, HourLight::OUT, HourLight::BACK] {
            assert_eq!((c.at(0.0), c.at(1.0)), (0.0, 1.0));
            let mut last = 0.0;
            for i in 1..=100 {
                let v = c.at(i as f32 / 100.0);
                assert!(v >= last, "{c:?} falls at {i}");
                last = v;
            }
        }
        // CSS's ease-in at its middle (a known figure).
        assert!((HourLight::OUT.at(0.5) - 0.3153).abs() < 1e-3, "{}", HourLight::OUT.at(0.5));
    }

    /// The web plays the pass from app.js's `HOUR_LIGHT`, which carries these figures and curves.
    #[test]
    fn the_web_plays_the_same_pass() {
        let js =
            std::fs::read_to_string(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../apps/office-web/static/app.js")).expect("app.js");
        let expected = format!(
            "var HOUR_LIGHT = {{ total: {}, glossOut: {}, inkFrom: {}, inkFor: {}, wordsOut: {}, seat: {}, wordsBack: {}, glossIn: {}, ease: \"{}\", out: \"{}\", back: \"{}\" }};",
            HourLight::TOTAL,
            HourLight::GLOSS_OUT,
            HourLight::INK_FROM,
            HourLight::INK_FOR,
            HourLight::WORDS_OUT,
            HourLight::SEAT,
            HourLight::WORDS_BACK,
            HourLight::GLOSS_IN,
            HourLight::EASE.css(),
            HourLight::OUT.css(),
            HourLight::BACK.css(),
        );
        assert!(js.contains(&expected), "app.js should declare\n{expected}");
    }
}
