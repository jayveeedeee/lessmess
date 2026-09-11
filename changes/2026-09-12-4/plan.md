# 2026-09-12-4: Explicit change lifecycle — close, reopen, commit

- Change ID: 2026-09-12-4
- Created: 2026-09-12
- Branch: main
- Status: see [ledger.md](ledger.md)

## Objective and context

Three lifecycle requirements from the user:

1. **Explicit close**: changes must not be marked `Done` when tasks complete — only the user closes a change, via a button. This amends the AGENTS.md status workflow (agents report "ready for close-out" instead of marking `Done`).
2. **Reopen**: a closed change can be reopened (`Done` → `In progress`).
3. **Commit button**: a board action that has an agent write the commit message and create the git commit for the current uncommitted work, watched live in the embedded terminal.

## Current behavior

AGENTS.md rule 8 lets an agent mark a change `Done` once tasks and acceptance criteria pass. No close/reopen actions exist in the UI. No commit flow exists.

## Target behavior

- AGENTS.md: only the user closes a change; when all non-cancelled tasks are `Done` and acceptance criteria pass, agents leave the change `In progress` and report ready for close-out. `Done` ↔ `In progress` reopen transitions are user actions, recorded in ledgers.
- Board header: **Close change** button (confirm dialog if tasks are incomplete) ↔ **Reopen** when closed; both update the per-change ledger overall status and the root-ledger row.
- Board header: **Commit** button → creates an opencode session primed to review uncommitted changes, write a commit message, and `git commit` (no push), mapped to the change and opened in the terminal so the user watches it happen.

## Scope

- AGENTS.md status-workflow amendment (user-only close, reopen).
- Store: `SetChangeStatus(id, status)` writing both ledgers atomically (per-change + root row), with reload + notify.
- Endpoints: `POST /changes/{id}/close`, `POST /changes/{id}/reopen`, `POST /changes/{id}/commit`.
- UI: Close/Reopen + Commit buttons in the board header; confirm logic; terminal auto-open for the commit session.
- Tests + live dogfood.

## Non-goals

- Server-side git operations (the agent runs git; tasktracker never commits itself).
- Cancel-flow UI (remains an editor action per AGENTS.md).
- Commit-message generation without an agent (no heuristic fallback).

## Acceptance criteria

1. AGENTS.md states user-only close + reopen; ledgers and validator remain consistent.
2. Close updates both ledgers to `Done`; Reopen returns them to `In progress`; board button reflects state.
3. Commit button creates a primed, mapped session and opens the terminal.
4. Live dogfood: close/reopen round trip; commit flow produces a real git commit.
5. `go test ./...` passes; `tasktracker validate` clean.

## Tasks

1. [LIF-00: AGENTS.md user-only close + reopen amendment](tasks/00-workflow-amendment.md)
2. [LIF-01: Store status transitions + close/reopen endpoints](tasks/01-status-endpoints.md)
3. [LIF-02: Commit-session endpoint](tasks/02-commit-endpoint.md)
4. [LIF-03: Board lifecycle buttons](tasks/03-lifecycle-buttons.md)
5. [LIF-04: Dogfood](tasks/04-dogfood.md)
