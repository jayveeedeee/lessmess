---
id: KAN-07
title: Dogfood, end-to-end verification, README
---

# KAN-07: Dogfood, end-to-end verification, README

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the tool against this repository's real `changes/` tree, build the static single binary, and document build/usage in `README.md`.

## Dependencies

KAN-05, KAN-06.

## Scope

In scope: `CGO_ENABLED=0 go build` binary; full manual end-to-end checklist (plan acceptance criteria 1–7); README with build, run, usage, safety notes (localhost binding, no auth, create/update-only); confirmation that dogfooding writes left all change files spec-compliant (`tasktracker validate` clean).
Out of scope: release artifacts, CI, packaging.

## Implementation steps

1. Build the static binary; run `tasktracker serve` from the repo root.
2. Execute the acceptance checklist: board renders real changes; move a test card and confirm the ledger rewrite; create then cancel a throwaway task/change and clean up by hand-editing (server has no delete); externally edit a ledger and confirm SSE refresh; run `validate`.
3. Write `README.md`.
4. Final `go vet ./...`, `go test ./...`, `tasktracker validate` — all clean.

## Verification

- Every plan acceptance criterion checked with evidence recorded in this task's notes.
- `tasktracker validate` on this repository exits 0 after all dogfooding writes.

## Completion criteria

- Plan acceptance criteria 1–8 all pass; README exists and matches actual behavior.

## Files affected

- `README.md` (new)
- Any file touched by dogfood writes (verified compliant afterward)

## Notes

- Per-criterion evidence (2026-09-11):
  1. serve from repo root serves the real `changes/` tree — ✅ smoke runs on :18099 returned index, board, task detail, validation report.
  2. Five columns, cards from ledger rows in row order — ✅ board HTML shows all five status columns and 8 cards; after a live `POST /move`, KAN-06 appeared at the top of `In progress`.
  3. Drag/reorder rewrites the ledger — ✅ at unit level (byte-preservation, move semantics) and API level (live move on the real repo; file re-parsed clean). ⏳ The browser drag gesture itself awaits a human click-through (no headless browser in this environment).
  4. Create task/change produce spec-compliant files — ✅ throwaway task (KAN-08) and change (2026-09-11-2) created via API on a repo copy; `validate` OK after creation and again after hand-cleanup; cleaned copy byte-identical to the real repo.
  5. External edit appears via SSE — ✅ live `curl /events` received `event: fs` after an external append to a task file (line reverted).
  6. `validate` enforces the rules with non-zero exit — ✅ broken fixture printed per-rule violations, exit 1; real repo exits 0 with `OK`.
  7. Single static binary — ✅ `CGO_ENABLED=0 go build` produced a 13.9 MB Mach-O arm64 binary; ran from `/tmp` with embedded assets only (app.js/htmx/css all 200).
  8. `go test ./...` — ✅ all packages pass; `go vet` clean. `node --check web/static/app.js` clean.
- Residual: interactive browser click-through (drag feel, form submits, console) — to be confirmed by the user; then KAN-05 and this task can be marked Done.
- 2026-09-11: user confirmed the interactive pass and approved the final UI ("looks good"). All eight acceptance criteria now confirmed; criterion 3's browser portion covered by the user's session (hover/drag region interactions) plus the live API move on the real repo. Task complete.
