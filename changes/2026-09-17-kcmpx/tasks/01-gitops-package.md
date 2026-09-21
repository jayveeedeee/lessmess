
# WTP-01: internal/gitops — git mechanics and worktree state

Status: see [../ledger.md](../ledger.md).

## Objective

Create the `internal/gitops` package that owns every new git/`gh` operation —
branch creation, worktree add/list/remove, dirty checks, push, PR create and
comment — plus the `.lessmess/worktrees.json` state file. One auditable package
for all out-of-session git execution.

## Dependencies

None (WTP-00 is independent; WTP-02 consumes both).

## Scope

- Package `internal/gitops` importing only `lessmess/internal/model` and stdlib.
- Command execution behind an injectable seam (package-level `runner` func or
  interface) so tests drive real temp repos or fakes without network/gh.
- Worktree state file: `changeID → {branch, path, prURL, reviewState, created}`,
  written with `model.WriteFileAtomic`, read fail-open.

## Implementation steps

1. `Client` type constructed with the repo dir; methods:
   - `IsRepo() bool`, `CurrentBranch() (string, error)`
   - `CreateBranch(name, base string) error` (fails if the branch exists)
   - `WorktreeAdd(id, path, branch string) error`
   - `WorktreeList() ([]string, error)` (parse `git worktree list --porcelain`)
   - `WorktreeRemove(path string) error`, `WorktreeDirty(path string) (bool, error)`
   - `DirtyPaths() ([]string, error)` (main-tree `status --porcelain` paths,
     for scaffold's dirt warning)
   - `CopyProjectConfig(path string) error` (copy root `opencode.json` into the
     worktree; best-effort, logged)
   - `Push(branch string) error` (explicit `git push -u origin <branch>`)
   - `CreatePR(branch, title, bodyFile string) (url string, err error)` via
     `gh pr create --head <branch> --title … --body-file …`
   - `CommentPR(prURL, bodyFile string) error` via `gh pr comment`
2. State file API: `Load(dir)` / `Save(dir, map)` + per-change helpers
   `Get`/`Update`; missing file = empty state; malformed file = error surfaced
   to callers (never silently ignored for writes).
3. All commands run with short default timeouts and captured stderr; errors
   wrap stderr for actionable messages.
4. `Runner` seam: production runner shells out; tests inject a fake or use temp
   repos. `gh` methods additionally check `gh --version` availability and
   return a typed `ErrGHUnavailable`.

## Verification

- Table tests against real temp repos: init → branch create → worktree add →
  dirty check (touch a file) → clean after commit → worktree list/remove.
- Push/PR tested against a local bare remote for push; `gh` methods tested via
  the fake runner (arg construction, URL parse, ErrGHUnavailable path).
- State file round-trip, atomic replace, malformed-file error.
- `go vet ./internal/gitops/ && go test ./internal/gitops/`.

## Completion criteria

- All gitops operations exist with tests, no dependency on `server`/`store`,
  and the state file follows the one-file-per-feature tooling-state pattern.

## Files affected

- `internal/gitops/gitops.go`, `state.go`, `fakes_test.go`, `gitops_test.go` (new)

## Notes

- Exec confinement convention (`internal/AGENTS.md`) currently names three
  packages; this package is the sanctioned fourth — WTP-07 updates the docs.
- Push refuses explicitly when no remote is configured (`ErrNoRemote`) so the
  close gate (WTP-05) can produce a precise message.
