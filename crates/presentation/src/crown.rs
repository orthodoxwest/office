//! Home's crown: the consecration roundel at the head of the arch. A tap on it turns the page's
//! look over, Nave to Apse or Apse to Nave, and the change plays as dusk or dawn in the arch. It is
//! never announced: the menu's Theme row stays the way to choose, and the crown only answers the
//! reader who finds it. Every front reads the rule and the play's timetable here; app.js mirrors
//! both.

/// The menu's Theme row: Default follows the device; Nave is light and Apse dark.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ThemeChoice {
    Default,
    Nave,
    Apse,
}

impl ThemeChoice {
    /// Whether the page is dark under this choice on a device that is (or is not) dark.
    pub fn dark(self, device_dark: bool) -> bool {
        match self {
            ThemeChoice::Default => device_dark,
            ThemeChoice::Nave => false,
            ThemeChoice::Apse => true,
        }
    }
}

/// What a tap on the crown leaves behind: the choice to save, and the choice it turned away from,
/// which the front keeps in memory (never saves) for the tap back.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct CrownTap {
    pub choice: ThemeChoice,
    pub restore: Option<ThemeChoice>,
}

/// The choice after a tap on the crown. The look always turns over. A look that matches the
/// device's is saved as Default, so a reader who wanders back to the device's look is following
/// the device again rather than pinned to it. The tap back undoes the tap exactly: `restore`, the
/// choice the last tap turned away from, comes back when the new tap lands on its look, so a
/// reader who had chosen Apse on a dark device and taps twice still has Apse, not Default.
pub fn after_crown_tap(choice: ThemeChoice, device_dark: bool, restore: Option<ThemeChoice>) -> CrownTap {
    let dark = !choice.dark(device_dark);
    if let Some(back) = restore.filter(|r| r.dark(device_dark) == dark) {
        return CrownTap { choice: back, restore: None };
    }
    let next = match (dark == device_dark, dark) {
        (true, _) => ThemeChoice::Default,
        (false, true) => ThemeChoice::Apse,
        (false, false) => ThemeChoice::Nave,
    };
    CrownTap { choice: next, restore: Some(choice) }
}

/// A stretch of the play: when it begins, in ms from the tap, and how long it runs.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Span {
    pub at: u32,
    pub len: u32,
}

/// A cubic-bezier ease, as CSS writes it: (x1, y1, x2, y2).
pub type Ease = [f32; 4];

/// A pigment, as red, green and blue.
pub type Rgb = [u8; 3];

/// One direction of the crown's play: the timetable of a dusk or a dawn, in ms from the tap. The
/// cross answers the tap at once; the sky changes in the head of the arch, behind a front that
/// crosses it; then the page takes the new light, spreading out of the arch from under the
/// frieze, with a thin veil of the hour's colour over the half-turned wall. Light arrives faster
/// than it leaves. Every front plays to these figures; app.js mirrors them and office-web tests
/// the two agree.
#[derive(Clone, Copy, Debug, PartialEq)]
pub struct CrownPlay {
    /// When the look itself changes under the play (the saved choice is written at the tap).
    /// Until then an interruption has nothing to play out and the change is simply made.
    pub turn: u32,
    /// The front crossing the head of the arch: down from above the point at dusk, up from under
    /// the horizon at dawn. It eases by [`FRONT_EASE`].
    pub front: Span,
    /// The page beyond the arch taking the new look. Its shape eases by [`SPREAD_EASE`]; on a
    /// screen wider than 700 it runs [`SPREAD_WIDE`] longer.
    pub spread: Span,
    /// The veil over the turning wall, from its first trace to its last (both in ms from the tap).
    pub veil: (u32, u32),
    /// The play's length. It ends later only where the page's far corner is not yet covered.
    pub end: u32,
    /// The veil's colour, and its strength at its height.
    pub veil_ink: Rgb,
    pub veil_peak: f32,
    /// The share of the new look under a line at which its ink is handed over: where the ground
    /// is dark enough for a light ink at dusk, or just light enough for a dark one at dawn.
    pub ink_at: f32,
    /// How the page's own opacity rises through the spread.
    pub glow_ease: Ease,
}

/// Nave to Apse.
pub const DUSK: CrownPlay = CrownPlay {
    turn: 650,
    front: Span { at: 800, len: 1700 },
    spread: Span { at: 2200, len: 1750 },
    veil: (2000, 3500),
    end: 4000,
    veil_ink: [92, 94, 160],
    veil_peak: 0.36,
    ink_at: 0.6,
    glow_ease: [0.55, 0.1, 0.7, 1.0],
};

/// Apse to Nave.
pub const DAWN: CrownPlay = CrownPlay {
    turn: 650,
    front: Span { at: 800, len: 1600 },
    spread: Span { at: 1850, len: 1800 },
    veil: (1650, 3600),
    end: 3700,
    veil_ink: [226, 174, 140],
    veil_peak: 0.3,
    ink_at: 0.33,
    glow_ease: [0.5, 0.2, 0.7, 1.0],
};

/// The play for a tap that turns the page dark (dusk) or light (dawn).
pub fn crown_play(to_dark: bool) -> CrownPlay {
    if to_dark { DUSK } else { DAWN }
}

/// The front in the head of the arch.
pub const FRONT_EASE: Ease = [0.3, 0.0, 0.7, 1.0];
/// The spread's shape: near enough even, since it starts from nothing under the frieze and ends
/// at the screen's corner.
pub const SPREAD_EASE: Ease = [0.4, 0.0, 0.5, 1.0];
/// How much longer the spread runs on a screen wider than 700, in ms.
pub const SPREAD_WIDE: u32 = 100;
/// The feather of the spread's edge, in card widths where the spread begins: wide, so the light
/// fills the room and no line crosses it.
pub const SPREAD_FEATHER: f32 = 0.42;
/// The spread's shape has reached every place by this share of the spread's length, and grows
/// by an even ratio bent a little toward its start ([`SPREAD_BEND`]: size = first x (last /
/// first)^(t^bend)).
pub const SPREAD_SHAPE_PART: f32 = 0.45;
pub const SPREAD_BEND: f32 = 0.8;
/// The page's own opacity rises over this share of the spread's length, so every place begins
/// early and none is done before the end.
pub const SPREAD_GLOW_PART: f32 = 0.96;
/// The veil's shape runs this much larger than the page's, and is done by this share of the
/// spread, so the veil is whole where the page is half turned.
pub const VEIL_LEAD: f32 = 1.4;
pub const VEIL_PART: f32 = 0.8;
/// Dawn's veil through the wall's grey passage: a thin rose-umber at this strength, between the
/// blush it begins and ends in.
pub const DAWN_VEIL_UMBER: Rgb = [200, 100, 74];
pub const DAWN_VEIL_LOW: f32 = 0.15;
/// A line's ink changes hands in this long (old ink out and new ink in, never both), in ms.
pub const INK_HANDOVER: u32 = 140;
/// The sky's pigments, fresco earths: the afterglow is yellow ochre with red ochre only at the
/// horizon, its ember on the night sky burnt sienna; first light is lime white, parted from the
/// sky's teal by a rose-lilac.
pub const SKY_OCHRE: Rgb = [201, 151, 63];
pub const SKY_RED_OCHRE: Rgb = [181, 101, 63];
pub const SKY_EMBER: Rgb = [160, 86, 62];
pub const SKY_LIME: Rgb = [244, 232, 223];
pub const SKY_ROSE_LILAC: Rgb = [196, 163, 166];
/// A second tap on the cross while the page is turning is kept, once, and plays the way back at
/// the settle; a tap closer than this to the last (ms) is the same tap's flurry, not a change of
/// mind.
pub const TAP_APART: u32 = 450;
/// A touch elsewhere while it plays does not cut it: what is left runs out in this long (ms).
pub const RUSH: u32 = 300;

#[cfg(test)]
mod tests {
    use super::ThemeChoice::{Apse, Default, Nave};
    use super::*;

    #[test]
    fn default_turns_to_the_device_opposite() {
        assert_eq!(after_crown_tap(Default, false, None), CrownTap { choice: Apse, restore: Some(Default) });
        assert_eq!(after_crown_tap(Default, true, None), CrownTap { choice: Nave, restore: Some(Default) });
    }

    #[test]
    fn a_look_matching_the_device_is_saved_as_default() {
        assert_eq!(after_crown_tap(Apse, false, None).choice, Default);
        assert_eq!(after_crown_tap(Nave, true, None).choice, Default);
    }

    #[test]
    fn the_tap_back_restores_the_choice_it_turned_from() {
        for device_dark in [false, true] {
            for start in [Default, Nave, Apse] {
                let there = after_crown_tap(start, device_dark, None);
                assert_ne!(there.choice.dark(device_dark), start.dark(device_dark));
                let back = after_crown_tap(there.choice, device_dark, there.restore);
                assert_eq!(back, CrownTap { choice: start, restore: None }, "{start:?} on dark={device_dark}");
            }
        }
    }

    #[test]
    fn a_stale_restore_on_the_wrong_look_is_ignored() {
        // The device turned dark between taps: restoring Nave would not turn the look over.
        assert_eq!(after_crown_tap(Default, true, Some(Apse)).choice, Nave);
    }

    #[test]
    fn the_play_keeps_its_order() {
        for play in [DUSK, DAWN] {
            // The look turns before the front shows, the page follows the head, and the veil
            // lies over the spread from before its start to (near) its end.
            assert!(play.turn < play.front.at);
            assert!(play.front.at < play.spread.at && play.spread.at < play.front.at + play.front.len);
            assert!(play.veil.0 < play.spread.at && play.veil.1 <= play.end);
            assert!(play.spread.at + play.spread.len + SPREAD_WIDE <= play.end + 50);
            assert!(play.veil_peak < 0.5 && (0.0..1.0).contains(&play.ink_at));
        }
        // Light arrives faster than it leaves.
        const { assert!(DAWN.end < DUSK.end) };
        assert_eq!(crown_play(true), DUSK);
        assert_eq!(crown_play(false), DAWN);
    }
}
