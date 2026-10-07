# CMG-02: Verify: vet, tests, validate, README check

## Why

The gate is a deliberate API contract change (409/404 where commits used to
succeed), so the whole-suite pass and a docs check are the closeout gate
before the user reviews.

## What

- `go vet ./...` and `go test ./...` from the repo root, both green.
- `lessmess validate` clean.
- README.md: if it documents the per-change commit endpoint's behavior,
  update it to the gated contract; otherwise leave untouched.

## Files affected

- `README.md` (only if the endpoint's contract is documented there)

## Verification

- `go vet ./... && go test ./...` exit 0; `lessmess validate` reports no
  violations; any README edit matches the shipped codes and messages.
