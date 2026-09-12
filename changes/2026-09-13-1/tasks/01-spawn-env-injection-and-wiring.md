---
id: TUI-01
title: Spawn env injection and terminalWS wiring
---

# TUI-01: Spawn env injection and terminalWS wiring

Status: see [../ledger.md](../ledger.md).

## Objective

Let `terminalWS` spawn the opencode TUI with `XDG_CONFIG_HOME` pointing at the
generated config from TUI-00, with a fail-open fallback, and verify the XDG
redirect does not hide global opencode configuration the TUI process needs.

## Dependencies

- TUI-00 (generator exists to be wired in).

## Scope

- `internal/terminal/terminal.go`: env-aware spawn (additive — e.g.
  `SpawnWithEnv`; `Spawn` delegates with nil env = inherit) so existing
  callers/tests keep compiling.
- `internal/server/terminal.go`: call the generator before spawn; on success
  append `XDG_CONFIG_HOME=<xdgDir>` to the inherited environment, on error log
  a warning and spawn exactly as today.
- Tests for the new behavior.

## Implementation steps

1. Add the env-aware spawn to `terminal.Manager` (set `cmd.Env` only when env
   is non-nil).
2. In `terminalWS`, call `ensureTUIConfig(...)`; on success pass
   `append(os.Environ(), "XDG_CONFIG_HOME="+xdgDir)` to the env-aware spawn;
   on failure `log.Printf` a warning and use the plain spawn.
3. Keep the change minimal — no other terminal behavior (resize, bridging,
   close semantics) touched.
4. XDG blast-radius check: run `opencode2 debug paths` with and without the
   override and record what moves; confirm project `opencode.json` resolution
   (cwd-based) is unaffected and note whether any global config path the TUI
   process reads moves (if it does, evaluate linking/copying it into
   `.lessmess/xdg/` and record the decision in the ledger).

## Verification

- `go test ./internal/terminal/ ./internal/server/`:
  - env-aware spawn passes the given environment (e.g. spawn a command that
    echoes an injected var);
  - nil env inherits the parent environment;
  - `terminalWS` falls back to the plain spawn (terminal still opens) when the
    generator fails.
- `go vet ./...`.
- Record the `debug paths` comparison outcome in this task's Notes.

## Completion criteria

- Embedded spawns carry the `XDG_CONFIG_HOME` override; failure path spawns
  without it and logs; blast-radius findings recorded; all tests pass.

## Files affected

- `internal/terminal/terminal.go`
- `internal/terminal/terminal_test.go`
- `internal/server/terminal.go`
- `internal/server/terminal_test.go` (if extended for the fallback)

## Notes

- `SpawnCommand` stays injectable; this change is about the environment, not
  the command.
- Implemented: `terminal.Manager.SpawnWithEnv` (nil env = inherit; `Spawn`
  delegates), `terminalWS` calls `ensureTUIConfig` and passes `xdgEnv(xdg)`
  (replaces any inherited `XDG_CONFIG_HOME` so the override wins under
  first-match getenv semantics), fallback logs via slog and spawns with nil env.
- `xdgEnv` lives in `tuiconfig.go` beside the generator; duplicate env entries
  are filtered because Go/libc `getenv` typically returns the first match.
- Blast-radius check (2026-09-13): this opencode2 build has no `debug paths`;
  used `opencode2 debug config` ("list configuration sources") with and
  without `XDG_CONFIG_HOME=/tmp/tui-xdg-check` — outputs byte-identical, so
  providers/plugins/global `opencode.json` still resolve from
  `~/.config/opencode` and only `cli.json` follows the override. Project
  `opencode.json` is cwd-based and unaffected. No linking/copying needed.
- Tests: `TestSpawnWithEnvPassesEnv`, `TestSpawnNilEnvInherits` (terminal pkg),
  `TestTerminalWSInjectsTUIConfigEnv`, `TestTerminalWSConfigFailureFallsBack`,
  `TestXDGEnvReplacesInherited` (server pkg); `go vet ./...` + full
  `go test ./...` green 2026-09-13.
