# 2026-09-24-ic3l8: Mobile-first chat-first UI

- Change ID: 2026-09-24-ic3l8
- Created: 2026-09-24
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Make the web UI mobile-first: phones are the primary target, desktop keeps working unchanged in
function. The chat overlay becomes the surface where change work happens; the kanban board page is
removed outright. The changes page becomes a simple card list. Session resume and switching move
into the two menus the user already touches: the header's breadcrumb dropdown and the chat
composer's ⋮ options menu.

## Current behavior

- Changes page (`/`): a 7-column sortable table (ID, Title, Prefix, Status, Tasks, Updated, Plan
  button) with no mobile layout, plus Discussions, Commit all, and "New change session".
- Change page (`/changes/{id}`): a kanban board (drag-and-drop status moves, `?task=` drill-down
  sub-boards) carrying every lifecycle control: Continue session, Plan, Sessions panel (list,
  new, unlink, spawn-change handoff picker), Commit, Close/Reopen, worktree strip, subagent chips.
- Chat overlay (as reshaped by change 2026-09-21-aw36x, commit be42a6c): the composer floats over
  the transcript and its options expand in place as a quick-action grid inside the composer —
  Plan / Tasks / Runtime / Compact (`#chat-more-menu`, toggled by `#chat-more-btn`, which carries
  the context-usage ring and flips to a Close treatment while open); send and interrupt are one
  stateful icon button. The Work panel still has no backend — it mirrors the board's HTML fragment
  by scraping card DOM from `GET /changes/{id}`.
- Breadcrumb dropdown (location menu): reworked as an app-bar location control — a back chevron
  (`#location-back`) plus the current-location dropdown listing the full trail; the change crumb
  exists only while a change-bound chat is open.
- SSE: index full-reloads on workflow events when no chat is open; boards refresh via fragments.

## Target behavior

- **Changes page**: card list — exactly title, status pill, `x/y` task count, updated date. Tapping
  a card resumes the change's session in the chat overlay (continue last-opened, else newest, else
  create). Compact persistent sort. In-place SSE refresh so an open chat is never interrupted.
- **No board**: `/changes/{id}` HTML redirects to `/?change=<id>` (preserving `?session=`), which
  enters the change context and resumes its session. The JSON API survives for API clients.
  Sortable/drag-drop die with the board; task status changes go through the task detail modal's
  select (with verification evidence for Test), decomposition moves to a "Decompose" button in that
  modal.
- **Work panel**: renders from a new JSON feed (`GET /changes/{id}/tasks[?task=]`) with subtask
  rollups — no board DOM dependency.
- **Sessions quick action + sheet**: a "Sessions" quick action joins the composer's quick-action
  grid (gated on change-bound sessions like Plan/Tasks) and opens a Sessions sheet: session list
  (switch/new/unlink, subagent + spawned-from badges) and the change actions footer: Commit,
  Close/Reopen (open-task confirm), worktree pills/remove/review. Spawning a change from a
  handoff artifact stays agent-driven via `POST /changes/{id}/spawn-change` — the UI affordance
  was built and then removed on user review (no UI for it).
- **Breadcrumb dropdown**: the trail shows the change (by name) only while that change's session
  is open in Chat — it is the current location (`Changes / <Change Name>`), panel and document
  crumbs stack above it, and "Chat" survives only for unbound discussion sessions. There is no
  persisted change state: closing the chat or returning to the list leaves just `Changes`, and
  re-entry is the change cards (both resume via the shared helper). (Two earlier iterations were
  removed on review: a standalone "Resume session" row, then a visit-persisted crumb — both read
  as stale, always-present state.)
- **Responsive sweep**: full-screen modals, ≥44px touch targets, safe-area insets, explorer /
  settings / setup / opencode-page polish at phone widths; desktop functionally untouched.

## Scope

- `web/templates/` (index, board removal, chat sheets, task modal), `web/static/app.js`,
  `web/static/app.css`, vendored Sortable removal.
- `internal/server/`: new tasks JSON endpoint, board route → redirect for HTML, index JSON stays,
  render tests updated; no workflow-state semantics change.
- `README.md` board mentions updated.
- Coordination: planned against the post-be42a6c chat surface (quick-action grid, floating
  composer, app-bar location control from change 2026-09-21-aw36x). That change's MAC-76
  (composer options-control centering) is still uncommitted in the working tree — implementation
  of the UI tasks here starts after it lands, and UI-04 rebases on whatever aw36x committed by
  then. Its remaining open tasks (MAC-20…24) are release/packaging only and don't touch `web/`.

## Non-goals

- No changes to workflow rules, statuses, or store semantics; no new lifecycle endpoints beyond the
  read-only tasks JSON feed.
- Desktop is not redesigned — it must keep working; only the card list is new there.
- Terminal overlay behavior unchanged; explorer/settings features unchanged (responsive CSS only).
- `POST /changes` stays API-only; no task-creation forms are resurrected.

## Design decisions

- Chat is the change surface (user decision): the board page dies rather than slimming down.
- Card contents are exactly the four user-named fields; no visible change ID.
- Change-level actions live in the Sessions sheet opened from a Sessions quick action in the
  composer's quick-action grid (user decision, adapted from the pre-be42a6c ⋮ menu to the
  quick-action grid aw36x shipped), not on cards or a hub page.
- Resume entry points: breadcrumb dropdown bottom (user decision) + card tap (user decision).
- Old `/changes/{id}` deep links redirect into the resume flow rather than 404ing.

## Acceptance criteria

- On a phone viewport every former board capability is reachable: resume/start session, switch
  sessions, new session, unlink, spawn change, commit, close/reopen (with open-task warning),
  worktree info/remove/review, plan reading, task status with evidence, decompose, subtask
  navigation.
- The kanban board, drag-and-drop, and Sortable are gone; `/changes/{id}` HTML lands in the
  chat-first flow.
- Changes list shows exactly the four fields, sorts with persistence, and tapping resumes.
- An open chat is never reloaded away by SSE events.
- `go vet ./... && go test ./...` and `lessmess validate` pass; render/server tests assert the new
  DOM and routes.

## Tasks

1. UI-00 — Give the chat Work panel its own JSON task feed
2. UI-01 — Add a Sessions sheet to the chat options menu with change actions
3. UI-02 — Add Resume session to the breadcrumb location menu
4. UI-03 — Redesign the changes list as mobile cards with tap-to-resume
5. UI-04 — Remove the kanban board and reroute /changes/{id}
6. UI-05 — Responsive polish pass across all pages

Order: UI-00 → UI-01 → UI-02 → UI-03 → UI-04 → UI-05. UI-04 (board removal) intentionally last
among the structural tasks: it is only safe once every board function has a new home.
