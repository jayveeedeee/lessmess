---
id: WTP-05
title: Gated close — clean check, push, PR, agent review
---

# WTP-05: Gated close — clean check, push, PR, agent review

Status: see [../ledger.md](../ledger.md).

## Objective

When the feature is enabled, closing a worktree change runs the gated pipeline
— clean worktree → push → `gh pr create` → unattended reviewer session → PR
comment — and only then sets `Done`. Every gate failure blocks with an
actionable error and leaves the status untouched.

## Dependencies

- WTP-02 (worktree exists), WTP-03 (resolution), WTP-04 (sessions run there).

## Scope

- `closeChange` (`internal/server/lifecycle.go`): after `CloseOutReady`, when
  enabled and the change has a worktree, run the pipeline; disabled → today's
  behavior exactly.
- Reviewer session: gardener pattern (create with `reviewModel` via
  `spawnSessionWithModel` de-escalation, prime, `WaitDone`, delete). Prime
  includes: branch diff summary (`git diff --stat base…branch`), plan.md
  content, change ID, and instructions to write `changes/<id>/review.md`
  (verdict, findings, risks) and touch nothing else.
- PR body generated from plan.md (deterministic transformation, written to a
  temp file for `--body-file`); PR URL recorded in `.lessmess/worktrees.json`.
- Review file posted via `gh pr comment --body-file`; review state recorded
  (`pending → done/failed`).
- New error responses per step (dirty worktree, push refused/no remote, gh
  unavailable/failed, reviewer failed) — precise, agent-and-human readable.
- Reopen of a closed worktree change reattaches (no git ops beyond verification
  that the worktree still exists).

## Implementation steps

1. Extract a `closePipeline` function so the handler stays readable; each step
   returns a typed error mapped to a distinct HTTP status/message.
2. Dirty check via gitops; message points at the change's commit button.
3. Push + PR creation; body built from plan.md; URL persisted.
4. Reviewer runner mirroring `gardenerRunner`'s create/prime/wait/delete shape;
   on failure record `failed` and block close (the user chose gating; the
   review verdict itself stays informational).
5. PR comment after the review file exists.
6. Set `Done` (existing `SetChangeStatus` path), enqueue docs gardening as
   today.
7. httptest coverage: each gate failing in turn, full-success with fakes,
   disabled fast path, reopen behavior.

## Verification

- Automated pipeline tests with the gitops fake and a fake session client.
- Manual end-to-end against a scratch remote with real `gh`: dirty-close
  rejection, clean close producing PR + `review.md` + comment + Done.

## Completion criteria

- Enabled close cannot complete without a pushed branch and an open PR; the
  review artifact exists in the change dir and on the PR; disabled close is
  unchanged; all failures are actionable.

## Files affected

- `internal/server/lifecycle.go`, new `internal/server/closepipeline.go`,
  `lifecycle_test.go`, `docssession.go` (runner pattern reference)

## Notes

- Review verdict is recorded, never blocks — explicit user decision.
- `WaitDone` needs a generous timeout (review of a full diff); follow the
  gardener's context handling and make the timeout configurable via the
  existing settings layer only if the fixed default proves wrong.
