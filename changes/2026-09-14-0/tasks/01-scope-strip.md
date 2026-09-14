---
id: SPS-01
title: Settings scope strip with segmented control
---

# SPS-01: Settings scope strip with segmented control

Status: see [../ledger.md](../ledger.md).

## Objective

Give the settings page a permanent, cleanly styled scope row directly below the
top menu bar: a segmented Personal/Project control, labels exactly "Personal" and
"Project" with no further text, sticky beneath the header so it never scrolls away.

## Dependencies

SPS-00 (the strip sticks below the now-sticky header; its `top` offset assumes
the header's 52px height).

## Scope

- `web/templates/settings.html` — replace `.settings-head`'s inline radios with a
  new strip element (e.g. `<div class="settings-scopebar">`) placed so it renders
  directly under the global header, above the settings body. Keep
  `role="radiogroup"`, keep `input[type="radio"][name="settings-scope"]` with
  values `project`/`personal` (default `project` checked); labels become exactly
  `Personal` / `Project` (drop the muted parenthetical spans).
- `web/static/app.css` — strip styles: surface background, bottom border, sticky
  `top: 52px`, z-index just below the header; segmented control: pill pair in a
  bordered rounded container, active segment accent-filled (existing pill idiom),
  native radio dots visually hidden, `:focus-visible` ring on the active label,
  hover state; remove the old `.scope-toggle` rule.

## Implementation steps

1. Restructure the scope markup in `settings.html` per scope above; remove the
   `h1` and radios from `.settings-head` (the `h1` itself is relocated by
   SPS-02 — interim placement directly above the body is fine).
2. Add the strip + segmented-control CSS; delete superseded `.settings-head` /
   `.scope-toggle` rules that no longer apply.
3. Ensure `initSettings`' `input[name="settings-scope"]` change wiring still
   switches layers (no JS edit expected — verify badges/placeholders re-render).
4. Check both themes and keyboard tab/arrow navigation within the radiogroup.

## Verification

- Rebuild the binary and load `/settings`: strip sits directly below the top menu
  bar and stays pinned while scrolling.
- Toggling Personal/Project re-renders field values, placeholders, and source
  badges; saving writes the selected layer (`PUT /api/settings?scope=...`).
- `go test ./...` — `render_test.go` assertions (`name="settings-scope"`,
  `id="settings-page"`) still pass without test edits.

## Completion criteria

- Strip is radios-only, sticky under the header, labels exactly Personal/Project.
- Segmented control styled consistently with the app's pill idiom in both themes.
- Scope switching and save behavior unchanged; tests green.

## Files affected

- `web/templates/settings.html`
- `web/static/app.css`

## Notes

- Whether the strip spans the full viewport width or the content column is a
  visual judgment call during review; the plan accepts either, preferring the
  cleaner option. Implemented as full-bleed via negative margins canceling
  `main`'s 1.25rem padding, sticky at `top: 52px`, z-index 19.
- Scope adjustment: both `.settings { max-width }` caps (720px/860px) were
  removed in this task rather than SPS-02 because the flush strip requires full
  width; recorded in the ledger decision log.
- Evidence (2026-09-14): served HTML contains `settings-scopebar`/`scope-seg`
  with both radios' names, values, and `checked` state intact, so `initSettings`
  wiring is untouched; server tests green with zero test edits; keyboard path
  kept via opacity-hidden radios (`:focus-visible` ring on the pill).
