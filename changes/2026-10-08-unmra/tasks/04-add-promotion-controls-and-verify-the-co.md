# CHAT-04: Add promotion controls and verify the complete chat lifecycle

## Why

Users need an obvious but deliberate path from a conversation to tracked work.
The entire lifecycle must work from the new Chats destination without losing
history, drafts, navigation context, or the distinction between planning and
implementing.

## What

- Add Promote to change to the eligible standalone chat action menu. Open a
  small accessible confirmation with editable suggested title and valid prefix.
  State plainly that confirmation starts scaffolding/planning, not implementation.
- Handle cancellation, unavailable eligibility, busy/stale state, progress, and
  errors without hiding the conversation or encouraging duplicate submissions.
- Follow successful binding from Chats and other entry pages, preserving the
  same session and exposing existing change Work/Sessions controls. Remove the
  standalone card only after binding and ensure the new change is discoverable.
- Keep promotion and late title updates lossless for drafts, transcript state,
  scroll, breadcrumbs, and mobile auxiliary-view behavior.
- Document Chats, one-time naming, unchanged free-chat edit behavior, planning
  discussions, and promotion approval boundaries in `README.md`.
- Run integrated regression verification for session discovery, naming, manual
  rename, promotion, reload/resume, service failures, and hub-prefixed URLs.

## Files affected

- `web/templates/layout.html` and `web/templates/chats.html`
- `web/static/app.js` and `web/static/app.css`
- `web/chat_queue_test.mjs` or focused browser-glue tests with Go wrappers
- `internal/server/render_test.go`, promotion tests, and hub/base-path tests
- `README.md`

## Verification

- Exercise New chat → first user input → generated title → manual rename →
  confirmed promotion → same-session change resume, with no duplicated change
  or conversation and no implementation started implicitly.
- Verify cancel, busy/stale/error, double-click, unsupported-service, and delayed
  naming behavior, including unchanged drafts and preserved history.
- Check keyboard/focus behavior, mobile and desktop layouts, browser back/reload,
  and both single-project and `/p/example` hub mode after rebuilding assets.
- Run `go vet ./...`, `go test ./...`, focused browser-glue tests, and the
  lessmess workflow validator. Record verification evidence before Test;
  Done and closing the change remain the user's actions.
