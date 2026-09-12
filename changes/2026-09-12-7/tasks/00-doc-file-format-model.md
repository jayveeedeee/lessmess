---
id: DOC-00
title: Doc file format model
---

# DOC-00: Doc file format model

Status: see [../ledger.md](../ledger.md).

## Objective

Define and implement the file format for `STRUCTURE.md` and `AGENTS.md` auto
sections: marker delimiters, freshness metadata, and byte-preserving merge
semantics, with parsers and serializers in `internal/model` alongside the existing
format code.

## Dependencies

- —

## Scope

- Marker constants: `<!-- tasktracker:begin -->` / `<!-- tasktracker:end -->`.
- Parse a doc file into prefix / auto-section / suffix segments.
- Freshness metadata line inside the auto section: last-refresh date, source change
  ID (or `seed` / `manual`), tree hash.
- Merge: replace only the auto section of an existing file, preserving everything
  outside markers byte-for-byte; create files that don't exist yet.
- Reuse atomic temp+rename writes (`internal/model/atomic.go`).
- Unit tests with `testdata/` fixtures.

## Implementation steps

1. Add `internal/model/docfile.go` with marker constants, a `DocFile` type
   (prefix/auto/suffix), `ParseDocFile`, and `RenderDocFile`/merge functions.
2. Define the freshness metadata format (single HTML-comment or delimited line
   inside the auto section) with parse/serialize helpers.
3. Handle edge cases: missing file, no markers (treat whole file as human content,
   append auto section), multiple marker pairs (error — validation catch),
   unterminated marker (error).
4. Write table-driven tests: round-trip, byte-identical preservation outside
   markers, metadata round-trip, creation from scratch, error cases.

## Verification

- `go test ./internal/model` passes, including new tests.
- Fixtures in `internal/model/testdata/` cover merge, creation, and error paths.

## Completion criteria

- Merge never alters bytes outside markers (proven by test).
- Metadata parses back exactly what was written.
- Error cases return typed errors suitable for the validation layer.

## Files affected

- `internal/model/docfile.go` (new)
- `internal/model/docfile_test.go` (new)
- `internal/model/testdata/` (new fixtures)

## Notes

- Keep the format HTML-comment-based so it renders invisibly in markdown viewers.
- 2026-09-12 — Implemented in `internal/model/docfile.go`. Decisions: markers are
  matched as substrings (not line-anchored) so parse→render is byte-exact for any
  file; `MergeDoc` normalizes the auto body to a single trailing newline and
  appends after human content when no markers exist; metadata line is
  `<!-- tasktracker-meta: refreshed=... source=... tree=... -->` with unknown keys
  ignored for forward compatibility; corrupt marker structure is a typed
  `*model.Error` so callers can refuse to write.
- Verification evidence: `go test ./internal/model -count=1` passes — round-trip
  byte-identity on `testdata/doc_structure.md`, human-only file preservation,
  five corrupt-marker error cases, merge create/append/replace (golden
  `testdata/doc_agents_merged.md`), merge refusal on corrupt markers, meta
  round-trip/absent/malformed/unknown-key cases. Full `go vet ./... && go test
  ./... -count=1` green; `gofmt` clean.
