# Ledger — 2026-09-12-0

- Change ID: 2026-09-12-0
- Plan: [plan.md](plan.md)
- Branch: main
- Overall status: Done
- Last updated: 2026-09-12

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [OCI-00](tasks/00-verify-api-contracts.md) | Verify opencode API contracts | Done | — | 2026-09-12 | Contracts recorded in task notes. Key finding: service PTY connect-token is 403-gated (even via CLI) → terminal pivots to in-process PTY. |
| [OCI-01](tasks/01-opencode-client.md) | opencode client package | Done | OCI-00 | 2026-09-12 | Verified: 9 fixture tests pass; live smoke (discover/create/rename/delete) passes against real service. |
| [OCI-02](tasks/02-proxy-layer.md) | API/SSE/WS proxy layer | Done | OCI-01 | 2026-09-12 | Verified: WS bridge echo, resize, bad-session 400, origin-rejection 403 tests pass; deps creack/pty v1.1.24 + coder/websocket v1.8.15 added. |
| [OCI-03](tasks/03-session-mapping.md) | Session mapping store and endpoints | Done | OCI-01 | 2026-09-12 | Verified: CRUD+persistence, corrupt-file handling, endpoints (list/create/unlink), enrichment fallback, .tasktracker path — all tests pass. |
| [OCI-04](tasks/04-terminal-ui.md) | Terminal UI with xterm.js | Done | OCI-02, OCI-03 | 2026-09-12 | Verified: xterm vendored+pinned; render test covers markup; live probe — session created via endpoint, WS bridge streamed ~393KB of real TUI output; probe artifacts cleaned up. |
| [OCI-05](tasks/05-new-change-session.md) | New change session flow | Done | OCI-04 | 2026-09-12 | Verified: flow tests pass (orchestration, prompt content, HX-Redirect, 503 no-service, 502-keeps-change, 422 no-title); e2e lands in OCI-07. |
| [OCI-06](tasks/06-permissions-config.md) | Permissions config and docs | Done | — | 2026-09-12 | Verified: opencode.json committed; live run without --auto executed shell+file ops with no prompt; README security section written. |
| [OCI-07](tasks/07-dogfood.md) | Dogfood and acceptance verification | Done | OCI-05, OCI-06 | 2026-09-12 | All six acceptance criteria verified with evidence (see task notes); cleanup done; validate OK. |

## Decision log

- 2026-09-12: Embedded terminal (PTY + xterm.js) chosen over a custom chat console (user).
- 2026-09-12: Scaffold-first change creation: tasktracker creates the change, then primes the session (user).
- 2026-09-12: Permissions pre-approved via committed `opencode.json`; `--auto` flag on spawned TUI documented as fallback (user chose config).
- 2026-09-12: Multiple sessions per change; mapping = change ID → session list in `.tasktracker/sessions.json` (user).
- 2026-09-12: Proxy injects Basic auth server-side; service password never reaches the browser.
- 2026-09-12: Target the shared background service; `opencode2 serve` dedicated instance is the documented fallback.
- 2026-09-12: Verified against live system: discovery (`service status`), auth (Basic `opencode` + `service.json` password), `--session` resume flag, SSE `/api/event`.
- 2026-09-12: PTY pivot (OCI-00): service PTY `connect-token` 403s for all request variants including the official CLI (gated, likely Console-only). Terminal layer now spawns `opencode2 --session <id>` in an in-process PTY (creack/pty) bridged to our own WebSocket; service API used for plain REST only. Also verified: agent turns block on permission prompts without `--auto`/pre-approved config.
- 2026-09-12: OCI-02 scope simplified: the browser-facing REST proxy and SSE pass-through are dropped — all service calls (session create/rename/prompt/enrichment) happen server-side via the opencode client, so the browser only needs the terminal WS bridge and mapping endpoints. Smaller surface, same capability.
- 2026-09-12: New deps for the terminal bridge: `github.com/creack/pty` v1.1.24 (PTY spawn/resize), `github.com/coder/websocket` v1.8.15 (WS bridge, MIT).
- 2026-09-12: Task-ID prefix for this change registered as `OCI`.
