<!-- tasktracker:begin -->
# Structure: tasktracker

<!-- tasktracker-meta: refreshed=2026-09-12 source=manual tree=41b32469d607 -->

Single-binary kanban server and docs toolkit for the changes/ workflow, with the CLI, Go packages, and embedded web UI at the repository root.

## Entries

| Entry | Purpose |
| --- | --- |
| `cmd/` | Go command entrypoints, with one subdirectory per built program. |
| `internal/` | Core Go packages behind the tasktracker binary: workflow model, store, HTTP server, docs management, opencode client, and terminal sessions. |
| `web/` | Embedded web UI assets for tasktracker, bundling HTML templates and static files. |
| `README.md` | Project overview covering build, usage, safety, opencode integration, and docs management. |
| `agentsdocs.json` | Coverage config for the agent-facing docs system: include and exclude globs. |
| `go.mod` | Go module definition and dependency requirements. |
| `go.sum` | Dependency checksums for module verification. |
| `mine.png` | Ad-hoc Minesweeper debug screenshot kept at the repo root, gitignored. |
| `opencode.json` | Pre-approved opencode agent permissions for sessions in this repository. |
| `overlap.png` | Ad-hoc UI overlap debug screenshot kept at the repo root, gitignored. |
| `tasktracker` | Local compiled tasktracker binary (gitignored build output from `cmd/tasktracker`). |
| `ui.png` | Screenshot of the tasktracker web UI kept at the repo root. |
<!-- tasktracker:end -->
