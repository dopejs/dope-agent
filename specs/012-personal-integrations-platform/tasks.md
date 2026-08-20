# Tasks: Personal Integrations Platform

**Input**: Design documents from `/specs/012-personal-integrations-platform/`  
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/integration-platform-surfaces.md](./contracts/integration-platform-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes API, schema, event, persistence, runtime, approval, and restart surfaces, so targeted unit, integration, contract, and restart verification is required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the feature scaffolding and shared file layout for daemon-owned integration resources and fake-backend probe work.

- [X] T001 Create integrations package scaffolding in `daemon/internal/integrations/types.go`, `daemon/internal/integrations/manager.go`, `daemon/internal/integrations/backend.go`, and `daemon/internal/integrations/fake_backend.go`
- [X] T002 [P] Create integration API schema placeholders in `schemas/api/create-integration.request.schema.json`, `schemas/api/integration-resource.schema.json`, `schemas/api/integration-list.response.schema.json`, `schemas/api/report-integration-readiness.request.schema.json`, `schemas/api/set-integration-default.request.schema.json`, `schemas/api/create-integration-probe.request.schema.json`, and `schemas/api/integration-binding-summary.schema.json`
- [X] T003 [P] Create integration event schema placeholders in `schemas/events/integration-registered.event.schema.json`, `schemas/events/integration-updated.event.schema.json`, `schemas/events/integration-readiness-changed.event.schema.json`, and `schemas/events/integration-default-changed.event.schema.json`
- [X] T004 [P] Create integration API handler scaffolding in `daemon/internal/api/integrations.go` and route registration stubs in `daemon/internal/api/server.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land the shared persistence, manager wiring, fake-backend abstraction, and validation scaffolding that block all user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Add SQLite migration and store record types for integration resources and binding snapshot persistence in `daemon/internal/store/store.go`
- [X] T006 [P] Add shared integration resource, account binding, backend binding, and binding-summary structs in `daemon/internal/integrations/types.go` and `daemon/internal/api/types.go`
- [X] T007 [P] Implement store helpers for integration upsert/list/get and canonical-default group queries in `daemon/internal/store/store.go`
- [X] T008 [P] Wire integrations manager startup, dependency injection, and persisted restore hooks into `daemon/internal/app/app.go` and `daemon/internal/api/server.go`
- [X] T009 [P] Add fake integration backend interface and deterministic fake backend scaffolding in `daemon/internal/integrations/backend.go` and `daemon/internal/integrations/fake_backend.go`
- [X] T010 [P] Add integration schema validator coverage scaffolding in `daemon/internal/contracts/validator.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T011 [P] Add shared redacted provenance and secret-resolution helpers for integrations in `daemon/internal/integrations/provenance.go` and `daemon/internal/api/server.go`
- [X] T012 [P] Add foundational integration manager and restart persistence regressions in `daemon/internal/integrations/manager_test.go`, `daemon/internal/store/store_test.go`, and `daemon/internal/app/app_test.go`

**Checkpoint**: Integration persistence, manager wiring, fake-backend abstraction, and contract scaffolding are ready; user story work can now proceed.

---

## Phase 3: User Story 1 - Inspect Integration Readiness (Priority: P1) 🎯 MVP

**Goal**: Operators can create, inspect, and update daemon-owned integration resources with explicit readiness, account binding, provenance, and canonical-default truth.

**Independent Test**: Create two integration records for the same account in `KURA_ENV=test`, move them through `not_configured`, `auth_pending`, `healthy`, `degraded`, and `unavailable`, promote one as canonical default, and confirm list/detail routes expose readiness, account identity, environment scope, and provenance without raw config access.

### Tests for User Story 1

- [X] T013 [P] [US1] Add integration create/list/detail/readiness/default API contract coverage in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T014 [P] [US1] Add manager and store regressions for readiness transitions, duplicate account groups, and canonical-default replacement in `daemon/internal/integrations/manager_test.go` and `daemon/internal/store/store_test.go`

### Implementation for User Story 1

- [X] T015 [P] [US1] Implement integration CRUD, readiness updates, and canonical-default logic in `daemon/internal/integrations/manager.go` and `daemon/internal/integrations/types.go`
- [X] T016 [P] [US1] Implement integration create/list/get/readiness/default route handlers in `daemon/internal/api/integrations.go` and `daemon/internal/api/server.go`
- [X] T017 [US1] Persist account binding, backend binding, readiness, and canonical-default state in `daemon/internal/store/store.go` and `daemon/internal/integrations/manager.go`
- [X] T018 [US1] Project integration request and response shapes in `daemon/internal/api/types.go`, `schemas/api/create-integration.request.schema.json`, `schemas/api/integration-resource.schema.json`, `schemas/api/integration-list.response.schema.json`, `schemas/api/report-integration-readiness.request.schema.json`, and `schemas/api/set-integration-default.request.schema.json`
- [X] T019 [US1] Publish integration registered, updated, readiness-changed, and default-changed events in `daemon/internal/integrations/manager.go`, `daemon/internal/events/bus.go`, `schemas/events/integration-registered.event.schema.json`, `schemas/events/integration-updated.event.schema.json`, `schemas/events/integration-readiness-changed.event.schema.json`, and `schemas/events/integration-default-changed.event.schema.json`
- [X] T020 [US1] Expose redacted provenance, required operator action, and degraded-versus-unavailable truth in `daemon/internal/integrations/provenance.go`, `daemon/internal/api/integrations.go`, and `schemas/api/integration-resource.schema.json`

**Checkpoint**: User Story 1 is complete when operators can inspect integration readiness and canonical-default truth entirely through operator-visible resources and events.

---

## Phase 4: User Story 2 - Run Personal-System Work On Shared Truth (Priority: P2)

**Goal**: Integration-backed verification work runs on the existing runtime, workflow,
and approval plane using fake integration probes, with redacted integration provenance
attached to runs, tool calls, workflow steps, and approvals.

**Independent Test**: In `KURA_ENV=test`, create a run, execute one read-only fake
integration probe and one approval-gated mutation probe against the canonical-default
integration, exercise the same binding projection through a representative workflow
execution path, then confirm runtime, workflow-step, approval, and tool-call truth
retain integration identity, readiness, approval, and redacted provenance without
domain-specific behavior.

### Tests for User Story 2

- [X] T021 [P] [US2] Add fake integration probe API and binding-summary contract tests in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T022 [P] [US2] Add degraded-versus-unavailable probe gating and approval regressions in `daemon/internal/integrations/manager_test.go`, `daemon/internal/api/server_test.go`, and `daemon/internal/runtime/runtime_test.go`
- [X] T023 [P] [US2] Add restart-safe probe linkage regressions in `daemon/internal/app/app_test.go` and `daemon/internal/store/store_test.go`
- [X] T040 [P] [US2] Add workflow-hosted integration probe regressions proving workflow-step bindings and approval linkage survive execution in `daemon/internal/api/workflows_test.go`, `daemon/internal/runtime/runtime_test.go`, and `daemon/internal/policy/policy_test.go`

### Implementation for User Story 2

- [X] T024 [P] [US2] Extend runtime tool-call models and persisted documents with `integrationBindings` in `daemon/internal/runtime/runtime.go` and `daemon/internal/store/store.go`
- [X] T025 [P] [US2] Extend workflow-step and approval resources with `integrationBindings` in `daemon/internal/orchestration/types.go`, `daemon/internal/policy/policy.go`, `schemas/api/workflow-step-resource.schema.json`, and `schemas/api/approval-resource.schema.json`
- [X] T026 [US2] Implement fake integration probe execution, readiness gating, and approval handoff in `daemon/internal/integrations/fake_backend.go` and `daemon/internal/integrations/manager.go`
- [X] T027 [US2] Implement run-scoped integration probe route handlers and runtime tool-call creation in `daemon/internal/api/integrations.go`, `daemon/internal/api/server.go`, and `schemas/api/create-integration-probe.request.schema.json`
- [X] T028 [US2] Attach immutable redacted integration-binding snapshots to tool calls, workflow steps, and approvals in `daemon/internal/integrations/bindings.go`, `daemon/internal/api/server.go`, `daemon/internal/api/workflows.go`, and `schemas/api/tool-call-resource.schema.json`
- [X] T029 [US2] Persist and restore probe-linked integration binding snapshots across restart in `daemon/internal/store/store.go` and `daemon/internal/app/app.go`
- [X] T041 [US2] Implement workflow-step integration binding propagation for integration-backed execution in `daemon/internal/api/workflows.go`, `daemon/internal/orchestration/manager.go`, `daemon/internal/runtime/runtime.go`, and `daemon/internal/integrations/bindings.go`

**Checkpoint**: User Story 2 is complete when fake integration probes execute through the
normal runtime, workflow, and approval plane with durable integration-binding truth and
explicit unavailable blocking.

---

## Phase 5: User Story 3 - Reuse One Integration Contract Across Domains (Priority: P3)

**Goal**: The integration substrate remains domain-agnostic and reusable across multiple domain kinds and backend kinds without redefining readiness, account binding, or provenance semantics.

**Independent Test**: Register representative calendar and mail integration resources backed by different backend kinds, inspect their resource and binding-summary projections, and confirm they share the same readiness vocabulary and downstream binding contract without domain-specific special cases.

### Tests for User Story 3

- [X] T030 [P] [US3] Add multi-domain and multi-backend contract regressions for integration resource normalization in `daemon/internal/contracts/contracts_test.go` and `daemon/internal/integrations/manager_test.go`
- [X] T031 [P] [US3] Add workflow and runtime regressions showing optional `integrationBindings` do not break non-integration behavior in `daemon/internal/api/workflows_test.go`, `daemon/internal/api/server_test.go`, and `daemon/internal/runtime/runtime_test.go`

### Implementation for User Story 3

- [X] T032 [P] [US3] Implement backend-binding normalization and domain-agnostic validation in `daemon/internal/integrations/backend.go` and `daemon/internal/integrations/manager.go`
- [X] T033 [P] [US3] Implement reusable integration binding snapshot builders for future domains in `daemon/internal/integrations/bindings.go` and `daemon/internal/integrations/types.go`
- [X] T034 [US3] Finalize generic integration binding summary and backend-binding schema surfaces in `schemas/api/integration-resource.schema.json`, `schemas/api/integration-binding-summary.schema.json`, `schemas/api/tool-call-resource.schema.json`, and `schemas/api/approval-resource.schema.json`
- [X] T035 [US3] Preserve backward-compatible non-integration runtime and workflow behavior while keeping `integrationBindings` optional in `daemon/internal/runtime/runtime.go`, `daemon/internal/api/server.go`, and `daemon/internal/api/workflows.go`

**Checkpoint**: User Story 3 is complete when the integration contract is demonstrably reusable across domain kinds and backend kinds without redefining readiness semantics or breaking existing non-integration flows.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish documentation, contract fixtures, and recorded validation for the complete integration substrate.

- [X] T036 [P] Update integration operator guidance and roadmap docs in `docs/runtime/operator-trust-model.md`, `docs/runtime/daemon-roadmaps.md`, `docs/harness/harness-architecture.md`, and `docs/runtime/daemon-api-and-event-model.md`
- [X] T037 [P] Finalize schema fixtures and validator coverage for all integration API and event surfaces in `daemon/internal/contracts/contracts_test.go` and `daemon/internal/contracts/validator.go`
- [X] T038 [P] Run the manual `KURA_ENV=test` fake integration walkthrough and record observed results in `specs/012-personal-integrations-platform/quickstart.md`
- [X] T039 Record automated verification commands, residual risks, and rollback notes in `specs/012-personal-integrations-platform/plan.md` and `specs/012-personal-integrations-platform/quickstart.md`
- [X] T042 [P] Add regression coverage proving integration readiness gating does not rewrite delivery or notification outcomes in `daemon/internal/api/workflows_test.go`, `daemon/internal/runtime/runtime_test.go`, and `daemon/internal/policy/policy_test.go`
- [X] T043 [P] Document the integration-readiness versus delivery-outcome boundary in `docs/runtime/operator-trust-model.md`, `docs/runtime/daemon-roadmaps.md`, and `docs/specs/013-delivery-and-notifications.md`
- [X] T044 [P] Update downstream domain specs to reference the shared integration readiness and account-binding contract in `docs/specs/014-calendar-integration.md` and `docs/specs/015-mail-integration.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all story work.
- **Phase 3: US1**: Depends on Phase 2; establishes the MVP integration resource and readiness surface.
- **Phase 4: US2**: Depends on Phase 2 and reuses the resource model from US1 to execute fake integration probes through runtime, workflow, and approval truth.
- **Phase 5: US3**: Depends on Phase 2 and builds on the reusable binding shapes from US1 and US2 to prove domain-agnostic reuse.
- **Phase 6: Polish**: Depends on all user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on other user stories.
- **US2 (P2)**: Builds on US1 integration resources and canonical-default truth but remains independently testable once fake probe execution and binding snapshots are implemented across runtime, workflow, and approval surfaces.
- **US3 (P3)**: Builds on the binding and projection surfaces from US1 and US2 to prove cross-domain reuse without changing their primary behavior.

### Within Each User Story

- Tests and contract coverage land before or alongside implementation and must fail before the story is considered complete.
- Manager and store changes precede API projection tasks that depend on persisted truth.
- Route handlers follow resource or probe manager behavior rather than inventing state locally.
- Story-specific docs or recorded validation happen only after the corresponding behavior is functional.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together.
- In Foundational, shared types, store helpers, app wiring, fake backend scaffolding, validator work, and provenance helpers can proceed in parallel.
- For each user story, API/contract tests and manager/runtime tests can be written in parallel.
- Within stories, persistence work and route/schema serialization work can proceed in parallel before final integration tasks.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T013 [US1] Add integration create/list/detail/readiness/default API contract coverage in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T014 [US1] Add manager and store regressions for readiness transitions, duplicate account groups, and canonical-default replacement in daemon/internal/integrations/manager_test.go and daemon/internal/store/store_test.go"

# Implementation in parallel
Task: "T015 [US1] Implement integration CRUD, readiness updates, and canonical-default logic in daemon/internal/integrations/manager.go and daemon/internal/integrations/types.go"
Task: "T016 [US1] Implement integration create/list/get/readiness/default route handlers in daemon/internal/api/integrations.go and daemon/internal/api/server.go"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T021 [US2] Add fake integration probe API and binding-summary contract tests in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T022 [US2] Add degraded-versus-unavailable probe gating and approval regressions in daemon/internal/integrations/manager_test.go, daemon/internal/api/server_test.go, and daemon/internal/runtime/runtime_test.go"

# Implementation in parallel
Task: "T024 [US2] Extend runtime tool-call models and persisted documents with integrationBindings in daemon/internal/runtime/runtime.go and daemon/internal/store/store.go"
Task: "T025 [US2] Extend workflow-step and approval resources with integrationBindings in daemon/internal/orchestration/types.go, daemon/internal/policy/policy.go, schemas/api/workflow-step-resource.schema.json, and schemas/api/approval-resource.schema.json"
```

## Parallel Example: User Story 3

```bash
# Tests in parallel
Task: "T030 [US3] Add multi-domain and multi-backend contract regressions for integration resource normalization in daemon/internal/contracts/contracts_test.go and daemon/internal/integrations/manager_test.go"
Task: "T031 [US3] Add workflow and runtime regressions showing optional integrationBindings do not break non-integration behavior in daemon/internal/api/workflows_test.go, daemon/internal/api/server_test.go, and daemon/internal/runtime/runtime_test.go"

# Implementation in parallel
Task: "T032 [US3] Implement backend-binding normalization and domain-agnostic validation in daemon/internal/integrations/backend.go and daemon/internal/integrations/manager.go"
Task: "T033 [US3] Implement reusable integration binding snapshot builders for future domains in daemon/internal/integrations/bindings.go and daemon/internal/integrations/types.go"
```

## Implementation Strategy

### Roadmap-Closed Delivery Sequence

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1, then validate integration create/list/detail, readiness transitions, and canonical-default behavior in `KURA_ENV=test`.
4. Continue through Phase 4 so integration-backed execution is covered on runtime, workflow, and approval truth rather than stopping at the operator-readiness surface.
5. Complete Phase 5 and Phase 6 before declaring roadmap 27 closed.

### Incremental Delivery

1. Land Setup + Foundational to establish the integrations package, persistence, fake backend abstraction, and contract scaffolding.
2. Deliver US1 for daemon-owned integration resources, readiness truth, provenance, and canonical-default behavior.
3. Deliver US2 for fake integration probes and runtime/workflow/approval linkage.
4. Deliver US3 for cross-domain/backend reuse guarantees and backward-compatible optional binding summaries.
5. Finish with docs, delivery-boundary guardrails, downstream spec references, fixture validation, and recorded manual verification.

### Parallel Team Strategy

1. One engineer lands store, app wiring, validator scaffolding, and shared types in Setup + Foundational.
2. After Foundational is complete:
   - Engineer A takes US1 resource routes, manager logic, and readiness events.
   - Engineer B takes US2 fake probe execution and runtime/workflow/approval linkage.
   - Engineer C takes US3 backend normalization and optional binding projections once US1/US2 shapes are stable.

## Notes

- `[P]` means the task can run in parallel because it targets different files or only depends on completed foundational work.
- Every user story has explicit tests because this roadmap changes operator-visible behavior and contract-backed surfaces.
- Existing connector, capability, MCP, workflow, delivery, and non-integration runtime behavior must remain backward compatible throughout implementation.
- Manual quickstart validation complements but does not replace API, store, runtime, contract, and restart coverage.
