# Ledger — 2026-09-13-4

- Change ID: 2026-09-13-4
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
| [ONB-00](tasks/00-onboarding-state-file.md) | Onboarding state file helpers | Done | — | 2026-09-14 | Verified: roundtrip/malformed-fail-open/pending-transition tests pass; vet clean. |
| [ONB-01](tasks/01-setup-mode-server.md) | Setup-mode server shell with hot-open | Done | ONB-00 | 2026-09-14 | Verified: sentinel+guard+swap-once tests pass; partial-tree case (changes/ without root ledger, found in user testing on truendo2) now triggers setup mode while a corrupt ledger stays fatal; live E2E on both scenarios green. |
| [ONB-07](tasks/07-readme-and-verification.md) | README and full verification | Done | ONB-05, ONB-06 | 2026-09-14 | Verified: README documents first-run + setup API; vet/test/build green; validate exit 0; live E2E matrix executed (evidence in Notes). |
| [ONB-06](tasks/06-onboarding-banner.md) | Index banner and Settings re-entry | Done | ONB-00, ONB-05 | 2026-09-14 | Verified: render test (pending/dismissed/completed) passes; live on this repo: banner shows, dismiss persists, Settings link + /setup reachable. |
| [ONB-05](tasks/05-wizard-ui.md) | Wizard UI (template, client flow, styles) | Done | ONB-02, ONB-03, ONB-04 | 2026-09-14 | Verified: render tests (incl. chrome-absence) + JS checks pass; wizard E2E green; agent/model lists populate; picker is a lazy expandable tree with implied-descendant display; wizard is a centered card with underline step highlights and no app chrome. |
| [ONB-04](tasks/04-docs-seed-endpoint.md) | Server-side docs seed honoring agent/model | Done | ONB-01 | 2026-09-14 | Verified: summarizer passes agent/model (first-slash split), endpoint 503/409/run/status tests pass; CLI docs seed now uses SessionDefaults; full suite green. |
| [ONB-03](tasks/03-bootstrap-endpoint.md) | Bootstrap endpoint with docs-coverage option | Done | ONB-01 | 2026-09-14 | Verified: full-loop + partial-tree + exclusion tests pass; excludeDirs accepts base names and nested paths (422 on globs/escapes), redundant ancestors normalized, existing configs updated with hand-authored patterns preserved; GET /api/setup/dirs serves lazy children with per-dir excluded state. |
| [ONB-02](tasks/02-prereq-checks-endpoint.md) | Prerequisite checks endpoint | Done | ONB-01 | 2026-09-14 | Verified: faked ok/warn/fail + staged service-failure tests pass; changes-present now distinguishes present / partial (no root ledger) / absent; live run against truendo2 reports ready:true with the partial-tree warning. |

## Notes

- Gates (2026-09-13): `go vet ./...`, `go test ./...`, and `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess` all green; `lessmess validate` exit 0 (the stale-STRUCTURE.md warnings reconcile when this change closes).
- Live E2E on empty dirs: setup mode serves the wizard (all normal routes 503/redirect pre-bootstrap); prereqs report real binary/service/git/writable state; bootstrap creates all artifacts and hot-opens in place (index answers immediately, no restart); settings save works in setup mode; docs skip = zero sessions; completion persists to `.lessmess/onboarding.json`; restart goes straight to the normal UI; the banner on this repo shows once and dismiss persists; Settings re-entry link and `/setup` on the normal server both work.
- Live docs-seed run (budget 1, real opencode service): endpoint, async job, and status polling all work. The root dir failed seed confinement ("AGENTS.md: content outside the tasktracker markers changed") and rolled back byte-identical, staying pending/resumable.
- Finding (pre-existing, not introduced here): `lessmess docs seed` on a freshly `lessmess init`'d repo hits the same root-dir confinement failure with the service-default model — the bootstrapped root AGENTS.md is a large marker-less file the model rewrites despite the append-only rule. Follow-up candidate (out of scope): have `init` write an empty marker section into the root AGENTS.md so the seed pass has an explicit append target.
- Remaining for user acceptance: clicking through the wizard in a real browser (all APIs it calls are verified); everything else is evidenced above.

## Decision log

- 2026-09-13 — User decision: the wizard lives in the web UI via a setup-mode server; the CLI stays non-interactive.
- 2026-09-13 — User decision: onboarding appears once per repo, persisted in `.lessmess/onboarding.json`, re-openable from Settings; absent file means incomplete.
- 2026-09-13 — User decision: initial docs generation is an explicit opt-in wizard step (default skip); skipping must mean zero LLM sessions.
- 2026-09-13 — User decision: a missing/unreachable opencode service is handled by detect + instructions + re-check; the wizard never auto-starts it.
- 2026-09-13 — Architecture: setup mode is a separate small mux (never a nil-store `Server`); hot-open runs the existing serve wiring once via a `main.go`-owned boot closure and an atomic handler swap; one route registrar is shared by the setup mux and the normal `Server` mux so `/setup` and `/api/setup/*` work in both worlds.
- 2026-09-13 — Agent/model defaults save to the personal layer by default (machine-specific availability); project layer selectable in the wizard.
- 2026-09-13 — Folded-in bug fix: seed sessions ignored configured agent/model (`OpenCodeSummarizer` used plain `CreateSession`); fixed via `SessionClient.CreateSessionWith` + `SessionDefaults`, server-side and CLI, without a settings-package refactor and without a 400-retry in the seed path.
- 2026-09-13 — Partial trees count as uninitialized: `changes/` existing without a root ledger triggers setup mode (bootstrap creates the ledger merge-safely); only a ledger that exists but fails to parse stays a hard startup error. Found via user testing against truendo2.
- 2026-09-13 — Agent/model option lists merge the location-scoped service list with the default-location list (deduped, scoped wins): the opencode service omits built-in primary agents from scoped lists outside its home location but accepts them for sessions anywhere (verified empirically). Applies to the wizard and the Settings page; save-time validation uses the same merged list so built-ins pass and typos still 422.
- 2026-09-13 — User-requested additions during acceptance testing: the bootstrap step gained a docs-exclusion picker (top-level dirs, built-in excludes pre-checked/disabled; chosen names written as base-name patterns, which prune whole subtrees and match same-named dirs at any depth — same semantics as DefaultExclude), and the wizard renders as a centered card.
- 2026-09-13 — Exclusions also UPDATE an existing `agentsdocs.json`: an explicit bootstrap submission replaces base-name patterns with the picker's selection while preserving hand-authored path/glob patterns (e.g. "web/static"), so already-bootstrapped repos can change exclusions from the wizard; init itself still never modifies an existing config.
- 2026-09-13 — Nested exclusions: the picker is a lazy tree (GET /api/setup/dirs?dir=); top-level picks stay base-name patterns, nested picks become slash paths; redundant ancestors are normalized away; merge preserves patterns the picker cannot represent (globs or paths with no matching dir on disk).
- 2026-09-13 — Onboarding chrome: the setup page renders without the app navs, docs bell, or violations banner (layout conditional on Page == setup), and the step highlights mirror the topnav style (weight + accent underline), not pills.
