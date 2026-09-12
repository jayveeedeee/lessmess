---
id: TST-03
title: Update tests, README, and run full verification
---

# TST-03: Update tests, README, and run full verification

Status: see [../ledger.md](../ledger.md).

## Objective

Bring every hard-coded five-column expectation up to date, document the new
column and gate in the README, and run the full verification suite for the
change.

## Dependencies

- TST-00 (model status and template)
- TST-01 (workflow text + drift-pinned embedded copy)
- TST-02 (pill styling; verified here end-to-end)

## Scope

- `internal/server/server_test.go`: `len(resp.Columns)` expectation 5 → 6
  (and any per-column assertions affected by the new order).
- `internal/server/render_test.go`: expected status pill list gains `Test`.
- Any other test/fixture that enumerates the five statuses (grep for the
  status names to be sure).
- `README.md`: board bullet — six columns including `Test`, and a sentence
  that agents stop at `Test` while `Done` is user-gated.

## Implementation steps

1. Update the server/render test expectations for six columns in the order
   Not started, In progress, Blocked, Test, Done, Cancelled.
2. Grep the repo (excluding `changes/` history) for status enumerations and
   update any missed spot; note each in this task's notes.
3. Update the `README.md` board bullet.
4. Run the full suite: `go vet ./...`, `go test ./...`,
   `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`,
   `./lessmess validate`.
5. Manual smoke: open a board in the browser, drag a card `In progress` →
   `Test` → `Done`, confirm the ledger file records each move; check the pill
   in both themes.

## Verification

1. All commands in step 4 exit clean.
2. Manual smoke (step 5) behaves as described; screenshot or console log not
   required, but record the outcome in notes.
3. Re-check the plan's acceptance criteria 1–5.

## Completion criteria

- Suite green, `lessmess validate` clean, README accurate, and the board
  demonstrably persists `Test` on drag; change acceptance criteria confirmed.

## Files affected

- `internal/server/server_test.go`
- `internal/server/render_test.go`
- `README.md`
- (any other spot found in step 2, recorded in notes)

## Notes

- Status enumerations found by grep and disposition: `web/static/app.js`
  close-change confirm (lines ~621/643) kept as-is — warning on close with
  `Test` tasks is the desired nudge; `AGENTS.md` overall-status lists are
  change-level and unchanged; historical `changes/` docs untouched.
- Smoke evidence (2026-09-13, fresh binary on :9091): board HTML contained
  `data-status` for all six columns in order Not started, In progress,
  Blocked, Test, Done, Cancelled, and rendered `status-test` pills;
  `POST /changes/2026-09-12-15/move` with `{"task":"TST-03","status":"Test"}`
  persisted `Test` in the ledger, and a second move restored priority order.
- Not verified by the agent: pill legibility eyeballed in both browser themes
  (CSS is var-driven, matching the existing pattern) — left to user
  acceptance.
- Operational note: the long-running board on :9090 still runs the pre-change
  binary; restart it from the freshly built `lessmess` so it recognizes the
  `Test` status (the old binary would flag ledgers containing `Test`).
