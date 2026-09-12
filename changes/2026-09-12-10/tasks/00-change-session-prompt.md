---
id: BSB-00
title: Change-scoped prime prompt for change sessions
---

# BSB-00: Change-scoped prime prompt for change sessions

Status: see [../ledger.md](../ledger.md).

## Objective

Prime every newly created change session with instructions that bind it to its
change, so the agent keeps all requested work inside that change instead of
scaffolding a new one.

## Dependencies

—

## Scope

- Add `changePrompt(changeID, title string) string` in
  `internal/server/changesession.go` (next to `discussionPrompt`).
- Call it from `createChangeSession` in `internal/server/mapping.go` via
  `s.oc.Prompt` immediately after session creation; on prime failure, delete
  the opencode session and return 502 (same pattern as the discussion flow).
- Unit tests.

## Implementation steps

1. Write `changePrompt`: states the binding (`<id> — <title>`); instructs the
   agent to read `changes/<id>/plan.md` and `changes/<id>/ledger.md` first;
   every request in this conversation is work on this change; refine the plan
   and add task files + ledger rows under the existing ID prefix per AGENTS.md;
   never create a new change directory and never call `/changes/scaffold`;
   for genuinely unrelated work, ask the user to start a new discussion from
   the index page.
2. In `createChangeSession`, after `CreateSession` succeeds and before the
   mapping add, call `s.oc.Prompt(ctx, sess.ID, changePrompt(id, title))`.
   On error: `s.oc.DeleteSession` (background context) and return
   502 `"prime change session: ..."`.
3. Tests: prompt contains the change ID/title, the no-scaffold prohibition,
   and the existing-prefix instruction; `createChangeSession` primes the
   session (captured by the fake client) and a prime failure cleans up the
   session and returns 502.

## Verification

- `go vet ./...` and `go test ./internal/server/` green.

## Completion criteria

- New change sessions are primed with the change-scoped prompt; prime failure
  leaves no session behind; tests cover both.

## Files affected

- `internal/server/changesession.go`
- `internal/server/mapping.go`
- `internal/server/changesession_test.go` (and/or mapping/session test file)

## Notes

- The prompt builder lives beside `discussionPrompt` for discoverability even
  though its only caller sits in `mapping.go`.
- Verified 2026-09-12: `go vet ./...` + `go test ./...` green;
  `TestCreateSessionEndpoint` asserts the bound prompt content,
  `TestCreateSessionPrimeFailure` asserts the session is deleted on prime
  failure. Live: session `ses_f6933b382ffe4E50xSxvi3wyp1` created via
  `POST /changes/2026-09-12-10/sessions` carried the prompt as its first
  message and answered scoped to the change.
