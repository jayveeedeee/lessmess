---
id: CID-03
title: Learnings upkeep and full verification
---

# CID-03: Learnings upkeep and full verification

Status: see [../ledger.md](../ledger.md).

## Objective

Refresh the doc learnings that describe the counter scheme and run the
repo-level verification that closes out the change.

## Dependencies

- CID-00, CID-01, CID-02 (their outcomes are what the learnings describe).

## Scope

In-marker learnings in `internal/store/AGENTS.md` and
`internal/server/AGENTS.md`, plus the final verification pass described in
[../plan.md](../plan.md). No code changes.

## Implementation steps

1. `internal/store/AGENTS.md`: update the learning stating "Sequence numbers
   are never reused (highest existing + 1)" to describe random-suffix
   minting with a collision check, citing this change ID (2026-09-15-1).
2. `internal/server/AGENTS.md`: update the learning about the index sort
   ("the unpadded counter compares numerically") to describe date-descending
   then ledger-position ordering, citing this change ID.
3. Edit only inside the `tasktracker:` markers with content outside them
   preserved byte-for-byte; the close-out gardener may refine further.
4. Full verification:
   - `go vet ./...`
   - `go test ./...`
   - Rebuild: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`
   - `lessmess validate` on this repository — must be clean with its mixed
     numeric and random IDs.
5. Confirm `README.md` needs no update (it does not document the naming
   scheme); revisit only if verification proves otherwise.

## Verification

- All commands in step 4 pass with no violations.
- `grep -rn "highest existing" internal/*/AGENTS.md` and
  `grep -rn "compares numerically" internal/server/AGENTS.md` no longer
  describe the counter scheme (or the updated text explicitly supersedes it
  with this change ID).

## Completion criteria

- Learnings match shipped behavior and cite 2026-09-15-1.
- Full verification evidence recorded in the ledger notes.

## Files affected

- `internal/store/AGENTS.md`
- `internal/server/AGENTS.md`

## Notes

- The plan's acceptance criteria become checkable only after this task's
  step 4 — record the outcomes in the ledger before moving tasks to Test.
