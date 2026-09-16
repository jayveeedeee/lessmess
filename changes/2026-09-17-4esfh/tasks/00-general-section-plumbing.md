---
id: PROJ-00
title: General settings section and project-name plumbing
---

# PROJ-00: General settings section and project-name plumbing

Status: see [../ledger.md](../ledger.md).

## Objective

Introduce the `general` settings section with `general.projectName`, the directory-basename
fallback, and the API surface every other task consumes.

## Dependencies

None — foundation task.

## Scope

- `internal/server/settings.go`, `settingsapi.go`, `settingschange.go`
- README settings mention
- Tests; no template or JS changes

## Implementation steps

1. In `settings.go`: add `GeneralSettings{ProjectName string \`json:"projectName,omitempty"\`}`;
   add `General GeneralSettings \`json:"general"\`` to `Settings`; add
   `EffectiveGeneralSettings{ProjectName string \`json:"projectName"\`}` and the matching
   `EffectiveSettings.General` field.
2. In `mergeSettings`: `eff.General.ProjectName = pickStr("general.projectName", project.General.ProjectName, personal.General.ProjectName)`.
3. In `loadEffectiveSettings(repoDir)`: after merging, if `eff.General.ProjectName == ""` set it to
   `filepath.Base(repoDir)` (guard degenerate empties with `lessmess`). Keep the sources map
   reporting `default` when unset — the fallback is a display default, not a stored value.
4. Add `effectiveProjectName(repoDir string) string` — loads effective settings (logged warning on
   load error, like the existing accessors), applies the fallback; this is the single read path
   for PROJ-01/03.
5. In `patchSection`: `case "general": return strict(&layer.General)`.
6. In `settingsapi.go`: add `DefaultProjectName string \`json:"defaultProjectName,omitempty"\`` to
   `settingsResponse`, set to `filepath.Base(repoDir)` in `settingsAPIView`.
7. In `settingschange.go`: add `case "general.projectName"` to the `settingsFieldValue` allowlist
   (in lockstep with `EffectiveSettings`).
8. README: one short mention of the `general.projectName` setting where the settings files are
   described.

## Verification

- New tests (patterns: `settingswiring_test.go`, `settingsapi_test.go`): fallback applies when both
  layers unset; personal/project layering wins in the right order; sources stay `default` for the
  unset case; `applySettingsPatch` accepts `{general:{projectName}}` into the requested layer and
  rejects unknown fields/sections as before; `settingsAPIView` populates `defaultProjectName` and
  the effective name; allowlist accepts `general.projectName` (extend a `settingschange_test.go`
  case).
- `go vet ./... && go test ./...` from the repo root.

## Completion criteria

All new tests pass; existing settings tests unchanged and green; the accessor and API fields exist
for PROJ-01/02/03 to build on.

## Files affected

`internal/server/settings.go`, `internal/server/settingsapi.go`, `internal/server/settingschange.go`,
`README.md`, test files beside them.

## Notes

- `readSettingsLayer` uses plain `json.Unmarshal`, so older readers tolerate the new key — no
  compatibility work needed.
- Decision recorded in plan: fallback computed at read time, never persisted.
