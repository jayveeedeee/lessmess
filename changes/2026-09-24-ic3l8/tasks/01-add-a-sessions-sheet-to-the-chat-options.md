# UI-01: Add a Sessions sheet to the chat options menu with change actions

## Why

With the kanban board removed, every board-page function needs a home. The user decided: session
switching AND change-level actions live in a "Sessions" sheet opened from the ⋮ options menu below
the chat input — one thumb-friendly surface.

## Current behavior

- The ⋮ menu (`#chat-more-menu`) offers Work / Agents / Controls / Compact; panels open in the
  chat shell's `data-chat-view` sheet pattern (`#chat-tasks`, `#chat-agents`, `#chat-controls-sheet`).
- Sessions management lives on the board page: `#sessions-panel` (list with Chat/unlink, New
  session), the Spawn-change form (handoff picker), the Continue button, Commit, Close/Reopen, and
  the worktree strip (branch, uncommitted, missing, PR link, review link, Remove worktree).
- Subagent sessions render as chips on task cards; parent-only ("stray") sessions land in
  `#board-subs`.

## Target behavior

- New "Sessions" item in `#chat-more-menu`, shown when the active session is bound to a change
  (same gating as the Work item), opening a new sheet `#chat-sessions-sheet` in the existing
  `data-chat-view` pattern (backdrop, Escape, view stack, inert handling all reused).
- Sheet content, fed by `GET /changes/{id}/sessions` (change resolved via
  `/api/sessions/{id}/change`):
  - Session list: title, created date, live/dead dimming, `from <change>` spawn badge, task chip
    for task-bound sessions (subagent sessions included — they replace the card chips and
    `#board-subs` strip). Tapping a session switches to it (`openChat` + `markOpened`).
  - "New session" button (`POST /changes/{id}/sessions`) and unlink action (DELETE, with confirm)
    per row.
  - Spawn change: the handoff picker flow (`GET /changes/{id}/handoffs`, `POST
    /changes/{id}/spawn-change` with title/prefix) moved over intact, errors surfaced inline.
  - Change actions footer: Commit (`POST /changes/{id}/commit` + `commit-status` poll),
    Close change / Reopen (client-side confirm reusing the open-task warning before posting),
    worktree pills (branch, uncommitted, missing, PR link, review link opening `#detail`), and
    Remove worktree (refused while dirty — show the server error).
- Works on narrow viewports first: full-width sheet, ≥44px rows.

## Verification

- From a change-bound chat at 390px width, every former board action is reachable from the sheet:
  switch/new/unlink session, spawn change via handoff, commit, close (confirm warns while tasks
  are open), reopen, worktree info/remove/review.
- A discussion (unbound) session shows no Sessions item.
- Spawned-from badges and dead-session dimming render as on the old panel.
- `go vet ./... && go test ./...` pass.
