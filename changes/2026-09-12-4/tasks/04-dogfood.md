---
id: LIF-04
title: Dogfood
---

# LIF-04: Dogfood

Status: see [../ledger.md](../ledger.md).

## Objective

Verify the lifecycle features live: close/reopen round trip and a real agent-driven git commit.

## Dependencies

LIF-03.

## Scope

In scope: throwaway change → close (both ledgers `Done`, validator consistent) → reopen (back to `In progress`) → cleanup; commit flow on the real repo producing a real commit (message written by the agent); full suite + validate.
Out of scope: new features.

## Implementation steps

1. Close/reopen a throwaway change via endpoints; verify both ledgers each time; clean up.
2. Run the commit flow from a board; watch the agent commit in the terminal/WS; confirm via `git log`.
3. Record evidence; final `go vet`, `go test ./...`, `tasktracker validate`.

## Verification

- Plan acceptance criteria 1–5 all have recorded evidence.

## Completion criteria

- All criteria pass with evidence in the notes.

## Files affected

- None beyond notes (cleanup of throwaways).

## Notes

- Evidence (2026-09-12):
  1. AGENTS.md amendment — rule 8 (user-only close) + rule 9 (reopen) + handoff alignment; validator consistent.
  2. Close/reopen round trip — `SetChangeStatus` wrote both ledgers atomically (unit test + live round trip on 2026-09-12-2, restored to `Done` afterward); board button is state-aware (screenshots: "Close change" on In progress, "Reopen" on Done; confirm dialog for incomplete tasks in app.js).
  3. Commit button — `POST /changes/{id}/commit` creates, primes, and maps a commit session; the terminal opens on it.
  4. Live commit — against a throwaway git repo seeded with uncommitted changes: the agent reviewed status/diff, wrote a structured message (summary + body), staged, and committed ~50s after the POST (`82b0c1f "Dogfood the commit flow with uncommitted test changes"`); working tree clean; no push/amend; session deleted after (204).
  5. `go vet` + `go test ./...` green; `tasktracker validate` OK after cleanup (throwaway repo removed).
- Residual: the commit flow depends on the agent faithfully honoring "commit only" — guarded by the prompt and the repo's `opencode.json` (`git push` denied); the user watches it live in the terminal by design.

