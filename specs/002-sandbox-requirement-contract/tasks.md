---

description: "Task list for Sandbox Requirement Declaration Contract"

---

# Tasks: Sandbox Requirement Declaration Contract

**Input**: Design documents from `/specs/002-sandbox-requirement-contract/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This feature changes execution-boundary, contract,
schema, event, redaction, and restart-durability behavior, so targeted tests and contract
verification are required.

**Organization**: Tasks are grouped by user story to enable independent implementation and
testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this belongs to (`US1`, `US2`, `US3`)
- Include exact file paths in descriptions

## Path Conventions

- Daemon code lives under `daemon/internal/...`
- API and event schemas live under `schemas/api/` and `schemas/events/`
- Feature artifacts live under `specs/002-sandbox-requirement-contract/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable fixtures and assertion helpers used across all stories

- [x] T001 Create shared consumer declaration fixture builders in `daemon/internal/sandbox/manager_test.go`
- [x] T002 [P] Add reusable API and contract assertion helpers for declaration, secret-scope, and provenance metadata in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish the shared declaration, secret-scope, and durability primitives
required by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T003 Define shared consumer declaration, secret binding, and policy record types in `daemon/internal/sandbox/types.go`
- [x] T004 Extend SQLite schema and persistence helpers for durable consumer policy records and secret scope bindings in `daemon/internal/store/store.go` and `daemon/internal/store/store_test.go`
- [x] T005 Extend declaration-aware evaluation, explain, and unsupported-guarantee handling in `daemon/internal/sandbox/manager.go` and `daemon/internal/sandbox/manager_test.go`
- [x] T006 Create shared consumer metadata projection helpers in `daemon/internal/api/types.go` and `daemon/internal/api/server.go`

**Checkpoint**: Shared declaration, secret-scope, and persistence primitives are ready for
story work

---

## Phase 3: User Story 1 - Shared execution requirements become explicit (Priority: P1) 🎯 MVP

**Goal**: Bring current adopters onto one shared declaration contract for managed providers,
skill-selection surfaces, and daemon-owned high-risk local tool paths

**Independent Test**: Inspect managed-provider behavior, current skill registry and explicit
skill-selection surfaces, and the current high-risk tool-call path, then verify they all
use the same declaration vocabulary, reject unsupported stronger guarantees, and no longer
depend on hidden consumer-specific execution rules.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T007 [P] [US1] Add declaration coverage for managed-provider and daemon-owned local-tool evaluation in `daemon/internal/managedproviders/managedproviders_test.go` and `daemon/internal/api/server_test.go`
- [x] T008 [P] [US1] Add skill declaration and explicit skill-selection regression tests in `daemon/internal/skills/registry_test.go` and `daemon/internal/chat/service_test.go`

### Implementation for User Story 1

- [x] T009 [P] [US1] Implement shared declaration adapters for managed-provider operations in `daemon/internal/managedproviders/bridges.go`, `daemon/internal/managedproviders/claude.go`, and `daemon/internal/managedproviders/codex.go`
- [x] T010 [P] [US1] Add declaration-bearing metadata for current skill registry and explicit skill-selection surfaces in `daemon/internal/skills/registry.go`, `daemon/internal/chat/service.go`, and `daemon/internal/api/types.go`
- [x] T011 [US1] Converge daemon-owned high-risk local tool calls onto declaration-aware sandbox evaluation in `daemon/internal/api/server.go`, `daemon/internal/runtime/runtime.go`, and `daemon/internal/sandbox/manager.go`
- [x] T012 [US1] Update additive declaration contracts for sandbox, skill, and tool-call surfaces in `schemas/api/sandbox-execution.resource.schema.json`, `schemas/api/sandbox-explain.response.schema.json`, `schemas/api/skill-detail.response.schema.json`, `schemas/api/skill-registry.response.schema.json`, `schemas/api/tool-call-resource.schema.json`, `schemas/events/sandbox-decision-recorded.event.schema.json`, and `daemon/internal/contracts/contracts_test.go`

**Checkpoint**: Managed providers, current skill registry and explicit skill-selection
surfaces, and the current high-risk tool-call path share one explicit declaration contract
and can be tested independently

---

## Phase 4: User Story 2 - Secret scope and redaction stay explicit and safe (Priority: P2)

**Goal**: Enforce per-consumer-instance secret scope with reusable defaults and keep all
operator-visible surfaces redacted

**Independent Test**: Exercise secret-backed managed-provider paths, the current high-risk
tool-call path, and skill-facing inspection surfaces, then verify secret resolution is
explicit per consumer instance and environment, unavailable secrets are classified
clearly, and no secret-bearing values leak through config, explanation, event, or history
surfaces.

### Tests for User Story 2 ⚠️

- [x] T013 [P] [US2] Add test-versus-production secret-scope resolution, unavailable-secret, and redaction regression tests for sandbox and config surfaces in `daemon/internal/sandbox/manager_test.go` and `daemon/internal/api/server_test.go`
- [x] T014 [P] [US2] Add per-consumer-instance secret authorization tests for managed providers and local tools in `daemon/internal/managedproviders/managedproviders_test.go` and `daemon/internal/policy/policy_test.go`

### Implementation for User Story 2

- [x] T015 [US2] Implement environment-scoped secret-scope binding models and resolution helpers in `daemon/internal/sandbox/types.go`, `daemon/internal/sandbox/manager.go`, and `daemon/internal/store/store.go`
- [x] T016 [P] [US2] Apply per-consumer-instance secret scope and redaction to managed-provider state and local-tool request handling in `daemon/internal/managedproviders/bridges.go` and `daemon/internal/api/server.go`
- [x] T017 [P] [US2] Project redacted environment-scoped secret-scope metadata and default-rule attribution through operator-visible responses in `daemon/internal/api/types.go`, `schemas/api/config.response.schema.json`, `schemas/api/provider-auth-state.response.schema.json`, `schemas/api/sandbox-result.schema.json`, `schemas/api/sandbox-execution.resource.schema.json`, `schemas/api/skill-detail.response.schema.json`, and `schemas/api/skill-registry.response.schema.json`
- [x] T018 [US2] Align secret-scope and redaction guidance in `docs/harness/sandbox-execution-plane.md`, `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-tasks.md`, and `specs/002-sandbox-requirement-contract/quickstart.md`

**Checkpoint**: Secret scope is explicit per consumer instance and all updated surfaces stay
redacted and independently testable

---

## Phase 5: User Story 3 - Consumer provenance is durable and queryable (Priority: P3)

**Goal**: Persist durable cross-consumer provenance for launched, denied, unsupported, and
preflight-only paths, including restart-safe visibility

**Independent Test**: Run managed-provider flows, skill-facing surfaces, and the current
high-risk tool-call path, restart the daemon, and verify operators can still query
consumer kind, consumer instance, declaration identity, secret-scope outcome, and
terminal state without reconstructing from logs.

### Tests for User Story 3 ⚠️

- [x] T019 [P] [US3] Add durable provenance and restart regression tests for denied and preflight-only records in `daemon/internal/store/store_test.go` and `daemon/internal/app/app_test.go`
- [x] T020 [P] [US3] Add API and contract regression coverage for consumer provenance across sandbox, provider, skill, and tool-call surfaces in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`

### Implementation for User Story 3

- [x] T021 [US3] Implement durable consumer policy record persistence and restore wiring in `daemon/internal/store/store.go`, `daemon/internal/app/app.go`, and `daemon/internal/sandbox/manager.go`
- [x] T022 [P] [US3] Attach declaration and policy-record provenance to managed-provider and sandbox execution metadata in `daemon/internal/managedproviders/bridges.go`, `daemon/internal/managedproviders/claude.go`, `daemon/internal/managedproviders/codex.go`, and `daemon/internal/sandbox/manager.go`
- [x] T023 [P] [US3] Surface durable provenance through local-tool, skill, approval, and decision paths in `daemon/internal/api/server.go`, `daemon/internal/api/types.go`, `daemon/internal/runtime/runtime.go`, and `daemon/internal/skills/registry.go`
- [x] T024 [US3] Update additive provenance schemas and events in `schemas/api/approval-resource.schema.json`, `schemas/api/decision-resource.schema.json`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/sandbox-execution.resource.schema.json`, `schemas/api/sandbox-result.schema.json`, `schemas/api/provider-auth-state.response.schema.json`, `schemas/events/policy-decision-recorded.event.schema.json`, `schemas/events/sandbox-decision-recorded.event.schema.json`, `schemas/events/provider-auth-started.event.schema.json`, `schemas/events/provider-auth-completed.event.schema.json`, `schemas/events/provider-auth-refreshed.event.schema.json`, `schemas/events/provider-auth-revoked.event.schema.json`, and `daemon/internal/contracts/contracts_test.go`

**Checkpoint**: Durable provenance survives restart and remains queryable across current
consumer families

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, operator guidance, and rollback readiness

- [x] T025 [P] Update operator trust and runtime guidance for declaration-backed local tools in `docs/runtime/operator-trust-model.md` and `docs/runtime/runtime-architecture.md`
- [x] T026 [P] Run `make daemon-contract-test` and record results in `specs/002-sandbox-requirement-contract/quickstart.md`
- [x] T027 [P] Run targeted daemon verification and record results in `specs/002-sandbox-requirement-contract/quickstart.md`
- [x] T028 [P] Run full daemon regression verification and record results in `specs/002-sandbox-requirement-contract/quickstart.md`
- [x] T029 Finalize rollback and readiness notes in `specs/002-sandbox-requirement-contract/quickstart.md` and `docs/runtime/daemon-roadmaps.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational; establishes the shared declaration contract and the MVP boundary
- **User Story 2 (Phase 4)**: Depends on User Story 1 because secret scope attaches to the shared declaration model and adopter wiring
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 because durable provenance must carry declaration identity and secret-scope outcomes across current adopters
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on later stories
- **User Story 2 (P2)**: Depends on User Story 1 because secret-scope policy must resolve against the shared declaration contract
- **User Story 3 (P3)**: Depends on User Stories 1 and 2 because durable provenance must reflect both declaration and secret-scope results

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Shared adapters or metadata carriers before adopter-specific behavior changes
- Core behavior before schema and contract closure
- Docs and quickstart updates after the story behavior is working

### Parallel Opportunities

- `T002` can run in parallel with `T001`
- `T007` and `T008` can run in parallel
- `T009` and `T010` can run in parallel
- `T013` and `T014` can run in parallel
- `T016` and `T017` can run in parallel
- `T019` and `T020` can run in parallel
- `T022` and `T023` can run in parallel
- `T026`, `T027`, and `T028` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 1

```bash
# Launch declaration regression tests together:
Task: "Add declaration coverage for managed-provider and daemon-owned local-tool evaluation in daemon/internal/managedproviders/managedproviders_test.go and daemon/internal/api/server_test.go"
Task: "Add skill declaration and explicit skill-selection regression tests in daemon/internal/skills/registry_test.go and daemon/internal/chat/service_test.go"

# Launch adopter-specific declaration work together:
Task: "Implement shared declaration adapters for managed-provider operations in daemon/internal/managedproviders/bridges.go, daemon/internal/managedproviders/claude.go, and daemon/internal/managedproviders/codex.go"
Task: "Add declaration-bearing metadata for current skill registry and explicit skill-selection surfaces in daemon/internal/skills/registry.go, daemon/internal/chat/service.go, and daemon/internal/api/types.go"
```

## Parallel Example: User Story 2

```bash
# Launch secret-scope regression coverage together:
Task: "Add secret-scope resolution and redaction regression tests for sandbox and config surfaces in daemon/internal/sandbox/manager_test.go and daemon/internal/api/server_test.go"
Task: "Add per-consumer-instance secret authorization tests for managed providers and local tools in daemon/internal/managedproviders/managedproviders_test.go and daemon/internal/policy/policy_test.go"
```

## Parallel Example: User Story 3

```bash
# Launch durable provenance coverage together:
Task: "Add durable provenance and restart regression tests for denied and preflight-only records in daemon/internal/store/store_test.go and daemon/internal/app/app_test.go"
Task: "Add API and contract regression coverage for consumer provenance across sandbox, provider, skill, and tool-call surfaces in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Inspect managed-provider, skill, and local-tool declaration behavior independently

### Incremental Delivery

1. Complete Setup + Foundational → shared declaration, secret-scope, and persistence primitives ready
2. Deliver User Story 1 → validate common declaration contract across current adopters
3. Deliver User Story 2 → validate explicit secret scope and redaction behavior
4. Deliver User Story 3 → validate durable provenance and restart-safe queryability
5. Finish with contract, targeted, and full-suite verification plus rollback notes

### Parallel Team Strategy

With multiple developers:

1. One developer handles foundational declaration and persistence primitives
2. After Foundational:
   - Developer A: managed-provider adopter work in `daemon/internal/managedproviders/`
   - Developer B: skill and chat declaration/provenance work in `daemon/internal/skills/`, `daemon/internal/chat/`, and `daemon/internal/api/`
   - Developer C: local-tool, policy, and runtime convergence work in `daemon/internal/api/`, `daemon/internal/runtime/`, `daemon/internal/policy/`, and `schemas/`

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] labels map each story task back to the corresponding user story in `spec.md`
- Each user story is independently testable at its checkpoint
- Current skill adoption in this slice is limited to registry and explicit skill-selection surfaces; generic executable-skill subprocess support remains out of scope
- Daemon-owned local-tool work in this slice refers to the current high-risk tool-call path, not a generic arbitrary script runner
- Avoid hidden fallback logic, undeclared secret inheritance, or docs that imply stronger backend guarantees than `subprocess` actually provides
