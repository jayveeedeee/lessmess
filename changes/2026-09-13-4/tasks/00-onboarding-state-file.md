---
id: ONB-00
title: Onboarding state file helpers
---

# ONB-00: Onboarding state file helpers

Status: see [../ledger.md](../ledger.md).

## Objective

Provide the persisted record of onboarding completion: `.lessmess/onboarding.json` with fail-open read helpers and atomic writes, following the one-file-per-feature tooling-state convention used by `sessions.json`, `docs-queue.json`, `settings.json`, etc.

## Dependencies

—

## Scope

- New `internal/server/onboarding.go` and `internal/server/onboarding_test.go`.
- Schema: `{ "version": 1, "completedAt": "<RFC3339, empty = incomplete>", "dismissed": bool, "steps": { "prereqs": "...", "bootstrap": "...", "agent": "...", "docs": "..." } }` — step values are short strings (`ok`, `done`, `set`, `skipped`, `seeded`); keep the map open-ended.
- Helpers: `loadOnboarding(dir)` (missing file → zero value, no error; malformed → zero value + logged warning, i.e. fail-open like settings), `saveOnboarding(dir, state)` (MkdirAll + `model.WriteFileAtomic`), and `onboardingPending(dir) bool` (`completedAt == "" && !dismissed`).

## Implementation steps

1. Define the `onboardingState` struct with the schema above and a `onboardingPath` constant (`.lessmess/onboarding.json` via `store.StateDirName`).
2. Implement `loadOnboarding` mirroring `readSettingsLayer`'s fail-open semantics.
3. Implement `saveOnboarding` with `model.WriteFileAtomic` (0o644), creating `.lessmess/` as needed.
4. Implement `onboardingPending`.
5. Write unit tests: roundtrip, missing file, malformed file fail-open, pending logic transitions (incomplete → dismissed → completed).

## Verification

- `go test ./internal/server -run Onboarding -v` passes.
- `go vet ./internal/server` clean.

## Completion criteria

- Helpers exist with the schema above, all tests pass, and no other code references the file yet (consumers land in ONB-01/ONB-06).

## Files affected

- `internal/server/onboarding.go` (new)
- `internal/server/onboarding_test.go` (new)

## Notes

- Decision 4 in [../plan.md](../plan.md): absent file means incomplete, never an error.
