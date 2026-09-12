<!-- tasktracker:begin -->
# Agent notes: cmd/lessmess

- This directory is the CLI entrypoint; `main.go` is the only file and delegates to `internal/server`, `internal/store`, `internal/docs`, and `internal/opencode`.
- `run(args)` parses a subcommand first (`serve`, `validate`, `init`, `docs seed`) and returns an exit code; `main` just calls `os.Exit(run(os.Args[1:]))`.
- Each subcommand builds its own `flag.FlagSet` and shares a `--dir` flag pointing at the repository root containing `changes/` (default `.`).
- The opencode integration is optional: discovery failures are logged as warnings and the server/docs seeder continue in degraded mode.
- `queueStale` reads `.lessmess/docs-queue.json` (via `store.StateDirName`) directly, but its schema is owned by `internal/server`; only the `stale` field is consumed here.
- (2026-09-12-7) `init` reports per-artifact `created`/`merged`/`skipped` actions; `docs seed` accepts `--dry-run` (no writes, no cursor) and `--budget N` (summarizer sessions per run, 0 = unlimited).
- (2026-09-12-7) `runDocsSeed` absolutizes `--dir` with `filepath.Abs` before creating opencode sessions; the service rejects a relative session directory with a 500.
- (2026-09-12-7) `validate` exits 1 only for errors/violations; docs warnings (missing files, freshness lag) print and exit 0.
- (2026-09-12-14) The program is `cmd/lessmess` (module `lessmess`), built with `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`; `serve`, `validate`, and `docs seed` call `store.MigrateStateDir(dir)` before touching state, while `init` bootstraps fresh and does not migrate.
<!-- tasktracker:end -->
