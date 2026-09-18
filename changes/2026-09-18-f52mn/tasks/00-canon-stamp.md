---
id: WCV-00
title: "Canon as single source: stamp, print command, init stops writing"
---

# WCV-00: Canon as single source: stamp, print command, init stops writing

Status: see [../ledger.md](../ledger.md).

## Objective

Make the embedded workflow text the single versioned source of the rules: stamp it, stop maintaining any `AGENTS.md` mirror, and give humans and stray sessions a way to print it.

## Dependencies

—

## Scope

- `internal/docs/assets/workflow_agents.md`, `internal/docs/init.go`, `internal/docs/init_test.go`, `cmd/lessmess/main.go`.

## Implementation steps

1. Stamp: `<!-- tasktracker:workflow v2 -->` as the asset's header comment (v2 — v1 is the pre-stamp legacy text); plain integer, bumped on every future wording change.
2. Light wording pass on the asset for injection delivery: no "written into AGENTS.md" framing; add one provenance line (injected into spawned sessions; `lessmess workflow print` emits it).
3. Versioned accessor (e.g. `WorkflowCanon() (text string, version int)`), replacing or wrapping `WorkflowInstructions()`.
4. `init`: stop writing the workflow text — a created root `AGENTS.md` becomes a minimal human-facing header plus the empty marker section; the marker-append behavior for older markerless files stays (it is the seed/gardener append target, not workflow delivery).
5. Delete the drift test; update init tests: created `AGENTS.md` contains no workflow text and has the marker section.
6. `lessmess workflow print` subcommand: prints the canon to stdout; purely embedded output, no repository reads.

## Verification

- `go vet ./...` and `go test ./internal/docs/` green.
- `lessmess workflow print` emits the stamped text.
- Fixture: `lessmess init` creates an `AGENTS.md` with no rulebook and with markers; an existing `AGENTS.md` is left untouched.

## Completion criteria

The asset is the only copy of the rules, stamped v2; nothing writes the rulebook into repositories; the rules are printable on demand.

## Files affected

- `internal/docs/assets/workflow_agents.md`, `internal/docs/init.go`, `internal/docs/init_test.go`, `cmd/lessmess/main.go`

## Notes

Reshaped by the 2026-09-18 injection pivot: the drift test and the in-task `AGENTS.md` remediation are gone; the source-repo strip moved to WCV-04 so board sessions stay self-sufficient until injection lands.
