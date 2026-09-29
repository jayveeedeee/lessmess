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
  queue/watcher (~lines 60–70), require `DocsEnabled(st.Dir)`; keep the existing
  `docs.LoadConfig` check and its unreadable-config warning. Missing setting
  → no docsQ/docsW, no warning spam (disabled is a normal state now).
- `SetOpencode` / gardener wiring, `Close()`, SSE `docsCh`: unchanged — they
  already nil-guard on docsQ/docsW.
- Add one runtime-state helper for inactive responses. It must distinguish:
  setting Off; setting On with missing config; unreadable config; and setting
  On + readable config while `docsQ` is still nil (restart required). This
  avoids claiming the feature is active after a setting/config write when the
  startup-only queue has not been built yet. Model enough state for EXP-02 to
  expose `enabled`, config presence/readability, runtime-active, and
  restart-required without duplicating the rules.
- Gate runtime endpoints on `s.docsQ`: manual refresh, normal-server docs seed,
  Explorer detail/chat, and validation. In `/api/validate`, skip both
  `docs.ValidateDocs` and `docs.MissingDocDirs` when inactive; return an empty
  docs list and omit (or zero) `docsSeedPending`, so a dormant config cannot
  light the docs bell.
- Keep configuration surfaces separate from runtime surfaces: the exclusions
  reader remains available to report `hasConfig`; initialization/exclusion
  writes use the live effective setting rather than `s.docsQ`, allowing the
  user to prepare config before the one required restart.
- `internal/server/explorer.go` + `web/templates/explorer.html`: the disabled
  error-box text points at Settings → Docs (and mentions coverage config);
  keep the `Enabled=false` view contract so render tests stay simple.
- `cmd/lessmess/main.go` (`validate`, `docs seed` CLI): use
  `server.DocsEnabled` so the CLI respects the same gate. `validate` always
  performs workflow validation but skips only docs validation/queue-stale
  reads when disabled; `docs seed` refuses with a Settings → Docs message.
- Unit tests: server construction with setting off + config present → nil
  queue; setting on + no config → nil queue; setting on + config → queue
  built; `/api/validate` emits no docs findings/pending count when off; handler
  wording covers off, missing, unreadable, and restart-required states.

Out of scope: the initialize-coverage endpoint (EXP-02), wizard/init removals
(EXP-03).

## Implementation steps

1. Add the effective-settings check in `server.New`'s docs construction path.
2. Introduce a small runtime-state helper for inactive-cause messages.
3. Gate runtime handlers and the complete docs portion of `/api/validate`;
   reword the Explorer disabled box.
4. Gate the CLI `validate`/`docs seed` docs paths with `DocsEnabled`.
5. Add/extend unit tests; run `go vet ./... && go test ./...`.

## Verification

- `go test ./internal/server/ -run 'Docs|Server'` green with the new matrix.
- Manual matrix (rebuild + restart): (a) setting off + config present → docs
  inert, Explorer disabled box names Settings; (b) setting on + no config →
  same inert state with missing-config wording; (c) setting on + config →
  current docs behavior (bell, refresh, seed, gardener on close).
- CLI fixture matrix: setting on + config reports docs findings as before;
  setting off + config performs workflow validation but reports no docs
  findings.

## Completion criteria

- Docs subsystem starts if and only if `docs.enabled` is effectively on and
  `agentsdocs.json` is readable; restart applies toggles.
- All inactive surfaces use accurate Settings/restart wording; no regression
  in the enabled path, and a dormant config cannot produce notifications.
