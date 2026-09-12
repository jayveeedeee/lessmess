# Agent notes: internal

<!-- tasktracker:begin -->
- Purpose: home of tasktracker's Go application packages — workflow model, store, HTTP server, docs management, opencode client, and PTY terminal — all consumed by `cmd/tasktracker`.
- Dependency direction flows inward: `model` is the lowest-level parser/serializer, `store` builds on it, and `server` builds on `store` plus `docs`, `opencode`, and `terminal`.
- Each subdirectory has its own `AGENTS.md` with package-specific notes; read that file before editing code in the package.
- The `changes/` tree is canonical data that these packages never own: mutations re-read on-disk content and write atomically via `model.WriteFileAtomic`, so external edits are never clobbered.
- Verify changes from the repo root with `go vet ./...` and `go test ./...`.
- (manual) The docs-system work is split across two packages: `docs` owns coverage config, tree walking, STRUCTURE.md generation/validation, and seed orchestration, while `server` owns the refresh queue, gardener runner, docs watcher, and explorer handlers.
- (manual) Each package in this tree — `model`, `store`, `docs`, `server`, `opencode`, and `terminal` — has its own `AGENTS.md`; there are no covered subdirectories beyond those packages.
- (manual) The docs subsystem is inert unless the target repository has a root `agentsdocs.json`; `docs.LoadConfig` returns nil and consumers no-op, so guard any new code on a non-nil config.
<!-- tasktracker:end -->
