---
id: PROJ-03
title: Onboarding wizard Project name step
---

# PROJ-03: Onboarding wizard Project name step

Status: see [../ledger.md](../ledger.md).

## Objective

Collect the project name in the onboarding wizard as a dedicated step right after Prerequisites.

## Dependencies

- PROJ-00 (schema + `GET /api/settings` effective value; the setup mux already serves it).

## Scope

- `web/templates/setup.html`, `web/static/app.js` (`initSetup`)
- No new endpoints.

## Implementation steps

1. `setup.html`: insert `<li data-step-nav="name">Project name</li>` between Prerequisites and
   Bootstrap in `#setup-steps-nav`, and a `<section class="setup-step" data-step="name" hidden>`
   after the prereqs section containing: intro copy ("Shown next to the logo and as the browser
   tab name"), `#setup-project-name` text input, a `setup-scope` fieldset with the established
   radio pair but **Project checked by default** (shared repo branding), actions
   `#setup-name-save` ("Save and continue") + `#setup-name-skip` ("Use the folder name"), and
   `#setup-name-status`.
2. `app.js` `initSetup`: on entering the name step, prefill `#setup-project-name` from
   `GET /api/settings` (`effective.general.projectName` — already includes the folder fallback);
   keep it editable if the fetch fails.
3. Wire the step chain: prereqs Continue → `showStep("name")`; `#setup-name-save` PUTs
   `{general:{projectName}}` to `/api/settings?scope=<radios>` (mirror the agent-save handler,
   including its status text handling and `state.nameSaved`), then continues to bootstrap;
   `#setup-name-skip` continues without a request. `setup-bootstrap-next` still targets the agent
   step.
4. Finish step: when `state.nameSaved`, include the chosen name as a row in `#setup-finish-summary`.
5. Leave every existing `data-step`/`data-step-nav` key untouched — this is a pure insertion.

## Verification

- Render test: the setup template contains the `name` step, its nav item, and the new ids.
- Manual pass: run `/setup` on an initialized repo — prereqs Continue lands on the name step
  prefilled with the folder/effective name; Save (both scopes) persists and the next header/tab
  render shows it; Skip proceeds without saving; bootstrap → agent → docs → finish still flow
  exactly as before; finish summary lists the saved name.

## Completion criteria

The wizard collects the project name with a working scope choice and prefill, existing steps and
their keys are unchanged, and skip leaves the folder-name default in place.

## Files affected

`web/templates/setup.html`, `web/static/app.js`, a render test.

## Notes

- Scope default diverges from the agent step (personal) on purpose — the project name is shared
  branding; noted in the plan's design decisions.
- Re-running the wizard from Settings → General must update an existing name the same way
  (prefill comes from the effective settings, so it does).
- Verification evidence 2026-09-17: setup render test pins `data-step="name"`,
  `data-step-nav="name"`, `#setup-project-name`, `#setup-name-save`/`-skip`,
  `setup-name-scope`; live check confirmed the step markup renders on /setup and the save PUT
  (`{general:{projectName}}`) works through the setup shell's `/api/settings`. Full wizard
  click-through awaits user review.
