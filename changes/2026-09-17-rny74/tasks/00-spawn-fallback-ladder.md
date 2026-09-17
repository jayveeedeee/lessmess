---
id: SPF-00
title: Agent-preserving spawn fallback ladder
---

# SPF-00: Agent-preserving spawn fallback ladder

Status: see [../ledger.md](../ledger.md).

## Objective

Replace the single drop-everything retry in `spawnSessionWithModel` with a
de-escalation ladder — agent+model → agent only → model only → plain — so a
rejected model never costs the user their agent (and vice versa).

## Dependencies

None. This is the behavior fix everything else builds on.

## Scope

- `internal/server/settings.go`: `spawnSessionWithModel` only.
- Warning logs for each de-escalation step.
- Unit tests in `internal/server`.

Out of scope: persistence, UI, opencode.json handling (SPF-01/02/03).

## Implementation steps

1. Refactor `spawnSessionWithModel` to attempt creates in order:
   1. `eff.Session.Agent` + model ref (current behavior),
   2. agent only (nil model ref) — only if agent is non-empty,
   3. model only (empty agent) — only if a model ref was built,
   4. plain `oc.CreateSession`.
2. Each step runs only on a 400 `opencode.APIError` from the previous step
   (any other error returns immediately, unchanged from today).
3. Log a warning per failed step naming the step, the attempted agent/model,
   and the service error message; keep one final warn when plain create is
   used.
4. Return the first successful session; preserve the existing guarantee that
   a bad setting never blocks session creation.
5. Keep `spawnSession` (`s.spawnSession`) and the docs-runner wiring
   (`server.go` line 147) unchanged — they inherit the ladder.

## Verification

- New tests in `internal/server` with a fake `opencode.Client`-equivalent
  transport (httptest) that 400s configurable request shapes:
  - rejects agent+model, accepts agent-only → session created with agent,
    no model in body; exactly the expected attempts.
  - rejects agent+model and agent-only, accepts model-only → model sent,
    agent omitted.
  - rejects everything → plain create, no agent/model keys.
  - non-400 error on step 1 → no further attempts, error returned.
- Existing `settingswiring_test.go` expectations still pass (agent sent,
  model-only override for the gardener).
- `go vet ./... && go test ./...` from the repo root.

## Completion criteria

- Ladder implemented and covered by tests; no code path drops the agent
  while a model-only rejection was the cause, except via plain last resort.
- All repo checks pass.

## Files affected

- `internal/server/settings.go`
- `internal/server/settings_test.go` or a new
  `internal/server/spawnfallback_test.go`

## Notes

- Live evidence for the bug: tevr-core change session
  `2026-09-16-esl9l — Design-path cutover` landed `kg-orchestrator` (the
  repository's opencode `default_agent`) while discussion sessions seconds
  earlier got `build` — consistent with a 400 fallback dropping the agent.
- The opencode service honors explicit `agent` on create (verified live:
  200 with `agent=build` + model), so the ladder should rarely descend past
  step 2 in healthy setups.
