# Agent notes: internal/docs

<!-- tasktracker:begin -->
- Purpose: manages the per-folder AGENTS.md/STRUCTURE.md doc pairs for covered repos — loading `agentsdocs.json`, walking the tree, rendering STRUCTURE.md, seeding content, validating freshness, and refreshing skeletons.
- Core model: `Config` (`config.go`) decides coverage; `Walk`/`Dir`/`PostOrder` (`walk.go`) inventory the tree; `Build` (`structure.go`) renders STRUCTURE.md bottom-up so parent rollups quote child purposes.
- Everything is idempotent and merge-safe: writes go through `model.MergeDoc`/`model.WriteFileAtomic`, content outside the tasktracker markers is preserved byte-for-byte, and unchanged trees keep their prior freshness stamp.
- `Seed` (`seed.go`) runs three phases (skeletons, summarization, settle rollups) and resumes via `.tasktracker/docs-seed.json`; a missing `agentsdocs.json` disables the whole system (`LoadConfig` returns nil, consumers no-op).
- `ValidateDocs` treats structural problems as errors but coverage/freshness gaps as warnings by design, so docs lag never blocks a change close-out.
- (manual) `DirDocs` (`readdocs.go`) is the read path for the explorer UI: it returns a directory's purpose, per-entry blurbs, and meta, reporting a placeholder purpose as empty and a missing STRUCTURE.md as zero values without error, while corrupt markers are errors.
- (manual) Tests build STRUCTURE.md fixtures as literal marker/meta/table strings, so changes to the rendered format usually require updating `structure_test.go` and `readdocs_test.go` together.
- (manual) `internal/docs/assets/` holds the canonical embedded docs (workflow instructions, seed templates) that `init` and `seed` write into target repositories; the root AGENTS.md workflow text is one of them.
- (2026-09-12-7) `Seed` rewrites the deterministic skeletons every run and tracks only LLM-summarized dirs in the cursor; its confinement now matches the gardener (pre-pass snapshot, no deletions, created `AGENTS.md` must use markers), and a rollback may delete an unreviewed newly created `AGENTS.md`.
- (2026-09-12-7) Live seed sessions need an absolute repo root and can outlast the service's 30s HTTP cap; `OpenCodeSummarizer` uses one session per dir and `WaitDone` retries transport timeouts until the context expires.
- (2026-09-12-8) `DirDocs` is the explorer's only docs read path: `internal/server` builds its view model from `Walk` plus `DirDocs` per directory, so the "placeholder -> empty, missing file -> zero values, corrupt markers -> error" contract must stay stable when the carry-forward parser changes.
- (2026-09-12-9) `StaleDirs(root)` (`validate.go`) returns the sorted covered dirs whose STRUCTURE.md freshness meta is missing/unparseable or lags the walked tree hash — a missing STRUCTURE.md counts as stale and it returns nil when `agentsdocs.json` is absent; it computes hash-staleness directly for the refresh union instead of parsing `ValidateDocs` message strings.
<!-- tasktracker:end -->
