
# CHAT-01: UI: header Chat button and terminal wiring

Status: see [../ledger.md](../ledger.md).

## Objective

Put a **Chat** button in the header's right nav next to Settings that spawns a chat
session and opens it in the terminal overlay.

## Dependencies

- CHAT-00 (the `POST /chat/session` endpoint this button calls).

## Scope

- `web/templates/layout.html`, `web/static/app.js`; `web/static/app.css` only if the
  button needs more than the existing nav-link styling; a render test.

## Implementation steps

1. `layout.html`: inside `topnav-right` and before the Settings link, add a
   `<button id="chat-btn" type="button" title="Chat with the codebase">Chat</button>`.
   It inherits the `{{if ne .Page "setup"}}` guard already around the topnavs, so the
   setup page stays chrome-free. Keep it a button (it spawns, not navigates) styled to
   match the nav links — the docs bell is the precedent for a button in the header row;
   extend `app.css` only if needed.
2. `app.js`: a delegated click handler for `#chat-btn` following the explorer-chat
   pattern — disable while pending, `fetch("/chat/session", {method:"POST", headers:
   {"Content-Type":"application/json", Accept:"application/json"}})`, on success
   `openTerminal(j.session, j.title)`, on failure `alert(j.error || …)` in the
   handler's `catch`, `finally` re-enables. The terminal opens unconditionally: a
   Chat click is a deliberate open, so `session.autoOpenTerminal` never gates it
   (that gate is for programmatic opens like the discussion form) — same reasoning
   as the explorer chat. The handler must work on every page (index, board,
   explorer, settings) since it lives in the layout — wire it in the shared init
   path, not a page-specific one.
3. Render test: assert the header renders `#chat-btn` on a non-setup page and that the
   setup page omits it (the guard is shared with the navs — pin it).

## Verification

- `node --check web/static/app.js`.
- Render test for presence/absence per page.
- Manual: click Chat from the Changes page and from Settings — a terminal overlay opens
  on a fresh "Codebase chat …" session; with the opencode service stopped, the click
  alerts the 503 error and the button recovers.

## Completion criteria

Chat is one click away on every page, opens a live chat terminal, fails gracefully
without stranding the button, and never renders on the setup page.

## Files affected

`web/templates/layout.html`, `web/static/app.js`, `web/static/app.css` (if styling is
needed), a render test.
