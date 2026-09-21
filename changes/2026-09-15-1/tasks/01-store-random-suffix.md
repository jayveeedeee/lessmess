
# CID-01: Store mint random suffixes and accept both formats

Status: see [../ledger.md](../ledger.md).

## Objective

Change ID minting in the store from a per-date max-number scan to a random
five-character suffix, and accept legacy numeric IDs everywhere the current
regex is checked.

## Dependencies

- CID-00 (spec text exists; message wording follows it).

## Scope

`internal/store/store.go` (`changeIDRe`, `CreateChange`),
`internal/store/validate.go` (Rule 1 message), and the store tests. No
server or template changes.

## Implementation steps

1. Widen `changeIDRe` in `store.go` to
   `^\d{4}-\d{2}-\d{2}-(\d+|[a-z0-9]{5})$` — this automatically updates the
   `Change(id)` lookup guard and `validChangeDirName`; the date-prefix
   validity check in `validChangeDirName` stays as is.
2. Introduce an injectable randomness seam, e.g. a package-level
   `var randSuffix = func() string { ... }` backed by `crypto/rand`, so tests
   can force deterministic sequences (follows the repo's injectable-seam
   style used by `SpawnCommand`/`SetDocsRunner`).
3. In `CreateChange`, replace the max-number scan with: build a set of all
   existing directory names under `changes/` and `changes/archive/` that
   share the date prefix; mint `date-` + `randSuffix()`; regenerate on hit
   with a bounded retry (10 attempts), returning a wrapped `ErrInvalid`
   error if exhausted. Optionally log a debug/warn line when a retry occurs.
4. Update the `CreateChange` doc comment: no counter, no gap rule; uniqueness
   by random generation plus collision check.
5. Update the Rule 1 violation message in `validate.go` from
   "not a valid change directory name (want YYYY-MM-DD-N)" to name both
   accepted formats.
6. Tests:
   - `TestCreateChange`: minted ID matches `^\d{4}-\d{2}-\d{2}-[a-z0-9]{5}$`;
     root-ledger row and scaffolded files unchanged otherwise.
   - Replace `TestCreateChangeGapRule` with a uniqueness test: mint many
     (e.g. 200) same-date changes and assert all IDs are distinct.
   - Add a forced-collision test with a stubbed `randSuffix` returning a
     duplicate first, then a fresh value; assert the second value wins.
   - Add Rule 1 validation cases: legacy `2026-09-10-0` and `2026-09-10-12`
     accepted; new-format names accepted; uppercase, 4-char, 6-char,
     bad-date, and malformed names rejected with the updated message.

## Verification

- `go vet ./...` and `go test ./internal/store/...` pass.
- A scaffolded change in a scratch repo (temp dir with a fixture
  `changes/` tree) yields the new format and a valid tree per
  `lessmess validate --dir <scratch>`.

## Completion criteria

- No numeric allocation logic remains in `CreateChange`.
- Both ID formats validate; all new tests pass.

## Files affected

- `internal/store/store.go`
- `internal/store/validate.go`
- `internal/store/store_test.go`

## Notes

- `Sscanf`-based suffix parsing disappears entirely with the scan.
- The 10-retry bound is defensive; 36⁵ ≈ 60.4M space makes exhaustion
  practically impossible but keeps the loop honest.
