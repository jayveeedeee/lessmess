# MAC-58: Change task status from the detail view

## Objective

Allow task status changes directly from the task detail view, especially on
mobile where drag and drop is not the primary interaction.

## Dependencies

- MAC-57.

## Scope

- Add all workflow statuses to the task detail view.
- Submit through the existing deterministic task-status endpoint.
- Request verification evidence only when selecting `Test` and preserve the
  UI-only `Done` gate.
- Refresh the board after a successful update without closing the detail view.

## Implementation steps

1. Include change identity and status options in the task detail projection.
2. Render one clean responsive status select above the task prose.
3. Save immediately on selection, show errors inline, and refresh cards.
4. Add rendering and JavaScript contracts and run full verification.

## Verification

- Task detail rendering and interaction contract tests.
- Full tests, vet, JavaScript syntax, build, workflow validation, and diff check.

## Completion criteria

A user can open any task, select its new status without a separate save action,
provide evidence when needed, and see the board update on desktop or mobile.

## Files affected

- `internal/server/server.go`
- `internal/server/render.go`
- `internal/server/render_test.go`
- `web/templates/partials.html`
- `web/static/app.js`
- `web/static/app.css`

## Notes

- The form reuses `POST /changes/{id}/tasks/{task}/status`; no second mutation
  path or relaxed workflow rule was introduced.
- The status selector exposes every canonical workflow state, requests evidence
  only for `Test`, saves on change, keeps the detail view open, and refreshes
  board cards after a successful update.
- Removed the duplicate header status pill and the separate Update button.
- Verified with focused render contracts, `go vet ./...`, `go test ./...`,
  JavaScript syntax, static build, workflow validation, diff check, and served
  HTML/asset checks.
