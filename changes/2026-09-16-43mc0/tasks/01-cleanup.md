
# LRN-01: One-time curation of all AGENTS.md auto sections

Status: see [../ledger.md](../ledger.md).

## Objective

Rewrite every covered `AGENTS.md` marker-guarded auto section to the new
convention: current-state phrasing, no provenance prefixes, no duplicates, ≤15
entries per file — turning the accumulated change log into durable knowledge.

## Dependencies

LRN-00 (the convention being applied must already be defined and pinned).

## Scope

All 14 covered `AGENTS.md` files (approximate current entry counts):

- `./AGENTS.md` (54), `internal/server` (64), `web` (34), `web/templates` (30),
  `internal/docs` (16), `internal` (16), `internal/store` (9),
  `internal/docs/assets` (6), `cmd/lessmess` (6), `internal/model` (5),
  `internal/opencode` (4), `internal/terminal` (2), `internal/model/testdata`
  (2), `cmd` (1).

Rules applied to every section:

1. Reword change-narrative entries into current-state facts.
2. Merge entries that say the same thing; delete entries that are obsolete,
   superseded, or pure change narration with no durable content.
3. Drop `(change-id)`, `(seed)`, and `(manual)` prefixes.
4. Keep each section at ≤15 entries, biased toward the most operationally
   useful knowledge.
5. Preserve everything outside the markers byte-for-byte; keep exactly one
   begin/end marker pair.

## Implementation steps

1. Start with the two largest sections (`./AGENTS.md`, `internal/server`), then
   `web`, `web/templates`, then the remainder.
2. For each file: read the section, cross-check claims against the current tree
   (paths, symbols, behavior), rewrite per the rules above.
3. Where a kept learning cites paths, re-verify they resolve (the
   `StaleLearningRefs` lint will double-check).
4. After each batch, run the verification commands below.

## Verification

- `go vet ./... && go test ./...` from the repo root.
- Rebuild the binary (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`) and
  run `./lessmess validate` — no docs errors, no stale-learning findings.
- Scripted check: no auto-section line matches `^- \((20[0-9]{2}-[0-9]{2}-[0-9]{2}|seed|manual)\)`;
  per-file entry count ≤15.

## Completion criteria

All 14 sections conform; all checks green; the diff is reviewable per file.

## Files affected

- Every covered `AGENTS.md` (inside markers only).

## Notes

- Per AGENTS.md root learnings, a manual docs reconciliation normally needs no
  `changes/` record — here it is the substance of a recorded change, so it is a
  task like any other.
- Sections may legitimately end well under 15 entries; the cap is a ceiling,
  not a quota.
- Structural-pinning learnings (e.g. "tests hard-code the column count") are
  exactly the durable content to keep; date-stamped feature announcements
  ("X now does Y") are rewritten or dropped.
- Verification evidence (2026-09-16): scripted check found zero
  `(change-id)`/`(seed)`/`(manual)` prefixed lines across all 14 files and every
  section at ≤15 entries (largest: root, web, internal/server, internal/docs,
  internal at 15; web/templates 12; remainder smaller). `go vet ./...` and
  `go test ./...` green. Rebuilt binary: `lessmess validate` → `OK`, including
  the stale-learning lint — after rewording two of my own entries that flagged
  (bare `autosession.json` and placeholder `NN-slug.md`; both de-backticked).
  Final re-run of docs/store/server packages: ok.
- Per-file diffs are reviewable individually; every file's change is inside the
  markers only (workflow text above the root markers was LRN-00's edit).
