# RP-01: Cover the re-prime rule with regression tests and verify the suite

## Why

The defects were behavioral and only surfaced in real use (restart + reopen +
prompt). The rewritten evaluator needs tests that pin each spurious-fire path
so the rule cannot regress silently.

## What

Rewrite `internal/server/reprime_test.go` around the transcript rule using the
existing fake-opencode fixture style: (a) compaction followed by user turn →
no wrap, including after a simulated restart (fresh server, same transcript);
(b) completed compaction as newest event → exactly one wrap, and the next
prompt unwrapped; (c) failed compaction newest → never wrapped; (d) prompt
racing an in-flight compaction → single wrap; (e) unbound session and
unreadable-ledger → unwrapped; (f) tie timestamps → unwrapped. Then run the
full verification pass.

## Files affected

- `internal/server/reprime_test.go`

## Verification

- `go vet ./...` and `go test ./...` clean.
- `curl -s {{apiBase}}/api/validate` (or `lessmess validate`) clean.
- `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess` succeeds.
