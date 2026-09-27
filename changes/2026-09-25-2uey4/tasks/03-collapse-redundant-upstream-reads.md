# CHAT-03: Collapse redundant upstream reads

## Why

Every snapshot poll of every open view refetches the global
`ListActiveSessions`, and concurrent polls of the same session (multiple
tabs, stale views) each fetch the transcript independently. Under a
saturated service this self-inflicted multiplication adds exactly the
load that makes everything slower. Collapsing identical concurrent
reads is the largest request-volume reduction available client-side of
the service.

## What

- `ListActiveSessions`: serve through a process-local single-flight
  cache with a short TTL (~1s — busy-state staleness of at most one poll
  is invisible at the UI's cadence). One in-flight call shared by all
  waiters; expiry recomputes.
- Same-session snapshots: single-flight keyed by sessionID — concurrent
  polls of one session share a single in-flight upstream fetch result
  (reads are idempotent; last-writer-wins sharing is fine). Sequential
  polls 1.2s apart are unaffected.
- Helper colocated in `chat.go` or a new `internal/server/chatcache.go`;
  mutex-guarded, no goroutine leaks (waiters release via context).
- depends: CHAT-00 (composes with the fan-out).

## Files affected

- `internal/server/chat.go` (or new `internal/server/chatcache.go`)
- `internal/server/chat_test.go`

## Verification

- New tests with a call-counting fake: N concurrent snapshots of one
  session ⇒ exactly 1 transcript fetch; repeated polls within the TTL ⇒
  ≤1 `ListActiveSessions` call per TTL window.
- A failed shared fetch is not memoized: next poll retries upstream.
- `go vet ./... && go test ./...` clean.
