# Tasks: Computer-Use Capability Plane

**Input**: Design documents from `/specs/011-computer-use-plane/`  
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/computer-use-surfaces.md](./contracts/computer-use-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes API, schema, event, persistence, restart, artifact, and execution-boundary behavior, so targeted unit, integration, contract, and restart coverage is required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the feature scaffolding and shared file layout for browser session, action, and artifact work.

- [X] T001 Create computer-use package scaffolding in `daemon/internal/computeruse/types.go`, `daemon/internal/computeruse/manager.go`, and `daemon/internal/computeruse/driver.go`
- [X] T002 [P] Create computer-use API schema placeholders in `schemas/api/create-computer-use-session.request.schema.json`, `schemas/api/computer-use-session-resource.schema.json`, `schemas/api/computer-use-session-list.response.schema.json`, `schemas/api/create-computer-use-action.request.schema.json`, `schemas/api/computer-use-action-resource.schema.json`, `schemas/api/computer-use-action-list.response.schema.json`, `schemas/api/computer-use-artifact-resource.schema.json`, and `schemas/api/computer-use-artifact-content.response.schema.json`
- [X] T003 [P] Create computer-use event schema placeholders in `schemas/events/computer-use-session-created.event.schema.json`, `schemas/events/computer-use-session-status-changed.event.schema.json`, `schemas/events/computer-use-action-requested.event.schema.json`, `schemas/events/computer-use-action-status-changed.event.schema.json`, `schemas/events/computer-use-action-target-mismatch.event.schema.json`, and `schemas/events/computer-use-artifact-recorded.event.schema.json`
- [X] T004 [P] Create computer-use API handler scaffolding in `daemon/internal/api/computer_use.go` and route registration stubs in `daemon/internal/api/server.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the shared persistence, runtime linkage, artifact plumbing, and app wiring that block all user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Add SQLite migration and store record types for `computer_use_sessions`, `computer_use_actions`, and `computer_use_artifacts` in `daemon/internal/store/store.go`
- [X] T006 [P] Extend runtime and persisted tool-call linkage for `computerUseSessionId` and `computerUseActionId` in `daemon/internal/runtime/runtime.go` and `daemon/internal/store/store.go`
- [X] T007 [P] Add shared computer-use request and response structs plus route helpers in `daemon/internal/api/types.go` and `daemon/internal/api/server.go`
- [X] T008 [P] Expand shared artifact metadata and content plumbing in `daemon/internal/artifacts/service.go` and `daemon/internal/store/store.go`
- [X] T009 [P] Update runtime event scope fields for `computerUseSessionId` and `computerUseActionId` in `schemas/events/runtime-event.schema.json` and `daemon/internal/events/bus.go`
- [X] T010 [P] Add computer-use schema validator and contract coverage in `daemon/internal/contracts/validator.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T011 Wire computer-use manager startup, dependency injection, and restart interruption hooks into `daemon/internal/app/app.go` and `daemon/internal/api/server.go`
- [X] T012 [P] Add environment-scoped isolation regressions for sessions, actions, and artifacts in `daemon/internal/store/store_test.go` and `daemon/internal/api/server_test.go`

**Checkpoint**: Persistence, runtime linkage, artifact plumbing, and app wiring are ready; user story work can now proceed.

---

## Phase 3: User Story 1 - Inspect And Approve Browser Actions (Priority: P1) 🎯 MVP

**Goal**: Operators can open a run-scoped browser session, inspect current page context, approve or deny high-risk browser actions, and see minimal linked evidence on the approved path.

**Independent Test**: Start a browser session under a normal run, execute a low-risk navigate action, request a high-risk input or click action, inspect the pending action, deny it once, approve it once, and confirm the approved path records linked evidence history through session, action, and tool-call truth.

### Tests for User Story 1

- [X] T013 [P] [US1] Add session create/list/detail and close API contract tests in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T014 [P] [US1] Add computer-use manager and runtime tests for single-page session lifecycle, low-risk navigate execution, and explicit rejection of extra tabs or unsupported desktop-style actions in `daemon/internal/computeruse/manager_test.go` and `daemon/internal/runtime/runtime_test.go`
- [X] T015 [P] [US1] Add approval-gated action inspection, approval, denial, and minimal evidence-history regressions in `daemon/internal/api/server_test.go` and `daemon/internal/computeruse/manager_test.go`

### Implementation for User Story 1

- [X] T016 [P] [US1] Implement session create/list/get/close route handlers in `daemon/internal/api/computer_use.go` and `daemon/internal/api/server.go`
- [X] T017 [P] [US1] Implement run-scoped browser session lifecycle and single-page core actions in `daemon/internal/computeruse/manager.go` and `daemon/internal/computeruse/types.go`
- [X] T018 [P] [US1] Implement the phase 26 browser driver interface and deterministic test driver in `daemon/internal/computeruse/driver.go` and `daemon/internal/computeruse/driver_test.go`
- [X] T019 [US1] Implement action-scoped approval requests and inspect-before-act page context in `daemon/internal/api/computer_use.go`, `daemon/internal/api/server.go`, and `daemon/internal/computeruse/manager.go`
- [X] T020 [US1] Enforce explicit rejection for additional tabs, windows, and unsupported desktop-style requests in `daemon/internal/computeruse/manager.go`, `daemon/internal/computeruse/types.go`, and `daemon/internal/api/computer_use.go`
- [X] T021 [US1] Persist session and action history plus run-scoped reuse rules in `daemon/internal/store/store.go` and `daemon/internal/computeruse/manager.go`
- [X] T022 [US1] Capture and persist minimal screenshot or page-snapshot evidence for successful inspected actions in `daemon/internal/artifacts/service.go`, `daemon/internal/computeruse/manager.go`, and `daemon/internal/store/store.go`
- [X] T023 [US1] Project computer-use linkage and evidence summaries onto session, action, and tool-call responses in `daemon/internal/api/computer_use.go`, `daemon/internal/runtime/runtime.go`, `daemon/internal/api/types.go`, `schemas/api/computer-use-session-resource.schema.json`, `schemas/api/computer-use-action-resource.schema.json`, `schemas/api/tool-call-resource.schema.json`, and `schemas/api/tool-call-list.response.schema.json`
- [X] T024 [US1] Publish session and action lifecycle events in `daemon/internal/computeruse/manager.go`, `daemon/internal/events/bus.go`, `schemas/events/computer-use-session-created.event.schema.json`, `schemas/events/computer-use-session-status-changed.event.schema.json`, `schemas/events/computer-use-action-requested.event.schema.json`, and `schemas/events/computer-use-action-status-changed.event.schema.json`

**Checkpoint**: User Story 1 is complete when operators can inspect and approve or deny browser actions with run-scoped session, tool-call truth, and minimal evidence history.

---

## Phase 4: User Story 2 - Run Computer Use Inside Normal Workflows (Priority: P2)

**Goal**: Computer-use steps run inside normal runs and workflows, reuse sessions only within the owning run or workflow, and preserve backward-compatible non-computer-use execution.

**Independent Test**: Execute one `KURA_ENV=test` workflow that combines a computer-use step with another capability family, confirm the workflow exposes session, action, and linked evidence summaries through normal step and tool-call truth, and verify a schedule-owned or operator-owned run does not leak session reuse across boundaries.

### Tests for User Story 2

- [X] T025 [P] [US2] Add workflow and mixed-capability computer-use integration tests, including workflow-visible evidence summaries, in `daemon/internal/api/workflows_test.go` and `daemon/internal/api/server_test.go`
- [X] T026 [P] [US2] Add schedule-owned run and in-run session reuse regressions in `daemon/internal/api/schedules_test.go` and `daemon/internal/app/app_test.go`

### Implementation for User Story 2

- [X] T027 [P] [US2] Extend workflow step typing and workflow manager support for computer-use consumers in `daemon/internal/orchestration/types.go` and `daemon/internal/orchestration/manager.go`
- [X] T028 [P] [US2] Implement workflow-visible session, action, and evidence linkage projections in `daemon/internal/api/workflows.go`, `daemon/internal/api/types.go`, and `schemas/api/workflow-step-resource.schema.json`
- [X] T029 [US2] Implement run or workflow-owned session reuse boundaries and workflow action dispatch in `daemon/internal/computeruse/manager.go` and `daemon/internal/api/computer_use.go`
- [X] T030 [US2] Integrate computer-use execution with schedule-launched and operator-launched run ownership in `daemon/internal/app/app.go`, `daemon/internal/api/schedule_workflow_launcher.go`, and `daemon/internal/api/schedules.go`
- [X] T031 [US2] Preserve backward-compatible non-computer-use runtime flows while adding computer-use execution branching in `daemon/internal/api/server.go` and `daemon/internal/runtime/runtime.go`

**Checkpoint**: User Story 2 is complete when workflows and run-launched browser actions stay on the normal runtime plane with no hidden executor or cross-run session reuse.

---

## Phase 5: User Story 3 - Diagnose Outcomes With Evidence (Priority: P3)

**Goal**: Operators can inspect screenshots, snapshots, downloads, and distinct failure classes, including target mismatch and interruption outcomes, after browser work finishes or fails.

**Independent Test**: Exercise successful evidence capture, policy denial, unavailable-consumer failure, navigation failure, target mismatch, and restart interruption, then confirm each outcome is distinguishable and artifact-backed through operator-visible routes and events.

### Tests for User Story 3

- [X] T032 [P] [US3] Add artifact metadata/content contract tests and failure-class API coverage in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T033 [P] [US3] Add target-mismatch, unavailable-consumer, and restart interruption regressions in `daemon/internal/computeruse/manager_test.go`, `daemon/internal/app/app_test.go`, and `daemon/internal/store/store_test.go`

### Implementation for User Story 3

- [X] T034 [P] [US3] Implement target-match evaluation and immediate mismatch failure recording in `daemon/internal/computeruse/manager.go` and `daemon/internal/computeruse/types.go`
- [X] T035 [P] [US3] Implement artifact metadata persistence and content retrieval for screenshots, snapshots, and downloads in `daemon/internal/artifacts/service.go`, `daemon/internal/api/computer_use.go`, and `daemon/internal/store/store.go`
- [X] T036 [US3] Implement explicit failure-class mapping for policy denial, navigation failure, unavailable consumer, and target mismatch in `daemon/internal/computeruse/manager.go`, `daemon/internal/runtime/runtime.go`, and `schemas/api/computer-use-action-resource.schema.json`
- [X] T037 [US3] Implement restart-safe session and action interruption recovery plus post-restart artifact inspection in `daemon/internal/app/app.go` and `daemon/internal/store/store.go`
- [X] T038 [US3] Finalize artifact and action contract surfaces and publish target-mismatch and artifact-recorded events in `daemon/internal/computeruse/manager.go`, `daemon/internal/events/bus.go`, `daemon/internal/contracts/contracts_test.go`, `schemas/api/computer-use-artifact-resource.schema.json`, `schemas/api/computer-use-artifact-content.response.schema.json`, `schemas/api/computer-use-action-resource.schema.json`, `schemas/api/computer-use-action-list.response.schema.json`, `schemas/events/computer-use-action-target-mismatch.event.schema.json`, and `schemas/events/computer-use-artifact-recorded.event.schema.json`

**Checkpoint**: User Story 3 is complete when operators can reconstruct browser outcomes and evidence without raw logs.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish docs, performance verification, and recorded validation for the complete capability plane.

- [X] T039 [P] Update browser-first capability docs and operator guidance in `docs/runtime/operator-trust-model.md`, `docs/runtime/daemon-roadmaps.md`, and `docs/harness/harness-architecture.md`
- [X] T040 [P] Add performance coverage for session lookup, action completion, and artifact metadata latency in `daemon/internal/api/server_test.go` and `daemon/internal/computeruse/manager_test.go`
- [X] T041 [P] Finalize schema fixture and validator coverage for `schemas/events/runtime-event.schema.json`, `schemas/api/create-computer-use-session.request.schema.json`, `schemas/api/computer-use-session-resource.schema.json`, `schemas/api/computer-use-session-list.response.schema.json`, `schemas/api/create-computer-use-action.request.schema.json`, `schemas/api/computer-use-action-resource.schema.json`, `schemas/api/computer-use-action-list.response.schema.json`, `schemas/api/computer-use-artifact-resource.schema.json`, `schemas/api/computer-use-artifact-content.response.schema.json`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/tool-call-list.response.schema.json`, `schemas/api/workflow-step-resource.schema.json`, `schemas/events/computer-use-session-created.event.schema.json`, `schemas/events/computer-use-session-status-changed.event.schema.json`, `schemas/events/computer-use-action-requested.event.schema.json`, `schemas/events/computer-use-action-status-changed.event.schema.json`, `schemas/events/computer-use-action-target-mismatch.event.schema.json`, and `schemas/events/computer-use-artifact-recorded.event.schema.json` in `daemon/internal/contracts/contracts_test.go`
- [X] T042 [P] Run manual `KURA_ENV=test` browser verification and record observed results in `specs/011-computer-use-plane/quickstart.md`
- [X] T043 Record automated verification commands, residual risks, and rollback notes in `specs/011-computer-use-plane/plan.md` and `specs/011-computer-use-plane/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all story work.
- **Phase 3: US1**: Depends on Phase 2; establishes the MVP browser session, approval, rejection, and minimal evidence surface.
- **Phase 4: US2**: Depends on Phase 2 and reuses the core session/action resources landed in US1.
- **Phase 5: US3**: Depends on Phase 2 and builds on the session/action execution path from US1 to add richer evidence, failure truth, and restart handling.
- **Phase 6: Polish**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on other user stories.
- **US2 (P2)**: Builds on US1 session, action, and minimal evidence resources but remains independently testable once workflow linkage is implemented.
- **US3 (P3)**: Builds on US1 execution truth and can be delivered after US2 or in parallel once the foundational session/action model is stable.

### Within Each User Story

- Tests and contract coverage land before or alongside implementation and must fail before the behavior is complete.
- Store and type changes precede API projections that depend on persisted truth.
- Computer-use manager and driver work precede route handlers that expose the resulting state.
- Story-specific docs and recorded validation happen only after the story behavior is functional.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together.
- In Foundational, runtime linkage, API typing, event-scope updates, artifact plumbing, and validator work can proceed in parallel.
- For each user story, API or contract tests and manager/store tests can be written in parallel.
- Within stories, persistence work and route serialization work can proceed in parallel before the final integration task.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T013 [US1] Add session create/list/detail and close API contract tests in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T014 [US1] Add computer-use manager and runtime tests for single-page session lifecycle, low-risk navigate execution, and explicit rejection of extra tabs or unsupported desktop-style actions in daemon/internal/computeruse/manager_test.go and daemon/internal/runtime/runtime_test.go"

# Implementation in parallel
Task: "T016 [US1] Implement session create/list/get/close route handlers in daemon/internal/api/computer_use.go and daemon/internal/api/server.go"
Task: "T018 [US1] Implement the phase 26 browser driver interface and deterministic test driver in daemon/internal/computeruse/driver.go and daemon/internal/computeruse/driver_test.go"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T025 [US2] Add workflow and mixed-capability computer-use integration tests, including workflow-visible evidence summaries, in daemon/internal/api/workflows_test.go and daemon/internal/api/server_test.go"
Task: "T026 [US2] Add schedule-owned run and in-run session reuse regressions in daemon/internal/api/schedules_test.go and daemon/internal/app/app_test.go"

# Implementation in parallel
Task: "T027 [US2] Extend workflow step typing and workflow manager support for computer-use consumers in daemon/internal/orchestration/types.go and daemon/internal/orchestration/manager.go"
Task: "T028 [US2] Implement workflow-visible session, action, and evidence linkage projections in daemon/internal/api/workflows.go, daemon/internal/api/types.go, and schemas/api/workflow-step-resource.schema.json"
```

## Parallel Example: User Story 3

```bash
# Tests in parallel
Task: "T032 [US3] Add artifact metadata/content contract tests and failure-class API coverage in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T033 [US3] Add target-mismatch, unavailable-consumer, and restart interruption regressions in daemon/internal/computeruse/manager_test.go, daemon/internal/app/app_test.go, and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T034 [US3] Implement target-match evaluation and immediate mismatch failure recording in daemon/internal/computeruse/manager.go and daemon/internal/computeruse/types.go"
Task: "T035 [US3] Implement artifact metadata persistence and content retrieval for screenshots, snapshots, and downloads in daemon/internal/artifacts/service.go, daemon/internal/api/computer_use.go, and daemon/internal/store/store.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate browser session creation, inspect-before-act approval, denial, explicit rejection of unsupported browser surface expansion, and minimal evidence history in `KURA_ENV=test`.
5. Stop if only the MVP computer-use safety surface is needed.

### Incremental Delivery

1. Land Setup + Foundational to establish store, runtime linkage, event-scope updates, artifact plumbing, and contract scaffolding.
2. Deliver US1 for run-scoped browser sessions, approval-gated actions, explicit unsupported-surface rejection, and minimal evidence history.
3. Deliver US2 for workflow and run integration without hidden execution paths.
4. Deliver US3 for full artifact retrieval, failure distinctions, and restart interruption truth.
5. Finish with docs, performance checks, and recorded verification.

### Parallel Team Strategy

1. One engineer lands store, runtime, event-scope, and app wiring in Setup + Foundational.
2. After Foundational is complete:
   - Engineer A takes US1 API, approval, core session lifecycle, and minimal evidence.
   - Engineer B takes US2 workflow and run ownership integration.
   - Engineer C takes US3 richer artifacts, failure truth, and restart recovery once US1 state models are stable.

## Notes

- `[P]` means the task can run in parallel because it targets different files or only depends on completed foundational work.
- Every user story has explicit tests because this roadmap changes operator-visible behavior and contract-backed surfaces.
- Existing non-computer-use run, workflow, and tool-call flows must remain backward compatible throughout implementation.
- Do not treat manual quickstart validation as a substitute for API, store, runtime, contract, and restart coverage.
