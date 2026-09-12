---
id: DOC-07
title: Validation and board staleness surfacing
---

# DOC-07: Validation and board staleness surfacing

Status: see [../ledger.md](../ledger.md).

## Objective

Extend `tasktracker validate` and the board UI so doc-system health is visible:
coverage, marker integrity, freshness against tree hashes, and pending/stale queue
state.

## Dependencies

- DOC-02 (tree hashing)
- DOC-05 (stale/queue state)

## Scope

- New validation rules (only when `agentsdocs.json` present): every covered dir has
  both doc files; marker structure intact (exactly one begin/end pair per file);
  freshness metadata parses; tree hash matches current tree (else "stale" finding);
  dirs flagged stale in `.tasktracker/` reported.
- Findings are warnings, not write-blockers, for freshness/staleness (docs lag by
  design); structural problems (marker corruption, unparsable metadata) are errors
  under the existing banner/refuse-writes discipline for doc files.
- Board: staleness indicator (e.g. banner entry or per-page badge) fed from the
  same checks, consistent with the existing validation-banner pattern.
- CLI `validate` output includes the docs section.

## Implementation steps

1. `internal/store/validate.go`: docs rules alongside existing change rules,
   reusing DOC-01 matcher and DOC-02 hashing.
2. Surface through the existing store→server validation channel; extend banner/
   templates minimally for the staleness indicator.
3. Tests: fixture repos with missing docs, corrupted markers, stale hashes; banner
   rendering coverage where practical.

## Verification

- `go test ./internal/store` (and affected server tests) pass.
- Manual: introduce a stale hash in a fixture; banner/CLI report it; fix; clears.

## Completion criteria

- All four check categories (coverage, markers, metadata, freshness) produce
  correct, actionable findings.
- Existing validation behavior for `changes/` is untouched (tests green).

## Files affected

- `internal/store/validate.go`
- `internal/store/validate_test.go` (or new docs-specific test file)
- `internal/server/render.go` / `web/` templates (indicator)

## Notes

- Keep the indicator quiet when the docs system is disabled (no config).
- 2026-09-12 — Implemented in `internal/docs/validate.go` (+ server/CLI/JS
  surfaces). Decisions:
  - Checks live in the docs package (`ValidateDocs(root, stale)`), NOT in
    internal/store — the store stays focused on the changes/ contract and
    gains no docs dependency. Findings use a new severity-typed `Finding`
    (error/warning), not the rule-numbered changes/ `Violation`.
  - Error severity: corrupt markers, unreadable/invalid metadata, config load
    failure. Warning severity: missing doc files, missing auto section,
    missing metadata, tree-hash staleness, queue-flagged stale. The CLI exits
    1 only for violations/errors; warnings print and exit 0.
  - The CLI peeks at `.tasktracker/docs-queue.json` for stale flags with a
    local struct reading only the `stale` field (schema stays server-owned);
    the server passes its live queue state instead.
  - Board: `/api/validate` gains a `docs` array; the banner lists docs errors
    alongside violations and docs warnings in a new amber `.banner.warn`
    variant (red reserved for violations/errors). Quiet when disabled, as
    required.
- Verification evidence: `go test ./internal/docs ./internal/server -count=1`
  — disabled repo silent, unseeded repo all-warnings (both doc files per
  covered dir), healthy after seed (zero findings), stale hash at leaf AND
  bubbled to root, corrupt markers error, queue stale reported; endpoint
  includes/omits findings correctly per config presence. Manual: `tasktracker
  validate` on this repo prints 27 warnings (unseeded) and exits 0. Full suite
  green; `gofmt`/`go vet` clean.
- 2026-09-12 — USER-REPORTED BUGFIX: the banner rendered "undefined: undefined"
  for every docs finding. Root cause: `docs.Finding` marshals lowercase JSON
  keys (`file`, `msg`) while the banner JS read `f.File`/`f.Msg` (matching
  `store.Violation`'s untagged, capitalized marshaling). Fixed in app.js
  (`f.file`/`f.msg`) and locked the wire format in
  `TestValidateEndpointIncludesDocsFindings` (raw-key assertions for
  severity/file/msg). All suites green.
