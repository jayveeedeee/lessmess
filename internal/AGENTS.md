# Agent notes: internal

<!-- tasktracker:begin -->
- Purpose: home of lessmess's Go application packages — workflow model, store, HTTP server, docs management, opencode client, and PTY terminal — all consumed by `cmd/lessmess`.
- Dependency direction flows inward: `model` is the lowest-level parser/serializer, `store` builds on it, and `server` builds on `store` plus `docs`, `opencode`, and `terminal`.
- Each subdirectory has its own `AGENTS.md` with package-specific notes; read that file before editing code in the package.
- The `changes/` tree is canonical data that these packages never own: mutations re-read on-disk content and write atomically via `model.WriteFileAtomic`, so external edits are never clobbered.
- Verify changes from the repo root with `go vet ./...` and `go test ./...`.
- (manual) The docs-system work is split across two packages: `docs` owns coverage config, tree walking, STRUCTURE.md generation/validation, and seed orchestration, while `server` owns the refresh queue, gardener runner, docs watcher, and explorer handlers.
- (manual) Each package in this tree — `model`, `store`, `docs`, `server`, `opencode`, and `terminal` — has its own `AGENTS.md`; there are no covered subdirectories beyond those packages.
- (manual) The docs subsystem is inert unless the target repository has a root `agentsdocs.json`; `docs.LoadConfig` returns nil and consumers no-op, so guard any new code on a non-nil config.
- (manual) The module is `lessmess`, so every package here imports its siblings as `lessmess/internal/<pkg>`; the binary name and module were renamed from tasktracker, while the workflow file formats and their marker names are unchanged.
- (manual) `os/exec` use is confined to three packages: `server` (`gitcommit.go`, read-only `git` status/diff for preview), `opencode` (background service status), and `terminal` (PTY); actual commits still happen inside primed opencode sessions, not in Go.
<!-- tasktracker:end -->
