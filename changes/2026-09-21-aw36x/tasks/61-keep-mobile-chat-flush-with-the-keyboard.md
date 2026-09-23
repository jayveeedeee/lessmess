# MAC-61: Keep mobile Chat flush with the keyboard

## Objective

Keep the mobile Chat surface flush with the software keyboard without exposing
the board behind it.

## Dependencies

- MAC-60.

## Scope

- Track the visual viewport's top, left, width, and height.
- Ask Android Chrome to resize the layout viewport for interactive widgets.
- Reposition Chat when the keyboard resizes or pans the visual viewport.
- Use an opaque compact overlay so underlying board content cannot bleed through.
- Preserve desktop sizing and existing safe-area treatment.

## Implementation steps

1. Extend the existing visual viewport synchronizer beyond height-only sizing.
2. Set `interactive-widget=resizes-content` in the viewport policy.
3. Anchor the Chat window to the synchronized visual viewport rectangle.
4. Make the mobile overlay opaque and pin the behavior with UI contracts.
5. Run full verification and restart the LAN-bound service.

## Verification

- Compact Chat viewport JavaScript and CSS contracts.
- Full tests, vet, JavaScript syntax, build, workflow validation, and diff check.

## Completion criteria

Opening or moving the mobile keyboard leaves no exposed strip between Chat and
the keyboard, including browsers that report a non-zero visual viewport offset.

## Files affected

- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

- The fix uses the existing visual viewport resize and scroll listeners.
- Chat now follows `offsetTop`, `offsetLeft`, `width`, and `height`; height-only
  synchronization could leave a strip exposed when the browser panned the
  visual viewport above the software keyboard.
- Compact Chat also uses an opaque overlay as a defensive fallback against
  underlying board bleed-through during viewport transitions.
- Android Chrome is explicitly configured with
  `interactive-widget=resizes-content`, making the layout viewport and fixed
  Chat shell end at the software keyboard rather than only shrinking the visual
  viewport above an unchanged page.
- Verified with focused viewport contracts, full tests, vet, JavaScript syntax,
  static build, workflow validation, diff check, and LAN-served asset checks.
