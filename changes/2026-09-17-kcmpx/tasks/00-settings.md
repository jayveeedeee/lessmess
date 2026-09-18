---
id: WTP-00
title: Git settings — worktrees switch, base branch, reviewer model
---

# WTP-00: Git settings — worktrees switch, base branch, reviewer model

Status: see [../ledger.md](../ledger.md).

## Objective

Extend `GitSettings` so the worktree/PR pipeline can be configured per the
layered settings pattern: a feature switch, the base branch for new change
branches, and the reviewer session's model override.

## Dependencies

None — first task of the change.

## Scope

- `internal/server/settings.go`: add `Worktrees *bool`, `BaseBranch string`,
  and `ReviewModel string` to `GitSettings`; materialize them in
  `EffectiveSettings` (`Worktrees` defaults false, `BaseBranch` defaults empty,
  `ReviewModel` defaults empty) and extend `settingsFieldValue`'s dotted-path
  allowlist in lockstep (`git.worktrees`, `git.baseBranch`, `git.reviewModel`).
- `internal/server/settingsapi.go` + settings template/JS: three new fields in
  the git section, saved via the existing per-section patch flow. No live
  validation for `baseBranch` (resolved against the repo at scaffold time);
  `reviewModel` follows the agent/model save-validation convention when the
  service is reachable.
- A small helper `worktreesEnabled(dir bool)`-style accessor used by later
  tasks (wraps the tri-state read).

## Implementation steps

1. Add the three fields to `GitSettings` with `omitempty` JSON tags and doc
   comments stating semantics (opt-in switch; base for `change/<id>` branches,
   empty = current branch at scaffold; model override de-escalating like
   `gardenerModel`).
2. Extend the effective-settings merge and the allowlist in one commit-sized
   unit so they cannot drift.
3. Add the settings-page fields following the existing git/general section
   markup, including save + reload behavior.
4. Unit tests: layered merge (project/personal/default), allowlist round-trip,
   patch apply replacing only the `git` section, template rendering of the new
   fields.

## Verification

- `go test ./internal/server/ -run 'Settings'` passes with the new cases.
- Settings page shows the fields; saving persists to the correct layer
  (project `lessmess.json` vs personal override) and reloads.

## Completion criteria

- The three settings exist, merge correctly, are editable in the UI, and the
  dotted-path allowlist and `EffectiveSettings` agree.

## Files affected

- `internal/server/settings.go`, `settingsapi.go`, `settings_test.go`,
  `settingsapi_test.go`, `settingswiring_test.go`
- `web/templates/*settings*`, `web/static/*` as needed

## Notes

- `git.defaultBranch` remains (informational root-ledger column) — wait: per
  plan, `BaseBranch` **promotes** it. Implementation decision recorded here:
  keep the JSON key `git.defaultBranch` and repurpose its meaning as the base
  branch, rather than adding `git.baseBranch` and migrating. The settings UI
  label changes to "base branch for change worktrees". Root-ledger behavior
  unchanged (still recorded). Update this note if the implementation prefers a
  new key.
- Reviewer model reuse: `GardenerModel(dir)`-style accessor (e.g.
  `ReviewModel(dir)`) for WTP-05.
