# MAC-78: Integrate choice questions into Chat composer

## Objective

Present pending session questions inside the floating Chat composer, with
compact navigation, dismissible UI, and custom text answers for choices.

## Dependencies

- MAC-77

## Scope

- Move the active pending form from the transcript into the rounded composer.
- Preserve form answers and active step across transcript polling.
- Replace text navigation with icon-only Back and Forward controls.
- Add a centered Close action that dismisses the question UI without changing
  the lessmess change or replying to the upstream form.
- Restore a custom text answer alongside provided single- and multi-select
  options.
- Return to the normal prompt after dismissal or submission.

## Implementation steps

1. Add a composer form host and mount fresh pending forms into it.
2. Generalize form state/event handling to mounted forms.
3. Add dismissal state and restore the prompt without submitting.
4. Rebuild form navigation as left/center/right icon controls.
5. Add and serialize custom choice answers, including validation and review.
6. Replace compact fullscreen form CSS with composer-contained styling.
7. Update server, controller, accessibility, and responsive contracts.

## Verification

- Run focused form rendering, wizard, composer, and accessibility tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

Pending questions occupy the rounded composer in place of the prompt, can be
navigated or dismissed, and accept custom text answers without losing state.

## Files affected

- `web/templates/layout.html`
- `web/templates/partials.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/chat_test.go`
- `internal/server/render_test.go`

## Notes
