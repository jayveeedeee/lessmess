# 2026-09-18-f52mn: Workflow canon versioning and sync

- Change ID: 2026-09-18-f52mn
- Created: 2026-09-18
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

The workflow text in a repository's root `AGENTS.md` (everything above the `tasktracker:begin` markers) is per-repo state written once by `lessmess init` and never updated, while the rules the binary enforces evolve with every build. Serving a new binary to an old repository leaves agents reading stale rules — observed in tevr-core (`2026-09-18-m23jf`): an agent invented a non-conforming sub-plan ledger header and hand-edited an overall status, producing rule-5 and rule-6 violations. Separately, the source repo's own `AGENTS.md` was reverted while code and embedded asset kept the newer text, breaking the drift test — the same disease at home.

This change makes the workflow rules a versioned canon owned by the binary and delivers them where agents actually receive instructions: injected directly into every board-spawned session's prime. Repositories are never written for delivery — existing files, `AGENTS.md` included, are no longer the tool's responsibility. (Pivoted 2026-09-18 from an earlier design that kept the text in `AGENTS.md` and auto-synced it at startup; see the ledger's decision log.)

## Current behavior

- The canon is embedded at `internal/docs/assets/workflow_agents.md`, drift-pinned to the source repo's `AGENTS.md` only.
- `lessmess init` writes the text merge-safely at bootstrap; nothing ever updates it afterwards; consumer repositories keep whatever text they bootstrapped with, forever.
- The three session primes (`discussionPrompt`, `changePrompt`, `taskPrompt`) and `handoffAddendum` announce "the change-management workflow defined in AGENTS.md" and cite its rules by number — they delegate rule authority to a file the tool does not keep current.
- `store.syncOverall` reconciles change-ledger ↔ derived status but returns early when they agree, never comparing the root row: a hand edit that picks the correct overall value but skips the root row leaves permanent rule-6 violations (tevr-core violation 3).

## Target behavior

| Piece | Behavior |
| --- | --- |
| Versioned canon | The embedded workflow text carries a version stamp (`<!-- tasktracker:workflow vN -->` in its header) and is the single source: no `AGENTS.md` mirror, no drift test. |
| Prime injection | Every discussion, change, and task session prime embeds the stamped canon followed by a precedence line: these injected rules are authoritative for change management; any workflow text found in repository files (e.g. a legacy `AGENTS.md` section) is inert. |
| Zero-touch delivery | Nothing is written to any repository file for delivery: `init` stops writing the workflow text (it still creates the marker append target), and `serve`/`validate` gain no sync hooks. |
| Retrieval | `lessmess workflow print` emits the canon to stdout, so humans and sessions the tool did not spawn can read the current rules on demand. |
| Observability | The canon version appears in each prime and is logged at spawn ("session primed with workflow vN"). |
| Root-row reconciliation | `syncOverall` also compares the root-ledger row with the change ledger's overall status and rewrites both via `SetChangeStatus` when they disagree — closing the blind spot that let tevr-core's violation persist. |

## Scope

- `internal/docs`: stamp, versioned canon accessor, `init` stops writing the workflow text.
- `cmd/lessmess`: `workflow print` subcommand.
- `internal/server`: prompt builders embed canon + precedence; spawn-path version logging.
- `internal/store`: root-row reconciliation in the overall-status sync.
- Source repo remediation: strip the rulebook from this repository's root `AGENTS.md` (after injection is live).
- `README.md`.

## Non-goals

- Any repository writes for delivery (zero-touch): no startup sync, no banners, no `.lessmess/workflow.json`, no validate warnings about workflow text.
- Cleaning legacy workflow text out of already-onboarded repos (tevr-core): it stays as inert documentation, neutralized by the precedence line.
- Gardener, explorer, and repo-commit prompts: self-contained, not workflow-bearing.
- Reaching sessions the tool did not spawn — accepted gap; mitigations are enforcement (validator/board flag breaches reviewably) and `workflow print` documented in the README.
- Fixing tevr-core's data (its two container headers); header normalization there is follow-up work in that repo (see acceptance criteria).

## Design decisions

1. **Session injection over file sync (user pivot, 2026-09-18)** — supersedes the same-day auto-sync + hash-guard design: persisting rules per-repo is what creates drift; delivering per-session cannot go stale, writes nothing, and deletes the entire sync/guard/warning apparatus. Zero-touch makes lessmess runnable on any project without modifying existing agent docs.
2. **Hash guard dropped with the pivot** — there is no persisted text left to guard. Recorded residues: (a) a stray, non-spawned session in a fresh repo acts without rules until the validator or board flags it — a detectable, correctable failure, strictly better than today's silent staleness; (b) legacy texts persist in old repos and become inert via the precedence line.
3. **Precedence line over cleanup** — one line in the prime is cheaper and safer than migrating legacy files.
4. **Root-row reconciliation folded in** (unchanged) — same incident, same invariant (two ledgers agree), one place to fix.
5. **Enforcement/onboarding separation** — the validator was always the real rulebook; the text is agent onboarding. Losing onboarding coverage for stray sessions costs a reviewable violation, not a silent breach.

## Detailed implementation approach

1. WCV-00 makes the canon single-source: stamp the asset, delete the drift test, stop `init` from writing workflow text, add `lessmess workflow print`.
2. WCV-01 embeds the stamped canon (with the precedence line) in the three prompt builders, rewording their "per AGENTS.md rule N" citations to point at the injected canon, and logs the version at spawn.
3. WCV-02 is cancelled — stale-text warnings are moot with no persisted text; prompt precedence moved into WCV-01; `workflow print` moved into WCV-00.
4. WCV-03 extends `syncOverall` (independent, can land in parallel).
5. WCV-04 strips this repository's root `AGENTS.md` rulebook (only after WCV-01 is live so sessions stay self-sufficient), updates the README, and verifies end-to-end with tevr-core as the acceptance environment.

## File-level impact

- `internal/docs/assets/workflow_agents.md` (stamp + delivery wording), `internal/docs/init.go` (versioned accessor; created root `AGENTS.md` becomes marker section only), `internal/docs/init_test.go` (drift test removed; init tests assert no workflow text).
- `cmd/lessmess/main.go` (`workflow print`).
- `internal/server/changesession.go` (three primes + `handoffAddendum` rewording, canon section, version log); no startup hooks anywhere.
- `internal/store/overall.go`, `internal/store/overall_test.go`.
- `AGENTS.md` (rulebook stripped; markers + learnings remain), `README.md`.

## Data, schema, and configuration changes

- None. No new state files (the previously planned `.lessmess/workflow.json` is never created); no settings changes; no canonical-data format changes beyond the stamp comment inside the asset.

## Safety, migration, and rollback

- No repository writes at all, so there is nothing per-repo to roll back. A bad canon is fixed by rebuilding; sessions primed with an older text keep it until they end (same semantics as a stale file read today). Legacy texts in old repos are inert, so cleanup is never urgent.

## Testing and verification strategy

- Unit tests: stamp/version accessor; `init` writes no workflow text and still creates the marker append target; every prime embeds canon + precedence (discussion, change, handoff, task); `syncOverall` heals root-row-only drift (the tevr-core reproduction).
- Source repo: nothing pins `AGENTS.md` to the asset; `lessmess validate` stays clean after the strip (learnings intact).
- Acceptance: a spawned change session's prime carries the current canon version; tevr-core served by the new build — its next agent-created container ledgers conform without violations, its legacy text untouched.

## Rollout sequence

Land WCV-00, then WCV-01 (agents are self-sufficient from here), WCV-03 in parallel, WCV-04 last (strip `AGENTS.md` only after injection is live); rebuild; serve tevr-core last.

## Risks and mitigations

- **Stray sessions without rules** (fresh repos, non-board harnesses) — accepted gap; the validator and board flag breaches reviewably, and `workflow print` is documented in the README for on-demand retrieval.
- **Legacy text confusion** — the precedence line makes injected rules win; old copies become inert documentation.
- **Prime size** — the canon is a few KB per session; trivial.
- **Mid-session rule changes** — existing sessions keep their primed text, identical to today's file-read semantics; new sessions always get current rules.

## Acceptance criteria

1. The embedded canon carries the version stamp; `lessmess workflow print` emits it; no test or code path pins it to `AGENTS.md`.
2. `init` creates a root `AGENTS.md` without workflow text (marker append target only) and never modifies existing agent docs.
3. New discussion/change/task sessions are primed with the stamped canon and the precedence line; spawn logs the version; `spawn-change` primes include it.
4. Root-row-only drift heals on the next status sync; covered by a test reproducing the tevr-core violation.
5. This repository's root `AGENTS.md` no longer carries the rulebook; README documents the delivery model; `go vet ./... && go test ./...` green.
6. tevr-core (real repo): a spawned session carries the current rules; subsequent agent-written sub-plan ledgers conform without violations; its legacy text remains untouched and inert (verified from that repo's side).

## Ordered task breakdown

1. [WCV-00 — Canon as single source: stamp, print command, init stops writing](tasks/00-canon-stamp.md)
2. [WCV-01 — Prime injection with precedence line](tasks/01-prime-injection.md)
3. [WCV-02 — Cancelled (scope absorbed)](tasks/02-warning-prompts.md)
4. [WCV-03 — Root-row reconciliation in status sync](tasks/03-root-row-sync.md)
5. [WCV-04 — Repo remediation, README, end-to-end verification](tasks/04-verification.md)

Execution status lives in [ledger.md](ledger.md).
