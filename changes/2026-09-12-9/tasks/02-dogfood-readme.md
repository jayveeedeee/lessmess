---
id: REF-02
title: Dogfood and README touch-up
---

# REF-02: Dogfood and README touch-up

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the bell/modal/refresh loop live on this repository and update README.

## Dependencies

- REF-00, REF-01

## Scope

- Serve with the new binary: badge shows the current docs findings; modal
  lists them grouped; button reconciles the current hash-stale dirs (real
  gardener job); badge/modal/tree update after completion without reload.
- README: bell/modal/refresh mention in the docs management section.
- Record evidence in Notes.

## Implementation steps

1. Run the acceptance criteria live; capture what happened.
2. Fix/tune anything the run reveals.
3. README edit; final `go vet ./... && go test ./...` and validate.

## Verification

- plan.md acceptance criteria all checked with evidence.

## Completion criteria

- Live loop demonstrated; README accurate; suites green.

## Files affected

- `README.md`
- Any tuning discovered

## Notes

- 2026-09-12 — DOGFOOD PASSED (serve :9097, real service): findings before = 6
  hash-stale dirs (EXP work); POST /docs/refresh enqueued exactly those as one
  manual job; gardener finished in ~1 min; queue clean; `/api/validate` docs
  findings = 0. Bonus production evidence: the gardener's manual-branch prompt
  added four accurate `(manual)`-prefixed learnings to the root AGENTS.md auto
  section (CLI subcommands, coverage semantics, PNG provenance, opencode.json
  envelope), and the drift test correctly tolerates the machine section.
  Mid-run fix: root ledger for this change was left at Planned when work began
  (rule 6 caught it) — synced to In progress.
- README's docs-management Visibility bullet now describes the bell/modal/
  refresh flow. Final: `go vet ./...` clean, full `go test ./... -count=1`
  green (5 packages), `tasktracker validate` OK (zero findings).