# Tasks: Evaluation And Replay Harness

**Input**: Design documents from `/specs/018-evaluation-replay-harness/`  
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/evaluation-replay-surfaces.md](./contracts/evaluation-replay-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes daemon persistence, schema-backed HTTP APIs, event contracts, SDK methods, web operator-shell behavior, and restart-visible evaluation history, so targeted Go unit/API/store/contract tests, SDK tests, web tests, contract validation, and manual `KURA_ENV=test` verification are required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create evaluation/replay scaffolding for daemon domain logic, API routes, schemas, SDK types, fixtures, and the web operator shell.

- [X] T001 Create evaluation domain scaffolding in `daemon/internal/evaluation/types.go`, `daemon/internal/evaluation/manager.go`, `daemon/internal/evaluation/fixtures.go`, `daemon/internal/evaluation/comparison.go`, and `daemon/internal/evaluation/manager_test.go`
- [X] T002 [P] Create evaluation API scaffolding in `daemon/internal/api/evaluation.go`, `daemon/internal/api/evaluation_test.go`, and register placeholder route groups in `daemon/internal/api/server.go`
- [X] T003 [P] Create evaluation schema placeholders in `schemas/api/replay-candidate-resource.schema.json`, `schemas/api/replay-candidate-list.response.schema.json`, `schemas/api/create-replay-attempt.request.schema.json`, `schemas/api/replay-attempt-resource.schema.json`, `schemas/api/replay-attempt-list.response.schema.json`, `schemas/api/create-replay-comparison.request.schema.json`, `schemas/api/replay-comparison-resource.schema.json`, `schemas/api/replay-comparison-list.response.schema.json`, `schemas/api/replay-drift-finding.schema.json`, `schemas/api/replay-fixture-resource.schema.json`, and `schemas/api/replay-fixture-list.response.schema.json`
- [X] T004 [P] Create evaluation event schema placeholders in `schemas/events/evaluation-replay-started.event.schema.json`, `schemas/events/evaluation-replay-completed.event.schema.json`, `schemas/events/evaluation-replay-blocked.event.schema.json`, `schemas/events/evaluation-replay-unreplayable.event.schema.json`, `schemas/events/evaluation-replay-failed.event.schema.json`, and `schemas/events/evaluation-comparison-completed.event.schema.json`
- [X] T005 [P] Add evaluation SDK and web-shell scaffolding in `sdk/ts/src/index.ts`, `sdk/ts/src/index.test.ts`, `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land shared durable evaluation records, route wiring, contract fixtures, typed SDK resources, and web-shell entry points that block all user stories.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T006 Define replay candidate, replay attempt, comparison result, drift finding, fixture, source ref, safety scope, and request/response types in `daemon/internal/evaluation/types.go` and `daemon/internal/api/types.go`
- [X] T007 Implement SQLite persistence, migrations, and restart restoration tests for evaluation records in `daemon/internal/store/store.go` and `daemon/internal/store/store_test.go`
- [X] T008 [P] Implement evaluation manager construction and app/server dependency wiring in `daemon/internal/evaluation/manager.go`, `daemon/internal/app/app.go`, and `daemon/internal/api/server.go`
- [X] T009 [P] Add canonical evaluation schema fixtures and validation coverage in `daemon/internal/contracts/contracts_test.go` and `daemon/internal/contracts/evaluation_contracts_test.go`
- [X] T010 [P] Define additive evaluation event payload helpers and publishing hooks in `daemon/internal/events/bus.go`, `daemon/internal/api/evaluation.go`, `schemas/events/evaluation-replay-started.event.schema.json`, `schemas/events/evaluation-replay-completed.event.schema.json`, `schemas/events/evaluation-replay-blocked.event.schema.json`, `schemas/events/evaluation-replay-unreplayable.event.schema.json`, `schemas/events/evaluation-replay-failed.event.schema.json`, and `schemas/events/evaluation-comparison-completed.event.schema.json`
- [X] T011 [P] Add shared SDK resource types and query/input types for replay candidates, attempts, comparisons, drift findings, and fixtures in `sdk/ts/src/index.ts`
- [X] T012 [P] Add base web operator-shell Evaluation/Replay view state, loading, empty, error, and refresh plumbing in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`
- [X] T013 Document foundational evaluation route family and storage expectations in `docs/runtime/daemon-api-and-event-model.md` and `docs/harness/harness-architecture.md`

**Checkpoint**: Durable evaluation records, route scaffolding, schemas, SDK types, and web-shell entry points are ready; user story work can now proceed.

---

## Phase 3: User Story 1 - Replay Representative Agent Work (Priority: P1) MVP

**Goal**: Operators can select a curated candidate or fixture in the web operator shell, inspect readiness, and launch a default non-live replay without raw route use or unintended side effects.

**Independent Test**: Select a curated representative prior run, workflow, or engineer-managed fixture in `KURA_ENV=test`, launch a non-live replay from the web shell, and inspect the attempt status, source linkage, readiness limitations, blocked reasons, and evidence refs after daemon restart.

### Tests for User Story 1

- [X] T014 [P] [US1] Add evaluation manager tests for curated eligibility, readiness statuses, source provenance, default non-live mode, and approval/side-effect blocking in `daemon/internal/evaluation/manager_test.go`
- [X] T015 [P] [US1] Add API and contract tests for `GET /v1/evaluation/replay-candidates`, `GET /v1/evaluation/replay-candidates/{candidateId}`, `POST /v1/evaluation/replay-candidates/{candidateId}/attempts`, and replay attempt schemas in `daemon/internal/api/evaluation_test.go` and `daemon/internal/contracts/evaluation_contracts_test.go`
- [X] T016 [P] [US1] Add store restart tests for replay candidate and replay attempt persistence in `daemon/internal/store/store_test.go`
- [X] T017 [P] [US1] Add SDK and web tests for candidate loading, readiness display, non-live replay launch, blocked approval/side-effect display, and attempt refresh in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

### Implementation for User Story 1

- [X] T018 [P] [US1] Implement replay candidate creation, curation metadata, readiness evaluation, source refs, and limitations in `daemon/internal/evaluation/manager.go` and `daemon/internal/evaluation/fixtures.go`
- [X] T019 [US1] Implement default non-live replay launch, approval-gated blocked/evidence-only handling, side-effect blocking, safety scope persistence, and replay attempt execution status mapping separate from comparison terminal status in `daemon/internal/evaluation/manager.go`
- [X] T020 [US1] Implement replay candidate and attempt persistence methods in `daemon/internal/store/store.go` and `daemon/internal/store/store_test.go`
- [X] T021 [US1] Implement replay candidate and attempt API handlers in `daemon/internal/api/evaluation.go`, route registration in `daemon/internal/api/server.go`, and response builders in `daemon/internal/api/types.go`
- [X] T022 [P] [US1] Finalize replay candidate, create replay attempt, replay attempt, and replay attempt list schemas in `schemas/api/replay-candidate-resource.schema.json`, `schemas/api/replay-candidate-list.response.schema.json`, `schemas/api/create-replay-attempt.request.schema.json`, `schemas/api/replay-attempt-resource.schema.json`, and `schemas/api/replay-attempt-list.response.schema.json`
- [X] T023 [P] [US1] Publish replay started, replay blocked, replay unreplayable, replay failed, and replay completed events from replay launch and terminal paths in `daemon/internal/api/evaluation.go`, `daemon/internal/events/bus.go`, `schemas/events/evaluation-replay-started.event.schema.json`, `schemas/events/evaluation-replay-blocked.event.schema.json`, `schemas/events/evaluation-replay-unreplayable.event.schema.json`, `schemas/events/evaluation-replay-failed.event.schema.json`, and `schemas/events/evaluation-replay-completed.event.schema.json`
- [X] T024 [P] [US1] Add SDK methods for listing replay candidates, inspecting candidates, creating replay attempts, listing attempts, and inspecting attempts in `sdk/ts/src/index.ts` and `sdk/ts/src/index.test.ts`
- [X] T025 [US1] Implement web operator-shell candidate list, readiness details, non-live replay launch action, attempt status panel, and source-link rendering in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

**Checkpoint**: User Story 1 is complete when replay launch works from the web shell for curated candidates, defaults to non-live behavior, and remains inspectable after restart.

---

## Phase 4: User Story 2 - Compare Before And After Outcomes (Priority: P2)

**Goal**: Operators can generate and inspect plane-level before/after comparisons that classify terminal status plus runtime, policy, integration, delivery, and evidence-summary differences where available.

**Independent Test**: Run a replay attempt against a supported deterministic fixture, create a comparison from the web shell, and confirm the comparison explains material differences by plane without manual log diffing.

### Tests for User Story 2

- [X] T026 [P] [US2] Add evaluation comparison tests for matched, runtime drift, policy drift, integration drift, delivery drift, evidence-summary drift, unknown drift, and unavailable evidence limitations in `daemon/internal/evaluation/manager_test.go` and `daemon/internal/evaluation/comparison_test.go`
- [X] T027 [P] [US2] Add API and contract tests for `POST /v1/evaluation/replay-attempts/{attemptId}/compare`, `GET /v1/evaluation/comparisons`, `GET /v1/evaluation/comparisons/{comparisonId}`, and comparison schemas in `daemon/internal/api/evaluation_test.go` and `daemon/internal/contracts/evaluation_contracts_test.go`
- [X] T028 [P] [US2] Add store restart tests for comparison result and drift finding persistence in `daemon/internal/store/store_test.go`
- [X] T029 [P] [US2] Add SDK and web tests for comparison generation, plane summary rendering, drift finding grouping, limitation display, and authoritative resource links in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

### Implementation for User Story 2

- [X] T030 [US2] Implement plane-level comparison generation over terminal status, runtime evidence, policy evidence, integration evidence, delivery evidence, and evidence-summary limitations in `daemon/internal/evaluation/comparison.go` and `daemon/internal/evaluation/manager.go`
- [X] T031 [US2] Implement comparison result and drift finding persistence methods in `daemon/internal/store/store.go` and `daemon/internal/store/store_test.go`
- [X] T032 [US2] Implement comparison API handlers and response builders in `daemon/internal/api/evaluation.go`, `daemon/internal/api/server.go`, and `daemon/internal/api/types.go`
- [X] T033 [P] [US2] Finalize create comparison, comparison resource, comparison list, and drift finding schemas in `schemas/api/create-replay-comparison.request.schema.json`, `schemas/api/replay-comparison-resource.schema.json`, `schemas/api/replay-comparison-list.response.schema.json`, and `schemas/api/replay-drift-finding.schema.json`
- [X] T034 [P] [US2] Publish comparison completed events in `daemon/internal/api/evaluation.go`, `daemon/internal/events/bus.go`, and `schemas/events/evaluation-comparison-completed.event.schema.json`
- [X] T035 [P] [US2] Add SDK methods for creating comparisons, listing comparisons, and inspecting comparisons in `sdk/ts/src/index.ts` and `sdk/ts/src/index.test.ts`
- [X] T036 [US2] Implement web operator-shell comparison action, terminal status display, plane summaries, drift finding groups, confidence/limitations display, and linked detail affordances in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

**Checkpoint**: User Story 2 is complete when the web shell can create and inspect plane-level comparisons that classify material drift without raw log diffing.

---

## Phase 5: User Story 3 - Maintain Regression Fixtures For Real Agent Flows (Priority: P3)

**Goal**: Engineers can maintain repo-managed replay fixtures for schedule, integration, and computer-use flows, and operators can consume those fixtures through replay/comparison surfaces.

**Independent Test**: Add or refresh schedule, integration, and computer-use fixtures, run automated replay/comparison coverage against them, and verify the web shell exposes fixture provenance, assumptions, limitations, and candidate linkage without in-product fixture editing.

### Tests for User Story 3

- [X] T037 [P] [US3] Add fixture manifest validation tests for schedule, integration, and computer-use fixtures in `daemon/internal/evaluation/fixtures_test.go` and `daemon/internal/evaluation/testdata/fixtures/`
- [X] T038 [P] [US3] Add API and contract tests for `GET /v1/evaluation/fixtures` and fixture schemas in `daemon/internal/api/evaluation_test.go` and `daemon/internal/contracts/evaluation_contracts_test.go`
- [X] T039 [P] [US3] Add automated regression coverage that replays and compares the schedule, integration, and computer-use fixtures in `daemon/internal/evaluation/manager_test.go` and `daemon/internal/app/app_test.go`
- [X] T040 [P] [US3] Add SDK and web tests for fixture listing, fixture provenance display, candidate linkage, and absence of in-product fixture editing controls in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

### Implementation for User Story 3

- [X] T041 [P] [US3] Add repo-managed schedule fixture manifest and captured evidence in `daemon/internal/evaluation/testdata/fixtures/schedule-basic/manifest.json` and `daemon/internal/evaluation/testdata/fixtures/schedule-basic/evidence.json`
- [X] T042 [P] [US3] Add repo-managed integration fixture manifest and captured evidence in `daemon/internal/evaluation/testdata/fixtures/integration-basic/manifest.json` and `daemon/internal/evaluation/testdata/fixtures/integration-basic/evidence.json`
- [X] T043 [P] [US3] Add repo-managed computer-use fixture manifest and captured evidence in `daemon/internal/evaluation/testdata/fixtures/computer-use-basic/manifest.json` and `daemon/internal/evaluation/testdata/fixtures/computer-use-basic/evidence.json`
- [X] T044 [US3] Implement fixture manifest loading, validation, provenance extraction, assumptions, limitations, and candidate generation in `daemon/internal/evaluation/fixtures.go` and `daemon/internal/evaluation/manager.go`
- [X] T045 [US3] Implement fixture metadata persistence and candidate linkage in `daemon/internal/store/store.go` and `daemon/internal/store/store_test.go`
- [X] T046 [US3] Implement fixture API handler and response builders in `daemon/internal/api/evaluation.go`, `daemon/internal/api/server.go`, and `daemon/internal/api/types.go`
- [X] T047 [P] [US3] Finalize fixture resource and fixture list schemas in `schemas/api/replay-fixture-resource.schema.json` and `schemas/api/replay-fixture-list.response.schema.json`
- [X] T048 [P] [US3] Add SDK method for listing replay fixtures in `sdk/ts/src/index.ts` and `sdk/ts/src/index.test.ts`
- [X] T049 [US3] Implement web operator-shell fixture list, provenance, assumptions, limitations, candidate linkage, and no-editing UX in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

**Checkpoint**: User Story 3 is complete when all three required fixture classes are repo-managed, replayable/comparable in automated paths, and consumable from the web shell.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish docs, contract notes, operational guidance, verification evidence, and rollout safety for roadmap 33.

- [X] T050 [P] Update evaluation and replay roadmap/API docs in `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-api-and-event-model.md`, `docs/harness/harness-architecture.md`, and `docs/specs/018-evaluation-and-replay-harness.md`
- [X] T051 [P] Update feature walkthrough, fixture guidance, rollback notes, and manual acceptance evidence placeholders in `specs/018-evaluation-replay-harness/quickstart.md` and `specs/018-evaluation-replay-harness/plan.md`
- [X] T052 [P] Update schema index and contract documentation for evaluation resources and events in `schemas/README.md` and `schemas/api/README.md`, and create or update `schemas/events/README.md`
- [X] T053 Run `cd daemon && go mod tidy` and record module fallout in `specs/018-evaluation-replay-harness/plan.md`
- [X] T054 Run automated verification for daemon evaluation, API, store, contract, SDK, and web paths; record route timing targets, supported-fixture classification rate, and results in `specs/018-evaluation-replay-harness/quickstart.md` and `specs/018-evaluation-replay-harness/plan.md`
- [X] T055 Run the manual `KURA_ENV=test` before/after replay walkthrough, restart daemon to confirm durable evaluation history, record the 10-minute replay completion target and 5-minute drift-determination target, and record observed candidate, attempt, comparison, fixture, and web-shell behavior in `specs/018-evaluation-replay-harness/quickstart.md` and `specs/018-evaluation-replay-harness/plan.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all user story work.
- **Phase 3: US1**: Depends on Phase 2; delivers MVP replay candidate inspection and non-live replay launch.
- **Phase 4: US2**: Depends on Phase 2 and uses replay attempts from US1 for meaningful comparisons.
- **Phase 5: US3**: Depends on Phase 2 and can proceed in parallel with US1/US2 fixture work, but full automated replay/comparison fixture validation depends on US1 and US2 behavior.
- **Phase 6: Polish**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on US2 or US3 for core replay launch.
- **US2 (P2)**: Starts after Foundational; comparison is independently testable with seeded attempts, but production completion should validate against US1 replay attempts.
- **US3 (P3)**: Starts after Foundational; fixture metadata and API work is independent, while fixture replay/comparison regressions depend on US1 and US2 implementation.

### Within Each User Story

- Tests must be written before or alongside implementation and must fail before the story is considered complete.
- Domain/store behavior precedes API handlers.
- API contracts precede SDK method finalization.
- SDK support precedes final web-shell integration.
- Story-specific docs and recorded walkthrough notes happen after behavior is working.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together after T001 or route/package naming is agreed.
- Foundational schema, event, SDK, and web scaffolding can proceed in parallel after T006 defines core names.
- In each user story, Go/domain tests, API/contract tests, store tests, and SDK/web tests can be written in parallel.
- Fixture manifests for schedule, integration, and computer-use can be authored in parallel.
- Documentation and schema index updates can proceed in parallel during Phase 6.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T014 [US1] Add evaluation manager tests for curated eligibility, readiness statuses, source provenance, default non-live mode, and approval/side-effect blocking in daemon/internal/evaluation/manager_test.go"
Task: "T015 [US1] Add API and contract tests for replay candidate and replay attempt surfaces in daemon/internal/api/evaluation_test.go and daemon/internal/contracts/evaluation_contracts_test.go"
Task: "T016 [US1] Add store restart tests for replay candidate and replay attempt persistence in daemon/internal/store/store_test.go"
Task: "T017 [US1] Add SDK and web tests for candidate loading and non-live replay launch in sdk/ts/src/index.test.ts and web/src/app/App.test.tsx"

# Implementation in parallel
Task: "T018 [US1] Implement replay candidate creation, curation metadata, readiness evaluation, source refs, and limitations in daemon/internal/evaluation/manager.go and daemon/internal/evaluation/fixtures.go"
Task: "T022 [US1] Finalize replay candidate and replay attempt schemas in schemas/api/"
Task: "T024 [US1] Add SDK methods for replay candidates and attempts in sdk/ts/src/index.ts"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T026 [US2] Add evaluation comparison tests in daemon/internal/evaluation/manager_test.go and daemon/internal/evaluation/comparison_test.go"
Task: "T027 [US2] Add API and contract tests for comparison surfaces in daemon/internal/api/evaluation_test.go and daemon/internal/contracts/evaluation_contracts_test.go"
Task: "T029 [US2] Add SDK and web tests for comparison generation and drift finding rendering in sdk/ts/src/index.test.ts and web/src/app/App.test.tsx"

# Implementation in parallel
Task: "T030 [US2] Implement plane-level comparison generation in daemon/internal/evaluation/comparison.go and daemon/internal/evaluation/manager.go"
Task: "T033 [US2] Finalize comparison and drift finding schemas in schemas/api/"
Task: "T035 [US2] Add SDK methods for comparisons in sdk/ts/src/index.ts"
```

## Parallel Example: User Story 3

```bash
# Fixture manifests in parallel
Task: "T041 [US3] Add repo-managed schedule fixture manifest and captured evidence in daemon/internal/evaluation/testdata/fixtures/schedule-basic/"
Task: "T042 [US3] Add repo-managed integration fixture manifest and captured evidence in daemon/internal/evaluation/testdata/fixtures/integration-basic/"
Task: "T043 [US3] Add repo-managed computer-use fixture manifest and captured evidence in daemon/internal/evaluation/testdata/fixtures/computer-use-basic/"

# Tests in parallel
Task: "T037 [US3] Add fixture manifest validation tests in daemon/internal/evaluation/fixtures_test.go and daemon/internal/evaluation/testdata/fixtures/"
Task: "T038 [US3] Add API and contract tests for GET /v1/evaluation/fixtures in daemon/internal/api/evaluation_test.go and daemon/internal/contracts/evaluation_contracts_test.go"
Task: "T040 [US3] Add SDK and web tests for fixture listing and no-editing UX in sdk/ts/src/index.test.ts and web/src/app/App.test.tsx"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate curated candidate listing, non-live replay launch, blocked/evidence-only side-effect behavior, and restart persistence in `KURA_ENV=test`.
5. Treat this as the MVP checkpoint only; roadmap 33 is not closed until comparison, fixtures, docs, and verification are complete.

### Incremental Delivery

1. Land Setup + Foundational to establish evaluation records, schemas, SDK types, and web-shell entry points.
2. Deliver US1 for candidate readiness and non-live replay launch.
3. Deliver US2 for plane-level comparison and drift findings.
4. Deliver US3 for schedule, integration, and computer-use fixtures.
5. Finish with docs, `go mod tidy`, automated verification, and the recorded manual walkthrough.

### Parallel Team Strategy

1. One engineer lands Setup + Foundational, especially evaluation types, store persistence, route scaffolding, schema fixtures, SDK support, and shared shell loading.
2. After Foundational is complete:
   - Engineer A takes US1 replay candidate and non-live attempt flow.
   - Engineer B takes US2 comparison and drift findings.
   - Engineer C takes US3 fixture manifests and fixture API/shell consumption.
3. Finish together on contract validation, documentation, quickstart verification, and rollout notes.

## Notes

- `[P]` means the task can run in parallel because it targets different files or depends only on completed foundational work.
- Every user story includes explicit tests because roadmap 33 changes persistence, operator-visible behavior, and schema-backed control-plane surfaces.
- Evaluation records must remain daemon-owned and restart-safe; the web client must not infer comparison truth from raw events.
- The primary completion surface for roadmap 33 is the web operator shell plus daemon-owned evaluation API; raw routes and docs alone are not sufficient.
- Run `go mod tidy` from `daemon/` before considering implementation complete.
