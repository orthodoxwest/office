# Lauds psalmody declarations: Office of the Dead

Issue #245 migrates the existing All Souls appointment without changing its
rendered office. The Monastic Diurnal pp.99*–104* supplies Psalms51,65,63,
Isaiah38, and Psalm150 alone as the Laudate. The Rest Eternal formula remains
an office-wide doxology rule (p.72*). This is a declaration migration, not a
new transcription or source attestation.

`lauds-psalmody` retains its existing `festal` marker. A proper can instead
select antiphon/psalm rows using the shared psalmody declaration grammar, with
`lauds-laudate-psalmody` supplying the separate final section. Rows accept
psalm or canticle corpus keys. Lauds does not use the Vespers `ferial` markers.
A concrete main declaration requires a Laudate declaration; validation checks
both row syntax and references. ProperID and Paschal proper redirects apply.
The declaration controls psalmody; it does not select the Office of the Dead
or alter the rest of an office's structure.

The hour template uses the generic `declared-lauds-psalmody` condition, and
`proper-psalmody` now resolves declarations for either major hour. This removes
the hand-expanded Dead psalmody blocks and replaces their feast-specific
exclusions with the declaration condition. Ordinary, festal, Saturday, Easter,
and Triduum appointments retain their existing forms. November14 remains
outside this repair.

`TestDeclaredLaudsOfTheDead` checks every All Souls observance in 2024–2051,
including Saturday and Sunday transfers, in all prayer forms. It asserts
psalm/canticle order, Psalm150 alone, and the six Rest Eternal doxologies.
The declaration resolution and validation tests cover redirects, weekday
suppression, missing Laudate, invalid references, unsupported markers, and
unsupported hour/ref combinations. An adversarial reviewer inspected the
new generic dispatch and conditions and found no actionable defect.

The same 2024–2051 calendar was composed before and after in all seven hours
and all three prayer forms: **214,767 offices**. Every serialized office,
including section boundaries, text, source references, prayer-form delivery,
and psalm/canticle types, matches exactly. Composition-decision metadata was
excluded because it records the renamed template conditions. Golden prayer
output likewise remains unchanged. The baseline executable was built from
789237e, whose tracked tree matches merged base6175dc0. Generated evidence
and comparison executables remain under ignored `output/lauds-review/`.
