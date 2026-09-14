---
id: ONB-02
title: Prerequisite checks endpoint
---

# ONB-02: Prerequisite checks endpoint

Status: see [../ledger.md](../ledger.md).

## Objective

`GET /api/setup/prereqs` reports the health of everything the wizard depends on, with remediation hints, so the UI can gate agent-dependent steps and offer a Re-check button (user decision: detect + instructions, never auto-start the service).

## Dependencies

- ONB-01 (route registrar and setup server exist)

## Scope

- New `internal/server/prereqs.go` (+test): check implementations and the endpoint handler.
- Checks (each `{id, name, status: ok|warn|fail, detail, remedy}`):
  1. `opencode2-binary` — `exec.LookPath("opencode2")`; fail blocks (needed for service discovery and terminal PTY).
  2. `opencode-service` — staged: `opencode.Discover` → `PasswordFromFile(DefaultPasswordPath())` → `Healthy`; fail reports which stage broke, with remedy text (start the service / check `~/.config/opencode/service.json`).
  3. `git-binary` and `git-repo` — warn-level only (commit features degrade gracefully already).
  4. `repo-writable` — create+remove a temp file under the repo dir; fail blocks bootstrap.
  5. `changes-present` — informational: drives which wizard step the UI resumes at.
- Response `{checks: [...], ready: bool}`; `ready` = no `fail` check.
- On a successful service check in setup mode, the discovered client is stored on the setup server so the settings options/validation endpoints can use it (normal mode uses `s.oc`).

## Implementation steps

1. Define the check result types and the ordered check list with injectable function vars (`lookPath`, `discoverClient`, `gitProbe`) following the `SpawnCommand` override pattern.
2. Implement each check with concise `detail` (what was found) and `remedy` (copy-pasteable fix) text.
3. Implement the handler on the shared registrar, updating the setup server's opencode accessor on success.
4. Tests: each check forced to ok/warn/fail via fakes; `ready` aggregation; endpoint JSON shape; client swap on recovery.

## Verification

- `go test ./internal/server -run Prereq -v` passes; `go vet ./internal/server` clean.
- Manual: with the service stopped vs running, the endpoint flips the `opencode-service` check and `ready` accordingly.

## Completion criteria

- All five checks report accurately across faked and live conditions; failing gates set `ready:false`; a recovered service is picked up without restart.

## Files affected

- `internal/server/prereqs.go` (new), `internal/server/prereqs_test.go` (new)
- `internal/server/setup.go` (route + client accessor wiring)

## Notes

- Keep remedy text generic (no assumed package manager); e.g. "install opencode2 and ensure it is on PATH", "start the opencode background service (opencode2 service …) and re-check".
- Do not probe network or accounts beyond the local service; the check must complete in well under the 10s timeout used elsewhere.
