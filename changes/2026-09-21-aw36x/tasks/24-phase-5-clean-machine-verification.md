# MAC-24: Phase 5 clean-machine verification

## Objective

Prove the complete packaged mobile workflow on clean macOS, Linux, and Windows environments.

## Dependencies

- MAC-23

## Scope

- Fresh install, upgrade, uninstall, onboarding, prerequisites, private-network mobile access, and full Chat workflow.
- Final support matrix and release/runbook documentation.

## Implementation steps

1. Install from each package channel on clean target systems with no compiler or source checkout.
2. Complete onboarding, initialize a sample repository, connect OpenCode, and run the Phase 1–4 mobile acceptance flows.
3. Verify LAN/VPN guidance, unchanged localhost default, shutdown/restart, upgrade with state preservation, and uninstall cleanup.
4. Confirm desktop Terminal on supported Unix systems and its explicit native-Windows capability state.
5. Publish the tested OS/architecture/package matrix and troubleshooting runbook.

## Verification

- Recorded clean-machine evidence for every supported target.
- `go vet ./...`, `go test ./...`, release checksums, package validations, and `lessmess validate` all pass.

## Completion criteria

The Phase 5 gate and complete roadmap acceptance are met: a new user can install without build tools and run the API-driven workflow on every supported OS.

## Files affected

- `README.md`
- Release and installation runbook
- Support/feature matrix
- Any defects found during clean-machine testing

## Notes

Do not close the umbrella change based only on CI; clean-machine and phone-browser evidence is required.
