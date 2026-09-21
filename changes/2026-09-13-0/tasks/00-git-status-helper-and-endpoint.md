
# CMT-00: Read-only git helper and status endpoint

Status: see [../ledger.md](../ledger.md).

## Objective

Give the server a strictly read-only way to report the repository's
uncommitted state, exposed as `GET /api/git/status`, so both the index
renderer (button enabled/disabled/hidden) and the commit modal (fresh
preview) can consume it.

## Dependencies

— (none; first task)

## Scope

- New file `internal/server/gitcommit.go`:
  - `gitFileStatus{Code, Path}` and `gitRepoStatus{Repo bool, Changes []gitFileStatus, Stat string}` types (with JSON tags).
  - `gitStatus(dir string) gitRepoStatus` helper using `os/exec`:
    1. `git -C dir rev-parse --is-inside-work-tree` — any failure (non-repo, missing git binary) → `Repo:false`.
    2. `git -C dir status --porcelain` — parse each line into two-letter status code + path (handle rename `R  old -> new` by keeping the raw remainder as the path display).
    3. `git -C dir diff --stat HEAD` — raw output string (covers staged+unstaged tracked changes; untracked files appear only in `Changes`).
  - Handler `gitStatusAPI`: `writeJSON(w, 200, gitStatus(s.st.Dir))`. Always 200; never errors to the client (degrades to `Repo:false`).
- `internal/server/server.go`: register `mux.HandleFunc("GET /api/git/status", s.gitStatusAPI)`.
- New file `internal/server/gitcommit_test.go` covering the helper and endpoint.

## Implementation steps

1. Create `gitcommit.go` with the types, helper, and handler. Use short `exec.CommandContext` timeouts (5 s) so a wedged git cannot hang a request.
2. Register the route in `server.go` next to the other `/api/*` routes.
3. Tests in `gitcommit_test.go`:
   - helper on a `t.TempDir()` that is not a git repo → `Repo:false`;
   - helper on a temp git repo (skip if `git` binary missing): init, configure user, commit one file, then modify it + add an untracked file → expect `Repo:true`, an `M` entry and a `??` entry, non-empty `Stat`;
   - endpoint via fixture store (`store.New(t.TempDir())` style, matching existing server tests) → 200 JSON with `repo:false` for a non-repo dir.

## Verification

- `go test ./internal/server/ -run 'TestGitStatus' -v`
- `go vet ./internal/server/`

## Completion criteria

- `GET /api/git/status` returns `{"repo":false}` outside a git repo and a
  full change list + diffstat inside a dirty one; all new tests pass.

## Files affected

- `internal/server/gitcommit.go` (new)
- `internal/server/gitcommit_test.go` (new)
- `internal/server/server.go` (route registration)

## Notes

- `git diff --stat HEAD` covers staged+unstaged tracked changes; untracked files appear only in `Changes`. On a repo with no commits the helper falls back to `git diff --stat --cached`. Tests verified all three repo states (non-repo, dirty, clean); `go vet ./...` and full `go test ./...` pass.
