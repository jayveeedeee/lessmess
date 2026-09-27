# SKIL-03: Snapshot-only compaction re-prime

## Why

Even with capabilities in compaction-proof skills, compaction still
eats the base prompt (binding facts) and the state snapshot. A bound
session that keeps working must regain "you are task X.Y of change X"
plus current state on its next turn — nothing more.

## What

- New file (e.g. `internal/server/reprime.go`): `compactionWatch` with
  mutex-guarded memory maps keyed by sessionID — `pending` (queued via
  lessmess), `lastSeen` (newest completed compaction observed),
  `primed` (newest already compensated). Methods: `markPending`,
  `observe`, `needsReprime`.
- `sessionCompact` (`internal/server/sessionlifecycle.go`): mark
  pending after a successful `CompactSession`.
- `chatSnapshot` (`internal/server/chat.go`): scan messages for
  `Type=="compaction" && Status=="completed"`, record newest by
  `Time.Created` — covers service-side auto-compaction.
- `chatPrompt` and `sessionDeliver`: for a bound session (mapping has
  change, optional task) with an uncompensated compaction, prepend:
  binding line + fresh state snapshot (via the existing
  `LedgerFile` view used by `changePrime`/`taskPrime`) + one line
  ("context was compacted; continue where you left off — procedures
  are in your `lessmess-*` skills"), then the user's text; clear the
  marker. Unbound sessions are never wrapped.
- Fail-open: any lookup/read error logs and sends the raw text.
- No persistence (memory-only), no event subscription, no settings
  toggle.

## Files affected

- `internal/server/reprime.go` (new)
- `internal/server/reprime_test.go` (new)
- `internal/server/server.go`
- `internal/server/sessionlifecycle.go`
- `internal/server/chat.go`

## Verification

- New tests: queued compaction → next `chatPrompt` wrapped (binding
  line, fresh snapshot content, user text after), marker cleared,
  second prompt unwrapped; transcript-observed completion without
  `sessionCompact` (auto-compaction path); unbound session unwrapped;
  unknown-change binding → raw text, 202 still returned; `sessionDeliver`
  parity.
- `go vet ./... && go test ./...`; existing `chat_test.go` compaction
  UI pins unchanged.
