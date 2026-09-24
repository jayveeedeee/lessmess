# UI-04: Remove the kanban board and reroute /changes/{id}

## Why

Once the Work panel (UI-00), Sessions sheet with change actions (UI-01), breadcrumb resume
(UI-02), and card tap-to-resume (UI-03) exist, the kanban board is fully redundant. The user
decided to remove it outright; deep links into it must land in the new chat-first flow.

## Current behavior

- `GET /changes/{id}` serves `board.html` (kanban + header buttons + sessions panel + worktree
  strip) or the `boardFragment` partial for htmx; drag-and-drop via SortableJS; SSE refreshes
  re-request the fragment; `?task=` renders drill-down sub-boards; `⤢` posts
  `POST /changes/{id}/expand` (the only UI entry to decomposition); `/changes/{id}?session=<sid>`
  auto-opens a session; `followSession` navigates there after a discussion scaffolds.
- `POST /changes` (API-only) still sets `HX-Redirect` to `/changes/{id}`.

## Target behavior

- Delete board templates and board-only client code: `board.html`, `boardFragment` (its last
  consumer went with UI-00), `initSortable` + `static/Sortable.min.js` + its VENDOR entry and
  layout script tag, `refreshBoard` and the board SSE branch, `boardTask()` scoping (resume keys
  become change-scoped `tt-last-session:<change>`), the board sessions panel/spawn form (moved in
  UI-01), Continue/Commit/Close buttons (moved in UI-01/02), and the kanban CSS blocks.
- Route: HTML requests to `GET /changes/{id}` redirect (302) to `/?change=<id>` (preserving
  `?session=` as `&session=` so auto-open still reconnects the exact session); `?task=` deep links
  redirect to the change root (subtask scope lives in the Work panel). The JSON API shape of
  `GET /changes/{id}` survives unchanged for API clients.
- Index client honors `?change=<id>`: enters that change's context (trail crumb) and resumes its
  session (replacing `autoOpenSession`); `followSession` assigns the new deep link so a
  scaffolding discussion reconnects in place; `POST /changes`'s HX-Redirect target updated to
  match.
- Decompose affordance: a "Decompose" button in the task detail modal posts
  `POST /changes/{id}/expand` (409 "container exists" surfaces as a message and opens the
  subtasks) — preserving the user-instructed-only decomposition rule.
- Docs/comments that describe board behavior (web doc pairs, README section on the board) are
  updated; the close-out gardener refreshes doc pairs afterwards regardless.

## Coordination

- Change 2026-09-21-aw36x actively reshapes the same files (its MAC-76 composer-control work is
  uncommitted at planning time; its remaining open tasks are release/packaging only). Start this
  task only after MAC-76 lands, and rebase on whatever aw36x committed by then — its board-header
  title work (task 63) is superseded by this removal.

## Verification

- `grep -ri 'board' web/templates web/static/app.js` finds no kanban remnants (allowed: the Work
  panel's internal naming); Sortable is no longer loaded.
- `/changes/<id>` in a browser lands on the list with that change's chat resuming; `?session=`
  reopens the exact session; the JSON endpoint still serves `boardResponse` fields.
- Decompose from the task modal creates the container and the Work panel scopes to its subtasks.
- `go vet ./... && go test ./...` green with board render tests replaced by card/sheet/redirect
  tests; `lessmess validate` clean.
