# MAC-59: Expose subtasks in Chat Work and task details

## Objective

Make decomposed tasks recognizable in Chat Work and provide a direct path from
task details into that task's nested Work list.

## Dependencies

- MAC-58.

## Scope

- Add a subtle subtask progress marker to decomposed task rows in Chat Work.
- Add a View subtasks action only to decomposed task details.
- Load the nested board into Chat Work without leaving Chat.
- Provide a return path from nested Work to the change's root task list.

## Implementation steps

1. Project subtree rollup data into task detail responses.
2. Preserve the board's existing subtask metadata in Chat Work rows.
3. Add nested Work loading and root-list navigation.
4. Style and verify the marker and detail action across desktop and mobile.

## Verification

- Nested board/detail and compact Work contracts.
- Full tests, vet, JavaScript syntax, build, workflow validation, and diff check.

## Completion criteria

Decomposed tasks carry a quiet progress marker in Work; opening one shows a
View subtasks action that replaces Work with its child task list and permits
returning to all tasks.

## Files affected

- `internal/server/server.go`
- `internal/server/render.go`
- `internal/server/render_nested_test.go`
- `internal/server/render_test.go`
- `web/templates/partials.html`
- `web/static/app.js`
- `web/static/app.css`

## Notes

- Existing subtree progress remains display-only and is not persisted.
- Chat Work rows show a restrained completion fraction only when a task has a
  sub-plan; the task detail then offers the explicit View subtasks action.
- Nested Work preserves Chat, labels the scope, and includes an All tasks path;
  the same detail action falls back to the scoped board outside Chat.
- Verified with focused nested/detail/Work contracts, full tests, vet,
  JavaScript syntax, static build, workflow validation, diff check, and served
  asset checks.
