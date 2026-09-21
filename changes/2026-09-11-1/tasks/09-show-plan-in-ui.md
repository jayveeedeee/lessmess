
# KAN-09: Show plan content in the UI

Status: see [../ledger.md](../ledger.md).

## Objective

Expose each change's `plan.md` in the UI: a Plan button on the change list (and on the board header) opens the rendered plan in the centered modal.

## Dependencies

KAN-08 (modal presentation established).

## Scope

In scope: `GET /changes/{id}/plan` route (HTML modal partial + JSON); store method to read `plan.md`; Plan button per row on the index change list and in the board header; reuse of the existing modal/prose styling.
Out of scope: plan editing, plan templates changes.

## Implementation steps

1. Store: `PlanFile(changeID)` returning raw plan markdown (path-guarded, fixed filename).
2. Server: route + handler rendering a `planDetail` partial (goldmark) for HTML/HX, JSON otherwise.
3. Templates: `planDetail` partial in `partials.html`; Plan button in the index table and the board header; add the `#detail` container to the index page.
4. Tests for the route (JSON + HTML + 404); build; screenshot-verify the modal.

## Verification

- New route tests pass; `go test ./...` green.
- Headless-Chrome screenshot of the plan modal inspected.

## Completion criteria

- Plan opens in the modal from both the change list and the board; tests pass.

## Files affected

- `internal/store/store.go`
- `internal/server/server.go`, `render.go`
- `web/templates/partials.html`, `index.html`, `board.html`

## Notes

- None yet.
