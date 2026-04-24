# Tasks: Operator Shell And Onboarding

**Input**: Design documents from `/specs/017-operator-shell-onboarding/`  
**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/operator-shell-surfaces.md](./contracts/operator-shell-surfaces.md), [quickstart.md](./quickstart.md)

**Tests**: Constitution rules apply. This roadmap changes web client behavior, SDK contracts, daemon HTTP API contracts, and operator-visible projections, so targeted Go API tests, contract tests, SDK tests, web tests, and manual `DOPE_ENV=test` verification are required.

**Organization**: Tasks are grouped by user story so each story can be implemented and verified as an independently testable increment.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create operator-shell scaffolding for the daemon API, SDK, schemas, and primary web shell.

- [X] T001 Create operator-shell API scaffolding in `daemon/internal/api/operator.go`, `daemon/internal/api/operator_projection.go`, `daemon/internal/api/operator_test.go`, and register route stubs in `daemon/internal/api/server.go`
- [X] T002 [P] Create operator projection schema placeholders in `schemas/api/operator-onboarding.response.schema.json`, `schemas/api/operator-readiness-item.schema.json`, `schemas/api/operator-first-use-action.schema.json`, `schemas/api/operator-activity-record.schema.json`, `schemas/api/operator-activity-list.response.schema.json`, `schemas/api/operator-diagnostic-finding.schema.json`, and `schemas/api/operator-diagnostic-list.response.schema.json`
- [X] T003 [P] Add operator-shell SDK type and method scaffolding in `sdk/ts/src/index.ts` and `sdk/ts/src/index.test.ts`
- [X] T004 [P] Replace the chat-console scaffold with operator-shell scaffold code in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land shared projection, contract, SDK, and web-shell foundations that block all user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T005 Add operator projection response types and shared shell-detail descriptors in `daemon/internal/api/types.go` and `daemon/internal/api/operator_projection.go`
- [X] T006 [P] Implement shared projection builders over auth, config, integrations, connectors, capabilities, policy, runtime, delivery, and persisted events in `daemon/internal/api/operator_projection.go`
- [X] T007 [P] Register authenticated operator projection routes in `daemon/internal/api/server.go` and implement handler shells in `daemon/internal/api/operator.go`
- [X] T008 [P] Extend contract validation fixtures and schema coverage for operator-shell surfaces in `daemon/internal/contracts/contracts_test.go` and `schemas/api/README.md`
- [X] T009 [P] Implement shared SDK support for operator projections, approval actions, authoritative detail fetches, and event-stream refresh hooks in `sdk/ts/src/index.ts` and `sdk/ts/src/index.test.ts`
- [X] T010 [P] Implement shared web-shell loading, empty/error state handling, shell-detail panel state, and bounded refetch plumbing in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

**Checkpoint**: Operator projection routes, schema validation, shared SDK support, and the base web-shell scaffold are ready; user story work can now proceed.

---

## Phase 3: User Story 1 - Complete First-Run Setup (Priority: P1) 🎯 MVP

**Goal**: A first-time operator can open the primary web shell, understand readiness state, complete the minimum onboarding path, and run one bounded first useful action without using raw daemon routes.

**Independent Test**: From a fresh `DOPE_ENV=test` environment, load the web shell, see the active environment and onboarding state, satisfy the minimum readiness set for the recommended bounded first useful action, run that action, and observe immediate result or status feedback in the shell.

### Tests for User Story 1

- [X] T011 [P] [US1] Add Go API and contract regressions for `GET /v1/operator/onboarding` and onboarding response schemas in `daemon/internal/api/operator_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T012 [P] [US1] Add SDK and web tests for onboarding loading, readiness grouping, optional follow-up setup, and first useful action feedback in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

### Implementation for User Story 1

- [X] T013 [P] [US1] Implement onboarding projection assembly from daemon truth in `daemon/internal/api/operator_projection.go` and `daemon/internal/api/operator.go`
- [X] T014 [P] [US1] Finalize onboarding schemas in `schemas/api/operator-onboarding.response.schema.json`, `schemas/api/operator-readiness-item.schema.json`, and `schemas/api/operator-first-use-action.schema.json`
- [X] T015 [US1] Expose the onboarding route and operator response typing in `daemon/internal/api/server.go` and `daemon/internal/api/types.go`
- [X] T016 [P] [US1] Add SDK methods for onboarding retrieval and bounded first useful action reuse in `sdk/ts/src/index.ts`
- [X] T017 [US1] Implement the environment banner, onboarding summary, readiness list, optional follow-up setup section, and first useful action panel in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

**Checkpoint**: User Story 1 is complete when a first-time operator can complete the minimum onboarding path and run one bounded first useful action entirely inside the primary web shell.

---

## Phase 4: User Story 2 - Inspect Approvals, Background Work, And Outcomes (Priority: P2)

**Goal**: Operators can inspect and act on approvals from the shell, and inspect recent schedules, workflows, deliveries, and related activity without reconstructing raw daemon state.

**Independent Test**: Seed a pending approval plus representative run, workflow, schedule, and delivery activity in `DOPE_ENV=test`, then use the web shell to inspect the approval inbox, approve or reject a pending item, and inspect recent activity through shell-resident authoritative detail panels.

### Tests for User Story 2

- [X] T018 [P] [US2] Add Go API and contract regressions for `GET /v1/operator/activity`, approval list/resolve usage, and linked activity projections in `daemon/internal/api/operator_test.go`, `daemon/internal/api/server_test.go`, and `daemon/internal/contracts/contracts_test.go`
- [X] T019 [P] [US2] Add SDK and web tests for approval inbox actions, recent activity rendering, shell-resident detail inspection, and refresh after approval resolution in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

### Implementation for User Story 2

- [X] T020 [P] [US2] Implement operator activity projection assembly across approvals, runs, workflows, schedules, deliveries, and persisted events in `daemon/internal/api/operator_projection.go` and `daemon/internal/api/operator.go`
- [X] T021 [P] [US2] Finalize operator activity schemas in `schemas/api/operator-activity-record.schema.json` and `schemas/api/operator-activity-list.response.schema.json`
- [X] T022 [US2] Add SDK methods for listing approvals, resolving approvals, loading operator activity, and fetching linked authoritative detail in `sdk/ts/src/index.ts` and `sdk/ts/src/index.test.ts`
- [X] T023 [US2] Implement approval inbox UI with direct approve/reject handling and same-shell decision detail in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`
- [X] T024 [US2] Implement recent activity feed rendering, shell-resident detail panels, and post-action refresh behavior in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

**Checkpoint**: User Story 2 is complete when approvals can be handled inside the shell and recent activity shows authoritative cross-domain outcomes with shell-resident detail inspection.

---

## Phase 5: User Story 3 - Diagnose Why Work Did Not Run Or Deliver (Priority: P3)

**Goal**: Operators can inspect diagnostic findings that distinguish readiness, approval, execution, and delivery failures from one web shell.

**Independent Test**: Seed degraded readiness and blocked or failed work in `DOPE_ENV=test`, then use the web shell to inspect diagnostics that identify the failing plane, severity, recommended action, and detail route without reading raw logs first.

### Tests for User Story 3

- [X] T025 [P] [US3] Add Go API and contract regressions for `GET /v1/operator/diagnostics`, environment-scoped findings, and detail-link projection in `daemon/internal/api/operator_test.go` and `daemon/internal/contracts/contracts_test.go`
- [X] T026 [P] [US3] Add web tests for diagnostics rendering, plane separation, filters, and shell-resident detail affordances in `web/src/app/App.test.tsx`

### Implementation for User Story 3

- [X] T027 [P] [US3] Implement diagnostic projection assembly across readiness, approval, execution, delivery, and computer-use truth in `daemon/internal/api/operator_projection.go` and `daemon/internal/api/operator.go`
- [X] T028 [P] [US3] Finalize diagnostics schemas in `schemas/api/operator-diagnostic-finding.schema.json` and `schemas/api/operator-diagnostic-list.response.schema.json`
- [X] T029 [P] [US3] Add SDK methods for diagnostics retrieval, filter support, and linked authoritative detail fetches in `sdk/ts/src/index.ts` and `sdk/ts/src/index.test.ts`
- [X] T030 [US3] Implement diagnostics panels, severity or plane filtering, and shell-resident detail handling in `web/src/app/App.tsx`, `web/src/app/App.test.tsx`, and `web/src/styles.css`

**Checkpoint**: User Story 3 is complete when the shell can explain why work is blocked, degraded, failed, or undelivered without collapsing those conditions into a generic error.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish docs, schema notes, verification evidence, and rollout safety for roadmap 32.

- [X] T031 [P] Update operator-shell roadmap and API docs in `docs/runtime/daemon-roadmaps.md`, `docs/runtime/daemon-api-and-event-model.md`, `docs/harness/harness-architecture.md`, and `docs/specs/017-operator-shell-and-onboarding.md`
- [X] T032 [P] Update feature walkthrough, verification notes, and rollback guidance in `specs/017-operator-shell-onboarding/quickstart.md` and `specs/017-operator-shell-onboarding/plan.md`
- [X] T033 Run `go mod tidy` in `daemon/` and record module fallout in `specs/017-operator-shell-onboarding/plan.md`
- [X] T034 Run automated verification for daemon, SDK, and web operator-shell paths, including local timing checks against operator projection and refresh targets, and record results in `specs/017-operator-shell-onboarding/quickstart.md` and `specs/017-operator-shell-onboarding/plan.md`
- [X] T035 Run the manual `DOPE_ENV=test` onboarding walkthrough, restart the daemon to confirm resumable onboarding and durable recent activity, and record observed onboarding, approval, activity, diagnostics, and first useful action behavior in `specs/017-operator-shell-onboarding/quickstart.md` and `specs/017-operator-shell-onboarding/plan.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1: Setup**: No dependencies; start immediately.
- **Phase 2: Foundational**: Depends on Phase 1; blocks all user story work.
- **Phase 3: US1**: Depends on Phase 2; delivers the MVP onboarding and bounded first-use flow.
- **Phase 4: US2**: Depends on Phase 2 and reuses the shared shell scaffold to add approval handling and recent activity.
- **Phase 5: US3**: Depends on Phase 2 and reuses the shared shell scaffold to add diagnostics.
- **Phase 6: Polish**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Starts after Foundational; no dependency on other stories.
- **US2 (P2)**: Starts after Foundational and integrates with the shared shell layout, but remains independently testable with seeded approvals and activity.
- **US3 (P3)**: Starts after Foundational and integrates with the shared shell layout, but remains independently testable with seeded degraded and failed conditions.

### Within Each User Story

- Go API and contract tests should be written before or alongside implementation and must
  fail before the story is considered complete.
- Daemon projection logic precedes SDK method finalization that depends on those
  responses.
- SDK support precedes final web-shell integration for each story.
- Story-specific docs and recorded walkthrough notes happen only after corresponding
  behavior is working.

### Parallel Opportunities

- Setup tasks marked `[P]` can run together.
- In Foundational, shared projection builders, contract fixtures, SDK support, and base
  web-shell state work can proceed in parallel once route scaffolding exists.
- For each user story, Go API and contract tests can be written in parallel with SDK and
  web tests.
- Within each story, schema work can run in parallel with daemon projection logic once the
  response shape is agreed.

---

## Parallel Example: User Story 1

```bash
# Tests in parallel
Task: "T011 [US1] Add Go API and contract regressions for GET /v1/operator/onboarding and onboarding response schemas in daemon/internal/api/operator_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T012 [US1] Add SDK and web tests for onboarding loading, readiness grouping, optional follow-up setup, and first useful action feedback in sdk/ts/src/index.test.ts and web/src/app/App.test.tsx"

# Implementation in parallel
Task: "T013 [US1] Implement onboarding projection assembly from daemon truth in daemon/internal/api/operator_projection.go and daemon/internal/api/operator.go"
Task: "T014 [US1] Finalize onboarding schemas in schemas/api/operator-onboarding.response.schema.json, schemas/api/operator-readiness-item.schema.json, and schemas/api/operator-first-use-action.schema.json"
```

## Parallel Example: User Story 2

```bash
# Tests in parallel
Task: "T018 [US2] Add Go API and contract regressions for GET /v1/operator/activity, approval list/resolve usage, and linked activity projections in daemon/internal/api/operator_test.go, daemon/internal/api/server_test.go, and daemon/internal/contracts/contracts_test.go"
Task: "T019 [US2] Add SDK and web tests for approval inbox actions, recent activity rendering, and refresh after approval resolution in sdk/ts/src/index.test.ts and web/src/app/App.test.tsx"

# Implementation in parallel
Task: "T020 [US2] Implement operator activity projection assembly across approvals, runs, workflows, schedules, deliveries, and persisted events in daemon/internal/api/operator_projection.go and daemon/internal/api/operator.go"
Task: "T021 [US2] Finalize operator activity schemas in schemas/api/operator-activity-record.schema.json and schemas/api/operator-activity-list.response.schema.json"
```

## Parallel Example: User Story 3

```bash
# Tests in parallel
Task: "T025 [US3] Add Go API and contract regressions for GET /v1/operator/diagnostics, environment-scoped findings, and detail-link projection in daemon/internal/api/operator_test.go and daemon/internal/contracts/contracts_test.go"
Task: "T026 [US3] Add web tests for diagnostics rendering, plane separation, filters, and shell-resident detail affordances in web/src/app/App.test.tsx"

# Implementation in parallel
Task: "T027 [US3] Implement diagnostic projection assembly across readiness, approval, execution, delivery, and computer-use truth in daemon/internal/api/operator_projection.go and daemon/internal/api/operator.go"
Task: "T028 [US3] Finalize diagnostics schemas in schemas/api/operator-diagnostic-finding.schema.json and schemas/api/operator-diagnostic-list.response.schema.json"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate onboarding, readiness projection, and bounded first useful action in `DOPE_ENV=test`.
5. Treat this as the MVP checkpoint only; roadmap 32 is not closed until US2, US3, and Phase 6 are complete.

### Incremental Delivery

1. Land Setup + Foundational to establish operator projections, schema validation, SDK support, and the shared web-shell scaffold.
2. Deliver US1 for onboarding, readiness projection, and bounded first useful action.
3. Deliver US2 for approval handling and recent activity inspection.
4. Deliver US3 for diagnostics across readiness, approval, execution, and delivery planes.
5. Finish with docs, `go mod tidy`, automated verification, and the recorded manual walkthrough.

### Parallel Team Strategy

1. One engineer lands Setup + Foundational, especially operator projection scaffolding, schema fixtures, SDK support, and shared shell loading.
2. After Foundational is complete:
   - Engineer A takes US1 onboarding and first useful action.
   - Engineer B takes US2 approval inbox and activity feed.
   - Engineer C takes US3 diagnostics.
3. Finish together on documentation, verification, and rollout notes.

## Notes

- `[P]` means the task can run in parallel because it targets different files or depends only on completed foundational work.
- Every user story includes explicit tests because roadmap 32 changes operator-visible behavior and schema-backed control-plane surfaces.
- Operator projections must remain daemon-owned summaries; the web client must not become a second source of truth.
- The primary completion surface for roadmap 32 is the web shell; TUI parity is explicitly out of scope for this task list.
