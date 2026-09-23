# MAC-43: Step-by-step Chat form wizard with review

## Objective

Replace stacked Chat form fields with a clean one-question-at-a-time wizard and
a reliable final review/submit step.

## Dependencies

- MAC-42.

## Scope

- Show one question at a time at every width; use a full-screen form view on
  compact screens.
- Provide Previous and Next controls in a stable footer.
- Validate the active question before advancing.
- End with a summary of answers and an explicit Submit button.
- Make choice rows visually compact and suppress oversized native controls.
- Show submitting, success, and failure state; prevent duplicate submits.
- Preserve typed form reply payloads, defaults, descriptions, and accessibility.

## Implementation steps

1. Render form steps, progress, navigation, and review hooks.
2. Add delegated step navigation, per-field validation, answer collection, and
   review rendering.
3. Wire final submission with clear pending/error behavior.
4. Add full-screen compact and restrained desktop styling.
5. Add structural, controller, payload, and responsive contracts.

## Verification

- Focused/full tests, vet, JavaScript syntax, browser interaction at 390px and
  desktop, build, validation, and diff check.

## Completion criteria

Multiple questions never stack into one long form, mobile shows one full-screen
question, and reviewed answers submit successfully exactly once.

## Files affected

- `web/templates/partials.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/chat_test.go`
- `internal/server/render_test.go`

## Notes

The wizard is client presentation only; OpenCode remains authoritative for
field definitions and reply acceptance.
