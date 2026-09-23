# MAC-44: Remove busy delivery mode controls

## Objective

Remove the composer delivery-mode row and its explanatory text from Chat.

## Dependencies

- MAC-43.

## Scope

- Remove the visible Queue/Steer selector shown while OpenCode works.
- Silently queue busy-session follow-ups for the next turn.
- Preserve capability gating for attachments, skills, and follow-up delivery.
- Leave pending inbox item management unchanged.

## Implementation steps

1. Remove delivery controls from the Chat template and CSS.
2. Keep busy-state synchronization without depending on removed elements.
3. Send `queue` as the fixed delivery mode and update contracts.

## Verification

- Focused/full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

No `While OpenCode is working`, Queue, Steer, or delivery explanation row is
visible in the composer.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

Queue is the conservative default because it does not alter the active turn.
