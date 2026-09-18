---
id: WTP-02
title: Scaffold creates branch, worktree, and worktree-side change docs
---

# WTP-02: Scaffold creates branch, worktree, and worktree-side change docs

Status: see [../ledger.md](../ledger.md).

## Objective

When `git.worktrees` is enabled, `POST /changes/scaffold` (and the handoff
spawn path) creates the change's branch and worktree first, creates
`changes/<id>/` inside the worktree, keeps the root-ledger row in the main
tree, and returns branch + worktree path so the planning session can continue.

## Dependencies

- WTP-00 (settings), WTP-01 (gitops).

## Scope

- `store.CreateChange` gains an explicit target directory for the change docs
  (main tree today; worktree when enabled) while the root-ledger write stays in
  the served dir — or gains an equivalent split API; keep the existing
  signature working for the disabled path.
- `scaffoldChange` and `spawnChange` in `internal/server/changesession.go`:
  run gitops before doc creation; on any gitops failure return 422/502 with an
  agent-facing message and create nothing (no partial change).
- Scaffold response body gains `branch` and `worktree` fields (both paths),
  plus an optional `warning` naming uncommitted main-tree files — dirt is
  allowed, never refused, but the worktree (cut from the committed base) won't
  contain it, so the agent must relay the list.
- Session rename/mapping unchanged; worktree registration recorded in
  `.lessmess/worktrees.json`.
- Feature disabled → exactly today's behavior, zero git calls.

## Implementation steps

1. Extend the store change-creation path with the split doc/root targets
   (WTP-03 finalizes resolution; this task only needs creation placement).
2. In `scaffoldChange`: read effective settings; if enabled —
   `IsRepo` → resolve base (`BaseBranch` else current branch) →
   check main-tree dirt via gitops (`DirtyPaths`, `status --porcelain`) and
   hold the list for the response →
   `CreateBranch(change/<id>)` → compute worktree path
   (`<parent>/<repo-basename>-worktrees/<id>`) → `WorktreeAdd` →
   `CopyProjectConfig` → create docs in the worktree → root ledger row in the
   main tree → record state. The dirt check runs before scaffold's own
   writes so the warning never lists the change's own root-ledger row.
3. Mirror the same sequence in `spawnChange` (handoff artifacts are read from
   the source change — which may itself live in a worktree; resolve via
   WTP-03's resolver once available).
4. Include `branch` + `worktree` in both JSON responses; extend the discussion
   prompt's rule 4/5 wording minimally so the agent knows the returned path is
   where the change's files live (full prompt work in WTP-07).
5. Failure handling: branch exists, worktree path taken, or any git error →
   fail the request before any workflow file is written; log with slog. Dirt
   is never a failure: it only populates `warning`.
6. Server wiring: construct the gitops client in `server.New` (nil when
   disabled or not a repo) following the `s.oc`/`s.docsQ` nil-disabled pattern.
7. httptest coverage: enabled-success (fake runner), disabled-noop, git-failure
   abort, dirty-main-tree success carrying `warning`, clean tree without one,
   idempotent retry behavior (409 on bound session unchanged).

## Verification

- Unit/httptest: all four cases above; response fields present.
- Manual smoke in this repo behind a scratch remote copy: scaffold via curl,
  observe branch, worktree, docs placement, root-ledger row.

## Completion criteria

- Enabled scaffold produces branch + worktree + correctly placed docs + root
  row + response fields; any git failure creates nothing; disabled scaffold is
  byte-identical to today.

## Files affected

- `internal/server/changesession.go`, `server.go`, `lifecycle.go` (spawn),
  `changesession_test.go`, `lifecycle_test.go`
- `internal/store/store.go`, `tree.go` (creation split)

## Notes

- Creation order matters: git first, files second, so a git failure never
  leaves a half-scaffolded change. The reverse ordering (docs then git) is the
  rollback hazard the plan calls out.
