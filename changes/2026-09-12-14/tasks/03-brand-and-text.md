
# REN-03: Brand and user-facing text sweep

Status: see [../ledger.md](../ledger.md).

## Objective

Every user-facing mention of the app says `lessmess`: UI brand, docs, usage text, and agent-facing prompts.

## Dependencies

- REN-00 (avoid conflicts with the import sweep); land after REN-01/REN-02 so path references are final.

## Scope

- `web/templates/layout.html`: `<title>` suffix and brand link text.
- `web/templates/explorer.html`: `tasktracker init` / `tasktracker docs seed` guidance.
- `web/static/app.js`, `web/static/app.css`: header comments.
- `cmd/lessmess/main.go`: doc comment, usage block, `run 'tasktracker init'` message.
- `internal/server/changesession.go`, `docssession.go`, `explorer.go`: prompt strings and comments (`changePrompt`, `discussionPrompt`, `gardenerPrompt`, `explorerPrompt`, `apiBase` comment) — only app-name references; marker-syntax strings (`tasktracker:begin/end`) stay.
- `internal/server/render_test.go:48`: brand assertion `"tasktracker"` → `"lessmess"`.
- `README.md`: full sweep except `tasktracker:begin/end` marker mentions.
- Root `AGENTS.md` workflow text (above the markers only): `.tasktracker/` → `.lessmess/`, "tasktracker maintains…" phrasing; then regenerate the drift asset with the documented awk command.

## Implementation steps

1. Templates and static comments.
2. CLI usage/messages and prompt strings (grep each prompt builder for `tasktracker` and reword app-name mentions only).
3. `README.md` sweep.
4. Root `AGENTS.md` workflow-text edits (outside markers only).
5. Regenerate drift asset: `awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md > internal/docs/assets/workflow_agents.md`.
6. Update `render_test.go` brand assertion; sweep remaining code comments mentioning the app name.

## Verification

- `go test ./...` passes (drift test pins AGENTS.md ↔ workflow_agents.md; render test checks the new brand).
- `grep -rni 'tasktracker'` outside non-goals (markers, `tt-` prefixes, `changes/`, testdata, covered-folder marker sections) returns nothing.

## Completion criteria

- UI, README, usage text, prompts, and workflow text all say lessmess; drift asset regenerated; tests green.

## Files affected

- `web/templates/layout.html`, `web/templates/explorer.html`, `web/static/app.js`, `web/static/app.css`
- `cmd/lessmess/main.go`
- `internal/server/changesession.go`, `internal/server/docssession.go`, `internal/server/explorer.go`, `internal/server/render_test.go`
- `README.md`, `AGENTS.md` (workflow text only), `internal/docs/assets/workflow_agents.md`

## Notes

- Do NOT touch: content inside doc markers (gardener reconciles at close), `changes/` history, `internal/model/testdata/`, marker syntax, `tt-` frontend prefixes.
- The awk command's `tasktracker:begin` pattern is marker syntax and stays as-is.
- Finding: agent prompts (discussion/change/gardener/explorer) had no app-name mentions — only marker syntax; the sole prompt-file edit was the `apiBase` comment.
- Verified 2026-09-12: layout/explorer templates, app.js/app.css comments, usage block, README (except marker-syntax line), AGENTS.md workflow text all say lessmess; drift asset regenerated via documented awk; validate.go finding strings now name `lessmess docs seed/refresh`; render_test brand assertion flips; `go test ./...` green (drift test passes).
