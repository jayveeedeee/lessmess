---
id: FORM-00
title: Remove board add-task form from UI
---

# FORM-00: Remove board add-task form from UI

Status: see [../ledger.md](../ledger.md).

## Objective

Remove the inline add-task/subtask form (title input + "Add task"/"Add subtask" button) from the change board header, and clean up everything that only existed to serve it.

## Dependencies

None.

## Scope

- `web/templates/board.html`: delete the `<form class="inline-form" hx-post="/changes/{{.Data.ID}}/tasks">…</form>` block (lines 32–37).
- `web/static/app.js`: delete the `form[hx-post*="/tasks"]` branch in the global `htmx:afterRequest` handler (~line 87); keep the failure-alert branch and the `form[hx-post="/changes/session"]` branch untouched.
- `internal/server/render_nested_test.go`: remove the `New task title` assertion from the root-board want list, and the `name="parent" value="FIX-00"` and `New subtask title` assertions from the drill-down want list; keep all other assertions.
- `README.md`: remove/adjust the "Add task / New change" bullet and the "Add subtask" drill-down sentence.

Out of scope: the `POST /changes/{id}/tasks` endpoint, `.inline-form` CSS (still used by `index.html`), all other board UI.

## Implementation steps

1. Edit `web/templates/board.html`: remove the form block from `.board-head`.
2. Edit `web/static/app.js`: remove the dead afterRequest branch for `/tasks` form posts, keeping the surrounding branches intact.
3. Edit `internal/server/render_nested_test.go`: drop the three form assertions; leave the remaining root-board and drill-down assertions as-is.
4. Edit `README.md`: drop the board add-task/subtask mentions.
5. Rebuild and smoke-check: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, restart the server, open a change board root and a task drill-down.

## Verification

- `go vet ./...` and `go test ./...` pass from the repo root.
- `lessmess validate` is clean.
- Rendered board root and drill-down contain no `inline-form` and no "Add task"/"Add subtask" button; breadcrumbs, status select, rollup pill, and cards unchanged.
- `grep -rn "inline-form" web/templates` matches only `index.html`.

## Completion criteria

- Board header renders no add-task/subtask form at any board level.
- Tests and vet pass; README no longer describes the removed form.

## Files affected

- `web/templates/board.html`
- `web/static/app.js`
- `internal/server/render_nested_test.go`
- `README.md`

## Notes

- Templates and static assets are embedded at build time — the change is invisible in a running server until the binary is rebuilt and restarted.
- Endpoint `POST /changes/{id}/tasks` intentionally retained as API surface (user decision, UI-only removal).
