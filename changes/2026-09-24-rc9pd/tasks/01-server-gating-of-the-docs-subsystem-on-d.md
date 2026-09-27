# EXP-01: Server gating of the docs subsystem on docs.enabled

Status: see [../ledger.md](../ledger.md).

## Objective

Make the effective `docs.enabled` setting the primary gate of the docs
subsystem: the queue/watcher build only when it is on *and* a readable
`agentsdocs.json` exists, and every disabled surface says why and points at
Settings → Docs.

## Dependencies

EXP-00 (the setting must exist).

## Scope

- `internal/server/server.go` (`server.New`): before building the docs
  queue/watcher (~lines 60–70), require
  `effectiveSettings().Docs.Enabled == true`; keep the existing
  `docs.LoadConfig` check and its unreadable-config warning. Missing setting
  → no docsQ/docsW, no warning spam (disabled is a normal state now).
- `SetOpencode` / gardener wiring, `Close()`, SSE `docsCh`: unchanged — they
  already nil-guard on docsQ/docsW.
- Handler messages where docs is off: distinguish the two causes —
  "docs system disabled (enable it in Settings → Docs)" vs the existing "docs
  coverage disabled (no agentsdocs.json)" — in `docsqueue.go` (refresh),
  `docsseed.go` (seed, exclusions save), `setupseed.go`, and the
  `validate` API stale-reasons block (which already gates on `docsQ != nil`).
- `internal/server/explorer.go` + `web/templates/explorer.html`: the disabled
  error-box text points at Settings → Docs (and mentions coverage config);
  keep the `Enabled=false` view contract so render tests stay simple.
- `cmd/lessmess/main.go` (`validate`, `docs seed` CLI): read the effective
  docs gate via the package-level settings helpers (same layering as the
  server) so the CLI respects the setting; `docs seed` error message mentions
  Settings → Docs.
- Unit tests: server construction with setting off + config present → nil
  queue; setting on + no config → nil queue; setting on + config → queue
  built; handler wording tests for both disabled causes.

Out of scope: the initialize-coverage endpoint (EXP-02), wizard/init removals
(EXP-03).

## Implementation steps

1. Add the effective-settings check in `server.New`'s docs construction path.
2. Introduce a small helper for the disabled-cause message so handlers and
   templates share one wording.
3. Reword the handler 503/no-op sites and the Explorer disabled box.
4. Gate the CLI `validate`/`docs seed` docs paths on the same helper.
5. Add/extend unit tests; run `go vet ./... && go test ./...`.

## Verification

- `go test ./internal/server/ -run 'Docs|Server'` green with the new matrix.
- Manual matrix (rebuild + restart): (a) setting off + config present → docs
  inert, Explorer disabled box names Settings; (b) setting on + no config →
  same inert state with missing-config wording; (c) setting on + config →
  current docs behavior (bell, refresh, seed, gardener on close).
- `lessmess validate` on this repo (setting on, config present) reports as
  before; with setting off it reports no docs findings.

## Completion criteria

- Docs subsystem starts if and only if `docs.enabled` is effectively on and
  `agentsdocs.json` is readable; restart applies toggles.
- All disabled surfaces use the Settings-pointing wording; no regression in
  the enabled path.
