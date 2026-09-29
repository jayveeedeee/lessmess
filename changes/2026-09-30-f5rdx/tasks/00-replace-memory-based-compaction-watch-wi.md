# RP-00: Replace memory-based compaction watch with transcript-based evaluation

## Why

The re-prime fires spuriously: restart amnesia re-arms compensated compactions,
failed compactions arm a wrapper for a compaction that never happened, and a
prompt racing an in-flight compaction double-fires. Root cause is memory-only
bookkeeping (`compactionWatch`) that cannot distinguish "already compensated"
from "not seen yet" across restarts.

## What

Rewrite `maybeReprime` detection as a stateless transcript rule: a bound
session needs a re-prime iff its newest completed compaction message is
strictly newer than its newest user message. Evaluate with one descending
`ListMessagesPage` fetch (small limit; user message found first → no re-prime;
completed compaction first → re-prime; equal timestamps → no re-prime; page
exhausted with a completed compaction and no user message → re-prime). Delete
`compactionWatch`, `markPending`, the `s.compacts` wiring in `server.go`, the
`markPending` call in `sessionlifecycle.go`, and the compaction-observation
block in `chat.go`'s snapshot walk. Keep `maybeReprime`'s signature, the
unbound-session guard, the ledger-snapshot preamble, and fail-open behavior.

## Files affected

- `internal/server/reprime.go`
- `internal/server/server.go`
- `internal/server/sessionlifecycle.go`
- `internal/server/chat.go`

## Verification

- `go vet ./...` and `go test ./...` clean (reprime tests are rewritten in
  RP-01; existing suites must not regress).
- Manual trace of the four target-behavior cases against the new evaluator.
