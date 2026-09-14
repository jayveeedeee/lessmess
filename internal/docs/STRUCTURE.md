<!-- tasktracker:begin -->
# Structure: internal/docs

<!-- tasktracker-meta: refreshed=2026-09-13 source=manual tree=d0a9e98c968f -->

Implements agentsdocs management: coverage config, per-folder STRUCTURE.md/AGENTS.md generation, seeding, summarization, validation, and refresh.

## Entries

| Entry | Purpose |
| --- | --- |
| `assets/` | Canonical agent-facing documentation assets embedded into generated repository docs. |
| `config.go` | agentsdocs.json loading and coverage glob matching |
| `config_test.go` | tests for config loading and coverage rules |
| `init.go` | bootstraps workflow files into a repository |
| `init_test.go` | tests for repository initialization |
| `readdocs.go` | Reads a directory's STRUCTURE.md into UI-facing purpose, entry blurbs, and meta. |
| `readdocs_test.go` | Tests for reading docs content, placeholder handling, and corrupt markers. |
| `seed.go` | resumable three-phase seed orchestration |
| `seed_test.go` | tests for seed orchestration |
| `structure.go` | STRUCTURE.md rendering and carry-forward logic |
| `structure_test.go` | tests for structure generation |
| `summarize.go` | LLM summarizer interface and opencode session runner |
| `summarize_test.go` | Tests for the opencode summarizer's configured agent and model defaults. |
| `validate.go` | docs health checks and freshness findings |
| `validate_test.go` | tests for docs validation |
| `walk.go` | covered-tree walk, Dir model, and post-order |
| `walk_test.go` | tests for tree walking |
<!-- tasktracker:end -->
