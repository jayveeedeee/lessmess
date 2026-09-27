# EXP-02: Initialize-coverage flow in the Settings Docs section

Status: see [../ledger.md](../ledger.md).

## Objective

Let the user turn docs on entirely from the UI: when docs are enabled but no
`agentsdocs.json` exists, the Docs section offers an "Initialize coverage"
action that writes the default config, reusing the same machinery the wizard
bootstrap used.

## Dependencies

EXP-00 and EXP-01 (the setting and the gate must exist; the flow is only
reachable in the enabled-but-unconfigured state).

## Scope

- New endpoint (settings area, e.g. `POST /api/docs/initialize`) in
  `internal/server/settingsapi.go` or a small `docssettings.go`: calls
  `docs.InitWithOptions(st.Dir, docs.InitOptions{Config: true})` (the exact
  call the wizard bootstrap makes today), merge-safe and idempotent; 409 or
  no-op success when config already exists; 409 when the docs setting is
  off (initialize only makes sense enabled).
- `web/templates/settings.html`: an initialize affordance in the docs
  section (button + status span, outside the per-section `[data-save]` PUT,
  like `#docs-exclusions-save`); hidden when effective config exists.
- `web/static/app.js` (`initSettings`): fetch the state (the settings view or
  the exclusions payload already carries `HasConfig` — reuse it), wire the
  button, show success/error in the status span, and reload the section
  state after success.
- Exclusions editor: keep working; when no config exists show a hint that
  initializing coverage enables it (it already no-ops cleanly today).
- Docs section help text: the `docs.enabled` field notes it takes effect on
  restart, and that coverage is initialized from here.
- Tests: endpoint unit tests (off → 409, on → writes default config,
  idempotent re-run, preserves hand-written globs via the existing
  `updateConfigExcludes` path), render/wiring assertions for the new affordance.

Out of scope: wizard removals (EXP-03, which frees the bootstrap handler
contract), any include/exclude glob editor beyond the existing picker.

## Implementation steps

1. Add the endpoint with the `docs.InitWithOptions` call and status codes.
2. Surface the affordance in settings.html and wire it in `initSettings`.
3. Add the hint states for the exclusions editor and the restart note.
4. Add endpoint + UI tests; run `go vet ./... && go test ./...`.

## Verification

- Unit tests green (`go test ./internal/server/ -run Docs`).
- Manual: with setting on and no config → button visible, click writes a
  default `agentsdocs.json`, restart brings docs up; with setting off →
  button hidden/409; with config present → button hidden.
- Hand-written `agentsdocs.json` globs survive an initialize re-run.

## Completion criteria

- A fresh repo can reach fully-running docs without ever touching the
  wizard or the CLI: enable setting → restart → Initialize coverage →
  restart.
