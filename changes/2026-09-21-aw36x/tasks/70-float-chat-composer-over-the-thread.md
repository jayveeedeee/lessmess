# MAC-70: Float Chat composer over the thread

## Objective

Float the unified Chat composer over the scrollable conversation so thread
content remains visible and scrollable through its transparent outer margins.

## Dependencies

- MAC-69

## Scope

- Overlay the composer at the bottom of the Chat thread.
- Extend the transcript beneath the composer.
- Track the composer's dynamic height for safe transcript bottom clearance.
- Let pointer and touch input pass through transparent composer margins.
- Stack status and pending follow-ups above the floating composer.

## Implementation steps

1. Position the composer and supporting status UI over the transcript.
2. Synchronize a composer-height CSS property with `ResizeObserver`.
3. Use that property for transcript scroll clearance and overlay placement.
4. Keep only visible composer children interactive.
5. Update rendering and behavior contracts.

## Verification

- Run focused Chat composer tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

Conversation content visibly scrolls behind the composer's outer margins while
the latest content can still be scrolled clear of the input field.

## Files affected

- `web/static/app.css`
- `web/static/app.js`
- `internal/server/render_test.go`

## Notes
