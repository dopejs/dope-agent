---

description: "Task list for MCP Execution Plane"

---

# Tasks: MCP Execution Plane

**Input**: Design documents from `/specs/003-mcp-execution-plane/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This roadmap changes execution-boundary, API, schema,
event, persistence, restart, and approval behavior, so targeted tests and contract
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
- Feature artifacts live under `specs/003-mcp-execution-plane/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable MCP fixtures, schema assertions, and verification helpers

- [x] T001 Create shared MCP server, tool catalog, and lifecycle fixture builders in `daemon/internal/mcp/manager_test.go`
- [x] T002 [P] Add reusable API assertion helpers for MCP server and tool resources in `daemon/internal/api/server_test.go`
- [x] T003 [P] Add reusable schema and event assertion helpers for MCP contracts in `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish MCP package, persistence, and consumer-contract primitives required
by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Define MCP server, lifecycle, tool catalog, and exposure rule types in `daemon/internal/mcp/types.go` and extend MCP consumer declarations in `daemon/internal/sandbox/types.go`
- [x] T005 Extend SQLite schema and persistence helpers for MCP servers, lifecycle state, tool catalog entries, and exposure rules in `daemon/internal/store/store.go` and `daemon/internal/store/store_test.go`
- [x] T006 Implement the MCP manager skeleton, stdio transport abstraction, store-backed restore hooks, and event publisher wiring in `daemon/internal/mcp/manager.go`, `daemon/internal/mcp/transport.go`, `daemon/internal/mcp/manager_test.go`, and `daemon/internal/app/app.go`
- [x] T007 [P] Add MCP resource projection helpers and route scaffolding in `daemon/internal/api/types.go` and `daemon/internal/api/server.go`
- [x] T008 [P] Extend sandbox consumer-view projection for `mcp_server` provenance in `daemon/internal/sandbox/manager.go`, `daemon/internal/api/types.go`, and `schemas/api/sandbox-consumer-view.schema.json`

**Checkpoint**: MCP package, persistence, and shared consumer-contract primitives are ready
for story work

---

## Phase 3: User Story 1 - MCP servers are registered with explicit sandbox policy (Priority: P1)

**Goal**: Add first-class daemon-managed MCP server resources with explicit sandbox
profile and declaration binding

**Independent Test**: Register and inspect MCP servers through daemon-visible routes, then
verify valid servers expose identity/profile/declaration state and invalid servers surface
an operator-visible failure reason without any unmanaged launch path.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T009 [P] [US1] Add MCP server registration, update, and inspection API regression tests in `daemon/internal/api/server_test.go`
- [x] T010 [P] [US1] Add MCP manager validation and persistence tests for profile and declaration binding in `daemon/internal/mcp/manager_test.go` and `daemon/internal/store/store_test.go`

### Implementation for User Story 1

- [x] T011 [US1] Implement MCP server register, update, enable, disable, and inspect logic in `daemon/internal/mcp/manager.go` and `daemon/internal/mcp/types.go`
- [x] T012 [US1] Implement MCP server resource routes in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [x] T013 [P] [US1] Add MCP server API schemas in `schemas/api/mcp-server-resource.schema.json`, `schemas/api/mcp-server-list.response.schema.json`, `schemas/api/mcp-server-create.request.schema.json`, and `schemas/api/mcp-server-update.request.schema.json`
- [x] T014 [US1] Extend additive config and sandbox explain projection for MCP registry binding in `daemon/internal/api/server.go`, `daemon/internal/sandbox/manager.go`, `schemas/api/config.response.schema.json`, and `schemas/api/sandbox-explain.response.schema.json`

**Checkpoint**: MCP servers are first-class daemon-managed resources with explicit sandbox
binding and independently testable inspection behavior

---

## Phase 4: User Story 2 - MCP lifecycle runs through sandbox-managed execution (Priority: P2)

**Goal**: Launch, stop, restart, and recover MCP servers through sandbox-backed lifecycle
management with explicit state and failure classification

**Independent Test**: Start, stop, restart, and recover representative MCP servers, then
verify lifecycle attempts run through sandbox-backed execution, classify failure modes
explicitly, and auto-restart previously enabled servers after daemon restart when current
policy and config are still valid.

### Tests for User Story 2 ⚠️

- [x] T015 [P] [US2] Add lifecycle start, stop, restart, cancel, timeout, failure-class, and `<=100 ms` preflight timing regression tests in `daemon/internal/mcp/manager_test.go` and `daemon/internal/api/server_test.go`
- [x] T016 [P] [US2] Add daemon restart and blocked auto-restart regression tests in `daemon/internal/app/app_test.go` and `daemon/internal/store/store_test.go`

### Implementation for User Story 2

- [x] T017 [US2] Implement dedicated stdio MCP transport session handling, handshake, discovery, health tracking, and cancellation plumbing in `daemon/internal/mcp/transport.go` and `daemon/internal/mcp/manager.go`
- [x] T018 [US2] Implement sandbox-backed MCP lifecycle orchestration for start, stop, restart, and cancellation in `daemon/internal/mcp/manager.go` and `daemon/internal/sandbox/manager.go`
- [x] T019 [US2] Implement MCP runtime state transitions, health tracking, and restart backoff in `daemon/internal/mcp/manager.go` and `daemon/internal/mcp/types.go`
- [x] T020 [US2] Wire MCP restore and auto-restart behavior into daemon bootstrap in `daemon/internal/app/app.go` and `daemon/internal/app/app_test.go`
- [x] T021 [P] [US2] Add MCP lifecycle action routes, cancellation handling, state projection updates, and lifecycle action response schemas in `daemon/internal/api/server.go`, `daemon/internal/api/types.go`, `schemas/api/mcp-server-lifecycle.response.schema.json`, and `daemon/internal/contracts/contracts_test.go`
- [x] T022 [US2] Add lifecycle event schemas and contract assertions in `schemas/events/mcp-server-registered.event.schema.json`, `schemas/events/mcp-server-updated.event.schema.json`, `schemas/events/mcp-server-started.event.schema.json`, `schemas/events/mcp-server-stopped.event.schema.json`, `schemas/events/mcp-server-failed.event.schema.json`, `schemas/events/mcp-server-health-changed.event.schema.json`, and `daemon/internal/contracts/contracts_test.go`

**Checkpoint**: MCP lifecycle is sandbox-backed, restart-safe, and independently testable
with explicit operator-visible state

---

## Phase 5: User Story 3 - MCP credentials and tool exposure stay policy-driven (Priority: P3)

**Goal**: Enforce per-server credential scope, persist tool catalog state, and expose
tools only through explicit per-tool/per-surface policy with tool-level approval gating

**Independent Test**: Configure secret-backed MCP servers and tool exposure rules, then
verify credentials resolve per server instance, redaction remains intact, approval applies
at tool exposure, and unallowlisted or unhealthy tools remain unavailable.

### Tests for User Story 3 ⚠️

- [x] T023 [P] [US3] Add MCP credential-scope, unavailable-secret, and redaction regression tests in `daemon/internal/mcp/manager_test.go` and `daemon/internal/api/server_test.go`
- [x] T024 [P] [US3] Add tool catalog, allowlist, and approval-gating regression tests in `daemon/internal/mcp/manager_test.go` and `daemon/internal/policy/policy_test.go`

### Implementation for User Story 3

- [x] T025 [US3] Extend sandbox declaration, secret binding, and policy-record handling for `consumer_kind=mcp_server` in `daemon/internal/sandbox/types.go`, `daemon/internal/sandbox/manager.go`, and `daemon/internal/store/store.go`
- [x] T026 [US3] Implement MCP tool catalog discovery, persistence, and stale-tool handling in `daemon/internal/mcp/manager.go` and `daemon/internal/store/store.go`
- [x] T027 [US3] Implement per-tool per-runtime-surface exposure rules and approval-aware decisions in `daemon/internal/mcp/manager.go`, `daemon/internal/api/server.go`, and `daemon/internal/policy/policy.go`
- [x] T028 [P] [US3] Add MCP tool, tool exposure update, and credential-projection schemas in `schemas/api/mcp-tool-resource.schema.json`, `schemas/api/mcp-tool-list.response.schema.json`, `schemas/api/mcp-tool-exposure-update.request.schema.json`, `schemas/api/approval-resource.schema.json`, `schemas/api/decision-resource.schema.json`, and `schemas/api/config.response.schema.json`
- [x] T029 [US3] Add tool exposure and approval event schemas plus contract assertions in `schemas/events/mcp-tool-exposure-updated.event.schema.json`, `schemas/events/policy-approval-requested.event.schema.json`, `schemas/events/policy-approval-resolved.event.schema.json`, `schemas/events/policy-decision-recorded.event.schema.json`, and `daemon/internal/contracts/contracts_test.go`

**Checkpoint**: MCP credentials and tools are governed by explicit per-server and per-tool
policy, with independent tests for redaction, approval, and allowlist behavior

---

## Phase 6: User Story 4 - Operators can verify MCP and sandbox stay aligned (Priority: P4)

**Goal**: Align MCP docs, audit surfaces, and verification coverage so operators can prove
MCP runs through sandbox rather than a side path

**Independent Test**: Follow the MCP quickstart flow and inspect daemon-visible routes,
events, and audit records to verify the same registry, lifecycle, credential, and tool
exposure truth is visible across docs and runtime surfaces.

### Tests for User Story 4 ⚠️

- [x] T030 [P] [US4] Add API and contract regression coverage for MCP routes, additive sandbox provenance, and event history in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`
- [x] T031 [P] [US4] Add end-to-end sandbox-alignment and event-history verification tests in `daemon/internal/mcp/manager_test.go` and `daemon/internal/app/app_test.go`

### Implementation for User Story 4

- [x] T032 [US4] Update MCP operator guidance in `docs/harness/sandbox-execution-plane.md` and `docs/harness/harness-architecture.md`
- [x] T033 [US4] Update roadmap, task, and operator-trust documentation for MCP closure in `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-tasks.md`, and `docs/runtime/operator-trust-model.md`
- [x] T034 [US4] Align local verification workflow and contract notes in `specs/003-mcp-execution-plane/quickstart.md` and `specs/003-mcp-execution-plane/contracts/mcp-sandbox-surfaces.md`

**Checkpoint**: Operators can validate MCP-sandbox alignment through docs, routes, events,
and audit surfaces without relying on undocumented behavior

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, readiness evidence, and rollback closure

- [x] T035 [P] Run `make daemon-contract-test` and record results in `specs/003-mcp-execution-plane/quickstart.md`
- [x] T036 [P] Run targeted daemon verification in `daemon/internal/mcp`, `daemon/internal/sandbox`, `daemon/internal/api`, `daemon/internal/app`, `daemon/internal/store`, `daemon/internal/policy`, and `daemon/internal/contracts`, including recorded lifecycle preflight timing coverage, then record results in `specs/003-mcp-execution-plane/quickstart.md`
- [x] T037 [P] Run full daemon regression verification with `go test ./...` in `daemon/` and record results in `specs/003-mcp-execution-plane/quickstart.md`
- [x] T038 Finalize rollback and roadmap-readiness notes in `specs/003-mcp-execution-plane/quickstart.md` and `docs/runtime/daemon-roadmaps.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational; establishes first-class MCP server resources and is the first independently verifiable slice inside the roadmap
- **User Story 2 (Phase 4)**: Depends on User Story 1 because lifecycle work requires persisted MCP server definitions and API resources
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 because credential scope and tool exposure depend on managed server identity plus working lifecycle state
- **User Story 4 (Phase 6)**: Depends on User Stories 1, 2, and 3 because docs and verification must reflect the full MCP subsystem behavior
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on later stories
- **User Story 2 (P2)**: Depends on User Story 1 because lifecycle starts from registered MCP server resources
- **User Story 3 (P3)**: Depends on User Stories 1 and 2 because credentials and exposure policy attach to managed server identity and live state
- **User Story 4 (P4)**: Depends on User Stories 1, 2, and 3 because verification and docs must match the implemented subsystem

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Persistence and type changes before route and schema closure
- Core behavior before docs and quickstart evidence
- Story checkpoint must pass before moving to the next dependent story

### Parallel Opportunities

- `T002` and `T003` can run in parallel
- `T007` and `T008` can run in parallel
- `T009` and `T010` can run in parallel
- `T013` can run in parallel with `T014` after `T012`
- `T015` and `T016` can run in parallel
- `T021` and `T022` can run in parallel after lifecycle behavior stabilizes
- `T023` and `T024` can run in parallel
- `T028` and `T029` can run in parallel after exposure behavior stabilizes
- `T030` and `T031` can run in parallel
- `T035`, `T036`, and `T037` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 1

```bash
# Launch MCP registry regression coverage together:
Task: "Add MCP server registration, update, and inspection API regression tests in daemon/internal/api/server_test.go"
Task: "Add MCP manager validation and persistence tests for profile and declaration binding in daemon/internal/mcp/manager_test.go and daemon/internal/store/store_test.go"

# Launch schema and projection closure together after route work:
Task: "Add MCP server API schemas in schemas/api/mcp-server-resource.schema.json, schemas/api/mcp-server-list.response.schema.json, schemas/api/mcp-server-create.request.schema.json, and schemas/api/mcp-server-update.request.schema.json"
Task: "Extend additive config and sandbox explain projection for MCP registry binding in daemon/internal/api/server.go, daemon/internal/sandbox/manager.go, schemas/api/config.response.schema.json, and schemas/api/sandbox-explain.response.schema.json"
```

## Parallel Example: User Story 2

```bash
# Launch lifecycle coverage together:
Task: "Add lifecycle start, stop, restart, cancel, timeout, failure-class, and <=100 ms preflight timing regression tests in daemon/internal/mcp/manager_test.go and daemon/internal/api/server_test.go"
Task: "Add daemon restart and blocked auto-restart regression tests in daemon/internal/app/app_test.go and daemon/internal/store/store_test.go"

# Close route and schema work together after lifecycle state is implemented:
Task: "Add MCP lifecycle action routes, cancellation handling, state projection updates, and lifecycle action response schemas in daemon/internal/api/server.go, daemon/internal/api/types.go, schemas/api/mcp-server-lifecycle.response.schema.json, and daemon/internal/contracts/contracts_test.go"
Task: "Add lifecycle event schemas and contract assertions in schemas/events/mcp-server-registered.event.schema.json, schemas/events/mcp-server-updated.event.schema.json, schemas/events/mcp-server-started.event.schema.json, schemas/events/mcp-server-stopped.event.schema.json, schemas/events/mcp-server-failed.event.schema.json, schemas/events/mcp-server-health-changed.event.schema.json, and daemon/internal/contracts/contracts_test.go"
```

## Parallel Example: User Story 3

```bash
# Launch credential and exposure coverage together:
Task: "Add MCP credential-scope, unavailable-secret, and redaction regression tests in daemon/internal/mcp/manager_test.go and daemon/internal/api/server_test.go"
Task: "Add tool catalog, allowlist, and approval-gating regression tests in daemon/internal/mcp/manager_test.go and daemon/internal/policy/policy_test.go"

# Close schema and event work together after behavior is implemented:
Task: "Add MCP tool, tool exposure update, and credential-projection schemas in schemas/api/mcp-tool-resource.schema.json, schemas/api/mcp-tool-list.response.schema.json, schemas/api/mcp-tool-exposure-update.request.schema.json, schemas/api/approval-resource.schema.json, schemas/api/decision-resource.schema.json, and schemas/api/config.response.schema.json"
Task: "Add tool exposure and approval event schemas plus contract assertions in schemas/events/mcp-tool-exposure-updated.event.schema.json, schemas/events/policy-approval-requested.event.schema.json, schemas/events/policy-approval-resolved.event.schema.json, schemas/events/policy-decision-recorded.event.schema.json, and daemon/internal/contracts/contracts_test.go"
```

## Parallel Example: User Story 4

```bash
# Launch verification coverage together:
Task: "Add API and contract regression coverage for MCP routes, additive sandbox provenance, and event history in daemon/internal/api/server_test.go and daemon/internal/contracts/contracts_test.go"
Task: "Add end-to-end sandbox-alignment and event-history verification tests in daemon/internal/mcp/manager_test.go and daemon/internal/app/app_test.go"
```

## Implementation Strategy

### Validation Order Inside One Roadmap Closure

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **VALIDATE LOCALLY**: Register and inspect MCP servers independently before starting lifecycle work
5. Continue through the remaining story phases before treating Roadmap 18 as complete

### Incremental Delivery

1. Complete Setup + Foundational → MCP package, persistence, and consumer-contract primitives ready
2. Complete User Story 1 → validate first-class MCP registry and explicit sandbox binding
3. Complete User Story 2 → validate sandbox-backed lifecycle, cancellation, and restart-safe recovery
4. Complete User Story 3 → validate per-server credential scope and explicit tool exposure policy
5. Complete User Story 4 → validate docs, audit surfaces, and operator verification
6. Finish with contract, targeted, timing, and full-suite verification plus rollback notes

### Parallel Team Strategy

With multiple developers:

1. One developer handles foundational MCP package and persistence work
2. After Foundational:
   - Developer A: registry and lifecycle manager work in `daemon/internal/mcp/` and `daemon/internal/app/`
   - Developer B: API route and schema closure in `daemon/internal/api/` and `schemas/api/`
   - Developer C: sandbox, policy, approval, and contract alignment in `daemon/internal/sandbox/`, `daemon/internal/policy/`, `schemas/events/`, and `daemon/internal/contracts/`

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] labels map each story task back to the corresponding user story in `spec.md`
- Each user story is independently testable at its checkpoint
- MCP tool exposure remains deny-by-default until a per-tool, per-surface rule is present
- Routine MCP server lifecycle work stays daemon-managed; approval attaches to tool exposure only
- Avoid hidden launch paths, profile-wide secret sharing, or docs that imply stronger backend guarantees than `subprocess` actually provides
