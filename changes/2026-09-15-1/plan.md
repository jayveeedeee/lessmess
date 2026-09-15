# 2026-09-15-1: Random change-ID suffixes

- Change ID: 2026-09-15-1
- Created: 2026-09-15
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Replace the per-date counter in change directory IDs (`YYYY-MM-DD-N`) with a
random 5-character lowercase alphanumeric suffix (`YYYY-MM-DD-xxxxx`). The
counter is allocated by scanning the local `changes/` tree for the highest
existing number, so two people working in parallel on the same repo can both
allocate the same next number on the same date and collide at merge time —
in the directory name and as adjacent root-ledger rows. A random suffix makes
collisions astronomically unlikely and harmless when branches merge.

## Current behavior

- `internal/store.CreateChange` scans `changes/` and `changes/archive/` for
  `date-*` directories, parses numeric suffixes, and mints `max+1`.
- `changeIDRe` (`internal/store/store.go`) is `^\d{4}-\d{2}-\d{2}-\d+$` and
  guards `Change(id)` lookups plus validation Rule 1 (active and archived).
- The workflow text (`AGENTS.md`, mirrored byte-for-byte in
  `internal/docs/assets/workflow_agents.md`) specifies zero-based per-date
  numbering with a no-reuse/gap rule.
- `Server.index` sorts the change list newest-first via `changeIDLess`: date
  lexicographically, counter numerically (`-9` below `-10`).
- `lessmess validate` enforces the naming regex for every change directory.

## Target behavior

- New change IDs are `YYYY-MM-DD-xxxxx` where `xxxxx` is exactly five
  characters from `[a-z0-9]`, minted with a cryptographically random source.
- Minting checks existing names (including `archive/`) and regenerates on the
  astronomically unlikely collision; the max-number scan and the workflow's
  gap/no-reuse rules are removed.
- Validation Rule 1 accepts both formats — `^\d{4}-\d{2}-\d{2}-(\d+|[a-z0-9]{5})$`
  — so every existing change directory stays valid with no migration.
- Index ordering stays newest-first: date descending, then root-ledger row
  position descending (last appended = newest), then ID ascending as fallback.
- The workflow text in `AGENTS.md` and the embedded asset describe the new
  scheme and continue to agree byte-for-byte.

## Scope

- `AGENTS.md` naming rules and validation item, plus regeneration of the
  pinned `internal/docs/assets/workflow_agents.md`.
- `internal/store`: ID regex, `CreateChange` minting, validation Rule 1
  message, and their tests.
- `internal/server`: index ordering comparator and its test.
- Learnings in `internal/store/AGENTS.md` and `internal/server/AGENTS.md`
  that describe the counter scheme (inside their doc markers).

## Non-goals

- No renaming or migration of existing `YYYY-MM-DD-N` directories.
- No change to task-file naming or task-ID prefixes (`00-…`, `EXC-00`).
- No user-configurable suffix alphabet, length, or format.
- No API changes: `POST /changes/scaffold` request/response shape is
  unchanged; the `change` field simply carries the new format.
- No UI changes beyond none needed — IDs are rendered verbatim.

## Design decisions

- Keep the `YYYY-MM-DD-` prefix: dates group changes and most sorting is the
  date prefix; only the suffix changes.
- Full `[a-z0-9]` alphabet (36⁵ ≈ 60.4M): IDs are machine-generated and
  click-through, so lookalike-character readability is not worth a smaller
  space or a more complex regex.
- `crypto/rand` for minting, behind an injectable source so tests can force
  collisions deterministically (matches the repo's injectable-seam style).
- Legacy numeric suffixes stay valid indefinitely: one union regex, no
  cut-over, mixed formats coexist within a single date.
- Root-ledger row position replaces the counter as the within-date ordering
  key. Rows are append-mostly, so last appended = newest reproduces today's
  order for legacy repos too (counters were allocated in append order).
- Other ID-sorted surfaces (`Store.Changes`/`Archived`) keep plain ID order;
  no surface they feed promises within-date chronology.

## Detailed implementation approach

1. Spec first (CID-00): rewrite the naming section and validation item 1 of
   `AGENTS.md`, then regenerate `internal/docs/assets/workflow_agents.md`
   with the documented awk command so the drift test passes.
2. Store (CID-01): widen `changeIDRe` to the union pattern; replace the
   max-number scan in `CreateChange` with random minting plus a
   set-based collision check over `changes/` and `changes/archive/`
   (bounded retries, error if exhausted); update the Rule 1 message.
3. Server (CID-02): thread the root-ledger row position into index rows and
   sort by (date desc, position desc, ID asc); retire the numeric-counter
   comparator.
4. Upkeep and verification (CID-03): refresh the stale in-marker learnings,
   then run the full test suite, rebuild, and `lessmess validate`.

## File-level impact

- `AGENTS.md` — naming rules and validation item text.
- `internal/docs/assets/workflow_agents.md` — regenerated wholesale.
- `internal/store/store.go` — `changeIDRe`, `CreateChange`.
- `internal/store/validate.go` — Rule 1 message text.
- `internal/store/store_test.go` — CreateChange and validation tests.
- `internal/server/server.go` — `changeIDLess`/`splitChangeID` replacement.
- `internal/server/server_test.go` — `TestIndexNewestFirst`.
- `internal/store/AGENTS.md`, `internal/server/AGENTS.md` — learnings.

## Data, API, message, configuration, or schema changes

- New change directories use the new name format; root-ledger rows reference
  them verbatim (schema unchanged).
- `POST /changes/scaffold` and `GET /api/changes` payloads keep their shapes.
- No configuration added; the suffix format is hardcoded as before.
- Validation Rule 1 violation text changes to name both accepted formats.

## Safety, security, rate-limit, migration, and rollback considerations

- `crypto/rand` avoids any seeding concerns; no user input reaches the
  suffix, so no injection surface.
- No data migration; rollback is rebuilding the previous binary — mixed
  historical IDs remain valid under both old and new rules.
- Collision retries are bounded (10) and practically never exhaust; exhaustion
  returns a normal error rather than blocking.

## Testing and verification strategy

- Unit tests: minted format matches `[a-z0-9]{5}`; uniqueness across many
  same-date mints; forced-collision regeneration via an injected source;
  Rule 1 accepts legacy and new names and rejects malformed ones
  (uppercase, 4 chars, bad date); index ordering with mixed same-date IDs.
- Repo-level: `go vet ./...`, `go test ./...`, rebuild the binary, and
  `lessmess validate` on this repository (mixed legacy/new IDs must be clean).
- Drift test confirms `AGENTS.md` and the embedded workflow asset agree.

## Observability requirements

- None required; a debug log line on collision retry is sufficient.

## Rollout sequence

1. Land spec text (CID-00), store (CID-01), server (CID-02) in order.
2. Rebuild the binary; scaffold a change in a scratch repo to see the new
   format end to end.
3. Refresh learnings and run full verification (CID-03).

## Risks and mitigations

- A hidden parser assuming a numeric suffix exists somewhere: mitigated by
  repo-wide grep during verification plus the full test suite and
  `lessmess validate` on the real mixed-format repo.
- Within-date ordering surprises for humans reading bare IDs: documented in
  the workflow text (ledger order, not suffix order).
- Parallel merge conflicts on the root ledger shrink but do not vanish for
  same-line edits; row adjacency is no longer systematically conflicting.

## Acceptance criteria

- A scaffolded change produces an ID matching `^\d{4}-\d{2}-\d{2}-[a-z0-9]{5}$`.
- Every pre-existing change directory (numeric IDs) still validates cleanly.
- Index page orders by date then root-ledger append order, newest first.
- `AGENTS.md`, the embedded asset, `lessmess validate`, and the actual
  minting behavior all describe and enforce the same scheme.
- Full verification passes: `go vet ./...`, `go test ./...`,
  rebuilt binary, `lessmess validate`.

## Tasks

1. [CID-00](tasks/00-workflow-text.md) — Update the workflow spec text.
2. [CID-01](tasks/01-store-random-suffix.md) — Store: mint random suffixes, accept both formats.
3. [CID-02](tasks/02-index-ordering.md) — Server: newest-first by ledger position.
4. [CID-03](tasks/03-learnings-and-verification.md) — Learnings upkeep and full verification.
