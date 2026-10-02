# CR-00: Replace memory-based compaction watch with transcript-based evaluation

## Why

The memory-only `compactionWatch` misfires three ways: restart amnesia
(empty `primed` map arms on any old completed compaction in the visible
page), unconditional `markPending` (failed compactions still arm the
wrapper), and the double-fire race (a prompt consuming `pending` before
`lastSeen` is set re-primes again on the next prompt). All three surface to
users as their message rewritten into "Context restoration: …" + their text
— the mid-session "appended to the compaction message" bug.

## What

- Delete `compactionWatch` (`pending`/`lastSeen`/`primed` maps,
  `markPending`/`observe`/`needsReprime`/`consume`) from
  `internal/server/reprime.go`.
- Add a stateless evaluator: a bound session needs a re-prime iff its newest
  completed compaction message is strictly newer than its newest user
  message, decided from one small descending `ListMessagesPage` fetch:
  - user message found before any completed compaction → no re-prime;
  - completed compaction found first → re-prime;
  - equal timestamps → no re-prime (already continued);
  - page exhausted with a completed compaction and no user message →
    re-prime (conservative).
- `maybeReprime` gains a request context for the transcript read and keeps
  its fail-open behavior: unbound sessions never wrapped, ledger-read
  failures send the plain text.
- Drop the `compacts` field and wiring in `internal/server/server.go`
  (lines 43, 60), the `markPending` call in
  `internal/server/sessionlifecycle.go` (lines 442-446), and the
  compaction-observation block in `internal/server/chat.go`
  (lines 266-282).
- No change to the preamble content or the ledger snapshot it carries.

## Files affected

- `internal/server/reprime.go`
- `internal/server/reprime_test.go` (fixtures rewritten; main body under CR-01)
- `internal/server/server.go`
- `internal/server/sessionlifecycle.go`
- `internal/server/chat.go`

## Verification

- `go vet ./...` and `go test ./...` clean (regression coverage itself lands
  in CR-01; existing reprime tests are updated here only as needed to
  compile against the new shape).
- `lessmess validate` clean.
