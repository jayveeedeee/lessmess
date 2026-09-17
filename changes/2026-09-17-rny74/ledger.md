# Ledger — 2026-09-17-rny74

- Change ID: 2026-09-17-rny74
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-17

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
| [SPF-00](tasks/00-spawn-fallback-ladder.md) | Agent-preserving spawn fallback ladder | Done | — | 2026-09-17 | Ladder agent+model→agent→model→plain; evidence: 5 new/updated tests green (`go vet ./... && go test ./...` OK) — model-rejected keeps agent, agent-rejected keeps model, non-400 stops, full-fail errors |
| [SPF-01](tasks/01-fallback-record-banner.md) | Persisted fallback record and board warning banner | Done | SPF-00 | 2026-09-17 | Evidence: record lifecycle (write on fallback, clear on clean), fail-open read, banner render/clean-absence tests green; full suite OK |
| [SPF-02](tasks/02-default-agent-notice.md) | default_agent divergence notice on the Settings page | Done | — | 2026-09-17 | Evidence: reader table (ok/absent/no-key/JSONC), response surfacing, page hook assertions green; full suite OK |
| [SPF-03](tasks/03-align-endpoint.md) | Align endpoint and button for opencode.json default_agent | Done | SPF-02 | 2026-09-17 | Evidence: patcher golden (order/bytes preserved, JSONC refused), align e2e (200/422/503, file untouched on refusal), UI hooks; `go vet ./... && go test ./...` + build + `lessmess validate` all OK |

Task dependencies: SPF-01 needs SPF-00's outcome steps; SPF-03 needs SPF-02's
notice and reader; SPF-00 and SPF-02 are independent roots.

## Decision log

- 2026-09-17 — Scope chosen with the user (option B): fallback hardening +
  visible fallback warning + divergence notice with an explicit align
  button. Silent write-through of `default_agent` (option C) was rejected
  for git-tree side effects and layering violations; pure config fix
  (option D) deferred to the user for tevr-core.
- 2026-09-17 — Ladder order is agent-first (agent-only before model-only
  before plain): the agent is identity/behavior, the model is cost/quality;
  plain create remains the last resort so a bad setting never blocks
  spawning.
- 2026-09-17 — Fallback visibility uses one new state file
  (`.lessmess/spawn-fallback.json`) rendered server-side in the index view,
  following the one-file-per-feature `.lessmess/` convention; no dismissal
  state — the record clears on the next clean spawn.
- 2026-09-17 — The align action is user-consented per click and validates
  the agent against the live service (422/503) before writing; the patcher
  preserves key order and refuses JSONC rather than risk corruption.
