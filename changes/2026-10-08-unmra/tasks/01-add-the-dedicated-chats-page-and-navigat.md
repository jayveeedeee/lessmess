# CHAT-01: Add the dedicated Chats page and navigation

## Why

Standalone conversations should be a destination of their own, not an auxiliary
list underneath tracked changes. A familiar card list also makes conversations
easier to find and resume.

## What

- Add the `/chats` page, renderer/template, and active Chats navigation alongside
  Changes and Explorer. Provide a New chat action using the existing chat spawn
  only inside the Chats section, matching Changes' section-local creation action.
- Replace the Settings navigation text with a gear icon, preserving its accessible
  label, tooltip, prefixed destination, and active state. Returning from service
  management goes to Chats rather than implicitly spawning a new conversation.
- Render recently active chat cards from the standalone feed, with safe titles,
  timestamps, and understandable loading, empty, and unavailable states.
- Remove the Discussions section from Changes while retaining its New change
  session entry. Unscaffolded planning sessions are discoverable in Chats.
- Give each chat a stable project-prefixed link that resumes the exact session.
  Support reload/back behavior and modified-click/plain-link navigation.
- Extend the location trail to Chats and the open conversation's title. Back
  from a standalone conversation returns to Chats; opening from another app
  page still works.
- Refresh list metadata without clobbering transcript state or unsent drafts.
  Preserve the hand-written template/vanilla JS/CSS structure and responsive
  chat shell.

## Files affected

- `internal/server/server.go` and `internal/server/render.go`
- New chat-page handler/tests under `internal/server/`
- `internal/server/render_test.go` and hub/base-path tests
- New `web/templates/chats.html`
- `web/templates/index.html` and `web/templates/layout.html`
- `web/templates/opencode.html` and `README.md`
- `web/static/app.js` and `web/static/app.css`
- `web/chat_queue_test.mjs` or a focused new browser-glue test with a Go wrapper

## Verification

- Assert Chats page/nav presence, active-route state, Changes-only content,
  setup/projects guards, chat-card links, and meaningful empty/error states.
- Assert that New chat appears only in Chats, New change session only in Changes,
  and the Settings gear has an accessible name and active state on Settings and
  service-management pages, with both single-project and hub-prefixed links.
- Verify opening/resuming a card, New chat, breadcrumbs, back/reload, modified
  clicks, and title/activity refresh without lost drafts.
- Test empty and `/p/example` base paths and mobile/desktop layout behavior.
- Build embedded assets before any manual browser checks; run focused renderer,
  routing, and browser-glue tests, then workflow validation.
