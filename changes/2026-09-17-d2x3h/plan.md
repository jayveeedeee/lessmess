# 2026-09-17-d2x3h: Remove board add-task form

- Change ID: 2026-09-17-d2x3h
- Created: 2026-09-17
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

The change board header carries a small inline form — a "New task title" input and an "Add task" button ("Add subtask" / "New subtask title" when drilled into a task board) — that creates task files via `POST /changes/{id}/tasks` with an optional `parent` field. The user wants this mini task form removed from the change view; task creation belongs in agent sessions, not the board header.

## Current behavior

- `web/templates/board.html` renders `<form class="inline-form" hx-post="/changes/{{.Data.ID}}/tasks">` (hidden `parent` input, title input, submit button) at the end of `.board-head`, with labels switching on `.Data.Task`.
- `web/static/app.js` (~line 87) matches `form[hx-post*="/tasks"]` in the global `htmx:afterRequest` handler to reset the input and `refreshBoard()` after a successful add.
- `internal/server/render_nested_test.go` asserts `New task title`, `name="parent" value="FIX-00"`, and `New subtask title` in the rendered board markup.
- `README.md` mentions "Add task / New change" (line ~94) and the "Add subtask" drill-down behavior (line ~121).
- The `.inline-form` CSS rule in `web/static/app.css` is shared with the index page's "New change session" form.

## Target behavior

- The board header (root change and task drill-down) has no inline add-task/subtask form; all other header controls (status select, Continue session, Plan, Sessions, Commit, Close/Reopen) are unchanged.
- Task creation happens through opencode sessions or the API, as with other workflow files.

## Scope

- Remove the inline form block from `web/templates/board.html`.
- Remove the now-dead `form[hx-post*="/tasks"]` branch from the `htmx:afterRequest` handler in `web/static/app.js`.
- Update `internal/server/render_nested_test.go` to drop the form-related assertions.
- Tidy the `README.md` mentions of the board add-task form and Add-subtask behavior.
- Keep the `POST /changes/{id}/tasks` endpoint (API surface, mirrors the UIX precedent of keeping `POST /changes`).

## Non-goals

- Removing or changing the `POST /changes/{id}/tasks` handler or `store` task-creation functions.
- Any other board UI changes (layout, controls, cards, terminal task panel).
- CSS changes: `.inline-form` stays (still used by `web/templates/index.html`).

## Design decisions

- UI-only removal, chosen by the user; the endpoint remains for API clients, consistent with change 2026-09-12-2 (kept `POST /changes` when its form was removed).
- The dead JS branch is deleted rather than left in place so the afterRequest handler only describes live flows; the shared failure-alert branch and the `/changes/session` branch are untouched.
- Render-test assertions are updated, not deleted wholesale: breadcrumbs, rollup, and `data-task` assertions remain valid targets.

## Acceptance criteria

1. Board page (root and `?task=` drill-down) renders no add-task/subtask form; the rest of the header is unchanged.
2. `go vet ./...` and `go test ./...` pass from the repo root.
3. `lessmess validate` is clean.
4. `README.md` no longer advertises the board's add-task form.

## Tasks

1. [FORM-00: Remove board add-task form from UI](tasks/00-remove-add-task-form.md)
