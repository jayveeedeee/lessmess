# RVT-01: Chat revert confirm dialog and one-click wiring

## Why

Today "Revert to here" opens the Controls drawer, shows a preview panel, demands a stage click, then a typed fingerprint plus `window.confirm` — four-plus steps for one undo action. The fingerprint is client-only friction the server never sees.

## What

- New confirm dialog in `web/templates/layout.html` following the Commit-all modal pattern (stable ids wired in `app.js`), plus `web/static/app.css` styling consistent with that modal.
- Dialog contents: message excerpt, collapsible affected-files summary lazily fetched from the existing `GET …/chat/diff?from=` endpoint, "Also restore file changes" checkbox **defaulting to unchecked**, and the caveat that file restoration after clearing a staged revert is not guaranteed.
- Clicking "Revert to here" (`data-chat-revert`) opens the dialog directly — the Controls drawer must not open. Confirm posts the new combined `POST …/revert` endpoint (RVT-00) with the files flag; "Reverting…" state disables dialog controls; busy sessions keep Confirm disabled.
- On success: close the dialog, `refreshChat()` (the poll shows truncated history), put the reverted message's raw text — the hidden `.chat-markdown-source` span, falling back to rendered text — into `cstate.drafts` + `#chat-prompt`, focus the composer, and confirm via the status line.
- Delete the old flow: `previewChatRevert`, the stage/fingerprint/commit click handlers, `lifecycleFingerprint`'s input gate, and the `revertTarget`/preview branches in `renderChatLifecycle`.
- Keep the Controls-drawer staged-revert box solely as stale-stage recovery; reword its copy and the `layout.html` lifecycle warning to match the new flow; keep capability gating (`revertStage`/`revertCommit` false → menu item disabled or dialog explains unavailability).
- Update render tests / pinned copy in `internal/server/*_test.go` where they assert the old revert UI.

## Files affected

- `web/templates/layout.html`
- `web/templates/partials.html` (only if the menu item needs a disabled hook)
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/*_test.go` (pinned markup/copy)

## Verification

- `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, `go vet ./...`, `go test ./...` clean.
- Manual: menu → dialog (no drawer) → Confirm → transcript truncates, composer holds the message text, focused; files checkbox unchecked by default and functional when ticked; busy and capability-gated states behave; failed commit points at the recovery box.
