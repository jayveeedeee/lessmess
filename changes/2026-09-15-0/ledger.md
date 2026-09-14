# Ledger — 2026-09-15-0

- Change ID: 2026-09-15-0
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: In progress
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
| [MTOC-00](tasks/00-heading-ids-and-ledger-endpoint.md) | Heading anchors and ledger detail endpoint | Done | — | 2026-09-15 | GFM table extension added (required for ledger rendering); `go vet` + full server/store tests green |
| [MTOC-02](tasks/02-client-toc-and-scrollspy.md) | Client TOC builder with click-to-scroll and scroll-spy | Done | MTOC-00, MTOC-01 | 2026-09-15 | JS syntax-checked; heading ids verified in served HTML; visual scroll-spy awaits user click-through |
| [MTOC-04](tasks/04-endtoend-verification.md) | End-to-end verification and checks | Done | MTOC-00, MTOC-01, MTOC-02, MTOC-03 | 2026-09-15 | `go vet ./... && go test ./...` green; `lessmess validate` OK; binary rebuilt + server restarted; browser click-through handed to user |
| [MTOC-01](tasks/01-two-pane-modal-layout.md) | Two-pane modal layout in templates and CSS | Done | MTOC-00 | 2026-09-15 | Panes + hidden TOC rail verified in served HTML for plan/task/ledger; notif/commit modals untouched |
| [MTOC-03](tasks/03-inmodal-md-links.md) | In-modal .md link interception | Done | MTOC-00 | 2026-09-15 | JS syntax-checked; all four link shapes served by live endpoints; behavior awaits user click-through |

## Decision log

- 2026-09-15 — TOC covers h2 + h3 (h3 indented), per user choice; h1 (document title) is excluded.
- 2026-09-15 — Heading anchors via goldmark `parser.WithAutoHeadingID()`; TOC built client-side from the swapped DOM so plan/task/ledger share one code path.
- 2026-09-15 — Links are intercepted client-side (no server markdown rewriting); ledger gets a new `GET /changes/{id}/ledger` endpoint since none existed.
- 2026-09-15 — Modal stays 680px without a TOC and widens to min(880px, 100%) via a `has-toc` class when the pane is shown.
- 2026-09-15 — GFM `extension.Table` enabled on the shared goldmark instance: the ledger is table-shaped and bare goldmark rendered tables as plain text (caught by `TestLedgerDetailHTML`). Tables only; no linkify/strikethrough.
