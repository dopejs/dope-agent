---

description: "Task list for Additional MCP Transports"

---

# Tasks: Additional MCP Transports

**Input**: Design documents from `/specs/008-additional-mcp-transports/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This roadmap changes MCP transport execution,
API/schema/event contracts, runtime tool-call provenance, reconnect or restore
behavior, and operator-visible audit surfaces, so targeted daemon tests and
contract verification are required.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependency)
- **[Story]**: Which user story this belongs to (`US1`, `US2`, `US3`)
- Include exact file paths in descriptions

## Path Conventions

- Daemon code lives under `daemon/internal/...` and `daemon/cmd/...`
- API and event schemas live under `schemas/api/` and `schemas/events/`
- Feature artifacts live under `specs/008-additional-mcp-transports/`
- Roadmap and operator docs live under `docs/harness/` and `docs/runtime/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable websocket transport fixtures, helper-server test
support, and contract assertions for Roadmap 23

- [x] T001 Create reusable websocket MCP fixture builders, disconnect controls, and transport-capability helpers in `daemon/internal/mcp/manager_test.go`
- [x] T002 [P] Add reusable API assertion helpers for transport capability responses, websocket auth summaries, and reconnect-state inspection in `daemon/internal/api/mcp_server_test.go` and `daemon/internal/api/server_test.go`
- [x] T003 [P] Add reusable contract assertion helpers for transport-capability, websocket server-resource, and reconnect event payloads in `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish shared types, transport scaffolding, and contract
primitives required by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Define additive transport capability, websocket config, websocket auth, and recovery snapshot types in `daemon/internal/mcp/types.go` and `daemon/internal/api/types.go`
- [x] T005 Extend MCP transport mux, manager state, and persistence primitives for websocket transport and recovery tracking in `daemon/internal/mcp/transport.go`, `daemon/internal/mcp/manager.go`, and `daemon/internal/store/store.go`
- [x] T006 [P] Add base schema scaffolding for transport capability inspection and websocket-compatible MCP resources in `schemas/api/mcp-transport-capability.schema.json`, `schemas/api/mcp-transport-capability-list.response.schema.json`, `schemas/api/config.response.schema.json`, `schemas/api/mcp-server-create.request.schema.json`, `schemas/api/mcp-server-update.request.schema.json`, and `schemas/api/mcp-server-resource.schema.json`
- [x] T007 [P] Add base event and contract scaffolding for websocket transport identity and reconnect history in `daemon/internal/contracts/contracts_test.go`, `schemas/events/mcp-server-registered.event.schema.json`, `schemas/events/mcp-server-updated.event.schema.json`, `schemas/events/mcp-server-started.event.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, and `schemas/events/tool-call-failed.event.schema.json`

**Checkpoint**: Shared websocket transport primitives are ready for story work

---

## Phase 3: User Story 1 - Inspect Transport Capability Truth (Priority: P1) 🎯 MVP

**Goal**: Let operators inspect explicit MCP transport readiness, prerequisite,
and mismatch truth before attempting to run a server

**Independent Test**: Inspect daemon transport surfaces and representative MCP
server definitions across supported transport families, then verify the daemon
returns explicit ready, blocked, unavailable, unsupported, or degraded truth
without requiring raw logs.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T008 [P] [US1] Add API regression tests for `GET /v1/mcp/transports`, additive `/v1/config` MCP transport projection, and explicit ready, blocked, unavailable, unsupported, and degraded capability truth in `daemon/internal/api/mcp_server_test.go` and `daemon/internal/api/server_test.go`
- [x] T009 [P] [US1] Add MCP manager regression tests for environment-scoped transport readiness, prerequisite classification, server-specific mismatch handling, and `<=100 ms` capability-preflight timing in `daemon/internal/mcp/manager_test.go`
- [x] T010 [P] [US1] Add contract regression coverage for transport-capability item and list responses plus additive config or server-resource projection in `daemon/internal/contracts/contracts_test.go`, `schemas/api/mcp-transport-capability.schema.json`, `schemas/api/mcp-transport-capability-list.response.schema.json`, `schemas/api/config.response.schema.json`, and `schemas/api/mcp-server-resource.schema.json`

### Implementation for User Story 1

- [x] T011 [US1] Implement transport capability computation for `stdio`, `streamable-http`, and `websocket` with environment-scoped prerequisite truth in `daemon/internal/mcp/manager.go`
- [x] T012 [US1] Add `GET /v1/mcp/transports` and additive `/v1/config` MCP transport projection in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [x] T013 [US1] Project websocket-capable server resource summaries, redacted auth readiness, and transport-specific availability reasoning in `daemon/internal/mcp/manager.go` and `daemon/internal/api/types.go`
- [x] T014 [P] [US1] Update API schemas for transport capability inspection and websocket-compatible server resources in `schemas/api/mcp-transport-capability.schema.json`, `schemas/api/mcp-transport-capability-list.response.schema.json`, `schemas/api/config.response.schema.json`, and `schemas/api/mcp-server-resource.schema.json`
- [x] T015 [P] [US1] Update roadmap and operator docs for transport capability inspection, host prerequisites, and explicit mismatch truth in `docs/runtime/daemon-roadmaps.md` and `docs/runtime/operator-trust-model.md`

**Checkpoint**: Operators can explain whether each supported MCP transport is
usable on the current host before lifecycle start attempts

---

## Phase 4: User Story 2 - Run A Real Server On A New Transport (Priority: P2)

**Goal**: Make `websocket` the first additional MCP transport family that runs
through the existing daemon-owned manager, lifecycle, authorization, and
runtime tool-call plane

**Independent Test**: Register one websocket MCP server, start it, discover
tools, invoke one tool through the existing runtime tool-call plane, and verify
the daemon does not fork into a transport-specific side path.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T016 [P] [US2] Add API and runtime regression tests for websocket server register/start, tool discovery, runtime tool-call provenance, auth-missing blocked truth, and redacted auth projection in `daemon/internal/api/mcp_server_test.go`, `daemon/internal/api/server_test.go`, and `daemon/internal/runtime/runtime_test.go`
- [x] T017 [P] [US2] Add MCP manager and app regression tests for websocket initialize, session health, tool invocation, restart restore, and `<=100 ms` websocket start-preflight timing in `daemon/internal/mcp/manager_test.go` and `daemon/internal/app/mcp_app_test.go`
- [x] T018 [P] [US2] Add contract regression coverage for websocket request, resource, tool-call, and lifecycle event surfaces in `daemon/internal/contracts/contracts_test.go`, `schemas/api/mcp-server-create.request.schema.json`, `schemas/api/mcp-server-update.request.schema.json`, `schemas/api/mcp-server-resource.schema.json`, `schemas/api/tool-call-resource.schema.json`, `schemas/events/mcp-server-registered.event.schema.json`, `schemas/events/mcp-server-updated.event.schema.json`, `schemas/events/mcp-server-started.event.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, and `schemas/events/tool-call-failed.event.schema.json`

### Implementation for User Story 2

- [x] T019 [US2] Implement websocket MCP transport session open, initialize, list-tools, call-tool, and close behavior in `daemon/internal/mcp/websocket.go` and `daemon/internal/mcp/transport.go`
- [x] T020 [US2] Implement websocket auth resolution, header injection, secret-ref enforcement, and anonymous-fallback denial in `daemon/internal/mcp/manager.go` and `daemon/internal/mcp/websocket.go`
- [x] T021 [US2] Wire websocket lifecycle start, discovery, runtime tool-call provenance, and transport-failure classification through the existing daemon MCP plane in `daemon/internal/mcp/manager.go`, `daemon/internal/api/server.go`, and `daemon/internal/runtime/runtime.go`
- [x] T022 [US2] Add a deterministic repo-owned websocket MCP helper server for targeted and manual verification in `daemon/cmd/mcp-websocket-helper/main.go`
- [x] T023 [P] [US2] Update MCP request and resource schemas for websocket config and redacted auth summary in `schemas/api/mcp-server-create.request.schema.json`, `schemas/api/mcp-server-update.request.schema.json`, and `schemas/api/mcp-server-resource.schema.json`
- [x] T024 [P] [US2] Update tool-call and lifecycle event schemas for websocket transport identity and failure truth in `schemas/events/mcp-server-registered.event.schema.json`, `schemas/events/mcp-server-updated.event.schema.json`, `schemas/events/mcp-server-started.event.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, and `schemas/events/tool-call-failed.event.schema.json`

**Checkpoint**: A real websocket MCP server can be started, inspected, and
invoked end-to-end through the same daemon-owned MCP manager and runtime
tool-call plane as existing transports

---

## Phase 5: User Story 3 - Preserve Recovery And Audit Truth (Priority: P3)

**Goal**: Make websocket reconnect, retry, cancellation, and restore behavior
bounded, explicit, and reconstructable from daemon-visible state, events, and
history

**Independent Test**: Trigger websocket disconnect and restart scenarios, then
confirm the daemon surfaces reconnect scheduling, retry attempts, restore
behavior, and terminal failure truth without requiring raw logs.

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T025 [P] [US3] Add MCP manager and app regression tests for bounded reconnect scheduling, reconnect success, reconnect exhaustion, restore after daemon restart, and environment-scoped recovery state in `daemon/internal/mcp/manager_test.go` and `daemon/internal/app/mcp_app_test.go`
- [x] T026 [P] [US3] Add API regression tests for reconnect-state inspection, recovery event history, cancelled or failed websocket lifecycle truth, and `<=100 ms` reconnect-preflight timing in `daemon/internal/api/mcp_server_test.go` and `daemon/internal/api/server_test.go`
- [x] T027 [P] [US3] Add contract regression coverage for websocket reconnect events, recovery projection, and redacted history truth in `daemon/internal/contracts/contracts_test.go`, `schemas/events/mcp-server-reconnect-scheduled.event.schema.json`, `schemas/events/mcp-server-reconnect-completed.event.schema.json`, and `schemas/events/mcp-server-reconnect-failed.event.schema.json`

### Implementation for User Story 3

- [x] T028 [US3] Implement bounded daemon-managed websocket reconnect policy, attempt counters, and terminal failure classification in `daemon/internal/mcp/manager.go` and `daemon/internal/mcp/websocket.go`
- [x] T029 [US3] Persist websocket recovery snapshots and restore-safe reconnect state in `daemon/internal/mcp/manager.go`, `daemon/internal/store/store.go`, and `daemon/internal/app/app.go`
- [x] T030 [US3] Publish reconnect-scheduled, reconnect-completed, reconnect-failed, and restore-truth event payloads through existing MCP audit surfaces in `daemon/internal/mcp/manager.go`
- [x] T031 [P] [US3] Update API and event schemas for websocket recovery projection and reconnect history in `schemas/api/mcp-server-resource.schema.json`, `schemas/events/mcp-server-reconnect-scheduled.event.schema.json`, `schemas/events/mcp-server-reconnect-completed.event.schema.json`, and `schemas/events/mcp-server-reconnect-failed.event.schema.json`
- [x] T032 [US3] Update architecture, roadmap, and quickstart docs for websocket reconnect policy, restore semantics, and operator-visible recovery truth in `docs/harness/harness-architecture.md`, `docs/runtime/daemon-roadmaps.md`, and `specs/008-additional-mcp-transports/quickstart.md`

**Checkpoint**: Websocket reconnect and restore behavior remains bounded,
restart-safe, and operator-visible through existing daemon history and events

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, design-artifact alignment, and roadmap-closure
evidence

- [x] T033 [P] Refresh feature design artifacts to match implemented websocket transport capability, auth, and recovery closure in `specs/008-additional-mcp-transports/research.md`, `specs/008-additional-mcp-transports/data-model.md`, and `specs/008-additional-mcp-transports/contracts/additional-mcp-transport-surfaces.md`
- [x] T034 [P] Run `make daemon-contract-test` and record results in `specs/008-additional-mcp-transports/quickstart.md`
- [x] T035 [P] Run targeted daemon verification in `daemon/internal/mcp`, `daemon/internal/api`, `daemon/internal/app`, `daemon/internal/runtime`, `daemon/internal/store`, and `daemon/internal/contracts`, then record results in `specs/008-additional-mcp-transports/quickstart.md`
- [x] T036 [P] Run full daemon regression verification with `go test ./...` in `daemon/` and record results in `specs/008-additional-mcp-transports/quickstart.md`
- [x] T037 [P] Execute the manual `KURA_ENV=test` operator acceptance workflow for `GET /v1/mcp/transports`, `GET /v1/config`, `GET /v1/mcp/servers/{id}`, plus websocket auth-blocked and end-to-end invocation paths, and record whether the inspection completes within `<=5 min` in `specs/008-additional-mcp-transports/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational; establishes capability truth and inspection surfaces
- **User Story 2 (Phase 4)**: Depends on User Story 1 because websocket execution must land on the finalized transport-capability and server-resource contracts
- **User Story 3 (Phase 5)**: Depends on User Story 2 because reconnect and restore semantics build on the completed websocket transport path
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on later stories
- **User Story 2 (P2)**: Depends on User Story 1 because websocket lifecycle and auth truth must project through the finalized transport-capability surfaces
- **User Story 3 (P3)**: Depends on User Story 2 because reconnect and restore semantics require a working websocket transport implementation

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Types and transport primitives before route and docs closure
- Manager behavior before API projection and schema closure
- Story checkpoint must pass before moving to the next dependent story

### Parallel Opportunities

- `T002` and `T003` can run in parallel
- `T006` and `T007` can run in parallel
- `T008`, `T009`, and `T010` can run in parallel
- `T014` and `T015` can run in parallel after capability behavior stabilizes
- `T016`, `T017`, and `T018` can run in parallel
- `T023` and `T024` can run in parallel after websocket route behavior stabilizes
- `T025`, `T026`, and `T027` can run in parallel
- `T031` and `T032` can run in parallel after reconnect behavior stabilizes
- `T033`, `T034`, `T035`, `T036`, and `T037` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 2

```bash
# Launch all tests for User Story 2 together:
Task: "Add API and runtime regression tests for websocket server register/start, tool discovery, runtime tool-call provenance, auth-missing blocked truth, and redacted auth projection in daemon/internal/api/mcp_server_test.go, daemon/internal/api/server_test.go, and daemon/internal/runtime/runtime_test.go"
Task: "Add MCP manager and app regression tests for websocket initialize, session health, tool invocation, restart restore, and preflight timing in daemon/internal/mcp/manager_test.go and daemon/internal/app/mcp_app_test.go"
Task: "Add contract regression coverage for websocket request, resource, tool-call, and lifecycle event surfaces in daemon/internal/contracts/contracts_test.go and schemas/api/, schemas/events/"

# Launch implementation work on different files in parallel:
Task: "Implement websocket MCP transport session open, initialize, list-tools, call-tool, and close behavior in daemon/internal/mcp/websocket.go and daemon/internal/mcp/transport.go"
Task: "Add a deterministic repo-owned websocket MCP helper server for targeted and manual verification in daemon/cmd/mcp-websocket-helper/main.go"
Task: "Update MCP request and resource schemas for websocket config and redacted auth summary in schemas/api/"
```

---

## Implementation Strategy

### First Validation Checkpoint

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **VALIDATE CHECKPOINT ONLY**: Verify transport capability inspection and mismatch truth independently
5. Continue with the remaining user stories; this checkpoint is not roadmap closure and
   must not be treated as shippable completion

### Incremental Delivery

1. Complete Setup + Foundational → websocket transport primitives ready
2. Add User Story 1 → validate capability inspection and prerequisite truth
3. Add User Story 2 → validate websocket lifecycle, auth, and runtime tool-call provenance
4. Add User Story 3 → validate bounded reconnect, restore, and recovery audit truth
5. Claim roadmap closure only after all story phases and final verification complete

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 transport capability and config projection
   - Developer B: User Story 2 websocket transport implementation and helper server after US1 contracts stabilize
   - Developer C: User Story 3 reconnect or restore semantics after US2 transport behavior stabilizes
3. Finish with shared contract and regression verification in Polish

---

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] label maps each story task to a user story for traceability
- Each user story should be independently completable and testable
- Verify required tests fail before implementing
- Avoid vague tasks, hidden compatibility work, same-file parallel conflicts, and transport-specific side paths outside the daemon-owned MCP plane
