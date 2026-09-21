
# ONB-04: Server-side docs seed honoring agent/model

Status: see [../ledger.md](../ledger.md).

## Objective

Add the opt-in initial docs generation the wizard offers: an asynchronous, budgeted, server-side `docs.Seed` with progress polling — and fix the existing gap where seed sessions ignore the configured `session.agent`/`session.model` (server-side and CLI).

## Dependencies

- ONB-01 (route registrar; settings endpoints usable for reading defaults)

## Scope

- `internal/docs/summarize.go`: extend `SessionClient` with `CreateSessionWith(ctx, title, dir, agent string, model *ModelRef)` (already implemented by `opencode.Client`); add `NewOpenCodeSummarizerWith(c, wait, agent, model)`; the old constructor delegates with empty values.
- `internal/server/settings.go`: export `SessionDefaults(dir) (agent, model string)` wrapping the existing stateless load (no settings refactor).
- New `internal/server/setupseed.go` (+test): `POST /api/setup/docs-seed` `{budget:int}` → 409 when a job is running, 503 without an opencode client or coverage config; otherwise start one goroutine running `docs.Seed` with a summarizer built from `SessionDefaults`; progress lines from the Seed `io.Writer` accumulate in an in-memory buffer. `GET /api/setup/docs-seed-status` → `{running, lines: [...], error}` following the commitStatus poller pattern. On success mark onboarding `steps.docs=seeded` (the wizard's skip path marks `skipped`).
- `cmd/lessmess/main.go`: `runDocsSeed` builds the summarizer via `NewOpenCodeSummarizerWith` + `SessionDefaults(dir)` (CLI parity for decision 7).

## Implementation steps

1. Extend the interface + constructors; update docs tests and any fakes (grep for existing `SessionClient` fakes).
2. Export `SessionDefaults`; unit-test it against layered fixture settings.
3. Implement the seed job: single in-flight guard, context cancelled on server `Close`, progress buffer capped (e.g. last 200 lines), completion/error recorded; status endpoint.
4. Wire the CLI path.
5. Tests: fake `SessionClient` captures the agent/model passed at create; status transitions idle → running → done/error; 409 on concurrent start; 503 without oc; CLI helper wiring compiles and is covered by an existing-pattern test.

## Verification

- `go test ./internal/docs ./internal/server -run 'Seed|SessionDefaults' -v` passes; `go vet ./...` clean.
- Manual: on a scratch repo with coverage, POST a `--budget 1` seed and watch status lines advance; confirm the created opencode session carries the configured agent (service session list) — and that the wizard-skip path started zero sessions.

## Completion criteria

- One in-flight budgeted seed with live status works from the server; both server and CLI seed paths pass the effective agent/model; skip means zero LLM sessions.

## Files affected

- `internal/docs/summarize.go`, `internal/docs/seed_test.go`/`summarize`-adjacent tests
- `internal/server/settings.go` (export only), `internal/server/setupseed.go` (new), `internal/server/setupseed_test.go` (new)
- `internal/server/setup.go` (route wiring)
- `cmd/lessmess/main.go`

## Notes

- No 400-retry in the seed path (decision 7 in [../plan.md](../plan.md)): values were validated at save time; a rejection shows up as a per-dir failure with the service's message.
- The resumable cursor `.lessmess/docs-seed.json` already makes re-runs cheap — the UI should say "already-summarized dirs are skipped".
- Budget semantics are inherited from `SeedOptions.Budget` (0 = unlimited); the wizard UI supplies the default and warning text (ONB-05).
