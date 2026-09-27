# 2026-09-25-2uey4: Resilient chat polling under a slow service

- Change ID: 2026-09-25-2uey4
- Created: 2026-09-25
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Multiple concurrently running sessions feel "blocked": long waits with no
errors, as if the API itself stalls. Investigation (2026-09-25) found no
cross-session locking anywhere in lessmess — the HTTP layer is fully
concurrent and prompts are fire-and-forget. The saturation lives in the
shared OpenCode service process (its snapshot machinery spawned ~22,887 git
processes that day, peaking ~3,900/hour) and at the shared model plan. What
lessmess *can* fix is its own amplification of that slowness: every ~1.2s
chat poll pays four sequential upstream round-trips, fails wholesale when
any one of them fails, and repeats identical work across views and tabs.
This change makes chat polling fast, bounded, and cheap under an unhealthy
or saturated service, so the UI degrades visibly instead of freezing.

## Current behavior

- `chatSnapshot` (`internal/server/chat.go`) issues four upstream reads
  **sequentially**: `ListMessagesPage`, then `ListActiveSessions`, then
  `ListPermissions`, then `ListForms`. Latency is the *sum* of all four.
- Any single failing read aborts the whole poll via
  `writeChatUpstreamError` — a slow `ListPermissions` throws away an
  already-fetched transcript and the UI shows nothing.
- Poll-path calls inherit the `opencode.Client` 30-second `http.Client`
  timeout, so a stalled service holds browser requests for up to 30s.
- Every snapshot poll of every open view refetches the global
  `ListActiveSessions`; concurrent polls of the same session (multiple
  tabs, stale views) each hit the service independently.
- Client side (`web/static/app.js`): `pollChat` re-fetches every ~1.2s
  while visible, one in-flight request per view, overlap aborted.

## Target behavior

- The four snapshot reads run **concurrently**; snapshot latency ≈ the
  slowest read instead of their sum.
- Auxiliary read failures (active/permissions/forms) **degrade the
  snapshot instead of failing it**: the transcript still renders, the
  missing sections are omitted; only a transcript-read failure fails the
  poll (there would be nothing to render).
- Poll-path upstream work carries a **bounded server-side budget** (a
  package constant well under the 30s client cap) so a stalled service
  returns a degraded/error response quickly instead of hanging requests.
- **Redundant reads collapse**: `ListActiveSessions` is served through a
  short-TTL single-flight cache, and concurrent same-session snapshot
  polls share one in-flight upstream fetch.
- No route, JSON, or template-shape changes beyond an optional
  degradation marker attribute on the snapshot root.

## Scope

- `internal/server/chat.go` — `chatSnapshot` fan-out, degradation policy,
  per-request upstream budget.
- A small cache/single-flight helper colocated in `chat.go` or a new
  `internal/server/chatcache.go`.
- `internal/server/chat_test.go` (+ fake client in tests) for the new
  timing, degradation, and call-count pins.
- `web/templates/` only if the optional degradation marker attribute is
  added (kept minimal; skip if it complicates the template contract).

## Non-goals

- No changes to the OpenCode service, its snapshot behavior, or provider
  configuration — the root saturations are documented in this plan and
  handled outside this change (git.worktrees opt-in, per-session model
  spread, an upstream report).
- No move of the close pipeline off the request path (separate concern,
  candidate for a future change).
- No event-stream subsystem for chat (the poll-based design stands) and
  no change to the ~1.2s client polling interval.
- No changes to the `opencode.Client` API surface; budgets are applied at
  call sites via context.

## Design decisions

- Fan-out with `golang.org/x/sync/errgroup` (already an available style
  in the module's dependency tree — otherwise a plain `WaitGroup` plus
  error channel; pick one and stay consistent).
- Budget: one `context.WithTimeout` around the whole fan-out
  (`chatPollBudget`, proposed 8s), not per call — simpler and caps total
  request latency predictably.
- Degradation rule: transcript read is authoritative and required;
  `ListActiveSessions` failure ⇒ `Busy=false`; permissions/forms failure
  ⇒ sections empty. Mark the snapshot root `data-degraded="true"` when
  any aux read failed, for debuggability, if cheap to thread through.
- Cache: process-local single-flight keyed globally for
  `ListActiveSessions` with ~1s TTL (busy-state staleness of ≤1 poll is
  invisible at the UI's cadence); same-session snapshot single-flight
  keyed by sessionID sharing one in-flight result to concurrent waiters.

## Acceptance criteria

- With a fake service delaying each of the four reads by ~1s, a snapshot
  completes in ≈1s, not ≈4s (pinned by test).
- A failing permissions/forms/active read still renders the transcript
  with HTTP 200 and the missing sections empty; a failing transcript read
  still fails the poll (both pinned by tests).
- With a service that never answers, a poll returns within the budget
  constant, far under 30s (pinned by test).
- N concurrent snapshots of one session produce 1 upstream transcript
  fetch and ≤1 `ListActiveSessions` call within the cache/single-flight
  window (call-counting fake, pinned by test).
- `go vet ./...` and `go test ./...` clean from the repo root; existing
  chat tests pass except error-shape tests intentionally updated for the
  degradation policy.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
