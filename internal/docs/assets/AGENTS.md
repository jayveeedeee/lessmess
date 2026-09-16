# Agent Guide: internal/docs/assets

<!-- tasktracker:begin -->
## Learnings

- Purpose: holds canonical, agent-facing documentation assets that get embedded into generated repository docs. These are docs only; no build or runtime code lives in this directory.
- `workflow_agents.md` is the source template for the repository `AGENTS.md` content (the `changes/` planning and ledger workflow) and the embed source for `init`.
- Treat these files as source of truth for wording: edit them here rather than only in generated outputs, and keep edits format-neutral since they affect every generated `AGENTS.md`.
- The drift test compares `workflow_agents.md` to the repo root `AGENTS.md`'s curated portion only (the machine auto section is ignored), so re-sync the asset after editing the root workflow text.
- Keep canonical prose free of a literal HTML-comment marker pair — write only the marker names; embedding one would give generated docs a second marker pair and fail docs validation.
- This one file defines the workflow contract every initialized repo inherits: the six-status vocabulary with user-gated `Done`, the random five-character change-ID suffix (legacy `YYYY-MM-DD-N` staying valid), nested task decomposition with container ledgers and recursive validation/close-out, and the deterministic overall-status path (`POST /changes/{id}/status`, never hand-editing an `Overall status:` line).
<!-- tasktracker:end -->
