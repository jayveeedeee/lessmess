# MAC-08: Phase 2 mobile verification

## Objective

Verify Phase 2 makes routine coding sessions complete and performant on mobile.

## Dependencies

- MAC-07

## Scope

- Automated regression, accessibility, long-session performance, and live mobile checks.
- Feature matrix and user documentation for Phase 2 controls.

## Implementation steps

1. Exercise attachments, references, tools, diffs, selectors, commands, skills, and context state against a real service.
2. Test a long transcript over a throttled mobile connection and record bounded request/render behavior.
3. Check keyboard, touch, screen-reader labels, scroll retention, and desktop Terminal regression.
4. Update README and the feature matrix with supported and deferred capabilities.

## Verification

- `go vet ./... && go test ./...`
- Static build, `lessmess validate`, and 375px/390px live smoke evidence.

## Completion criteria

The Phase 2 gate in `plan.md` is met and documented before Phase 3 starts.

## Files affected

- `README.md`
- Feature matrix location selected during implementation
- Relevant tests and package learnings

## Notes

Record measured behavior rather than claiming performance from fixture size alone.
