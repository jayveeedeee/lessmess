# CR-01: Cover the re-prime rule with regression tests and verify the suite

## Why

The rule must provably fire exactly once per completed compaction and never
on stale ones. Each former defect needs a pinned regression case so the
memory-based races cannot silently return.

## What

Rewrite `internal/server/reprime_test.go` fixtures around the stateless
rule, covering at minimum:

- restart case: transcript whose newest completed compaction is older than
  the newest user message → no re-prime (was: spurious wrap after restart);
- fresh-compaction case: completed compaction newer than every user message
  → re-prime, and after the evaluator marks compensation (next prompt), the
  following prompt is unwrapped;
- failed compaction (`status: "failed"`) → never counts;
- racing prompt: compaction lands between two prompts → exactly one wrap,
  on the first prompt after completion;
- tie timestamps (compaction `Time.Created` == user message) → no re-prime;
- unbound session and ledger-unreadable failure → text sent unwrapped
  (fail-open);
- empty/short transcript and pagination edge (compaction on the page, no
  user message) → re-prime.

Then run the full verification suite.

## Files affected

- `internal/server/reprime_test.go`

## Verification

- `go vet ./...`
- `go test ./...`
- `lessmess validate`
