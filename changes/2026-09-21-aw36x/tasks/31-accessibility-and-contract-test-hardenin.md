# MAC-31: Accessibility and contract-test hardening across breakpoints

## Objective

Harden the completed layout across compact and desktop widths for focus,
keyboard, touch, safe-area, and screen-reader behavior.

## Dependencies

- MAC-25.
- MAC-26.
- MAC-27.
- MAC-28.
- MAC-29.
- MAC-30.

## Scope

- Focus entry/return and top-most Escape/backdrop behavior for every surface.
- Accurate ARIA state and no tabbable controls in inactive views.
- 44px targets, safe-area insets, visible focus, and keyboard-safe sticky UI.
- Consolidated structural contracts and documented phone-width matrix.

## Implementation steps

1. Walk Work, reading, agents, controls, overflow, and composer action views.
2. Fix labels, focus order, ARIA state, and z-index/safe-area issues.
3. Consolidate render and JavaScript contract tests.
4. Run full automated and available phone-width checks.

## Verification

- `go vet ./... && go test ./...`.
- `node --check web/static/app.js`, static build, and `lessmess validate`.
- 375px, 390px, landscape, compact split, keyboard-only, and screen-reader
  checklist with any unavailable manual evidence stated explicitly.

## Completion criteria

The single-pane hierarchy is accessible and regression-pinned at its supported
breakpoints without changing Terminal behavior.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`
- Relevant Chat UI contract tests

## Notes

Do not claim manual evidence that was not run.

## Evidence

- Hardened compact Work, Agents, Controls, Contents, overflow, composer actions,
  reference picking, and reading-detail transitions so the top-most surface
  owns Escape, backdrop dismissal, focus entry, and focus return.
- Synchronized `hidden`, `aria-hidden`, `aria-expanded`, and `inert` state;
  trapped focus inside Chat; kept inactive views out of the tab order; and
  added accessible control/chip labels and polite status regions.
- Added compact 44px target coverage, safe-area-aware spacing, visible focus,
  and `visualViewport` Chat sizing while retaining the 840px breakpoint and
  the desktop one-auxiliary-panel arbiter.
- Pinned per-session draft, attachment, reference, skill, transcript scroll,
  and per-view scroll storage plus the unchanged Terminal/xterm resize hooks in
  the consolidated render/static contract test.
- Passed `gofmt`, `go test ./...`, `go vet ./...`,
  `node --check web/static/app.js`, `CGO_ENABLED=0 go build`,
  `git diff --check`, and the newly built binary's `lessmess validate`.

## Manual gaps

- No browser/device session was available, so 375px, 390px, phone landscape,
  compact split, on-screen-keyboard, and 1024px/1280px visual checks were not
  run manually.
- Keyboard-only browser traversal and screen-reader announcements were not run
  manually. Automated structural contracts cover their DOM, ARIA, focus, and
  target-size prerequisites but are not substitutes for those checks.
