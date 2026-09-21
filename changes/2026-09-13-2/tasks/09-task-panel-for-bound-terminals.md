
# SET-09: Task panel for change-bound terminals on any page

Status: see [../ledger.md](../ledger.md).

## Objective

A terminal whose session is bound to a change shows the task panel no
matter which page it was opened from — the settings page's Change-button
terminal included — instead of only on board pages. Reported gap: a
settings discussion that scaffolds a change mid-terminal never shows its
tasks.

## Dependencies

- SET-08

## Scope

- New endpoint `GET /api/sessions/{sessionID}/change` → `{"change": id}`
  (empty when unassigned), from the existing session→change mapping.
- `app.js`:
  - `syncTerminalTasks` generalized to mirror from any board-fragment
    source element plus an explicit change id (board-page behavior
    unchanged).
  - `openTerminal` on a non-board page resolves the session's binding via
    the new endpoint; when bound, it fetches `/changes/{id}` (HX fragment)
    into an off-DOM source and fills + unhides the panel.
  - Live pickup: on SSE `fs`/`write` debounce, an open panel re-fetches
    its fragment, and an unbound open terminal re-checks the binding — so
    a settings discussion that scaffolds mid-terminal grows its panel
    when the change appears (scaffold writes `changes/`, which fires the
    event).
- Handler test for the endpoint; `node --check` for the JS.

## Implementation steps

1. `sessionChange` handler in mapping.go + route; tests (bound,
   unassigned, unknown).
2. JS: tstate carries the session id; panel fill from fetched fragment;
   SSE hooks.
3. Full suite + `node --check`; rebuild + restart; verify live against a
   real change-bound session.

## Verification

- Endpoint returns the binding (verified live against the session bound
  to 2026-09-13-3); suite green.

## Completion criteria

- Opening a terminal for a change-bound session from the settings page
  (or Discussions) shows the task panel; board-page behavior unchanged;
  unassigned terminals stay full-width.

## Files affected

- `internal/server/mapping.go`, `internal/server/server.go`
- `internal/server/mapping_test.go` (or new test)
- `web/static/app.js`

## Notes

- The board fragment (`GET /changes/{id}` with an HX request) is already
  the panel's data contract on board pages; this reuses it off-board
  instead of inventing a JSON tasks API.
- The validation violation the user saw alongside this (root vs change
  ledger status for 2026-09-13-3) is a separate, pre-existing ledger-sync
  gap — fixed ad hoc in the root ledger; a durable fix is a candidate
  follow-up change, out of scope here.
- Verified 2026-09-13: `GET /api/sessions/ses_f65f409eaffeOIZVAc1Z6MSls2/change`
  (the user's settings-initiated session) returns `2026-09-13-3` live; the
  HX board fragment for that change renders its task card (DQP-00);
  `TestSessionChangeEndpoint` covers bound/unknown; full suite green;
  `node --check` clean; server rebuilt + restarted; `lessmess validate`
  exit 0 (root row for 2026-09-13-3 synced to In progress).
- Gotcha recorded for future curls: the board handler needs
  `Accept: text/html` AND `HX-Request: true` to serve the fragment; a
  bare curl gets JSON.