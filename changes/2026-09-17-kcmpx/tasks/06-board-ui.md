---
id: WTP-06
title: Board indicators, cleanup button, reopen reattach
---

# WTP-06: Board indicators, cleanup button, reopen reattach

Status: see [../ledger.md](../ledger.md).

## Objective

Make the worktree state visible and manageable on the board: branch,
worktree health, PR link, and review status on the change surface, plus a
manual worktree-remove action that refuses dirty worktrees.

## Dependencies

- WTP-02 (state exists), WTP-05 (PR/review state exists to show).

## Scope

- Change view payload (card/detail as the templates are structured today)
  gains: branch name, worktree present/missing/dirty, PR URL, review state.
- A worktree section/row on the change detail surface: branch, path, PR link
  (external), review status chip.
- `POST /changes/{id}/worktree/remove`: refuses when the worktree is dirty
  (422 naming the blocker), otherwise `git worktree remove` + state entry
  deletion; hidden/disabled when the feature is off or no worktree exists.
- Index/board list stays fast: state comes from `.lessmess/worktrees.json`
  plus at most one cheap git probe per visible worktree change, following the
  render-time pattern (`GitDirty` precedent).
- Reopen path: verify worktree existence, show "missing" state when a user
  deleted it manually (self-healing display, no auto-recreate in v1).

## Implementation steps

1. Extend the change view mapping with worktree fields (server-side compute,
   `data-*` keys per the index convention — never scrape display text).
2. Add the detail markup + minimal CSS following the existing card/detail
   style; PR link opens in a new tab.
3. Remove endpoint + confirmation in the UI; error surfacing consistent with
   other board actions.
4. Tests: view payload with/without worktree, remove endpoint (dirty refuse,
   clean success, feature-off 404/503 consistent with nil-disabled patterns).

## Verification

- `go test ./internal/server/ -run 'Worktree|ChangeView'`.
- Manual: board shows correct states across the lifecycle (active, dirty,
  closed-with-PR, removed).

## Completion criteria

- The board answers "where is this change's code, what's its branch, where is
  its PR, was it reviewed" at a glance; removal is safe and explicit.

## Files affected

- `internal/server/lifecycle.go` (remove endpoint), `mapping.go` or the change
  view mapper, `render.go` templates wiring
- `web/templates/*`, `web/static/app.js`/CSS as needed

## Notes

- Keep the probe lazy: only changes with state entries get git probes, so the
  index cost stays flat for repos without the feature.
