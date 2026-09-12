---
id: EXP-00
title: Exported per-dir docs reader
---

# EXP-00: Exported per-dir docs reader

Status: see [../ledger.md](../ledger.md).

## Objective

Expose a small, tested API in `internal/docs` that reads one directory's
`STRUCTURE.md` content (purpose line, entry blurbs, freshness meta) for UI
consumption, wrapping the existing unexported carry-forward parser.

## Dependencies

- —

## Scope

- New `internal/docs/readdocs.go` with an exported `DirDocs` function (or
  equivalent) returning purpose, blurbs, meta for a `*Dir` (or by path).
- Behavior for missing file / no markers / placeholders is defined and tested
  (missing → zero values, no error; corrupt → error).
- Unit tests.

## Implementation steps

1. Add `readdocs.go`: `type DocsContent struct { Purpose string; Blurbs map[string]string; Meta *model.DocMeta }` and
   `func DirDocs(d *Dir) (*DocsContent, error)` reading `d.Abs/STRUCTURE.md` via
   the existing parser.
2. Tests over fixture dirs: annotated file, skeleton with placeholders, missing
   file, corrupt markers.

## Verification

- `go test ./internal/docs` passes.

## Completion criteria

- The server can obtain everything the explorer renders from one exported call
  per directory.

## Files affected

- `internal/docs/readdocs.go` (new)
- `internal/docs/readdocs_test.go` (new)

## Notes

- Reuses `carryForward` (DOC-02); no parser duplication.
- 2026-09-12 — Implemented in `internal/docs/readdocs.go`: `DocsContent{Purpose,
  Blurbs, Meta}` + `DirDocs(d)`; placeholder purpose/blurbs normalize to empty;
  missing file yields zero values without error; corrupt markers/meta error.
  Verification: `go test ./internal/docs -count=1` covers missing/annotated/
  placeholder/corrupt cases; full suite green.
