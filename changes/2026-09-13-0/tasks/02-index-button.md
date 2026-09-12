---
id: CMT-02
title: Index page Commit all button
---

# CMT-02: Index page Commit all button

Status: see [../ledger.md](../ledger.md).

## Objective

Render a "Commit all" button next to "New change session" on the changes
index page, with its enabled/disabled/hidden state computed server-side
from the repository's git status at page load.

## Dependencies

- CMT-00 (uses `gitStatus`)

## Scope

- `internal/server/render.go`: extend `indexView` with `GitRepo bool` and `GitDirty bool`.
- `internal/server/server.go` `index`: for the HTML path only, call `gitStatus(s.st.Dir)` and populate the two fields (JSON response unchanged).
- `web/templates/index.html`: inside the Changes `.section-head`, render

  ```html
  {{if .Data.GitRepo}}
  <button type="button" id="commit-all-btn" class="btn-ghost"{{if not .Data.GitDirty}} disabled{{end}}>Commit all</button>
  {{end}}
  ```

  next to the existing form (final styling class per existing button conventions; keep the id `commit-all-btn` stable).
- Render test coverage for the three states.

## Implementation steps

1. Add the fields to `indexView` and populate them in `index` (guard: only in the `wantsHTML` branch).
2. Edit `index.html` so the button sits in the same `.section-head` row as the "New change session" form.
3. Extend/add a render or server test asserting: dirty fixture repo → `id="commit-all-btn"` present without `disabled`; clean repo → present with `disabled`; non-repo → absent. (Fixture git repos as in CMT-00; skip when `git` is unavailable.)

## Verification

- `go test ./internal/server/ -run 'TestIndex' -v`
- Manual: `lessmess serve` on this repo (currently dirty) shows the button enabled; on a clean/non-git dir it is disabled/absent.

## Completion criteria

- All three render states verified by tests; the button never appears for
  non-git repositories.

## Files affected

- `internal/server/render.go`
- `internal/server/server.go`
- `internal/server/server_test.go` or `render_test.go`
- `web/templates/index.html`

## Notes

- Page-load state can go stale while the page is open; that is intentional
  (decision: page load + rechecks). The modal re-fetches live status when
  opened (CMT-03).
