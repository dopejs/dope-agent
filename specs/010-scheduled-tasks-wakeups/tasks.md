# Tasks: Scheduled Tasks Wakeups

**Input**: Design documents from `/specs/010-scheduled-tasks-wakeups/`
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/schedule-surfaces.md](./contracts/schedule-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes API, schema, event, persistence, restart, and execution-boundary behavior, so targeted unit, integration, contract, and recovery coverage is required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the feature scaffolding and shared file layout for schedule resources, contracts, and scheduler-owned code.

- [X] T001 Create schedule API handler scaffolding in `daemon/internal/api/schedules.go`
- [X] T002 [P] Create scheduler domain scaffolding in `daemon/internal/scheduler/types.go` and `daemon/internal/scheduler/trigger.go`
- [X] T003 [P] Create schedule contract placeholders in `schemas/api/create-schedule.request.schema.json`, `schemas/api/schedule-resource.schema.json`, `schemas/api/schedule-list.response.schema.json`, `schemas/api/schedule-trigger-resource.schema.json`, and `schemas/api/schedule-attempt-resource.schema.json`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the shared persistence, lifecycle wiring, and contract foundations that block all schedule stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T004 Add SQLite migration and store record types for schedules, schedule targets, and dispatch attempts in `daemon/internal/store/store.go`
- [X] T005 [P] Add schedule request and response structs plus run/workflow linkage fields in `daemon/internal/api/types.go`
- [X] T006 [P] Add schedule event schema placeholders in `schemas/events/schedule-created.event.schema.json`, `schemas/events/schedule-status-changed.event.schema.json`, `schemas/events/schedule-dispatch-attempted.event.schema.json`, `schemas/events/schedule-dispatch-recorded.event.schema.json`, and `schemas/events/schedule-retry-scheduled.event.schema.json`
- [X] T007 [P] Extend API resource schemas for schedule linkage in `schemas/api/run-resource.schema.json`, `schemas/api/run-list.response.schema.json`, and `schemas/api/workflow-resource.schema.json`
- [X] T008 Wire scheduler startup and restored-state lifecycle into `daemon/internal/app/app.go` and `daemon/internal/scheduler/scheduler.go`
- [X] T009 Add base schedule contract validation entrypoints in `daemon/internal/contracts/contracts_test.go`
- [X] T010 [P] Add environment-scoped schedule isolation regressions in `daemon/internal/store/store_test.go` and `daemon/internal/api/server_test.go`

**Checkpoint**: Schedule persistence, app wiring, and contract scaffolding are ready; user story work can now proceed.

---

## Phase 3: User Story 1 - Schedule Future Work (Priority: P1) 🎯 MVP

**Goal**: Operators can create and inspect a one-time schedule, then observe it launch exactly one normal run or workflow when due.

**Independent Test**: Create a one-time schedule, verify it stays pending before its due time, then confirm it creates exactly one linked run or workflow after it becomes due.

### Tests for User Story 1

- [X] T011 [P] [US1] Add one-time schedule API and schema contract tests in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T012 [P] [US1] Add one-time due dispatch and run-linkage tests in `daemon/internal/scheduler/scheduler_test.go` and `daemon/internal/store/store_test.go`
- [X] T013 [P] [US1] Add one-time schedule cancel regressions for pre-dispatch behavior in `daemon/internal/api/server_test.go` and `daemon/internal/scheduler/scheduler_test.go`

### Implementation for User Story 1

- [X] T014 [P] [US1] Implement one-time schedule create/list/get route handlers in `daemon/internal/api/schedules.go` and `daemon/internal/api/server.go`
- [X] T015 [P] [US1] Implement one-time schedule persistence and query methods in `daemon/internal/store/store.go`
- [X] T016 [US1] Implement one-time trigger evaluation and normal run dispatch in `daemon/internal/scheduler/scheduler.go`
- [X] T017 [US1] Implement generic schedule cancel handling, including one-time pre-dispatch cancellation, in `daemon/internal/api/schedules.go`, `daemon/internal/api/server.go`, and `daemon/internal/scheduler/scheduler.go`
- [X] T018 [US1] Persist and project schedule-to-run or schedule-to-workflow linkage in `daemon/internal/store/store.go`, `daemon/internal/api/types.go`, and `schemas/api/run-resource.schema.json`
- [X] T019 [US1] Publish schedule creation and successful dispatch events in `daemon/internal/scheduler/scheduler.go` and `daemon/internal/events/bus.go`

**Checkpoint**: User Story 1 is complete when one-time schedules are operator-visible before fire time and create one normal downstream execution when due.

---

## Phase 4: User Story 2 - Manage Recurring Schedules (Priority: P2)

**Goal**: Operators can create recurring schedules, inspect timezone-aware next fire times, pause and resume them, and prevent overlapping executions.

**Independent Test**: Create a recurring schedule, observe at least one dispatch, pause before the next due time so no dispatch occurs, then resume and verify the next due time and future dispatching recover correctly.

### Tests for User Story 2

- [X] T020 [P] [US2] Add recurring schedule API and schema tests for create/pause/resume semantics in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T021 [P] [US2] Add recurring scheduler tests for timezone evaluation, next-due advancement, and non-reentrant overlap handling in `daemon/internal/scheduler/scheduler_test.go`

### Implementation for User Story 2

- [X] T022 [P] [US2] Implement cron-plus-timezone parsing and next-due calculation in `daemon/internal/scheduler/trigger.go` and `daemon/internal/scheduler/types.go`
- [X] T023 [P] [US2] Implement recurring schedule create, pause, resume, and cancel command routes in `daemon/internal/api/schedules.go` and `daemon/internal/api/server.go`
- [X] T024 [US2] Implement recurring schedule dispatch advancement and non-reentrant overlap skipping in `daemon/internal/scheduler/scheduler.go`
- [X] T025 [US2] Persist recurring pause/resume state, timezone metadata, and skipped-overlap attempt history in `daemon/internal/store/store.go`
- [X] T026 [US2] Finalize recurring trigger and schedule detail schemas in `schemas/api/create-schedule.request.schema.json`, `schemas/api/schedule-resource.schema.json`, `schemas/api/schedule-list.response.schema.json`, and `schemas/api/schedule-trigger-resource.schema.json`

**Checkpoint**: User Story 2 is complete when recurring schedules remain inspectable, timezone-correct, pauseable, resumable, and non-reentrant.

---

## Phase 5: User Story 3 - Understand Trigger Failures (Priority: P3)

**Goal**: Operators can distinguish dispatch failure, downstream failure, skipped or missed intervals, cancellation, retry progression, and retry exhaustion.

**Independent Test**: Exercise one successful schedule, one dispatch-side failure with retry/backoff, one downstream execution failure after successful dispatch, and one restart catch-up case, then confirm every outcome is distinct in persisted history and operator-visible events.

### Tests for User Story 3

- [X] T027 [P] [US3] Add API and contract tests for dispatch failure, retry, exhausted, and downstream failure truth in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T028 [P] [US3] Add restart catch-up and missed-interval recovery tests in `daemon/internal/app/app_test.go` and `daemon/internal/scheduler/scheduler_test.go`

### Implementation for User Story 3

- [X] T029 [P] [US3] Implement persisted dispatch-attempt history and outcome transitions in `daemon/internal/store/store.go` and `daemon/internal/scheduler/types.go`
- [X] T030 [US3] Implement bounded dispatch retry and backoff with operator-visible exhaustion in `daemon/internal/scheduler/scheduler.go`
- [X] T031 [US3] Implement dispatch-time target-reference resolution and invalid-target failure recording in `daemon/internal/scheduler/scheduler.go` and `daemon/internal/store/store.go`
- [X] T032 [US3] Implement bounded restart catch-up that dispatches only the most recent overdue trigger and records older missed intervals in `daemon/internal/app/app.go` and `daemon/internal/scheduler/scheduler.go`
- [X] T033 [US3] Link downstream run/workflow terminal status back into schedule attempt history in `daemon/internal/scheduler/scheduler.go`, `daemon/internal/store/store.go`, and `daemon/internal/api/types.go`
- [X] T034 [US3] Publish schedule failure, retry, skip, and catch-up events and finalize event schemas in `daemon/internal/scheduler/scheduler.go`, `schemas/events/schedule-created.event.schema.json`, `schemas/events/schedule-status-changed.event.schema.json`, `schemas/events/schedule-dispatch-attempted.event.schema.json`, `schemas/events/schedule-dispatch-recorded.event.schema.json`, and `schemas/events/schedule-retry-scheduled.event.schema.json`

**Checkpoint**: User Story 3 is complete when operators can reconstruct all schedule outcomes without raw logs.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish docs, fixtures, verification recording, and cross-story operator guidance.

- [X] T035 [P] Add schedule performance verification for create/detail latency and due-time detection in `daemon/internal/api/server_test.go`, `daemon/internal/scheduler/scheduler_test.go`, and `daemon/internal/app/app_test.go`
- [X] T036 [P] Update schedule contract fixtures and validator coverage in `daemon/internal/contracts/contracts_test.go`, `schemas/api/create-schedule.request.schema.json`, `schemas/api/schedule-resource.schema.json`, `schemas/api/schedule-list.response.schema.json`, `schemas/api/schedule-trigger-resource.schema.json`, `schemas/api/schedule-attempt-resource.schema.json`, `schemas/events/schedule-created.event.schema.json`, `schemas/events/schedule-status-changed.event.schema.json`, `schemas/events/schedule-dispatch-attempted.event.schema.json`, `schemas/events/schedule-dispatch-recorded.event.schema.json`, and `schemas/events/schedule-retry-scheduled.event.schema.json`
- [X] T037 [P] Document schedule lifecycle, restart catch-up, and operator usage in `docs/runtime/daemon-roadmaps.md`, `docs/harness/harness-architecture.md`, and `specs/010-scheduled-tasks-wakeups/quickstart.md`
- [X] T038 [P] Run manual `DOPE_ENV=test` schedule verification and record observed results in `specs/010-scheduled-tasks-wakeups/quickstart.md`
- [X] T039 Record automated verification commands, residual risks, and rollback notes in `specs/010-scheduled-tasks-wakeups/quickstart.md` and `specs/010-scheduled-tasks-wakeups/plan.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all story work.
- **Phase 3: US1**: Depends on Phase 2; establishes the MVP schedule surface.
- **Phase 4: US2**: Depends on Phase 2 and builds on the core schedule resource landed in US1.
- **Phase 5: US3**: Depends on Phase 2 and the core dispatch machinery from US1; restart catch-up and overlap-related truth are easiest to close after US2.
- **Phase 6: Polish**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on other user stories.
- **US2 (P2)**: Builds on US1 schedule creation and persistence but remains independently testable once implemented.
- **US3 (P3)**: Builds on US1 dispatch flow and reuses recurring semantics from US2 for overlap and catch-up truth.

### Within Each User Story

- Tests and contract coverage land before or alongside implementation and must fail before the behavior is complete.
- Store and type changes precede scheduler loop logic.
- Scheduler loop logic precedes API projections that depend on persisted outcome truth.
- Story-specific docs and verification happen only after the story behavior is functional.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together.
- In Foundational, API typing, schema files, and event schema scaffolding can proceed in parallel.
- For each user story, contract/API tests and scheduler/store tests can be written in parallel.
- Within stories, persistence work and route serialization work can proceed in parallel before the final scheduler integration task.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T011 [US1] Add one-time schedule API and schema contract tests in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T012 [US1] Add one-time due dispatch and run-linkage tests in daemon/internal/scheduler/scheduler_test.go and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T014 [US1] Implement one-time schedule create/list/get route handlers in daemon/internal/api/schedules.go and daemon/internal/api/server.go"
Task: "T015 [US1] Implement one-time schedule persistence and query methods in daemon/internal/store/store.go"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T020 [US2] Add recurring schedule API and schema tests for create/pause/resume semantics in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T021 [US2] Add recurring scheduler tests for timezone evaluation, next-due advancement, and non-reentrant overlap handling in daemon/internal/scheduler/scheduler_test.go"

# Implementation in parallel
Task: "T022 [US2] Implement cron-plus-timezone parsing and next-due calculation in daemon/internal/scheduler/trigger.go and daemon/internal/scheduler/types.go"
Task: "T023 [US2] Implement recurring schedule create, pause, resume, and cancel command routes in daemon/internal/api/schedules.go and daemon/internal/api/server.go"
```

## Parallel Example: User Story 3

```bash
# Tests in parallel
Task: "T027 [US3] Add API and contract tests for dispatch failure, retry, exhausted, and downstream failure truth in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T028 [US3] Add restart catch-up and missed-interval recovery tests in daemon/internal/app/app_test.go and daemon/internal/scheduler/scheduler_test.go"

# Implementation in parallel
Task: "T029 [US3] Implement persisted dispatch-attempt history and outcome transitions in daemon/internal/store/store.go and daemon/internal/scheduler/types.go"
Task: "T031 [US3] Implement dispatch-time target-reference resolution and invalid-target failure recording in daemon/internal/scheduler/scheduler.go and daemon/internal/store/store.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate one-time schedule creation, pending inspection, and single downstream dispatch in `DOPE_ENV=test`.
5. Stop if only the MVP trigger plane is needed.

### Incremental Delivery

1. Land Setup + Foundational to establish scheduler, persistence, and contract scaffolding.
2. Deliver US1 for one-time schedules and downstream linkage.
3. Deliver US2 for recurring schedule management and non-reentrant overlap behavior.
4. Deliver US3 for retry/backoff, missed/skipped history, restart catch-up, and explicit failure truth.
5. Finish with docs, contract fixtures, and recorded verification.

### Parallel Team Strategy

1. One engineer lands store and app wiring in Setup + Foundational.
2. After Foundational is complete:
   - Engineer A takes US1 API + scheduler dispatch
   - Engineer B takes US2 recurring and timezone semantics
   - Engineer C takes US3 retry/catch-up/failure truth once US1 dispatch machinery is available

## Notes

- `[P]` means the task can run in parallel because it targets different files or only depends on completed foundational work.
- Every user story has explicit tests because this roadmap changes operator-visible behavior and contract-backed surfaces.
- Existing direct run/workflow flows must remain backward compatible throughout implementation.
- Do not treat manual quickstart validation as a substitute for scheduler, API, store, contract, and restart coverage.
