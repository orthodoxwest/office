//! Every octave's General Rubrics VII.3 group, as the Diurnal's rank line under
//! each feast's title gives it (#696): "Double I Class with privileged Octave I
//! Order", "with Common Octave", "Double II Class with Simple Octave".

use std::collections::BTreeMap;
use std::path::Path;

use calendar::loader::load_feasts;
use calendar::model::OctaveClass;
use tools::fs::FsData;

#[test]
fn every_octave_has_the_diurnal_class() {
    let data = FsData::new(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data"));
    let feasts = load_feasts(&data).expect("feasts load");
    let classed: BTreeMap<&str, OctaveClass> =
        feasts.iter().filter(|f| f.has_octave || f.octave_class == OctaveClass::Simple).map(|f| (f.id.as_str(), f.octave_class)).collect();
    use OctaveClass::*;
    let expected = BTreeMap::from([
        ("easter-sunday", PrivilegedFirst),
        ("pentecost", PrivilegedFirst),
        ("epiphany", PrivilegedSecond),
        ("corpus-christi", PrivilegedSecond),
        ("christmas", PrivilegedThird),
        ("ascension", PrivilegedThird),
        ("st-george", Common),
        ("nativity-john-baptist", Common),
        ("ss-peter-paul", Common),
        ("assumption-bvm", Common),
        ("all-saints", Common),
        ("conception-bvm", Common),
        ("st-stephen", Simple),
        ("st-john-evangelist", Simple),
        ("holy-innocents", Simple),
        ("st-lawrence", Simple),
        ("nativity-bvm", Simple),
        ("octave-day-st-stephen", Simple),
        ("octave-day-st-john", Simple),
        ("octave-day-holy-innocents", Simple),
    ]);
    assert_eq!(classed, expected);
}
