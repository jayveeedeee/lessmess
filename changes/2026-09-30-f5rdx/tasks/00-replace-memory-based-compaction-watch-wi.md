# RP-00: Inject the compaction re-prime as a synthetic message

## Why

The restoration currently rides the user's prompt text: `maybeReprime`
rewrites the next send or queued follow-up into "Context restoration: …
[ledger] ----- *user text*". The queue renders that verbatim, so users see
their message appended to a compaction wall; and because queued prompts never
land in the transcript, the rule re-wraps every queue submission (cancelled
items mean it never stops). The service publishes the right vehicle —
`POST /api/session/{sessionID}/synthetic` with `resume: false` (admitted to
context, no model turn) — the same mechanism it uses for AGENTS.md updates.

## What

- `internal/opencode/lifecycle.go`: `AddSynthetic(ctx, cap, sessionID, text)`
  posting the strict body (`text`, `resume:false`) with body/variants pinned
  by `lifecycle_test.go`; extend `LifecycleCapabilities` to detect the exact
  method+path from the OpenAPI document (`Synthetic`).
- `internal/server/reprime.go`: replace `maybeReprime` with
  `ensureReprime(ctx, sessionID)` — bound-session guard, transcript rule
  (newest completed compaction strictly newer than the newest user message
  *or* marker-bearing synthetic), then `AddSynthetic` with the restoration
  text (marker prefix "Context restoration:", binding line, ledger snapshot,
  skills pointer). Capability absent or any failure → log and return
  (fail-open to no restoration, never to rewriting text).
- `internal/server/chat.go` (`chatPrompt`) and
  `internal/server/sessionlifecycle.go` (`sessionDeliver`): run
  `ensureReprime` before the prompt/deliver call and pass the user's text
  through **verbatim**.

## Files affected

- `internal/opencode/lifecycle.go`
- `internal/opencode/lifecycle_test.go`
- `internal/server/reprime.go`
- `internal/server/chat.go`
- `internal/server/sessionlifecycle.go`

## Verification

- `go vet ./...`, `go test ./...` clean after RP-02's matrix lands; interim:
  package builds and existing suites stay green.
- Manual trace: compaction → next send verbatim + synthetic injected once;
  queue submissions after the first see the synthetic and skip.
