<!-- tasktracker:begin -->
# Structure: internal/store

<!-- tasktracker-meta: refreshed=2026-09-16 source=2026-09-16-7ueiv tree=b6ff413c2146 -->

In-memory model of the changes/ tree with watching, validation, and safe atomic writes.

## Entries

| Entry | Purpose |
| --- | --- |
| `statedir.go` | Names the `.lessmess` tooling-state dir and migrates the legacy `.tasktracker` dir into it. |
| `statedir_test.go` | Tests for state-dir migration: renames legacy, keeps an existing current dir, and no-ops when neither exists. |
| `store.go` | Core Store type, tree scanning, and write operations |
| `store_test.go` | Unit tests for store loading and mutations |
| `tree.go` | Recursive task tree: node scanning, decomposition, parented task creation, governing-ledger moves, and subtree stats. |
| `tree_test.go` | Tests for the task tree: scan, decompose, child numbering, moves, stats, and watcher recursion. |
| `validate.go` | Checks the machine-checkable AGENTS.md validation rules |
| `validate_tree_test.go` | Tests for recursive task-tree validation violations and close-out readiness. |
| `watch.go` | fsnotify watcher with debounced reloads |
<!-- tasktracker:end -->
