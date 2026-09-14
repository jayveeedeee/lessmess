---
id: ONB-03
title: Bootstrap endpoint with docs-coverage option
---

# ONB-03: Bootstrap endpoint with docs-coverage option

Status: see [../ledger.md](../ledger.md).

## Objective

`POST /api/setup/bootstrap` runs the existing `docs.Init` bootstrap from the wizard — with a separate choice of whether docs coverage (`agentsdocs.json`) is written — reports per-artifact actions, records the onboarding step, and triggers the ONB-01 hot-open.

## Dependencies

- ONB-01 (boot callback + route registrar)

## Scope

- `internal/docs/init.go`: `InitOptions{Config bool}` + `InitWithOptions(root, opts)`; existing `Init(root)` delegates with all steps enabled (CLI and current tests untouched).
- `internal/server/setup.go` (or a new `bootstrap.go`): endpoint handler on the shared registrar.
- Request `{docsCoverage: bool}`; response `{actions: [{path, action}], reloaded: bool}`.
- Behavior: run init (coverage step skipped when `docsCoverage:false`) → mark onboarding `steps.bootstrap=done` → if `changes/` now exists and we're in setup mode, invoke boot once (`reloaded:true`); on the normal server (already booted) init re-runs idempotently with `reloaded:false`.
- Failures: 500 with the partial action list (init already returns partial actions on error).

## Implementation steps

1. Refactor `docs.Init` into `InitWithOptions` with the config step conditional; keep the public `Init` signature and semantics.
2. Add a table-driven test for skip-config (no `agentsdocs.json` written, other artifacts created).
3. Implement the endpoint: parse body, run init, update onboarding state, conditionally boot, respond.
4. Tests: temp-dir bootstrap creates artifacts and fires boot exactly once; second call is a no-op re-run (`reloaded:false`, actions `skipped`); skip-config honored; normal-server registration responds without booting.

## Verification

- `go test ./internal/docs ./internal/server -run 'Init|Bootstrap' -v` passes; `go vet ./...` clean.
- Manual: bootstrap on an empty dir via curl, then `GET /api/validate` (full server) answers 200 without a process restart.

## Completion criteria

- Wizard-triggered bootstrap is byte-equivalent to CLI `lessmess init` (minus the optional config), hot-open fires once, and repeated calls are harmless.

## Files affected

- `internal/docs/init.go`, `internal/docs/init_test.go`
- `internal/server/setup.go` (or new `bootstrap.go` + test)
- `cmd/lessmess/main.go` (only if the CLI wants the flag — optional; default keeps full init)

## Notes

- Decision 10 in [../plan.md](../plan.md): coverage config and docs generation are separate choices; this task only controls the config file.
- Keep `InitAction` values (`created`/`merged`/`skipped`) unchanged — the CLI prints them.
