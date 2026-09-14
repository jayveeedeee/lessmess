<!-- tasktracker:begin -->
# Structure: tasktracker

<!-- tasktracker-meta: refreshed=2026-09-13 source=manual tree=d38fdd717b10 -->

Single-binary kanban server and docs toolkit for the changes/ workflow, with the CLI, Go packages, and embedded web UI at the repository root.

## Entries

| Entry | Purpose |
| --- | --- |
| `cmd/` | Go command entrypoints, with one subdirectory per built program. |
| `internal/` | Core Go packages behind the tasktracker binary: workflow model, store, HTTP server, docs management, opencode client, and terminal sessions. |
| `web/` | Embedded web UI assets for lessmess, bundling HTML templates and static files. |
| `README.md` | Project overview covering build, usage, safety, opencode integration, and docs management. |
| `agentsdocs.json` | Coverage config for the agent-facing docs system: include and exclude globs. |
| `go.mod` | Go module definition and dependency requirements. |
| `go.sum` | Dependency checksums for module verification. |
| `lessmess` | Local compiled binary of the CLI, gitignored build output rather than a source entry. |
| `lessmess.json` | Committed project-layer settings (session, prompts, git, ui, docs sections) edited via the Settings page; optional, with unset fields inheriting built-in defaults. |
| `mine.png` | Ad-hoc Minesweeper debug screenshot kept at the repo root, gitignored. |
| `opencode.json` | Pre-approved opencode agent permissions for sessions in this repository. |
| `overlap.png` | Ad-hoc UI overlap debug screenshot kept at the repo root, gitignored. |
| `ui.png` | Screenshot of the lessmess web UI kept at the repo root. |
<!-- tasktracker:end -->
