//! How a dropped initial fits the text beside it: the web's optical profile for every capital
//! of the bundled Junicode (`style.css`, `[data-initial]`), so the native apps fit each
//! painted letter's contour as the web does rather than setting every capital as a box.
//!
//! As Chrome sets `initial-letter`, the fit is to the letter's ink, not its advance: the ink's
//! left edge stands at the margin (moved by `hang`), the lines beside it start `gap` past its
//! right edge, and its top meets the first line's cap height, so a capital that rises above
//! cap height (A, T) stands that much lower.

/// Junicode's cap height, in em (`OS/2.sCapHeight` 663 of the app's 1034-unit em).
pub const CAP_HEIGHT: f32 = 0.641;

/// The size of a two-line initial, in ems of the text beside it, as CSS `initial-letter: 2`
/// sizes it: its cap height spans one line pitch and the text's own cap height, so its top
/// meets the first line's capitals and its foot stands on the second line's baseline.
pub fn initial_size(line_height_em: f32) -> f32 {
    1.0 + line_height_em / CAP_HEIGHT
}

/// The em of the profiles' `gap`, `hang` and `depth`, in ems of the text: the web's initial is
/// declared at this size, and `initial-letter` draws it larger without changing its em.
pub const PROFILE_EM: f32 = 3.05;

/// The size of a raised initial, standing on its line's baseline, in ems of the text (`.initial-raised`).
pub const RAISED_SIZE: f32 = 1.65;

/// The space after a raised initial, in its own em.
pub const RAISED_GAP: f32 = 0.035;

/// One capital's fit. `gap`, `hang` and `depth` are in the declared initial's em ([`PROFILE_EM`]);
/// `tuck` and `raised_tuck` in the text's.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct InitialFit {
    /// Space between the letter and the lines beside it; negative lets them run in under an
    /// arm or beside a narrow stem (F, P, T, V, W), or over Q's long tail.
    pub gap: f32,
    /// How far the first line's opening word moves from that edge: in under a high arm (T, F),
    /// or out past a wide top (A, L, R).
    pub tuck: f32,
    /// How far the letter stands out into the margin: round and pointed capitals overshoot.
    pub hang: f32,
    /// Extra clearance below the second line for a descending tail: the next line too stands
    /// beside the letter. No capital of the current face needs it.
    pub depth: f32,
    /// How far the opening word moves beside a raised initial, which shares its line: in under
    /// a high arm (T, V, W, Y).
    pub raised_tuck: f32,
}

const PLAIN: InitialFit = InitialFit { gap: 0.06, tuck: 0.0, hang: 0.0, depth: 0.0, raised_tuck: 0.0 };

/// The fit for an initial; a letter outside the profiled capitals takes the plain fit.
pub fn initial_fit(letter: char) -> InitialFit {
    let fit = |gap, tuck, hang| InitialFit { gap, tuck, hang, ..PLAIN };
    let high = |fit: InitialFit| InitialFit { raised_tuck: -0.04, ..fit };
    match letter {
        'A' => fit(0.065, -0.7, -0.02),
        'B' => fit(0.06, -0.15, 0.0),
        'C' => fit(0.07, -0.05, -0.025),
        'D' => fit(0.05, 0.05, 0.0),
        'E' => fit(0.075, -0.15, 0.0),
        'F' => fit(-0.035, 0.5, 0.0),
        'G' => fit(0.005, 0.1, -0.025),
        'H' => fit(0.05, 0.1, 0.0),
        'I' => fit(-0.01, 0.0, 0.0),
        'J' => fit(-0.005, 0.4, 0.0),
        'K' => fit(0.065, -0.15, 0.0),
        'L' => fit(0.055, -0.8, 0.0),
        'M' => fit(0.07, 0.05, 0.0),
        'N' => fit(-0.035, 0.3, 0.0),
        'O' => fit(0.045, 0.0, -0.025),
        'P' => fit(-0.15, 0.85, 0.0),
        'Q' => fit(-0.45, -0.05, -0.025),
        'R' => fit(0.085, -0.5, 0.0),
        'S' => fit(0.055, -0.05, 0.0),
        'T' => high(fit(-0.1, 0.5, -0.06)),
        'U' => fit(-0.045, 0.35, 0.0),
        'V' => high(fit(-0.205, 0.85, -0.025)),
        'W' => high(fit(-0.23, 0.9, -0.025)),
        'X' => fit(0.08, -0.05, 0.0),
        'Y' => high(fit(-0.135, 0.65, -0.025)),
        'Z' => fit(0.07, -0.1, 0.0),
        _ => PLAIN,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The web's profiles and initial sizes, read from its stylesheet: the two must not drift apart.
    #[test]
    fn every_profile_matches_the_web() {
        let css = include_str!("../../../apps/office-web/static/style.css");
        let mut seen = 0;
        for line in css.lines() {
            let Some(rest) = line.trim().strip_prefix(".elements [data-initial=\"") else { continue };
            let letter = rest.chars().next().unwrap();
            let value = |name: &str| {
                rest.split(';')
                    .find_map(|decl| decl.trim().trim_start_matches(|c| c != '-').strip_prefix(&format!("--initial-{name}:")))
                    .map_or(0.0, |v| v.trim().trim_end_matches("em").parse::<f32>().unwrap())
            };
            let web = InitialFit { gap: value("gap"), tuck: value("tuck"), hang: value("hang"), depth: value("depth"), raised_tuck: 0.0 };
            assert_eq!(InitialFit { raised_tuck: 0.0, ..initial_fit(letter) }, web, "{letter}");
            seen += 1;
        }
        assert_eq!(seen, 26);
        // The raised tuck is one rule over the letters that take it.
        let rule = css.split(":is(").find(|r| r.contains("--initial-raised-tuck")).expect("raised tuck rule");
        let (letters, body) = rule.split_once(')').unwrap();
        let tuck: f32 =
            body.split("--initial-raised-tuck:").nth(1).unwrap().split(';').next().unwrap().trim().trim_end_matches("em").parse().unwrap();
        let raised: Vec<char> =
            letters.split(',').map(|l| l.trim().trim_start_matches("[data-initial=\"").chars().next().unwrap()).collect();
        for letter in 'A'..='Z' {
            let want = if raised.contains(&letter) { tuck } else { 0.0 };
            assert_eq!(initial_fit(letter).raised_tuck, want, "{letter}");
        }
        // The em the profiles are measured in: the psalm initial's declared size.
        let rule = css.split(".psalm-verses .verse:first-child::first-letter {").nth(1).expect("psalm initial").split('}').next().unwrap();
        let size: f32 = rule.split("font-size:").nth(1).unwrap().split("em").next().unwrap().trim().parse().unwrap();
        assert_eq!(size, PROFILE_EM);
        // The raised initial's size and space, from the web's `.initial-raised` rule.
        let rule = css.split("initial-letter: normal;").nth(1).expect("raised rule").split('}').next().unwrap();
        let value = |name: &str| rule.split(name).nth(1).unwrap().trim_start().split("em").next().unwrap().trim().parse::<f32>().unwrap();
        assert_eq!(value("font-size:"), RAISED_SIZE);
        assert_eq!(value("margin: 0"), RAISED_GAP);
    }
}
