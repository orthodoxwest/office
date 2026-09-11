---
name: ordo-verify
description: Assess calendar and composed-office parity against local archdiocesan ordos, diagnose discrepancies, and verify calendar, engine, or corpus repairs.
---

# Ordo verification

Establish whether the app supplies the appointed office. A perfect match to
an error-bearing PDF is not the target; a difference is a finding to diagnose.

## Authority and evidence

Locate the available ordos in the resources directory; use the newest local
archdiocesan ordo for current practice. Set the comparison year explicitly
rather than relying on script defaults. Older ordos are comparison witnesses,
not grounds for overriding a newer appointment. A closed issue or an old
classification is not itself a clergy ruling.

When the current ordo conflicts with the Diurnal, normative rubrics, or older
ordos, document the conflict for clergy and leave the affected entries
unapplied. Distinguish confirmed app defects, proposed printed errata, and
unresolved source conflicts. Independently check computus discrepancies by
arithmetic and the stated calendar convention. Do not reproduce a numerical
error to improve agreement.

Read the printed page when an extracted instruction, scope, or wording is in
doubt. Keep PDFs outside Git and generated evidence under ignored `output/`.
Text discovery and transcription follow [the Diurnal pipeline](../../../scripts/DIURNAL-PIPELINE.md)
and [AGENTS.md](../../../AGENTS.md), including independent readings and guarded
corpus/attestation writes. Source verification does not certify an appointment or composed office.

## Capture a reproducible comparison

Work from the checkout being assessed and build its binary. Record its commit,
local changes, PDF hash, target year, and default/scoped calendar being compared.
Locate resources explicitly in a worktree; they may not be at `../resources`.
Set `ORDO_YEAR`, `ORDO_RESOURCES`, and a fresh `ORDO_RUN` beneath `output/`:

```bash
make build
mkdir -p "$ORDO_RUN"
pdftotext -layout "$ORDO_RESOURCES/$ORDO_YEAR-ordo.pdf" "$ORDO_RUN/reference.txt"
./office ordo "$ORDO_YEAR" > "$ORDO_RUN/ours.txt"
./office rubrics "$ORDO_YEAR" > "$ORDO_RUN/rubrics.tsv"
python3 scripts/project-status.py --year "$ORDO_YEAR" --resources "$ORDO_RESOURCES" \
  --output "$ORDO_RUN" --offline
```

The status command writes Markdown, JSON, and a finding CSV. `--offline` skips
its optional GitHub query; check the current ruling issue when adjudicating a
finding. The triage ledger is `data/review/ordo-triage.csv`; reassess stale
classifications instead of inheriting them as facts.

For focused diagnostics, `python3 scripts/ordo-compare.py COMMAND REFERENCE APP`
accepts `calendar`, `colors`, and `vespers` with `ours.txt`; `rubrics`,
`commemorations`, and `antiphons` use `rubrics.tsv`. Prefer commemoration names
to presence/absence alone. The generated ordo's Tabula supports separate checks
of computus and moveable dates.

## Diagnose before repairing

First check the comparison's boundaries: all expected dates parsed, no next-year
or appendix contamination, the default office separated from optional offices,
and titles/quotes/commemoration lists extracted correctly. Missing I/II Vespers
notation makes ownership unasserted; it is not an assertion of a different owner.
Known parser defects belong in regression tests, not a growing exceptions list
in this skill.

For a substantive discrepancy, connect the printed appointment to the actual
civil date, hour, liturgical owner, requested slot, selected source, and fallback
path. A verified text can still be selected for the wrong office. Useful tools:

```bash
./office review explain HOUR YYYY-MM-DD
./office review resolution-inventory -start "$ORDO_YEAR" -years 1 -json \
  > "$ORDO_RUN/resolution.json"
./office review provenance -start "$ORDO_YEAR" -years 1 \
  > "$ORDO_RUN/provenance.txt"
```

Inspect the rendered hour as well as its resolution evidence. Check I versus II
Vespers, seasonal/ProperID redirects, outgoing versus incoming commemorations,
and whether a proper ends at None. When changing a resolver, examine its other
affected slots; repairing one fallback can expose a missing hour-specific proper.
Group symptoms by supported cause and retain date-level evidence. Keep evolving
findings and errata in run reports, the triage ledger, and relevant issues.

## Verify and report

Freeze before/after outputs and use the **same comparator** on both. If the
comparator changes, reprocess the frozen baseline too; report measurement
corrections separately from application repairs. Check newly introduced and
changed discrepancies, not only the net count.

After implementation, use direct appointment/boundary regressions, inspect
rendered changes, and run the repository's required checks. Refresh golden
snapshots after reviewing why they changed. A documentation-only skill edit
does not require a new annual parity sweep.

Report comparable assertions, differences, cleared repair targets, and remaining
uncertainty. Keep these measures distinct:

- Headline/incipit matches do not verify ranks, full wording, collects, psalmody,
  or complete office structure. Matins and Mass are outside this app's supported
  office comparison.
- Static proper-slot coverage permits suppressions and fallbacks and may miss
  redirects. No unresolved text markers does not mean every proper is correct.
- Hash-bound source attestations and usage-weighted coverage measure source
  verification, not correct selection.

For composition review, follow the source-requirement checklist in
[REVIEWING.md](../../../REVIEWING.md#composition-review): record the cited rule,
expected behavior, representative/boundary cases, and linked tests or open
questions. `review plan` only supplies examples of observed engine behavior;
its feature and sample counts are not completion measures. Whole-page signoffs
are retired. Keep known incomplete Triduum work separate from ordinary-year
assessment.

Do not turn the aggregate agreement percentage, old “known residue,” or the
number of likely ordo typos into a claim of whole-office correctness.
