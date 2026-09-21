
# LRN-03: Stale-reference lint in internal/docs

Status: see [../ledger.md](../ledger.md).

## Objective

Detect `AGENTS.md` learnings that cite paths which no longer exist,
deterministically, across every covered directory.

## Dependencies

None.

## Scope

- New `internal/docs/learnings.go` with `StaleLearningRefs(root)`
  returning `map[string][]string` (repo-rel dir → missing refs, sorted).
- Integration into `ValidateDocs` as warning findings.
- v1 detects path references only (see plan non-goals).

## Implementation steps

1. Walk the covered tree; for each dir read `AGENTS.md`, parse with
   `model.ParseDocFile`, lint **only** the auto section; no auto section
   → skip.
2. Extract backticked spans (`` `…` ``). Skip a span when it: contains
   whitespace, starts with `http`, contains a marker string, resolves to
   nothing path-like (see 3), or any segment starts with `.` (hidden
   paths such as `.lessmess/…` — false-positive prone).
3. Path-likeness: contains "/" with a dotted final segment, or is a
   dotted name (extension of 1–6 alphanumerics). Trim trailing
   punctuation (`,;:.`) before checks — careful not to strip a real
   trailing dot segment; trim only after the final path segment's
   extension.
4. Resolution — a ref is **missing** only when none of these hit:
   repo-root-relative file/dir exists; dir-relative file/dir exists;
   base name matches one of the dir's walked entries (file or subdir);
   base name matches an entry row in the dir's own `STRUCTURE.md`.
5. `ValidateDocs`: for each dir/ref emit
   `Finding{SeverityWarning, File: <dir>/AGENTS.md, Msg: learning cites missing path "<ref>"}`,
   merged into the sorted output. Nil when config absent. Walk/hash
   errors behave like existing callers (error finding).

## Verification

- Extraction table tests: `internal/server/foo.go` (candidate),
  `anthropic/claude-sonnet-4-5` (skipped — no dotted final segment),
  `` `spawnSession` `` (bare identifier, skipped), code snippets with
  spaces (skipped), URLs (skipped), `.lessmess/x.json` (skipped),
  trailing punctuation trimmed.
- Resolution tests on a fixture tree: hit via repo-rel, dir-rel, walked
  entry, and STRUCTURE.md row; miss flags a warning with the exact
  message/file; auto-section-only (human text outside markers with a
  dead path is not flagged in v1); no config → nil.

## Completion criteria

`StaleLearningRefs` and the `ValidateDocs` integration pass the tables
above; `go vet ./...` / `go test ./...` green.

## Files affected

- `internal/docs/learnings.go` (new)
- `internal/docs/learnings_test.go` (new)
- `internal/docs/validate.go`
- `internal/docs/validate_test.go`

## Notes

- Precision over recall: a missed dead ref costs nothing today (no
  consumer); a false positive costs user trust in the bell.
- Per-dir ref lists are capped only by reality — keep counts small by
  construction (conservative extraction), no arbitrary cap.
