# CMG-01: UI gate: hide the sessions-panel Commit button off-worktree

## Why

The sessions-panel footer already hides the worktree strip and worktree-remove
button when the change JSON carries no `worktree`, but the Commit button is
always visible — so in main-tree mode users are offered a commit that would
sweep other changes' work. The button must appear only where per-change
committing is safe: a live worktree.

## What

- `web/templates/layout.html`: ship `#chat-change-commit-btn` with the
  `hidden` attribute so it cannot flash before the change JSON arrives
  (render_test.go's assertion is presence-only — it stays green).
- `web/static/app.js` `refreshSessionsSheetChangeInfo`: fetch the element
  beside `wtRemoveBtn` and gate it with the same payload signal:
  `commitBtn.hidden = !wt || wt.State === "missing"`. No hint text — the
  button simply disappears; Commit all on Changes is the main-tree path.

The click-wiring block (spinner, POST, `pollSheetCommitStatus`) stays
untouched.

## Files affected

- `web/templates/layout.html` (new)
- `web/static/app.js` (new)

## Verification

- `go test ./...` green (render_test.go id assertions unchanged).
- Manual: on a worktrees-off repo, open a change's sessions panel — no
  Commit button; on a worktree-backed change the button shows and still
  runs the existing flow.
