<!-- tasktracker:begin -->
# Agent notes: cmd

- This directory is a standard Go layout container; each child directory is a separate `package main` program, currently only `lessmess/` (the binary is invoked as `lessmess` while the repository folder stays `tasktracker`, and `main.go` imports `lessmess/internal/...`).
- Program logic belongs under `internal/`; keep files here limited to CLI wiring, flag parsing, and process lifecycle.
- Add a new command as `cmd/<name>/main.go` rather than adding subcommands to an existing binary.
- Run and test from the repository root so relative `--dir` defaults and `changes/` paths resolve correctly; `serve` is the exception — omitting `--dir` means hub mode over the global registry, not the working directory.
<!-- tasktracker:end -->
