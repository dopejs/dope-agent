---

description: "Task list for Sandbox Managed Provider Convergence"
---

# Tasks: Sandbox Managed Provider Convergence

**Input**: Design documents from `/specs/001-sandbox-managed-providers/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This feature changes execution-boundary, provider
auth, schema, and event behavior, so targeted tests and contract verification are required.

**Organization**: Tasks are grouped by user story to enable independent implementation and
testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., `US1`, `US2`, `US3`)
- Include exact file paths in descriptions

## Path Conventions

- Daemon code lives under `daemon/internal/...`
- API and event schemas live under `schemas/api/` and `schemas/events/`
- Feature artifacts live under `specs/001-sandbox-managed-providers/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable fixtures and assertion helpers used across all stories

- [X] T001 Create shared managed-provider local-state fixture helpers in `daemon/internal/managedproviders/managedproviders_test.go`
- [X] T002 [P] Add reusable provider-auth metadata and sandbox provenance assertions in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish the shared sandbox and provider metadata primitives required by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Define managed-provider requirement and provenance types in `daemon/internal/sandbox/types.go`
- [X] T004 Implement shared local-state declaration, sensitivity summary, and fail-closed helpers in `daemon/internal/managedproviders/bridges.go`
- [X] T005 Extend sandbox metadata plumbing for managed-provider requirement evaluation and enforcement-strength reporting in `daemon/internal/sandbox/manager.go`
- [X] T006 Create additive provider-auth metadata carrier support in `daemon/internal/managedproviders/bridges.go`, `daemon/internal/managedproviders/claude.go`, `daemon/internal/managedproviders/codex.go`, and `daemon/internal/api/server.go`

**Checkpoint**: Shared requirement, provenance, and metadata primitives are ready for story work

---

## Phase 3: User Story 1 - Managed provider actions stay inside sandbox policy (Priority: P1) 🎯 MVP

**Goal**: Move in-scope managed-provider workflows behind declared sandbox requirements and fail closed on undeclared access

**Independent Test**: Trigger managed-provider auth-status, logout, and prompt-execution workflows and confirm declared access succeeds while undeclared local-state access is denied without fallback.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T007 [P] [US1] Add fail-closed declaration tests for Claude and Codex auth-status, logout, and prompt-execution workflows in `daemon/internal/managedproviders/managedproviders_test.go`
- [X] T008 [P] [US1] Add sandbox decision coverage for declared versus undeclared managed-provider access in `daemon/internal/sandbox/manager_test.go`

### Implementation for User Story 1

- [X] T009 [P] [US1] Update Claude auth-status, logout, and prompt-execution requirement declarations in `daemon/internal/managedproviders/claude.go`
- [X] T010 [P] [US1] Update Codex auth-status, logout, and prompt-execution requirement declarations in `daemon/internal/managedproviders/codex.go`
- [X] T011 [US1] Integrate baseline allow and fail-closed denial behavior through `daemon/internal/managedproviders/bridges.go` and `daemon/internal/app/app.go`

**Checkpoint**: In-scope managed-provider workflows now run only within declared sandbox boundaries and can be validated independently

---

## Phase 4: User Story 2 - Operators can explain provider failures and provenance (Priority: P2)

**Goal**: Make managed-provider sandbox provenance, failure class, and enforcement strength visible through existing daemon surfaces

**Independent Test**: Execute one success path and one denied or failing path, then verify provider auth state, sandbox inspection, and contract fixtures expose provider/action provenance plus distinct failure classes.

### Tests for User Story 2 ⚠️

- [X] T012 [P] [US2] Add API and contract tests for additive provider-auth metadata and sandbox provenance in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T013 [P] [US2] Add managed-provider failure-classification regression tests in `daemon/internal/managedproviders/managedproviders_test.go`

### Implementation for User Story 2

- [X] T014 [US2] Project logical managed-provider operation summaries into provider auth metadata in `daemon/internal/managedproviders/claude.go`, `daemon/internal/managedproviders/codex.go`, and `daemon/internal/api/server.go`
- [X] T015 [US2] Enrich subprocess-backed sandbox execution metadata and backend metadata in `daemon/internal/managedproviders/bridges.go` and `daemon/internal/sandbox/manager.go`
- [X] T016 [US2] Update additive API and event contracts in `schemas/api/provider-auth-state.response.schema.json`, `schemas/api/sandbox-execution.resource.schema.json`, `schemas/api/sandbox-result.schema.json`, `schemas/events/provider-auth-started.event.schema.json`, `schemas/events/provider-auth-completed.event.schema.json`, `schemas/events/provider-auth-refreshed.event.schema.json`, `schemas/events/provider-auth-revoked.event.schema.json`, `schemas/events/sandbox-execution-requested.event.schema.json`, `schemas/events/sandbox-execution-started.event.schema.json`, `schemas/events/sandbox-execution-completed.event.schema.json`, `schemas/events/sandbox-execution-failed.event.schema.json`, `schemas/events/sandbox-execution-cancelled.event.schema.json`, `schemas/events/sandbox-execution-denied.event.schema.json`, and `daemon/internal/contracts/contracts_test.go`

**Checkpoint**: Operators can explain managed-provider decisions and failures from daemon-visible surfaces without log-only debugging

---

## Phase 5: User Story 3 - Supported workflows remain usable across environments (Priority: P3)

**Goal**: Preserve supported managed-provider workflows, test-versus-production isolation, and sensitive-state redaction after convergence

**Independent Test**: Run the supported managed-provider workflows in the test environment and confirm they still work when declarations are satisfied, remain separated from production-only state, and redact sensitive local-state details.

### Tests for User Story 3 ⚠️

- [X] T017 [P] [US3] Add test-versus-production separation and redaction regression tests in `daemon/internal/managedproviders/managedproviders_test.go` and `daemon/internal/app/app_test.go`
- [X] T018 [P] [US3] Add end-to-end managed-provider API regression coverage in `daemon/internal/api/server_test.go`

### Implementation for User Story 3

- [X] T019 [US3] Enforce environment-aware local-state resolution and redacted sensitive-state summaries in `daemon/internal/managedproviders/claude.go`, `daemon/internal/managedproviders/codex.go`, and `daemon/internal/managedproviders/bridges.go`
- [X] T020 [US3] Align operator guidance for workflow scope, enforcement-strength limits, and verification in `docs/harness/sandbox-execution-plane.md`, `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-tasks.md`, and `specs/001-sandbox-managed-providers/quickstart.md`

**Checkpoint**: Supported workflows remain usable in the test environment and the rollout documents the final operating boundary

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, contract closure, and rollout readiness

- [X] T021 [P] Run `make daemon-contract-test` and record contract verification notes in `specs/001-sandbox-managed-providers/quickstart.md`
- [X] T022 [P] Run targeted daemon package verification, confirm the <=100 ms daemon-side preflight overhead target for in-scope workflows, and record results in `specs/001-sandbox-managed-providers/quickstart.md`
- [X] T023 [P] Run full daemon regression verification and record final status in `specs/001-sandbox-managed-providers/quickstart.md`
- [X] T024 Finalize rollback and readiness notes in `specs/001-sandbox-managed-providers/quickstart.md` and `docs/runtime/daemon-roadmaps.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational completion; delivers the MVP boundary closure
- **User Story 2 (Phase 4)**: Depends on User Story 1 behavior existing so provenance and failure detail can be attached to real flows
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 so regression and environment isolation validate the final shaped behavior
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Depends on User Story 1 because provenance and failure surfaces must reflect the converged workflows
- **User Story 3 (P3)**: Depends on User Stories 1 and 2 because regression and environment validation require both behavior and operator-visible outputs

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Shared declaration or metadata helpers before provider-specific bridge changes
- Provider-specific bridge changes before schema or documentation closure
- Story-specific docs and verification after core behavior is working

### Parallel Opportunities

- `T002` can run in parallel with `T001`
- `T007` and `T008` can run in parallel
- `T009` and `T010` can run in parallel
- `T012` and `T013` can run in parallel
- `T017` and `T018` can run in parallel
- `T021`, `T022`, and `T023` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 1

```bash
# Launch User Story 1 tests together:
Task: "Add fail-closed declaration tests for Claude and Codex auth-status, logout, and prompt-execution workflows in daemon/internal/managedproviders/managedproviders_test.go"
Task: "Add sandbox decision coverage for declared versus undeclared managed-provider access in daemon/internal/sandbox/manager_test.go"

# Launch provider-specific implementation together:
Task: "Update Claude auth-status, logout, and prompt-execution requirement declarations in daemon/internal/managedproviders/claude.go"
Task: "Update Codex auth-status, logout, and prompt-execution requirement declarations in daemon/internal/managedproviders/codex.go"
```

## Parallel Example: User Story 2

```bash
# Launch provenance-facing tests together:
Task: "Add API and contract tests for additive provider-auth metadata and sandbox provenance in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "Add managed-provider failure-classification regression tests in daemon/internal/managedproviders/managedproviders_test.go"
```

## Parallel Example: User Story 3

```bash
# Launch regression coverage together:
Task: "Add test-versus-production separation and redaction regression tests in daemon/internal/managedproviders/managedproviders_test.go and daemon/internal/app/app_test.go"
Task: "Add end-to-end managed-provider API regression coverage in daemon/internal/api/server_test.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Run the User Story 1 tests and confirm undeclared access fails closed

### Incremental Delivery

1. Complete Setup + Foundational → shared requirement and provenance primitives ready
2. Deliver User Story 1 → validate managed-provider boundary closure
3. Deliver User Story 2 → validate operator-visible provenance and failure classification
4. Deliver User Story 3 → validate regression safety, redaction, and environment isolation
5. Finish with contract and full-suite verification

### Parallel Team Strategy

With multiple developers:

1. One developer handles foundational sandbox and bridge primitives
2. After Foundational:
   - Developer A: Claude-side convergence work in `daemon/internal/managedproviders/claude.go`
   - Developer B: Codex-side convergence work in `daemon/internal/managedproviders/codex.go`
   - Developer C: API/schema/contract visibility work in `daemon/internal/api/server_test.go`, `schemas/api/`, and `daemon/internal/contracts/contracts_test.go`

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] labels map every story task back to the corresponding user story in `spec.md`
- Each user story is independently testable at its checkpoint
- Verification tasks are mandatory because this feature changes execution-boundary and contract behavior
- Avoid hidden fallback logic, undocumented schema drift, or docs that imply stronger isolation than the subprocess backend actually provides
