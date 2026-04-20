---

description: "Task list for Complete MCP Runtime And Catalog"

---

# Tasks: Complete MCP Runtime And Catalog

**Input**: Design documents from `/specs/006-mcp-runtime-and-catalog/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This roadmap changes MCP runtime execution,
catalog install flows, API/schema/event contracts, remote transport behavior,
and operator-visible audit surfaces, so targeted daemon tests and contract
verification are required.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependency)
- **[Story]**: Which user story this belongs to (`US1`, `US2`, `US3`, `US4`)
- Include exact file paths in descriptions

## Path Conventions

- Daemon code lives under `daemon/internal/...`
- API and event schemas live under `schemas/api/` and `schemas/events/`
- Repo-supported operator helpers live under `scripts/`
- Feature artifacts live under `specs/006-mcp-runtime-and-catalog/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable MCP catalog, invocation, and contract fixtures for
the completion slice

- [x] T001 Create bundled-catalog fixture builders and prerequisite-matrix helpers in `daemon/internal/mcp/manager_test.go`
- [x] T002 [P] Add reusable MCP tool-call provenance and failure assertion helpers in `daemon/internal/api/server_test.go` and `daemon/internal/runtime/runtime_test.go`
- [x] T003 [P] Add reusable contract assertion helpers for catalog, install, and remote-transport payloads in `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish shared types, persistence, and contract primitives
required by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Define additive MCP catalog, install provenance, remote transport, and invocation types in `daemon/internal/mcp/types.go` and `daemon/internal/api/types.go`
- [x] T005 Extend runtime and SQLite persistence for MCP invocation provenance, catalog origin, and install method fields in `daemon/internal/runtime/runtime.go`, `daemon/internal/store/store.go`, and `daemon/internal/store/mcp_store_test.go`
- [x] T006 [P] Add base schema scaffolding for additive MCP server and tool-call projection fields in `schemas/api/mcp-server-resource.schema.json`, `schemas/api/mcp-server-list.response.schema.json`, `schemas/api/tool-call-resource.schema.json`, and `schemas/api/tool-call-list.response.schema.json`
- [x] T007 [P] Add base event and contract scaffolding for MCP install audit and invocation provenance in `daemon/internal/contracts/contracts_test.go`, `schemas/events/mcp-server-updated.event.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, and `schemas/events/tool-call-failed.event.schema.json`

**Checkpoint**: Shared MCP completion primitives are ready for story work

---

## Phase 3: User Story 1 - MCP Tools Are Callable Through The Daemon (Priority: P1)

**Goal**: Make daemon-managed MCP tools execute through the existing runtime
tool-call plane with explicit approval, provenance, and failure truth

**Independent Test**: Register a representative MCP server, allowlist a
discovered tool for a runtime surface, invoke it through the daemon, and verify
the result, approval behavior, server identity, and failure classification stay
operator-visible.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T008 [P] [US1] Add stdio MCP tool-invocation regression tests for allowlisted, approval-gated, unhealthy-server, and redacted-result outcomes in `daemon/internal/api/server_test.go` and `daemon/internal/runtime/runtime_test.go`
- [x] T009 [P] [US1] Add MCP manager invocation regression tests for failure-class mapping, session-state linkage, and invocation preflight timing in `daemon/internal/mcp/manager_test.go`
- [x] T010 [P] [US1] Add contract regression coverage for MCP tool-call provenance, failure classes, and redacted operator-visible payloads in `daemon/internal/contracts/contracts_test.go`, `schemas/api/tool-call-resource.schema.json`, `schemas/api/tool-call-list.response.schema.json`, `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, and `schemas/events/tool-call-failed.event.schema.json`

### Implementation for User Story 1

- [x] T011 [US1] Implement MCP tool execution through the existing runtime tool-call plane in `daemon/internal/api/server.go` and `daemon/internal/runtime/runtime.go`
- [x] T012 [US1] Enforce MCP tool exposure, approval, and session-health checks at invocation time in `daemon/internal/mcp/manager.go` and `daemon/internal/api/server.go`
- [x] T013 [P] [US1] Project MCP server, session, transport, authorization provenance, and redacted invocation outputs through API resource builders in `daemon/internal/api/types.go`, `daemon/internal/api/server.go`, `schemas/api/tool-call-resource.schema.json`, and `schemas/api/tool-call-list.response.schema.json`
- [x] T014 [P] [US1] Persist MCP tool-call provenance, terminal failure classes, redacted invocation outputs, and redacted history projections in `daemon/internal/store/store.go` and `daemon/internal/store/store_test.go`
- [x] T015 [P] [US1] Update tool-call and policy event payloads for MCP invocation audit with redacted operator-visible output fields in `schemas/events/tool-call-requested.event.schema.json`, `schemas/events/tool-call-completed.event.schema.json`, `schemas/events/tool-call-failed.event.schema.json`, and `schemas/events/policy-decision-recorded.event.schema.json`

**Checkpoint**: MCP tools are runnable through the daemon’s existing
tool-call plane with explicit approval and provenance truth

---

## Phase 4: User Story 2 - Operators Can Install Curated MCP Servers Into The Test Environment (Priority: P1)

**Goal**: Provide a truthful bundled MCP catalog and converged install flows so
operators can bring up starter MCP servers in `DOPE_ENV=test` without
hand-authoring raw JSON

**Independent Test**: Inspect the catalog, install representative starter
definitions into `DOPE_ENV=test`, and verify each installed server appears as a
first-class MCP resource with truthful prerequisite, credential, and
availability state.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T016 [P] [US2] Add bundled-catalog list, detail, and install regression tests in `daemon/internal/api/mcp_server_test.go` and `daemon/internal/api/server_test.go`
- [x] T017 [P] [US2] Add MCP manager catalog availability, install origin, and environment-separation regression tests in `daemon/internal/mcp/manager_test.go` and `daemon/internal/store/mcp_store_test.go`
- [x] T018 [P] [US2] Add contract regression coverage for catalog list/detail/install schemas and install audit events in `daemon/internal/contracts/contracts_test.go`, `schemas/api/mcp-catalog-entry.schema.json`, `schemas/api/mcp-catalog-list.response.schema.json`, `schemas/api/mcp-catalog-detail.response.schema.json`, `schemas/api/mcp-catalog-install-request.schema.json`, `schemas/api/mcp-catalog-install-result.schema.json`, `schemas/api/mcp-server-resource.schema.json`, `schemas/api/mcp-server-list.response.schema.json`, `schemas/events/mcp-catalog-install-requested.event.schema.json`, `schemas/events/mcp-catalog-install-completed.event.schema.json`, and `schemas/events/mcp-catalog-install-failed.event.schema.json`
- [x] T019 [P] [US2] Add timing regression coverage for catalog list, install, and MCP tool-invocation preflight staying within `<=100 ms` daemon-side overhead in `daemon/internal/mcp/manager_test.go` and `daemon/internal/api/server_test.go`

### Implementation for User Story 2

- [x] T020 [US2] Implement bundled starter catalog definitions and truthful availability evaluation for `filesystem`, `Context7`, `GitHub`, `Postgres`, and `Slack` in `daemon/internal/mcp/catalog.go` and `daemon/internal/mcp/manager.go`
- [x] T021 [US2] Implement daemon API catalog list, detail, and install routes plus install result projection in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [x] T022 [US2] Implement repo-supported catalog installation workflow with `DOPE_ENV=test` default in `scripts/install-mcp-catalog-entry.sh`
- [x] T023 [US2] Preserve installed-server origin, install method, operator-modified protection, and test-vs-production separation in `daemon/internal/mcp/manager.go`, `daemon/internal/store/store.go`, and `daemon/internal/api/server.go`
- [x] T024 [P] [US2] Add additive API schemas for catalog list/detail/install surfaces and installed-server origin fields in `schemas/api/mcp-catalog-entry.schema.json`, `schemas/api/mcp-catalog-list.response.schema.json`, `schemas/api/mcp-catalog-detail.response.schema.json`, `schemas/api/mcp-catalog-install-request.schema.json`, `schemas/api/mcp-catalog-install-result.schema.json`, `schemas/api/mcp-server-resource.schema.json`, and `schemas/api/mcp-server-list.response.schema.json`
- [x] T025 [P] [US2] Add install audit event schemas for requested, completed, and failed catalog actions in `schemas/events/mcp-catalog-install-requested.event.schema.json`, `schemas/events/mcp-catalog-install-completed.event.schema.json`, and `schemas/events/mcp-catalog-install-failed.event.schema.json`
- [x] T026 [P] [US2] Update MCP server inspection and event projections for catalog availability and secret-redacted prerequisite truth in `schemas/events/mcp-server-updated.event.schema.json`, `schemas/api/mcp-tool-resource.schema.json`, and `schemas/api/event-list.response.schema.json`

**Checkpoint**: Operators can inspect and install starter MCP entries in the
test environment through converged daemon and repo workflows

---

## Phase 5: User Story 3 - MCP Transport Support Covers Local Stdio And One Remote Mode (Priority: P2)

**Goal**: Add `streamable-http` as the first remote MCP transport while keeping
transport choice, health, and failure semantics explicit beside existing stdio
support

**Independent Test**: Register one stdio MCP server and one remote MCP server,
then confirm discovery, lifecycle truth, tool invocation, and failure
classification remain consistent across both transport families.

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T027 [P] [US3] Add `streamable-http` transport lifecycle and invocation regression tests in `daemon/internal/mcp/manager_test.go` and `daemon/internal/api/server_test.go`
- [x] T028 [P] [US3] Add restart, unreachable-endpoint, and unsupported-transport regression tests in `daemon/internal/app/mcp_app_test.go` and `daemon/internal/mcp/manager_test.go`
- [x] T029 [P] [US3] Add contract regression coverage for remote transport declarations, server resources, and health/failure events in `daemon/internal/contracts/contracts_test.go`, `schemas/api/mcp-server-create.request.schema.json`, `schemas/api/mcp-server-update.request.schema.json`, `schemas/api/mcp-declaration.schema.json`, `schemas/api/mcp-server-resource.schema.json`, `schemas/events/mcp-server-health-changed.event.schema.json`, and `schemas/events/mcp-server-failed.event.schema.json`

### Implementation for User Story 3

- [x] T030 [US3] Implement `streamable-http` connection setup, handshake, and invoke path in `daemon/internal/mcp/transport.go` and `daemon/internal/mcp/streamable_http.go`
- [x] T031 [US3] Wire remote transport lifecycle, restart recovery, and explicit transport failure classification through `daemon/internal/mcp/manager.go` and `daemon/internal/app/app.go`
- [x] T032 [P] [US3] Project remote transport identity, endpoint summaries, and unsupported truth through `daemon/internal/api/types.go`, `daemon/internal/api/server.go`, `schemas/api/mcp-server-create.request.schema.json`, `schemas/api/mcp-server-update.request.schema.json`, `schemas/api/mcp-declaration.schema.json`, `schemas/api/mcp-server-resource.schema.json`, and `schemas/api/mcp-tool-resource.schema.json`
- [x] T033 [P] [US3] Mark remote starter entries such as `Context7` truthfully as ready, unavailable, or unsupported during install and inspection in `daemon/internal/mcp/catalog.go` and `schemas/events/mcp-server-health-changed.event.schema.json`

**Checkpoint**: Local stdio and remote `streamable-http` MCP transports share
one operator-visible daemon control plane

---

## Phase 6: User Story 4 - MCP Starter Catalog Remains Truthful And Auditable (Priority: P3)

**Goal**: Align bundled catalog docs, daemon-visible MCP state, and audit
history so operators and future engineers can trust install and invocation
behavior without hidden defaults

**Independent Test**: Follow the documented MCP install-and-verify workflow for
bundled servers, then confirm docs, API responses, and event history all
describe the same catalog entries, prerequisites, install results, and
invocation behavior.

### Tests for User Story 4

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T034 [P] [US4] Add operator-facing regression coverage for catalog, install, and invocation audit alignment in `daemon/internal/api/server_test.go` and `daemon/internal/contracts/contracts_test.go`

### Implementation for User Story 4

- [x] T035 [US4] Update MCP architecture and trust-model docs for catalog install, transport coverage, and audit boundaries in `docs/harness/harness-architecture.md` and `docs/runtime/operator-trust-model.md`
- [x] T036 [US4] Update roadmap and task-tracking docs for the completed MCP slice in `docs/runtime/daemon-roadmaps.md` and `docs/runtime/daemon-tasks.md`
- [x] T037 [US4] Record starter catalog matrix, install workflows, unavailable semantics, and the under-5-minute operator walkthrough in `specs/006-mcp-runtime-and-catalog/quickstart.md`
- [x] T038 [P] [US4] Refresh feature design artifacts to match implemented catalog and transport closure in `specs/006-mcp-runtime-and-catalog/research.md`, `specs/006-mcp-runtime-and-catalog/data-model.md`, and `specs/006-mcp-runtime-and-catalog/contracts/mcp-runtime-and-catalog-surfaces.md`

**Checkpoint**: Bundled catalog, daemon state, docs, and audit history stay
aligned and operator-trustworthy

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and roadmap-closure evidence

- [x] T039 [P] Run `make daemon-contract-test` and record results in `specs/006-mcp-runtime-and-catalog/quickstart.md`
- [x] T040 [P] Run targeted daemon verification in `daemon/internal/mcp`, `daemon/internal/api`, `daemon/internal/runtime`, `daemon/internal/app`, `daemon/internal/store`, and `daemon/internal/contracts`, then record results in `specs/006-mcp-runtime-and-catalog/quickstart.md`
- [x] T041 [P] Run full daemon regression verification with `go test ./...` in `daemon/` and record results in `specs/006-mcp-runtime-and-catalog/quickstart.md`
- [x] T042 [P] Execute the manual `DOPE_ENV=test` catalog install, invocation, unavailable-path, and operator-timing walkthrough and record evidence in `specs/006-mcp-runtime-and-catalog/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational; establishes MCP invocation on the canonical runtime plane
- **User Story 2 (Phase 4)**: Depends on Foundational; can proceed in parallel with User Story 1 once shared primitives exist
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 because remote transport must plug into the final invocation plane and bundled catalog model
- **User Story 4 (Phase 6)**: Depends on User Stories 1, 2, and 3 because docs and audit truth must match the implemented runtime and catalog behavior
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on later stories
- **User Story 2 (P1)**: Can start after Foundational; no dependency on User Story 1 for its independent install-and-inspect criteria
- **User Story 3 (P2)**: Depends on User Stories 1 and 2 because remote transport must work inside the final invocation and catalog surfaces
- **User Story 4 (P3)**: Depends on User Stories 1, 2, and 3 because operator guidance must reflect the implemented catalog and transport truth

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Persistence and contract scaffolding before route and docs closure
- Runtime execution behavior before operator docs and audit recording
- Story checkpoint must pass before moving to the next dependent story

### Parallel Opportunities

- `T002` and `T003` can run in parallel
- `T006` and `T007` can run in parallel
- `T008`, `T009`, and `T010` can run in parallel
- `T013`, `T014`, and `T015` can run in parallel after invocation behavior stabilizes
- `T016`, `T017`, `T018`, and `T019` can run in parallel
- `T024`, `T025`, and `T026` can run in parallel after catalog route behavior stabilizes
- `T027`, `T028`, and `T029` can run in parallel
- `T032` and `T033` can run in parallel after remote transport behavior stabilizes
- `T034` and `T038` can run in parallel
- `T039`, `T040`, `T041`, and `T042` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 2

```bash
# Launch all tests for User Story 2 together:
Task: "Add bundled-catalog list, detail, and install regression tests in daemon/internal/api/mcp_server_test.go and daemon/internal/api/server_test.go"
Task: "Add MCP manager catalog availability, install origin, and environment-separation regression tests in daemon/internal/mcp/manager_test.go and daemon/internal/store/mcp_store_test.go"
Task: "Add contract regression coverage for catalog entry/install schemas and install audit events in daemon/internal/contracts/contracts_test.go and schemas/api/, schemas/events/"
Task: "Add timing regression coverage for catalog list and install preflight staying within <=100 ms daemon-side overhead in daemon/internal/mcp/manager_test.go and daemon/internal/api/server_test.go"

# Launch implementation work on different files in parallel:
Task: "Implement bundled starter catalog definitions and truthful availability evaluation for filesystem, Context7, GitHub, Postgres, and Slack in daemon/internal/mcp/catalog.go and daemon/internal/mcp/manager.go"
Task: "Implement daemon API catalog list, detail, and install routes plus install result projection in daemon/internal/api/server.go and daemon/internal/api/types.go"
Task: "Implement repo-supported catalog installation workflow with DOPE_ENV=test default in scripts/install-mcp-catalog-entry.sh"
```

---

## Implementation Strategy

### First Validation Checkpoint

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **VALIDATE CHECKPOINT ONLY**: Verify MCP tools execute through the runtime
   tool-call plane with explicit approval and provenance
5. Continue with the remaining user stories; this checkpoint is not roadmap
   closure and must not be treated as shippable completion

### Incremental Delivery

1. Complete Setup + Foundational → shared MCP completion primitives ready
2. Add User Story 1 → validate end-to-end daemon invocation truth
3. Add User Story 2 → validate catalog install, unavailable semantics, and API/script convergence
4. Add User Story 3 → validate remote transport lifecycle and remote starter truth
5. Add User Story 4 → validate docs, audit history, and operator workflow alignment
6. Claim roadmap closure only after all story phases and final verification complete

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 MCP invocation and provenance work
   - Developer B: User Story 2 catalog and install workflow work
   - Developer C: User Story 3 remote transport work after US1 and US2 contracts stabilize
3. Finish with shared documentation, audit, and verification closure in User Story 4 and Polish

---

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] label maps each story task to a user story for traceability
- Each user story should be independently completable and testable
- Verify required tests fail before implementing
- Commit after each task or logical group
- Stop at story checkpoints to validate independently
- Avoid: vague tasks, wildcard paths, second MCP invoke planes, hidden install side paths, or docs that drift from implemented runtime truth
