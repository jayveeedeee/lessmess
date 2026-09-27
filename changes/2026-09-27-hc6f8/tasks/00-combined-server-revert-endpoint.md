# RVT-00: Combined server revert endpoint

## Why

The one-click revert flow needs stage + commit to happen as a single server-side action. Chaining the two existing endpoints from the browser forces an extra lifecycle round-trip between them (commit's `confirmation` needs the post-stage `updated` timestamp) and leaves a staged revert sitting idle if the client dies mid-flow.

## What

- Add `POST /api/sessions/{sessionID}/revert` in `internal/server/sessionlifecycle.go`, registered in `internal/server/server.go`.
- Request body: the existing stage-style `confirmation` (`sessionID`, `updated`) plus `files` (bool, default false) — and `messageID` selecting the revert point.
- Handler order: validate confirmation + capabilities (`revertStage` then `revertCommit`) → `StageRevert` with the given messageID/files → re-read session state → `CommitRevert` using the fresh staged `messageID` and `updated`.
- Reuse the response shape conventions of the sibling revert routes; on the commit step failing, surface the error and the staged state (the Controls-drawer recovery box remains the cleanup path).
- Existing `/revert/stage`, `/revert/commit`, `/revert/clear`, `/revert/preview` routes stay untouched.

## Files affected

- `internal/server/sessionlifecycle.go`
- `internal/server/server.go`
- `internal/server/sessionlifecycle_test.go` (or the existing test file covering these routes)

## Verification

- `go vet ./...` and `go test ./internal/server/...` (or `go test ./...`) pass with new tests covering: happy path stage→commit; confirmation mismatch 409; missing capability 4xx; stage-ok/commit-fail leaves a stale stage the recovery path can clear.
