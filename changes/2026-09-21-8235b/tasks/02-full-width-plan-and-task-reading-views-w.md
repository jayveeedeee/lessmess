# MCL-02: Full-width plan and task reading views with sticky mobile Contents

Status: see [../ledger.md](../ledger.md).

## Objective

Plan and task documents open as full-width reading views on compact widths,
with the desktop left table-of-contents rail replaced by a sticky top
Contents disclosure that stays usable with long documents and the virtual
keyboard.

## Dependencies

- MCL-00 (compact breakpoint and view mechanics).

## Scope

- `web/static/app.css`: compact-width rules for `#detail`/`.modal` reading
  views — full width, and `.modal-toc` converted from a side rail into a
  sticky top disclosure; keyboard-overlap safety (anchor scrolling accounts
  for the sticky header).
- `web/static/app.js`: `buildDetailTOC()` gains the mobile presentation —
  same h2/h3 harvesting and scroll-spy, rendered into a `<details>`-style
  disclosure pinned above `.modal-body`; TOC link clicks close the disclosure
  and scroll the target below the sticky bar.
- No template rework expected: `partials.html` keeps the empty
  `<nav class="modal-toc" hidden>` hook that `app.js` populates (contract
  tests pin its presence in all four detail shapes).

## Implementation steps

1. Under the compact breakpoint, make the plan/task reading view full-width
   (no side rail, no reserved rail gutter), replacing the `has-toc` widened
   modal treatment.
2. Restyle `.modal-toc` as a sticky top bar with a Contents disclosure
   (summary row + collapsible link list, h3s indented as today); keep
   `[hidden]` semantics and the two-heading threshold from `buildDetailTOC`.
3. Wire scroll-spy to keep the disclosure's active link in sync without
   expanding it; on link click, collapse the disclosure and offset-scroll the
   heading below the sticky bar (virtual-keyboard safe: measure with
   `visualViewport` where available or a conservative offset).
4. Ensure the disclosure remains above content while scrolling long
   documents (z-index within the modal, no overlap by composer or chat
   header), and that touch targets meet the 44px standard.
5. Extend the render contract tests: reading-view hooks and the Contents
   disclosure ids present; `.modal-toc` still present in all four detail
   templates.

## Verification

- `node --check web/static/app.js`; render/contract tests pass.
- Phone-width manual pass: open plan and a task from Work — content fills
  the width; Contents pins to the top, expands/collapses, scrolls to
  headings with correct offset under a raised keyboard; no list, rail, or
  panel sits beside the document.
- Desktop unchanged: left rail TOC and widened modal behave exactly as
  before.

## Completion criteria

Mobile plan/task reading is a distraction-free full-width view with a
sticky, keyboard-safe Contents disclosure; desktop TOC behavior is
untouched.

## Files affected

`web/static/app.css`, `web/static/app.js`, `internal/server/render_test.go`.
