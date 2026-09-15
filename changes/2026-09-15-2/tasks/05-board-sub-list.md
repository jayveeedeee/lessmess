---
id: PSB-05
title: Board UI — per-task sub list with talk button
---

# PSB-05: Board UI — per-task sub list with talk button

## Objective

Surface subagent sessions on the board: grouped under their task, with the existing
terminal overlay as the way to talk to them.

## Dependencies

- PSB-01, PSB-02 (fields exist and can be populated), PSB-00 (promptability
  confirmed, so "talk" is worth a button).
- PSB-03 for reconciled (never-bound) subs to show up.

## Scope

- `web/static/app.js` and the board templates only; data comes from the existing
  session-list endpoint (now with `task`/`parent`).

## Implementation steps

1. Group session-list entries with a `task` field under their task card in board
   column order; entries with `parent` but no `task` go under the change header;
   plain sessions render exactly as today.
2. Reuse the existing talk/terminal affordance per sub entry — the overlay already
   connects to arbitrary session IDs via `opencode2 --session <id>`.
3. Dead subs (live check fails) render in the existing `Live: false` style; no new
   error surface.
4. Keep the DOM-mirror pattern (board state rides the existing SSE/refresh flow);
   no new polling.

## Verification

- Manual: rebuild the binary, delegate one task from a fresh change session, see
  the sub appear under the task (bound or reconciled), open the terminal on it and
  get a reply.
- `go build ./...` + `go test ./...` green; `go vet ./...` clean.

## Completion criteria

- Subs are visible per task, talkable, and degrade gracefully when gone.

## Files affected

- `web/static/app.js`, `web/templates/board.html` (or the partials rendering task
  cards and the session list).

## Notes

- Remember the embedded web assets require a binary rebuild before manual
  verification (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`).
- Verification detail (2026-09-15): the first live probe accidentally ran against
  the user's other project's server on port 9091 (`go run` instance serving
  truendo2), creating a stray `2026-09-15-0` change directory and root-ledger row
  there. Both were removed immediately and the ledger restored byte-for-byte; no
  other writes occurred. Subsequent verification ran on port 9099 against a scratch
  repo only. Lesson: check `lsof -i :<port>` before pointing probes at a port.
- Also learned: `GET /changes/{id}` returns JSON by default (Accept negotiation) —
  HTML needs `Accept: text/html`. Early "markup missing" readings were this, not a
  template problem.
- The user's 9090 server still runs the pre-change binary; restarting it (user
  step) activates the new board markup, sessions payload, and PSB-04's prime for
  freshly created change sessions.
