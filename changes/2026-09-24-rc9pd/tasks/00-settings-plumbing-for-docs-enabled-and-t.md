# EXP-00: Settings plumbing for docs.enabled and the experimental badge

Status: see [../ledger.md](../ledger.md).

## Objective

Add the `docs.enabled` tri-state setting (default Off) with full lockstep
plumbing, and introduce a reusable "Experimental" badge shown on the Docs
section and the `git.worktrees` field. No runtime behavior changes yet — the
gate itself lands in EXP-01.

## Dependencies

None (first task).

## Scope

- `internal/server/settings.go`: `DocsSettings` gains `Enabled *bool` with
  `json:"enabled,omitempty"`; `mergeSettings` picks it with
  `pickBool("docs.enabled", false, ...)` (default **false**); effective
  settings expose it. Add an exported `DocsEnabled(repoDir)` read helper so
  the CLI and startup wiring use the exact same layered, fail-open semantics.
- `internal/server/settingsapi.go`: no new validation needed (plain bool),
  but confirm PUT round-trips it in both scopes and `settingsAPIView`
  surfaces it.
- `internal/server/settingschange.go`: add `docs.enabled` to the
  `settingsFieldValue` dotted-path allowlist so the "✦ change" discussion
  button works on it (AGENTS.md flags this as a mandatory lockstep edit).
- `web/templates/settings.html`: new bool field `docs.enabled`
  (`data-field="docs.enabled"` `data-kind="bool"`, Inherit/On/Off select,
  source badge, ✦ change button) as the first field of the docs section;
  add an `Experimental` badge to the Docs section header and to the
  `git.worktrees` field label; tweak the worktrees help text to mention
  experimental status.
- `web/static/app.js`: add `"docs.enabled": false` to `BOOL_DEFAULTS`
  (~line 4467) so the Inherit label and fallback render correctly.
- `web/static/app.css`: new `.exp-badge` chip styled after `.src-badge` and
  existing theme/status tokens (no new hard-coded palette), reused by both
  features.
- Unit tests: settings merge/default coverage for `docs.enabled`
  (default false, project override, personal override, malformed file fails
  open).

Out of scope: server gating (EXP-01), wizard/template removals (EXP-03).

## Implementation steps

1. Extend `DocsSettings` + `mergeSettings` with the default-off tri-state, add
   `DocsEnabled`, and update the effective-settings test fixtures.
2. Add `docs.enabled` to the `settingsFieldValue` allowlist in
   `settingschange.go`.
3. Add the settings.html field and the two Experimental badges; add
   `.exp-badge` CSS.
4. Add the `BOOL_DEFAULTS` entry in app.js.
5. Extend settings unit tests; run `go vet ./... && go test ./...`.

## Verification

- `go test ./internal/server/ -run Settings` covers default/override paths.
- `go vet ./... && go test ./...` green.
- Rebuild + restart: Settings page shows the new field with source badge and
  ✦ change button; Inherit reads "Off (default)"; both badges render on Docs
  section and worktrees field.

## Completion criteria

- `docs.enabled` round-trips through GET/PUT in both scopes with correct
  sources map entries and default false; `DocsEnabled` follows the same
  precedence and defaults false on malformed settings.
- Experimental badge visible on Docs section header and `git.worktrees`.
- No behavior change: docs still run purely on `agentsdocs.json` presence
  until EXP-01 lands.
