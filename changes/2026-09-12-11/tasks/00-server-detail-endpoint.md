
# EXD-00: Detail endpoint and route

Status: see [../ledger.md](../ledger.md).

## Objective

Add a server handler that renders the detail fragment for one covered
directory, so the explorer's right pane can be populated by htmx.

## Dependencies

— (none)

## Scope

- `internal/server/explorer.go`: new `explorerDetail` handler plus a helper
  that finds a node by relative path in the walked tree.
- `internal/server/server.go`: register `GET /explorer/detail`.
- No template work here beyond referencing the `explorerDetail` template name
  (template authored in EXD-01; coordinate on the name).

## Implementation steps

1. Add a `findExplorerNode(root *explorerNode, rel string) *explorerNode`
   helper (treat `"."`/root as the root node).
2. Add `func (s *Server) explorerDetail(w http.ResponseWriter, r *http.Request)`:
   - 503 when `s.docsQ == nil`.
   - Parse `dir` query param, clean it like `explorerChat` (`.` default).
   - 422 when the dir fails `s.docsQ.cfg.Covered` or does not exist on disk.
   - Build the explorer view, find the node, `slog.Warn` + 404 when absent,
     otherwise render the `explorerDetail` template with the node.
3. Register `mux.HandleFunc("GET /explorer/detail", s.explorerDetail)` in
   `server.go` beside the other explorer routes.
4. Add a server test (mirroring existing explorer tests, if any) covering:
   fragment 200 for a covered dir, 422 for uncovered, 503 when docs disabled.

## Verification

- `go vet ./... && go test ./internal/server/...` pass.
- Manual curl: `curl 'http://127.0.0.1:9090/explorer/detail?dir=internal'`
  returns HTML containing that directory's purpose and file blurbs once
  EXD-01's template exists (until then, a template error is expected — defer
  the curl check to EXD-01 if needed).

## Completion criteria

- Route registered; handler returns the fragment for covered dirs and the
  documented error statuses otherwise; tests pass.

## Files affected

- `internal/server/explorer.go`
- `internal/server/server.go`
- `internal/server/explorer_test.go` (or nearest existing explorer test file)

## Notes

- Reused `buildExplorerView`'s walk per request; no caching (same profile as
  `/explorer/tree`).
- `findExplorerNode` treats `"."` as the root node; the handler renders the
  `explorerDetail` template via the `partial` set (authored in EXD-01).
- Verified 2026-09-12: `go vet` clean; `TestFindExplorerNode`,
  `TestExplorerDetailRejectsBadDirs`, `TestExplorerDetailDocsDisabled` pass;
  200-fragment coverage added in EXD-01 (`TestExplorerDetailEndpoint`).
