<!-- tasktracker:begin -->
# Structure: internal/opencode

<!-- tasktracker-meta: refreshed=2026-09-21 source=2026-09-21-8235b tree=632a82eb6890 -->

Minimal Go client for the opencode background service HTTP API.

## Entries

| Entry | Purpose |
| --- | --- |
| `client.go` | Client, auth, discovery, and session API methods |
| `client_test.go` | Unit tests plus opt-in live smoke test |
| `integration.go` | Display-safe integration and credential adapters: list and get with method forms, key, OAuth, and command connect attempts, credential label, activation, and deletion |
| `integration_test.go` | Tests for the projection dropping secrets and unknown shapes and for redacted attempt statuses |
| `lifecycle.go` | Capability-gated session lifecycle: fork, staged revert, compaction, inbox and prompt delivery, compatible rename, child pages, and sanitized export |
| `lifecycle_test.go` | Tests for published and installed lifecycle contract variants plus shell and compaction message decoding |
| `management.go` | Service identity and location reads, provider and plugin status, and OpenAPI-driven management capability detection |
| `management_test.go` | Tests for identity fallback, positive status projections, exact capability detection, and rediscover retargeting |
| `mcp_permissions.go` | Display-safe MCP server and resource reads with runtime connect control, plus active and saved permission listing and removal |
| `mcp_permissions_test.go` | Tests for MCP failure categories, resource URI redaction, and permission listing and removal guards |
<!-- tasktracker:end -->
