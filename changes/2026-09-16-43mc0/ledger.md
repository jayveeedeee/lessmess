# Ledger — 2026-09-16-43mc0

- Change ID: 2026-09-16-43mc0
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: In progress
- Last updated: 2026-09-16

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
| [LRN-00](tasks/00-convention.md) | Curated-learnings convention in workflow text, prompts, and README | Test | — | 2026-09-16 | Drift test green; vet+full server/docs tests green; no old wording left in prompts or README |
| [LRN-01](tasks/01-cleanup.md) | One-time curation of all AGENTS.md auto sections | Test | LRN-00 | 2026-09-16 | All 14 sections rewritten (≈244 → ≤15 each); vet+tests green; rebuilt `lessmess validate` clean after fixing two self-inflicted lint flags |

## Decision log

- 2026-09-16 — Drop all provenance prefixes (`(change-id)`, `(seed)`, `(manual)`); attribution moves to git blame. Chosen over demoting prefixes to trailing tags: they encouraged changelog framing and were redundant with blame.
- 2026-09-16 — Section cap set to 15 learnings per file, enforced by prompt wording only; a `ValidateDocs` over-cap warning is left as a possible follow-up.
- 2026-09-16 — The one-time cleanup is done directly by the change session (reviewable per-file diff) rather than via gardener sessions, per the user's request that the curation be reviewed.
