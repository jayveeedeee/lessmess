---
id: WCV-02
title: Validate warning and prompt precedence
---

# WCV-02: Validate warning and prompt precedence

Status: see [../ledger.md](../ledger.md).

## Objective

Originally: surface stale/divergent workflow text as an amber validate warning and make prompts authoritative over a stale `AGENTS.md`.

## Dependencies

—

## Scope

Cancelled 2026-09-18 (injection pivot). With no workflow text persisted in repositories there is nothing to go stale and nothing to warn about; the prompt-precedence half moved into WCV-01; `lessmess workflow print` moved into WCV-00.

## Implementation steps

None. Residue intentionally accepted: no validate signal for workflow delivery (none needed); no reach into sessions the tool does not spawn (the validator and board flag breaches reviewably).

## Verification

—

## Completion criteria

Cancelled — the reason is recorded in the change ledger's decision log and row.

## Files affected

- —

## Notes

Cancellation recorded 2026-09-18 alongside the pivot decision.
