---
id: STS-01
title: Board status control
---

# STS-01: Board status control

Status: see [../ledger.md](../ledger.md).

## Objective

Give the board a minimal control that sets Planned / In progress / Blocked via
the new endpoint, so humans get the same deterministic path agents do.

## Dependencies

STS-00 (the endpoint must exist and respond before the UI can call it).

## Scope

- A small control on the change board header (next to the existing Close
  affordance) offering the three accepted statuses; the current status is
  preselected/disabled.
- Calls `POST /changes/{id}/status`; surfaces 409/4xx responses as non-intrusive
  errors; relies on the existing SSE `write` event to refresh the view.
- No index-page, modal, or task-card changes.

## Implementation steps

1. Render the control in the board template (`web/templates`), populated with the
   change's current overall status; disable the option equal to the current
   status.
2. Wire the submit in `web/static/app.js` to `POST /changes/{id}/status` with
   `{"status": ...}`, following the existing close/reopen fetch patterns
   (including error toast/handling conventions).
3. Confirm the existing SSE full-reload path repaints the board after the write
   event; no new event kinds needed.
4. Extend board render/interaction tests for the new control's markup and happy
   path.

## Verification

- `go vet ./... && go test ./...` green.
- Manual browser pass: switch In progress → Blocked → In progress; both ledger
  files update; `lessmess validate` stays clean; Close button still the only path
  to Done.

## Completion criteria

- Board can set the three statuses without page navigation or hand-editing files.
- Done is not reachable from the new control.

## Files affected

- `web/templates` (board template)
- `web/static/app.js`
- Board-related server tests

## Notes

- Keep the control visually consistent with the existing board header buttons;
  the status pill remains read-only display.
