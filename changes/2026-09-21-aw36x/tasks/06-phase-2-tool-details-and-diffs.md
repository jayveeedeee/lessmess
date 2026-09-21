# MAC-06: Phase 2 tool details and diffs

## Objective

Make tool execution and code changes reviewable on a phone instead of showing only summary rows.

## Dependencies

- MAC-05

## Scope

- Expandable tool input/output, timing, status, and errors.
- Affected-file summaries and readable unified diffs.
- Copy controls, wrapping, truncation, and explicit load-more for large output.

## Implementation steps

1. Normalize known tool states while preserving a generic fallback.
2. Render collapsed summaries with on-demand detail and safe text-only output.
3. Add mobile diff layout with line-level additions/deletions and long-line handling.
4. Test streaming transitions, failed tools, large output, binary files, and hostile text.

## Verification

- Render tests plus live edit/read/shell tool runs at 375px and desktop widths.

## Completion criteria

Users can understand what tools ran and review resulting edits without opening Terminal or a desktop diff viewer.

## Files affected

- `internal/server/chat.go`
- Chat templates and tests
- `web/static/app.js`
- `web/static/app.css`

## Notes

Default to summaries; do not ship megabytes of hidden output in every poll.
