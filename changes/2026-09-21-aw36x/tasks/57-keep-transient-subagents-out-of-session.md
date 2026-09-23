# MAC-57: Keep transient subagents out of session lists

## Objective

Keep change and task session lists limited to workflow sessions while retaining
temporary subagents in Chat's Agents hierarchy.

## Dependencies

- MAC-56.

## Scope

- Keep directly created change sessions visible.
- Keep explicitly or conventionally task-bound sessions visible.
- Do not promote unbound OpenCode descendants into workflow session lists.
- Preserve parent/child Chat navigation and existing OpenCode transcripts.

## Implementation steps

1. Restrict descendant reconciliation to valid task-prefixed sessions.
2. Filter previously mapped taskless children from change session responses.
3. Pin root, task-bound, transient-child, and deep-descendant behavior in tests.
4. Document the workflow-session boundary and run full verification.

## Verification

- Focused mapping and lifecycle tests.
- Full tests, vet, JavaScript syntax, build, workflow validation, and diff check.

## Completion criteria

Fresh change sessions and task sessions appear in their workflow lists;
temporary helper descendants appear only in Agents navigation.

## Files affected

- `internal/server/mapping.go`
- `internal/server/mapping_test.go`
- `README.md`

## Notes

- Visibility is based on workflow attachment, not agent names.
- Existing taskless child mappings remain on disk for compatibility but are no
  longer returned to the workflow list.
