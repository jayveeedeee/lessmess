# MAC-23: Phase 5 package manager distribution

## Objective

Provide straightforward package-manager installation and upgrades over the release artifacts.

## Dependencies

- MAC-22

## Scope

- Homebrew for macOS/Linux and one maintainable Windows channel, initially Winget or Scoop.
- Automated version/checksum updates, uninstall behavior, and prerequisite guidance.

## Implementation steps

1. Create the Homebrew tap/formula using released archives and architecture-specific checksums.
2. Select Winget or Scoop based on publication automation and user setup friction; add its manifest/package.
3. Automate package updates from successful releases without granting unnecessary repository access.
4. Document install, upgrade, uninstall, PATH, OpenCode/Git prerequisites, and service startup.
5. Test fresh install and upgrade from the previous version on each platform.

## Verification

- Package lint/validation plus clean Homebrew and Windows package installs that report the correct version.

## Completion criteria

Users can install and upgrade lessmess through normal platform commands without Go or Node.

## Files affected

- Package/tap manifests or external tap repository
- Release workflow integration
- README

## Notes

Prefer one reliable Windows package channel over shallow support for several.
