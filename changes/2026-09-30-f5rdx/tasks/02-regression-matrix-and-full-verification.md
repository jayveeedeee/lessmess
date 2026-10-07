# RP-02: Regression matrix and full verification

## Why

Every prior iteration of this feature shipped with a blind spot found in
live use (restart amnesia, failed-compaction arming, the queue loop). The
matrix must pin every path the redesign touches, especially the queue.

## What

Rewrite `internal/server/reprime_test.go` around injection semantics:
(a) compaction newest → next send verbatim + exactly one synthetic injected;
(b) subsequent sends/queue submissions skip (marker synthetic seen);
(c) queued-then-cancelled follow-up cannot re-trigger;
(d) unbound session → nothing injected;
(e) capability absent → nothing injected, prompt verbatim, no error;
(f) injection failure (service 500) → fail-open, prompt verbatim;
(g) tie timestamps → treated as continued.
Extend `chat_test.go` for the restoration row and the input-limit ceiling,
and `lifecycle_test.go` for the `AddSynthetic` body contract. Then the full
pass.

## Files affected

- `internal/server/reprime_test.go`
- `internal/server/chat_test.go`
- `internal/opencode/lifecycle_test.go`

## Verification

- `go vet ./...` and `go test ./...` clean.
- `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess` succeeds.
- `curl -s {apiBase}/api/validate` (or `lessmess validate`) clean.
- Live spot-check after rebuild+restart on a compacted bound session: queue
  shows verbatim text; one restoration row in the transcript.
