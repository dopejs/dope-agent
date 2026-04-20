---

description: "Task list for MCP Catalog Management"

---

# Tasks: MCP Catalog Management

**Input**: Design documents from `/specs/007-mcp-catalog-management/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Constitution rules apply. This roadmap changes MCP server lifecycle,
catalog provenance, schema-backed API and event surfaces, persistence, and
operator-visible history, so targeted daemon tests and contract verification are
required.

**Organization**: Tasks are grouped by user story to enable independent
implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no incomplete-task dependency)
- **[Story]**: Which user story this belongs to (`US1`, `US2`, `US3`)
- Include exact file paths in descriptions

## Path Conventions

- Daemon code lives under `daemon/internal/...`
- API and event schemas live under `schemas/api/` and `schemas/events/`
- Feature artifacts live under `specs/007-mcp-catalog-management/`
- Roadmap and operator docs live under `docs/runtime/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare reusable catalog-maintenance fixtures, drift assertions, and
contract helpers for Roadmap 22

- [x] T001 Create reusable catalog-managed MCP server fixtures, install-snapshot builders, and busy-state helpers in `daemon/internal/mcp/manager_test.go`
- [x] T002 [P] Add reusable API assertion helpers for lifecycle action results, drift summaries, and revalidation issues in `daemon/internal/api/mcp_server_test.go`
- [x] T003 [P] Add reusable contract assertion helpers for catalog lifecycle and revalidation payloads in `daemon/internal/contracts/contracts_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish shared types, persistence hooks, and contract scaffolding
required by every user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Define additive catalog-management types for lifecycle actions, install snapshots, drift summaries, and revalidation results in `daemon/internal/mcp/types.go` and `daemon/internal/api/types.go`
- [x] T005 Extend MCP manager and SQLite persistence primitives for catalog-management metadata inside the existing server document model in `daemon/internal/mcp/manager.go`, `daemon/internal/store/store.go`, and `daemon/internal/store/mcp_store_test.go`
- [x] T006 [P] Add base schema scaffolding for server-resource catalog-management projection and new lifecycle/revalidation result resources in `schemas/api/mcp-server-resource.schema.json`, `schemas/api/mcp-server-list.response.schema.json`, `schemas/api/mcp-catalog-lifecycle-result.schema.json`, and `schemas/api/mcp-catalog-revalidation-result.schema.json`
- [x] T007 [P] Add base event and contract scaffolding for catalog lifecycle and revalidation audit surfaces in `daemon/internal/contracts/contracts_test.go`, `schemas/events/mcp-catalog-lifecycle-requested.event.schema.json`, `schemas/events/mcp-catalog-lifecycle-completed.event.schema.json`, `schemas/events/mcp-catalog-lifecycle-failed.event.schema.json`, and `schemas/events/mcp-catalog-revalidation-completed.event.schema.json`

**Checkpoint**: Shared catalog-management primitives are ready for story work

---

## Phase 3: User Story 1 - Maintain Installed Catalog Entries (Priority: P1) 🎯 MVP

**Goal**: Let operators safely uninstall, refresh, and reinstall installed
catalog-managed MCP servers through daemon-owned workflows

**Independent Test**: Install a bundled catalog entry, then uninstall or
reinstall it through daemon-owned routes and confirm the resulting MCP
resource, provenance, and audit history remain explicit and correct.

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T008 [P] [US1] Add API regression tests for catalog-managed uninstall, refresh, reinstall, `busy` or `conflict` failures, cross-environment target isolation, missing-entry refresh/reinstall failure truth, and redacted lifecycle result payloads in `daemon/internal/api/mcp_server_test.go`
- [x] T009 [P] [US1] Add MCP manager and store regression tests for in-place refresh, same-`serverId` reinstall, uninstall removal, active-session conflict handling, environment-scoped maintenance history, and `<=100 ms` lifecycle preflight timing in `daemon/internal/mcp/manager_test.go`, `daemon/internal/store/mcp_store_test.go`, and `daemon/internal/store/store_test.go`
- [x] T010 [P] [US1] Add contract regression coverage for redacted lifecycle action responses, environment-scoped audit events, and missing-entry lifecycle failure classes in `daemon/internal/contracts/contracts_test.go`, `schemas/api/mcp-catalog-lifecycle-result.schema.json`, `schemas/events/mcp-catalog-lifecycle-requested.event.schema.json`, `schemas/events/mcp-catalog-lifecycle-completed.event.schema.json`, and `schemas/events/mcp-catalog-lifecycle-failed.event.schema.json`

### Implementation for User Story 1

- [x] T011 [US1] Implement catalog-managed `uninstall`, `refresh`, and `reinstall` manager actions with idle-only gating, environment-scope enforcement, explicit `missing_entry` classification, and fail-closed conflict behavior in `daemon/internal/mcp/manager.go`
- [x] T012 [US1] Add installed-server action routes and result projection for `/refresh`, `/reinstall`, and `/uninstall` in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [x] T013 [US1] Persist install snapshots, last lifecycle action truth, and same-identity reinstall semantics in `daemon/internal/mcp/manager.go` and `daemon/internal/store/store.go`
- [x] T014 [P] [US1] Update API schemas for lifecycle action results and additive installed-server resource fields in `schemas/api/mcp-catalog-lifecycle-result.schema.json`, `schemas/api/mcp-server-resource.schema.json`, and `schemas/api/mcp-server-list.response.schema.json`
- [x] T015 [P] [US1] Add lifecycle audit event schemas and payload projection for requested, completed, and failed maintenance actions in `schemas/events/mcp-catalog-lifecycle-requested.event.schema.json`, `schemas/events/mcp-catalog-lifecycle-completed.event.schema.json`, and `schemas/events/mcp-catalog-lifecycle-failed.event.schema.json`
- [x] T016 [US1] Publish operator-visible audit/history records for maintenance actions with redacted payloads, environment-scoped lookup truth, and preserved uninstall evidence after resource removal in `daemon/internal/mcp/manager.go`, `daemon/internal/api/types.go`, and `daemon/internal/store/store.go`

**Checkpoint**: Operators can safely uninstall, refresh, and reinstall
catalog-managed MCP servers through daemon-owned routes without hand-editing
registry state

---

## Phase 4: User Story 2 - Inspect Source, Version, And Drift (Priority: P2)

**Goal**: Make installed catalog-managed MCP resources expose source identity,
revision truth, and explicit drift state through daemon inspection surfaces

**Independent Test**: Install a catalog-managed MCP server, inspect it through
daemon routes, and verify the resource exposes source identity, install method,
revision or version metadata, and explicit drift state without reading raw
stored JSON.

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T017 [P] [US2] Add API regression tests for catalog provenance, installed versus current revision, drift state, missing-entry inspection, redacted source/version projection, and `<=100 ms` inspection preflight in `daemon/internal/api/mcp_server_test.go`
- [x] T018 [P] [US2] Add MCP manager regression tests for revision fingerprinting, install-snapshot replay, local-modification detection, missing-entry drift classification, and redacted drift summaries in `daemon/internal/mcp/manager_test.go`
- [x] T019 [P] [US2] Add contract regression coverage for additive redacted server-resource provenance and drift projection in `daemon/internal/contracts/contracts_test.go`, `schemas/api/mcp-server-resource.schema.json`, `schemas/api/mcp-server-list.response.schema.json`, `schemas/events/mcp-server-registered.event.schema.json`, and `schemas/events/mcp-server-updated.event.schema.json`

### Implementation for User Story 2

- [x] T020 [US2] Implement canonical catalog revision fingerprinting, persisted install snapshots, and drift classification in `daemon/internal/mcp/catalog.go` and `daemon/internal/mcp/manager.go`
- [x] T021 [US2] Project source identity, install method, installed or current revision, redacted drift status, and drift reason through installed-server resource builders in `daemon/internal/mcp/types.go`, `daemon/internal/api/types.go`, and `daemon/internal/api/server.go`
- [x] T022 [US2] Persist installed revision, current revision, install timestamps, and redacted drift summaries inside the MCP server document in `daemon/internal/mcp/manager.go` and `daemon/internal/store/store.go`
- [x] T023 [P] [US2] Update server resource schemas for additive `catalogManagement` inspection fields in `schemas/api/mcp-server-resource.schema.json` and `schemas/api/mcp-server-list.response.schema.json`
- [x] T024 [P] [US2] Update MCP server event schemas to carry source, revision, and drift truth in `schemas/events/mcp-server-registered.event.schema.json`, `schemas/events/mcp-server-updated.event.schema.json`, and `schemas/events/mcp-server-health-changed.event.schema.json`
- [x] T025 [US2] Update roadmap and feature quickstart guidance for source, version, and drift inspection semantics in `docs/runtime/daemon-roadmaps.md` and `specs/007-mcp-catalog-management/quickstart.md`

**Checkpoint**: Operators can explain where an installed MCP server came from,
which catalog revision it reflects, and whether it is drifted or locally
modified

---

## Phase 5: User Story 3 - Revalidate Installed Prerequisites (Priority: P3)

**Goal**: Let operators explicitly revalidate installed catalog-managed MCP
servers and see clear prerequisite-loss, drift, and runtime-health outcomes
before the next manual start attempt

**Independent Test**: Install a catalog-managed MCP server, remove or
invalidate one of its prerequisites, then trigger revalidation and confirm the
daemon surfaces explicit prerequisite-loss or blocked state without requiring a
start attempt.

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T026 [P] [US3] Add API regression tests for explicit `/revalidate` results, ordered issue lists, operator-trigger-only semantics, environment-scoped revalidation targeting, and redacted revalidation payloads in `daemon/internal/api/mcp_server_test.go`
- [x] T027 [P] [US3] Add MCP manager regression tests for prerequisite loss, multiple simultaneous issues, runtime-health separation, healthy revalidation no-op behavior, and `<=100 ms` revalidation preflight timing in `daemon/internal/mcp/manager_test.go` and `daemon/internal/app/mcp_app_test.go`
- [x] T028 [P] [US3] Add contract regression coverage for redacted revalidation result resources, environment-scoped audit events, and issue classification payloads in `daemon/internal/contracts/contracts_test.go`, `schemas/api/mcp-catalog-revalidation-result.schema.json`, and `schemas/events/mcp-catalog-revalidation-completed.event.schema.json`

### Implementation for User Story 3

- [x] T029 [US3] Implement explicit prerequisite revalidation, primary classification selection, environment-scope enforcement, and redacted `issues[]` aggregation in `daemon/internal/mcp/manager.go` and `daemon/internal/mcp/catalog.go`
- [x] T030 [US3] Add `/v1/mcp/servers/{serverId}/revalidate` route and API result projection in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [x] T031 [US3] Persist last revalidation snapshot, derived availability updates, redacted issue summaries, and environment-scoped history projection in `daemon/internal/mcp/manager.go` and `daemon/internal/store/store.go`
- [x] T032 [P] [US3] Add revalidation result schema and completed-event schema in `schemas/api/mcp-catalog-revalidation-result.schema.json` and `schemas/events/mcp-catalog-revalidation-completed.event.schema.json`
- [x] T033 [US3] Update operator docs for explicit revalidation, issue classification, and no-background-check semantics in `docs/runtime/daemon-roadmaps.md` and `specs/007-mcp-catalog-management/quickstart.md`

**Checkpoint**: Operators can explicitly revalidate installed catalog entries
and see prerequisite-loss, drift, and runtime-health truth before a start
attempt fails

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, design-artifact alignment, and roadmap-closure evidence

- [x] T034 [P] Refresh feature design artifacts to match implemented lifecycle, provenance, and revalidation closure in `specs/007-mcp-catalog-management/research.md`, `specs/007-mcp-catalog-management/data-model.md`, and `specs/007-mcp-catalog-management/contracts/mcp-catalog-management-surfaces.md`
- [x] T035 [P] Run `make daemon-contract-test` and record results in `specs/007-mcp-catalog-management/quickstart.md`
- [x] T036 [P] Run targeted daemon verification in `daemon/internal/mcp`, `daemon/internal/api`, `daemon/internal/app`, `daemon/internal/store`, and `daemon/internal/contracts`, then record results in `specs/007-mcp-catalog-management/quickstart.md`
- [x] T037 [P] Run full daemon regression verification with `go test ./...` in `daemon/` and record results in `specs/007-mcp-catalog-management/quickstart.md`
- [x] T038 [P] Execute the manual `DOPE_ENV=test` install-to-remove or install-to-refresh workflow and record operator-facing evidence in `specs/007-mcp-catalog-management/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion; blocks all story work
- **User Story 1 (Phase 3)**: Depends on Foundational; establishes the daemon-owned maintenance actions and audit truth
- **User Story 2 (Phase 4)**: Depends on User Story 1 because revision and drift truth build on persisted maintenance metadata and install snapshots
- **User Story 3 (Phase 5)**: Depends on User Stories 1 and 2 because revalidation must project final provenance and drift semantics
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational; no dependency on later stories
- **User Story 2 (P2)**: Depends on User Story 1 because refresh/reinstall persistence and install snapshots define the provenance and drift model
- **User Story 3 (P3)**: Depends on User Stories 1 and 2 because revalidation must distinguish prerequisite loss from finalized drift and runtime state

### Within Each User Story

- Write the listed tests first and ensure they fail before implementation
- Types and persistence before route and docs closure
- Manager semantics before API projection and contract closure
- Story checkpoint must pass before moving to the next dependent story

### Parallel Opportunities

- `T002` and `T003` can run in parallel
- `T006` and `T007` can run in parallel
- `T008`, `T009`, and `T010` can run in parallel
- `T014` and `T015` can run in parallel after lifecycle behavior stabilizes
- `T017`, `T018`, and `T019` can run in parallel
- `T023` and `T024` can run in parallel after provenance and drift behavior stabilizes
- `T026`, `T027`, and `T028` can run in parallel
- `T031` and `T032` can run in parallel after revalidation behavior stabilizes
- `T034`, `T035`, `T036`, `T037`, and `T038` can run in parallel after implementation stabilizes

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Add API regression tests for catalog-managed uninstall, refresh, reinstall, and busy/conflict failures in daemon/internal/api/mcp_server_test.go"
Task: "Add MCP manager and store regression tests for in-place refresh, same-serverId reinstall, uninstall removal, and active-session conflict handling in daemon/internal/mcp/manager_test.go and daemon/internal/store/mcp_store_test.go"
Task: "Add contract regression coverage for lifecycle action responses and audit events in daemon/internal/contracts/contracts_test.go and schemas/api/, schemas/events/"

# Launch implementation work on different files in parallel:
Task: "Persist install snapshots, last lifecycle action truth, and same-identity reinstall semantics in daemon/internal/mcp/manager.go and daemon/internal/store/store.go"
Task: "Add lifecycle audit event schemas and payload projection for requested, completed, and failed maintenance actions in schemas/events/"
```

---

## Implementation Strategy

### First Validation Checkpoint

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. **VALIDATE CHECKPOINT ONLY**: Verify uninstall, refresh, reinstall, and busy/conflict truth independently
5. Continue with the remaining user stories; this checkpoint is not roadmap closure and
   must not be treated as shippable completion

### Incremental Delivery

1. Complete Setup + Foundational → catalog-management primitives ready
2. Add User Story 1 → validate maintenance actions and audit truth
3. Add User Story 2 → validate provenance, revision, and drift inspection
4. Add User Story 3 → validate explicit revalidation and issue classification
5. Claim roadmap closure only after all story phases and final verification complete

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 lifecycle actions and audit behavior
   - Developer B: User Story 2 provenance, revision, and drift projection after US1 persistence stabilizes
   - Developer C: User Story 3 revalidation and docs closure after US1/US2 semantics stabilize
3. Finish with shared contract and regression verification in Polish

---

## Notes

- [P] tasks = different files, no incomplete-task dependency
- [Story] label maps each story task to a user story for traceability
- Each user story should be independently completable and testable
- Verify required tests fail before implementing
- Commit after each task or logical group
- Stop at story checkpoints to validate independently
- Avoid: vague tasks, wildcard paths, inactive-resource side models, silent overwrite of operator-modified resources, or background revalidation hidden behind explicit operator workflows
