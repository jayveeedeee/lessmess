
# SET-00: Settings storage, layering, and merge

Status: see [../ledger.md](../ledger.md).

## Objective

Create the settings model and layered storage in `internal/server`: typed
settings with built-in defaults, loading of the committed project file
(`lessmess.json`) and the gitignored personal file
(`.lessmess/settings.json`), per-field merge with source tracking, and
atomic per-layer saves.

## Dependencies

None.

## Scope

- New `internal/server/settings.go`:
  - `Settings` struct tree (`session`, `prompts`, `git`, `ui`, `docs`)
    with JSON tags; booleans as `*bool` (tri-state), strings empty = unset,
    prompt addenda as plain strings keyed per prompt
    (discussion, change, commit, repoCommit, gardener, explorer).
  - Built-in defaults (autoOpenTerminal true, showArchived true,
    autoGardenerOnClose true, everything else unset/empty).
  - Load: read project file (repo root `lessmess.json`) and personal file
    (`.lessmess/settings.json` via `store.StateDirName`); missing files are
    fine; malformed files fail open (defaults for that layer) and record a
    human-readable load error for surfacing.
  - Merge: effective value per leaf field = personal else project else
    default; produce a `sources` map (dotted field path →
    `default|project|personal`).
  - Save: write one layer atomically (`model.WriteFileAtomic`), creating
    `lessmess.json` or `.lessmess/settings.json` as needed; empty/absent
    fields are omitted from the written file so clearing works.
  - A mutex-guarded holder type the `Server` can own, with an
    `effective()` accessor returning a concrete (non-pointer) view.
  - Model-string helper: split `providerID/id` on the FIRST `/` only.
- New `internal/server/settings_test.go` covering the above.

## Implementation steps

1. Define the struct tree, defaults, and the concrete effective view.
2. Implement load (both layers, fail-open) and merge with sources.
3. Implement scoped save (partial update semantics: only submitted fields
   change; empty string / null clears that field from the layer).
4. Unit tests: defaults-only, project-only, personal-over-project,
   clear-restores-inherited-value, malformed file fail-open + error text,
   atomic write location, model-string split (incl. IDs containing `/`).

## Verification

- `go test ./internal/server/ -run Settings -v` passes.
- `go vet ./...` passes.

## Completion criteria

- Layered load/merge/save behaves per plan; sources map is accurate;
  malformed input never panics and never blocks the server.

## Files affected

- `internal/server/settings.go` (new)
- `internal/server/settings_test.go` (new)

## Notes

- Pure storage work: no HTTP, no wiring into handlers yet (SET-02/SET-03).
- File schema is identical for both layers; all fields optional.
