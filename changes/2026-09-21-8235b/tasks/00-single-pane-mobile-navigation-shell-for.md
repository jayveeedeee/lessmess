# MCL-00: Single-pane mobile navigation shell for the chat overlay

Status: see [../ledger.md](../ledger.md).

## Objective

Make Chat the only persistent pane at compact widths (≤ ~840px, including
phone landscape and split views). Every other chat surface — Work, plan, task,
agents, controls/session management — becomes a temporary full-width view with
explicit Back to Chat, so no sidebar competes with the conversation.

## Dependencies

- None (foundation for MCL-01, MCL-02, and MCL-04).

## Scope

- `web/static/app.css`: a single compact-width breakpoint (~840px) owning the
  chat shell — fold the existing `@media (max-width: 720px)` chat block
  (drawers, bottom sheet, safe-area rules) into it; full-width view styling.
- `web/static/app.js`: a small view-stack controller for the chat overlay —
  the transcript/composer pane is the root; opening Work, agents, controls,
  or a reading view pushes a view, Back/Escape pops it, and each view records
  its return position.
- `web/templates/layout.html`: only the structural hooks the shell needs
  (view containers, Back affordances); no behavior rewrites here.

## Implementation steps

1. Introduce the ~840px compact breakpoint in `app.css` and move the current
   720px chat rules (drawers, controls sheet, composer, touch targets,
   safe-area padding) under it, converting `#chat-agents` and
   `#chat-controls-sheet` from drawer/sheet styling to full-width views;
   `#chat-tasks` disappears as a drawer here (MCL-01 replaces its content).
2. Add the view controller in `app.js`: `openChatView(name)`/`closeChatView()`
   toggling a `data-chat-view` attribute on `.chat-window` (CSS drives which
   pane is visible), an `aria-hidden`/focus handoff per view, and Back buttons
   wired to the controller.
3. Preserve state across moves: transcript scroll per session already lives in
   `cstate.scrolls` — keep it updated on every view push/pop; drafts, skill
   chips, and draft files stay in the (never re-rendered) composer; record and
   restore each view's return position so Back lands where the user left.
4. Keep Escape and backdrop dismissal working per view (close the top view
   first, not the whole overlay), matching the existing `#detail`-before-
   overlay Escape precedent.
5. Render-test the structural contract: view containers/Back hooks present in
   `layout.html`, `data-chat-view` handling present in `app.js`.

## Verification

- `node --check web/static/app.js`; `go test ./internal/server/...` for the
  render contracts.
- Phone-width manual pass at 375px, 390px, and phone landscape: Chat is the
  sole persistent pane; opening each surface takes the full width; Back
  returns to Chat with transcript scroll, draft text, and chips intact.
- Escape closes the top view first; the overlay close button still closes
  Chat entirely.

## Completion criteria

At compact widths the chat overlay is a single-pane view stack with consistent
Back navigation and no state loss between views; desktop (above the
breakpoint) renders exactly as before.

## Files affected

`web/static/app.css`, `web/static/app.js`, `web/templates/layout.html`,
`internal/server/render_test.go`.
