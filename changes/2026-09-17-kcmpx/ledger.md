# Ledger — 2026-09-17-kcmpx

- Change ID: 2026-09-17-kcmpx
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: In progress
- Last updated: 2026-09-17

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Test | Implementation and verification are complete; awaiting user acceptance before Done. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [WTP-00](tasks/00-settings.md) | Git settings — worktrees switch, base branch, reviewer model | Test | — | 2026-09-17 | Evidence: merge tests (17 sources, worktrees default false, personal wins), allowlist round-trip (git.worktrees/defaultBranch/reviewModel), ReviewModel precedence, patch round-trip through strict decoder; `go vet ./... && go test ./...` green |
| [WTP-01](tasks/01-gitops-package.md) | internal/gitops — git mechanics and worktree state | Test | — | 2026-09-17 | Evidence: 14 tests green — real-repo lifecycle (branch/worktree add/dirty/remove/list, push to local bare remote, config copy) + scripted-commander cases (PR arg construction + URL parse, ErrGHUnavailable, ErrNoRemote never pushes, ErrBranchExists); state round-trip/malformed/atomic; `go vet ./... && go test ./...` + CGO_ENABLED=0 build green |
| [WTP-02](tasks/02-scaffold-worktree.md) | Scaffold creates branch, worktree, and worktree-side change docs | Test | WTP-00, WTP-01 | 2026-09-17 | Evidence: 4 httptest flows with real git (enabled+clean, dirty-warn, git-failure aborts with nothing written, disabled legacy) green; live smoke on a scratch clone: branch `change/2026-09-17-jl4pi` + worktree + state + docs-in-worktree + main-tree row + response fields incl. warning and worktree-note all verified |
| [WTP-03](tasks/03-store-overlay.md) | Store overlay — worktree-aware reads, cross-tree status, validation | Test | WTP-01 | 2026-09-17 | Evidence: store tests (resolver matrix incl. stale self-heal, cross-tree SetChangeStatus, MintChangeID row-awareness, CreateChangeAt) green; live smoke: board + plan detail served from worktree, `lessmess validate` shows no rule-6 for the worktree change, close wrote worktree ledger Done + main row Done; watch.go covers worktree dirs |
| [WTP-04](tasks/04-session-routing.md) | Route change sessions, terminal, and commits into the worktree | Test | WTP-03 | 2026-09-17 | Evidence: capture tests — change session, commit session, and terminal cwd all in the worktree with the root-ledger stanza in the prime; legacy path byte-identical (prompt test pinned); spawn signature split settingsDir/sessionDir. opencode.json discovery inside a live worktree remains a user-side check (unit-covered for the copy itself) |
| [WTP-05](tasks/05-close-pipeline.md) | Gated close — clean check, push, PR, agent review | Test | WTP-02, WTP-03, WTP-04 | 2026-09-17 | Evidence: pipeline tests green — full success against a real local bare remote (push + PR + review file + comment + Done + reopen resolve), dirty gate 422, no-remote gate 502 with nothing recorded, missing review.md → reviewState failed + close blocked. Real-gh end-to-end deferred to the user (needs GitHub auth); sequence covered by scripted gh |
| [WTP-06](tasks/06-board-ui.md) | Board indicators, cleanup button, reopen reattach | Test | WTP-02, WTP-05 | 2026-09-17 | Evidence: strip tests (active/missing/dirty flags, absent for main-tree changes), remove endpoint (dirty 422 keeps worktree, clean removes dir+state, second call 404, disabled 409); JSON payload carries worktree; app.js confirm flow + CSS strip |
| [WTP-07](tasks/07-docs-and-prompts.md) | Prompts, README, and docs-system alignment | Test | WTP-02, WTP-04, WTP-05 | 2026-09-17 | Evidence: discussion prompt covers branch/worktree/worktree-note/warning handling (pinned tests green); README "Worktree pipeline (optional)" section + git settings table rows; learning drafts recorded in the task notes for the close-out gardener job |

Task dependencies: WTP-00 and WTP-01 are independent roots; WTP-02 needs both;
WTP-03 needs WTP-01; WTP-04 needs WTP-03; WTP-05 needs WTP-02/03/04; WTP-06
needs WTP-02/05; WTP-07 last.

## Decision log

- 2026-09-17 — Scope chosen with the user: worktree per change with docs
  traveling on the branch (root ledger stays central in the main tree, written
  only by the server, so parallel PRs never conflict); all change surfaces
  (sessions, terminal, commit button) follow the worktree; close is gated on
  push + PR creation when the feature is enabled.
- 2026-09-17 — The agent review records a verdict (review.md + PR comment) but
  does not block close — explicit user decision; only the PR gates.
- 2026-09-17 — Whole pipeline is opt-in via `git.worktrees` (default off);
  disabled behavior is byte-identical to today so no-remote/no-`gh` repos are
  never locked out.
- 2026-09-17 — Git mechanics run in Go via a new `internal/gitops` package
  (branch/worktree/push/gh) rather than primed sessions: deterministic,
  idempotent, testable with seams; sessions stay commit-only per the existing
  opencode.json push denial. This extends the exec-confinement convention from
  three packages to four (documented at close).
- 2026-09-17 — Branch naming `change/<id>`; worktrees at
  `<parent>/<repo>-worktrees/<id>` outside the served tree; existence probed
  via `git worktree list`, state (PR URL, review) in
  `.lessmess/worktrees.json`.
- 2026-09-17 — The scaffold-firing discussion session cannot move into the
  worktree (opencode session directory is immutable post-creation), so the
  scaffold response names branch + worktree path and planning continues by
  absolute path; all later sessions spawn worktree-native.
- 2026-09-17 — Dirty main tree at scaffold: allowed with a warning (user
  decision "warn") — the pre-write `status --porcelain` check populates the
  scaffold response's `warning` field naming uncommitted files the worktree
  will not contain; refusing was rejected because scaffold itself dirties the
  main tree with root-ledger rows, nagging on every subsequent scaffold.
- 2026-09-17 — Base branch setting became a combobox (user request): local
  branches listed via read-only `git for-each-ref` in the options payload
  (`branches`), filled into a `git.defaultBranch` datalist before the
  opencode-availability early return so it works with the service down.
  Datalist keeps free text, matching the agent/model field convention.
- 2026-09-17 — Review UX (user request): the reviewer session is persisted
  and bound to the change instead of deleted — it stays in the Sessions list
  for follow-up questions in its worktree terminal (prime failure still
  deletes: nothing unbound leaks); a board Review button serves
  `changes/<id>/review.md` through the reading modal; the dirty gate exempts
  `changes/<id>/` metadata (status flips, review.md) while code dirt still
  blocks; a re-close reuses the already-open PR instead of failing on
  "already exists".
- 2026-09-17 — Reviewer model, settings keys, and defaults per plan.md; v1
  non-goals: explorer/repo-panel worktree-awareness, auto cleanup, merge
  automation, review-gates-close.
