# PILL-02: Verify pill end-to-end

## Why

The web UI is embedded at build time, so template/static changes are invisible
until the binary is rebuilt and restarted; end-to-end verification must run
against the rebuilt binary, not just the source tree.

## What

- Rebuild with `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
- `go vet ./... && go test ./...` from the repo root.
- Update `internal/server/render_test.go`'s composer and inbox asset contract
  assertions to pin the new three-part action and both delivery choices.
- Manual browser pass across composer states: idle send, busy stop, busy +
  content pill expansion (stop/steer/queue + dividers, animation, reduced
  motion), queue and steer submits reflecting in `#chat-inbox`, collapse on
  draft clear and run end, ≤840px width, mutation lock disabling segments.

## Files affected

- `internal/server/render_test.go` (the embedded UI contract tests); other
  fixes land in the files from PILL-00/PILL-01.

## Verification

- `go vet ./...` and `go test ./...` clean; rebuilt binary serves the pill and
  every acceptance-criteria state in the plan checks out in the browser.
