---
id: CMT-03
title: Commit modal markup, CSS, and JS flow
---

# CMT-03: Commit modal markup, CSS, and JS flow

Status: see [../ledger.md](../ledger.md).

## Objective

Implement the confirmation modal: live preview of uncommitted changes,
confirm/cancel, busy spinner while the commit session runs, polling to
"✓ Committed", and graceful error/clean-tree handling.

## Dependencies

- CMT-01 (`POST /api/git/commit`, `GET /api/git/commit-status`)
- CMT-02 (`#commit-all-btn` on the page)

## Scope

- `web/templates/index.html`: add a `#commit-modal` overlay following the
  `#notif-modal` pattern in `layout.html` (`.modal` window, `.modal-head`
  with title "Commit all changes" + `.modal-close`, `.modal-body` with
  `#commit-file-list`, `#commit-stat`, and actions row holding
  `#commit-confirm-btn` + `#commit-modal-status`).
- `web/static/app.css`: styles for `#commit-modal` (overlay + hidden rule,
  mirroring `#notif-modal`), the porcelain list (monospace status code
  column), and the diffstat block; reuse `.modal`, `.spinner`, and button
  primitives.
- `web/static/app.js`: new `initCommitAll()` called from the existing
  `DOMContentLoaded` init block:
  1. Click `#commit-all-btn` (guard `disabled`) → `fetch("/api/git/status")` → populate the modal:
     - `repo:false` or empty `changes` → show "Nothing to commit." and disable Confirm;
     - otherwise render one row per change (`code` + `path`) and the `stat` text; open modal.
  2. Confirm → busy state (`disabled`, spinner + "Committing…") → `POST /api/git/commit` → on 201 poll `GET /api/git/commit-status?session=` every 3 s (cap ~10 min, mirroring `pollCommitStatus`) → done → "✓ Committed", disable `#commit-all-btn` on the page, and after ~2 s close the modal and refresh the Discussions list via the existing `loadDiscussions()`.
  3. Error paths: status fetch failure → message in modal; POST failure (incl. 422 clean-tree race) → `#commit-modal-status` shows the server error, Confirm re-enables; poll contact loss → same copy style as the board commit ("check the session in Discussions").
  4. Close affordances: `.modal-close` button, backdrop click, and Escape (hook into the existing global handlers or add equivalents that do not interfere with `#detail`/`#notif-modal`); closing is blocked only while a commit is mid-flight (or simply allowed — the session continues server-side; keep behavior consistent and documented in code).

## Implementation steps

1. Markup in `index.html` (template is index-scoped, so the modal lives here rather than `layout.html`).
2. CSS additions in `app.css` (docs-excluded tree, no gardener concerns).
3. `initCommitAll()` in `app.js`; register it in the init block.
4. Manual verification (no JS test harness exists in this project).

## Verification

- Build: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, restart the server, hard-refresh (`.AssetsV` cache-bust handles JS/CSS).
- On this repo (dirty): button enabled → modal lists current `M`/`AD`/etc. entries + diffstat → Cancel closes; reopen → Confirm → spinner → "✓ Committed" → page button disabled → session appears under Discussions and the commit exists (`git log -1`).
- Clean repo: modal says "Nothing to commit.", Confirm disabled.

## Completion criteria

- Full flow works in the running UI as described in plan.md target
  behavior steps 2–4, with all error paths showing sensible messages.

## Files affected

- `web/templates/index.html`
- `web/static/app.css`
- `web/static/app.js`

## Notes

- Keep element ids stable: `commit-all-btn`, `commit-modal`,
  `commit-file-list`, `commit-stat`, `commit-confirm-btn`,
  `commit-modal-status` — `templates/AGENTS.md` requires id stability for
  JS-wired controls.
- Implemented: modal markup in `index.html` (always rendered on the page;
  `initCommitAll` returns early without `#commit-all-btn`), styles after the
  notif block in `app.css`, and `initCommitAll()` in `app.js` registered in
  the `DOMContentLoaded` block. Modal close (✕, backdrop, Escape) is blocked
  while a commit is in flight; on "✓ Committed" the page button disables and
  `loadDiscussions()` surfaces the session. Verified live via a second
  instance on :9099 (button enabled on this dirty repo, status endpoint
  returns the real porcelain list) — the real click-through commit is left
  for user acceptance (it creates a real session and commit).
- Revised on user feedback (2026-09-13): the two-block preview (file list +
  diffstat) read as duplicated. Now one merged list — status code, path, and
  per-file `+/-` counts from `git diff --numstat HEAD` (untracked files show
  no counts) — with a single `git diff --shortstat HEAD` summary line in
  `#commit-stat`. Server `stat` field replaced by per-row `added`/`deleted`
  + `summary`; `TestGitStatusHelper` asserts the merged counts. Verified
  live: 73/90 rows carry counts (the other 17 are untracked), summary reads
  "73 files changed, 737 insertions(+), 184 deletions(-)".
