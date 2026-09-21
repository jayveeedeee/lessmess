# Agent notes: gitops

<!-- tasktracker:begin -->
## Learnings

- This package is the one sanctioned place for mutating git/gh execution in Go; every other git touchpoint (e.g. `server/gitcommit.go`) stays read-only, and commits happen inside primed opencode sessions — do not add write-path git calls elsewhere.
- `Client` (gitops.go) shells out to the real git binary via the injectable `Commander` seam; tests either build real temp repositories (`realClient` in gitops_test.go) or substitute scripted fakes with `WithCommander`. Sentinel errors `ErrBranchExists`, `ErrNoRemote`, `ErrGHUnavailable`, `ErrDirty`, and `ErrNotFound` are the API callers branch on.
- `WorktreeDir` puts per-change worktrees at `<parent>/<repo>-worktrees/<id>` — deliberately outside the served tree so git status, docs walks, and store globs never see them; `CopyProjectConfig` clones `opencode.json` into each worktree so worktree-scoped sessions keep the same permission allowlist.
- state.go persists `.lessmess/worktrees.json` (`WorktreesState` keyed by change ID) but git's `worktree list` remains the truth about existence — entries are hints for PR URL, review state, branch, and path, and callers must probe `WorktreeList` before trusting `Path`. Unlike fail-open state elsewhere, a malformed worktrees.json is a hard error, and an empty `StateDir` disables persistence.
- PR bodies are caller-built files passed by path (`CreatePR`, `CommentPR` use `--body-file`); the package never composes markdown itself, and `Push` fails fast with `ErrNoRemote` before any network call when origin is absent.
<!-- tasktracker:end -->
