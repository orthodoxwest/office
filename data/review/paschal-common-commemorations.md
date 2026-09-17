# Paschal commemoration Commons and alternate collect appointments

Ordinary dedicated commemoration slots could hide the existing seasonal
antiphons and versicles. Twenty-eight aliases now select the Paschal forms
at Lauds and II Vespers for Apostles, Evangelists, Confessors, Bishop
Confessors, Doctors, Virgins, and Virgin Martyrs. I-Vespers selection retains
its existing priority. The appointments follow Diurnal pp.3*–5*,17*–19*;
the earlier repairs cover Martyrs and Holy Women.

The source check also found two missing Alleluias. The Apostles' I-Vespers
versicle on p.17* has Alleluia in both the versicle and response, whereas
the previous draft-book reading omitted the first. The Bishop Confessors'
II-Vespers antiphon lacked a Paschal entry altogether; p.3* supplies the
seasonal form of the antiphon printed on p.41*. Independent page-image
readings agree after typography normalization, and hash-bound attestations
record both texts. Source-image identity and the second reader's uncertainty
about the starred page glyph/punctuation are retained with the comparison.

The 2026 Ordo pp.46,61 explicitly appoints the alternate Confessor collect,
"Attend", for Joseph of Arimathea and Alexis Toth. Both now select the
existing verified Common text. Both legacy Joseph records share the same
proper so calendar duplicate selection cannot change the prayer. Explicit
ProperName values keep titles out of the shared collect.

`TestPaschalCommonCommemorationAppointments` covers all seven categories
at Lauds, I and II Vespers, and both Alleluias in the Apostles' versicle.
`TestJosephAndAlexisCollectAppointments` checks their four dated2026
appointments and name substitution in all prayer forms.

A comparison of71,589 private offices in2024–2051 changes506 date/Hour
pairs,17 in2026. Celebration owners and commemoration element counts are
unchanged. Commemoration differences are limited to antiphons, versicles,
and the two appointed collects. The31 changed principal offices contain
only the corrected Apostles/Evangelists' I-Vespers versicle or Bishop
Confessors' II-Vespers antiphon; element order and all other principal
elements are identical. Full principal-element comparisons establish that
boundary separately from the office hashes.

No engine branches, rank, occurrence, concurrence, or commemoration-order
rules change. The separate ruling holds, including March17 ordering and
Corpus Christi/Barnabas eligibility, remain outside this repair. The
same-Common variation rule in issue138 is not implemented here. This
continues issue124's proper/fallback audit. Reproducible diagnostics and
independent readings are retained under ignored `output/paschal-common-review/`.
