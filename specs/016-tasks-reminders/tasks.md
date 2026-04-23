# Tasks: Tasks And Reminders

**Input**: Design documents from `/specs/016-tasks-reminders/`  
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/reminder-domain-surfaces.md](./contracts/reminder-domain-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes API, schema, event, persistence, runtime, workflow, scheduler, and delivery linkage surfaces, so targeted unit, integration, contract, restart, and due-loop verification is required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the reminder-domain scaffolding and contract placeholders that later phases fill in.

- [X] T001 Create reminder package scaffolding in `daemon/internal/reminders/types.go`, `daemon/internal/reminders/manager.go`, `daemon/internal/reminders/manager_test.go`, and `daemon/internal/reminders/follow_up.go`
- [X] T002 [P] Create reminder API handler scaffolding in `daemon/internal/api/reminders.go` and register route stubs in `daemon/internal/api/server.go`
- [X] T003 [P] Create reminder API schema placeholders in `schemas/api/create-reminder.request.schema.json`, `schemas/api/reminder-resource.schema.json`, `schemas/api/reminder-list.response.schema.json`, `schemas/api/reminder-trigger-resource.schema.json`, `schemas/api/reminder-workflow-launch.schema.json`, `schemas/api/reminder-follow-up-link.schema.json`, `schemas/api/acknowledge-reminder.request.schema.json`, `schemas/api/snooze-reminder.request.schema.json`, `schemas/api/complete-reminder.request.schema.json`, `schemas/api/dismiss-reminder.request.schema.json`, `schemas/api/reschedule-reminder.request.schema.json`, `schemas/api/cancel-reminder.request.schema.json`, `schemas/api/reminder-occurrence-resource.schema.json`, `schemas/api/reminder-occurrence-list.response.schema.json`, `schemas/api/reminder-action-resource.schema.json`, and `schemas/api/reminder-action-list.response.schema.json`
- [X] T004 [P] Create reminder event schema placeholders in `schemas/events/reminder-created.event.schema.json`, `schemas/events/reminder-updated.event.schema.json`, `schemas/events/reminder-occurrence-created.event.schema.json`, `schemas/events/reminder-occurrence-transitioned.event.schema.json`, `schemas/events/reminder-workflow-launch-started.event.schema.json`, `schemas/events/reminder-workflow-launch-failed.event.schema.json`, and `schemas/events/reminder-delivery-linked.event.schema.json`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land shared persistence, reminder manager wiring, and cross-plane linkage foundations that block all reminder stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Add SQLite migration and store record types for `reminders`, `reminder_occurrences`, `reminder_actions`, and additive reminder linkage columns on runs and workflows in `daemon/internal/store/store.go`
- [X] T006 [P] Add shared reminder, trigger, occurrence, action-history, workflow-launch, and follow-up-link structs in `daemon/internal/reminders/types.go` and `daemon/internal/api/types.go`
- [X] T007 [P] Implement store helpers for reminder CRUD, occurrence listing, action-history reads, active-occurrence lookup, and reminder linkage persistence in `daemon/internal/store/store.go`
- [X] T008 [P] Extract shared background workflow-launch support for reminder reuse alongside schedules in `daemon/internal/api/schedule_workflow_launcher.go` and `daemon/internal/orchestration/manager.go`
- [X] T009 [P] Wire reminder manager startup, restore hooks, and due-loop registration into `daemon/internal/app/app.go` and `daemon/internal/api/server.go`
- [X] T010 [P] Extend contract validation scaffolding for reminder schemas and events in `daemon/internal/contracts/validator.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T011 [P] Add reminder-owned delivery linkage foundation in `daemon/internal/delivery/linkage.go`, `daemon/internal/delivery/types.go`, `daemon/internal/api/delivery_projection.go`, and `daemon/internal/api/types.go`
- [X] T012 [P] Add foundational restart and environment-scope regressions for persisted reminder records and due restoration in `daemon/internal/app/app_test.go` and `daemon/internal/store/store_test.go`

**Checkpoint**: Reminder persistence, app wiring, workflow-launch reuse, and delivery-linkage foundations are ready; story work can now proceed.

---

## Phase 3: User Story 1 - Create And Receive Personal Reminders (Priority: P1) 🎯 MVP

**Goal**: Users can create one-time or recurring reminders, inspect them as reminder-domain resources, and receive truthful due notifications through the shared delivery plane.

**Independent Test**: Create one one-time reminder and one recurring reminder, inspect both before due time, let them become due, and confirm `/v1/reminders`, `/v1/reminders/{id}`, and `/v1/reminders/occurrences` show reminder-owned truth plus linked shared-delivery outcomes.

### Tests for User Story 1

- [X] T013 [P] [US1] Add API and contract regressions for reminder create, list, detail, occurrence list/detail, environment-scoped delivery linkage, and shared delivery preference or digest reuse in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T014 [P] [US1] Add reminder manager and store regressions for one-time create, recurring create, next-due projection, notification-only due processing, and active-occurrence projection in `daemon/internal/reminders/manager_test.go` and `daemon/internal/store/store_test.go`

### Implementation for User Story 1

- [X] T015 [P] [US1] Implement reminder create/list/get behavior for one-time and recurring triggers in `daemon/internal/reminders/manager.go` and `daemon/internal/reminders/types.go`
- [X] T016 [P] [US1] Implement due evaluation, occurrence creation, notification-only dispatch, and latest delivery linkage in `daemon/internal/reminders/manager.go`, `daemon/internal/delivery/manager.go`, and `daemon/internal/delivery/linkage.go`
- [X] T017 [US1] Implement reminder create/list/detail and occurrence list/detail route handlers in `daemon/internal/api/reminders.go` and `daemon/internal/api/server.go`
- [X] T018 [US1] Persist reminder resources, next-due projections, occurrence rows, and delivery linkage summaries in `daemon/internal/store/store.go` and `daemon/internal/api/delivery_projection.go`
- [X] T019 [US1] Finalize reminder create/list/detail and occurrence API schemas in `schemas/api/create-reminder.request.schema.json`, `schemas/api/reminder-resource.schema.json`, `schemas/api/reminder-list.response.schema.json`, `schemas/api/reminder-trigger-resource.schema.json`, `schemas/api/reminder-occurrence-resource.schema.json`, and `schemas/api/reminder-occurrence-list.response.schema.json`
- [X] T020 [US1] Publish reminder-created, reminder-occurrence-created, and reminder-delivery-linked events in `daemon/internal/reminders/manager.go`, `schemas/events/reminder-created.event.schema.json`, `schemas/events/reminder-occurrence-created.event.schema.json`, and `schemas/events/reminder-delivery-linked.event.schema.json`

**Checkpoint**: User Story 1 is complete when reminder resources, due occurrences, and shared-delivery linkage are all inspectable without exposing raw scheduler internals.

---

## Phase 4: User Story 2 - Manage Explicit Reminder Lifecycle (Priority: P2)

**Goal**: Users and operators can inspect and act on explicit reminder lifecycle states, including overdue and missed occurrence truth that survives restart.

**Independent Test**: Let multiple reminders become due, then acknowledge, snooze, complete, dismiss, reschedule, and cancel representative occurrences while also forcing one overdue occurrence and one missed rollover, then verify the lifecycle and history remain inspectable after restart.

### Tests for User Story 2

- [X] T021 [P] [US2] Add API and contract regressions for acknowledge, snooze, complete, dismiss, reschedule, cancel, and reminder action-history routes in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T022 [P] [US2] Add reminder manager, app, and store regressions for overdue versus missed, unresolved recurring rollover, acknowledged-history preservation, and restart restoration in `daemon/internal/reminders/manager_test.go`, `daemon/internal/app/app_test.go`, and `daemon/internal/store/store_test.go`

### Implementation for User Story 2

- [X] T023 [P] [US2] Implement occurrence state-machine transitions and append-only action recording for acknowledge, snooze, complete, dismiss, cancel, and reschedule in `daemon/internal/reminders/manager.go` and `daemon/internal/reminders/types.go`
- [X] T024 [P] [US2] Implement overdue detection, unresolved rollover-to-missed behavior, acknowledged-history preservation, and active-occurrence selection in `daemon/internal/reminders/manager.go` and `daemon/internal/scheduler/trigger.go`
- [X] T025 [US2] Implement reminder lifecycle command handlers and reminder action-history routes in `daemon/internal/api/reminders.go` and `daemon/internal/api/server.go`
- [X] T026 [US2] Persist action-history rows, overdue and missed timestamps, snooze metadata, and recurrence rollover bookkeeping in `daemon/internal/store/store.go`
- [X] T027 [US2] Finalize lifecycle command and action-history schemas in `schemas/api/acknowledge-reminder.request.schema.json`, `schemas/api/snooze-reminder.request.schema.json`, `schemas/api/complete-reminder.request.schema.json`, `schemas/api/dismiss-reminder.request.schema.json`, `schemas/api/reschedule-reminder.request.schema.json`, `schemas/api/cancel-reminder.request.schema.json`, `schemas/api/reminder-action-resource.schema.json`, and `schemas/api/reminder-action-list.response.schema.json`
- [X] T028 [US2] Publish reminder-updated and reminder-occurrence-transitioned events for lifecycle, overdue, and missed truth in `daemon/internal/reminders/manager.go`, `schemas/events/reminder-updated.event.schema.json`, and `schemas/events/reminder-occurrence-transitioned.event.schema.json`

**Checkpoint**: User Story 2 is complete when reminder lifecycle commands and occurrence history produce durable, restart-safe state truth without collapsing overdue and missed into one status.

---

## Phase 5: User Story 3 - Track Lightweight Follow-Up And Workflow-Linked Reminders (Priority: P3)

**Goal**: Users can attach lightweight follow-up links and optional workflow launch behavior to reminders while keeping reminder, workflow, and delivery truth separated.

**Independent Test**: Create one follow-up reminder linked to existing work and one reminder configured to launch a workflow, let them become due, and confirm source linkage, workflow linkage, auto-acknowledge-on-launch, and launch-failure behavior remain independently inspectable.

### Tests for User Story 3

- [X] T029 [P] [US3] Add API and contract regressions for calendar-linked and run-or-workflow-linked follow-up references, reminder-triggered workflow launch, run and workflow reminder linkage, and workflow-start failure semantics in `daemon/internal/api/server_test.go`, `daemon/internal/api/workflows_test.go`, and `daemon/internal/contracts/contracts_test.go`
- [X] T030 [P] [US3] Add reminder, runtime, calendar, and store regressions for auto-acknowledge-on-launch, launch failure staying due or overdue, stale-source visibility, calendar-link reuse, and one non-calendar typed follow-up reference in `daemon/internal/reminders/manager_test.go`, `daemon/internal/runtime/runtime_test.go`, `daemon/internal/calendar/manager_test.go`, and `daemon/internal/store/store_test.go`

### Implementation for User Story 3

- [X] T031 [P] [US3] Implement follow-up link modeling, typed source references, stale-source detection, and source-summary projection in `daemon/internal/reminders/follow_up.go`, `daemon/internal/reminders/manager.go`, and `daemon/internal/api/types.go`
- [X] T032 [P] [US3] Implement reminder-triggered workflow launch wiring with separate reminder, run, and workflow truth plus auto-acknowledge-on-success semantics in `daemon/internal/reminders/manager.go`, `daemon/internal/api/schedule_workflow_launcher.go`, `daemon/internal/runtime/runtime.go`, and `daemon/internal/orchestration/manager.go`
- [X] T033 [US3] Extend reminder and workflow inspection routes to surface follow-up links and reminder linkage in `daemon/internal/api/reminders.go`, `daemon/internal/api/workflows.go`, and `daemon/internal/api/types.go`
- [X] T034 [US3] Persist follow-up link state, stale-source markers, and reminder linkage on runs and workflows in `daemon/internal/store/store.go`
- [X] T035 [US3] Finalize follow-up, workflow-launch, run and workflow reminder linkage, and delivery outcome schemas in `schemas/api/reminder-workflow-launch.schema.json`, `schemas/api/reminder-follow-up-link.schema.json`, `schemas/api/run-resource.schema.json`, `schemas/api/workflow-resource.schema.json`, `schemas/api/run-list.response.schema.json`, and `schemas/api/delivery-outcome-resource.schema.json`
- [X] T036 [US3] Publish reminder-workflow-launch-started, reminder-workflow-launch-failed, and reminder-updated follow-up projections in `daemon/internal/reminders/manager.go`, `schemas/events/reminder-workflow-launch-started.event.schema.json`, `schemas/events/reminder-workflow-launch-failed.event.schema.json`, and `schemas/events/reminder-updated.event.schema.json`

**Checkpoint**: User Story 3 is complete when reminder follow-up links and workflow launches are truthful, environment-scoped, and operator-visible without redefining calendar, mail, runtime, or delivery ownership.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish docs, verification, module hygiene, performance validation, and recorded rollout guidance for the reminder domain.

- [X] T037 [P] Update reminder architecture, API, and operator-trust docs in `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-api-and-event-model.md`, `docs/runtime/operator-trust-model.md`, `docs/runtime/daemon-tasks.md`, and `docs/specs/016-tasks-and-reminders.md`
- [X] T038 [P] Update harness and manual validation guidance for reminder due loops, delivery routing, and workflow-link testing in `docs/harness/harness-architecture.md` and `specs/016-tasks-reminders/quickstart.md`
- [X] T039 Run `go mod tidy` in `daemon/` and record any module fallout in `specs/016-tasks-reminders/plan.md`
- [X] T040 Add reminder performance smoke coverage for create or inspect latency, due detection latency, occurrence transition persistence, and delivery-link projection timing in `daemon/internal/reminders/manager_test.go`, `daemon/internal/api/server_test.go`, and `specs/016-tasks-reminders/quickstart.md`
- [X] T041 Run reminder automated verification in `daemon/` with `go test ./internal/reminders ./internal/api ./internal/store ./internal/app ./internal/runtime ./internal/orchestration ./internal/scheduler ./internal/delivery ./internal/contracts ./internal/calendar ./internal/mail`, targeted reminder route tests, and `make daemon-contract-test`, then record results in `specs/016-tasks-reminders/quickstart.md`
- [X] T042 Run the manual `DOPE_ENV=test` reminder walkthrough for notification-only delivery, shared delivery preference or digest reuse, recurring rollover, snooze, workflow success, workflow-start failure, calendar-linked follow-up, and one non-calendar follow-up link case, then record observed results and rollback notes in `specs/016-tasks-reminders/quickstart.md` and `specs/016-tasks-reminders/plan.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all user story work.
- **Phase 3: US1**: Depends on Phase 2; delivers the MVP reminder resource and due-notification slice.
- **Phase 4: US2**: Depends on Phase 2 and builds on US1 reminder resources to add durable lifecycle truth.
- **Phase 5: US3**: Depends on Phase 2 and is safest after US1 and US2 establish stable reminder, occurrence, and lifecycle models.
- **Phase 6: Polish**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on later stories.
- **US2 (P2)**: Builds on US1 reminder resource and occurrence truth to add explicit lifecycle transitions and history.
- **US3 (P3)**: Builds on US1 reminder ownership and US2 occurrence truth to add follow-up references and workflow linkage without collapsing planes.

### Within Each User Story

- Tests and contract coverage land before or alongside implementation and must fail before the story is considered complete.
- Store and type changes precede route projection work that depends on persisted truth.
- Reminder manager behavior precedes API handlers and cross-plane projection wiring.
- Story-specific docs and recorded walkthrough notes happen only after the corresponding behavior is functional.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together.
- In Foundational, shared typing, store helpers, workflow-launch extraction, app wiring, contract validation, delivery linkage work, and restart tests can proceed in parallel.
- For each user story, API and contract tests can be written in parallel with manager and store regressions.
- Within each story, schema work can proceed in parallel with manager and route implementation once the persistence shape is stable.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T013 [US1] Add API and contract regressions for reminder create, list, detail, occurrence list/detail, and environment-scoped delivery linkage in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T014 [US1] Add reminder manager and store regressions for one-time create, recurring create, next-due projection, notification-only due processing, and active-occurrence projection in daemon/internal/reminders/manager_test.go and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T015 [US1] Implement reminder create/list/get behavior for one-time and recurring triggers in daemon/internal/reminders/manager.go and daemon/internal/reminders/types.go"
Task: "T016 [US1] Implement due evaluation, occurrence creation, notification-only dispatch, and latest delivery linkage in daemon/internal/reminders/manager.go, daemon/internal/delivery/manager.go, and daemon/internal/delivery/linkage.go"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T021 [US2] Add API and contract regressions for acknowledge, snooze, complete, dismiss, reschedule, cancel, and reminder action-history routes in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T022 [US2] Add reminder manager, app, and store regressions for overdue versus missed, unresolved recurring rollover, acknowledged-history preservation, and restart restoration in daemon/internal/reminders/manager_test.go, daemon/internal/app/app_test.go, and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T023 [US2] Implement occurrence state-machine transitions and append-only action recording for acknowledge, snooze, complete, dismiss, cancel, and reschedule in daemon/internal/reminders/manager.go and daemon/internal/reminders/types.go"
Task: "T024 [US2] Implement overdue detection, unresolved rollover-to-missed behavior, acknowledged-history preservation, and active-occurrence selection in daemon/internal/reminders/manager.go and daemon/internal/scheduler/trigger.go"
```

## Parallel Example: User Story 3

```bash
# Tests in parallel
Task: "T029 [US3] Add API and contract regressions for follow-up links, reminder-triggered workflow launch, run and workflow reminder linkage, and workflow-start failure semantics in daemon/internal/api/server_test.go, daemon/internal/api/workflows_test.go, and daemon/internal/contracts/contracts_test.go"
Task: "T030 [US3] Add reminder, runtime, calendar, and store regressions for auto-acknowledge-on-launch, launch failure staying due or overdue, stale-source visibility, and calendar-link reuse in daemon/internal/reminders/manager_test.go, daemon/internal/runtime/runtime_test.go, daemon/internal/calendar/manager_test.go, and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T031 [US3] Implement follow-up link modeling, typed source references, stale-source detection, and source-summary projection in daemon/internal/reminders/follow_up.go, daemon/internal/reminders/manager.go, and daemon/internal/api/types.go"
Task: "T032 [US3] Implement reminder-triggered workflow launch wiring with separate reminder, run, and workflow truth plus auto-acknowledge-on-success semantics in daemon/internal/reminders/manager.go, daemon/internal/api/schedule_workflow_launcher.go, daemon/internal/runtime/runtime.go, and daemon/internal/orchestration/manager.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate reminder creation, inspection, due processing, and shared-delivery linkage in `DOPE_ENV=test`.
5. Treat this as the first executable checkpoint only; roadmap 31 is not closed until US2, US3, and Phase 6 complete.

### Incremental Delivery

1. Land Setup + Foundational to establish the reminder package, persistence, app wiring, workflow-launch reuse, and delivery linkage.
2. Deliver US1 for reminder creation, inspection, due processing, and shared-delivery truth.
3. Deliver US2 for explicit lifecycle commands, overdue versus missed truth, and recurring occurrence history.
4. Deliver US3 for follow-up links, reminder-triggered workflow launch, and cross-plane inspection linkage.
5. Finish with docs, `go mod tidy`, performance validation, verification, and rollback notes.

### Parallel Team Strategy

1. One engineer lands Setup + Foundational, especially store changes, app wiring, and shared launcher extraction.
2. After Foundational is complete:
   - Engineer A takes US1 reminder create, inspect, and due-notification flow.
   - Engineer B takes US2 lifecycle transitions, overdue or missed logic, and restart safety.
   - Engineer C takes US3 follow-up references and workflow-launch linkage.

## Notes

- `[P]` means the task can run in parallel because it targets different files or depends only on completed foundational work.
- Every user story includes explicit tests because roadmap 31 changes operator-visible behavior and schema-backed control-plane surfaces.
- Reminder lifecycle truth, delivery truth, and workflow truth must remain separate throughout implementation.
- Manual quickstart validation complements but does not replace API, store, runtime, scheduler, delivery, contract, and restart coverage.
