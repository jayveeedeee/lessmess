# 2026-10-02-gvomn: Gate per-change commits to worktree-backed changes

- Change ID: 2026-10-02-gvomn
- Created: 2026-10-02
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

The per-change Commit button in the Chat sessions panel posts
`POST /changes/{id}/commit`, which spawns an opencode commit session in
`changeSessionDir(id)` and instructs it to `git add -A` and commit. When a
change has no live worktree (worktrees feature off, a change created before
the feature was enabled, or a hand-deleted worktree), that directory is the
**main tree**, so the commit sweeps every in-flight change's work and manual
edits into one commit attributed to "this change". The button is also always
visible in the UI, with no worktree gate.

The user decided: a per-change commit only exists where the change owns an
isolated tree. Main-tree changes commit via the index page's repo-wide
"Commit all" only, which is honestly labeled for exactly that sweeping
semantic. This also clarifies the path for a future instant-commit fast path
(worktree-only), which is explicitly out of scope here.

## Current behavior

- `commitChange` (internal/server/lifecycle.go) has no worktree guard: with
  no live worktree it primes the commit session in the main tree, where the
  `commitPrompt`'s `git add -A` stages all uncommitted work repo-wide.
- The sessions-panel footer (`refreshSessionsSheetChangeInfo` in
  web/static/app.js) hides the worktree strip and worktree-remove button
  when the change JSON carries no `worktree`, but the Commit button is
  always visible — it ships visible in layout.html and is never gated.
- `worktreeViewFor` reports `State: "missing"` for a registered-but-gone
  worktree; `changeSessionDir`'s `worktreeEntry` probe self-heals such
  entries to absent and falls back to the main tree — so a "missing"
  worktree is also a sweeping-commit hazard.

## Target behavior

- `POST /changes/{id}/commit` refuses non-worktree-backed changes, mirroring
  `worktreeRemove`'s codes and wording: `409 "the worktree pipeline is
  disabled"` when worktrees are off; `404 "no live worktree registered for
  change <id>"` when the feature is on but `worktreeEntry(id)` misses.
- The sessions-panel Commit button is hidden unless the change JSON carries
  a worktree whose `State` is not `"missing"`; layout.html ships it
  `hidden` so it cannot flash before the change JSON arrives.
- Worktree-backed commits are unchanged: 201, same prompt (safe inside an
  isolated worktree), same status polling.
- The predicate everywhere is `worktreeEntry` — the same probe
  `changeSessionDir` uses — so the endpoint's verdict and the session's
  actual working directory can never disagree.

## Scope

- internal/server/lifecycle.go: guard in `commitChange` (codes/wording
  mirror `worktreeRemove`; no teaching prose).
- web/static/app.js: gate button visibility in
  `refreshSessionsSheetChangeInfo` beside the existing `wtRemoveBtn.hidden`
  pattern.
- web/templates/layout.html: ship `#chat-change-commit-btn` `hidden`.
- internal/server/lifecycle_test.go: repurpose the current 201 test (its
  fixture is non-worktree) to the new refusal; add 409 (feature off) and
  404 (feature on, no entry) cases.
- README.md only if it documents the per-change commit endpoint's behavior
  contract.

## Non-goals

- Hybrid instant-commit fast path (typed message → direct git commit) —
  planned as its own change.
- Any footer hint or messaging about where to commit instead.
- Changes to repo-wide Commit all, `prompts.commit` settings, or
  `commitPrompt` text.
- Commit-status polling, close pipeline, or worktree feature semantics.

## Design decisions

- Per-change predicate, not the global setting: feature-on with a legacy
  non-worktree change must refuse too, and `worktreeEntry` (with its
  git-probe self-heal) is the single source of truth shared with
  `changeSessionDir`.
- 409/404 and error wording copy `worktreeRemove` verbatim for consistency;
  errors state the fact, not the remedy.
- The button stays in the DOM (`hidden`, like close/reopen) so
  render_test.go's presence-only id assertion and the wiring block stay
  untouched.
- Deliberate API contract change: clients committing a main-tree change via
  the per-change endpoint now get 409/404. That is the point of the change.

## Acceptance criteria

- Worktrees off: `POST /changes/{id}/commit` → 409 `"the worktree pipeline
  is disabled"`.
- Worktrees on, no live entry (never registered, self-healed stale, or
  missing on disk): → 404 `"no live worktree registered for change <id>"`.
- Live worktree: 201 + primed commit session, unchanged from today.
- Sessions panel: Commit button hidden for main-tree and missing-worktree
  changes, visible for active/dirty worktree-backed changes, no flash on
  open.
- `go vet ./...` and `go test ./...` green; `lessmess validate` clean.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
