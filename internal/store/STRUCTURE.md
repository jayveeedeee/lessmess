<!-- tasktracker:begin -->
# Structure: internal/store

<!-- tasktracker-meta: refreshed=2026-09-12 source=seed tree=88f34dcffa1a -->

In-memory model of the changes/ tree with watching, validation, and safe atomic writes.

## Entries

| Entry | Purpose |
| --- | --- |
| `store.go` | Core Store type, tree scanning, and write operations |
| `store_test.go` | Unit tests for store loading and mutations |
| `validate.go` | Checks the machine-checkable AGENTS.md validation rules |
| `watch.go` | fsnotify watcher with debounced reloads |
<!-- tasktracker:end -->
