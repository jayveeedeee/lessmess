
# LIF-03: Board lifecycle buttons

Status: see [../ledger.md](../ledger.md).

## Objective

Add Close change / Reopen and Commit buttons to the board header with confirm logic and terminal auto-open.

## Dependencies

LIF-01, LIF-02.

## Scope

In scope: board template buttons (state-aware Close ↔ Reopen), app.js handlers (POST + board refresh; confirm when closing with incomplete tasks; commit → open terminal on the returned session).
Out of scope: endpoints (prior tasks).

## Implementation steps

1. Template: render "Close change" when overall ≠ `Done`, "Reopen" when `Done`; Commit button.
2. app.js: close/reopen POST then refresh board; confirm dialog listing incomplete-task count when applicable.
3. commit POST → `openTerminal(session)` on response; error alerts.
4. Render tests for button states; screenshot both states.

## Verification

- Render tests pass; screenshots inspected; live click-through in LIF-04.

## Completion criteria

- Buttons present, state-aware, wired to endpoints.

## Files affected

- `web/templates/board.html`, `web/static/app.js`, possibly `app.css`

## Notes

- None yet.
