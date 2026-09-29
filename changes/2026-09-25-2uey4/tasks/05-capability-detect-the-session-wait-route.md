# CHAT-05: Capability-detect the session wait route

## Why

The installed OpenCode service (2.0.17) serves the wait endpoint only at
`POST /api/experimental/session/{sessionID}/wait`; lessmess's `WaitDone`
(`internal/opencode/client.go`) hardcodes `POST /api/session/{id}/wait`
and receives a 404. Verified 2026-09-29 against the live OpenAPI
document and a direct probe. Because a 404 is not a transport timeout,
`WaitDone` returns it immediately — no retry — so every dependent flow
breaks:

- the docs gardener: the session runs and edits files, the wait fails,
  the job is flagged stale (`wait gardener: opencode API 404: Not
  Found`), and every refresh re-spawns the same doomed cycle — the docs
  bell can never clear;
- commit-status polling (`internal/server/lifecycle.go`): 404 ⇒
  `done=false` forever, so the Commit-all spinner never completes;
- the worktree close pipeline's reviewer wait
  (`internal/server/closepipeline.go`): every close would fail at the
  review step.

This is also a convention violation: volatile service surface must be
capability-detected from the OpenAPI document (exact method + path),
never hardcoded — the pattern `StandaloneSkillRoute` already follows.

## What

- Add route detection for the wait endpoint (mirror
  `StandaloneSkillRoute`): fetch `/openapi.json`, look for
  `POST /api/session/{sessionID}/wait` first, then
  `POST /api/experimental/session/{sessionID}/wait`; prefer the plain
  route when both exist.
- Resolve once per `Client` (cached, e.g. `sync.Once`-guarded), falling
  back to the plain route when the OpenAPI document is unreadable —
  preserving today's behavior on older services and offline probes.
- `WaitDone` uses the resolved route; its transport-timeout retry loop
  is unchanged.
- Call sites (`docssession.go`, `lifecycle.go`, `closepipeline.go`)
  stay untouched.

## Files affected

- `internal/opencode/client.go`
- `internal/opencode/client_test.go`

## Verification

- Tests over an httptest fake serving an OpenAPI document:
  1. only the experimental route declared ⇒ `WaitDone` calls it (200 ⇒
     nil error);
  2. both routes declared ⇒ the plain route is called;
  3. only the plain route declared ⇒ current behavior;
  4. OpenAPI fetch fails ⇒ fallback to the plain route;
  5. transport timeouts on the resolved route still retry until ctx
     expires (existing WaitDone pin stays green).
- `go vet ./... && go test ./...` from the repo root.
- Live confirmation after rebuild+restart: `POST /docs/refresh`
  completes jobs and clears the six stale flags in
  `.lessmess/docs-queue.json`.
