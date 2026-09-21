# MAC-30: Desktop single-auxiliary-panel rule and conversation min-width

## Objective

Allow at most one Chat auxiliary panel on desktop and preserve a readable
minimum conversation width. Terminal remains unchanged.

## Dependencies

- None.

## Scope

- Arbitrate Tasks/Work, Agents, and Controls so newest-opened wins.
- Cap panel widths and give the conversation a firm minimum width.
- Leave each panel's close behavior and the Terminal overlay intact.

## Implementation steps

1. Centralize desktop auxiliary-panel opening and closing.
2. Route every panel entry point through the arbiter.
3. Add minimum/capped width CSS for common desktop sizes.
4. Add UI contract assertions for arbitration.

## Verification

- Exercise panel switching at 1024px and 1280px.
- Verify Terminal open/input/resize/close behavior is unchanged.

## Completion criteria

Desktop Chat never displays competing auxiliary panels or squeezes the
conversation below its readable minimum.

## Files affected

- `web/static/app.js`
- `web/static/app.css`
- Chat UI contract tests

## Notes

This task is independent of the compact view stack.
