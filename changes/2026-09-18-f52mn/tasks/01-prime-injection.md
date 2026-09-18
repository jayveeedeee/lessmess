---
id: WCV-01
title: Prime injection with precedence line
---

# WCV-01: Prime injection with precedence line

Status: see [../ledger.md](../ledger.md).

## Objective

Deliver the workflow canon where agents actually receive instructions: embed the stamped canon in every board-spawned session's prime, with the injected rules declared authoritative over any repository-file workflow text.

## Dependencies

WCV-00

## Scope

- `internal/server/changesession.go` prompt builders (`discussionPrompt`, `changePrompt`, `taskPrompt`, `handoffAddendum`) and version logging on the three spawn paths. Other prompts (gardener, explorer, repo-commit) are self-contained and out of scope.

## Implementation steps

1. Build the canon section once (server construction or lazy init): a heading ("Change-management workflow — canon vN"), the asset text, and the precedence line: these injected rules are authoritative for change management in this repository; if any repository file (for example a legacy workflow section in `AGENTS.md`) disagrees, these rules win.
2. Append the section to `discussionPrompt`, `changePrompt`, and `taskPrompt`; reword their "defined in AGENTS.md" phrasing and the "AGENTS.md rule N" citations (rules 3/4/9) to cite the injected canon.
3. `handoffAddendum`: reword its "AGENTS.md rule 3" citation the same way.
4. Log the version at spawn in all three creation paths (`slog.Info(... "workflow", version)`).
5. Prompt tests: every prime contains the stamped canon and the precedence line.

## Verification

- `go test ./internal/server/` green, including the prompt assertions.
- Manual: create a discussion session and a change session from the board; their primes (visible in opencode) carry canon vN.

## Completion criteria

Every session the tool spawns is primed with the current rules and needs no repository file to know them.

## Files affected

- `internal/server/changesession.go`, prompt tests; `internal/server/settings.go` only if the canon section integrates with `promptWith` assembly.

## Notes

Existing sessions keep their prime — the same semantics as the old file-read model; only new sessions get new rules.
