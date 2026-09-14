# Ledger — 2026-09-13-5

- Change ID: 2026-09-13-5
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-14

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Test | Implementation and verification are complete; awaiting user acceptance before Done. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [LRN-00](tasks/00-ancestor-targets.md) | Ancestor targets and DocsJob shape | Done | — | 2026-09-14 | Verified: ancestor table tests + queue roundtrip pass; close job carries Ancestors=["internal"]; stale/reconcile paths cover ancestors. |
| [LRN-01](tasks/01-gardener-prompt-sections.md) | Two-section gardener prompt with removal accounting | Done | LRN-00 | 2026-09-14 | Verified: prompt table tests (review section, deletion license, accounting, manual unchanged, no-ancestor case) pass. |
| [LRN-09](tasks/09-seed-exclusions-verification.md) | README and verification for seed/exclusions | Done | LRN-07, LRN-08 | 2026-09-14 | Verified: README documents missing-file targeting, force, nested exclusions; live matrix green (partial/missing/force/exclusion + UI hooks). |
| [LRN-05](tasks/05-gardener-model-setting.md) | docs.gardenerModel setting | Done | — | 2026-09-14 | Verified: precedence tests; spawn override + fallback via fake service; PUT 422 names field; clear restores inheritance; Settings row renders. |
| [LRN-07](tasks/07-server-seed-endpoint.md) | Resumable seed on the normal server | Done | — | 2026-09-14 | Verified (post-feedback): file-based missing-docs targeting (cursor ignored for missing dirs), force mode redoes all, plain re-run returns the use-force hint; live E2E confirms all three. |
| [LRN-08](tasks/08-exclusions-editor.md) | Exclusions editor and bell seed button | Done | LRN-07 | 2026-09-14 | Verified (post-feedback): Settings exclusions widget is the wizard's lazy nested tree (same endpoint/rows); bell shows Run missing docs (N) + Force checkbox with confirm; nested POST prunes subtrees (live). |
| [LRN-06](tasks/06-docs-and-verification.md) | README, package docs, and full verification | Done | LRN-01, LRN-02, LRN-04, LRN-05 | 2026-09-14 | Verified: gates green (vet/test/build, validate exit 0); live E2E on scratch repo — lint flag → refresh job → gardener removed dead ref → finding cleared; override model logged and used. |
| [LRN-02](tasks/02-ancestor-verify-restore.md) | Snapshot/verify/restore for ancestor targets | Done | LRN-00, LRN-01 | 2026-09-14 | Verified: ancestor out-of-marker violation restores + fails job; in-marker learning deletion persists; existing cage tests green. |
| [LRN-03](tasks/03-reference-lint.md) | Stale-reference lint in internal/docs | Done | — | 2026-09-14 | Verified (post-feedback): sibling-variant resolution (`truservice.yaml` ↔ `truservice.yaml.template`) + deduplicated findings; truendo2 validate read-only dropped ~67 warnings to the 7 legitimately-stale ones. |
| [LRN-04](tasks/04-refresh-union-lint-context.md) | Refresh union and lint context in manual jobs | Done | LRN-03 | 2026-09-14 | Verified (post-feedback): refresh union chunks into ≤10-dir gardener jobs (unit + 13-dir live-shape test); one fragile giant session can no longer flag everything stale. |

## Notes

- Gates (2026-09-13): `go vet ./...`, `go test ./...` (all packages), `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess` green; `lessmess validate` exit 0 (remaining warnings: stale STRUCTURE.md freshness, reconciled at close-out; 2 lint findings for literally-missing state files, correct by design).
- Live E2E on a scratch repo (2026-09-13, real opencode service): `lessmess init` + `serve` on :18080 → `docs.gardenerModel` visible in `/api/settings` (source `default`); PUT valid live model → 200 saved to personal layer, effective shows it; PUT `prov/definitely-not-real` → 422 `gardener unknown model …`; AGENTS.md citing `gone/deleted.go` → `/api/validate` warns `learning cites missing path`; `POST /docs/refresh` → 202 enqueues `[".", "internal", "internal/model"]`; serve log shows `gardener session model model=fireworks-ai/accounts/fireworks/models/deepseek-v4p1-flash`; job done in ~160 s; the gardener removed the dead learning (prompt had named it) and the finding cleared. Scratch rig deleted afterward.
- Not exercised live: change-close ancestor flow (fires when this change closes — its own tasks list `internal/server`, `internal/docs`, `web/templates`, and root files, so the close-out job will carry those ancestors as review-and-fix targets).
- This repository's running server (port 9090) predates the change: the new endpoints/fields go live there after rebuilding and restarting the binary.
- Precision finding recorded in the decision log: the first live lint run produced 55 mostly-false-positive warnings, which drove the known-extension allowlist, qualifier-character rejection, and generous resolution (suffix/base-name match anywhere, `.lessmess` state included).
- Live E2E round 2 (2026-09-13, post-acceptance-feedback rework, scratch repo): missing count 4 → full seed (app, docs/nested, docs, root all summarized) → plain re-run returns `all covered directories have doc files (use force to redo)` → deleting `app/AGENTS.md` makes missing=1 → run-missing re-runs only `app` → `{"force":true}` re-runs all 4 → nested exclusion `docs` saved and pruned `docs/nested` → force checkbox and exclusions widget present in rendered HTML. Scratch rig deleted afterward.
- truendo2 incident + resolution (2026-09-14, read-only diagnosis): ~60 identical `truservice.yaml` lint warnings (generated-not-committed deploy artifact, template sibling present in every service dir) + 7 stale STRUCTURE.md warnings; refresh clicks amplified warnings because the ~67-dir union ran as one fragile gardener session. Fixed by sibling-variant resolution, finding dedup, and ≤10-dir job chunking. Post-fix `lessmess validate --dir truendo2` (read-only) shows only the 7 stale warnings; one refresh click clears them.

## Decision log

- 2026-09-13 — User decision: combined scope — ancestor review-and-fix gardening (B) plus deterministic stale-reference lint (C); both accepted over lint-only or gardening-only alternatives.
- 2026-09-13 — User decision: new `docs.gardenerModel` setting; precedence gardener override → `session.model` → service default; model only (no agent override); seed sessions out of scope.
- 2026-09-13 — Manual reconciliation jobs do not expand ancestors; the lint routes parent-dir rot into refresh instead (keeps manual scope global-hash-driven).
- 2026-09-13 — Lint v1 detects backticked path-like references only, with conservative resolution (repo-rel, dir-rel, walked entries, own STRUCTURE.md rows); behavioral staleness and hidden-segment paths are out of scope.
- 2026-09-13 — Lint precision rules revised after a live run on this repository (55 findings, nearly all false positives): added a known-extension allowlist as the symbol filter, qualifier-character rejection, and generous resolution (full-path suffix or base-name match anywhere in the tree, `.lessmess` state included). Live re-run leaves only literally-missing files flagged. Overrides the previous resolution bullet above.
- 2026-09-13 — User-requested scope addition during acceptance: resumable seed on the normal server + exclusions editor. Folding into this change (LRN-07..09) because the session is bound to 2026-09-13-5 and the scaffold endpoint refuses a second change per session; a separate change would require a fresh discussion session.
- 2026-09-13 — Seed endpoint reuses the setup wizard's package-level job registry and the same resumable cursor, so wizard, CLI, and server runs all continue one another's pending dirs; no new state file.
- 2026-09-13 — Exclusions editor writes through the wizard's `updateConfigExcludes` semantics (replace picker-representable patterns, preserve hand-authored globs/stale names); v1 covers top-level base names only; the root directory stays always-covered.
- 2026-09-13 — Deletion license for stale learnings is scoped to review-and-fix (ancestor) prompt sections; update-section preserve wording stays verbatim; replies must account for every removal/edit.
- 2026-09-13 — Fixed the documented root-seed confinement blocker as part of this scope: `init` now appends an empty marker section to freshly created — and to marker-less pre-existing — root AGENTS.md files, giving seed/gardener passes an explicit append target. Live proof: a fresh repo now seeds to pending=0 including the root, which previously could never complete (confinement rollback kept it pending forever).
- 2026-09-13 — Acceptance feedback (user): "run missing docs" must be file-existence based — non-excluded dirs lacking doc files run regardless of the cursor — plus a force mode that redoes everything regardless; and the exclusions editor must have full wizard parity (lazy nested tree), not top-level only. Reopened LRN-07/08/09.
- 2026-09-13 — Seed selection model after the feedback: `SeedOptions.Dirs` restricts the run and overrides the cursor for listed dirs (explicit request); `SeedOptions.Force` ignores the cursor for the whole tree. The bell's non-force run targets `docs.MissingDocDirs` (missing STRUCTURE.md or AGENTS.md); the cursor remains the CLI/wizard resume mechanism.
- 2026-09-13 — Lint round-3 rules from the truendo2 incident: references with a sibling variant (`<ref>.*` file, e.g. `truservice.yaml` next to `truservice.yaml.template`) resolve as generated artifacts; identical missing paths across directories collapse into one finding with a count and example dir; manual refresh unions chunk into ≤10-dir gardener jobs so a single fragile session cannot flag everything stale at once.
