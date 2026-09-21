
# EXD-03: Explorer JS — selection state and refresh restore

Status: see [../ledger.md](../ledger.md).

## Objective

Extend the explorer behavior in `app.js`: track the selected directory,
highlight it, and preserve both open nodes and the selection (plus the
visible detail) across live tree refreshes.

## Dependencies

- EXD-01 (markup hooks: `.xdir` summaries, `#explorer-detail`, `data-rel`)

## Scope

- `web/static/app.js` (explorer block only).

## Implementation steps

1. Track selection in a variable (default `"."`). Delegated click on
   `.xtree summary.xdir` sets it and moves the `.selected` class to that
   summary. (htmx performs the detail fetch itself; JS only manages state.)
2. Keep the existing `.explorer-chat` handler untouched — its
   preventDefault/stopPropagation already keeps chat clicks from selecting
   or toggling.
3. `refreshExplorerTree`: in addition to open `data-rel`s, capture the
   selected rel; after swapping, restore open nodes and re-apply `.selected`
   (fall back to root if the node vanished).
4. After a tree refresh, re-fetch `/explorer/detail?dir=<selected>` into
   `#explorer-detail` so the detail pane tracks docs changes too.
5. Initial page state needs no JS (root open + detail server-rendered), but
   ensure the root summary carries `.selected` on first load — either from
   the template or by JS init.

## Verification

- Load `/explorer`, click around: highlight follows the click, detail swaps.
- Touch a covered dir's STRUCTURE.md (or run a docs refresh) while a
  non-root directory is selected: tree re-renders, same nodes stay open,
  same row stays selected, detail content updates.
- Chat buttons in tree rows and detail header still open the terminal
  overlay without toggling or selecting.

## Completion criteria

- Selection survives tree refreshes with root fallback; detail re-fetches on
  `docs` events; no regressions to chat or other pages' JS.

## Files affected

- `web/static/app.js`

## Notes

- Explorer JS keeps the delegated-listener, plain-`var` style.
- Two findings from implementation:
  1. Tree fragments swapped via `innerHTML` must go through `htmx.process`
     or the new summaries' `hx-get` never fires.
  2. htmx attaches its click listener directly to each summary, so the chat
     handler moved to the capture phase — `stopPropagation` there keeps chat
     clicks from also triggering a detail fetch (bubble phase ran too late).
- Root summary carries `selected` from the template; JS normalizes on
  refresh and falls back to root when the selected dir disappears.
- Verified 2026-09-12: `node --check` clean, rebuilt server serves the new
  script (content-hashed `?v=`), and headless screenshots confirm selection
  styling; `TestExplorerChatHappyPath` still passes for the chat flow.
