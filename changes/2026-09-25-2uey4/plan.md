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

## 2026-09-29 addendum: second root cause (config-write re-init cascades)

A user-visible outage ("clicked decompose and all other sessions could no
longer talk to the backend or load") was forensically traced to a
different mechanism than poll amplification: the OpenCode service reacts
to any write of a watched per-repo config file (`opencode.json`) with
`config.updated` and a **full location re-initialization** — watchers,
providers, models, agents, commands, plugins, and the skills cache are
torn down and rebuilt — degrading every running session in the location.
`EnsureSkillsCatalog` writes `opencode.json` at lessmess boot whenever
the computed skills URL (`http://<PublicBase>/skills/`, which embeds
host/port/base) differs from the stored entry, so restarts with changed
invocation flags reliably fire the cascade (evidence: file mtime and
`config.updated` second-aligned at 19:56:12Z; a cascade at 19:54:38Z in
the decompose window with the skills cache hash flipping). CHAT-04 makes
that write conditional: a stale-but-live catalog entry is probed and left
in place, trading skill-body staleness for not stalling every session.
The service-side behavior (re-init degrading running sessions) is an
upstream concern, documented here for the report.

## 2026-09-29 addendum 2: the wait-route drift blocking docs healing

Attempting the docs heal surfaced a third defect: the installed service
serves `POST /api/experimental/session/{sessionID}/wait` only, while
`WaitDone` hardcodes the plain route and 404s — breaking the gardener
(jobs fail after doing their edits, dirs stay stale forever), commit-
status polling (never done), and the worktree close reviewer wait.
CHAT-05 makes the route capability-detected from the OpenAPI document
per the package's own convention, with the plain route as offline
fallback. Healing the docs queue requires this fix first.

A full same-nature audit (2026-09-29) cross-checked every client route
against the live OpenAPI document: **`wait` is the only ungated drifted
route**. All other drifted surface already follows the capability
convention and degrades to a clean 503 (`ErrCapabilityUnavailable`):
MCP connect/disconnect select the `/api/experimental/mcp/...` variants,
inbox delivery selects between steer/queue and the `/{inboxID}/{delivery}`
PATCH, revert-clear and rename-PATCH are detected, export calls its
experimental route directly (the fix pattern CHAT-05 mirrors), and
`/api/health` is only a detected fallback behind `/api/info`. Writers of
service-watched config files are exactly two: the boot skills patch
(CHAT-04) and the manual Align button; gardener `AGENTS.md` edits
empirically fire no `config.updated` cascade (root `AGENTS.md` edited
2026-09-29T20:10Z with no config event in the service log).

## Scope

- `internal/server/chat.go` — `chatSnapshot` fan-out, degradation policy,
  per-request upstream budget.
- `internal/server/skillsconfig.go` — probe-before-patch for the boot
  skills-catalog write (CHAT-04).
- `internal/opencode/client.go` — capability-detect the session wait
  route (CHAT-05).
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
- Skills-catalog write (CHAT-04): probe the existing `/skills/` URL
  before replacing it; skip the rewrite while it still serves a live
  catalog. Healing remains the failure direction — a dead or
  non-catalog probe answers patches exactly as today.

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
- A boot with a changed skills URL but a still-live old entry leaves
  `opencode.json` byte-identical (probe-and-skip, pinned by test); a
  dead old entry still heals.
- Against a service declaring only the experimental wait route,
  `WaitDone` succeeds on it; against one declaring both, the plain route
  wins; against an unreadable OpenAPI document, today's plain-route
  behavior is preserved (all pinned by tests).
- `go vet ./...` and `go test ./...` clean from the repo root; existing
  chat tests pass except error-shape tests intentionally updated for the
  degradation policy.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
