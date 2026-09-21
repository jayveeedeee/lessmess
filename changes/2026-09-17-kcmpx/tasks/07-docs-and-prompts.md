
# WTP-07: Prompts, README, and docs-system alignment

Status: see [../ledger.md](../ledger.md).

## Objective

Align every durable instruction surface with the new model: agent primes
(worktree + root-ledger-central rules), the human-facing README, and the
repo's own STRUCTURE/AGENTS notes (new package, exec confinement).

## Dependencies

- WTP-02, WTP-04, WTP-05 (the behaviors being documented).

## Scope

- `changePrompt` / `taskPrompt` / discussion prompt (`changesession.go`): final
  wording for the worktree stanza — work happens in the worktree on branch
  `change/<id>`; the root ledger is server-maintained in the main tree and must
  never be edited from a worktree; commit normally (commit-only rule unchanged;
  push remains forbidden to sessions — the server pushes at close).
- Discussion prompt rule 4: scaffold response now also names branch + worktree;
  the planning session continues against the worktree path.
- `README.md`: new section covering the opt-in worktree pipeline (settings,
  scaffold/close flow, PR gating, reviewer, cleanup).
- Repo docs: `internal/` and root `STRUCTURE.md`/`AGENTS.md` learnings for the
  `gitops` package and the four-package exec confinement (these are
  marker-guarded machine sections — updated via the normal gardener flow at
  close; this task drafts the learning lines and verifies the constraint
  documentation, e.g. `internal/AGENTS.md`'s exec-confinement bullet, is
  updated as part of the change's own gardening job).
- `.lessmess`-adjacent: nothing (state file documented in WTP-01 comments and
  README).

## Implementation steps

1. Finalize prompt wording; keep the pinned test structure
   (`changesession_test.go` prompt tests) green or updated deliberately.
2. Update the discussion prompt's scaffold step for the new response fields.
3. README section + settings mention.
4. Draft the learning lines for `internal/gitops`, the store resolver, and the
   close pipeline (concise, current-state, no change narration) so the
   gardener's close job lands them correctly.

## Verification

- `go test ./internal/server/ -run 'Prompt'` (pinned prompt tests).
- README renders correctly; a fresh read of the change/task primes states the
  three rules (worktree cwd, root-ledger central, commit-only).

## Completion criteria

- No prompt or doc contradicts the shipped behavior; the root-ledger-central
  rule is stated wherever agents are primed for worktree changes.

## Files affected

- `internal/server/changesession.go` + prompt tests
- `README.md`
- Learning drafts recorded in this task; applied via the docs gardener at close

## Notes

- Prompt text is pinned by tests that exist to prevent drift; extend them with
  the worktree stanza assertions rather than relaxing the existing ones.
- Learning drafts for the close-out docs job (current-state phrasing, no
  change narration — applied via the gardener):
  - `internal/gitops` (new package): "Purpose: git and gh mechanics for the
    worktree pipeline — branch/worktree add/remove, dirty checks, push, PR
    create/comment — behind an injectable Commander seam; owns
    `.lessmess/worktrees.json`."
  - `internal/` root learning: replace the exec-confinement learning — os/exec
    is confined to `server` (read-only git status), `opencode`, `terminal`, and
    `gitops` (the one sanctioned mutating git layer); commits still happen in
    opencode sessions, pushes in gitops at gated close.
  - `internal/server`: worktree pipeline learnings — `git.worktrees` opt-in;
    scaffold cuts `change/<id>` + worktree and writes change docs in the
    worktree while root-ledger rows stay main-tree; `store` resolves
    worktree-backed changes through `SetChangeRoot`; change-bound sessions,
    terminal, and commits follow `changeSessionDir`; close runs the gated
    pipeline (clean → push → PR → reviewer → comment) before Done.
  - `internal/store`: `SetChangeRoot`/`ChangeRootFunc` — root rows without a
    main-tree directory resolve through the injected hook; stale entries
    self-heal to unresolved (rule-6 violation).
  - `cmd/lessmess`: validate wires `server.WorktreeChangeRoot` so
    worktree-backed changes validate at their worktree location.
- `prompts.review` (a settings addendum key for the reviewer prime) was
  deliberately deferred; the reviewer prompt is built-in only for now.
