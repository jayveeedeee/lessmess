
# MTOC-02: Client TOC builder with click-to-scroll and scroll-spy

Status: see [../ledger.md](../ledger.md).

## Objective

Populate the modal's TOC pane from the rendered document after each swap:
h2 entries with h3 nested beneath, click-to-scroll, and a scroll-spy that
highlights the section in view. Hide the pane entirely for short documents.

## Dependencies

- MTOC-00 (heading ids exist server-side).
- MTOC-01 (the `.modal-toc` pane and `.has-toc` width class exist).

## Scope

- `web/static/app.js` — after-swap hook for `#detail` plus scroll/preview
  wiring.
- `web/static/app.css` — TOC link and active-state styling.

## Implementation steps

1. Extend the existing `htmx:afterSwap` listener: when the swapped target is
   `#detail`, run `buildDetailTOC()`.
2. `buildDetailTOC()`: collect `.modal-body h2, .modal-body h3`; if fewer
   than 2, keep the pane hidden and remove `has-toc`; otherwise populate the
   nav with anchor links (reuse goldmark's ids; fall back to generating a
   slug + setting the id), add `has-toc` to `.modal`, unhide the pane.
3. TOC click: `scrollIntoView({behavior:"smooth", block:"start"})` on the
   target heading inside `.modal-body`; prevent the default hash jump.
4. Scroll-spy: rAF-throttled scroll listener on `.modal-body` picking the
   last heading above the pane top; toggles an `active` class on the matching
   TOC link.
5. Styling: small muted links, h3 indented, `active` in the accent color
   with a left indicator; `--border`/`--muted`/accent vars consistent with
   the rest of the file.

## Verification

- Rebuild + restart; open this change's plan modal (many h2/h3): TOC lists
  sections in order, h3 indented, click scrolls smoothly, spy tracks while
  scrolling.
- Open a short task (few sections): no TOC pane, 680px modal.
- Terminal task panel still opens task modals correctly (same `#detail`
  target).

## Completion criteria

- TOC builds on every `#detail` swap, scroll-spy works, short docs hide the
  pane, no console errors.

## Files affected

- `web/static/app.js`, `web/static/app.css`.

## Notes

- The after-swap hook is the only integration point; direct `innerHTML`
  writers into `#detail` do not exist today (all detail loads are htmx).
- Verification evidence (2026-09-15): `node --check app.js` clean; heading
  ids confirmed in served HTML (`<h2 id="objective-and-context">` etc.);
  TOC/scroll-spy are client-side and need the user's browser click-through
  (part of MTOC-04 handoff).
