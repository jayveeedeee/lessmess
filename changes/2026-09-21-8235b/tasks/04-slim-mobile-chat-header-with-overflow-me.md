# MCL-04: Slim mobile chat header with overflow menu

Status: see [../ledger.md](../ledger.md).

## Objective

Reduce permanent mobile header actions: navigation, title, and session state
stay visible; Terminal, Controls (lifecycle, session management), and other
rare actions move into an overflow menu; Work or Agents appear only when
relevant.

## Dependencies

- MCL-00 (view shell the header belongs to).
- MCL-01 (the Work entry point this header shows contextually).

## Scope

- `web/templates/layout.html`: a `#chat-more-btn` overflow button and its
  menu container in `.chat-head`; existing button ids (`chat-agents-btn`,
  `chat-controls-btn`, `chat-terminal-btn`, `chat-tasks-btn`→Work) remain,
  so `app.js` and contract tests keep their hooks.
- `web/static/app.js`: overflow menu open/close (Escape/backdrop safe),
  routing menu items to their existing handlers; contextual visibility rules
  for Work and Agents preserved (`setChatTasks`, descendants logic).
- `web/static/app.css`: compact-width header layout — brand/title +
  state first, contextual actions (Work, Agents), overflow last; desktop
  header unchanged.

## Implementation steps

1. Under the compact breakpoint only, move Terminal and Controls out of the
   header row into the overflow menu (plain list, 44px rows, labelled); the
   menu opens from `#chat-more-btn`, closes on selection, Escape, or backdrop
   tap, and hands focus back to the button.
2. Keep Work and Agents in the header when relevant (session bound to a
   change / live descendants respectively) and hidden otherwise, exactly
   mirroring today's `hidden` rules.
3. Keep title truncation and session state visible: `#chat-title` and the
   status line never sacrifice space to actions; brand icon/project name stay
   hidden at compact widths as today.
4. Desktop keeps the current explicit buttons — the overflow menu exists only
   within the compact media query (CSS) and its handler is width-agnostic but
   unreachable on desktop.
5. Update the chat UI contract tests: `#chat-more-btn` present; existing ids
   still asserted so no handler silently orphans.

## Verification

- `node --check web/static/app.js`; render/contract tests pass.
- Manual pass at 375px/390px/phone landscape: header shows title + state +
  (Work | Agents when relevant) + overflow; Terminal and Controls reachable
  from the menu; every menu item opens its existing surface; desktop header
  is pixel-identical to today.

## Completion criteria

Compact widths get a calm, single-row header with only relevant actions
visible and everything else one tap away in an overflow menu; desktop is
unchanged.

## Files affected

`web/templates/layout.html`, `web/static/app.js`, `web/static/app.css`,
`internal/server/render_test.go`.
