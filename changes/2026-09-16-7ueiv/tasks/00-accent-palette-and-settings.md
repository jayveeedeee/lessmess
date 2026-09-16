---
id: ACC-00
title: Accent palette, ui.accent setting, and roll-once initialization
---

# ACC-00: Accent palette, ui.accent setting, and roll-once initialization

Status: see [../ledger.md](../ledger.md).

## Objective

Establish the Go-side foundation: the curated accent palette, the `ui.accent` setting end to end (schema, merge, validation, options, Change-button allowlist), and the roll-once random initialization persisted to the personal settings layer.

## Dependencies

- None — this is the first task and the foundation for ACC-01 and ACC-03.

## Scope

- New `internal/server/accent.go`: palette data, lookup, resolution + roll-once persistence.
- `UISettings.Accent` / `EffectiveUISettings.Accent` with merge/clear semantics matching the existing plain-string fields.
- `PUT /api/settings` validation (422 on unknown id, always enforced — no service dependency).
- `GET /api/settings/options` gains the palette (`accents`).
- `settingsFieldValue` allowlist gains `ui.accent`.
- Unit tests for all of the above. No UI, no icon serving, no CSS changes here.

## Implementation steps

1. Create `internal/server/accent.go` with `AccentColor{ID, Label, Dark, DarkHover, Light, LightHover}` and `AccentPalette` — orange first as the default, then teal, green, blue, violet, pink, fuchsia, red, amber, cyan. Dark-theme values for light hues (green, amber, cyan) one step darker so white button text stays legible; light-theme values from the 600/700 step of the same hue family. Include `AccentByID(id)` and `DefaultAccent()`.
2. Add the resolve/roll helper: given effective settings, return the configured accent when valid; when `ui.accent` is unset in both layers, pick uniformly at random (`math/rand`, seed-independent), atomically persist it to the personal layer using the existing single-layer save path, log `rolled initial accent color: <id>`, and cache the result in a `sync.Once` so one process rolls at most once; if the persist fails, log a warning and keep the rolled value for the session. An unknown stored id logs a warning and falls back to default without rewriting the file.
3. Extend `settings.go` (`UISettings`, `EffectiveUISettings`) with `Accent string`; mirror how `git.defaultBranch` flows through merge and patch handling, including clearing (empty restores inheritance).
4. In `settingsapi.go`, validate a non-empty submitted `ui.accent` against the palette with 422 on unknown (independent of service reachability), and add the `accents` array (`id`, `label`, `hex` = dark value) to the options payload.
5. Add `"ui.accent"` to `settingsFieldValue` in `settingschange.go`.
6. Tests: palette lookup; roll-once (persisted to `.lessmess/settings.json`, idempotent across calls, respects pre-set project or personal values, invalid stored value fails open without rewrite); PUT 422/accept; options payload; allowlist; merge/clear.

## Verification

- `go vet ./...` and `go test ./...` green.
- Hand-run against the test fixture or a scratch repo: `PUT /api/settings?scope=personal` with `{"ui":{"accent":"teal"}}` persists to `.lessmess/settings.json`; with `{"ui":{"accent":"nope"}}` returns 422; `/api/settings/options` lists the palette.
- Deleting the personal accent key and re-reading effective settings rolls a new random value and writes it.

## Completion criteria

- Palette, setting, roll-once, and validation exist with passing tests; no UI or asset serving yet; behavior fully drivable via the API.

## Files affected

- `internal/server/accent.go` (new), `internal/server/accent_test.go` (new)
- `internal/server/settings.go`, `internal/server/settingsapi.go`, `internal/server/settingschange.go`
- `internal/server/settings_test.go`, `internal/server/settingsapi_test.go`, `internal/server/settingschange_test.go` / `settingswiring_test.go` (allowlist coverage)

## Notes

- Patch presence semantics for plain strings must match `git.defaultBranch` exactly — check how `applySettingsPatch` distinguishes absent vs. empty before wiring `Accent`.
- One process serves one repository (see root AGENTS.md), so the `sync.Once` cache is safe; a racing double-roll across processes is harmless (atomic writes, both values valid).
- Clearing both layers re-rolls by design (documented in plan.md); do not add a "has rolled" marker.
