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
