# Implementation Plan: Tasks And Reminders

**Branch**: `016-tasks-reminders` | **Date**: 2026-04-23 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/016-tasks-reminders/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Add a first-class daemon-owned reminders domain on top of the existing trigger,
workflow, runtime, and delivery planes. The design closes roadmap 31 by introducing a
dedicated `daemon/internal/reminders` package that persists reminder resources,
occurrence history, explicit lifecycle actions, lightweight follow-up links, and
optional workflow-launch configuration while reusing scheduler trigger semantics,
shared delivery routing, and the normal run/workflow execution path. Verification stays
in `DOPE_ENV=test` and focuses on truthful distinction among reminder lifecycle truth,
workflow-launch truth, and delivery truth.

## Technical Context

**Language/Version**: Go 1.24.0; Markdown docs; JSON Schema contracts  
**Primary Dependencies**: `daemon/internal/api`, new `daemon/internal/reminders`,
`daemon/internal/app`, `daemon/internal/store`, `daemon/internal/events`,
`daemon/internal/runtime`, `daemon/internal/orchestration`,
`daemon/internal/scheduler`, `daemon/internal/delivery`, `daemon/internal/calendar`,
`daemon/internal/mail`, `daemon/internal/contracts`, existing auth wiring, and a shared
background workflow launcher extracted from or built alongside the existing
schedule-workflow launch path  
**Storage**: SQLite daemon state with additive persistence for reminder resources,
reminder occurrences, reminder action history, and lightweight follow-up linkage plus
additive reminder linkage on runs/workflows and latest-delivery linkage on reminder
occurrences; no binary blob storage is required in phase 31  
**Testing**: `go test ./internal/reminders ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/delivery ./internal/contracts ./internal/calendar ./internal/mail`,
`make daemon-contract-test`, targeted reminder-route and recurring-lifecycle regressions,
and one manual `DOPE_ENV=test` walkthrough using `test_sink` delivery plus one
deterministic workflow-launch path  
**Target Platform**: macOS/Linux local daemon in `DOPE_ENV=test` by default, using the
existing localhost HTTP API, SQLite store, and operator-authenticated `/v1/*`
control-plane routes  
**Project Type**: Go daemon and harness control-plane service with schema-backed HTTP
and event contracts  
**Performance Goals**: create or inspect a reminder from persisted local state in
`<=500 ms`; detect and record a due reminder occurrence in `<=1 s` of its due time on
local test hardware for up to 100 active reminders; persist an occurrence state
transition or linked workflow-launch result in `<=1 s`; emit a reminder-domain delivery
outcome in `<=2 s` after the reminder occurrence reaches a deliverable state excluding
connector latency  
**Constraints**: roadmap 31 MUST keep reminder resources distinct from low-level
schedule resources while reusing phase 25 trigger semantics and phase 28 delivery
targets and preferences; lifecycle truth MUST distinguish `due`, `acknowledged`,
`snoozed`, `completed`, `dismissed`, `cancelled`, `overdue`, and `missed`; successful
workflow launch auto-acknowledges a due occurrence but does not complete it; failed
workflow launch leaves the occurrence `due` or later `overdue`; recurring reminders keep
at most one active unresolved occurrence at a time; acknowledged occurrences remain
historical and do not roll into `missed`; lightweight follow-up may reference calendar,
mail, workflow, or run truth without redefining those domains; full project management,
team assignment, and habit coaching remain out of scope  
**Scale/Scope**: one operator-managed daemon, low hundreds of reminders, low tens of due
occurrences at once, at most one active unresolved occurrence per recurring reminder,
and one repo-owned `test_sink` plus one deterministic workflow-launch path sufficient to
close roadmap 31 without live third-party dependencies

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes roadmap 31 only: first-class reminder
  resources, recurring and one-time reminder occurrences, explicit reminder lifecycle
  state, durable occurrence history, reminder-triggered workflow launch, lightweight
  follow-up links, restart-safe reminder truth, and shared delivery reuse. Full task
  planning, team assignment, and habit coaching remain out of scope.
- Production-grade change control: PASS. The design adds a dedicated
  `daemon/internal/reminders` plane and additive API, schema, event, store, and
  workflow-linkage changes rather than refactoring scheduler, runtime, or delivery
  broadly. Rollback is a single change-set revert of reminder-specific routes,
  persistence, projections, and docs while leaving phases 25, 28, 29, and 30 intact.
- Contracts and auditability: PASS. The plan names additive reminder, occurrence,
  action-history, follow-up-link, workflow-linkage, and delivery-linkage surfaces
  together with the schema and doc updates needed to keep overdue versus missed truth,
  auto-acknowledgement, and delivery linkage inspectable.
- Verification and observability: PASS. The design requires targeted package, contract,
  restart, recurring-rollover, and workflow-linkage regressions plus one manual
  `DOPE_ENV=test` walkthrough. Operator-visible reminder resources and occurrence/action
  history replace raw-log reconstruction.
- Environment and secrets: PASS. Local planning and later verification stay in
  `DOPE_ENV=test`; notification-only reminder validation requires no live connectors; any
  optional linked calendar/mail behavior reuses existing environment-scoped bindings and
  does not introduce new secret-bearing execution paths.

Post-design re-check:

- PASS. The design remains roadmap-closed to the first reminders slice and does not
  drift into full project management, team assignment, or memory products.
- PASS. Reminder due processing remains on the existing trigger, workflow, and delivery
  planes with additive reminder-owned records rather than a parallel executor.
- PASS. Reminder lifecycle, workflow execution, and delivery outcomes remain separate
  truths, which preserves operator auditability under partial failure.

## Project Structure

### Documentation (this feature)

```text
specs/016-tasks-reminders/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── reminder-domain-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── calendar/
│   ├── contracts/
│   ├── delivery/
│   ├── events/
│   ├── mail/
│   ├── orchestration/
│   ├── reminders/
│   ├── runtime/
│   ├── scheduler/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

docs/
├── harness/
└── runtime/

AGENTS.md
```

**Structure Decision**: Keep reminder resource management, recurrence evaluation,
occurrence lifecycle transitions, follow-up-link validation, restart recovery, and
workflow-trigger bookkeeping in a new `daemon/internal/reminders` package.
`daemon/internal/api` exposes additive reminder and occurrence routes.
`daemon/internal/app` wires reminder restoration and the reminder due loop beside the
existing scheduler and delivery managers. `daemon/internal/scheduler` remains the owner
of reusable trigger semantics rather than top-level reminder resources. `daemon/internal/runtime`
and `daemon/internal/orchestration` remain the only workflow execution owners after a
reminder launches downstream work. `daemon/internal/delivery` remains the owner of
background-result routing. `daemon/internal/store` persists reminder documents and
linkage indexes. `daemon/internal/calendar` and `daemon/internal/mail` continue to own
their source-domain truth for follow-up references. `schemas/` and `docs/` carry the new
contract and operator-guidance surfaces. `AGENTS.md` should point at this plan for
downstream task generation.

## Complexity Tracking

No constitution violations remain. The design avoids treating raw schedules as the
reminder API, avoids collapsing reminder state into workflow or delivery status, avoids
multiple active unresolved recurring occurrences, and avoids live connector or live
calendar/mail dependencies for roadmap closure.

## Implementation Notes

- Add daemon-owned reminder resources rather than deriving reminder truth from raw
  schedules or generic workflow metadata.
- Reuse scheduler trigger semantics for one-time and recurring next-due calculation,
  timezone handling, and restart-safe due evaluation, but persist reminder resources and
  reminder occurrences separately from schedule resources.
- Model reminder lifecycle around first-class reminder occurrences and explicit reminder
  action history. `overdue` remains a still-actionable late occurrence; `missed`
  remains a historical occurrence that no longer represents active work.
- Keep recurring reminders at one active unresolved occurrence at a time. When the next
  recurrence arrives and the prior occurrence is still unresolved, mark the prior
  occurrence `missed` and create one new `due` occurrence. Acknowledged occurrences stay
  historical and do not roll into `missed`.
- Represent reminder-triggered workflow launch as reminder-owned configuration, not as a
  new low-level schedule target exposed to operators. Successful launch auto-acknowledges
  the occurrence; failed launch leaves the occurrence `due` and later eligible for
  `overdue`.
- Reuse the normal run/workflow execution plane by extracting or reusing a shared
  background workflow launcher instead of inventing a reminder-only executor.
- Reuse the shared delivery plane by emitting reminder-owned background outcomes with
  additive reminder linkage. Reminder delivery failure must never overwrite reminder
  lifecycle truth.
- Store follow-up links as typed references to existing source-domain resources or
  operation IDs rather than copying source-domain state into reminder documents. When the
  source disappears, keep the reminder and surface stale or missing source truth
  explicitly.
- Keep local verification intentionally deterministic: one notification-only reminder,
  one recurring rollover case, one snooze path, one reminder-linked workflow success,
  one workflow-launch failure, and one calendar-linked follow-up are sufficient to close
  roadmap 31.

## Automated Verification

- `cd daemon && go test ./internal/reminders ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/delivery ./internal/contracts ./internal/calendar ./internal/mail`
- `make daemon-contract-test`
- `cd daemon && go test ./internal/api -run 'TestReminderRoutesCreateInspectOccurrencesAndActions|TestReminderRoutesReuseDigestDeliveryPreference|TestReminderLifecycleRoutesAndWorkflowLinkage|TestScheduleWorkflowLauncherPersistsReminderLinkageOnRunsAndWorkflows|TestReminderRoutePerformanceSmoke' -count=1`
- `cd daemon && go test ./internal/reminders -run 'TestManagerTickCreatesDueOccurrenceAndLinksDeliveryOutcome|TestManagerRecurringRemindersMarkMissedAndPreserveAcknowledgedHistory|TestManagerWorkflowLinkedReminderAcknowledgesOnSuccessAndStaysDueOnFailure|TestManagerRefreshesFollowUpLinkStaleness|TestManagerPerformanceSmoke' -count=1`

These commands are expected to cover:

- reminder create, list, inspect, cancel, reschedule, snooze, acknowledge, complete, and
  dismiss behavior
- one-time and recurring due evaluation with explicit overdue versus missed truth
- recurring rollover rules for unresolved versus acknowledged prior occurrences
- reminder-triggered workflow success and launch-failure semantics
- additive linkage from reminder occurrences to runs, workflows, and delivery outcomes
- restart-safe restoration of reminder resources, occurrence history, and active due
  state
- follow-up-link validation and stale-source truth without redefining calendar or mail
  domain state

Executed on 2026-04-23:

- `go mod tidy` in `daemon/` completed with no `go.mod` or `go.sum` fallout
- `go test ./internal/reminders ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/delivery ./internal/contracts ./internal/calendar ./internal/mail` passed
- `go test ./internal/api -run 'TestReminderRoutesCreateInspectOccurrencesAndActions|TestReminderRoutesReuseDigestDeliveryPreference|TestReminderLifecycleRoutesAndWorkflowLinkage|TestScheduleWorkflowLauncherPersistsReminderLinkageOnRunsAndWorkflows|TestReminderRoutePerformanceSmoke' -count=1` passed
- `go test ./internal/reminders -run 'TestManagerTickCreatesDueOccurrenceAndLinksDeliveryOutcome|TestManagerRecurringRemindersMarkMissedAndPreserveAcknowledgedHistory|TestManagerWorkflowLinkedReminderAcknowledgesOnSuccessAndStaysDueOnFailure|TestManagerRefreshesFollowUpLinkStaleness|TestManagerPerformanceSmoke' -count=1` passed
- `make daemon-contract-test` passed
- performance smoke logs recorded:
  - manager inspect `434.417µs`
  - manager due tick `129.352875ms`
  - manager occurrence projection `16.042µs`
  - manager acknowledge `329.917µs`
  - API create route `821.584µs`
  - API list route `63.333µs`
  - API occurrence route `25.334µs`

## Manual Verification

- `make daemon-run-test`
- `make daemon-test-status`
- pair or reuse a local bearer token
- create one `test_sink` delivery target and preference if a background reminder
  notification path will be validated
- create one one-time notification-only reminder and confirm it appears under
  `/v1/reminders` with no occurrence resolved yet
- wait for the reminder to become due and confirm `/v1/reminders/occurrences` shows a
  due occurrence with separate delivery linkage
- acknowledge the occurrence, then create another reminder and exercise snooze,
  complete, dismiss, and reschedule paths
- create one recurring reminder, let an occurrence remain unresolved until the next
  recurrence, and confirm the prior occurrence becomes `missed` while one new active due
  occurrence is created
- create one recurring reminder whose prior occurrence is acknowledged and confirm the
  next recurrence preserves the acknowledged occurrence as history rather than converting
  it to `missed`
- create one reminder configured to launch a workflow through a deterministic local
  entrypoint, confirm the due occurrence auto-acknowledges, and inspect linked run and
  workflow truth
- create one reminder-linked workflow that fails to start and confirm the reminder
  occurrence remains `due` or later `overdue` while workflow failure remains separate
- create one follow-up reminder linked to an existing calendar operation or artifact and
  confirm the reminder keeps source linkage without copying calendar truth into the
  reminder resource

Executed on 2026-04-23:

- primary `DOPE_ENV=test` walkthrough on `127.0.0.1:19192` confirmed:
  - one-time notification-only reminder `rem_af1eaed57523b78a` created as `pending`
  - due occurrence `rem_occ_3906f2bc55225546` surfaced separately from delivery truth
  - linked delivery `delivery_e4272a3804c248ec` reused digest preference
    `manual-pref-r31-b` and queued into summary window `summary_window_7a6aa135c1b3695a`
  - `acknowledge` on the overdue occurrence moved reminder truth to `acknowledged`
    without changing delivery truth
  - `snooze` on `rem_04fee8ef6c40f2e7` recorded a distinct `snoozed` action and updated
    reminder `nextDueAt`
  - `reschedule` on `rem_d10ee8aba45de1bf` kept reminder truth `pending` and moved
    `nextDueAt` from `2026-04-23T12:47:00Z` to `2026-04-23T12:49:00Z`
  - `complete` on `rem_5fa046ee00aaa910` transitioned overdue occurrence
    `rem_occ_6d69abe193f953c1` to `completed`
  - `dismiss` on `rem_cf7f0ff8fb69397f` transitioned overdue occurrence
    `rem_occ_b51815bf6203f6fd` to `dismissed`
  - recurring reminder `rem_90b4044e64fa83a9` rolled the first unresolved occurrence
    `rem_occ_fbb22ba4c79f54f7` to `missed` and created a new active occurrence
    `rem_occ_2bd1f59b7c5cd2be`
  - acknowledged recurring reminder `rem_40c3f4f853615e72` preserved first occurrence
    `rem_occ_8e26ee4b46cb4950` as `acknowledged` when the next recurrence created new
    active overdue occurrence `rem_occ_3b2f5e29d922f37f`
  - workflow-linked reminder `rem_e8022032ea7588ac` auto-acknowledged on launch and
    linked `run_2d58cb07ab888985` plus `wf_c9676f6f430f`
  - run-linked follow-up reminder `rem_3e9b2e69543bd428` preserved its typed run source
    reference
  - calendar-linked follow-up reminder `rem_bcff1ce4a5e978c3` stayed inspectable and
    projected `stale: true` for the missing calendar operation
- isolated `DOPE_ENV=test` walkthrough on `127.0.0.1:19193` with empty home and data
  roots confirmed workflow-start failure:
  - reminder `rem_0b8eb3856e4e7ac2` stayed `due` and then `overdue`
  - action history recorded `workflow_start_failed` with reason
    `workflow planning failed to start`

Manual validation notes:

- reminder API requests must use scheduler-native trigger kinds: `once` and `cron`;
  the older `"recurring"` example is obsolete
- for one-time reminders, choose a `fireAt` safely in the future during manual testing to
  avoid racing reminder creation against due evaluation

## Residual Risks

- Reusing scheduler trigger semantics without exposing raw schedule resources requires
  careful factoring so reminder due evaluation does not fork subtle timezone or
  restart-catch-up behavior from phase 25.
- If reminder-linked workflow launch and runtime-triggered delivery both emit user-facing
  notifications too broadly, operators may see duplicate background results.
- Follow-up links that reference calendar or mail truth depend on stable source-domain
  IDs; if those identifiers drift, stale-source handling must remain explicit rather than
  silently dropping reminder context.
- Phase 31 intentionally omits prioritization, team assignment, and rich task hierarchy;
  later roadmap work may need additive fields without breaking reminder history.

## Rollback Notes

- Rollback is a single change-set revert of reminder routes, reminder-manager wiring,
  reminder persistence, reminder schema updates, and reminder-specific docs.
- Existing schedules, workflows, runs, calendar resources, mail resources, and delivery
  outcomes remain valid historical truth if roadmap 31 is reverted because reminder
  linkage is additive.
- If rollback occurs after reminder resources were created, preserve reminder occurrence
  and action history as read-only audit truth even if new reminder creation is later
  disabled.
