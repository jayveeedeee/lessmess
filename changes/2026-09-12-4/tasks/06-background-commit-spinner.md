
# LIF-06: Background commit with button spinner

Status: see [../ledger.md](../ledger.md).

## Objective

Change the commit flow UX: no terminal pop-up. Clicking Commit disables the button and shows a spinner while the agent works; the spinner stops (button re-enables) when the opencode session goes idle.

## Dependencies

None.

## Scope

In scope: opencode client `WaitDone` (`POST /api/session/{id}/wait`, 204 when idle); `GET /changes/{id}/commit-status?session=` endpoint (short-timeout wait → `{done: bool}`); app.js spinner/disable/poll/re-enable logic (no `openTerminal`); CSS spinner + disabled state.
Out of scope: showing the commit hash, mapping changes (commit sessions stay mapped and openable via the Sessions panel).

## Implementation steps

1. Client: `WaitDone(ctx, id) error` — POST wait, nil on 204.
2. Server: commit-status endpoint — 2s wait; nil → done true; timeout → done false.
3. Tests: fake service (immediate 204 → done true; slow response → done false).
4. app.js: on click — disable button, start spinner, poll every 3s; on done — stop, re-enable, transient ✓ label; on error — stop, re-enable, alert. Remove the `openTerminal` call.
5. CSS: spinner element inside the button; disabled styling.
6. Live verify: prime a trivial session, watch busy → done transition via the endpoint.

## Verification

- Tests pass; live busy→done transition observed; button behavior confirmed.

## Completion criteria

- Commit runs in the background; button reflects busy/idle correctly without opening the terminal.

## Files affected

- `internal/opencode/client.go`, `internal/server/lifecycle.go`, `web/static/app.js`, `web/static/app.css`

## Notes

- Completion detection: `POST /api/session/{id}/wait` — verified live that it blocks for the duration of real work (15.4s during a count-to-60 turn) and returns 204 only when the session goes idle. Endpoint maps a 2s server-side wait to `{done}`.
- Verified: `TestCommitStatusEndpoint` (204 → done), `TestCommitStatusBusy` (slow wait → busy, endpoint caps at its 2s timeout), `TestCommitStatusNoService`; live probe — trivial turn reported done at t+2s, long turn blocked wait for 15.4s.
- UI: Commit button disables + shows CSS spinner + "Committing…" on click; polls `commit-status` every 3s (max ~10 min with a guard); on done shows "✓ Committed" for 3s then restores; on error restores and alerts. The terminal no longer opens automatically; the commit session remains mapped and openable via the Sessions panel.
- Note: the hashed asset URLs (LIF-05) mean this app.js change actually reaches the browser on next server restart.

