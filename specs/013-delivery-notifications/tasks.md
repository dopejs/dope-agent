# Tasks: Delivery And Notifications

**Input**: Design documents from `/specs/013-delivery-notifications/`  
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/delivery-plane-surfaces.md](./contracts/delivery-plane-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes API, schema, event, persistence, restart, and execution-boundary behavior, so targeted unit, integration, contract, and restart verification is required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the feature scaffolding and shared file layout for the daemon-owned delivery plane.

- [X] T001 Create delivery package scaffolding in `daemon/internal/delivery/types.go`, `daemon/internal/delivery/manager.go`, `daemon/internal/delivery/dispatcher.go`, `daemon/internal/delivery/digest.go`, `daemon/internal/delivery/adapters.go`, `daemon/internal/delivery/test_sink.go`, and `daemon/internal/delivery/connector_adapter.go`
- [X] T002 [P] Create delivery API handler scaffolding in `daemon/internal/api/delivery.go` and route registration stubs in `daemon/internal/api/server.go`
- [X] T003 [P] Create delivery API schema placeholders in `schemas/api/create-delivery-target.request.schema.json`, `schemas/api/delivery-target-resource.schema.json`, `schemas/api/delivery-target-list.response.schema.json`, `schemas/api/update-delivery-target-status.request.schema.json`, `schemas/api/upsert-delivery-preference.request.schema.json`, `schemas/api/delivery-preference-resource.schema.json`, `schemas/api/delivery-preference-list.response.schema.json`, `schemas/api/delivery-outcome-resource.schema.json`, `schemas/api/delivery-outcome-list.response.schema.json`, `schemas/api/delivery-attempt-resource.schema.json`, `schemas/api/delivery-summary-window.resource.schema.json`, and `schemas/api/delivery-summary-window-list.response.schema.json`
- [X] T004 [P] Create delivery event schema placeholders in `schemas/events/delivery-target-registered.event.schema.json`, `schemas/events/delivery-target-status-changed.event.schema.json`, `schemas/events/delivery-preference-updated.event.schema.json`, `schemas/events/delivery-outcome-created.event.schema.json`, `schemas/events/delivery-attempt-recorded.event.schema.json`, `schemas/events/delivery-outcome-status-changed.event.schema.json`, and `schemas/events/delivery-summary-emitted.event.schema.json`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the shared persistence, wiring, type system, and contract foundations that block all delivery stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Add SQLite migration and store record types for `delivery_targets`, `delivery_preferences`, `delivery_outcomes`, `delivery_attempts`, and `delivery_summary_windows` plus additive latest-delivery linkage in `daemon/internal/store/store.go`
- [X] T006 [P] Add shared delivery target, preference, outcome, attempt, and summary-window structs in `daemon/internal/delivery/types.go` and `daemon/internal/api/types.go`
- [X] T007 [P] Implement store helpers for delivery target, preference, outcome, attempt, and summary-window CRUD and lookup queries in `daemon/internal/store/store.go`
- [X] T008 [P] Wire delivery manager startup, dependency injection, and restored retry or digest lifecycle hooks into `daemon/internal/app/app.go` and `daemon/internal/api/server.go`
- [X] T009 [P] Implement transport adapter interfaces and deterministic `test_sink` scaffolding in `daemon/internal/delivery/adapters.go`, `daemon/internal/delivery/test_sink.go`, and `daemon/internal/delivery/connector_adapter.go`
- [X] T010 [P] Add delivery contract validation scaffolding in `daemon/internal/contracts/validator.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T011 [P] Add shared source-linkage helpers for runs, workflows, schedules, and integration-linked outcomes in `daemon/internal/delivery/linkage.go`, `daemon/internal/api/types.go`, `schemas/api/run-resource.schema.json`, `schemas/api/workflow-resource.schema.json`, and `schemas/api/schedule-attempt-resource.schema.json`
- [X] T012 [P] Add foundational restart and environment-scope regressions for delivery state restoration in `daemon/internal/app/app_test.go` and `daemon/internal/store/store_test.go`

**Checkpoint**: Delivery persistence, startup wiring, transport abstractions, and contract scaffolding are ready; user story work can now proceed.

---

## Phase 3: User Story 1 - Receive Background Results Reliably (Priority: P1) 🎯 MVP

**Goal**: Users can receive terminal background results from schedules, workflows, and integration-backed work through one active delivery target without an active foreground session.

**Independent Test**: Configure an active delivery target, run a scheduled task or workflow without an active foreground request, and confirm the terminal success or failure result is routed through the chosen delivery target.

### Tests for User Story 1

- [X] T013 [P] [US1] Add contract tests for delivery target, delivery preference, and delivery outcome routes plus `delivery.target_registered`, `delivery.target_status_changed`, and `delivery.preference_updated` event families in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T014 [P] [US1] Add background result delivery regressions for scheduled runs and workflows using the `test_sink` adapter plus foreground chat reply backward-compat coverage in `daemon/internal/api/server_test.go`, `daemon/internal/api/workflows_test.go`, and `daemon/internal/scheduler/scheduler_test.go`

### Implementation for User Story 1

- [X] T015 [P] [US1] Implement delivery target create/list/get/activate/disable logic and publish `delivery.target_registered` plus `delivery.target_status_changed` events in `daemon/internal/delivery/manager.go` and `daemon/internal/delivery/types.go`
- [X] T016 [P] [US1] Implement user-default delivery preference resolution and single-target routing in `daemon/internal/delivery/manager.go` and `daemon/internal/delivery/dispatcher.go`
- [X] T017 [US1] Implement delivery target, preference, and outcome route handlers, including preference upsert paths that emit `delivery.preference_updated`, in `daemon/internal/api/delivery.go` and `daemon/internal/api/server.go`
- [X] T018 [US1] Emit delivery outcomes from terminal scheduled runs and workflows in `daemon/internal/api/server.go`, `daemon/internal/api/workflows.go`, `daemon/internal/scheduler/scheduler.go`, and `daemon/internal/delivery/dispatcher.go`
- [X] T019 [US1] Implement `test_sink` delivery and the first connector-backed transport adapter in `daemon/internal/delivery/test_sink.go`, `daemon/internal/delivery/connector_adapter.go`, `daemon/internal/connectors/supervisor.go`, and `daemon/internal/connectors/discord/transport.go`
- [X] T020 [US1] Persist delivery target, preference, outcome, and source-linkage state in `daemon/internal/store/store.go` and `daemon/internal/api/types.go`
- [X] T021 [US1] Finalize delivery target, delivery preference, and delivery outcome API schemas in `schemas/api/create-delivery-target.request.schema.json`, `schemas/api/delivery-target-resource.schema.json`, `schemas/api/delivery-target-list.response.schema.json`, `schemas/api/update-delivery-target-status.request.schema.json`, `schemas/api/upsert-delivery-preference.request.schema.json`, `schemas/api/delivery-preference-resource.schema.json`, `schemas/api/delivery-preference-list.response.schema.json`, `schemas/api/delivery-outcome-resource.schema.json`, and `schemas/api/delivery-outcome-list.response.schema.json`

**Checkpoint**: User Story 1 is complete when background runs and workflows can route one terminal result to one active delivery target through the new delivery plane.

---

## Phase 4: User Story 2 - Inspect Delivery Truth Separately From Execution Truth (Priority: P2)

**Goal**: Operators can inspect per-attempt retry, suppression, and terminal delivery failure separately from run, workflow, schedule, and integration readiness truth.

**Independent Test**: Trigger successful work, then force suppression, retryable delivery failure, and terminal delivery failure, and confirm operators can inspect execution truth and delivery truth as separate ledgers from the same linked resources.

### Tests for User Story 2

- [X] T022 [P] [US2] Add contract and API regressions for per-attempt history, suppression, and delivery-versus-execution separation in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T023 [P] [US2] Add retry exhaustion, no-failover, and restart restoration regressions in `daemon/internal/delivery/dispatcher_test.go`, `daemon/internal/app/app_test.go`, and `daemon/internal/store/store_test.go`

### Implementation for User Story 2

- [X] T024 [P] [US2] Implement delivery attempt lifecycle and per-attempt history persistence in `daemon/internal/delivery/dispatcher.go`, `daemon/internal/delivery/types.go`, and `daemon/internal/store/store.go`
- [X] T025 [P] [US2] Implement suppression policy handling and explicit outcome status transitions in `daemon/internal/delivery/manager.go` and `daemon/internal/delivery/dispatcher.go`
- [X] T026 [US2] Project latest-delivery summaries onto run, workflow, and schedule-attempt resources without mutating execution truth in `daemon/internal/api/types.go`, `daemon/internal/api/server.go`, `daemon/internal/api/workflows.go`, `schemas/api/run-resource.schema.json`, `schemas/api/workflow-resource.schema.json`, and `schemas/api/schedule-attempt-resource.schema.json`
- [X] T027 [US2] Link connector-backed delivery attempts to existing `connector_messages` transport evidence in `daemon/internal/delivery/connector_adapter.go`, `daemon/internal/store/store.go`, and `schemas/api/delivery-attempt-resource.schema.json`
- [X] T028 [US2] Publish delivery outcome and attempt event families in `daemon/internal/delivery/manager.go`, `daemon/internal/delivery/dispatcher.go`, `daemon/internal/events/bus.go`, `schemas/events/delivery-outcome-created.event.schema.json`, `schemas/events/delivery-attempt-recorded.event.schema.json`, and `schemas/events/delivery-outcome-status-changed.event.schema.json`
- [X] T029 [US2] Expose operator-visible retry, suppression, failure, and no-failover fields in outcome inspection responses and schemas in `daemon/internal/api/delivery.go`, `schemas/api/delivery-outcome-resource.schema.json`, and `schemas/api/delivery-attempt-resource.schema.json`

**Checkpoint**: User Story 2 is complete when operators can reconstruct delivery retries, suppression, and terminal failure without confusing them with source execution or integration readiness truth.

---

## Phase 5: User Story 3 - Reuse Delivery Targets And Summary Preferences (Priority: P3)

**Goal**: Delivery targets and preferences are reusable across schedules, workflows, and integration-linked results, and routine-success outcomes can be summarized through digest windows while failures and urgent results still deliver immediately.

**Independent Test**: Configure user-default preferences plus one integration-specific override, create background results from multiple sources, then confirm routine-success outcomes can join one digest window while urgent and failed results bypass the digest path and targets remain reusable.

### Tests for User Story 3

- [X] T030 [P] [US3] Add contract and API tests for integration-specific overrides and target reuse across schedules, workflows, and integration-linked outcomes in `daemon/internal/api/server_test.go`, `daemon/internal/api/workflows_test.go`, `daemon/internal/api/schedules_test.go`, and `daemon/internal/contracts/contracts_test.go`
- [X] T031 [P] [US3] Add digest-window, immediate-failure-bypass, and `delivery.summary_emitted` regressions in `daemon/internal/delivery/digest_test.go`, `daemon/internal/app/app_test.go`, and `daemon/internal/store/store_test.go`

### Implementation for User Story 3

- [X] T032 [P] [US3] Implement integration-specific preference overrides on top of user-default routing in `daemon/internal/delivery/manager.go`, `daemon/internal/delivery/linkage.go`, and `daemon/internal/integrations/bindings.go`
- [X] T033 [P] [US3] Implement summary-window membership, emission, restart restoration, and `delivery.summary_emitted` publication in `daemon/internal/delivery/digest.go`, `daemon/internal/delivery/dispatcher.go`, and `daemon/internal/store/store.go`
- [X] T034 [US3] Implement summary-window list and detail routes in `daemon/internal/api/delivery.go` and `daemon/internal/api/server.go`
- [X] T035 [US3] Finalize delivery preference and summary-window API schemas in `schemas/api/upsert-delivery-preference.request.schema.json`, `schemas/api/delivery-preference-resource.schema.json`, `schemas/api/delivery-preference-list.response.schema.json`, `schemas/api/delivery-summary-window.resource.schema.json`, and `schemas/api/delivery-summary-window-list.response.schema.json`
- [X] T036 [US3] Preserve a reusable delivery target model across `test_sink` and connector-backed transports in `daemon/internal/delivery/types.go`, `daemon/internal/delivery/test_sink.go`, `daemon/internal/delivery/connector_adapter.go`, and `schemas/api/delivery-target-resource.schema.json`
- [X] T037 [US3] Wire integration-linked outcome routing so overrides apply without rewriting integration readiness truth in `daemon/internal/api/workflows.go`, `daemon/internal/api/server.go`, `daemon/internal/policy/policy.go`, and `daemon/internal/delivery/linkage.go`

**Checkpoint**: User Story 3 is complete when delivery targets and preferences are reusable across source types and digest windows work only for routine-success results.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish docs, contract fixtures, verification recording, and downstream roadmap references for the full delivery plane.

- [X] T038 [P] Update delivery contract fixtures and validator coverage for target, preference, outcome, attempt, and summary event families in `daemon/internal/contracts/contracts_test.go`, `daemon/internal/contracts/validator.go`, and all delivery-facing schema files under `schemas/api/` and `schemas/events/`
- [X] T039 [P] Document the delivery plane, connector boundary, and delivery-versus-execution truth in `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-api-and-event-model.md`, `docs/runtime/operator-trust-model.md`, and `docs/channels/channel-reply-progression.md`
- [X] T040 [P] Update downstream roadmap specs to reference the shared delivery plane in `docs/specs/014-calendar-integration.md`, `docs/specs/015-mail-integration.md`, and `docs/specs/016-tasks-and-reminders.md`
- [X] T041 [P] Run the manual `DOPE_ENV=test` delivery walkthrough and record observed results in `specs/013-delivery-notifications/quickstart.md`
- [X] T042 Record automated verification commands, residual risks, and rollback notes in `specs/013-delivery-notifications/plan.md` and `specs/013-delivery-notifications/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all story work.
- **Phase 3: US1**: Depends on Phase 2; establishes the MVP delivery plane for background results.
- **Phase 4: US2**: Depends on Phase 2 and builds on the core delivery resources from US1 to make retry, suppression, and failure truth inspectable.
- **Phase 5: US3**: Depends on Phase 2 and the base routing model from US1; digest windows and integration overrides are easiest to close after US2 solidifies outcome and attempt truth.
- **Phase 6: Polish**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on other user stories.
- **US2 (P2)**: Builds on US1 delivery resources and routing, but remains independently verifiable once per-attempt history and latest-delivery summaries are implemented.
- **US3 (P3)**: Builds on US1 routing and US2 attempt truth to add reusable target preferences, integration overrides, and digest grouping.

### Within Each User Story

- Tests and contract coverage land before or alongside implementation and must fail before the behavior is considered complete.
- Store and type changes precede route and serialization work that depends on persisted truth.
- Delivery manager and dispatcher logic precede source-emission wiring from runtime, workflows, schedules, and integration-linked results.
- Story-specific docs and recorded validation happen only after the corresponding behavior is functional.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together.
- In Foundational, shared typing, store helpers, app wiring, adapter scaffolding, and contract validation work can proceed in parallel.
- For each user story, API/contract tests and runtime or store regressions can be written in parallel.
- Within each story, persistence work and schema or route projection work can proceed in parallel before final source-integration tasks.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T013 [US1] Add contract tests for delivery target, delivery preference, and delivery outcome routes plus delivery.target_registered, delivery.target_status_changed, and delivery.preference_updated event families in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T014 [US1] Add background result delivery regressions for scheduled runs and workflows using the test_sink adapter plus foreground chat reply backward-compat coverage in daemon/internal/api/server_test.go, daemon/internal/api/workflows_test.go, and daemon/internal/scheduler/scheduler_test.go"

# Implementation in parallel
Task: "T015 [US1] Implement delivery target create/list/get/activate/disable logic and publish delivery.target_registered plus delivery.target_status_changed events in daemon/internal/delivery/manager.go and daemon/internal/delivery/types.go"
Task: "T016 [US1] Implement user-default delivery preference resolution and single-target routing in daemon/internal/delivery/manager.go and daemon/internal/delivery/dispatcher.go"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T022 [US2] Add contract and API regressions for per-attempt history, suppression, and delivery-versus-execution separation in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T023 [US2] Add retry exhaustion, no-failover, and restart restoration regressions in daemon/internal/delivery/dispatcher_test.go, daemon/internal/app/app_test.go, and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T024 [US2] Implement delivery attempt lifecycle and per-attempt history persistence in daemon/internal/delivery/dispatcher.go, daemon/internal/delivery/types.go, and daemon/internal/store/store.go"
Task: "T025 [US2] Implement suppression policy handling and explicit outcome status transitions in daemon/internal/delivery/manager.go and daemon/internal/delivery/dispatcher.go"
```

## Parallel Example: User Story 3

```bash
# Tests in parallel
Task: "T030 [US3] Add contract and API tests for integration-specific overrides and target reuse across schedules, workflows, and integration-linked outcomes in daemon/internal/api/server_test.go, daemon/internal/api/workflows_test.go, daemon/internal/api/schedules_test.go, and daemon/internal/contracts/contracts_test.go"
Task: "T031 [US3] Add digest-window, immediate-failure-bypass, and delivery.summary_emitted regressions in daemon/internal/delivery/digest_test.go, daemon/internal/app/app_test.go, and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T032 [US3] Implement integration-specific preference overrides on top of user-default routing in daemon/internal/delivery/manager.go, daemon/internal/delivery/linkage.go, and daemon/internal/integrations/bindings.go"
Task: "T033 [US3] Implement summary-window membership, emission, restart restoration, and delivery.summary_emitted publication in daemon/internal/delivery/digest.go, daemon/internal/delivery/dispatcher.go, and daemon/internal/store/store.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate one scheduled or workflow-originated background result reaching one active delivery target in `DOPE_ENV=test`.
5. Treat this as the first executable checkpoint only; roadmap 28 is not complete until US2, US3, and Phase 6 close the full delivery-plane scope.

### Incremental Delivery

1. Land Setup + Foundational to establish the delivery package, persistence, startup wiring, transport adapters, and contract scaffolding.
2. Deliver US1 for background result routing and active target delivery.
3. Deliver US2 for per-attempt history, suppression, retry, and additive latest-delivery summaries.
4. Deliver US3 for integration overrides, target reuse, and routine-success digest grouping.
5. Finish with docs, contract fixtures, downstream spec references, and recorded manual verification.

### Parallel Team Strategy

1. One engineer lands store, app wiring, and contract scaffolding in Setup + Foundational.
2. After Foundational is complete:
   - Engineer A takes US1 target routes, routing resolution, and source-emission wiring.
   - Engineer B takes US2 attempt lifecycle, retry semantics, and latest-delivery summary projection.
   - Engineer C takes US3 digest windows, integration overrides, and downstream documentation once the shared outcome model is stable.

## Notes

- `[P]` means the task can run in parallel because it targets different files or only depends on completed foundational work.
- Every user story has explicit tests because this roadmap changes operator-visible behavior and contract-backed surfaces.
- Existing foreground reply flows, run truth, workflow truth, schedule truth, and integration readiness truth must remain backward compatible throughout implementation.
- Manual quickstart validation complements but does not replace API, store, runtime, contract, and restart coverage.
