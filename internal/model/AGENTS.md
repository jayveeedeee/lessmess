# internal/model

<!-- tasktracker:begin -->
## Notes for agents

- This package is the markdown model for the `changes/` workflow: it parses, mutates, and re-renders the canonical workflow files per the AGENTS.md schemas.
- Key files: `model.go` (status vocabularies and `Empty`), `table.go` (table and link cells), `ledger.go` (root and change ledgers), `taskfile.go` (frontmatter), `docfile.go` (auto sections), `serialize.go` (mutations), `templates.go` (new-file rendering).
- Parsers are strict: pinned columns, valid statuses, and link cells required; literal `|` in a cell is an error, and unknown task-file frontmatter keys are rejected while unknown tasktracker-meta keys are tolerated.
- Mutation methods (`MoveTask`, `AppendTask`, `AppendRow`, `Update`, `SetOverall`) must preserve all non-table content byte-for-byte, and writes should go through `WriteFileAtomic` without clobbering human content outside the tasktracker markers.
- Tests rely on byte-stable fixtures under `testdata/`; do not regenerate them to match new output.
- (2026-09-12-7) `docfile.go` adds the docs-file format model: markers are matched as substrings, `ParseDocFile` splits prefix/auto/suffix, and `MergeDoc` rewrites only the auto section (appending after human content when no markers exist) while preserving all bytes outside it; corrupt marker structure yields a typed `*model.Error` so callers can refuse to write.
- (2026-09-12-7) The auto section carries a single freshness meta line recording refreshed date, source, and tree hash; unknown meta keys are ignored for forward compatibility.
- (2026-09-12-15) `TaskStatusOrder` now has six entries with `StatusTest` inserted immediately before `StatusDone`; `Valid()` iterates the order, so a new status only needs the constant plus an order placement, and `templates.go`'s scaffolded per-change status-definitions table must list it in the same position.
- (2026-09-16-le45q) Container task ledgers reuse the change-ledger machinery: `TaskLedger`/`ParseTaskLedger` (`ledger.go`) accept only the minimal headers `- Task: <id> (change <change-id>)` and `- Last updated:`, share the `taskTable` row-mutation core and `TaskColumns` with `ChangeLedger`, and `RenderTaskLedger` (`templates.go`) scaffolds a container's `ledger.md`.
- (2026-09-16-le45q) `taskid.go` holds the dotted task-ID helpers (`ParentTaskID`, `ChildTaskID`, `LastTaskSegment`, `TaskIDDepth`, `DottedSegmentsValid`); the rule they encode — the last dotted segment must equal the filename sequence — is positional, so task-file frontmatter stays exactly `id`+`title`.
<!-- tasktracker:end -->
