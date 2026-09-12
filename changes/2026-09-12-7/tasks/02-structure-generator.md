---
id: DOC-02
title: Deterministic STRUCTURE.md generator
---

# DOC-02: Deterministic STRUCTURE.md generator

Status: see [../ledger.md](../ledger.md).

## Objective

Implement the bottom-up tree walk that renders `STRUCTURE.md` auto sections for all
covered directories without any LLM involvement: entry listing, child rollup
summaries, purpose-blurb merge-forward, and tree hashing for freshness checks.

## Dependencies

- DOC-00 (doc file format)
- DOC-01 (coverage config)

## Scope

- Walk covered dirs leaves-first so parent rollups can embed child summaries.
- For each dir: list immediate entries (files + covered child dirs), carry forward
  existing purpose blurbs from the current `STRUCTURE.md` (parse via DOC-00),
  emit placeholder blurbs for new entries, drop removed entries.
- Rollup: each covered child dir contributes a one-line summary (from its own
  `STRUCTURE.md` header/purpose line) to the parent's "below this folder" section.
- Compute a stable tree hash per dir (names + hashes of entries) and store it in
  the freshness metadata.
- Idempotent: regenerating an unchanged tree yields byte-identical files.
- Pure rendering — no file writes here; callers write via DOC-00 merge.

## Implementation steps

1. `internal/docs/walk.go`: covered-dir traversal returning a bottom-up ordered
   dir list plus per-dir entry inventory.
2. `internal/docs/structure.go`: blurb extraction from existing files, rendering
   the auto section (header, entries table/list, child rollups), tree hashing.
3. Tests over fixture trees (use `t.TempDir`): creation, blurb carry-forward, entry
   add/remove/rename, rollup composition, idempotence, hash stability and change
   detection.

## Verification

- `go test ./internal/docs` passes.
- Idempotence test proves byte-identical second run.
- Hash changes iff the covered tree changes (fixture-based assertions).

## Completion criteria

- Generator output for fixtures matches golden files.
- Existing human-written blurbs survive regeneration verbatim.

## Files affected

- `internal/docs/walk.go` (new)
- `internal/docs/structure.go` (new)
- `internal/docs/structure_test.go`, `walk_test.go` (new)

## Notes

- Placeholder blurb style should make "needs a blurb" obvious to the gardener/seed
  LLM pass (e.g. an em dash or `TODO` marker) without breaking markdown rendering.
- 2026-09-12 — Implemented in `internal/docs/walk.go` + `structure.go`. Decisions:
  - Single `## Entries` table covers files AND covered subdirs; a subdir row always
    quotes the child's current purpose line (single source of truth — annotate a
    child in its own STRUCTURE.md). No separate "Subdirectories" section: the
    table is the rollup. Deviates from the task's original two-section sketch;
    simpler and self-consistent.
  - Placeholder for unwritten purposes/blurbs is the em dash `—` (matches the
    ledger Empty convention).
  - Tree hash = recursive structure-only (names + kinds), 12-char truncated
    sha256; file content deliberately irrelevant (content-driven learnings belong
    to the close-out flow).
  - Freshness carry-forward: when the recomputed hash equals the existing meta's
    hash, the previous refreshed/source stamp is carried verbatim — this makes
    file-level regeneration truly byte-identical for unchanged trees.
  - Existing-file carry-forward covers the purpose line and non-placeholder file
    blurbs; cell text is sanitized single-line with `|` → `/`.
  - Walk never descends into uncovered dirs, never follows symlinks (listed as
    files), and excludes hidden entries plus the doc files themselves.
  - Corrupt markers in an existing STRUCTURE.md make Build fail (no-clobber).
- Verification evidence: `go test ./internal/docs -count=1` passes — golden leaf
  output, idempotent rebuild from disk, carry-forward of purpose/blurbs across a
  file addition with hash advancement at leaf and root, content-only change not
  affecting the hash, corrupt-marker refusal, symlink handling, PostOrder.
  `gofmt`/`go vet` clean; full suite green.
