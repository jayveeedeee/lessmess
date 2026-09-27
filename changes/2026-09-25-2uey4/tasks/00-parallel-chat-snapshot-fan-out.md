# CHAT-00: Parallel chat snapshot fan-out

## Why

`chatSnapshot` runs its four upstream reads one after another
(`ListMessagesPage` → `ListActiveSessions` → `ListPermissions` →
`ListForms`), so every ~1.2s poll pays the *sum* of four service
round-trips. When the shared OpenCode service is saturated (the measured
production condition), that sum is what makes the chat UI feel frozen.
Running the reads concurrently caps snapshot latency at the slowest single
read instead.

## What

- Restructure `chatSnapshot` (`internal/server/chat.go`) so the four
  upstream reads execute concurrently (`errgroup`, or a `WaitGroup` +
  error channel if errgroup is unavailable — pick one, stay consistent).
- The transcript (`ListMessagesPage`) remains the authoritative result;
  its ordering relative to the aux reads must not change the rendered
  output.
- Preserve the existing behavior when the transcript read itself fails
  (`writeChatUpstreamError`) — full failure handling is CHAT-01's job;
  this task only parallelizes, keeping each read's failure fatal as
  today.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`

## Verification

- New test: a fake service (httptest-backed `opencode.Client`) delays
  each of the four reads ~300ms; assert the snapshot handler completes in
  well under the sequential total (e.g. <600ms) — proving overlap.
- `go vet ./... && go test ./...` from the repo root; all existing
  `chat_test.go` tests pass unchanged.
