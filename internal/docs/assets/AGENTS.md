# Agent Guide: internal/docs/assets

<!-- tasktracker:begin -->
## Learnings

- Purpose: holds canonical, agent-facing documentation assets that get embedded into generated repository docs. These are docs only; no build or runtime code lives in this directory.
- `workflow_agents.md` is the embed source (`//go:embed` in `init.go`) merged into a fresh repository's root `AGENTS.md`; it is a short preamble, not the full workflow contract (see below).
- Treat these files as source of truth for wording: edit them here rather than only in generated outputs, and keep edits format-neutral since they affect every generated `AGENTS.md`.
- The drift test compares `workflow_agents.md` to the repo root `AGENTS.md`'s curated portion only, cutting at the begin-marker line (the machine auto section is ignored), so re-sync the asset after editing the root workflow text.
- Keep canonical prose free of a literal HTML-comment marker pair — write only the marker names; embedding one would give generated docs a second marker pair and fail docs validation.
- The asset is deliberately a short preamble pointing at the tool-owned JSON store, the narrative `changes/` markdown, and board-spawned sessions: the detailed workflow contract (statuses, task lifecycle, endpoints, prime composition) lives in `internal/server`'s embedded instruction modules (`instructions.json`, composed by `instructions.go`, listed at `GET /workflow/instructions`) — contract changes happen there, not here.
<!-- tasktracker:end -->
