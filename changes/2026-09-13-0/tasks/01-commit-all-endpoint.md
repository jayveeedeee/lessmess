---
id: CMT-01
title: Repo-wide commit endpoint and prompt
---

# CMT-01: Repo-wide commit endpoint and prompt

Status: see [../ledger.md](../ledger.md).

## Objective

Add `POST /api/git/commit` and `GET /api/git/commit-status`: confirm-time
re-check that the tree is dirty, spawn an opencode session primed with a
repo-wide commit prompt, map it to the unassigned Discussions bucket, and
let the UI poll for completion.

## Dependencies

- CMT-00 (uses `gitStatus` for the confirm-time re-check)

## Scope

- `internal/server/gitcommit.go` (continued):
  - `repoCommitPrompt()` — variant of `commitPrompt` in `lifecycle.go` with
    no change-ID reference: review `git status`/`git diff`, write a good
    message (`changes/` records are context, not necessarily the subject),
    `git add -A` + commit, never push/amend/rebase/reset/switch branches,
    report hash + summary.
  - `commitAll` handler for `POST /api/git/commit`:
    1. `gitStatus(s.st.Dir)`; if `!Repo` → 422 "not a git repository"; if no changes → 422 "nothing to commit" (no session created).
    2. `s.oc == nil` → 503; `s.mapErr != nil` → 503 (same guards as `commitChange`).
    3. Create session titled `repo — git commit` in `s.st.Dir`, `addUnassigned` mapping entry, prime with `repoCommitPrompt()`; prime failure deletes the session and unmaps (mirror the cleanup in `commitChange`/`createDiscussionSession`).
    4. 201 `{"session": "<id>"}`.
- `internal/server/server.go`: register `POST /api/git/commit` → `s.commitAll` and `GET /api/git/commit-status` → `s.commitStatus` (the existing handler is already change-agnostic — it only reads the `session` query param).
- Tests in `gitcommit_test.go`.

## Implementation steps

1. Write `repoCommitPrompt()` and `commitAll` following the structure of `commitChange` in `lifecycle.go` (timeouts, error codes, cleanup on failure).
2. Register both routes.
3. Tests (fixture temp git repo + fake opencode client, matching the existing lifecycle/changesession test patterns):
   - commit on a dirty repo → 201, fake client saw a session primed with a prompt that contains "git add -A" and "NEVER push" and no `changes/<id>` reference; mapping contains the session in the unassigned bucket;
   - commit on a clean repo → 422, no session created;
   - commit with `s.oc == nil` → 503;
   - `GET /api/git/commit-status` without `ses_` prefix → 400 (reused handler).

## Verification

- `go test ./internal/server/ -run 'TestCommitAll|TestGitCommitStatus' -v`
- `go vet ./internal/server/`

## Completion criteria

- Dirty tree → 201 with a mapped, primed session; clean tree → 422 with no
  session; status polling works at the new route; tests pass.

## Files affected

- `internal/server/gitcommit.go`
- `internal/server/gitcommit_test.go`
- `internal/server/server.go`

## Notes

- No server-side in-flight lock: the client disables Confirm while a commit
  runs (CMT-03); a determined double-POST could spawn two sessions, both of
  which would try to commit — accepted risk, recorded in the plan.
- Implemented alongside CMT-00 in `internal/server/gitcommit.go`. On mapping
  failure the session is deleted; on prime failure the session stays mapped
  (inspectable) and 502 returns its id, mirroring `commitChange`. All
  endpoint tests pass (`TestCommitAll*`, `TestRepoCommitStatusRoute`).
