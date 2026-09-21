# MAC-20: Phase 5 release and version foundation

## Objective

Establish reproducible versioned releases before adding platform packages.

## Dependencies

- MAC-19

## Scope

- Version command/build metadata, release naming, supported platform matrix, license decision, checksums, and release notes.
- Reproducible CGO-free builds with embedded assets.

## Implementation steps

1. Add a version command populated by linker flags with a useful development fallback.
2. Choose and add the distribution license before publishing packages.
3. Define artifact names, target OS/architectures, archive contents, and checksums.
4. Add local release configuration and tests for version/build output.

## Verification

- Local snapshot release builds every declared target and each binary reports the expected version.

## Completion criteria

Release artifacts have a stable contract suitable for automation and package-manager formulas.

## Files affected

- `cmd/lessmess/main.go`
- Release configuration
- License and README
- CLI tests

## Notes

Do not publish until native platform tests in later tasks pass.
