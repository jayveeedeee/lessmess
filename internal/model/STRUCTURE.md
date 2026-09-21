<!-- tasktracker:begin -->
# Structure: internal/model

<!-- tasktracker-meta: refreshed=2026-09-18 source=seed tree=677e77cc5402 -->

Parses and serializes the markdown files of the `changes/` workflow defined in AGENTS.md.

## Entries

| Entry | Purpose |
| --- | --- |
| `testdata/` | Sample workflow-format fixtures used by parser and serializer tests. |
| `atomic.go` | Atomic file writes via temp file and rename |
| `docfile.go` | Splits and merges STRUCTURE/AGENTS auto sections |
| `docfile_test.go` | Tests for doc-file parsing, merging, and metadata |
| `ledger.go` | Parses root and per-change ledger markdown |
| `ledger_test.go` | Tests for ledger and task-file parsing |
| `model.go` | Package doc, status vocabularies, and parse errors |
| `serialize.go` | Mutates ledgers and re-renders their tables |
| `serialize_test.go` | Tests for ledger mutation, templates, and atomic writes |
| `state.go` | Canonical JSON workflow state: index and per-change types, strict versioned load, atomic save, and schema validation. |
| `state_test.go` | Tests for the JSON state: load and save round trips, version and unknown-field rejection, and validation issues. |
| `table.go` | Markdown table parsing, rendering, and link cells |
| `taskfile.go` | Parses task-file YAML frontmatter |
| `taskid.go` | Dotted task-ID helpers for nested decomposition: parent, child, last segment, depth, and segment validation. |
| `taskid_test.go` | Tests for the dotted task-ID helpers. |
| `taskledger_test.go` | Tests for the container task-ledger format: minimal headers, parse, render round-trip, and shared row mutations. |
| `templates.go` | Renders new task, plan, and ledger files |
<!-- tasktracker:end -->
