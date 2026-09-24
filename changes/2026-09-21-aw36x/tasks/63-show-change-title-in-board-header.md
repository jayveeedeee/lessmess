# MAC-63: Show change title in board header

## Objective

Show the human-readable change name in the board header instead of its dated
storage identifier.

## Dependencies

- MAC-62

## Scope

- Use the change title for the root board heading.
- Use the change title for the root crumb on nested task boards.
- Remove the current change-ID prefix from the visible Chat session title.
- Preserve the change ID in URLs and machine-readable attributes.

## Implementation steps

1. Add the title to the board view with an ID fallback for unreadable state.
2. Render the title in root and nested board navigation.
3. Normalize mapped session titles for the Chat header, including navigation
   refreshes and renames.
4. Pin the board and Chat display contracts with tests.

## Verification

- Run focused and full server tests, vet, validation, build, and served-page
  checks.

## Completion criteria

The board displays `Mobile API Chat` rather than its dated change ID,
without changing routing or API identifiers.

## Files affected

- `internal/server/render.go`
- `internal/server/render_test.go`
- `internal/server/render_nested_test.go`
- `web/templates/board.html`

## Notes

The identifier remains authoritative internally and is intentionally retained
in links and `data-change` attributes.
