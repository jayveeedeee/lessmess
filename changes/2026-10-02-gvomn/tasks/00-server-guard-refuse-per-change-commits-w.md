# CMG-00: Server guard: refuse per-change commits without a live worktree

## Why

`commitChange` currently primes the commit session in `changeSessionDir(id)`,
which is the main tree whenever the change has no live worktree. The prompt's
`git add -A` then sweeps all uncommitted work — other in-flight changes,
manual edits — into one commit attributed to this change. The endpoint needs
the same gate the UI is getting: no live worktree, no per-change commit.

## What

In `internal/server/lifecycle.go`'s `commitChange`, after the existing
change/opencode/mapping guards, add the worktree gate mirroring
`worktreeRemove` (internal/server/worktree.go) verbatim in code and wording:

- `!s.worktreesEnabled()` → `409` `"the worktree pipeline is disabled"`.
- else `s.worktreeEntry(id)` miss → `404` `"no live worktree registered for
  change " + id`.

`worktreeEntry` is the same self-healing probe `changeSessionDir` uses, so
the endpoint verdict and the session directory can never disagree.

Tests in `internal/server/lifecycle_test.go`: the existing 201 test runs on
a non-worktree fixture — repurpose it to assert the new refusal (its 201
coverage already exists in `worktree_test.go` on a worktree fixture). Add:

- feature off → 409 with that exact message.
- feature on, no registered entry → 404 with that exact message.

## Files affected

- `internal/server/lifecycle.go` (new)
- `internal/server/lifecycle_test.go` (new)

## Verification

- `go test ./internal/server/ -run 'TestCommitChange|Lifecycle' -v` shows
  the 409 and 404 cases passing and the repurposed case asserting refusal.
- `go test ./...` green — `worktree_test.go`'s 201 path unchanged.
