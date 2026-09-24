# MP-07: Tolerant legacy ledger parsing and unavailable-boot retry

## Goal

Two live-use corrections surfaced while adding real legacy repositories to
the hub:

1. Legacy repos whose ledgers backtick the header IDs (a common
   hand/GPT-authored convention — e.g. `- Change ID: \`2026-09-09-0\``)
   must migrate cleanly: the parser kept the backticks, so the migrator's
   identity check `ledger.ChangeID != row.Change` failed even though the
   IDs read identically, and the project showed "unavailable" with a
   baffling message ("ledger header names \`2026-09-09-0\`").
2. A project slot that booted "unavailable" (e.g. that same transient
   parse refusal, or any non-ErrNoChanges boot error) never retried — even
   after the underlying cause was fixed, the landing kept showing
   unavailable until a full restart.

## Approach

- `internal/model/ledger.go`: a `trimBackticks` helper drops one wrapping
  backtick pair (IDs never legitimately contain backticks). Applied to the
  `- Change ID:` value in `ParseChangeLedger` and to both captured IDs of
  the `- Task: <id> (change <change-id>)` header in `ParseTaskLedger`.
- `internal/server/hub.go` `ensureMounted`: slots whose status is
  `unavailable` are re-mounted (their old slot held no resources — a failed
  boot opened nothing), so fixing the repository and refreshing the landing
  page heals the project without a restart. `ready` and `setup` slots are
  left untouched.
- Unit tests for both; the real-repo check is `lessmess migrate --dry-run`
  against the repository that exposed the bug.

## Files affected

- `internal/model/ledger.go`
- `internal/model/ledger_test.go`
- `internal/server/hub.go`
- `internal/server/hub_test.go`

## Verification

- `go test ./internal/model/ -run Ledger -v` covers backticked IDs.
- `go test ./internal/server/ -run 'SelfHeal|RetriesUnavailable' -v`.
- `lessmess migrate --dry-run --dir <legacy repo>` reports the conversion
  plan instead of the header mismatch.
