# CHAT-02: Bound poll-path upstream latency

## Why

Poll-path calls inherit the `opencode.Client`'s 30-second `http.Client`
timeout. When the service stalls, each browser poll can hang for up to
30s while the UI shows `aria-busy` — the "blocked API" feel. A poll that
cannot answer quickly should answer *degraded* quickly.

## What

- Wrap the snapshot fan-out in one `context.WithTimeout` budget defined
  as a package constant (`chatPollBudget`, proposed 8s — comfortably
  above healthy service latency, far below the 30s client cap). Applied
  around the whole fan-out, not per call.
- On budget expiry the handler returns the degraded shape where possible
  (per CHAT-01: transcript if it made it back, otherwise the standard
  upstream-error path — the poll retrying in 1.2s is the recovery).
- Scope strictly to the poll path (`chatSnapshot`); mutating endpoints
  (prompt, replies, interrupt) keep their existing semantics.
- depends: CHAT-00, CHAT-01 (budget + degradation compose).

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`

## Verification

- New test: fake service that never answers; the snapshot handler
  returns within the budget constant (+ small scheduling slack), not 30s.
- Timing test from CHAT-00 still passes with the budget in place.
- `go vet ./... && go test ./...` clean.
