# RVT-02: End-to-end verification pass

## Why

The revert flow spans a new endpoint, template markup, and chat state handling; a final pass verifies the assembled behavior and the repository contract before handing the change to the user.

## What

- Full build and checks: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, `go vet ./...`, `go test ./...`, `lessmess validate`.
- Exercise the flow against the running server (rebuild + restart so embedded assets are live): one-click revert happy path, files-flag on and off, busy gating, capability-gated service messaging, stale-stage recovery after a forced failure.
- Confirm no regression in sibling lifecycle flows (fork, compact) and that the Controls drawer no longer opens on revert.

## Files affected

- None (verification only; fixes land in the owning task's files).

## Verification

- All checks green with evidence noted on this task; the acceptance criteria in `plan.md` each observed or explicitly waived by the user.
