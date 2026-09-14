---
id: ONB-01
title: Setup-mode server shell with hot-open
---

# ONB-01: Setup-mode server shell with hot-open

Status: see [../ledger.md](../ledger.md).

## Objective

Make `lessmess serve` start on a repo without `changes/` and serve a setup-mode surface (wizard page + setup APIs only), then swap to the full server in-process once bootstrap creates the store — no restart.

## Dependencies

- ONB-00 (setup completion records onboarding state)

## Scope

- `internal/store/store.go`: exported sentinel `store.ErrNoChanges` wrapped (via `%w`) into `Open`'s missing-`changes/` error, preserving the existing human message text; a malformed root ledger stays a plain fatal error.
- New `internal/server/setup.go`: `setupServer` (repo dir, renderer, opencode-client accessor, boot callback) plus `registerSetupRoutes(mux, env)` shared by the setup mux and the normal `Server` mux.
- `internal/server/server.go`: `Server.New` calls `registerSetupRoutes` (wizard reachable after hot-open and on initialized repos); the actual endpoint implementations land in ONB-02/03/04 — this task wires the registrar and the shell only (placeholder 501s are acceptable for not-yet-implemented endpoints).
- `cmd/lessmess/main.go`: `runServe` branches on `errors.Is(err, store.ErrNoChanges)` into setup mode: build the setup server with a `boot` closure that runs the exact existing serve wiring (`store.Open` → `Watch` → `server.New` → `SetOpencode` → `PublicBase`) against the live signal context, then atomically swaps the root handler. Shutdown closes whichever child is active.
- Setup-mode guarding: `GET /` (and HTML GETs of normal pages) serve/redirect to the wizard; all other normal API routes return 503 JSON.

## Implementation steps

1. Add `ErrNoChanges` and wrap it in `Open` (keep the message prefix `changes/ directory not found under <dir>`); adjust/add store test.
2. Implement `setupServer` with an `atomic.Value` (or equivalent) current handler and a swap-once `boot` method that also marks `steps.bootstrap`/state via ONB-00 helpers when the swap succeeds.
3. Implement `registerSetupRoutes(mux, env)` with an `env` struct (dir, `oc func() *opencode.Client`, boot callback or nil on the normal server).
4. Call the registrar from `Server.New`; add a `setup` template-set placeholder route serving a minimal page until ONB-05 lands the real one.
5. Rework `runServe` for the setup-mode branch with the boot closure and clean shutdown of the active child.
6. Tests: sentinel detection; setup mux guard (503/redirect); swap-once boot with a fake boot callback; normal server still serves `/setup` routes.

## Verification

- `go test ./internal/store ./internal/server -run 'NoChanges|Setup' -v` passes; `go vet ./...` clean.
- Manual: build the binary, `lessmess serve --dir <empty tmp>` serves a page at `/` (not exit 1), `/api/validate` returns 503; after a faked boot the full UI answers without restart.

## Completion criteria

- Setup mode starts and guards correctly, hot-open swaps exactly once with the real store wiring, and all tests pass.

## Files affected

- `internal/store/store.go`, `internal/store/store_test.go`
- `internal/server/setup.go` (new), `internal/server/setup_test.go` (new)
- `internal/server/server.go`, `internal/server/render.go` (placeholder template set)
- `cmd/lessmess/main.go`

## Notes

- Architecture decisions 1–3 in [../plan.md](../plan.md): separate mux (never nil-store `Server`), boot closure owned by `main.go`, one registrar for both muxes.
- Watch out: `server.New` currently assumes the store is non-nil everywhere; the registrar must not touch `s.st`.
