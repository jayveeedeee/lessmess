# 2026-09-17-kcmpx: Worktree-per-change with gated PR and agent review

- Change ID: 2026-09-17-kcmpx
- Created: 2026-09-17
- Branch: — (developed in the main tree; the feature itself introduces branches for *future* changes)
- Status: see [ledger.md](ledger.md)

## Objective and context

lessmess changes are developed directly in the served repository's working tree:
every session (change, task subagent, commit) works in `s.st.Dir`, commits land on
whatever branch is checked out, and closing a change is a pure status transition.
Parallel changes therefore stomp on each other's uncommitted files, there is no
per-change branch history, and no PR exists for review.

The user wants:

1. Scaffolding a change to auto-create a **git branch and worktree** for it, so
   each change is developed in isolation on its own branch.
2. Closing a change to **create a PR** from that branch.
3. The PR to be **auto-reviewed by an agent**.

Decisions made with the user (2026-09-17):

- **Docs travel on the branch**: `changes/<id>/` records live in the worktree.
  The root ledger stays central in the main tree, written only by the server —
  branches never touch the root ledger or other changes' files, so parallel PRs
  do not conflict.
- **All change surfaces follow the worktree**: change-bound sessions, task
  subagent sessions, commit sessions, the per-change terminal, and the
  per-change commit button operate inside the worktree. The board shows branch,
  worktree, PR, and review state. The explorer and the repo-wide git panel stay
  on the main tree.
- **PR gates close**: when the feature is enabled, close hard-fails unless the
  branch pushes and a PR is created. The review verdict is recorded but does not
  block. A settings switch disables the whole pipeline, restoring today's
  behavior (so repos without a remote or `gh` are never locked out).

## Current behavior

- `POST /changes/scaffold` (`internal/server/changesession.go`) calls
  `store.CreateChange`, which creates `changes/<id>/` and the root-ledger row in
  the main tree. No git operations happen.
- `GitSettings.DefaultBranch` (`internal/server/settings.go`) is informational
  only: recorded in the root ledger's `Branch` column, no branch is created.
- Git awareness in Go is read-only (`internal/server/gitcommit.go`: status/diff
  for preview). All commits happen inside primed opencode sessions; the repo's
  `opencode.json` denies `git push` to agent sessions.
- `opencode.Client.CreateSession(ctx, title, directory)` accepts any directory,
  but `spawnSession` (`internal/server/settings.go`) always passes `s.st.Dir`.
- `POST /changes/{id}/close` (`internal/server/lifecycle.go`) gates on
  `store.CloseOutReady` (all tasks Test/Done), sets `Done`, and best-effort
  enqueues docs gardening.
- `store.SetChangeStatus` writes the change ledger and root-ledger row as one
  atomic operation within a single directory.
- Change-doc reads (plan, ledgers, tasks, containers, handoffs) resolve paths
  under `st.Dir`; `lessmess validate` walks the same single tree.

## Target behavior

When `git.worktrees` is enabled (opt-in, default off):

**Scaffold.** `POST /changes/scaffold` first creates branch `change/<id>` from
the configured base branch and registers a worktree at
`<parent>/<repo>-worktrees/<id>`, then creates `changes/<id>/` *inside the
worktree* and the root-ledger row in the main tree. The response includes the
branch and worktree path so the just-bound discussion session (whose directory
is fixed to the main tree) can keep planning against the worktree by absolute
path. Every later session spawned for the change — task subagents, commit
sessions, board-created sessions — is created with `directory = worktree`.
Before any writes, the main tree's uncommitted files are checked: dirt is
**allowed** but returned in the scaffold response as a `warning` naming the
files, since the worktree is cut from the committed base and will not contain
them. Any git failure fails the scaffold with a clear, agent-facing error.

**During the change.** The store resolves change-doc reads for a change through
its worktree when one is active; `SetChangeStatus` writes the change ledger in
the worktree and the root-ledger row in the main tree back-to-back. The
per-change terminal and commit button target the worktree. Validation
(`lessmess validate`) uses the same resolution so it does not flag root-ledger
rows whose directories live in worktrees.

**Close (gated).** After the existing `CloseOutReady` gate, close additionally
requires, in order: the worktree is clean (else 422 naming the commit button);
the branch pushes; `gh pr create` succeeds (title from the change, body from
plan.md; URL recorded); a reviewer session runs unattended, writes its verdict
to `changes/<id>/review.md`, and the server posts it as a PR comment. Only then
is the status set to `Done`. Each step's failure blocks close with an actionable
message and leaves the status untouched.

**Lifecycle.** Reopen reattaches the existing worktree. Worktree removal is a
manual board action (refuses when the worktree is dirty); nothing is deleted
automatically in v1.

## Scope

- New `internal/gitops` package: branch create, worktree add/list/remove,
  dirty check, push, `gh pr create`/`gh pr comment`, plus the
  `.lessmess/worktrees.json` state file (written via `model.WriteFileAtomic`).
- `GitSettings` extension: `worktrees` (bool), `baseBranch` (promotes today's
  informational `defaultBranch` to the base for new change branches),
  `reviewModel` (reviewer session model override, de-escalating like
  `gardenerModel`). Settings page fields and the dotted-path allowlist extended
  in lockstep with `EffectiveSettings`.
- Scaffold, store overlay, validation, session/terminal/commit routing, close
  pipeline, board UI, and prompt updates as described under Target behavior.
- README update for the new user-visible workflow.

## Non-goals

- Worktree-awareness for the explorer and the repo-wide git-status/commit panel.
- Automatic worktree cleanup or merged-PR detection (merge stays a manual GitHub
  action in v1).
- Review verdicts blocking close.
- PR merge automation, branch protection, or CI integration.
- Supporting worktrees for repositories without git (feature stays opt-in; when
  enabled, git is required and its absence fails loudly).

## Design decisions

- **Dirty main tree at scaffold**: allowed, never refused — the check runs
  before scaffold's own writes and reports all uncommitted paths in the
  response's `warning` field (user decision 2026-09-17). The agent relays it;
  refusing was rejected because scaffold itself dirties the main tree with
  root-ledger rows, so a strict check would nag on every scaffold after the
  first.
- **Branch naming**: `change/<id>` (e.g. `change/2026-09-17-kcmpx`) — unambiguous,
  collides with nothing, sorts naturally.
- **Worktree location**: sibling of the served repo, `<parent>/<repo>-worktrees/<id>`,
  outside the repository so it never pollutes status, docs walks, or store globs.
- **Worktree truth**: existence is probed via `git worktree list` at use time;
  `.lessmess/worktrees.json` (one file per feature, per the tooling-state
  pattern) records only what git cannot tell us: PR URL, review state, created
  timestamp. A stale JSON entry is self-healing.
- **Dependency direction**: `internal/gitops` sits beside `store` (imports only
  `model`); `store` receives a resolver interface for "where do change X's docs
  live" so the store never imports gitops and the CLI (`validate`) can wire the
  same resolver. This extends the current `os/exec` confinement (server,
  opencode, terminal) with one auditable package — `internal/AGENTS.md` and
  `STRUCTURE.md` are updated at close.
- **Root ledger centralization**: the server writes root-ledger rows in the main
  tree only, at scaffold. Worktree-side prompts (change/task primes) state that
  the root ledger is maintained centrally and that agents touch only their own
  change's ledger. This is what keeps parallel branches conflict-free.
- **Cross-tree status writes**: `SetChangeStatus` performs the worktree change
  ledger write and the main-tree root-ledger row write back-to-back. The single
  atomic write is weakened to a two-file sequence; both writes stay atomic
  individually and failure of the second is logged and surfaced in the response.
- **Reviewer mechanics**: one unattended session (gardener pattern: create,
  prime, `WaitDone`, delete) primed with the branch diff summary, plan.md, and
  instructions to write `changes/<id>/review.md` and make no other edits. The
  server then posts the file via `gh pr comment --body-file`. Capturing the
  reply via API is not needed (the client has no messages endpoint); the file is
  the artifact.
- **Discussion-session wrinkle**: the session that fires scaffold cannot move to
  the worktree (directory is fixed at creation, client has no move). It receives
  branch + worktree path in the scaffold response and continues planning by
  absolute path; all implementation sessions spawn worktree-native.
- **`defaultBranch` promotion**: reused as the base branch for new change
  branches rather than adding a second field; unset falls back to the repo's
  current branch at scaffold time.
- **Worktree project config**: a new worktree may not inherit the repo's
  `opencode.json` permission allowlist (opencode project discovery from a
  worktree directory is unverified). `gitops` copies `opencode.json` into the
  worktree root at creation; implementation verifies agent behavior in a real
  worktree and adjusts (symlink/copy) as needed.

## Implementation approach

Ordered by dependency; see the task files for step-level detail.

1. **WTP-00 Settings** — extend `GitSettings`, effective merge, allowlist,
   settings page.
2. **WTP-01 gitops** — new package with injectable command seams; unit-tested
   against temp git repos; owns `.lessmess/worktrees.json`.
3. **WTP-02 Scaffold** — branch + worktree creation, change docs in the
   worktree, root ledger central, response fields, failure modes.
4. **WTP-03 Store overlay** — resolver interface, change-doc resolution,
   cross-tree `SetChangeStatus`, worktree-aware validation.
5. **WTP-04 Session routing** — spawn paths, terminal cwd, commit button, board
   created sessions; scaffold response disclosure.
6. **WTP-05 Close pipeline** — clean gate, push, PR create, reviewer session,
   PR comment, failure surfacing.
7. **WTP-06 Board UI** — branch/worktree/PR/review indicators, cleanup button,
   reopen reattach.
8. **WTP-07 Docs and prompts** — change/task primes, README, AGENTS/STRUCTURE
   notes for the new package and exec confinement.

## File-level impact

- `internal/gitops/` — new package (git.go, worktrees state, fakes, tests).
- `internal/server/settings.go` — `GitSettings`, effective settings, allowlist.
- `internal/server/settingsapi.go` + web templates — settings fields.
- `internal/server/changesession.go` — scaffold flow, prompts.
- `internal/server/lifecycle.go` — close pipeline, commit routing.
- `internal/server/mapping.go`, `autosession.go` — spawn routing.
- `internal/server/terminal.go` — cwd selection.
- `internal/server/gitcommit.go` — unchanged (main-tree, read-only).
- `internal/server/server.go` — wiring resolver + gitops into `Server`.
- `internal/store/` — resolver interface, change-dir resolution, cross-tree
  status write, validation.
- `cmd/lessmess` — validate wiring of the resolver.
- `web/templates`, `web/static` — board indicators, buttons, settings fields.
- `README.md`, `opencode.json` (only if worktree config discovery requires it),
  root + folder AGENTS/STRUCTURE docs at close.

## Data, API, and configuration changes

- Settings: `git.worktrees` (bool, default false), `git.baseBranch` (string,
  optional), `git.reviewModel` (string, optional). Layered as usual.
- State: new `.lessmess/worktrees.json` (change ID → branch, path, PR URL,
  review state, created).
- API: scaffold response gains `branch` and `worktree` (plus `warning` naming
  uncommitted main-tree files when any exist); close gains
  step-specific 4xx errors; new `POST /changes/{id}/worktree/remove`; change
  views gain `branch`/`worktree`/`pr`/`review` fields.
- Workflow data: no schema changes to plan/ledger/task files. Root-ledger
  `Branch` column becomes the real branch name for worktree changes.

## Safety, security, and rollback

- Feature is opt-in; off means byte-identical behavior to today.
- Go-side git operations are create-only (branch, worktree) plus push/PR at an
  explicit user close; no rebase, reset, amend, or force anywhere.
- Push uses the ambient git credential helper; `gh` must already be
  authenticated — failures surface as actionable close errors, never silent.
- Rollback: disable `git.worktrees`; existing worktrees remain on disk and can
  be removed via the board button or `git worktree remove`; branches persist;
  no workflow-data migration exists to undo.
- The reviewer prompt forbids all edits except `review.md`; abuse surface
  matches the gardener's containment approach.

## Testing and verification strategy

- `gitops`: table tests against real temp git repos (init, branch, worktree add,
  dirty detection, push to a local bare remote, `gh` behind a seam fake).
- `store`: resolver resolution (worktree present/absent/stale), cross-tree
  status writes, validation against a split tree.
- `server`: httptest flows — scaffold with fake gitops (success, dirty repo,
  git absent), close pipeline steps failing one at a time, session spawn
  directory capture via the existing injectable spawn seam.
- Manual: one real end-to-end change in this repo — scaffold, work in the
  worktree, close against a scratch remote; `go vet ./... && go test ./...`;
  `lessmess validate` with an active worktree.

## Observability

- slog events for: branch/worktree created, push, PR created, review session
  started/finished, worktree removed, and every close-gate failure with the
  failing step named.
- Board surfaces the same states (branch, worktree ok/missing, PR URL, review
  status) so no log diving is needed for normal operation.

## Rollout sequence

1–8 in task order (settings → gitops → scaffold → store → sessions → close →
UI → docs). The feature stays disabled until the user flips `git.worktrees` in
Settings, then is exercised by the next scaffolded change end-to-end.

## Risks and mitigations

- **Weakened atomicity** of change/root-ledger writes across trees — mitigated
  by back-to-back writes, failure logging, and a repair note in the ledger
  decision log; drift is visible on the board.
- **opencode project discovery in worktrees** may not pick up
  `opencode.json` — mitigated by copying the file at worktree creation and
  verifying manually in WTP-04.
- **`gh`/push credentials are environment-dependent** — the close gate fails
  with precise step errors; the settings switch removes the gate entirely.
- **Parallel worktrees of the same repo share one index-lock-free store** — git
  worktrees are designed for this; dirty checks are per-worktree.
- **Stale worktrees after manual deletion** — probed via `git worktree list`
  before use; the UI shows "missing" and offers removal from state.

## Acceptance criteria

1. With `git.worktrees` enabled, scaffolding creates `change/<id>`, a worktree
   at `<parent>/<repo>-worktrees/<id>`, `changes/<id>/` inside the worktree,
   and a root-ledger row in the main tree; the scaffold response names branch
   and worktree path, and warns naming uncommitted main-tree files when any
   exist (scaffold still succeeds).
2. Change sessions, task subagent sessions, commit sessions, board-created
   sessions, and the per-change terminal all operate inside the worktree.
3. The board renders plan/ledger/tasks for a worktree change from the worktree;
   `lessmess validate` passes with an active worktree.
4. Close on a dirty worktree fails with guidance; close on a clean worktree
   pushes, creates a PR whose body summarizes plan.md, runs the reviewer,
   writes `changes/<id>/review.md`, posts it as a PR comment, and only then
   sets `Done`.
5. With the feature disabled, scaffold, sessions, and close behave exactly as
   today; no git operations occur.
6. Reopen reattaches the worktree; the board's remove button refuses dirty
   worktrees; nothing is ever deleted automatically.
7. `go vet ./...` and `go test ./...` pass; README documents the workflow.

## Tasks

1. [WTP-00](tasks/00-settings.md) — Git settings: worktrees switch, base branch, reviewer model
2. [WTP-01](tasks/01-gitops-package.md) — internal/gitops: git mechanics and worktree state
3. [WTP-02](tasks/02-scaffold-worktree.md) — Scaffold creates branch, worktree, and worktree-side change docs
4. [WTP-03](tasks/03-store-overlay.md) — Store overlay: worktree-aware reads, cross-tree status, validation
5. [WTP-04](tasks/04-session-routing.md) — Route change sessions, terminal, and commits into the worktree
6. [WTP-05](tasks/05-close-pipeline.md) — Gated close: clean check, push, PR, agent review
7. [WTP-06](tasks/06-board-ui.md) — Board indicators, cleanup button, reopen reattach
8. [WTP-07](tasks/07-docs-and-prompts.md) — Prompts, README, and docs-system alignment
