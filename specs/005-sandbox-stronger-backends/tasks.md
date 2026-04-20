---

description: "Task list for Sandbox Stronger Backends"

---

# Tasks: Sandbox Stronger Backends

**Input**: Design documents from `/specs/005-sandbox-stronger-backends/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This roadmap changes sandbox inspection,
selection, execution, persistence, runtime provenance, and schema-backed
contracts, so targeted daemon tests and contract verification are required.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependency)
- **[Story]**: Which user story this belongs to (`US1`, `US2`, `US3`)
- Include exact file paths in descriptions

## Path Conventions

- Daemon code lives under `daemon/internal/...`
- API and event schemas live under `schemas/api/` and `schemas/events/`
- Feature artifacts live under `specs/005-sandbox-stronger-backends/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable stronger-backend fixtures, executable-skill
manifests, and contract assertions for Roadmap 20

- [x] T001 Create reusable `docker` backend fixture helpers and host-availability test scaffolding in `daemon/internal/sandbox/manager_test.go`
- [x] T002 [P] Add executable-skill fixture helpers for baseline, explicit-`docker`, and `docker`-required manifests in `daemon/internal/skills/registry_test.go`
- [x] T003 [P] Add reusable API and contract assertion helpers for backend capability, unsupported outcomes, and stronger-backend provenance in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish backend-capability, host-prerequisite, and persistence
primitives required by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Define additive backend capability, host status, and backend-selection types in `daemon/internal/sandbox/types.go` and `daemon/internal/api/types.go`
- [x] T005 Implement shared host-capability detection, backend readiness caching, and startup/restore plumbing in `daemon/internal/sandbox/manager.go` and `daemon/internal/app/app.go`
- [x] T006 Extend runtime and SQLite persistence for backend selection, host prerequisite snapshots, and unsupported terminal states in `daemon/internal/runtime/runtime.go`, `daemon/internal/store/store.go`, and `daemon/internal/store/store_test.go`
- [x] T007 [P] Extend contract fixture vocabulary for backend capability, explain selection, and stronger-backend execution payloads in `daemon/internal/contracts/contracts_test.go`
- [x] T008 [P] Add additive schema scaffolding for backend capability and selection fields in `schemas/api/sandbox-profile.schema.json`, `schemas/api/sandbox-profile-list.response.schema.json`, `schemas/api/sandbox-explain.response.schema.json`, `schemas/api/sandbox-decision.schema.json`, `schemas/api/sandbox-result.schema.json`, and `schemas/api/sandbox-execution.resource.schema.json`

**Checkpoint**: Backend-capability, readiness, and persistence foundations are
ready for story work

---

## Phase 3: User Story 1 - Operators Can Inspect Backend Guarantees (Priority: P1) 🎯 MVP

**Goal**: Make sandbox inspection and explain surfaces state truthful backend
guarantees, prerequisites, and mismatch outcomes without requiring code reading

**Independent Test**: Inspect sandbox profiles, explain results, and operator
guidance for representative workloads, then verify the operator can determine
whether `subprocess` is sufficient, `docker` is required, or the request is
`unsupported`.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T009 [P] [US1] Add backend capability, availability, and explain mismatch regression tests in `daemon/internal/sandbox/manager_test.go`
- [x] T010 [P] [US1] Add profile list/detail, explain, and config projection regression tests in `daemon/internal/api/server_test.go`
- [x] T011 [P] [US1] Add contract regression coverage for backend capability and explain payloads in `daemon/internal/contracts/contracts_test.go`, `schemas/api/sandbox-profile.schema.json`, `schemas/api/sandbox-profile-list.response.schema.json`, `schemas/api/sandbox-explain.response.schema.json`, `schemas/api/sandbox-decision.schema.json`, `schemas/api/sandbox-result.schema.json`, and `schemas/api/config.response.schema.json`

### Implementation for User Story 1

- [x] T012 [US1] Implement backend capability matrix, host-prerequisite evaluation, and truthful explain selection outcomes in `daemon/internal/sandbox/manager.go` and `daemon/internal/sandbox/types.go`
- [x] T013 [US1] Project backend capability, availability, and mismatch truth through API response builders and routes in `daemon/internal/api/types.go` and `daemon/internal/api/server.go`
- [x] T014 [P] [US1] Update schema-backed API contracts for profile, explain, and config capability fields in `schemas/api/sandbox-profile.schema.json`, `schemas/api/sandbox-profile-list.response.schema.json`, `schemas/api/sandbox-explain.response.schema.json`, `schemas/api/sandbox-decision.schema.json`, `schemas/api/sandbox-result.schema.json`, and `schemas/api/config.response.schema.json`
- [x] T015 [US1] Update operator-facing explain and inspection guidance in `docs/harness/sandbox-execution-plane.md`

**Checkpoint**: Operators can inspect backend guarantees, prerequisites, and
unsupported outcomes through daemon-visible surfaces and committed docs

---

## Phase 4: User Story 2 - Higher-Risk Consumers Can Require Stronger Isolation (Priority: P2)

**Goal**: Allow explicitly declared executable skills to require `docker`
through the existing sandbox execution plane with truthful unsupported and
provenance behavior

**Independent Test**: Configure an executable skill that explicitly requires
`docker`, then verify successful launch, unsupported host mismatch, timeout,
cancellation, and provenance inspection all flow through the current sandbox and
runtime surfaces without silent fallback.

### Tests for User Story 2 ⚠️

- [x] T016 [P] [US2] Add executable-skill registry tests for explicit `docker` selection, opt-in defaults, and unavailable-state projection in `daemon/internal/skills/registry_test.go`
- [x] T017 [P] [US2] Add skill-backed tool-call regression tests for `docker` launch, unsupported mismatch, approval, and provenance in `daemon/internal/api/server_test.go` and `daemon/internal/runtime/runtime_test.go`
- [x] T018 [P] [US2] Add stronger-backend launch, access-rule mismatch, timeout, cancellation, and restart regression tests in `daemon/internal/sandbox/manager_test.go` and `daemon/internal/app/app_test.go`
- [x] T019 [P] [US2] Add timing regression coverage for backend selection and explain/execution preflight staying within `<=100 ms` in `daemon/internal/sandbox/manager_test.go` and `daemon/internal/api/server_test.go`
- [x] T020 [P] [US2] Add contract regression coverage for `docker` executable-skill inspection, decision/result mismatch classes, tool-call, sandbox execution, and event payloads in `daemon/internal/contracts/contracts_test.go`, `schemas/api/executable-skill-manifest.schema.json`, `schemas/api/skill-summary.schema.json`, `schemas/api/skill-detail.response.schema.json`, `schemas/api/skill-registry.response.schema.json`, `schemas/api/sandbox-decision.schema.json`, `schemas/api/sandbox-result.schema.json`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/tool-call-list.response.schema.json`, `schemas/api/sandbox-execution.resource.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, `schemas/events/tool-call-failed.event.schema.json`, `schemas/events/sandbox-execution-requested.event.schema.json`, `schemas/events/sandbox-execution-started.event.schema.json`, `schemas/events/sandbox-execution-completed.event.schema.json`, `schemas/events/sandbox-execution-failed.event.schema.json`, and `schemas/events/sandbox-execution-cancelled.event.schema.json`

### Implementation for User Story 2

- [x] T021 [US2] Extend executable-skill manifest parsing and inspection projection for explicit backend requirements in `daemon/internal/skills/registry.go` and `daemon/internal/api/types.go`
- [x] T022 [US2] Implement `docker` backend selection, prerequisite enforcement, stronger filesystem/network enforcement, launch-time access-rule mismatch classification, and fail-closed `unsupported` behavior in `daemon/internal/sandbox/manager.go`
- [x] T023 [US2] Integrate skill-backed tool calls with stronger-backend selection, approval handling, and runtime linkage in `daemon/internal/api/server.go` and `daemon/internal/runtime/runtime.go`
- [x] T024 [US2] Persist stronger-backend identity, host prerequisite snapshots, unsupported outcomes, mismatch classes, and recovery state in `daemon/internal/store/store.go`
- [x] T025 [P] [US2] Update additive schema contracts for executable-skill backend requirements, mismatch classification, and stronger-backend runtime resources in `schemas/api/executable-skill-manifest.schema.json`, `schemas/api/skill-summary.schema.json`, `schemas/api/skill-detail.response.schema.json`, `schemas/api/skill-registry.response.schema.json`, `schemas/api/sandbox-decision.schema.json`, `schemas/api/sandbox-result.schema.json`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/tool-call-list.response.schema.json`, and `schemas/api/sandbox-execution.resource.schema.json`
- [x] T026 [P] [US2] Update additive event contracts for stronger-backend execution and skill-backed runtime outcomes in `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, `schemas/events/tool-call-failed.event.schema.json`, `schemas/events/sandbox-execution-requested.event.schema.json`, `schemas/events/sandbox-execution-started.event.schema.json`, `schemas/events/sandbox-execution-completed.event.schema.json`, `schemas/events/sandbox-execution-failed.event.schema.json`, and `schemas/events/sandbox-execution-cancelled.event.schema.json`
- [x] T027 [US2] Preserve test-versus-production separation, secret handling, and operator-visible audit truth for `docker` executable skills in `daemon/internal/skills/registry.go`, `daemon/internal/api/server.go`, and `daemon/internal/sandbox/manager.go`

**Checkpoint**: Explicitly declared executable skills can require `docker`
through the existing sandbox plane with truthful unsupported, approval, and
provenance behavior

---

## Phase 5: User Story 3 - Teams Can Continue Sandbox Migration Without Losing Context (Priority: P3)

**Goal**: Capture backend capability matrix, remaining consumer inventory,
degradation rules, and host prerequisites in durable artifacts for follow-on
sandbox work

**Independent Test**: Review the roadmap artifacts and operator docs after the
change, then verify a new engineer can identify which backends exist, which
consumer families are already sandbox-backed, which families remain deferred,
and what prerequisites block future stronger-backend rollout.

### Implementation for User Story 3

- [x] T028 [P] [US3] Refresh committed backend capability matrix, selection rules, and migration entities in `specs/005-sandbox-stronger-backends/research.md`, `specs/005-sandbox-stronger-backends/data-model.md`, and `specs/005-sandbox-stronger-backends/contracts/sandbox-stronger-backend-surfaces.md`
- [x] T029 [US3] Update Roadmap 20 scope, migration inventory, and stronger-backend sequence in `docs/runtime/daemon-roadmaps.md`
- [x] T030 [US3] Update backend tradeoff, host prerequisite, degradation, and deferred-family guidance in `docs/harness/sandbox-backend-comparison.md` and `docs/runtime/operator-trust-model.md`
- [x] T031 [P] [US3] Add explicit operator-facing acceptance and recording steps for the `<=5 minute` inspection goal in `specs/005-sandbox-stronger-backends/quickstart.md` and `docs/runtime/daemon-tasks.md`

**Checkpoint**: Future sandbox work can continue from committed artifacts
instead of reconstructing backend and migration intent from history

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, release-readiness evidence, and scope closure

- [x] T032 [P] Run `make daemon-contract-test` and record results in `specs/005-sandbox-stronger-backends/quickstart.md`
- [x] T033 [P] Run targeted daemon verification in `daemon/internal/sandbox`, `daemon/internal/api`, `daemon/internal/skills`, `daemon/internal/runtime`, `daemon/internal/store`, `daemon/internal/app`, and `daemon/internal/contracts`, then record results in `specs/005-sandbox-stronger-backends/quickstart.md`
- [x] T034 [P] Run full daemon regression verification with `go test ./...` in `daemon/` and record results in `specs/005-sandbox-stronger-backends/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational; establishes inspection and explain truth
- **User Story 2 (Phase 4)**: Depends on User Story 1 because stronger-backend execution needs capability and mismatch semantics first
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 because roadmap artifacts and operator docs must reflect the implemented capability and migration boundary
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on later stories
- **User Story 2 (P2)**: Depends on User Story 1 because executable-skill `docker` rollout relies on truthful backend capability and explain behavior
- **User Story 3 (P3)**: Depends on User Stories 1 and 2 because documentation and migration inventory must match the implemented backend posture

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Type, persistence, and schema changes before route and doc closure
- Sandbox selection behavior before runtime linkage and provenance assertions
- Story checkpoint must pass before moving to the next dependent story

### Parallel Opportunities

- `T002` and `T003` can run in parallel
- `T007` and `T008` can run in parallel
- `T009`, `T010`, and `T011` can run in parallel
- `T014` and `T015` can run in parallel after `T012` and `T013`
- `T016`, `T017`, `T018`, `T019`, and `T020` can run in parallel
- `T024`, `T025`, and `T026` can run in parallel after launch behavior stabilizes
- `T028` and `T031` can run in parallel
- `T032`, `T033`, and `T034` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 2

```bash
# Launch all tests for User Story 2 together:
Task: "Add executable-skill registry tests for explicit docker selection, opt-in defaults, and unavailable-state projection in daemon/internal/skills/registry_test.go"
Task: "Add skill-backed tool-call regression tests for docker launch, unsupported mismatch, approval, and provenance in daemon/internal/api/server_test.go and daemon/internal/runtime/runtime_test.go"
Task: "Add stronger-backend launch, timeout, cancellation, and restart regression tests in daemon/internal/sandbox/manager_test.go and daemon/internal/app/app_test.go"
Task: "Add contract regression coverage for docker executable-skill inspection, mismatch classes, tool-call, sandbox execution, and event payloads in daemon/internal/contracts/contracts_test.go and schemas/"

# Launch implementation work on different files in parallel:
Task: "Extend executable-skill manifest parsing and inspection projection for explicit backend requirements in daemon/internal/skills/registry.go and daemon/internal/api/types.go"
Task: "Implement docker backend selection, prerequisite enforcement, stronger filesystem/network enforcement, launch-time access-rule mismatch classification, and fail-closed unsupported behavior in daemon/internal/sandbox/manager.go"
Task: "Persist stronger-backend identity, host prerequisite snapshots, unsupported outcomes, mismatch classes, and recovery state in daemon/internal/store/store.go"
```

---

## Implementation Strategy

### Validation First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **VALIDATE CHECKPOINT ONLY**: Verify backend capability inspection, explain mismatch
   truth, and operator guidance independently
5. Continue with the remaining user stories; this checkpoint is not roadmap closure and
   must not be treated as shippable completion

### Incremental Delivery

1. Complete Setup + Foundational → capability foundation ready
2. Add User Story 1 → validate operator inspection and explain truth
3. Add User Story 2 → validate `docker` executable-skill execution, enforcement, and unsupported semantics
4. Add User Story 3 → validate migration inventory, operator acceptance flow, and follow-on guidance
5. Claim roadmap closure only after all story phases and final verification complete

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 capability inspection and explain work
   - Developer B: User Story 2 executable-skill `docker` execution and provenance work
   - Developer C: User Story 3 docs and migration inventory closure after US1/US2 semantics stabilize
3. Finish with shared contract and regression verification in Polish

---

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] label maps each story task to a user story for traceability
- Each user story should be independently completable and testable
- Verify required tests fail before implementing
- Commit after each task or logical group
- Stop at story checkpoints to validate independently
- Avoid: vague tasks, wildcard paths, silent backend fallback, or docs that drift from implemented runtime truth
