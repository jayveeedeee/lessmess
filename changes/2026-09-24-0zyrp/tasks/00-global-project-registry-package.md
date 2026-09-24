# MP-00: Global project registry package

## Goal

A small, reusable package that owns the user-level registry of projects — the single source of truth for which directories this lessmess instance serves — with no HTTP or store dependencies.

## Approach

- New package `internal/registry` with `registry.go`:
  - `Config{Projects []Project}`, `Project{Slug, Path, Added string}` — display names are deliberately **not** stored; they derive from each repo's `general.projectName` at render time (fallback: `filepath.Base(path)`).
  - `Path()` resolves the config location: `os.UserConfigDir()` → `<userconfig>/lessmess/config.json`, overridable via `LESSMESS_CONFIG` (needed for tests and unusual servers).
  - `Load()` is fail-open: missing file → empty registry, malformed JSON → empty registry + logged warning (same philosophy as `internal/server` settings).
  - `Save()` writes atomically (pattern: `model.WriteFileAtomic`) and creates the parent dir.
  - `Add(path)` validates the path is an absolute, existing directory; slugifies the folder basename to `[a-z0-9-]` (lowercased, non-alphanumerics → `-`, collapsed/trimmed, length-capped) and dedupes with `-2`, `-3`… suffixes; returns the created `Project`.
  - `Remove(slug)`, `Get(slug)`, `List()` — pure operations over the loaded config; callers own persistence (Load → mutate → Save).
- Unit tests: config roundtrip, missing/malformed fail-open, slug generation (spaces, mixed case, unicode, collisions), `Add` validation rejections (relative, nonexistent, file-not-dir), `Remove` of unknown slug.

## Files affected

- `internal/registry/` (new package)

## Verification

- `go test ./internal/registry/ -v` green, including the fail-open and slug-dedupe cases.
- `go vet ./...` clean.
