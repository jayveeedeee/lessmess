# QM-02: Verify queue recovery regressions and document the contract

## Why

Queue correctness spans service state, independently polled browser state, and explicit user actions. Verification must exercise behavior rather than only pinning JavaScript source strings.

## What

- Complete regression coverage for normal delivery, failed/interrupted runs, compaction, reconnect, capability absence, and same-item recovery races.
- Document explicit recovery and uncertainty semantics in README.
- Run full Go checks, executable client checks, and workflow validation; record limitations honestly. Do not restart a user's live server or resume the reported session without separate authorization.

## Files affected

- `README.md`
- Adapter, server, and executable client regression tests for QM-00/QM-01
- This change's plan/task verification prose

## Verification

`go vet ./...`, `go test ./...`, executable client test command, build to an approved temporary path, and lessmess validation. Tasks finish at Test, not Done.

### Evidence — 2026-10-03

- With the installed toolchain on PATH: `go vet ./...` and `go test ./...` pass, including the Node client wrapper.
- `node --test web/chat_queue_test.mjs`: 20/20 pass. `node --check web/static/app.js` passes.
- `CGO_ENABLED=0 go build` succeeds to an approved temporary binary; that binary's `validate --dir /Users/jd/GitHub/tasktracker` succeeds with no workflow violations. The API validator agrees. Docs warnings include existing queue confinement flags and tree-map staleness; machine-owned maps were not hand-edited.
- Focused `go test -race` for the outcome/snapshot/recovery/compaction/client tests passes. The broader race run passes server/web but reports the pre-existing unsynchronized counter in `TestWaitDoneRetriesTransportTimeout`, unchanged from HEAD; that unrelated fixture is left untouched.
- `gofmt -d` for affected Go files and `git diff --check` are clean.
- README documents same-item explicit recovery, stale-state warnings, Stop/reconnect behavior, and the client test command.
- The running server was not replaced or restarted, and no prompt or recovery mutation was sent to the reported ARC session. Embedded UI changes require rebuild/restart before live acceptance; no browser verification is claimed.
