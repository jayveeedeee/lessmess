<!-- tasktracker:begin -->
# Structure: internal/store

<!-- tasktracker-meta: refreshed=2026-09-12 source=manual tree=1b26302dfb7f -->

In-memory model of the changes/ tree with watching, validation, and safe atomic writes.

## Entries

| Entry | Purpose |
| --- | --- |
| `statedir.go` | Names the `.lessmess` tooling-state dir and migrates the legacy `.tasktracker` dir into it. |
| `statedir_test.go` | Tests for state-dir migration: renames legacy, keeps an existing current dir, and no-ops when neither exists. |
| `store.go` | Core Store type, tree scanning, and write operations |
| `store_test.go` | Unit tests for store loading and mutations |
| `validate.go` | Checks the machine-checkable AGENTS.md validation rules |
| `watch.go` | fsnotify watcher with debounced reloads |
<!-- tasktracker:end -->
