
# JSI-05: Board and detail views from JSON

Status: see [../ledger.md](../ledger.md).

## Objective

Rework the web UI's document views for the JSON world: ledger detail pages become generated views rendered from state, task detail composes JSON identity with the markdown prose body, and the index/board keep their current shape over the new store.

## Dependencies

JSI-02, JSI-03

## Scope

- Replace the raw `ledger.md` detail handlers (`PlanFile`/`LedgerFile`/`ContainerLedgerFile` readers) with a generated "state view" per change: task table (status, deps, updated, notes), decision log, overall status with derived badge — rendered through the shared goldmark/HTML pipeline and served with the same routing (`wantsHTML`/`isHX` branching preserved).
- Task detail view: JSON metadata header + rendered prose body from the referenced md file (keep `dropLeadingH1`-style presentation trims server-side).
- Plan detail: unchanged (still the md file).
- Index page and board: verify against the new store (statusRank, newest-first ordering, rollups); adjust any template assumptions about ledger files.
- Remove dead store readers once no consumer uses them; update link targets that pointed at `ledger.md`.

## Implementation steps

1. Implement the state-view renderer (server-side, from `ChangeState`).
2. Rework detail routes/templates; keep partial-vs-JSON responses.
3. Audit templates and `app.js` for ledger-file assumptions (links, TOC, md interception) and adjust.
4. View tests: golden HTML for state view and task detail; index/board tests ported.

## Verification

`go vet ./... && go test ./...` green; manual pass on the migrated repo: board, drill-down boards, breadcrumbs, detail modals, sortable index, docs refresh unaffected.

## Completion criteria

All document views render correctly from JSON + prose with no references to ledger md files; dead readers removed.

## Files affected

- `internal/server/` (detail handlers, render.go, tests), `internal/store/` (reader removal), `web/templates/`, `web/static/app.js`

## Notes

- The core landed with JSI-02: the ledger detail routes serve deterministic generated-markdown views rendered from the JSON state (`internal/store/view.go`), so the existing modal pipeline, `app.js` link mapping, and detail tests kept working unchanged; task detail composes JSON identity (via `TaskFile`) with the raw prose body.
- This task closed the remainder: stale template copy (setup wizard's bootstrap list, settings' branch help) updated to the JSON model; view coverage confirmed via the existing golden-ish tests (`TestLedgerDetail`, `TestContainerLedgerEndpoint`, `TestNestedBoardMarkup`, `TestNestedTaskDetail`, `TestReviewDetailModal`).
- No dead store readers remained: `PlanFile`, `TaskFile`, `LedgerFile`, `ContainerLedgerFile` all serve live consumers.
- Verification: `go vet ./... && go test ./...` green (2026-09-18).