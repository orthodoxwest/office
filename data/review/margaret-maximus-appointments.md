# Margaret of Scotland and Maximus of Constantinople

The 2026 archdiocesan Ordo p.70 appoints the Holy Women commemoration
antiphons for Margaret: "The kingdom of heaven" on June9 at Vespers and
"Give her" on June10 at Lauds. Her Memorial had been assigned the male
Confessor text category. Correcting it to `holy-woman` selects the existing
Common antiphons and versicles while preserving her proper collect.
Dedicated Paschal commemoration aliases also retain the Alleluias printed
on Diurnal p.5*, instead of falling through to ordinary Common slots.

For Maximus, Ordo p.89 explicitly appoints "Attend" at Lauds, the little
Hours, and Vespers. The missing proper collect caused a fallback to
"O God, who makest us glad". The alternate Confessor collect on Diurnal
p.45* is now transcribed in the Common and selected by Maximus's proper
alias. Independent Codex and Sonnet image readings agreed after the
pipeline's comparison normalization. The new text has a hash-bound source
attestation; the alias records the Ordo appointment. The shared conclusion
handling expands the printed abbreviation.

These are data corrections; they introduce no engine conditions and change
no eligibility, rank, calendar ownership, or commemoration ordering.
`TestMargaretAndMaximusAppointments` checks both Margaret appointments,
Maximus's major and little Hours, proper-name substitution, and Eusebius's
unchanged collect on August13, in all three prayer forms.
`TestPaschalHolyWomanCommemoration` covers the seasonal Common fallback.

A comparison of 214,767 composed offices in 2024–2051 (seven Hours, all
three forms) found603 changed office forms, or201 date/Hour pairs. Changes
are confined to June9 Vespers, June10 Lauds, August12 Vespers, and August13
Lauds/Terce/Sext/None/Vespers. In2026 these are eight date/Hour pairs.
The generated Ordo changes only Margaret's two commemoration incipits.
The before/after hashes, independent reading, and source-cache metadata
remain under ignored `output/saint-propers-review/`.

This is a bounded part of issue124's proper/fallback audit. It does not
implement the separate same-Common variation rule in issue138.
