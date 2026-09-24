# MP-01: Multi-project serve mode

## Goal

`lessmess serve` without `--dir` boots every registered project in one process, mounts each at `/p/<slug>/`, and serves a minimal read-only landing page at `/`. `serve --dir X` is untouched.

## Approach

- `cmd/lessmess` `runServe`: change `--dir` default to empty; when set, run today's path verbatim. When empty, load the registry and build the hub (below) with a per-project boot callback. Generalize the existing `boot` closure into a `BootFunc(dir, publicBase string) (http.Handler, close func(), err error)`; `publicBase` is `host:port` (legacy) or `host:port/p/<slug>` (hub), so agent-facing primes get prefix-aware curl bases. The opencode client is discovered once and shared by every project server.
- New `internal/server/hub.go`: a hub handler owning the root namespace —
  - `GET /` renders a minimal projects landing (list: name, slug, path, availability; links into `/p/<slug>/`). Add/remove arrive in MP-04; this task ships the read-only page so the mode is usable end-to-end.
  - `GET /api/projects` returns the registry list with an `available` flag per project.
  - `/static/` and default-accent brand assets (`/icon.svg`, `/favicon.ico`, `/apple-touch-icon.png` — no accent roll, there is no repo to persist to) served once at the root; per-project brand routes keep resolving each repo's own accent under its prefix.
  - Projects mount via `http.StripPrefix("/p/", slugMux)`; unknown slug → 404.
  - Per-project boot at startup: `store.ErrNoChanges` mounts that project's setup wizard under its prefix (hot-swap stays per-project — the setup handler swaps only its own slot); a vanished directory stays registered but marked unavailable with a friendly stub page; any other boot error is logged and surfaces as unavailable, never fatal to the whole hub.
- The hub keeps each project's handler + close func so MP-04 can add/remove at runtime.

## Files affected

- `cmd/lessmess/main.go`
- `internal/server/hub.go` (new)
- `web/templates/projects.html` (new, minimal)
- `web/templates/layout.html` (register the page kind)

## Verification

- With `LESSMESS_CONFIG` pointing at a fixture registry of two fixture repos, `lessmess serve` (no `--dir`) serves both boards at their prefixes; `/` lists both; unknown `/p/nope` → 404; a repo missing `changes/` shows the setup wizard under its prefix.
- `lessmess serve --dir <fixture>` regression: existing behavior and URLs byte-identical.
- `go test ./...`, `go vet ./...` green.
