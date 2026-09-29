# CSF-02: Render tests and full verification

## Why

The templates contract is test-asserted (`internal/server/render_test.go` pins
`#change-sort` and the card `data-*` attributes), and the repo requires a clean
vet/test/validate pass before work is considered done.

## What

- Extend `internal/server/render_test.go` to assert the new
  `#change-filter-status` select (with the All + five statuses) and the
  `data-status` attribute on rendered change cards, in the style of the
  existing index-render assertions.
- Run the full gate: `go vet ./...`, `go test ./...`, `lessmess validate`.
- Rebuild the binary (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`) so
  the embedded assets actually serve the feature, and note the restart
  requirement for the running server.

## Files affected

- `internal/server/render_test.go`

## Verification

- `go vet ./...` and `go test ./...` pass; `lessmess validate` reports clean;
  the rebuilt binary serves the filter on the changes page.
