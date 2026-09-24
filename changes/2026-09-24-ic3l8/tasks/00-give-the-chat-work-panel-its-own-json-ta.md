# UI-00: Give the chat Work panel its own JSON task feed

## Why

The chat overlay's Work panel (`#chat-tasks`) has no backend of its own. `renderTaskPanel`
(web/static/app.js ~3579) scrapes `.cards[data-status]` / `.card` DOM out of the board's HTML
fragment, fetched by `loadSessionTasks` via `GET /changes/{id}` with an `HX-Request` header. The
kanban board is being removed later in this change, so the Work panel must be decoupled from board
HTML first.

## Current behavior

- `loadSessionTasks(changeID, taskID)` fetches the board fragment and hands the parsed DOM to
  `setChatTasks` → `renderTaskPanel`, which walks columns/cards to build its grouped list.
- `renderSubs` separately scans `#board .card[data-task]` for subagent chip placement.
- `GET /changes/{id}` with `Accept: application/json` returns `boardResponse`, but its `Tasks` is a
  flat root-level `[]model.TaskState` — no subtask rollups (`hasSub`/`subDone`/`subTotal` live only
  on the display-only `cardView`), no detail URLs, no scope support.

## Target behavior

- New JSON endpoint `GET /changes/{id}/tasks` (optional `?task=<container id>`) returning one row
  per task in the requested scope (change root, or one container's children): `id`, `title`,
  `status`, `path`, `hasSub`, `subDone`, `subTotal`, `updated`, and the task-detail href. Rows are
  ordered by seq; the payload also carries the scope's title (change title or container task title)
  and overall status so the panel header can show it.
- `renderTaskPanel` renders groups in `model.TaskStatusOrder` from that payload client-side: status
  group headers with counts, task rows opening the existing `#detail` modal (`hx-get`), subtask
  markers, Plan link footer, and the "All tasks" root button when scoped to a container.
- `loadSessionTasks`, `setChatTasks`, and the SSE refresh path in `scheduleRefresh` switch to the
  JSON feed; no board HTML is fetched anywhere after this task.
- Subagent sessions are NOT rendered from board DOM here — their new home is the Sessions sheet
  (UI-01); this task only stops the board-DOM dependency.

## Verification

- With the chat open on a change-bound session, the network log shows `GET /changes/{id}/tasks`
  JSON and no board fragment request when opening/refreshing Work.
- Work panel lists root tasks grouped by status; opening a decomposed task's subtasks scopes the
  panel and "All tasks" returns to the root; the Plan link opens the plan modal.
- A task status change in the detail modal is reflected in the panel on the next SSE-driven
  refresh.
- `go vet ./... && go test ./...` pass; new handler test covers the endpoint (scope, rollups, 404
  unknown task).
