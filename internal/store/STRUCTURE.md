<!-- tasktracker:begin -->
# Structure: internal/store

<!-- tasktracker-meta: refreshed=2026-09-18 source=2026-09-18-f52mn tree=976d7f299d13 -->

In-memory model of the changes/ tree with watching, validation, and safe atomic writes.

## Entries

| Entry | Purpose |
| --- | --- |
| `overall.go` | Derives a change's overall status from its task tree and converges drifted statuses into both ledgers via SetChangeStatus. |
| `overall_test.go` | Tests for overall-status derivation and sync: the derivation table, Done preservation, and convergence of external ledger edits in both ledgers. |
| `statedir.go` | Names the `.lessmess` tooling-state dir and migrates the legacy `.tasktracker` dir into it. |
| `statedir_test.go` | Tests for state-dir migration: renames legacy, keeps an existing current dir, and no-ops when neither exists. |
| `store.go` | Core Store type, tree scanning, and write operations |
| `store_test.go` | Unit tests for store loading and mutations |
| `tree.go` | Recursive task tree: node scanning, decomposition, parented task creation, governing-ledger moves, and subtree stats. |
| `tree_test.go` | Tests for the task tree: scan, decompose, child numbering, moves, stats, and watcher recursion. |
| `validate.go` | Checks the machine-checkable AGENTS.md validation rules |
| `validate_tree_test.go` | Tests for recursive task-tree validation violations and close-out readiness. |
| `watch.go` | fsnotify watcher with debounced reloads |
| `worktree_test.go` | Tests for worktree-backed change resolution: unresolvable root rows violate rule 6 and resolve cleanly once a change root is wired. |
<!-- tasktracker:end -->
