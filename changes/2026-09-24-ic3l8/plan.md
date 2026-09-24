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
- Chat overlay: transcript + composer with a ⋮ menu (Work / Agents / Controls / Compact); the Work
  panel has no backend — it mirrors the board's HTML fragment by scraping card DOM from
  `GET /changes/{id}`.
- Breadcrumb dropdown (location menu): trail-only navigation; the change crumb exists only while a
  change-bound chat is open.
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
- **⋮ Sessions sheet**: session list (switch/new/unlink, subagent + spawned-from badges), the
  spawn-change handoff flow, and the change actions footer: Commit, Close/Reopen (open-task
  confirm), worktree pills/remove/review.
- **Breadcrumb dropdown**: a "Resume session" action pinned at the bottom whenever a change context
  exists, sharing the same resume helper as the cards; the change crumb persists for the visit.
- **Responsive sweep**: full-screen modals, ≥44px touch targets, safe-area insets, explorer /
  settings / setup / opencode-page polish at phone widths; desktop functionally untouched.

## Scope

- `web/templates/` (index, board removal, chat sheets, task modal), `web/static/app.js`,
  `web/static/app.css`, vendored Sortable removal.
- `internal/server/`: new tasks JSON endpoint, board route → redirect for HTML, index JSON stays,
  render tests updated; no workflow-state semantics change.
- `README.md` board mentions updated.

## Non-goals

- No changes to workflow rules, statuses, or store semantics; no new lifecycle endpoints beyond the
  read-only tasks JSON feed.
- Desktop is not redesigned — it must keep working; only the card list is new there.
- Terminal overlay behavior unchanged; explorer/settings features unchanged (responsive CSS only).
- `POST /changes` stays API-only; no task-creation forms are resurrected.

## Design decisions

- Chat is the change surface (user decision): the board page dies rather than slimming down.
- Card contents are exactly the four user-named fields; no visible change ID.
- Change-level actions live inside the ⋮ Sessions sheet (user decision), not on cards or a hub page.
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
