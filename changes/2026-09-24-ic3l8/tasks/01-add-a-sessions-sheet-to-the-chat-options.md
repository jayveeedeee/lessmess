# UI-01: Add a Sessions quick action and sheet to the chat composer

## Why

With the kanban board removed, every board-page function needs a home. The user decided: session
switching AND change-level actions live in a "Sessions" sheet opened from the chat composer's
options area — one thumb-friendly surface. Since planning, change 2026-09-21-aw36x (commit
be42a6c) replaced the ⋮ pop-up menu with an in-composer quick-action grid, so the entry point is a
new quick action in that grid.

## Current behavior

- The composer's options expand in place as a quick-action grid (`#chat-more-menu` inside
  `.chat-compose-row`, toggled by `#chat-more-btn` which carries the context-usage ring and flips
  to a Close treatment while open): Plan (`#chat-plan-btn`), Tasks (`#chat-tasks-btn`), Runtime
  (`#chat-controls-btn`), Compact (`#chat-compact-btn`). Plan and Tasks ship `hidden` and app.js
  unhides them when the session is change-bound.
- Panels open in the chat shell's `data-chat-view` sheet pattern (`#chat-tasks`, `#chat-agents`,
  `#chat-controls-sheet` — the latter now split into a compact Runtime variant plus
  `.chat-controls-extended` sections).
- Sessions management lives on the board page: `#sessions-panel` (list with Chat/unlink, New
  session), the Spawn-change form (handoff picker), the Continue button, Commit, Close/Reopen, and
  the worktree strip (branch, uncommitted, missing, PR link, review link, Remove worktree).
- Subagent sessions render as chips on task cards; parent-only ("stray") sessions land in
  `#board-subs`.

## Target behavior

- New "Sessions" quick action in the quick-action grid — an icon+label button
  (`#chat-sessions-btn`, `role="menuitem"` like its siblings), shipping `hidden` and unhidden by
  app.js exactly when the Plan/Tasks actions are (session bound to a change) — opening a new sheet
  `#chat-sessions-sheet` in the existing `data-chat-view` pattern (backdrop, Escape, view stack,
  inert handling all reused).
- Sheet content, fed by `GET /changes/{id}/sessions` (change resolved via
  `/api/sessions/{id}/change`):
  - Session list: title, created date, live/dead dimming, `from <change>` spawn badge, task chip
    for task-bound sessions (subagent sessions included — they replace the card chips and
    `#board-subs` strip). Tapping a session switches to it (`openChat` + `markOpened`).
  - "New session" button (`POST /changes/{id}/sessions`) and unlink action (DELETE, with confirm)
    per row.
  - Change actions footer: Commit (`POST /changes/{id}/commit` + `commit-status` poll),
    Close change / Reopen (client-side confirm reusing the open-task warning before posting),
    worktree pills (branch, uncommitted, missing, PR link, review link opening `#detail`), and
    Remove worktree (refused while dirty — show the server error).
- Revised on user review: the spawn-change handoff picker was built into the sheet and then
  removed — the flow stays agent-driven via `POST /changes/{id}/spawn-change` (the workflow
  prime teaches it), with no UI affordance.
- Works on narrow viewports first: full-width sheet, ≥44px rows.

## Verification

- From a change-bound chat at 390px width, every former board action is reachable from the sheet:
  switch/new/unlink session, spawn change via handoff, commit, close (confirm warns while tasks
  are open), reopen, worktree info/remove/review.
- A discussion (unbound) session shows no Sessions quick action.
- Spawned-from badges and dead-session dimming render as on the old panel.
- `go vet ./... && go test ./...` pass.
