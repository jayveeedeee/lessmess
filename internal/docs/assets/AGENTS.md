# Agent Guide: internal/docs/assets

<!-- tasktracker:begin -->
- Purpose: holds canonical, agent-facing documentation assets that get embedded into generated repository docs.
- `workflow_agents.md` is the source template for the repository `AGENTS.md` content, covering the `changes/` planning and ledger workflow.
- Treat these files as source of truth for wording; edit them here rather than only in generated outputs.
- Changes to these assets affect every generated `AGENTS.md`, so keep edits format-neutral.
- These are docs only; no build or runtime code lives in this directory.
- (2026-09-12-7) `workflow_agents.md` is the embed source for `init`; the drift test compares it to the repo root `AGENTS.md`'s curated portion only (the machine auto section is ignored), so re-sync after editing the root workflow text.
- (2026-09-12-7) Keep canonical prose free of a literal HTML-comment marker pair (write only the marker names); embedding one would give generated docs a second marker pair and fail docs validation.
- (2026-09-12-15) `workflow_agents.md` now carries the six-status vocabulary and the user-gated `Done` rule; because this asset is embedded into every initialized repo, its wording defines the task lifecycle for all downstream repositories.
<!-- tasktracker:end -->
