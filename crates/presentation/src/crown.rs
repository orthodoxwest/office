//! Home's crown: the consecration roundel at the head of the arch. A tap on it turns the page's
//! look over, Nave to Apse or Apse to Nave, and the change plays as dusk or dawn in the arch. It is
//! never announced: the menu's Theme row stays the way to choose, and the crown only answers the
//! reader who finds it. Every front reads the rule here; app.js mirrors it.

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
}
