
# DSC-02: Keep the terminal open when a discussion scaffolds

Status: see [../ledger.md](../ledger.md).

## Objective

When a discussion session opened from the index page scaffolds a change, the terminal overlay must not close. Today the index page's SSE handler full-reloads the page, destroying the terminal; instead the browser should follow the session to its new board, where the existing `?session=` auto-open flow reconnects the same session with its task panel.

## Dependencies

- None (independent of DSC-00/DSC-01 code, but verified on the restarted server from DSC-01).

## Scope

- Client JS only: the SSE `scheduleRefresh` handler and a new follow-the-session helper in `web/static/app.js` (docs-excluded).
- No server, template, endpoint, or workflow changes.

## Implementation steps

1. In `scheduleRefresh` (`web/static/app.js:106-120`), guard the index branch: when `terminalOpen() && tstate.session`, do **not** `location.reload()`; instead call the new `followSession()`. Keep `checkValidation()` running either way.
2. Add `followSession()`: fetch `/api/sessions/{tstate.session}/change`; if a change comes back, `location.assign("/changes/" + change + "?session=" + tstate.session)` — `autoOpenSession` (`app.js:776-782`) then reopens the terminal on the board and strips the query param. If still unassigned, do nothing (no reload; the next event or navigation refreshes).
3. Leave the board-page branch (`refreshBoard()`) unchanged — fragments already preserve the terminal.
4. On `closeTerminal()`, if `page === "index"`, trigger one `scheduleRefresh()` so a deferred refresh happens once the overlay is gone and a reload is safe again.

## Verification

- Live: open a new change session from the index, have the agent scaffold a throwaway change; observe no reload/kick-out — the browser lands on `/changes/{id}?session=…`, the board renders, and the terminal reconnects to the same session with the task panel visible.
- Live regression: with a terminal open on the index, confirm unrelated board activity no longer reloads the page (stale list acceptable until navigation), and that closing the terminal restores refresh behavior.
- `go vet ./... && go test ./...` stays green (JS-only change, but cheap gate).

## Completion criteria

- Scaffold from an index-page discussion lands on the new change's board with the same session open — no manual navigation, no terminal loss.
- No console errors during the transition; `?session=` param stripped after auto-open.

## Files affected

- `web/static/app.js`

## Notes

- Root cause (verified 2026-09-14): `scaffoldChange` writes `changes/` → store watcher → SSE `fs`/`write` → `app.js:111` `location.reload()` on the index; the terminal-panel re-resolution at lines 112–118 never runs because the reload destroys the JS context first. Board pages were immune (fragment swap).
- Deliberate trade-off: while a terminal is open on the index, background `changes/` writes no longer refresh the list — self-heals on navigation or terminal close. If acceptance finds this too stale, follow-up option is a fragment-swapped index list (bigger change, not in scope).
