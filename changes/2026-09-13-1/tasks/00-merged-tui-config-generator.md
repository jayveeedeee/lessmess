---
id: TUI-00
title: Merged TUI cli.json generator
---

# TUI-00: Merged TUI cli.json generator

Status: see [../ledger.md](../ledger.md).

## Objective

Add a helper in `internal/server` that generates the chrome-free opencode CLI
config used by embedded terminals: the user's `~/.config/opencode/cli.json`
(if present) with `tabs.enabled: false` and `session.sidebar: "hide"` forced
on top, written atomically to `.lessmess/xdg/opencode/cli.json`.

## Dependencies

None.

## Scope

- New `internal/server/tuiconfig.go` with a function along the lines of
  `ensureTUIConfig(stateDir string) (xdgDir string, err error)`.
- New `internal/server/tuiconfig_test.go`.
- No changes to `terminalWS` or `internal/terminal` here (that is TUI-01).

## Implementation steps

1. Locate the user config the same way opencode does: `$XDG_CONFIG_HOME` (or
   default `~/.config`) + `/opencode/cli.json`. Missing file is not an error —
   start from an empty object.
2. Parse as strict JSON; on failure apply a conservative JSONC cleanup (strip
   `//` and `/* */` comments outside strings, trailing commas) and retry; on
   continued failure return a sentinel that callers treat as "overrides-only"
   and log a warning.
3. Force `tabs.enabled=false` and `session.sidebar="hide"` (creating parent
   objects as needed), preserving every other key — including `$schema` —
   byte-value-for-value. If the user explicitly set either key, the override
   wins.
4. Write the result to `<repo>/.lessmess/xdg/opencode/cli.json` via
   `model.WriteFileAtomic` (create parent dirs), and return
   `<repo>/.lessmess/xdg` as the `XDG_CONFIG_HOME` value.
5. Keep the function pure with respect to the user's file: read-only access;
   all writes under `.lessmess/`.

## Verification

- `go test ./internal/server/ -run TUIConfig` covering:
  - no user file → overrides-only output;
  - strict-JSON user file → unrelated keys preserved, forced keys win;
  - commented/trailing-comma user file → merged when recoverable,
    overrides-only + warning otherwise;
  - output lands at `.lessmess/xdg/opencode/cli.json`, parent dirs created,
    write is atomic;
  - user file byte-identical after the call.
- `go vet ./internal/server/`.

## Completion criteria

- Generator merged with all tests above passing and no writes outside
  `.lessmess/`.

## Files affected

- `internal/server/tuiconfig.go` (new)
- `internal/server/tuiconfig_test.go` (new)

## Notes

- Schema source of truth: `https://opencode.ai/v2/cli.json` —
  `tabs.enabled` boolean ("persistent tab strip vs pinned quick-switch"),
  `session.sidebar` enum `"auto" | "hide"`.
- Implemented as `ensureTUIConfig(repoDir)` in `internal/server/tuiconfig.go`:
  resolves the user file via `$XDG_CONFIG_HOME`/`~/.config`, merges with
  `mergedTUIConfig` (strict JSON → conservative `stripJSONC` retry →
  overrides-only + `degraded` warning), forces the two keys, preserves all
  other user keys (incl. their `$schema`), adds the published `$schema` when
  absent, and writes atomically to `.lessmess/xdg/opencode/cli.json`.
  Verification: 7 tests in `tuiconfig_test.go`, full suite green 2026-09-13.
