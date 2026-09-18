# Ledger — 2026-09-16-le45q

- Change ID: 2026-09-16-le45q
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: In progress
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
| [NTD-00](tasks/00-workflow-spec.md) | Extend workflow spec | Done | — | 2026-09-16 | Drift test green; full suite green; validate clean after rebuild |
| [NTD-04](tasks/04-server-board-api.md) | Server nested routes, expand, drill-down board | Test | NTD-03 | 2026-09-17 | Reopened+fixed: AllTaskStats now counts all nodes (roots included) so the header pill and index x/y work on flat changes; pill hidden at zero tasks; suite green, live board verified (7/8, 6/6) |
| [NTD-07](tasks/07-readme-verification.md) | README and end-to-end verification | Done | NTD-00, NTD-01, NTD-02, NTD-03, NTD-04, NTD-05, NTD-06 | 2026-09-16 | README updated; full suite + drift + validate green; live-binary smoke: expand→subtasks→drill-down→move→422/200 close gate |
| [NTD-06](tasks/06-web-ui.md) | Web UI badges, drill-down, modal links | Done | NTD-04, NTD-05 | 2026-09-16 | Badges/crumbs/expand/parent form/data-doc modal links/markup tests green; user browser pass pending in Test |
| [NTD-05](tasks/05-sessions-autospawn.md) | Sessions dotted binding, task prompt, auto-spawn | Test | NTD-04 | 2026-09-18 | Reopened+done: task sessions now PLAN ONLY on arrival (subtasks Not started, own status untouched, execution only on explicit instruction); changePrompt + AGENTS.md rule 7 match; prompt test pins the new rules; suite green, rebuilt+restarted |
| [NTD-03](tasks/03-store-validation.md) | Store recursive validation and close-out readiness | Done | NTD-02 | 2026-09-16 | Tree-walk rule 2/3/5, stray-dir + depth + dotted checks, CloseOutReady; nested violation tests green; validate clean |
| [NTD-02](tasks/02-store-task-tree.md) | Store recursive task tree and operations | Done | NTD-01 | 2026-09-16 | Tree scan, Decompose/CreateTask(parent)/MoveTask, SubtreeStats, watcher recursion — all tested; full suite green |
| [NTD-01](tasks/01-model-container-ledgers.md) | Model container ledgers and dotted IDs | Done | NTD-00 | 2026-09-16 | Shared taskTable core; ParseTaskLedger/RenderTaskLedger; taskid.go helpers; all model + full suite green |

## Notes

- 2026-09-16 NTD-00 done: AGENTS.md gained the "Task decomposition (sub plans)" section (containers, dotted IDs, governing ledgers, user-instructed governance, display-only rollup); root-ledger rule 3 + header sentence reworded to the governing-ledger formulation (starter skeleton in `internal/docs/init.go` and this repo's root ledger updated to match); validation rules 2/3 and close-out step 9 made recursive. Embedded copy regenerated with the documented awk command; drift test, full `go vet`+`go test`, and `lessmess validate` (pre- and post-rebuild) all green.
- 2026-09-16 NTD-01…03: shared `taskTable` mutation core with `ParseTaskLedger`/`RenderTaskLedger`/taskid helpers (`internal/model`); recursive scan + `DecomposeTask`/`CreateTask(parent)`/governing-ledger `MoveTask` + `SubtreeStats` + recursive validation + `CloseOutReady` (`internal/store/tree.go`, `validate.go`), watcher recursion tested.
- 2026-09-16 NTD-04…06: nested task-detail route (`{file...}`), `POST /expand` (201/409), `?task=` drill-down boards with breadcrumbs and display-only progress, recursive index counts, recursive close gate; dotted `taskTitleRe`, `taskPrompt`, once-only auto-spawn via `.lessmess/autosession.json` swept on board render + expand; board UI badges/crumbs/expand/parent-aware add-form, context-aware in-modal links (`data-doc`), per-task session scoping and Continue keying.
- 2026-09-16 NTD-07: README "Nested tasks (sub plans)" section; vet + full suite + drift + validate green, plus a live-binary smoke (expand → subtasks → drill-down → move → 422/200 close gate). All tasks at `Test`, awaiting user acceptance; browser checklist in tasks/06 notes; session behavior verified against fake services, worth one live-service look.
- 2026-09-17 scope change (user direction): the board-header progress pill is removed entirely — card badges and the recursive index counts remain; no header progress UI. NTD-04 briefly reopened for the descendants-only bug, then the feature was withdrawn rather than fixed in place.

## Decision log

- 2026-09-16: Nested containers chosen over flat-with-Parent-column — additive, no ledger migration, recursion by construction (planning discussion).
- 2026-09-16: Rollup is display-only (Test+Done count complete, Cancelled excluded from denominator); statuses stay manual and Done stays user-gated.
- 2026-09-16: Re-parenting out of scope for v1; collapse = delete the container directory.
- 2026-09-16: Auto-spawn one session per sub plan (user choice) — best-effort, once-only via `.lessmess/autosession.json`, manual Start as retry.
- 2026-09-16: Decomposition is user-instructed only (user choice) — board action, explicit instruction, or approved plan; agents propose, never act.
- 2026-09-17: Header progress pill withdrawn (user direction) — rollup display remains on decomposed cards only; the underlying `SubtreeStats`/`AllTaskStats` stay for the recursive index counts.
- 2026-09-18: Task-session semantics settled (user direction): decomposition is a planning act — the auto-spawned session drafts subtasks (`Not started`) on arrival, never changes its own task's status, and executes nothing until the user explicitly instructs it. Recorded as AGENTS.md decomposition rule 7; `taskPrompt` step 0 + changePrompt step 6 teach it; the earlier dispatch-model question was canceled in favor of this rule alone.

