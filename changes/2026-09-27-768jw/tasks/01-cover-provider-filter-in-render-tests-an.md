# PMF-01: Cover provider filter in render tests and verify build

## Why

`internal/server/render_test.go` asserts on specific JS snippets served to the browser; the new filter code should be pinned there, and the standard build/vet/test gates must pass before the change is ready.

## What

- Extend the render-test JS-snippet expectations to cover the new provider filter wiring (select id, provider population, combined filtering call).
- Run the full gates: `go vet ./...`, `go test ./...`, `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, and `lessmess validate`.

## Files affected

- `internal/server/render_test.go`

## Verification

- `go vet ./...` and `go test ./...` pass with the new assertions in place; `lessmess validate` reports no violations.
