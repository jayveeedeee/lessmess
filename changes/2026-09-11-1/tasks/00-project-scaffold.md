---
id: KAN-00
title: Project scaffold and CLI entry
---

# KAN-00: Project scaffold and CLI entry

Status: see [../ledger.md](../ledger.md).

## Objective

Create the Go module and the `tasktracker` CLI skeleton with `serve` and `validate` subcommands so later tasks have a buildable home.

## Dependencies

None.

## Scope

In scope: `go.mod` (module `tasktracker`), `cmd/tasktracker/main.go` with subcommand dispatch, `--port` (default 8080), `--dir` (default `.`), `--host` (default 127.0.0.1) flags for `serve`, a minimal HTTP handler returning 200, and graceful shutdown. `validate` is a stub that prints "not implemented" and exits 2 until KAN-06.
Out of scope: parsing, templates, real handlers.

## Implementation steps

1. `go mod init tasktracker`.
2. Write `cmd/tasktracker/main.go`: subcommand parsing via stdlib `flag` (`serve`, `validate`), config struct, HTTP server with stdlib mux, placeholder `GET /` handler, signal-based graceful shutdown.
3. `go build ./...` and smoke-run.

## Verification

- `go build ./...` succeeds.
- `go run ./cmd/tasktracker serve --port 18099` starts; `curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:18099/` returns 200; Ctrl-C shuts down cleanly.
- `go run ./cmd/tasktracker validate` prints the stub message and exits 2.
- `go vet ./...` clean.

## Completion criteria

- Module builds; both subcommands behave as specified above.

## Files affected

- `go.mod`
- `cmd/tasktracker/main.go`

## Notes

- Go 1.26.1 confirmed available in the environment.
