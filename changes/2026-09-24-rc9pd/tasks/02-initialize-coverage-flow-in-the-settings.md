# EXP-02: Initialize-coverage flow in the Settings Docs section

Status: see [../ledger.md](../ledger.md).

## Objective

Let the user turn docs on entirely from the UI: when docs are enabled but no
`agentsdocs.json` exists, the Docs section offers an "Initialize coverage"
action that writes the default config through a config-only primitive rather
than re-running the workflow bootstrap.

## Dependencies

EXP-00 and EXP-01 (the setting and the gate must exist; the flow is only
reachable in the enabled-but-unconfigured state).

## Scope

- `internal/docs/init.go`: export a narrow config initializer (for example
  `docs.InitConfig(root)`) around the existing write-if-absent config logic.
  It writes only `agentsdocs.json`; it must not merge `AGENTS.md`, create
  workflow state, touch `.gitignore`, or create `opencode.json`.
- New normal-server endpoints in a small `docssettings.go`:
  - `GET /docs/status` returns effective enabled, config exists/readable,
    runtime active, restart required, and any config error, using EXP-01's
    shared state helper.
  - `POST /docs/initialize` requires the live effective `docs.enabled` and a
    genuinely missing config, then calls the narrow initializer. Return
    idempotent success for an existing readable config, 409 when docs is off,
    and 409 (without overwrite) when `agentsdocs.json` exists but is unreadable.
- `web/templates/settings.html`: an initialize affordance in the docs
  section (button + status span, outside the per-section `[data-save]` PUT,
  like `#docs-exclusions-save`); hidden when effective config exists.
- `web/static/app.js` (`initSettings`): fetch `/docs/status`, wire the button,
  show success/error/restart-required states, and reload status + exclusions
  after saves and initialization. Saving `docs.enabled=On` must reveal the
  action immediately; no restart is required before initialization. An
  unreadable existing config gets repair guidance, never an initialize action
  that silently skips or overwrites it.
- Exclusions editor: keep working; when no config exists show a hint that
  initializing coverage enables it (it already no-ops cleanly today).
- Docs section help text: the `docs.enabled` field notes it takes effect on
  restart, and that coverage is initialized from here.
- Tests: docs-package test proving the narrow initializer writes only the
  config; status matrix tests; initialize endpoint tests (off → 409, on →
  writes default config, idempotent readable re-run preserves a hand-written
  config byte-for-byte, malformed existing config → 409/no overwrite), plus
  render/wiring assertions for the new affordance.

Out of scope: wizard removals (EXP-03, which frees the bootstrap handler
contract), any include/exclude glob editor beyond the existing picker.

## Implementation steps

1. Add and test the config-only docs initializer.
2. Add the status and initialize endpoints with the narrow initializer and
   explicit malformed-config handling.
3. Surface the affordance in settings.html and wire it in `initSettings`.
4. Add the hint states for the exclusions editor and the restart note.
5. Add endpoint + UI tests; run `go vet ./... && go test ./...`.

## Verification

- Unit tests green (`go test ./internal/server/ -run Docs`).
- Manual: save setting On with no config → button becomes visible immediately,
  click writes a default `agentsdocs.json`, one restart brings docs up; with
  setting Off → button hidden/409; with readable config present → button
  hidden; with malformed config → repair guidance and no overwrite.
- Hand-written `agentsdocs.json` survives an initialize re-run byte-for-byte;
  no other bootstrap artifact changes.

## Completion criteria

- A fresh repo can reach fully-running docs without ever touching the wizard
  or CLI: save On → Initialize coverage → restart once.
