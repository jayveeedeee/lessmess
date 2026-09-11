---
id: OCI-01
title: opencode client package
---

# OCI-01: opencode client package

Status: see [../ledger.md](../ledger.md).

## Objective

Implement `internal/opencode`: service discovery, Basic auth, and typed REST wrappers for the endpoints the integration needs.

## Dependencies

OCI-00 (verified contracts).

## Scope

In scope: discovery (parse `opencode2 service status`, cache URL, re-discover on failure), auth (read password from `~/.config/opencode/service.json`, inject Basic), client methods: CreateSession, GetSession, ListSessions, RenameSession, Prompt, CreatePTY, CreatePTYConnectToken; context-aware, with timeouts. Types per the verified schemas.
Out of scope: SSE/WS proxying (OCI-02), mapping (OCI-03).

## Implementation steps

1. Types + discovery + auth transport (http.RoundTripper injecting Basic auth).
2. REST methods per OCI-00 contracts.
3. Fixture tests (recorded responses) for every method; discovery parsing tests; auth failure handling.
4. Optional live smoke test gated behind `TT_LIVE_OPENCODE=1`.

## Verification

- `go test ./internal/opencode` passes on fixtures; live smoke passes when enabled.

## Completion criteria

- Package compiles with full fixture coverage of wrapped endpoints.

## Files affected

- `internal/opencode/` (new)

## Notes

- (fill in during execution)
