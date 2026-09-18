# Ledger — 2026-09-18-f52mn

- Change ID: 2026-09-18-f52mn
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Planned
- Last updated: 2026-09-18

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
| [WCV-02](tasks/02-warning-prompts.md) | Validate warning and prompt precedence | Cancelled | — | 2026-09-18 | Nothing persisted to go stale; precedence → WCV-01, `workflow print` → WCV-00 |
| [WCV-00](tasks/00-canon-stamp.md) | Canon as single source: stamp, print command, init stops writing | Cancelled | — | 2026-09-18 | Reshaped by the injection pivot; drift test deleted |
| [WCV-01](tasks/01-prime-injection.md) | Prime injection with precedence line | Cancelled | WCV-00 | 2026-09-18 | Replaces startup auto-sync + hash guard |
| [WCV-03](tasks/03-root-row-sync.md) | Root-row reconciliation in status sync | Cancelled | — | 2026-09-18 | Independent; the tevr-core violation-3 blind spot |
| [WCV-04](tasks/04-verification.md) | Repo remediation, README, end-to-end verification | Cancelled | WCV-00, WCV-01, WCV-03 | 2026-09-18 | Strips AGENTS.md only after injection lands; tevr-core is the acceptance environment |

## Decision log

- 2026-09-18: Auto-sync chosen (user) over warn+explicit and sync-once-then-warn — the tool owns the workflow text because it enforces the rules that text describes; git makes syncs reviewable and revertable. Superseded the same day by the injection pivot below.
- 2026-09-18: Hash-based customization guard (`.lessmess/workflow.json`) rather than canon history; amber warning, never a red violation, for stale/divergent text. Dropped with the pivot — no persisted text remains to guard.
- 2026-09-18: Injection pivot (user): workflow rules are delivered by injecting the stamped canon into every board-spawned session's prime; repositories are never written for delivery (zero-touch — lessmess runs on any project without modifying existing files, AGENTS.md included). Accepted residues: stray non-spawned sessions act ruleless until the validator/board flags them (detectable, unlike today's silent staleness); legacy texts in old repos persist as inert documentation, neutralized by a precedence line in the prime. WCV-02 cancelled (warnings moot; precedence → WCV-01; `workflow print` → WCV-00).
- 2026-09-18: Root-row reconciliation (tevr-core violation 3) folded into this change rather than a separate one — same incident, same two-ledger invariant.
- 2026-09-18: Scaffolded via the public POST /changes API because the planning session that designed this change is bound to 2026-09-16-le45q; no session is bound to this change yet.
