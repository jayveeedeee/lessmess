# MAC-02: Parallel mobile chat interface

## Objective

Add a touch-friendly Chat interface beside Terminal and make Chat the practical
session experience on narrow screens without removing or weakening the desktop
terminal.

## Dependencies

- MAC-01

## Scope

- Full-screen chat overlay with transcript, status, composer, send, interrupt,
  close, and task-context controls.
- Chat actions anywhere sessions are opened; Terminal remains explicitly
  available for the same session.
- Initial fetch and non-overlapping polling only while visible, with clean stop
  on close/background and immediate convergence after reconnect.
- Draft preservation, intentional auto-scroll, duplicate-submit prevention,
  keyboard submission rules, accessible labels, and touch targets.
- Responsive task context: side panel on wide screens and a collapsible drawer
  on narrow screens.

## Implementation steps

1. Add a chat overlay to the shared layout without changing the terminal
   overlay IDs or behavior. Add clear Chat and Terminal entry actions to session
   rows and subagent controls.
2. On narrow viewports, make Continue and successful session auto-open flows use
   Chat; retain current Terminal behavior on desktop and respect the existing
   auto-open setting as the gate.
3. Implement open/close, snapshot polling, prompt submission, interrupt,
   permission replies, and form replies in focused vanilla JavaScript. Ensure
   only one poll is in flight and use page visibility to pause work.
4. Preserve the composer outside transcript swaps. Keep the reader's scroll
   position unless already near the bottom; announce new activity accessibly.
5. Reuse the existing terminal task-panel source, adapting it to an on-demand
   mobile drawer and a desktop side pane rather than creating another task data
   endpoint.
6. Add responsive CSS for 375px/390px portrait, safe-area insets, long code and
   tool output, and virtual-keyboard-friendly viewport sizing.
7. Extend render/client tests for required hooks and add browser-level or DOM
   tests where the repository's current tooling permits; do not introduce a JS
   build system solely for testing.

## Verification

- Existing terminal WebSocket tests and terminal DOM-hook assertions pass.
- At 375px and 390px widths there is no page-level horizontal overflow, all
  actions are reachable, and the task drawer does not cover the composer.
- Manual run covers prompt, live refresh, interrupt, permission, form, scroll-up
  refresh, background/resume, close/reopen, and switching between Chat and
  Terminal on the same session.

## Completion criteria

A phone user can operate an active lessmess/OpenCode session end to end without
using the terminal, while a desktop user can continue using the terminal exactly
as before.

## Files affected

- `web/templates/layout.html`
- `web/templates/index.html`
- `web/templates/board.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`
- `web/AGENTS.md` and `web/templates/AGENTS.md` as needed

## Notes

Do not introduce a new UI framework, router, state store, or persistent chat
preference. Viewport-based defaulting plus explicit Chat/Terminal actions is
enough for this change.
