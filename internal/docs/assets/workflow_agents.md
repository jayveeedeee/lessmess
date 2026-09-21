# Repository Agent Instructions

This repository uses **lessmess** for change management.

- Workflow state (changes, tasks, statuses, dependencies, decision logs) is tool-owned JSON under `.lessmess/workflow/` — committed, but **never edit it by hand**. Alter state only through the API endpoints or the board; `lessmess validate` checks the contract.
- Narrative documents — `changes/<id>/plan.md` and task prose under `changes/<id>/tasks/` — are markdown and edited directly.
- Change work happens in board-spawned sessions: their prime messages carry the current workflow instructions, deterministically selected and injected by the server from its embedded instruction modules (`GET /workflow/instructions` lists them).
- Do not scaffold changes yourself: discuss with the user and use the scaffold endpoint a session prime provides.

The sections below the markers are machine-maintained repository knowledge (doc pairs), refreshed by the doc gardener.
