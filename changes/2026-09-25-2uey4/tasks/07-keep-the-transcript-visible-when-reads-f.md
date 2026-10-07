# CHAT-07: Keep the transcript visible when reads fail under load

## Why

Reported 2026-10-03: with multiple sessions running, opening a chat
sometimes leaves the "Loading conversation…" placeholder stuck. That
placeholder (app.js `openChat`) is only replaced by a *successful*
snapshot, and under a saturated service the transcript read can exceed
the 8s `chatPollBudget` (CHAT-02) — the original complaint was exactly
"very long API wait times" under load. The budget made each failure
fast, but every poll keeps failing while the service stays slow, so the
placeholder never clears: faster errors, stickier symptom. The old 30s
client window often eventually loaded; the placeholder loop is a
regression for the slow-but-alive regime.

## What

- **Last-good memo**: after a successful latest-page fan-out
  (cursor == ""), store the raw `snapshotData` per session. When a later
  snapshot's transcript read fails, serve the memoized data with
  `Degraded: true` (and a server log line naming the age) instead of
  failing the poll — a stale transcript beats a placeholder, and the
  ~1.2s poll retry keeps trying for fresh content in the background.
- Memo TTL ~10 minutes; after expiry (or with no memo — first open) the
  failure path stays the honest error shape. History pages
  (cursor != "") are never memoized.
- **Budget retune**: `chatPollBudget` 8s → 15s — covers the common
  slow-but-alive read latency (the old eventual successes) while
  staying half the client's 30s cap; with the memo in place failures
  are now cheap to absorb.
- **Observability**: the fatal transcript-failure path logs a warning
  (today it is silent server-side, which is why the incident left no
  trail).

## Files affected

- `internal/server/chatcache.go` (memo)
- `internal/server/chat.go` (wiring, budget, logging)
- `internal/server/server.go` (field + init)
- `internal/server/chat_test.go`

## Verification

- Test: poll 1 succeeds against a fake (transcript seeded, memo
  stored); fake then breaks the message endpoint; poll 2 returns 200
  with the OLD transcript body and `data-degraded="true"`; restoring
  the fake clears the marker on the next poll.
- Test: with the memo expired (TTL shrunk in test) and the endpoint
  broken, the poll returns the upstream error shape.
- Existing budget/degradation/collapse tests stay green; `go vet ./...`
  and `go test ./...` clean from the repo root.
