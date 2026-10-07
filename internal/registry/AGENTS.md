# Agent notes: internal/registry

<!-- tasktracker:begin -->
## Learnings

- Purpose: the user-level registry of repository directories one lessmess instance serves — instance state, deliberately stored outside any repository, so hub mode (`serve` with no `--dir`) has one authoritative project list while `serve --dir` never touches it. The file resolves through `LESSMESS_CONFIG`, then the XDG config home, defaulting to `~/.config/lessmess/config.json` — the same user-level convention the opencode service follows for its own config.
- The package builds only on `model` (for `WriteFileAtomic`); keep HTTP, store, and server imports out so `cmd` and `server` can both use it at the dependency floor. State follows the lessmess fail-open philosophy: a missing file reads as an empty registry, a malformed file as empty plus a warning that the next save replaces, and every write is atomic with the parent directory created.
- The pure helpers (`Add`/`Get`/`Remove`/`List` over a `Config`) leave persistence to the caller (load → mutate → save); `Store` is the mutex-guarded wrapper that persists per operation and re-reads the file on every call, so external edits between calls always win — the hub uses `Store` as its single writer handle.
- Slugs are the stable identity (URL mounts and API paths): slugified folder basename capped at 40 chars with `-2`/`-3` collision suffixes, so remove-then-re-add of the same directory reproduces its slug. Display names are deliberately not stored — they derive from each repo's `general.projectName` (basename fallback) at render time — and registry management is UI-only by design: the server is the file's only writer, with no CLI verbs.
<!-- tasktracker:end -->
