use super::*;

#[test]
fn unresolved_markers_are_detected_in_rendered_text() {
    assert_eq!(unresolved_marker("[Text not found: ordinary/lauds/collect]"), Some("[Text not found: ordinary/lauds/collect]"));
    assert_eq!(
        unresolved_marker("Ant. [Commemoration text not found: commemoration-antiphon for st-x] V."),
        Some("[Commemoration text not found: commemoration-antiphon for st-x]")
    );
    assert!(unresolved_marker("[Little Hours versicle not found: proper/x/short-responsory]").is_some());
    // Canticle section markup and ordinary brackets are not markers.
    assert_eq!(unresolved_marker("[section: Benedicite] O all ye Works of the Lord"), None);
    assert_eq!(unresolved_marker("(which he had promised afore) [sic]"), None);
}

#[test]
fn seasonal_hymn_endings_replace_only_iambic_dimeter_quatrains() {
    // Diurnal p. 364: "all Hymns of the same metre".
    assert_eq!(syllables("All laud to God the Father be;"), 8);
    assert_eq!(syllables("To God the Holy Paraclete."), 8);
    assert_eq!(syllables("Earth's mighty fabric ruleth and directeth,"), 11);
    assert_eq!(syllables("Only and Trinal."), 5);
    let lm = "Title\n\nNow that the daylight fills the sky,\n\nAll laud to God the Father be;\nAll praise, eternal Son, to thee;\nAll glory, as is ever meet,\nTo God the Holy Paraclete.\n\nAmen.";
    assert!(has_common_metre_ending(lm));
    let sapphic = "His be the glory, power and salvation\nWho, o'er the heavens, dwelling in the highest,\nEarth's mighty fabric ruleth and directeth,\nOnly and Trinal.\n\nAmen.";
    assert!(!has_common_metre_ending(sapphic));
    let trochaic = "Glory be to God, and honour\nIn the highest, as is meet,\nTo the Son, and to the Father,\nAnd the eternal Paraclete,\nWhose is boundless praise and power\nThrough the ages infinite.\n\nAmen.";
    assert!(!has_common_metre_ending(trochaic));
    assert!(!has_common_metre_ending(
        "Father, Son eternal,\nHoly Ghost supernal,\nWith one praise we bless thee,\nThree in One confess thee.\n\nAmen."
    ));
}
