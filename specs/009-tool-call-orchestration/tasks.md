---

description: "Task list for Tool-Call Orchestration"

---

# Tasks: Tool-Call Orchestration

**Input**: Design documents from `/specs/009-tool-call-orchestration/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This roadmap changes runtime execution coordination,
API/schema/event contracts, SQLite persistence, restart behavior, and operator-visible
audit surfaces, so targeted daemon tests and contract verification are required.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependency)
- **[Story]**: Which user story this belongs to (`US1`, `US2`, `US3`)
- Include exact file paths in descriptions

## Path Conventions

- Daemon code lives under `daemon/internal/...`
- API and event schemas live under `schemas/api/` and `schemas/events/`
- Feature artifacts live under `specs/009-tool-call-orchestration/`
- Roadmap and operator docs live under `docs/harness/` and `docs/runtime/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable workflow fixtures, inspection helpers, and contract
assertions for Roadmap 24

- [X] T001 Create reusable workflow fixture builders, dependency-graph helpers, and retry-state assertions in `daemon/internal/orchestration/manager_test.go`
- [X] T002 [P] Add reusable API assertion helpers for workflow resources, workflow-step linkage, and interruption status in `daemon/internal/api/server_test.go`
- [X] T003 [P] Add reusable contract assertion helpers for workflow resource and workflow event payloads in `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish shared orchestration types, persistence, and contract scaffolding
required by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Define workflow, workflow-step, dependency-edge, handoff, and runtime-linkage types in `daemon/internal/orchestration/types.go` and `daemon/internal/api/types.go`
- [X] T005 Implement additive environment-scoped workflow persistence primitives, workflow query filtering, and restart-interruption scaffolding in `daemon/internal/store/store.go` and `daemon/internal/app/app.go`
- [X] T006 [P] Add base API schema scaffolding for workflow planning and inspection in `schemas/api/create-workflow.request.schema.json`, `schemas/api/workflow-resource.schema.json`, `schemas/api/workflow-list.response.schema.json`, `schemas/api/workflow-step-resource.schema.json`, `schemas/api/workflow-dependency-resource.schema.json`, and `schemas/api/workflow-handoff-resource.schema.json`
- [X] T007 [P] Add base workflow event schema scaffolding in `schemas/events/workflow-planned.event.schema.json`, `schemas/events/workflow-started.event.schema.json`, `schemas/events/workflow-status-changed.event.schema.json`, and `schemas/events/workflow-step-status-changed.event.schema.json`
- [X] T008 [P] Add additive workflow linkage fields to runtime types and schema surfaces in `daemon/internal/runtime/runtime.go`, `schemas/api/run-resource.schema.json`, `schemas/api/step-resource.schema.json`, and `schemas/api/tool-call-resource.schema.json`

**Checkpoint**: Shared orchestration primitives and contract scaffolding are ready for
story work

---

## Phase 3: User Story 1 - Inspect Workflow Planning Truth (Priority: P1) 🎯 MVP

**Goal**: Let operators submit a workflow goal and inspect an explicit, auditable, and
frozen workflow plan before execution begins

**Independent Test**: Create a workflow from a run goal, inspect the workflow resource,
and verify selected steps, dependency order, rationale, handoff intent, and expected
approval mode are visible without any runtime step or tool call having started.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T009 [P] [US1] Add orchestration planner unit tests for goal-driven workflow generation, bounded dependency graphs, planning-failure truth, and `<=2 s` plan creation on local test fixtures in `daemon/internal/orchestration/manager_test.go`
- [X] T010 [P] [US1] Add API regression tests for `POST /v1/runs/{runId}/workflows`, `GET /v1/runs/{runId}/workflows`, `GET /v1/runs/{runId}/workflows/{workflowId}`, environment-scoped workflow inspection, and `<=500 ms` workflow detail retrieval in `daemon/internal/api/server_test.go`
- [X] T011 [P] [US1] Add contract regression coverage for workflow planning request/list/detail responses and `workflow.planned` events in `daemon/internal/contracts/contracts_test.go`, `schemas/api/create-workflow.request.schema.json`, `schemas/api/workflow-resource.schema.json`, `schemas/api/workflow-list.response.schema.json`, and `schemas/events/workflow-planned.event.schema.json`

### Implementation for User Story 1

- [X] T012 [US1] Implement goal-driven workflow planning, immutable persisted plan creation, and `workflow.planned` event emission in `daemon/internal/orchestration/manager.go`
- [X] T013 [US1] Add workflow create/list/get HTTP routes and workflow response shaping in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [X] T014 [US1] Persist selection rationale, dependency edges, handoff summaries, and expected approval metadata in `daemon/internal/store/store.go` and `daemon/internal/orchestration/types.go`
- [X] T015 [P] [US1] Update operator and roadmap docs for inspect-before-start workflow planning in `docs/harness/harness-architecture.md`, `docs/runtime/daemon-roadmaps.md`, and `docs/runtime/daemon-api-and-event-model.md`

**Checkpoint**: Operators can explain why a workflow was planned and what approvals it
may require before any execution begins

---

## Phase 4: User Story 2 - Execute A Controlled Multi-Step Workflow (Priority: P2)

**Goal**: Execute a frozen multi-step workflow through the existing runtime step and
tool-call plane with explicit retry, cancellation, and partial-failure truth

**Independent Test**: Start a planned workflow, observe normal runtime steps and tool
calls for each executing workflow step, trigger one retry and one cancellation or
partial-failure path, and confirm workflow history remains consistent with concrete
runtime history.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T016 [P] [US2] Add orchestration execution unit tests for sequential scheduling, bounded dependency readiness, frozen-plan enforcement, bounded retry transitions, and `<=100 ms` ready-step scheduling overhead on local test fixtures in `daemon/internal/orchestration/manager_test.go`
- [X] T017 [P] [US2] Add API and app regression tests for workflow start, workflow cancel, runtime linkage, partial-failure recording, cancellation truth, environment-scoped workflow execution visibility, and legacy non-workflow tool-call behavior in `daemon/internal/api/server_test.go` and `daemon/internal/app/app_test.go`
- [X] T018 [P] [US2] Add contract regression coverage for workflow start/status/step-status events, additive runtime linkage fields, and backward-compatible single-step tool-call contracts in `daemon/internal/contracts/contracts_test.go`, `schemas/events/workflow-started.event.schema.json`, `schemas/events/workflow-status-changed.event.schema.json`, `schemas/events/workflow-step-status-changed.event.schema.json`, `schemas/api/run-resource.schema.json`, `schemas/api/step-resource.schema.json`, and `schemas/api/tool-call-resource.schema.json`

### Implementation for User Story 2

- [X] T019 [US2] Implement workflow start/cancel orchestration state machine and frozen-plan execution in `daemon/internal/orchestration/manager.go`
- [X] T020 [US2] Wire workflow execution onto existing runtime steps and tool calls in `daemon/internal/orchestration/manager.go`, `daemon/internal/runtime/runtime.go`, and `daemon/internal/api/server.go`
- [X] T021 [US2] Implement bounded step retry, blocked-state propagation, cancellation handling, and partial-failure recording in `daemon/internal/orchestration/manager.go` and `daemon/internal/store/store.go`
- [X] T022 [P] [US2] Publish workflow started, workflow status-changed, and workflow step-status-changed events with runtime linkage in `daemon/internal/orchestration/manager.go` and `daemon/internal/api/server.go`
- [X] T023 [P] [US2] Update runtime API and verification docs for workflow execution, retry, partial failure, and cancellation behavior in `docs/runtime/daemon-api-and-event-model.md` and `specs/009-tool-call-orchestration/quickstart.md`

**Checkpoint**: A planned workflow can execute through the existing runtime plane with
operator-visible retry, cancellation, and partial-failure truth

---

## Phase 5: User Story 3 - Coordinate Mixed Tool Families Safely (Priority: P3)

**Goal**: Support mixed MCP, local-tool, and executable-skill workflows with explicit
handoffs, distinct blocked outcomes, and restart-safe interruption truth

**Independent Test**: Run at least one mixed workflow in `KURA_ENV=test` using two
consumer families, inspect handoff records and workflow-step linkage, then trigger an
approval or availability block and a daemon restart to verify blocked and interrupted
states remain explicit.

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T024 [P] [US3] Add orchestration unit tests for mixed-family dependency resolution, handoff visibility, and blocked-step classification in `daemon/internal/orchestration/manager_test.go`
- [X] T025 [P] [US3] Add API and app regression tests for mixed MCP plus local-tool or skill workflows, daemon-restart interruption truth, and no cross-environment workflow leakage in `daemon/internal/api/server_test.go` and `daemon/internal/app/app_test.go`
- [X] T026 [P] [US3] Add contract regression coverage for workflow handoff projection, blocked/interrupted statuses, and mixed-family runtime linkage in `daemon/internal/contracts/contracts_test.go`, `schemas/api/workflow-resource.schema.json`, `schemas/api/workflow-step-resource.schema.json`, `schemas/api/workflow-handoff-resource.schema.json`, `schemas/events/workflow-status-changed.event.schema.json`, and `schemas/events/workflow-step-status-changed.event.schema.json`

### Implementation for User Story 3

- [X] T027 [US3] Implement mixed-family consumer selection, handoff recording, and dependency readiness evaluation in `daemon/internal/orchestration/manager.go`
- [X] T028 [US3] Surface approval-denied, policy-blocked, and consumer-unavailable outcomes distinctly on workflow-step records and API projections in `daemon/internal/orchestration/manager.go` and `daemon/internal/api/types.go`
- [X] T029 [US3] Mark in-flight workflows interrupted during daemon restart restore while preserving completed-step truth in `daemon/internal/app/app.go`, `daemon/internal/store/store.go`, and `daemon/internal/orchestration/manager.go`
- [X] T030 [P] [US3] Update workflow schemas and event payloads for handoffs, interrupted status, and mixed-family linkage in `schemas/api/workflow-resource.schema.json`, `schemas/api/workflow-step-resource.schema.json`, `schemas/api/workflow-handoff-resource.schema.json`, `schemas/events/workflow-status-changed.event.schema.json`, and `schemas/events/workflow-step-status-changed.event.schema.json`
- [X] T031 [P] [US3] Update architecture and operator-trust docs for mixed-family guarantees and restart interruption semantics in `docs/harness/harness-architecture.md`, `docs/runtime/operator-trust-model.md`, and `docs/runtime/daemon-roadmaps.md`

**Checkpoint**: Mixed-family workflows remain policy-safe, handoff-visible, and
restart-auditable without any execution bypass path

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, documentation closure, and roadmap-completion evidence

- [X] T032 [P] Refresh feature design artifacts to match implemented workflow planning, execution, and interruption closure in `specs/009-tool-call-orchestration/research.md`, `specs/009-tool-call-orchestration/data-model.md`, and `specs/009-tool-call-orchestration/contracts/tool-call-orchestration-surfaces.md`
- [X] T033 [P] Run `make daemon-contract-test` and record results in `specs/009-tool-call-orchestration/quickstart.md`
- [X] T034 [P] Run targeted daemon verification in `daemon/internal/orchestration`, `daemon/internal/api`, `daemon/internal/runtime`, `daemon/internal/store`, `daemon/internal/app`, and `daemon/internal/contracts`, including legacy single-step regression, environment-scope isolation, and stated local timing targets, then record results in `specs/009-tool-call-orchestration/quickstart.md`
- [X] T035 [P] Run full daemon regression verification with `go test ./...` in `daemon/` and record results in `specs/009-tool-call-orchestration/quickstart.md`
- [X] T036 [P] Execute the manual `KURA_ENV=test` mixed-workflow operator acceptance flow from `specs/009-tool-call-orchestration/quickstart.md` and record whether planning inspection completes within `<=5 min`, mixed execution succeeds without bypass paths, and existing non-workflow tool calls remain unaffected in `specs/009-tool-call-orchestration/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational; establishes workflow planning truth and base workflow routes
- **User Story 2 (Phase 4)**: Depends on User Story 1 because execution must build on finalized workflow resources, planning output, and runtime linkage contracts
- **User Story 3 (Phase 5)**: Depends on User Story 2 because mixed-family coordination and interruption truth require the core execution state machine and linkage surfaces
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on later stories
- **User Story 2 (P2)**: Depends on User Story 1 because the daemon must have inspectable persisted workflow plans before it can execute them
- **User Story 3 (P3)**: Depends on User Story 2 because mixed-family coordination and restart interruption semantics require a working workflow execution layer

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Types and persistence scaffolding before route and event closure
- Planning before execution
- Execution before mixed-family coordination and restart interruption
- Story checkpoint must pass before moving to the next dependent story

### Parallel Opportunities

- `T002` and `T003` can run in parallel
- `T006`, `T007`, and `T008` can run in parallel
- `T009`, `T010`, and `T011` can run in parallel
- `T015` can run in parallel with late-stage US1 implementation once route shapes stabilize
- `T016`, `T017`, and `T018` can run in parallel
- `T022` and `T023` can run in parallel after workflow execution behavior stabilizes
- `T024`, `T025`, and `T026` can run in parallel
- `T030` and `T031` can run in parallel after mixed-family behavior stabilizes
- `T032`, `T033`, `T034`, `T035`, and `T036` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 2

```bash
# Launch all tests for User Story 2 together:
Task: "Add orchestration execution unit tests for sequential scheduling, bounded dependency readiness, frozen-plan enforcement, and bounded retry transitions in daemon/internal/orchestration/manager_test.go"
Task: "Add API and app regression tests for workflow start, workflow cancel, runtime linkage, partial-failure recording, and cancellation truth in daemon/internal/api/server_test.go and daemon/internal/app/app_test.go"
Task: "Add contract regression coverage for workflow start/status/step-status events and additive runtime linkage fields in daemon/internal/contracts/contracts_test.go and schemas/events/, schemas/api/"

# Launch implementation work on different files in parallel:
Task: "Implement bounded step retry, blocked-state propagation, cancellation handling, and partial-failure recording in daemon/internal/orchestration/manager.go and daemon/internal/store/store.go"
Task: "Publish workflow started, workflow status-changed, and workflow step-status-changed events with runtime linkage in daemon/internal/orchestration/manager.go and daemon/internal/api/server.go"
Task: "Update runtime API and verification docs for workflow execution, retry, partial failure, and cancellation behavior in docs/runtime/daemon-api-and-event-model.md and specs/009-tool-call-orchestration/quickstart.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Verify planning inspection and planning-failure truth independently
5. Continue only after the inspect-before-start workflow contract is stable

### Incremental Delivery

1. Complete Setup + Foundational → workflow primitives and contracts ready
2. Add User Story 1 → validate goal-driven planning and inspection
3. Add User Story 2 → validate execution, retry, cancellation, and partial failure
4. Add User Story 3 → validate mixed-family handoffs, blocked outcomes, and restart interruption
5. Claim roadmap closure only after all story phases and final verification complete

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 planning routes and workflow resources
   - Developer B: User Story 2 execution state machine and runtime linkage after US1 contracts stabilize
   - Developer C: User Story 3 mixed-family handoffs and interruption semantics after US2 execution behavior stabilizes
3. Finish with shared contract, regression, and manual verification in Polish

---

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] label maps each story task to a user story for traceability
- Each user story should be independently completable and testable
- Verify required tests fail before implementing
- Avoid vague tasks, hidden compatibility work, same-file parallel conflicts, and any orchestration path that bypasses the existing runtime step and tool-call plane
- Tasks are now fully checked off against the implemented workflow planning,
  execution, mixed-family, and interruption coverage.
