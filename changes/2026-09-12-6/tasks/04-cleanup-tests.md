
# SCF-04: Remove placeholder machinery, rewrite tests

Status: see [../ledger.md](../ledger.md).

## Objective

Delete the superseded placeholder-titling flow (GEN) and update all tests to the discussion-first model.

## Dependencies

SCF-01, SCF-02.

## Scope

In scope: remove `placeholderTitle` and its word lists, the old `primePrompt`, and rewrite/replace `changesession_test.go` cases that asserted the old flow (placeholder scaffold, title-required, old prompt contents).
Out of scope: functional changes beyond removal.

## Implementation steps

1. Delete placeholder generator + old prompt builder + dead code paths.
2. Rewrite tests: discussion creation (prompt injection, `_unassigned` mapping), scaffold trigger (success, validation, idempotent move), HX behaviors if kept.
3. Full suite green; `tasktracker validate` clean.

## Verification

- `go test ./...` passes with no references to the removed flow.

## Completion criteria

- No placeholder code remains; tests reflect the new model.

## Files affected

- `internal/server/changesession.go`, `internal/server/changesession_test.go`

## Notes

- None yet.
