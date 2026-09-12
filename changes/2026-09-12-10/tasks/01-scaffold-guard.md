---
id: BSB-01
title: Scaffold guard for bound sessions
---

# BSB-01: Scaffold guard for bound sessions

Status: see [../ledger.md](../ledger.md).

## Objective

Make `POST /changes/scaffold` refuse any session that is already mapped to a
change, so the one-discussion-one-change invariant is enforced deterministically
server-side.

## Dependencies

—

## Scope

- `internal/server/mapping.go`: `changeOf(session string) (string, bool)`
  lookup across all change keys (excluding the `_unassigned` bucket).
- `internal/server/changesession.go`: guard at the top of `scaffoldChange`
  (after body validation, before `CreateChange`) returning 409 with an
  agent-facing JSON message naming the change.
- Unit tests.

## Implementation steps

1. Add `changeOf` on `mapping`: iterate `m.data` skipping `unassignedKey`;
   return the change ID whose entries contain the session.
2. In `scaffoldChange`, after request validation and the `mapErr` check: if
   `changeOf` finds the session, write 409 with
   `{"error": "session already bound to change <id>; continue the work within that change (refine plan.md, add tasks/ledger rows), do not scaffold a new change", "change": "<id>"}`
   and create nothing.
3. Keep existing behavior for the other two cases: `_unassigned` sessions move
   on success; unmapped sessions scaffold and direct-link.
4. Tests: bound session → 409, no change directory created, root ledger
   untouched; unassigned session still scaffolds and moves; unmapped session
   still scaffolds and direct-links.

## Verification

- `go vet ./...` and `go test ./internal/server/` green; a bound-session curl
  against a test server returns 409 with the redirect message.

## Completion criteria

- Bound sessions can never trigger a scaffold; both pre-existing paths are
  covered by tests and unchanged.

## Files affected

- `internal/server/mapping.go`
- `internal/server/changesession.go`
- `internal/server/changesession_test.go`

## Notes

- No exceptions to the guard (user decision): genuinely new work starts as a
  new discussion from the index page.
- Verified 2026-09-12: `TestScaffoldBoundSessionRefused` (409, nothing
  created, mapping untouched), `TestMappingChangeOf`, plus preserved-path
  tests `TestScaffoldFlow`/`TestScaffoldIdempotentMove` — all green. Live:
  scaffold curl from this change's own bound session returned
  `409 {"change":"2026-09-12-10", ...}` and the root ledger stayed clean.
