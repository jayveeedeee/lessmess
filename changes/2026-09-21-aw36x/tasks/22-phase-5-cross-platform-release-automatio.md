# MAC-22: Phase 5 cross-platform release automation

## Objective

Build, verify, checksum, and publish supported binaries automatically from version tags.

## Dependencies

- MAC-21

## Scope

- CI matrices for macOS, Linux, and Windows on supported architectures.
- Tag-gated GitHub Releases, checksums, changelog text, and failure-safe permissions.

## Implementation steps

1. Add native test jobs for each supported OS and architecture where runners exist.
2. Configure release automation to build only after tests pass and only for version tags.
3. Publish archives plus checksums using least-privilege workflow permissions.
4. Add a dry-run/snapshot path for pull requests without publishing.
5. Verify downloaded artifacts independently on each OS.

## Verification

- CI dry run, test tag in a safe release context, checksum verification, and executable smoke tests.

## Completion criteria

A version tag deterministically produces installable, verified artifacts for every supported target with no manual rebuild.

## Files affected

- `.github/workflows/`
- Release configuration
- README release documentation

## Notes

Do not claim an architecture supported when the OpenCode prerequisite is unavailable there.
