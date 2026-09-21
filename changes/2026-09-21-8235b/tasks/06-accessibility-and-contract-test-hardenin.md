# MCL-06: Accessibility and contract-test hardening across breakpoints

Status: see [../ledger.md](../ledger.md).

## Objective

Sweep the reworked chat surfaces for keyboard focus, Escape/backdrop
behavior, touch targets, safe-area padding, and screen-reader labels, and
consolidate the phone-width checks into focused tests so the acceptance
criteria are pinned by the suite.

## Dependencies

- MCL-00, MCL-01, MCL-02, MCL-03, MCL-04, MCL-05 (hardening pass over the
  finished surfaces).

## Scope

- `web/static/app.js` and `web/static/app.css`: fixes surfaced by the sweep
  (focus traps/handoffs per view, `aria-expanded`/`aria-controls` accuracy,
  `aria-hidden` on inactive views, visible focus rings on 44px targets,
  safe-area insets on every new fixed surface within the z-index ladder).
- `internal/server/render_test.go` (+ any chat UI contract tests): the
  consolidated assertions for view containers, Back hooks, overflow menu,
  `+` action sheet, busy-only Interrupt, sticky Contents, and desktop
  arbitration.

## Implementation steps

1. Walk each new surface (Work, reading views, agents, controls, overflow
   menu, `+` sheet) verifying: opening moves focus into the view, Escape and
   backdrop close the top-most surface only, focus returns to the invoking
   control, and inactive views are `aria-hidden` with no tabbable content.
2. Verify labels: every icon-only control (`+`, overflow, Back, chips'
   remove buttons) has an accessible name; toggles keep `aria-expanded` in
   sync with the view controller.
3. Verify touch ergonomics: 44px minimum targets, safe-area padding on all
   new fixed/sticky surfaces, contents usable with the virtual keyboard
   raised (sticky Contents offset, composer above keyboard).
4. Consolidate the render/contract tests for everything structural, and add
   the phone-width matrix (375px, 390px, phone landscape, compact split) as a
   documented manual checklist in the task notes.
5. Run the full suites and fix whatever the sweep uncovers; no behavior
   changes beyond the fixes.

## Verification

- `node --check web/static/app.js`; `go vet ./... && go test ./...` clean.
- Screen-reader smoke pass (labels announced, view changes announced via
  focus/`aria-live` where already present).
- Phone-width manual matrix executed and recorded in the task notes;
  keyboard-only pass through open→compose→attach→send→interrupt→back.

## Completion criteria

The reworked chat passes keyboard, screen-reader, touch-target, safe-area,
and phone-width checks, with the structural contract pinned by tests.

## Files affected

`web/static/app.js`, `web/static/app.css`, `internal/server/render_test.go`.
