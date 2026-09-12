<!-- tasktracker:begin -->
# Structure: internal/model

<!-- tasktracker-meta: refreshed=2026-09-12 source=seed tree=e163636b37e6 -->

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
| `table.go` | Markdown table parsing, rendering, and link cells |
| `taskfile.go` | Parses task-file YAML frontmatter |
| `templates.go` | Renders new task, plan, and ledger files |
<!-- tasktracker:end -->
