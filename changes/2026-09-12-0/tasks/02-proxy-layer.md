---
id: OCI-02
title: API/SSE/WS proxy layer
---

# OCI-02: API/SSE/WS proxy layer

Status: see [../ledger.md](../ledger.md).

## Objective

Add the terminal WebSocket bridge so the browser can drive an in-process PTY (running the opencode TUI) through tasktracker.

## Dependencies

OCI-01.

## Scope

In scope: a WebSocket bridge (`GET /terminal/ws?session={id}`) that spawns `opencode2 --session {id}` via creack/pty (cwd = repo root), bidirectionally copies between the browser WS and the PTY, handles resize control frames, and kills the PTY on disconnect (the opencode session persists in the service). PTY registry for shutdown cleanup.
Out of scope (dropped, 2026-09-12 decision log): browser-facing REST proxy and SSE pass-through — the browser needs neither because all service calls are orchestrated server-side via the opencode client. Also out: mapping endpoints (OCI-03), UI (OCI-04).

## Implementation steps

1. `go get github.com/creack/pty github.com/coder/websocket` (record deps in ledger).
2. `internal/terminal`: manager — Spawn(command, args, cwd, cols, rows) via creack/pty, Resize, Read/Write, Kill, registry for shutdown.
3. Server route `GET /terminal/ws?session={id}`: spawn → upgrade (origin-restricted to our host) → pump PTY→WS and WS→PTY (JSON control frame `{type:"resize",cols,rows}` vs raw input) → kill on close.
4. Tests: WS bridge against a fake PTY command (`cat` echo), resize handling, unknown session 404.

## Verification

- `go test ./...` passes, including the WS echo test through the real bridge.

## Completion criteria

- Bridge copies bytes both ways and applies resizes; PTY killed on disconnect.

## Files affected

- `internal/terminal/` (new)
- `internal/server/` (WS bridge route)

## Notes

- (fill in during execution)
