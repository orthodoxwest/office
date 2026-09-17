# Monica, Helen, and Elizabeth Memorial texts

The 2026 Ordo pp.60,90,113 explicitly appoints the Holy Women Common for
Monica, Helen, and Elizabeth. Their Memorial records still selected the male
Confessor Common. Helen's dedicated antiphons were already correct, but her
versicles and collect continued to fall through to that Common.

All three records now use `holy-woman`. Monica's printed collect is restored
from Diurnal p.525 and Helen's from p.591; independent Codex and Sonnet page
readings agree after normalization. Helen's prayer is addressed to Christ,
so its separately recorded conclusion is `who-livest`. Elizabeth takes the
existing "Hear us" collect (p.53*) appointed by the Ordo. Her explicit
ProperName keeps her descriptive title out of the prayer's name substitution.

Monica's commemoration retains the Paschal Alleluias appointed on p.5* using
the Common aliases added with the Margaret/Maximus correction. The source
ledger records the two new prayer readings. Obsolete Holy Women zero-use
classifications are removed where they no longer appear in the current
report; the remaining generic-slot and psalmody classifications describe
their actual fallback or Memorial scope.

`TestHolyWomenMemorialAppointments` checks all five 2026 date/Hour
appointments in all three prayer forms, the antiphon/versicle/collect,
Monica's Alleluias, Elizabeth's name, and Helen's conclusion metadata.
The 2024–2051 comparison covers214,767 composed offices and changes417
office forms (139 date/Hour pairs), confined to these three Memorials.
The 2026 changes are May4 Lauds, August17 Vespers, August18 Lauds,
November4 Vespers, and November5 Lauds.

No engine rules, celebration owners, ranks, eligibility, or ordering change.
The independent readings, source-page references, before/after hashes and
report remain under ignored `output/holy-women-review/`; source images and
the hash-bound Diurnal cache remain in the original workspace's output.
This continues issue124's proper/fallback audit.

Local validation: `make golden`, `make check`, and `make test-coverage`
passed, and the pre-push hook repeated `make check` successfully.
