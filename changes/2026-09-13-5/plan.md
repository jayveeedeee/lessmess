# 2026-09-13-5: Learning staleness: ancestor gardening, reference lint, gardener model

- Change ID: 2026-09-13-5
- Created: 2026-09-13
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

The docs subsystem keeps `AGENTS.md`/`STRUCTURE.md` pairs per covered
directory. When a change removes or alters a feature, `STRUCTURE.md`
self-heals deterministically (tree hash + skeleton refresh), but
`AGENTS.md` learnings have no detection or repair mechanism:

- Learnings about child directories routinely live in **ancestor**
  `AGENTS.md` files (seed skims covered children; cross-cutting
  conventions belong up a level; manual writes). A close-out job only
  ever targets the dirs listed in "Files affected", so a stale learning
  in a parent is never even visited.
- Within visited dirs, the prompt is preserve-biased ("keep existing
  learnings unless one is now wrong; fix those in place") and never
  mentions removals, so deletions happen by luck, not by design.
- Nothing detects rot anywhere: a learning citing a deleted file
  persists indefinitely and misleads future agents.

Additionally, all docs sessions currently use the shared session
agent/model defaults; the user wants a dedicated model override for the
gardener, falling back to the session default.

Goal: make learning staleness *detected* (deterministic lint), *reached*
(ancestor review-and-fix targets), and *repairable under the right
model* (`docs.gardenerModel` setting).

## Current behavior

- `touchedDocsDirs` (`internal/server/touched.go`) maps "Files affected"
  bullets to nearest covered ancestors; that set is the job's whole
  world. `gardenerPrompt` forbids touching anything else.
- `DocsJob` has Change/Title/Dirs/Enqueued; `.lessmess/docs-queue.json`
  holds pending + stale.
- `gardenerRunner.RunDocsJob` (`internal/server/docssession.go`):
  whole-tree skeleton refresh → snapshot job dirs → one gardener session
  → `verifyGardener` confinement → settle pass. Restore may delete files
  the session created.
- `ValidateDocs` (`internal/docs/validate.go`) reports structural errors
  and coverage/freshness warnings; `POST /docs/refresh` enqueues one
  manual job for the union of queue-stale and hash-stale dirs.
- `SetOpencode` (`internal/server/server.go`) wires a spawn closure into
  `gardenerRunner` that resolves session agent/model via
  `SessionDefaults` (`internal/server/settings.go`) with a 400-retry
  fallback that drops both.

## Target behavior

1. **Ancestor review-and-fix.** Close-out jobs additionally carry the
   covered ancestors of every touched dir (deduped). The gardener prompt
   gets two sections: *update* dirs (unchanged rules) and *review-and-fix*
   dirs, where the model must fix or delete learnings referencing what
   the change removed or changed (deletion explicitly licensed), add
   only genuinely level-appropriate learnings, and account for every
   removal/edit in its reply. Ancestors are full targets in the
   snapshot/verify/restore cage.
2. **Stale-reference lint.** A deterministic check in `internal/docs`
   parses every covered `AGENTS.md` auto section, extracts backticked
   path-like references, and flags those resolving to nothing (checked
   repo-relative, dir-relative, against the dir's walked entries and its
   own STRUCTURE.md entries). Findings are warnings in `ValidateDocs`
   (bell + `lessmess validate`). `POST /docs/refresh` unions
   lint-flagged dirs into the manual reconciliation job, whose prompt
   names the flagged references.
3. **Gardener model.** New tri-state setting `docs.gardenerModel`.
   Precedence for docs-queue jobs: `docs.gardenerModel` →
   `session.model` → service default. Save-time validation, Settings
   page row with live suggestions, `settingsFieldValue` entry.

## Scope

- Ancestor computation, `DocsJob.Ancestors`, two-section prompt,
  verify/restore coverage for ancestors (change-close jobs only).
- Reference lint over covered `AGENTS.md` auto sections; findings
  integration; refresh union; lint context in manual job prompts.
- `docs.gardenerModel` schema field, precedence helper, spawn wiring,
  save-time validation, Settings page row.
- README and package doc updates.
- **Scope addition (2026-09-13, user-requested): resumable seed on the
  normal server + exclusions editor** (LRN-07..09). The seed cursor
  already makes `lessmess docs seed` resumable; this adds a normal-server
  `POST /docs/seed` + `GET /docs/seed-status` sharing the setup wizard's
  job registry and cursor, a pending-dirs count in `/api/validate` with a
  bell "Seed pending docs" button, and a lightweight exclusions editor
  (`GET/POST /docs/exclusions`, Settings Docs section) persisting to
  `agentsdocs.json` with the wizard's replace-picker-patterns /
  preserve-hand-authored-globs semantics.

## Non-goals

- Behavioral staleness detection (prose claims with no path reference).
- A separate curation/consolidation pass or periodic review.
- `gardenerAgent` override (model only; trivial follow-up if wanted).
- Seed sessions (`docs seed`, onboarding) — they keep session defaults.
- Ancestor expansion for manual reconciliation jobs (the lint routes
  parent rot instead).
- Any change to marker syntax, workflow text, or the changes/ workflow.
- UI changes beyond the Settings row and the bell/Settings additions
  listed above.
- Root-directory exclusion (the root is always covered by design) and
  nested exclusion editing outside the wizard's lazy picker (v1 editor is
  top-level base names, matching their any-depth semantics).
- Excluding folders from a seed run via request parameters — exclusions
  live in `agentsdocs.json` and always apply.

## Design decisions

- **Ancestors, not "check parents read-only":** attention without write
  license cannot repair anything; ancestors become first-class targets
  inside the existing cage instead of a new mechanism.
- **Manual jobs stay single-section.** Their dir set is hash/lint-driven
  and already global; ancestor expansion there would balloon scope. Part
  C covers the parent-dir gap by routing flagged dirs into refresh.
- **Deletion license is scoped:** the preserve-bias clause keeps the
  update-section wording verbatim; only review-and-fix sections get the
  explicit "fix or delete, then account for it" language. The reply
  becomes an auditable accounting.
- **Lint v1 is path-only and conservative** to keep precision high:
  backticked spans only; must be path-shaped with a **known file
  extension** (an allowlist — this is the main symbol filter, since
  Go-style identifiers like `server.New` or `Server.index` read as
  dotted names but have no file extension); skips spans with
  whitespace, http(s), marker strings, qualifier characters
  (`*model.Error`, `cmd/<name>/…`), and hidden segments. Resolution is
  deliberately generous: a reference resolves when it exists
  repo-root-relative, dir-relative, anywhere in the tree by full-path
  suffix or base name (learnings freely cite child-dir files by bare
  name), or as `.lessmess` tooling state. Only spans surviving those
  shape rules *and* failing all resolution are flagged. Warnings only,
  no auto-fix without a job.
- **Warnings only, never blocking:** consistent with "docs lag by
  design"; nothing in the change workflow gates on docs findings.
- **Model override applies to every queue-driven docs job** (change
  close, manual, lint-routed) because they share one spawn closure;
  seed paths are untouched per user scope.
- **Backward compatibility:** `DocsJob.Ancestors` and `LintRefs` are
  optional JSON fields; existing `.lessmess/docs-queue.json` state
  loads unchanged. `docs.gardenerModel` unset behaves exactly as today.

## Detailed implementation approach

- `internal/server/touched.go`: add lexical ancestor computation — for
  each touched dir, every covered dir on its path to the root
  (`cfg.Covered` gates each; root `.` always covered when a config
  exists), deduped against the primary set, sorted. `touchedDocsDirs`
  gains a sibling (or returns both sets); `enqueueDocsRefresh` populates
  `DocsJob.Ancestors`.
- `internal/server/docsqueue.go`: `DocsJob` gains
  `Ancestors []string` and `LintRefs map[string][]string` (both
  `omitempty`). `docsRefresh` adds `docs.StaleLearningRefs` dirs to the
  union (lint error → log and continue with the other sets) and stores
  the flagged refs in the manual job.
- `internal/server/docssession.go`: `gardenerPrompt` renders update and
  review-and-fix sections (manual branch keeps one section but lists
  `LintRefs` when present). `RunDocsJob` snapshots/verifies/restores
  update ∪ review targets. slog job-done line gains the model used.
- `internal/docs`: new `learnings.go` with `StaleLearningRefs(root)`
  (dir → missing refs) following the extraction/resolution rules above;
  `ValidateDocs` emits one warning per flagged ref
  (`learning cites missing path "x/y.go"`), sorted with existing
  findings; nil when no config.
- `internal/server/settings.go`: `DocsSettings.GardenerModel` +
  `EffectiveDocsSettings.GardenerModel` + merge; `GardenerModel(repoDir)`
  helper implementing the precedence chain. `spawnSession` gains a model
  override seam (keeping the 400-retry fallback); the `SetOpencode`
  gardener closure resolves the chain (agent still from
  `SessionDefaults`).
- `internal/server/settingsapi.go`: PUT validation treats
  `docs.gardenerModel` like `session.model` (422 on unknown when the
  service is reachable); `settingsFieldValue` gains `docs.gardenerModel`;
  Settings page row (badge reads "inherits session model" when unset)
  with live model suggestions from `/api/settings/options`.
- `README.md`: docs-management section (ancestor review-and-fix, lint
  findings, extended refresh union) and settings table row.

## File-level impact

- `internal/server/touched.go`, `docsqueue.go`, `docssession.go`,
  `settings.go`, `settingsapi.go`, `server.go` (spawn wiring)
- `internal/docs/learnings.go` (new), `validate.go`
- `web/templates/*` + `web/static/*` (Settings page row only)
- `README.md`
- Tests: `internal/server/*_test.go` (touched/queue/prompt/settings
  suites), `internal/docs/learnings_test.go` (new), `validate_test.go`

## Data, API, configuration, schema changes

- `DocsJob` JSON: optional `ancestors`, `lintRefs` (state file
  `.lessmess/docs-queue.json` gains both; old files load unchanged).
- Settings schema: `docs.gardenerModel` (string, tri-state) in both
  layers; effective view exposes it; no other endpoint changes.
- `ValidateDocs` findings grow warning entries (additive; bell and
  `lessmess validate` render them via the existing severity grouping).

## Safety, security, rate-limit, migration, rollback

- All doc writes remain inside the snapshot/verify/restore cage; the
  lint is read-only; settings writes stay atomic and layer-scoped.
- No migration: new job fields and the setting are optional; absent
  values reproduce today's behavior exactly.
- Rollback: revert the change; queue state with unknown fields is
  already tolerated (structs ignore absent fields; old binaries ignore
  new ones only if JSON decoding is lenient — `docsQueueState` uses
  default decoding, so old binaries reading new state files keep the
  new fields out of the struct harmlessly).
- No new external calls; the lint walks local files only.

## Testing and verification strategy

- Unit/table tests: ancestor computation (nested, dedup, root, hidden,
  uncovered), `DocsJob` roundtrip, prompt rendering (both sections,
  deletion license, accounting, manual + lint refs), lint extraction
  (model IDs, snippets, URLs, hidden paths, bare identifiers skipped)
  and resolution (repo-rel, dir-rel, entry hits/misses), settings merge
  and 422 validation, `docsRefresh` union with fake lint.
- Server tests: violation in an ancestor target restores and fails the
  job; created `AGENTS.md` in an ancestor without markers is a
  violation; refresh includes lint-flagged dirs; `nothing to refresh`
  when all sets empty.
- Gates: `go vet ./...`, `go test ./...`,
  `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`,
  `lessmess validate`.
- Live E2E: (1) learning citing a deleted path → warning appears,
  refresh routes a manual job naming it; (2) close a change whose tasks
  list removed files → ancestors appear as review-and-fix targets;
  (3) set `docs.gardenerModel` → next docs job session created with it;
  clear it → falls back to session model.

## Observability

- `docs job done` log line gains the resolved model.
- Lint findings surface through the existing bell (amber) and
  `lessmess validate`; no new notification channel.

## Rollout sequence

Ordered tasks: ancestors (LRN-00 → 01/02), lint (LRN-03 → 04), setting
(LRN-05), docs + full verification (LRN-06). Each task is independently
buildable and testable; no feature flag needed since unset/empty
behavior is identical to today.

## Risks and mitigations

- **Lint false positives annoy users** → conservative path-likeness and
  multi-interpretation resolution; warnings only; findings are per-ref
  so noise is visible and bounded.
- **Gardener over-deletes in ancestors** → deletion license limited to
  review-and-fix sections; reply must account for each removal;
  confinement cage still guards structure; snapshots allow manual
  recovery via git.
- **Root `AGENTS.md` writes** (root is an ancestor of everything) → the
  workflow text lives outside markers and is byte-verified; this repo's
  closes already update root learnings safely.
- **Model override points at a stale/unknown model** → save-time
  validation (422) plus the existing spawn-time 400 fallback drops the
  override rather than blocking the job.

## Acceptance criteria

1. Closing a change whose task files list removed/changed files produces
   a job whose prompt contains both sections and whose
   snapshot/verify/restore covers the ancestors.
2. A learning citing a nonexistent path yields a warning finding in the
   bell and `lessmess validate`; `POST /docs/refresh` includes its dir
   in a manual job whose prompt names the flagged reference.
3. `docs.gardenerModel` set (either layer) → the next docs job session
   is created with that model; unset → `session.model`; both unset →
   service default; unknown values are rejected at save time when the
   service is reachable.
4. All gates green; README documents the three behaviors; unset/empty
   configuration reproduces today's behavior.

## Tasks

1. [LRN-00](tasks/00-ancestor-targets.md) — Ancestor targets and DocsJob shape
2. [LRN-01](tasks/01-gardener-prompt-sections.md) — Two-section gardener prompt with removal accounting
3. [LRN-02](tasks/02-ancestor-verify-restore.md) — Snapshot/verify/restore for ancestor targets
4. [LRN-03](tasks/03-reference-lint.md) — Stale-reference lint in internal/docs
5. [LRN-04](tasks/04-refresh-union-lint-context.md) — Refresh union and lint context in manual jobs
6. [LRN-05](tasks/05-gardener-model-setting.md) — docs.gardenerModel setting
7. [LRN-06](tasks/06-docs-and-verification.md) — README, package docs, and full verification
8. [LRN-07](tasks/07-server-seed-endpoint.md) — Resumable seed on the normal server
9. [LRN-08](tasks/08-exclusions-editor.md) — Exclusions editor and bell seed button
10. [LRN-09](tasks/09-seed-exclusions-verification.md) — README and verification for seed/exclusions
