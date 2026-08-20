# Tasks: Calendar Integration

**Input**: Design documents from `/specs/014-calendar-integration/`  
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/calendar-domain-surfaces.md](./contracts/calendar-domain-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes API, schema, event, persistence, runtime, workflow, schedule, and delivery linkage surfaces, so targeted unit, integration, contract, and restart verification is required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the feature scaffolding and shared file layout for the daemon-owned calendar domain.

- [x] T001 Create calendar package scaffolding in `daemon/internal/calendar/types.go`, `daemon/internal/calendar/manager.go`, `daemon/internal/calendar/backend.go`, `daemon/internal/calendar/fake_backend.go`, and `daemon/internal/calendar/artifacts.go`
- [x] T002 [P] Create calendar API handler scaffolding in `daemon/internal/api/calendar.go` and route registration stubs in `daemon/internal/api/server.go`
- [x] T003 [P] Create calendar API schema placeholders in `schemas/api/calendar-account-resource.schema.json`, `schemas/api/calendar-account-list.response.schema.json`, `schemas/api/calendar-event-resource.schema.json`, `schemas/api/calendar-event-list.response.schema.json`, `schemas/api/create-calendar-availability-query.request.schema.json`, `schemas/api/calendar-availability-query-resource.schema.json`, `schemas/api/create-calendar-event.request.schema.json`, `schemas/api/update-calendar-event.request.schema.json`, `schemas/api/cancel-calendar-event.request.schema.json`, `schemas/api/calendar-operation-resource.schema.json`, `schemas/api/calendar-operation-list.response.schema.json`, and `schemas/api/calendar-event-artifact.schema.json`
- [x] T004 [P] Create calendar event schema placeholders in `schemas/events/calendar-account-projected.event.schema.json`, `schemas/events/calendar-operation-requested.event.schema.json`, `schemas/events/calendar-operation-completed.event.schema.json`, `schemas/events/calendar-operation-failed.event.schema.json`, and `schemas/events/calendar-artifact-recorded.event.schema.json`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the shared persistence, fake-backend abstraction, manager wiring, and contract foundations that block all calendar stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T005 Add SQLite migration and store record types for `calendar_accounts`, `calendar_operations`, and `calendar_artifacts` plus linkage indexes in `daemon/internal/store/store.go`
- [x] T006 [P] Add shared calendar account, operation, event artifact, availability query, and operation-summary structs in `daemon/internal/calendar/types.go` and `daemon/internal/api/types.go`
- [x] T007 [P] Implement store helpers for calendar account projection, calendar operation, artifact, and availability query CRUD/list/get in `daemon/internal/store/store.go`
- [x] T008 [P] Wire calendar manager startup, dependency injection, and restore hooks into `daemon/internal/app/app.go` and `daemon/internal/api/server.go`
- [x] T009 [P] Extend the fake integration backend and calendar backend interfaces for deterministic calendar-domain support in `daemon/internal/integrations/backend.go`, `daemon/internal/integrations/fake_backend.go`, `daemon/internal/calendar/backend.go`, and `daemon/internal/calendar/fake_backend.go`
- [x] T010 [P] Add calendar contract validation scaffolding in `daemon/internal/contracts/validator.go` and `daemon/internal/contracts/contracts_test.go`
- [x] T011 [P] Add shared calendar-operation projection types for runtime, workflow, schedule, and delivery surfaces in `daemon/internal/calendar/types.go`, `daemon/internal/api/types.go`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/workflow-step-resource.schema.json`, `schemas/api/schedule-attempt-resource.schema.json`, and `schemas/api/delivery-outcome-resource.schema.json`
- [x] T012 [P] Add foundational restart and environment-scope regressions for persisted calendar records in `daemon/internal/app/app_test.go` and `daemon/internal/store/store_test.go`

**Checkpoint**: Calendar persistence, manager wiring, fake-backend abstraction, and contract scaffolding are ready; user story work can now proceed.

---

## Phase 3: User Story 1 - Inspect Availability And Calendar State (Priority: P1) 🎯 MVP

**Goal**: Users and operators can inspect the selected calendar account projection, list and inspect events, and run busy/free lookups without mutating calendar state.

**Independent Test**: Register healthy fake calendar integrations, inspect `/v1/calendar/accounts`, issue one read with an explicit `integrationId` and one without it, then list or inspect events and run a busy/free lookup while confirming the chosen account projection, event identity, canonical-default fallback, and availability truth are returned without any mutation side effects.

### Tests for User Story 1

- [x] T013 [P] [US1] Add contract tests for calendar account projection, explicit-`integrationId` route selection, canonical-default fallback, event list/detail, and availability query routes plus `calendar.account_projected` coverage in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [x] T014 [P] [US1] Add manager and store regressions for explicit-`integrationId` selection, canonical-default fallback, primary calendar projection, and busy/free non-mutation behavior in `daemon/internal/calendar/manager_test.go` and `daemon/internal/store/store_test.go`

### Implementation for User Story 1

- [x] T015 [P] [US1] Implement calendar account projection, explicit-`integrationId` read selection, canonical-default fallback, and fake backend read models in `daemon/internal/calendar/backend.go`, `daemon/internal/calendar/fake_backend.go`, and `daemon/internal/calendar/manager.go`
- [x] T016 [P] [US1] Implement event list/detail inspection and `busy_free` operation handling in `daemon/internal/calendar/manager.go` and `daemon/internal/calendar/types.go`
- [x] T017 [US1] Implement calendar account, event inspection, and availability query route handlers with optional request-scoped `integrationId` selection in `daemon/internal/api/calendar.go` and `daemon/internal/api/server.go`
- [x] T018 [US1] Persist calendar account projections, read operations, event snapshot artifacts for inspection routes, and availability summary artifacts for `busy_free` in `daemon/internal/store/store.go` and `daemon/internal/calendar/artifacts.go`
- [x] T019 [US1] Finalize calendar account, event inspection, and availability API schemas including optional `integrationId` selection surfaces in `schemas/api/calendar-account-resource.schema.json`, `schemas/api/calendar-account-list.response.schema.json`, `schemas/api/calendar-event-resource.schema.json`, `schemas/api/calendar-event-list.response.schema.json`, `schemas/api/create-calendar-availability-query.request.schema.json`, and `schemas/api/calendar-availability-query-resource.schema.json`
- [x] T020 [US1] Publish account projection, calendar read operation, and artifact events in `daemon/internal/calendar/manager.go`, `schemas/events/calendar-account-projected.event.schema.json`, `schemas/events/calendar-operation-requested.event.schema.json`, `schemas/events/calendar-operation-completed.event.schema.json`, `schemas/events/calendar-operation-failed.event.schema.json`, and `schemas/events/calendar-artifact-recorded.event.schema.json`

**Checkpoint**: User Story 1 is complete when calendar account projection, event inspection, and busy/free lookup are operator-visible and independently testable without any event mutation.

---

## Phase 4: User Story 2 - Create, Move, And Cancel Events Truthfully (Priority: P2)

**Goal**: Users can create, update, and cancel timed single events on the primary calendar while preserving event identity and truthfully rejecting out-of-scope mutation requests.

**Independent Test**: Create one timed event through the fake calendar backend, update it, cancel it, and confirm the same event identity persists across operations while recurring, all-day, attendee, and alternate-calendar mutation requests are rejected explicitly.

### Tests for User Story 2

- [x] T021 [P] [US2] Add contract and API regressions for timed event create/update/cancel routes, explicit-`integrationId` mutation selection, canonical-default fallback, and operation inspection responses in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [x] T022 [P] [US2] Add manager and store regressions for stable event identity, stale-state and conflict truth, timezone defaults, and cancel lifecycle in `daemon/internal/calendar/manager_test.go` and `daemon/internal/store/store_test.go`
- [x] T023 [P] [US2] Add regression coverage for recurring-event, all-day-event, attendee, and alternate-calendar mutation rejection in `daemon/internal/calendar/manager_test.go` and `daemon/internal/api/server_test.go`

### Implementation for User Story 2

- [x] T024 [P] [US2] Implement timed single-event create/update/cancel logic, explicit-`integrationId` mutation selection, canonical-default fallback, and primary-timezone defaults in `daemon/internal/calendar/manager.go` and `daemon/internal/calendar/types.go`
- [x] T025 [P] [US2] Implement explicit rejection of recurring, all-day, attendee, and alternate-calendar mutation requests in `daemon/internal/calendar/manager.go` and `daemon/internal/api/calendar.go`
- [x] T026 [US2] Implement event mutation and calendar operation inspection route handlers with optional request-scoped `integrationId` selection in `daemon/internal/api/calendar.go` and `daemon/internal/api/server.go`
- [x] T027 [US2] Persist mutation operation records and structured event artifacts with stable external event identity only when backend event state was observed in `daemon/internal/store/store.go` and `daemon/internal/calendar/artifacts.go`
- [x] T028 [US2] Finalize create/update/cancel request schemas with optional `integrationId` override, operation schemas, and event artifact schema in `schemas/api/create-calendar-event.request.schema.json`, `schemas/api/update-calendar-event.request.schema.json`, `schemas/api/cancel-calendar-event.request.schema.json`, `schemas/api/calendar-operation-resource.schema.json`, `schemas/api/calendar-operation-list.response.schema.json`, and `schemas/api/calendar-event-artifact.schema.json`
- [x] T029 [US2] Expose conflict, stale-state, unavailable, and no-op mutation truth in `daemon/internal/api/types.go`, `daemon/internal/api/calendar.go`, and `schemas/api/calendar-operation-resource.schema.json`
- [x] T030 [US2] Publish mutation-completed, mutation-failed, and artifact lifecycle events in `daemon/internal/calendar/manager.go` and `schemas/events/calendar-operation-completed.event.schema.json`, `schemas/events/calendar-operation-failed.event.schema.json`, and `schemas/events/calendar-artifact-recorded.event.schema.json`

**Checkpoint**: User Story 2 is complete when timed single-event mutation is truthful, identity-preserving, and all out-of-scope writes fail explicitly.

---

## Phase 5: User Story 3 - Run Calendar Work Through Schedules And Shared Delivery (Priority: P3)

**Goal**: Scheduled and workflow-driven calendar operations run through the normal runtime plane and route their terminal results through the shared delivery plane without collapsing calendar, readiness, and delivery truth together.

**Independent Test**: Execute a scheduled or workflow-driven calendar inspection or mutation against the fake backend, then confirm runtime/workflow surfaces expose `calendarOperationSummaries` and the resulting background delivery outcome links back to the calendar operation truth.

### Tests for User Story 3

- [x] T031 [P] [US3] Add API and contract regressions for scheduled and workflow-driven calendar operations plus delivery linkage in `daemon/internal/api/workflows_test.go`, `daemon/internal/api/schedules_test.go`, `daemon/internal/api/server_test.go`, and `daemon/internal/contracts/contracts_test.go`
- [x] T032 [P] [US3] Add restart and projection regressions for `calendarOperationSummaries` on tool calls, workflow steps, schedule attempts, and delivery outcomes in `daemon/internal/app/app_test.go`, `daemon/internal/runtime/runtime_test.go`, and `daemon/internal/store/store_test.go`

### Implementation for User Story 3

- [x] T033 [P] [US3] Attach immutable `calendarOperationSummaries` to tool calls, workflow steps, and schedule attempts in `daemon/internal/runtime/runtime.go`, `daemon/internal/api/types.go`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/workflow-step-resource.schema.json`, and `schemas/api/schedule-attempt-resource.schema.json`
- [x] T034 [P] [US3] Implement background calendar operation emission from workflow and schedule execution paths in `daemon/internal/api/workflows.go`, `daemon/internal/api/schedule_workflow_launcher.go`, and `daemon/internal/scheduler/scheduler.go`
- [x] T035 [US3] Link background calendar outcomes to shared delivery outcomes without collapsing truth planes in `daemon/internal/api/calendar.go`, `daemon/internal/delivery/linkage.go`, `daemon/internal/api/workflows.go`, and `schemas/api/delivery-outcome-resource.schema.json`
- [x] T036 [US3] Implement calendar operation list/detail filtering by run, workflow, schedule, and delivery linkage in `daemon/internal/api/calendar.go` and `daemon/internal/store/store.go`
- [x] T037 [US3] Finalize background calendar contract projections and event coverage in `schemas/api/tool-call-resource.schema.json`, `schemas/api/workflow-step-resource.schema.json`, `schemas/api/schedule-attempt-resource.schema.json`, `schemas/api/delivery-outcome-resource.schema.json`, and `schemas/events/calendar-operation-completed.event.schema.json`
- [x] T038 [US3] Extend the fake calendar backend with deterministic scheduled inspection, mutation, and stale-result scenarios in `daemon/internal/calendar/fake_backend.go` and `daemon/internal/integrations/fake_backend.go`

**Checkpoint**: User Story 3 is complete when background calendar work is independently testable through workflow and schedule paths and its delivery linkage remains additive and inspectable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish docs, schema fixtures, recorded verification, and rollback guidance for the full calendar domain roadmap.

- [x] T039 [P] Update calendar schema fixtures and validator coverage in `daemon/internal/contracts/contracts_test.go`, `daemon/internal/contracts/validator.go`, and all calendar-facing schema files under `schemas/api/` and `schemas/events/`
- [x] T040 [P] Document the calendar domain, truth-plane separation, and operator guidance in `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-api-and-event-model.md`, `docs/runtime/operator-trust-model.md`, and `docs/harness/harness-architecture.md`
- [x] T041 [P] Update downstream roadmap specs to reference the concrete calendar contract in `docs/specs/014-calendar-integration.md`, `docs/specs/015-mail-integration.md`, and `docs/specs/016-tasks-and-reminders.md`
- [x] T042 [P] Run the manual `KURA_ENV=test` calendar walkthrough with one explicit-`integrationId` read, one canonical-default read, one mutation, and one delivery-linked background run, then record observed results and latency measurements against the plan targets in `specs/014-calendar-integration/quickstart.md`
- [x] T043 Record automated verification commands, residual risks, rollback notes, and the phase-29 latency verification procedure in `specs/014-calendar-integration/plan.md` and `specs/014-calendar-integration/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all story work.
- **Phase 3: US1**: Depends on Phase 2; establishes the MVP calendar inspection and availability surface.
- **Phase 4: US2**: Depends on Phase 2 and builds on the account projection and operation model from US1 to add truthful mutation.
- **Phase 5: US3**: Depends on Phase 2 and is easiest to close after US1 and US2 define stable calendar operation and artifact truth.
- **Phase 6: Polish**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on other user stories.
- **US2 (P2)**: Builds on US1 account projection and operation truth but remains independently testable once timed-event mutation and artifact persistence exist.
- **US3 (P3)**: Builds on US1/US2 operation truth to project background calendar execution onto workflow, schedule, and delivery surfaces.

### Within Each User Story

- Tests and contract coverage land before or alongside implementation and must fail before the story is considered complete.
- Store and type changes precede API projection work that depends on persisted truth.
- Calendar manager behavior precedes route handlers and background execution wiring.
- Story-specific docs and recorded validation happen only after the corresponding behavior is functional.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together.
- In Foundational, shared typing, store helpers, app wiring, fake backend extension, validator work, and projection-shape work can proceed in parallel.
- For each user story, API/contract tests and manager/store regressions can be written in parallel.
- Within each story, persistence and schema work can proceed in parallel with route wiring once the manager behavior is stable.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T013 [US1] Add contract tests for calendar account projection, explicit-integrationId route selection, canonical-default fallback, event list/detail, and availability query routes plus calendar.account_projected coverage in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T014 [US1] Add manager and store regressions for explicit-integrationId selection, canonical-default fallback, primary calendar projection, and busy/free non-mutation behavior in daemon/internal/calendar/manager_test.go and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T015 [US1] Implement calendar account projection, explicit-integrationId read selection, canonical-default fallback, and fake backend read models in daemon/internal/calendar/backend.go, daemon/internal/calendar/fake_backend.go, and daemon/internal/calendar/manager.go"
Task: "T016 [US1] Implement event list/detail inspection and busy_free operation handling in daemon/internal/calendar/manager.go and daemon/internal/calendar/types.go"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T021 [US2] Add contract and API regressions for timed event create/update/cancel routes, explicit-integrationId mutation selection, canonical-default fallback, and operation inspection responses in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T022 [US2] Add manager and store regressions for stable event identity, stale-state and conflict truth, timezone defaults, and cancel lifecycle in daemon/internal/calendar/manager_test.go and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T024 [US2] Implement timed single-event create/update/cancel logic, explicit-integrationId mutation selection, canonical-default fallback, and primary-timezone defaults in daemon/internal/calendar/manager.go and daemon/internal/calendar/types.go"
Task: "T025 [US2] Implement explicit rejection of recurring, all-day, attendee, and alternate-calendar mutation requests in daemon/internal/calendar/manager.go and daemon/internal/api/calendar.go"
```

## Parallel Example: User Story 3

```bash
# Tests in parallel
Task: "T031 [US3] Add API and contract regressions for scheduled and workflow-driven calendar operations plus delivery linkage in daemon/internal/api/workflows_test.go, daemon/internal/api/schedules_test.go, daemon/internal/api/server_test.go, and daemon/internal/contracts/contracts_test.go"
Task: "T032 [US3] Add restart and projection regressions for calendarOperationSummaries on tool calls, workflow steps, schedule attempts, and delivery outcomes in daemon/internal/app/app_test.go, daemon/internal/runtime/runtime_test.go, and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T033 [US3] Attach immutable calendarOperationSummaries to tool calls, workflow steps, and schedule attempts in daemon/internal/runtime/runtime.go, daemon/internal/api/types.go, schemas/api/tool-call-resource.schema.json, schemas/api/workflow-step-resource.schema.json, and schemas/api/schedule-attempt-resource.schema.json"
Task: "T034 [US3] Implement background calendar operation emission from workflow and schedule execution paths in daemon/internal/api/workflows.go, daemon/internal/api/schedule_workflow_launcher.go, and daemon/internal/scheduler/scheduler.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate calendar account projection, event inspection, and busy/free lookup independently in `KURA_ENV=test`.
5. Treat this as the first executable checkpoint only; roadmap 29 is not closed until US2, US3, and Phase 6 complete.

### Incremental Delivery

1. Land Setup + Foundational to establish the calendar package, persistence, fake backend support, and contract scaffolding.
2. Deliver US1 for account projection, event inspection, and availability truth.
3. Deliver US2 for timed-event mutation with stable event identity and explicit out-of-scope rejection.
4. Deliver US3 for background workflow/schedule execution and shared delivery linkage.
5. Finish with docs, schema fixtures, recorded manual verification, and rollback notes.

### Parallel Team Strategy

1. One engineer lands store, app wiring, validator scaffolding, and fake backend extension in Setup + Foundational.
2. After Foundational is complete:
   - Engineer A takes US1 account projection, event inspection, and availability routes.
   - Engineer B takes US2 mutation rules, artifact lifecycle, and mutation contract coverage.
   - Engineer C takes US3 workflow/schedule/delivery linkage once the shared operation model is stable.

## Notes

- `[P]` means the task can run in parallel because it targets different files or only depends on completed foundational work.
- Every user story has explicit tests because roadmap 29 changes operator-visible behavior and contract-backed surfaces.
- Existing integrations, runtime, workflow, schedule, and delivery behavior must remain backward compatible throughout implementation.
- Manual quickstart validation complements but does not replace API, store, runtime, contract, and restart coverage.
