# CHAT-06: Tolerate CLI rename in service discovery

## Why

The opencode CLI renamed `opencode2` → `opencode` in its 2026-09-22
update. Both binaries still exist, but the old `opencode2` reads only
the old registration and prints `stopped` — it cannot see the live
service that `opencode` reports at `http://127.0.0.1:49374`. lessmess's
`Discover` (`internal/opencode/client.go`) runs only `opencode2 service
status`, parses no URL out of `stopped`, and fails — so **every lessmess
start since the update silently disables the entire opencode
integration** (chat, sessions, docs gardener, settings validation all
503 or no-op). Observed live on 2026-10-02: server boots logged
`opencode integration disabled err="opencode2 service status: …"` and
docs jobs failed with `opencode service unavailable` while the service
was healthy and answering.

## What

- `Discover` tries the known CLI names in order — `opencode2` first
  (historical), then `opencode` (renamed) — via a package-level
  `discoverCommands` seam.
- A CLI whose output parses to a URL wins immediately; a CLI that runs
  but reports no URL (e.g. `stopped`) is skipped, not fatal; an exec
  failure (binary absent, or a same-named CLI without the subcommand)
  is recorded and the next name tried.
- When no CLI answers, the shared registration file
  (`~/.local/state/opencode/service.json`, field `url`) is the final
  fallback — discovery then works even with no service CLI on PATH.
- Discovery fails only when every source is unusable, with an error
  naming what was tried.
- No behavior change on machines where `opencode2` still works: it
  keeps winning on the first try.

## Files affected

- `internal/opencode/client.go`
- `internal/opencode/client_test.go`

## Verification

- Unit tests with executable script fakes in `t.TempDir()` bound to the
  `discoverCommands` seam:
  1. first candidate exits non-zero, second prints a URL ⇒ the URL is
     returned (fallback works);
  2. first prints `stopped` (no URL), second prints a URL ⇒ the URL is
     returned (registration drift tolerated);
  3. first prints a URL ⇒ it wins without running the second;
  4. all fail ⇒ error naming the candidates;
  5. all CLIs fail but `~/.local/state/opencode/service.json` exists ⇒
     its `url` is returned.
- `go vet ./... && go test ./...` from the repo root.
- Live confirmation after rebuild + restart with a full PATH: the boot
  log carries no `opencode integration disabled` warning, and a docs
  refresh runs gardener sessions (which then also exercises the CHAT-05
  wait-route fix end to end).
