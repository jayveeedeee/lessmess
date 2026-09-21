
# ONB-07: README and full verification

Status: see [../ledger.md](../ledger.md).

## Objective

Document the onboarding flow for users and run the full change-level verification before close-out review.

## Dependencies

- ONB-05 (wizard UI complete)
- ONB-06 (banner + re-entry complete)

## Scope

- `README.md`: first-run section — what happens when you `lessmess serve` an uninitialized repo (setup mode), the wizard steps, what gets written (`AGENTS.md`, `changes/`, `.gitignore`, `opencode.json`, optional `agentsdocs.json`), where agent/model defaults live, the opt-in docs seed (budget, resume cursor), `.lessmess/onboarding.json`, and how to re-run onboarding from Settings. Keep it concise and consistent with the existing README structure.
- Full gates: `go vet ./...`, `go test ./...`, `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
- Manual E2E matrix per [../plan.md](../plan.md) "Testing and verification strategy": blank repo full flow (skip + opt-in), restart no-wizard, this-repo banner dismiss, CLI `docs seed` honoring configured agent/model.
- Update `plan.md`/ledger notes with verification evidence.

## Implementation steps

1. Write the README section (commands, endpoints, files; no screenshots).
2. Run the gates; fix anything red.
3. Run the E2E matrix with the rebuilt binary; record evidence.
4. Sweep: plan/ledger/task files agree with what shipped.

## Verification

- All gates green; E2E evidence recorded in the ledger; README renders correctly and matches actual behavior.

## Completion criteria

- Docs accurate, gates green, E2E matrix executed and evidenced; change ready for user review.

## Files affected

- `README.md`
- `changes/2026-09-13-4/*` (evidence/notes updates)

## Notes

- Per repo learning: README is the human-facing overview and must be updated when user-visible commands, endpoints, or workflows change.
