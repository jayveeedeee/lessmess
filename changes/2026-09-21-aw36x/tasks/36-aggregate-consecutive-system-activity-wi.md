# MAC-36: Aggregate consecutive system activity with live status

## Objective

Aggregate consecutive reasoning and command activity into one compact System
disclosure with a live current-activity summary.

## Dependencies

- MAC-35.

## Scope

- Group consecutive reasoning, tool, and shell activity without swallowing
  conversational text, errors, permissions, or forms.
- While activity is running, show a small accessible spinner and a concise
  current action such as `Thinking` or `Running shell`.
- Once all grouped activity settles, label the collapsed group `System`.
- Expanding System reveals the current activity lines, which retain their own
  nested expandable details.
- Preserve lazy detail loading, transcript refresh, and history behavior.

## Implementation steps

1. Represent consecutive passive activity as grouped transcript data.
2. Render an accessible outer System disclosure around existing activity rows.
3. Derive active summary text from authoritative message/tool status.
4. Add spinner, nested disclosure, and compact-state styles.
5. Add grouping, running, completed, and lazy-detail contracts.

## Verification

- Focused Chat tests, full Go tests/vet, JavaScript syntax, build, validation,
  and diff check.

## Completion criteria

Consecutive activity consumes one collapsed line, announces the current action
while active, settles to `System`, and preserves nested details when expanded.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`
- `web/templates/partials.html`
- `web/static/app.css`
- `web/static/app.js` if disclosure state needs refresh preservation

## Notes

The outer disclosure must not eagerly load nested tool or diff payloads.
