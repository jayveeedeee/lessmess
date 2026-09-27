<!-- tasktracker:begin -->
# Structure: internal/registry

<!-- tasktracker-meta: refreshed=2026-09-24 source=2026-09-24-0zyrp tree=64aa62642799 -->

User-level registry of the repository directories one lessmess instance serves: fail-open config IO, atomic saves, slug allocation, and a mutex-guarded store.

## Entries

| Entry | Purpose |
| --- | --- |
| `registry.go` | Config types, fail-open load and atomic save, slug generation with dedupe, pure add/get/remove helpers, and the mutex-guarded Store that persists per operation. |
| `registry_test.go` | Tests for config roundtrip, fail-open loads, slug generation and dedupe, Add validation and duplicate rejection, and Store add/remove persistence. |
<!-- tasktracker:end -->
