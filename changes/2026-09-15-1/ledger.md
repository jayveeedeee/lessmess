# Ledger — 2026-09-15-1

- Change ID: 2026-09-15-1
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-15

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
| [CID-00](tasks/00-workflow-text.md) | Update the workflow spec text | Done | — | 2026-09-15 | Naming rules, structure tree, validation item rewritten; asset regenerated; `go test ./internal/docs/...` ok, leftover-grep clean |
| [CID-02](tasks/02-index-ordering.md) | Server newest-first by ledger position | Done | CID-01 | 2026-09-15 | Index sorts date desc then ledger position desc; `go test ./internal/server/...` ok incl. reworked `TestIndexNewestFirst` |
| [CID-01](tasks/01-store-random-suffix.md) | Store mint random suffixes and accept both formats | Done | CID-00 | 2026-09-15 | Union regex, randSuffix seam, bounded mint retry; full `go test ./...` ok; rebuilt binary validated a scratch repo with a random-ID change (OK) plus this mixed-format repo |
| [CID-03](tasks/03-learnings-and-verification.md) | Learnings upkeep and full verification | Done | CID-00, CID-01, CID-02 | 2026-09-15 | Store/server learnings updated citing 2026-09-15-1; `go vet` clean; `go test ./...` all ok; rebuilt binary; `./lessmess validate` OK; README needs no change (no naming mentions) |

## Decision log

- 2026-09-15: Keep the `YYYY-MM-DD-` date prefix; only the suffix changes
  (user request targeted the counter at the end of the ID).
- 2026-09-15: Suffix alphabet is full `[a-z0-9]`, five chars, `crypto/rand`;
  lookalike characters are not excluded because IDs are machine-generated
  and click-through (user accepted the recommendation).
- 2026-09-15: Legacy numeric IDs stay valid via one union regex — no
  migration or renaming of existing directories.
- 2026-09-15: Within-date newest-first ordering switches from the numeric
  counter to root-ledger row position (append order); reproduces today's
  order on legacy repos.
- 2026-09-15: Flipping the change to In progress via direct file edits
  initially left the root-ledger row at Planned; rule 6 (surfaced by the
  scratch-repo validation) caught it and the row was updated to agree.

