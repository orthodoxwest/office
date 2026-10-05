//! Wording the vicariate deliberately takes over the printed Diurnal. Page-image
//! rechecks conform the corpus to the Diurnal, so each ruling is pinned here.

use std::path::Path;
use tools::corpus_edit::read_corpus_body;

fn body(key: &str) -> String {
    let data = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../data");
    read_corpus_body(&data, key).expect("corpus key").split_whitespace().collect::<Vec<_>>().join(" ")
}

/// #277/#278: the supervising priest asked for "humanity of our crucified Lord".
#[test]
fn sacrosanctae_keeps_the_vicariate_wording() {
    let text = body("ordinary/session/sacrosanctae");
    assert!(
        text.contains("to the humanity of our crucified Lord, Jesus Christ,"),
        "Sacrosanctae lost the vicariate wording ruled in #277/#278:\n{text}"
    );
    assert!(!text.contains("crucified Humanity"), "Sacrosanctae reverted to the Diurnal wording:\n{text}");
}
