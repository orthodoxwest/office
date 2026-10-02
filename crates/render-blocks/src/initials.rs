//! How a dropped initial fits the text beside it: the web's optical profile for every capital
//! of the bundled EB Garamond (`style.css`, `[data-initial]`), so the native apps fit each
//! painted letter's contour as the web does rather than setting every capital as a box.
//!
//! As Chrome sets `initial-letter`, the fit is to the letter's ink, not its advance: the ink's
//! left edge stands at the margin (moved by `hang`), the lines beside it start `gap` past its
//! right edge, and its top meets the first line's cap height, so a capital that rises above
//! cap height (A, T) stands that much lower.

/// EB Garamond's cap height, in em (`OS/2.sCapHeight` 650 of 1000 units).
pub const CAP_HEIGHT: f32 = 0.65;

/// The size of a two-line initial, in ems of the text beside it, as CSS `initial-letter: 2`
/// sizes it: its cap height spans one line pitch and the text's own cap height, so its top
/// meets the first line's capitals and its foot stands on the second line's baseline.
pub fn initial_size(line_height_em: f32) -> f32 {
    1.0 + line_height_em / CAP_HEIGHT
}

/// One capital's fit. `gap`, `hang` and `depth` are in the initial's em; `tuck` in the text's.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct InitialFit {
    /// Space between the letter and the lines beside it; negative lets them run in under an
    /// arm or beside a narrow stem (F, P, T, V, W).
    pub gap: f32,
    /// How far the first line's opening word moves from that edge: in under a high arm (T, F),
    /// or out past a wide top (A, L, R).
    pub tuck: f32,
    /// How far the letter stands out into the margin: round and pointed capitals overshoot.
    pub hang: f32,
    /// Extra clearance below the second line for a descending tail (Q): the next line too
    /// stands beside the letter.
    pub depth: f32,
}

const PLAIN: InitialFit = InitialFit { gap: 0.06, tuck: 0.0, hang: 0.0, depth: 0.0 };

/// The fit for an initial; a letter outside the profiled capitals takes the plain fit.
pub fn initial_fit(letter: char) -> InitialFit {
    let fit = |gap, tuck, hang| InitialFit { gap, tuck, hang, depth: 0.0 };
    match letter {
        'A' => fit(0.06, -0.8, -0.02),
        'B' => fit(0.08, -0.2, 0.0),
        'C' => fit(0.08, -0.05, -0.025),
        'D' => fit(0.03, 0.1, 0.0),
        'E' => fit(0.08, -0.15, 0.0),
        'F' => fit(-0.135, 0.65, 0.0),
        'G' => fit(-0.005, 0.0, -0.025),
        'H' | 'K' | 'X' => fit(0.06, 0.0, 0.0),
        'I' => fit(0.03, 0.0, 0.0),
        'J' | 'N' | 'U' => fit(-0.055, 0.35, 0.0),
        'L' => fit(0.06, -0.9, 0.0),
        'M' | 'S' | 'Z' => fit(0.06, -0.05, 0.0),
        'O' => fit(0.045, 0.0, -0.025),
        'P' => fit(-0.215, 0.85, 0.0),
        'Q' => InitialFit { depth: 0.1, ..fit(0.06, -0.2, -0.025) },
        'R' => fit(0.08, -0.8, 0.0),
        'T' => fit(-0.15, 0.65, -0.06),
        'V' => fit(-0.265, 1.0, -0.025),
        'W' => fit(-0.215, 0.85, -0.025),
        'Y' => fit(-0.1, 0.6, -0.025),
        _ => PLAIN,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The web's profiles, read from its stylesheet: the two tables must not drift apart.
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
            let web = InitialFit { gap: value("gap"), tuck: value("tuck"), hang: value("hang"), depth: value("depth") };
            assert_eq!(initial_fit(letter), web, "{letter}");
            seen += 1;
        }
        assert_eq!(seen, 26);
    }

    #[test]
    fn a_two_line_initial_spans_a_line_pitch_and_a_cap_height() {
        // 20px text on a 33px line: 20 + 33 / 0.65 px.
        assert!((initial_size(1.65) * 20.0 - 70.77).abs() < 0.01);
    }
}
