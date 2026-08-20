---

description: "Task list for Skill And Local Tool Sandbox Execution"

---

# Tasks: Skill And Local Tool Sandbox Execution

**Input**: Design documents from `/specs/004-skill-tool-sandbox/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This roadmap changes execution-boundary, runtime,
approval, schema, event, persistence, and restart behavior, so targeted tests and contract
verification are required.

**Organization**: Tasks are grouped by user story to enable independent implementation and
testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependency)
- **[Story]**: Which user story this belongs to (`US1`, `US2`, `US3`, `US4`)
- Include exact file paths in descriptions

## Path Conventions

- Daemon code lives under `daemon/internal/...`
- API and event schemas live under `schemas/api/` and `schemas/events/`
- Feature artifacts live under `specs/004-skill-tool-sandbox/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable executable-skill fixtures, runtime assertion helpers, and
verification scaffolding for Roadmap 19

- [X] T001 Create executable-skill fixture helpers for valid, invalid, and approval-gated manifests in `daemon/internal/skills/registry_test.go`
- [X] T002 [P] Add reusable API assertion helpers for skill execution and linked tool-call resources in `daemon/internal/api/server_test.go`
- [X] T003 [P] Add reusable contract assertion helpers for executable-skill, tool-call, and sandbox linkage fixtures in `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish manifest types, runtime linkage, and persistence primitives required
by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T004 Define executable-skill manifest, availability, and invocation metadata types in `daemon/internal/skills/registry.go`, `daemon/internal/runtime/runtime.go`, and `daemon/internal/sandbox/types.go`
- [X] T005 Extend SQLite persistence and restore helpers for additive tool-call/sandbox linkage needed by skill execution in `daemon/internal/store/store.go` and `daemon/internal/store/store_test.go`
- [X] T006 Implement shared helper plumbing for sandbox-backed skill execution requests and runtime linkage in `daemon/internal/api/server.go`, `daemon/internal/runtime/runtime.go`, and `daemon/internal/app/app.go`
- [X] T007 [P] Add additive API projection helpers for executable-skill availability and skill-backed tool-call responses in `daemon/internal/api/types.go` and `daemon/internal/api/server.go`
- [X] T008 [P] Extend contract fixture vocabulary for skill-backed tool calls and sandbox-linked execution payloads in `daemon/internal/contracts/contracts_test.go` and `schemas/api/tool-call-resource.schema.json`

**Checkpoint**: Manifest, runtime linkage, and persistence foundations are ready for story work

---

## Phase 3: User Story 1 - Executable skills declare real execution requirements (Priority: P1) 🎯 MVP

**Goal**: Extend the skill registry and inspection surfaces so executable skills declare
explicit sandbox requirements and invalid manifests remain visible as `unavailable`

**Independent Test**: Inspect representative executable skills through the real skill
routes and registry APIs, then verify valid skills expose execution requirements while
invalid skills remain visible as `unavailable` with an explicit reason.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T009 [P] [US1] Add executable-skill manifest parsing, default approval, and unavailable-state regression tests in `daemon/internal/skills/registry_test.go`
- [X] T010 [P] [US1] Add skill list/detail API regression tests for executable manifest and unavailable projection in `daemon/internal/api/server_test.go`
- [X] T011 [P] [US1] Add schema and contract regression coverage for executable-skill inspection payloads in `daemon/internal/contracts/contracts_test.go`, `schemas/api/skill-summary.schema.json`, `schemas/api/skill-detail.response.schema.json`, and `schemas/api/skill-registry.response.schema.json`

### Implementation for User Story 1

- [X] T012 [US1] Implement executable-manifest parsing, validation, and unavailable-state projection in `daemon/internal/skills/registry.go`
- [X] T013 [US1] Extend skill API response builders and route handling for executable manifest visibility in `daemon/internal/api/types.go` and `daemon/internal/api/server.go`
- [X] T014 [P] [US1] Add execution-oriented skill declaration metadata and default `ask` approval handling in `daemon/internal/skills/registry.go` and `daemon/internal/sandbox/types.go`
- [X] T015 [US1] Update contract fixtures and operator-facing docs for executable-skill inspection semantics in `daemon/internal/contracts/contracts_test.go` and `specs/004-skill-tool-sandbox/contracts/skill-tool-sandbox-surfaces.md`

**Checkpoint**: Executable skills are independently inspectable with explicit requirements and operator-visible unavailable reasons

---

## Phase 4: User Story 2 - Local tool and skill execution use one sandbox path (Priority: P2)

**Goal**: Route executable-skill launches and the current high-risk local tool path
through sandbox-backed execution with consistent approval and failure behavior

**Independent Test**: Execute representative executable skills and high-risk local tools
through their real runtime paths, then verify success, denial, approval-gated, timeout,
cancellation, secret-redaction, and environment-scoped execution outcomes all resolve
through sandbox-backed execution within the declared preflight budget.

### Tests for User Story 2 ⚠️

- [X] T016 [P] [US2] Add executable-skill launch, approval, denial, timeout, and unsupported-backend regression tests in `daemon/internal/api/server_test.go` and `daemon/internal/sandbox/manager_test.go`
- [X] T017 [P] [US2] Add runtime manager regression tests for skill-backed tool-call lifecycle and terminal status handling in `daemon/internal/runtime/runtime_test.go`
- [X] T018 [P] [US2] Add contract regression coverage for skill-backed tool-call, approval, and sandbox execution payloads in `daemon/internal/contracts/contracts_test.go`, `schemas/api/create-tool-call.request.schema.json`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/approval-resource.schema.json`, `schemas/api/decision-resource.schema.json`, `schemas/api/sandbox-execution.resource.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, `schemas/events/tool-call-failed.event.schema.json`, `schemas/events/sandbox-execution-requested.event.schema.json`, `schemas/events/sandbox-execution-started.event.schema.json`, `schemas/events/sandbox-execution-completed.event.schema.json`, `schemas/events/sandbox-execution-failed.event.schema.json`, `schemas/events/sandbox-execution-cancelled.event.schema.json`, and `schemas/events/sandbox-execution-denied.event.schema.json`
- [X] T040 [P] [US2] Add regression tests for executable-skill secret ref resolution, redacted operator-visible output, and `KURA_ENV`-scoped execution separation in `daemon/internal/skills/registry_test.go`, `daemon/internal/api/server_test.go`, and `daemon/internal/sandbox/manager_test.go`
- [X] T041 [P] [US2] Add timing regression coverage that executable-manifest validation and sandbox execution preflight stay within `<=100 ms` in `daemon/internal/skills/registry_test.go` and `daemon/internal/api/server_test.go`

### Implementation for User Story 2

- [X] T019 [US2] Implement API request handling for executable-skill launches on the existing tool-call path in `daemon/internal/api/server.go`
- [X] T020 [US2] Implement runtime tool-call support for skill-backed executions, including additive invocation metadata and terminal states in `daemon/internal/runtime/runtime.go`
- [X] T021 [US2] Implement sandbox execution request construction for executable skills and migrate the current high-risk local tool path to real sandbox launches in `daemon/internal/api/server.go` and `daemon/internal/sandbox/manager.go`
- [X] T022 [US2] Persist additive skill/local-tool sandbox linkage on tool calls and executions in `daemon/internal/store/store.go`
- [X] T023 [P] [US2] Update API and event schemas for skill-backed tool-call execution, approval, sandbox outcomes, and redacted secret projections in `schemas/api/tool-call-resource.schema.json`, `schemas/api/approval-resource.schema.json`, `schemas/api/decision-resource.schema.json`, `schemas/api/sandbox-execution.resource.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, `schemas/events/tool-call-failed.event.schema.json`, `schemas/events/sandbox-execution-requested.event.schema.json`, `schemas/events/sandbox-execution-started.event.schema.json`, `schemas/events/sandbox-execution-completed.event.schema.json`, `schemas/events/sandbox-execution-failed.event.schema.json`, `schemas/events/sandbox-execution-cancelled.event.schema.json`, and `schemas/events/sandbox-execution-denied.event.schema.json`
- [X] T042 [US2] Implement environment-scoped executable-skill secret ref resolution and sandbox env injection without crossing `~/.kura-test` / `~/.kura` boundaries in `daemon/internal/skills/registry.go`, `daemon/internal/api/server.go`, and `daemon/internal/sandbox/manager.go`
- [X] T043 [US2] Implement operator-visible redaction for secret values and secret-derived material across skill-backed tool-call, approval, decision, and sandbox execution surfaces in `daemon/internal/api/types.go`, `daemon/internal/api/server.go`, and `daemon/internal/runtime/runtime.go`

**Checkpoint**: In-scope executable skills and high-risk local tools execute only through sandbox-backed paths

---

## Phase 5: User Story 3 - Runtime history and sandbox provenance stay linked (Priority: P3)

**Goal**: Make runtime tool history, approvals, consumer policy records, and sandbox
execution stay linked across normal execution and daemon restart recovery

**Independent Test**: Execute skill and local-tool actions, inspect runtime plus sandbox
history, restart the daemon around recorded activity, and verify the execution path stays
reconstructable with interrupted in-flight work recovered as `cancelled`.

### Tests for User Story 3 ⚠️

- [X] T024 [P] [US3] Add restart and recovery regression tests for in-flight skill/local-tool executions in `daemon/internal/app/app_test.go` and `daemon/internal/sandbox/manager_test.go`
- [X] T025 [P] [US3] Add runtime-to-sandbox provenance regression tests for tool-call listing/get and policy lookup in `daemon/internal/api/server_test.go` and `daemon/internal/store/store_test.go`
- [X] T026 [P] [US3] Add contract regression coverage for additive provenance and cancelled recovery state in `daemon/internal/contracts/contracts_test.go`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/sandbox-execution.resource.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, `schemas/events/tool-call-failed.event.schema.json`, and `schemas/events/sandbox-execution-cancelled.event.schema.json`

### Implementation for User Story 3

- [X] T027 [US3] Implement durable runtime-to-sandbox linkage and recovery-aware tool-call state transitions in `daemon/internal/runtime/runtime.go` and `daemon/internal/store/store.go`
- [X] T028 [US3] Implement daemon restart recovery that marks interrupted in-flight skill/local-tool executions as `cancelled` in `daemon/internal/app/app.go` and `daemon/internal/sandbox/manager.go`
- [X] T029 [US3] Extend approval, decision, and tool-call API projections with linked consumer-policy and sandbox provenance in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [X] T030 [P] [US3] Update schema-backed persistence and contract fixtures for runtime-to-sandbox provenance linkage in `daemon/internal/contracts/contracts_test.go`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/approval-resource.schema.json`, `schemas/api/decision-resource.schema.json`, and `schemas/api/sandbox-execution.resource.schema.json`

**Checkpoint**: Operators can reconstruct skill/local-tool execution paths and restart outcomes without logs only

---

## Phase 6: User Story 4 - Operators can verify no supported tool path bypasses sandbox (Priority: P4)

**Goal**: Align docs, contracts, and local verification so operators can prove the
supported executable-skill and high-risk local-tool paths no longer bypass sandbox

**Independent Test**: Follow the documented quickstart and contract validation flow, then
confirm docs, API/event surfaces, and operator-visible records all describe the same
execution, approval, provenance, and recovery behavior.

### Tests for User Story 4 ⚠️

- [X] T031 [P] [US4] Add end-to-end API and contract regression coverage for supported skill/local-tool sandbox alignment in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T032 [P] [US4] Add operator-facing restart/verification regression coverage for no-bypass execution paths in `daemon/internal/app/app_test.go` and `daemon/internal/sandbox/manager_test.go`

### Implementation for User Story 4

- [X] T033 [US4] Update operator guidance for executable-skill and local-tool sandbox behavior in `docs/harness/sandbox-execution-plane.md`, `docs/harness/skill-registry-and-prompt-support.md`, and `docs/runtime/operator-trust-model.md`
- [X] T034 [US4] Update verification procedure and delivery tracking for Roadmap 19 closure in `docs/runtime/daemon-tasks.md` and `specs/004-skill-tool-sandbox/quickstart.md`
- [X] T035 [P] [US4] Align contract notes and scope statements in `specs/004-skill-tool-sandbox/contracts/skill-tool-sandbox-surfaces.md`, `specs/004-skill-tool-sandbox/research.md`, and `specs/004-skill-tool-sandbox/data-model.md`

**Checkpoint**: Operators can validate skill/local-tool sandbox alignment through docs, routes, events, and runtime history

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, release-readiness evidence, and rollback closure

- [X] T036 [P] Run `make daemon-contract-test` and record results in `specs/004-skill-tool-sandbox/quickstart.md`
- [X] T037 [P] Run targeted daemon verification in `daemon/internal/skills`, `daemon/internal/api`, `daemon/internal/runtime`, `daemon/internal/sandbox`, `daemon/internal/store`, `daemon/internal/policy`, `daemon/internal/app`, and `daemon/internal/contracts`, then record results in `specs/004-skill-tool-sandbox/quickstart.md`
- [X] T038 [P] Run full daemon regression verification with `go test ./...` in `daemon/` and record results in `specs/004-skill-tool-sandbox/quickstart.md`
- [X] T039 Finalize roadmap closure, rollback, and scope-boundary notes in `docs/runtime/daemon-roadmaps.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational; establishes executable manifest and inspection truth
- **User Story 2 (Phase 4)**: Depends on User Story 1 because runtime execution needs executable manifest and projection behavior
- **User Story 3 (Phase 5)**: Depends on User Story 2 because provenance and restart recovery attach to launched runtime/sandbox executions
- **User Story 4 (Phase 6)**: Depends on User Stories 1, 2, and 3 because docs and verification must reflect the implemented execution path
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on later stories
- **User Story 2 (P2)**: Depends on User Story 1 because executable-skill execution requires manifest and inspection truth
- **User Story 3 (P3)**: Depends on User Story 2 because provenance and restart recovery depend on real sandbox-backed execution
- **User Story 4 (P4)**: Depends on User Stories 1, 2, and 3 because verification and docs must match the implemented behavior

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Type/persistence changes before route and schema closure
- Runtime and sandbox behavior before docs and quickstart evidence
- Story checkpoint must pass before moving to the next dependent story

### Parallel Opportunities

- `T002` and `T003` can run in parallel
- `T007` and `T008` can run in parallel
- `T009`, `T010`, and `T011` can run in parallel
- `T014` and `T015` can run in parallel after `T012`
- `T016`, `T017`, `T018`, `T040`, and `T041` can run in parallel
- `T022` and `T023` can run in parallel after execution behavior stabilizes
- `T024`, `T025`, and `T026` can run in parallel
- `T029` and `T030` can run in parallel after provenance behavior stabilizes
- `T031` and `T032` can run in parallel
- `T036`, `T037`, and `T038` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Add executable-skill manifest parsing, default approval, and unavailable-state regression tests in daemon/internal/skills/registry_test.go"
Task: "Add skill list/detail API regression tests for executable manifest and unavailable projection in daemon/internal/api/server_test.go"
Task: "Add schema and contract regression coverage for executable-skill inspection payloads in daemon/internal/contracts/contracts_test.go, schemas/api/skill-summary.schema.json, schemas/api/skill-detail.response.schema.json, and schemas/api/skill-registry.response.schema.json"

# Launch implementation work on different files in parallel:
Task: "Implement executable-manifest parsing, validation, and unavailable-state projection in daemon/internal/skills/registry.go"
Task: "Add execution-oriented skill declaration metadata and default ask approval handling in daemon/internal/skills/registry.go and daemon/internal/sandbox/types.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Verify executable-skill inspection, default approval posture, and
   `unavailable` visibility independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → foundation ready
2. Add User Story 1 → inspect executable skills independently → deploy/demo MVP
3. Add User Story 2 → validate sandbox-backed execution and approval behavior → deploy/demo
4. Add User Story 3 → validate provenance and restart recovery → deploy/demo
5. Add User Story 4 → validate docs and operator verification closure → roadmap ready

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1
   - Developer B: User Story 2 scaffolding after US1 contracts settle
   - Developer C: Contract/schema prep and verification scaffolding
3. Complete provenance/restart and docs phases after execution behavior stabilizes

---

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify required tests fail before implementing
- Stop at each checkpoint to validate the story independently
- Avoid hidden unmanaged subprocess paths, silent fallback behavior, and contract drift
