---
id: LRN-05
title: docs.gardenerModel setting
---

# LRN-05: docs.gardenerModel setting

Status: see [../ledger.md](../ledger.md).

## Objective

A tri-state `docs.gardenerModel` setting gives docs-queue jobs their own
model with the fallback chain: gardener override → `session.model` →
service default.

## Dependencies

None.

## Scope

- Schema + merge in `internal/server/settings.go`; precedence helper.
- Spawn wiring for the gardener closure in `internal/server/server.go`.
- Save-time validation and `settingsFieldValue` in
  `internal/server/settingsapi.go`.
- Settings page row (template + client JS suggestions/badge).

## Implementation steps

1. `DocsSettings.GardenerModel string` (`json:"gardenerModel,omitempty"`)
   and `EffectiveDocsSettings.GardenerModel`; standard tri-state merge
   (personal over project, empty clears, malformed fails open).
2. Helper `GardenerModel(repoDir string) string`: effective
   `docs.gardenerModel` when set, else effective `session.model`
   (i.e. `SessionDefaults`' model). Agent stays `SessionDefaults`'.
3. Spawn seam: extend the `spawnSession` internals so the gardener
   closure in `SetOpencode` can pass a model override while keeping the
   400-retry fallback (on 400, drop agent/model exactly as today).
4. `settingsFieldValue`: add `docs.gardenerModel` (lockstep rule).
5. PUT validation: non-empty value validated against the live model
   list like `session.model` (422 when unknown; skipped when the
   service is unreachable).
6. Settings page: row in the docs section with model suggestions from
   `/api/settings/options`; per-field badge shows the supplying layer,
   or "inherits session model" when unset; saving writes the selected
   scope only; clearing restores inheritance.

## Verification

- Unit tests: merge across layers; helper precedence (override set,
  override unset + session model set, both unset); `settingsFieldValue`
  returns the dotted path; validation 422 on unknown with fake client,
  skipped when unreachable.
- Wiring test: fake service client asserts the gardener session is
  created with the override model; with the override cleared, with
  `session.model` only.
- Render test: page shows the row, suggestions, and the inherit badge.

## Completion criteria

Setting the field changes the next enqueued job's session model without
a restart; clearing it restores inheritance; unknown values are
rejected at save time when the service is reachable.

## Files affected

- `internal/server/settings.go`
- `internal/server/settingsapi.go`
- `internal/server/server.go`
- `web/templates/settings.html` (Settings page template)
- `web/static/` (client JS for the row)
- `internal/server/settings_test.go`, `settingsapi_test.go`,
  `settingswiring_test.go`

## Notes

- Applies to all queue-driven docs jobs (close-out, manual,
  lint-routed) via the shared closure; seed sessions unaffected.
- No `gardenerAgent` in this change (plan non-goal).
- Remember the embedded-assets rule: template/JS changes require a
  rebuild to be visible in a running server.
