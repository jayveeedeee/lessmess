
# NTD-04: Server nested routes, expand, drill-down board

Status: see [../ledger.md](../ledger.md).

## Objective

Serve the nested structure: task detail for any depth, the expand endpoint, drill-down boards, and recursive counts — without sessions (NTD-05) or UI polish (NTD-06).

## Dependencies

NTD-03

## Scope

- `internal/server`: routes, board view/fragment data, expand endpoint, recursive summaries. No prompt/session changes.

## Implementation steps

1. Replace `GET /changes/{id}/tasks/{file}` with `GET /changes/{id}/tasks/{file...}`; keep `Store.TaskFile`'s path guard (no `..`, no absolutes) and add a route test for traversal rejection.
2. Add `POST /changes/{id}/expand` with `{"task": "<id>"}`: calls `DecomposeTask`; on success returns the created container path; 409 when it already exists.
3. Extend `POST /changes/{id}/tasks` with an optional `parent` task ID (empty = change root).
4. Board reads `?task=<id>`: the kanban fragment renders that node's children (same column model, statuses, drag targets scoped to the governing ledger), plus a breadcrumb from the root change through ancestors; the header shows the node's rollup badge and the change's overall progress. Unknown/deep task ID → 404.
5. `MoveTask` endpoint accepts dotted IDs (governing-ledger resolution happens in the store).
6. `changeSummary`/index and the JSON board response expose recursive task counts (`x/y` complete) via `SubtreeStats`.
7. Wire the recursive close-out gate into the close handler using `CloseOutReady`, listing offending task IDs in the refusal.

## Verification

- `go test ./internal/server/` green: nested detail HTML/JSON, traversal rejection, expand (201/409), parented create, drill-down fragment with breadcrumb, dotted move, recursive counts, close blocked by a grandchild and allowed on a complete tree.
- `render_test.go` expectations updated for new markup hooks (badges, breadcrumb) with NTD-06 consumers in mind.

## Completion criteria

Every nested structure is fully servable and close-out is correctly gated; the API is complete enough for NTD-05/NTD-06 to build on without further route changes.

## Files affected

- `internal/server/server.go`
- `internal/server/lifecycle.go`
- `web/templates/board.html`, `web/templates/partials.html` (fragment data only; visual polish in NTD-06)
- `internal/server/render_test.go`, `internal/server/server_test.go`

## Notes

Keep the board DOM shape (`#board`, `.cards[data-status]`, `.card[data-task]`) stable — the terminal task panel mirrors it client-side.
