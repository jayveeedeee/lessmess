# PILL-00: Composer pill markup and styles

## Why

The busy composer's queue/steer actions need a home: the plan calls for the
round stop button to expand into a three-segment pill (stop / steer / queue)
with 2px dividers and an expansion animation. The markup and styling must exist
before the behavior can be wired.

## What

- `web/templates/layout.html`: wrap `#chat-send-btn` in a `#chat-action-pill`
  container and add `#chat-steer-btn` (directional glyph) and
  `#chat-queue-btn` (stacked lines ending in a right-pointing arrow) segments — queue as `type=submit`,
  steer and stop as `type=button`, all accent-styled icon-only buttons (no
  visible words) with `aria-label`/`title`.
- `web/static/app.css`: pill container (44px height, circle when collapsed,
  pill when expanded), 2px vertical dividers, segment glyphs matching the
  existing `.chat-action-button` pattern, expansion/collapse animation
  (width + border-radius morph, segment fade/slide, divider scale),
  `prefers-reduced-motion: reduce` instant swap, and ≤840px sizing. Keep both
  ends at a fixed 22px radius while width changes; animating `50%` radius
  stretches the circle before it becomes a pill.

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`

## Verification

- Collapsed busy state renders the round stop button visually identical to the
  current UI; expanded state renders three segments with dividers.
- `go build` succeeds and the rebuilt binary serves the new markup (assets are
  embedded, so a rebuild is required to see anything).
