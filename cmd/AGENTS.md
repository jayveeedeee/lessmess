<!-- tasktracker:begin -->
# Agent notes: cmd

- This directory is a standard Go layout container; each child directory is a separate `package main` program, currently only `lessmess/`.
- Program logic belongs under `internal/`; keep files here limited to CLI wiring, flag parsing, and process lifecycle.
- Add a new command as `cmd/<name>/main.go` rather than adding subcommands to an existing binary.
- Run and test from the repository root so the `--dir` flag and `changes/` paths resolve correctly.
- (manual) The single program is `cmd/lessmess` (module `lessmess`), renamed from `cmd/tasktracker`; the built binary is invoked as `lessmess`, and `main.go` imports `lessmess/internal/...`.
<!-- tasktracker:end -->
