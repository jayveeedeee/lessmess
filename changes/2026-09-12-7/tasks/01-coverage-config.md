
# DOC-01: Coverage configuration (agentsdocs.json)

Status: see [../ledger.md](../ledger.md).

## Objective

Define the committed `agentsdocs.json` config format that decides which directories
get the doc pair, and implement its loader with validation and sensible defaults in
a new `internal/docs` package.

## Dependencies

- —

## Scope

- Config schema: include/exclude globs, per-directory overrides, default exclusions
  (`node_modules`, `.git`, `dist`, build outputs, hidden directories, `.tasktracker`,
  `changes`).
- Loader: read from repo root, validate globs, produce a matcher used by walk/seed/
  hook code.
- Absent config = docs system disabled (all consumers must no-op cleanly).
- A default config generator (used by `init` — DOC-03).
- This repository's own `agentsdocs.json`.

## Implementation steps

1. Create `internal/docs/config.go`: types, `Load`, defaults, validation errors.
2. Implement directory matching (glob evaluation relative to repo root, exclude
   wins over include, hidden-dir and VCS-dir skips always on).
3. Unit tests: glob edge cases, exclude precedence, missing file vs. invalid file,
   defaults.
4. Write this repo's `agentsdocs.json` (cover `cmd`, `internal`, `web`, `changes`;
   exclude `web/static` vendored assets if deemed noise — decide during
   implementation and record the decision).

## Verification

- `go test ./internal/docs` passes for config tests.
- Loading this repo's config yields the intended covered set (test with a walk dry
  run or explicit assertion).

## Completion criteria

- Config round-trips and validates; consumers can ask "is dir X covered?" cheaply.
- Missing config is distinguishable from invalid config (disabled vs. error).

## Files affected

- `internal/docs/config.go` (new)
- `internal/docs/config_test.go` (new)
- `agentsdocs.json` (new, committed)

## Notes

- Config is committed because it governs canonical files; runtime state stays in
  `.tasktracker/` per the AGENTS.md tooling-state rule.
- 2026-09-12 — Implemented in `internal/docs/config.go`. Decisions: absent config
  returns nil (disabled), invalid config is an error — callers distinguish the two;
  empty `include` defaults to `["**"]`; user `exclude` is added to the built-in
  defaults (`node_modules`, `vendor`, `dist`, `build`, `out`, `target`) rather than
  replacing them; patterns without a slash match base names at any depth, patterns
  with a slash match the full relative path, `**` matches any number of segments;
  the repo root is always covered when a config exists; hidden dirs and the whole
  `changes/` tree are never covered (canonical workflow data only, per AGENTS.md).
- This repo's `agentsdocs.json` covers everything except `web/static` (vendored
  third-party assets already documented by `web/static/VENDOR.md` — excluded to
  keep generated docs meaningful).
- Verification evidence: `go test ./internal/docs -count=1` passes — absent/
  invalid config, defaults, base-name vs. full-path excludes, `**` edges, include
  narrowing, OS separators, plus `TestRepoConfig` loading the real repo
  `agentsdocs.json` and asserting the intended covered set. Full `go vet ./... &&
  go test ./... -count=1` green; `gofmt` clean.
