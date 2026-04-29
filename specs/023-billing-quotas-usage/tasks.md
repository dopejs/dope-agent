---
description: "Task list for Roadmap 38 - Billing, Quotas, And Usage Accounting"
---

# Tasks: Billing, Quotas, And Usage Accounting

**Input**: Design documents from `specs/023-billing-quotas-usage/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the project constitution and feature spec. Every behavior change
below includes targeted daemon, contract, restart, concurrency, audit, SDK, or smoke
verification before the implementation is considered complete.

**Organization**: Tasks are grouped by user story so each story can be implemented and
verified as an independently testable increment. US1, US2, and US3 are P1 and together
form the hosted quota enforcement MVP. US4 and US5 are P2 and close administration plus
planning completeness.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable with other marked tasks in the same phase because it touches
  different files and has no dependency on incomplete tasks
- **[Story]**: maps to user stories from `spec.md`
- Every task includes concrete repository paths

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm active context, preserve Roadmap 38 contracts, and add shared
fixtures used by multiple story phases.

- [X] T001 Verify active feature context and record any mismatch in `specs/023-billing-quotas-usage/quickstart.md`: current branch must be `023-billing-quotas-usage`, `.specify/feature.json` must point to `specs/023-billing-quotas-usage`, and `AGENTS.md` must point to `specs/023-billing-quotas-usage/plan.md`.
- [X] T002 [P] Create shared two-tenant billing fixture helpers with finite, unlimited, and development plan seeds in `daemon/internal/billing/test_fixtures_test.go`.
- [X] T003 [P] Create shared billing API tenant/principal fixture helpers for owner, admin, operator, and viewer contexts in `daemon/internal/api/hosted_billing_test.go`.
- [X] T004 [P] Create shared fake operation identity and quota category fixtures for run, workflow, tool-call, integration, artifact, and evaluation tests in `daemon/internal/billing/operation_fixtures_test.go`.
- [X] T005 [P] Create shared billing contract fixture loader helpers in `daemon/internal/contracts/billing_contracts_test.go`.
- [X] T006 [P] Add Roadmap 38 fixture notes and fake tenant naming conventions to `specs/023-billing-quotas-usage/quickstart.md`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land shared billing types, persistence, default plans, operation identity,
stable denial, audit, and schema foundations that all user stories depend on.

**CRITICAL**: No user story phase should begin until this phase is complete.

### Foundational Tests

- [X] T007 [P] Add quota catalog completeness tests for every required category in `daemon/internal/billing/catalog_test.go`.
- [X] T008 [P] Add billing storage migration tests for plans, quota definitions, overrides, periods, counters, reservations, usage events, denials, manual adjustments, and retention policy projection in `daemon/internal/store/billing_schema_test.go`.
- [X] T009 [P] Add operation identity stability tests for retry and restart inputs in `daemon/internal/billing/operation_key_test.go`.
- [X] T010 [P] Add stable quota denial payload tests for exhausted quota and quota-state-unavailable cases in `daemon/internal/billing/denial_test.go`.
- [X] T011 [P] Add billing audit event construction tests for plan, override, reservation, commit, refund, denial, adjustment, and recovery decisions in `daemon/internal/audit/billing_audit_test.go`.
- [X] T012 [P] Add contract tests for plan, quota, usage, denial, reservation, commit, refund, adjustment, and billing audit schemas in `daemon/internal/contracts/billing_contracts_test.go`.

### Foundational Implementation

- [X] T013 Create tenant plan, quota definition, quota override, quota period, usage counter, reservation, usage event, denial, manual adjustment, and retention policy types in `daemon/internal/billing/types.go`.
- [X] T014 Implement the initial quota catalog with stable categories and denial reason codes in `daemon/internal/billing/catalog.go`.
- [X] T015 Implement operation identity helpers for run, workflow, tool-call, live-validation, integration, artifact, and evaluation operations in `daemon/internal/billing/operation_key.go`.
- [X] T016 Implement stable quota denial errors and response projection helpers in `daemon/internal/billing/denial.go`.
- [X] T017 Implement effective quota projection and local development/unlimited plan semantics in `daemon/internal/billing/projection.go`.
- [X] T018 Implement reservation, commit, release, refund, denial, and manual adjustment service methods in `daemon/internal/billing/manager.go`.
- [X] T019 Add SQLite migrations and indexes for billing tables in `daemon/internal/store/store.go`.
- [X] T020 Implement billing store methods for plan, quota, period, counter, reservation, usage event, denial, and adjustment records in `daemon/internal/store/billing.go`.
- [X] T021 Add default development/unlimited plan bootstrap and hosted active-plan migration helpers in `daemon/internal/store/billing_defaults.go`.
- [X] T022 Add billing audit event construction helpers in `daemon/internal/audit/billing.go`.
- [X] T023 Register billing event names and event payload projections in `daemon/internal/events/billing.go`.
- [X] T024 Add API schemas for billing plan, quota projection, usage summary, denial, reservation, commit, refund, manual adjustment, and admin requests in `schemas/api/billing-plan.response.schema.json`, `schemas/api/billing-usage.response.schema.json`, `schemas/api/billing-quota-resource.schema.json`, `schemas/api/billing-denial-resource.schema.json`, `schemas/api/billing-manual-adjustment.request.schema.json`, and `schemas/api/billing-reservation-resolution.request.schema.json`.
- [X] T025 Add billing event schemas in `schemas/events/billing-usage-reserved.event.schema.json`, `schemas/events/billing-usage-committed.event.schema.json`, `schemas/events/billing-usage-refunded.event.schema.json`, `schemas/events/billing-quota-denied.event.schema.json`, `schemas/events/billing-manual-adjustment-created.event.schema.json`, and `schemas/events/billing-reservation-recovery-decided.event.schema.json`.
- [X] T026 Wire `daemon/internal/billing.Manager` construction into daemon startup dependencies in `daemon/internal/app/app.go` and `daemon/internal/api/server.go`.

**Checkpoint**: Foundation ready. Billing state can be represented, persisted, projected, audited, and exposed through stable schema contracts.

---

## Phase 3: User Story 1 - Tenant Owner Inspects Plan And Usage (Priority: P1)

**Goal**: Tenant owners can inspect active plan, effective quotas, usage, reservations,
remaining allowance, UTC period boundaries, carryover, denials, adjustments, and explicit
unlimited/development plan state without seeing another tenant.

**Independent Test**: Create a hosted tenant with a non-unlimited plan and seeded usage,
plus another same-shaped tenant. Confirm tenant A owner sees only tenant A plan, quota,
usage, reservations, period state, carryover, denials, and adjustments; confirm local
development/unlimited plan projection is explicit.

### Tests for User Story 1

- [X] T027 [P] [US1] Add plan and usage inspection API tests for seeded finite, unlimited, and development tenants in `daemon/internal/api/hosted_billing_test.go`.
- [X] T028 [P] [US1] Add tenant isolation tests for plan, quota, usage, reservation, denial, and adjustment inspection in `daemon/internal/api/hosted_billing_test.go`.
- [X] T029 [P] [US1] Add UTC period reset and carryover projection tests in `daemon/internal/billing/projection_test.go`.
- [X] T030 [P] [US1] Add effective quota projection store tests for committed, reserved, adjusted, and carryover amounts in `daemon/internal/store/billing_test.go`.
- [X] T031 [P] [US1] Add SDK tests for plan, usage, quota, and denial inspection types in `sdk/ts/src/index.test.ts`.
- [X] T032 [P] [US1] Add contract tests for billing inspection response schemas in `daemon/internal/contracts/billing_contracts_test.go`.

### Implementation for User Story 1

- [X] T033 [US1] Implement tenant-scoped plan, quota, usage, and denial read projections in `daemon/internal/billing/projection.go`.
- [X] T034 [US1] Implement `GET /v1/billing/plan`, `GET /v1/billing/usage`, `GET /v1/billing/quotas`, and `GET /v1/billing/denials` handlers in `daemon/internal/api/hosted_billing.go`.
- [X] T035 [US1] Register billing inspection routes with protected tenant context in `daemon/internal/api/server.go`.
- [X] T036 [US1] Update tenant audit read projection to include billing and usage audit records without cross-tenant leakage in `daemon/internal/api/tenants.go`.
- [X] T037 [US1] Add TypeScript SDK methods and types for billing plan, usage, quotas, and denials in `sdk/ts/src/index.ts`.
- [X] T038 [US1] Audit existing billing/tenant inspection surfaces in `web/src/app/App.tsx` and `tui/src/cli.ts`; update the existing surface to expose billing inspection when present, or record a no-op rationale in `specs/023-billing-quotas-usage/quickstart.md` when no such surface exists.
- [X] T039 [US1] Document tenant owner plan and usage inspection behavior in `docs/runtime/hosted-billing-quotas.md`.
- [X] T040 [US1] Run targeted US1 verification commands and record results in `specs/023-billing-quotas-usage/quickstart.md`.

**Checkpoint**: User Story 1 is independently functional and testable through `go test ./internal/billing ./internal/store ./internal/api ./internal/contracts` plus `pnpm test:clients`.

---

## Phase 4: User Story 2 - Work Is Denied Before Costly Or Side-Effecting Consumption (Priority: P1)

**Goal**: Guarded run, workflow, live validation, integration, tool-call, artifact, and
evaluation entry points reserve quota before expensive or side-effecting work begins and
return stable quota denials before consumption when over limit or quota state is
unavailable for hosted tenants.

**Independent Test**: Configure exhausted quotas for each guarded category, attempt every
guarded entry point, and confirm each denial happens before resource consumption, backend
operation, live side effect, artifact write, or durable launch.

### Tests for User Story 2

- [X] T041 [P] [US2] Add run launch quota denial tests proving `POST /v1/runs` denies before `runtime.CreateRun` in `daemon/internal/api/billing_enforcement_test.go`.
- [X] T042 [P] [US2] Add workflow launch and workflow start quota denial tests in `daemon/internal/api/billing_enforcement_test.go`.
- [X] T043 [P] [US2] Add runtime tool-call quota denial tests proving tool calls are not created or invoked before reservation succeeds in `daemon/internal/api/billing_enforcement_test.go`.
- [X] T044 [P] [US2] Add hosted quota-state-unavailable fail-closed tests and local unlimited allow tests in `daemon/internal/api/billing_enforcement_test.go`.
- [X] T045 [P] [US2] Add Roadmap 38 live-validation preflight gate contract tests in `daemon/internal/api/billing_live_validation_test.go`, proving allowed, denied, fail-closed, retry, and no-Roadmap-40-executor behavior.
- [X] T046 [P] [US2] Add calendar, mail, and integration operation quota gate tests in `daemon/internal/api/billing_integration_operations_test.go`.
- [X] T047 [P] [US2] Add artifact write estimate reservation tests in `daemon/internal/artifacts/billing_test.go`, including estimate denial, actual-smaller refund, actual-larger over-limit commit, and future denial after over-limit commit.
- [X] T048 [P] [US2] Add replay/evaluation attempt quota gate tests in `daemon/internal/evaluation/billing_test.go`.
- [X] T049 [P] [US2] Add stable quota-denial schema contract tests for guarded entry points in `daemon/internal/contracts/billing_contracts_test.go`.

### Implementation for User Story 2

- [X] T050 [US2] Implement shared route-level reserve/commit/refund helpers in `daemon/internal/api/billing_enforcement.go`.
- [X] T051 [US2] Wire run launch quota reservation and commit/refund into `handleRuns` and `persistRun` in `daemon/internal/api/server.go`.
- [X] T052 [US2] Wire workflow launch and workflow start reservation lifecycle into `handleRunWorkflows` and `handleRunWorkflowStart` in `daemon/internal/api/server.go`.
- [X] T053 [US2] Wire runtime tool-call reservation lifecycle into `handleRunStepToolCalls`, `handleRunStepToolCallComplete`, and `handleRunStepToolCallFail` in `daemon/internal/api/server.go`.
- [X] T054 [US2] Implement the Roadmap 38 live-validation quota preflight gate adapter in `daemon/internal/api/billing_enforcement.go`, wire any concrete live-validation entry point that already exists in `daemon/internal/api/server.go`, and otherwise register the not-yet-mounted gate contract without creating the Roadmap 40 executor.
- [X] T055 [US2] Wire integration operation reservation lifecycle into calendar handlers in `daemon/internal/api/calendar.go`.
- [X] T056 [US2] Wire integration operation reservation lifecycle into mail handlers in `daemon/internal/api/mail_execution.go` and `daemon/internal/api/mail_projection.go`.
- [X] T057 [US2] Wire artifact byte estimate reservation, actual-byte commit, actual-smaller refund, and actual-larger over-limit commit with future-work denial into `daemon/internal/artifacts/service.go`.
- [X] T058 [US2] Wire replay/evaluation attempt reservation lifecycle into `daemon/internal/evaluation/manager.go` and `daemon/internal/evaluation/runtime_recorder.go`.
- [X] T059 [US2] Update run, workflow, tool-call, integration, artifact, and replay API schemas to include stable quota-denial references where applicable in `schemas/api/error-response.schema.json`.
- [X] T060 [US2] Document fail-closed hosted denial and local unlimited behavior in `docs/runtime/hosted-billing-quotas.md`.
- [X] T061 [US2] Run targeted US2 verification commands and record results in `specs/023-billing-quotas-usage/quickstart.md`.

**Checkpoint**: User Story 2 is independently functional and testable through `go test ./internal/api ./internal/artifacts ./internal/evaluation ./internal/contracts`.

---

## Phase 5: User Story 3 - Usage Accounting Survives Retry, Failure, Restart, And Concurrency (Priority: P1)

**Goal**: Reservations, commits, refunds, releases, denials, manual adjustments, and
ambiguous restart recovery are idempotent by operation identity and concurrency-safe under
simultaneous launches.

**Independent Test**: Exercise retry, failure-before-consumption, cancellation, daemon
restart, ambiguous recovery, and concurrent last-unit scenarios for the same operation
identity and confirm exactly one correct accounting outcome.

### Tests for User Story 3

- [X] T062 [P] [US3] Add reservation idempotency tests for duplicate reserve, commit, refund, release, and denial calls in `daemon/internal/billing/manager_test.go`.
- [X] T063 [P] [US3] Add multi-category atomic reservation tests proving partial reservations roll back on any denied category in `daemon/internal/billing/manager_test.go`.
- [X] T064 [P] [US3] Add concurrent last-unit reservation tests using parallel goroutines in `daemon/internal/billing/concurrency_test.go`.
- [X] T065 [P] [US3] Add restart recovery tests for committed, released, refunded, and operator-action-needed pending reservations in `daemon/internal/billing/recovery_test.go`.
- [X] T066 [P] [US3] Add storage-level operation key uniqueness and transaction rollback tests in `daemon/internal/store/billing_test.go`.
- [X] T067 [P] [US3] Add failure-before-consumption and cancellation refund tests for guarded API helpers in `daemon/internal/api/billing_enforcement_test.go`.
- [X] T068 [P] [US3] Add ambiguous restart duplicate-work denial tests in `daemon/internal/api/billing_enforcement_test.go`.
- [X] T069 [P] [US3] Add billing recovery audit event tests in `daemon/internal/audit/billing_audit_test.go`.

### Implementation for User Story 3

- [X] T070 [US3] Implement transactional reservation locking and compare remaining quota inside `daemon/internal/store/billing.go`.
- [X] T071 [US3] Implement idempotent lifecycle replay handling in `daemon/internal/billing/manager.go`.
- [X] T072 [US3] Implement multi-category atomic reservation and rollback behavior in `daemon/internal/billing/manager.go`.
- [X] T073 [US3] Implement pending reservation recovery scanner and recovery decision writer in `daemon/internal/billing/recovery.go`.
- [X] T074 [US3] Wire restart recovery into daemon startup after store initialization in `daemon/internal/app/app.go`.
- [X] T075 [US3] Implement operator-action-needed duplicate-work denial checks in `daemon/internal/billing/manager.go`.
- [X] T076 [US3] Emit recovery decision audit records and billing events from `daemon/internal/audit/billing.go` and `daemon/internal/events/billing.go`.
- [X] T077 [US3] Update reservation and recovery response schemas in `schemas/api/billing-reservation-resource.schema.json` and `schemas/events/billing-reservation-recovery-decided.event.schema.json`.
- [X] T078 [US3] Document retry, restart, and operator-action-needed recovery behavior in `docs/runtime/hosted-billing-quotas.md`.
- [X] T079 [US3] Run targeted US3 verification commands and record results in `specs/023-billing-quotas-usage/quickstart.md`.

**Checkpoint**: User Story 3 is independently functional and testable through `go test ./internal/billing ./internal/store ./internal/api ./internal/audit`.

---

## Phase 6: User Story 4 - Admin Adjusts Plans And Quotas With Audit Evidence (Priority: P2)

**Goal**: Authorized administrators can assign plans, change quota overrides, make manual
adjustments, and resolve operator-action-needed reservations with required reasons and
structured audit evidence. Unauthorized users cannot mutate billing state.

**Independent Test**: Change a tenant plan, lower a quota below current usage, apply a
manual adjustment, resolve an operator-action-needed reservation, and verify projection,
denial, permission, and audit behavior.

### Tests for User Story 4

- [X] T080 [P] [US4] Add admin plan assignment API tests with audit evidence in `daemon/internal/api/hosted_billing_admin_test.go`.
- [X] T081 [P] [US4] Add quota override API tests for lowered quota below current or reserved usage in `daemon/internal/api/hosted_billing_admin_test.go`.
- [X] T082 [P] [US4] Add manual adjustment validation tests for required reason, non-negative effective usage, and unit consistency in `daemon/internal/billing/adjustment_test.go`.
- [X] T083 [P] [US4] Add operator-action-needed reservation resolution API tests in `daemon/internal/api/hosted_billing_admin_test.go`.
- [X] T084 [P] [US4] Add unauthorized viewer/operator denial tests for plan assignment, quota override, manual adjustment, and reservation resolution in `daemon/internal/api/hosted_billing_admin_test.go`.
- [X] T085 [P] [US4] Add billing and usage audit retention default tests in `daemon/internal/audit/billing_audit_test.go`.
- [X] T086 [P] [US4] Add SDK tests for admin plan assignment, quota override, manual adjustment, and reservation resolution types in `sdk/ts/src/index.test.ts`.

### Implementation for User Story 4

- [X] T087 [US4] Implement plan assignment, quota override, manual adjustment, and reservation resolution service methods in `daemon/internal/billing/admin.go`.
- [X] T088 [US4] Implement admin billing handlers in `daemon/internal/api/hosted_billing.go`.
- [X] T089 [US4] Register `POST /v1/admin/billing/tenants/{tenantId}/plan`, `/quota-overrides`, `/manual-adjustments`, and `/reservations/{reservationId}/resolve` routes in `daemon/internal/api/server.go`.
- [X] T090 [US4] Add canonical `PermissionBillingManage` / `billing.manage` administration permission, owner/admin role mapping, operator/viewer exclusion tests, and billing administration permission checks to `daemon/internal/identity/types.go`, `daemon/internal/identity/permissions.go`, and `daemon/internal/identity/permissions_test.go`.
- [X] T091 [US4] Implement lowered-quota immediate enforcement projection and denial behavior in `daemon/internal/billing/projection.go`.
- [X] T092 [US4] Implement billing and usage audit retention default projection in `daemon/internal/audit/billing.go`.
- [X] T093 [US4] Update admin request and response schemas in `schemas/api/billing-plan-assignment.request.schema.json`, `schemas/api/billing-quota-override.request.schema.json`, `schemas/api/billing-manual-adjustment.request.schema.json`, and `schemas/api/billing-reservation-resolution.request.schema.json`.
- [X] T094 [US4] Add TypeScript SDK admin methods and stable quota-denial error helpers in `sdk/ts/src/index.ts`.
- [X] T095 [US4] Document admin plan, override, manual adjustment, lowered-quota, and audit retention behavior in `docs/runtime/hosted-billing-quotas.md`.
- [X] T096 [US4] Run targeted US4 verification commands and record results in `specs/023-billing-quotas-usage/quickstart.md`.

**Checkpoint**: User Story 4 is independently functional and testable through `go test ./internal/billing ./internal/api ./internal/audit ./internal/contracts` plus `pnpm test:clients`.

---

## Phase 7: User Story 5 - Planning Covers Every Guarded Quota Category (Priority: P2)

**Goal**: The first quota catalog and enforcement matrix remain complete during
implementation, and newly discovered expensive or side-effecting entry points cannot ship
without either an enforcement row or an explicit out-of-scope justification.

**Independent Test**: Run catalog and matrix completeness tests and verify every required
quota category and guarded entry point includes lifecycle points, operation identity,
concurrency guard, denial shape, and allowed/denied/retry/restart/concurrent tests.

### Tests for User Story 5

- [X] T097 [P] [US5] Add quota catalog contract test that compares `contracts/quota-catalog.md` required categories with `daemon/internal/billing/catalog.go` in `daemon/internal/contracts/billing_catalog_contract_test.go`.
- [X] T098 [P] [US5] Add enforcement matrix completeness test for guarded run, workflow, tool-call, Roadmap 38 live-validation gate contract, integration, artifact, and evaluation entry points in `daemon/internal/contracts/billing_enforcement_matrix_test.go`.
- [X] T099 [P] [US5] Add route and service signature scan tests for unguarded expensive or side-effecting hosted entry points in `daemon/internal/contracts/billing_enforcement_matrix_test.go`.
- [X] T100 [P] [US5] Add quickstart smoke checklist test fixture for over-quota denial, refund, manual adjustment, and operator-action-needed recovery evidence in `daemon/internal/contracts/billing_smoke_contract_test.go`.

### Implementation for User Story 5

- [X] T101 [US5] Implement machine-readable quota catalog export for contract tests in `daemon/internal/billing/catalog.go`.
- [X] T102 [US5] Implement machine-readable enforcement matrix registration in `daemon/internal/api/billing_enforcement.go`.
- [X] T103 [US5] Update `specs/023-billing-quotas-usage/contracts/quota-catalog.md` with any implementation-discovered category corrections.
- [X] T104 [US5] Update `specs/023-billing-quotas-usage/contracts/enforcement-matrix.md` with any implementation-discovered guarded entry point corrections.
- [X] T105 [US5] Update `specs/023-billing-quotas-usage/contracts/billing-usage-surfaces.md` with final response, event, SDK, and denial shapes.
- [X] T106 [US5] Document matrix completeness and future category addition rules in `docs/runtime/hosted-billing-quotas.md`.
- [X] T107 [US5] Run targeted US5 verification commands and record results in `specs/023-billing-quotas-usage/quickstart.md`.

**Checkpoint**: User Story 5 is independently functional and testable through `go test ./internal/contracts ./internal/billing ./internal/api`.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Finish verification, documentation, generated artifacts, and roadmap-level
release readiness after the desired story phases are complete.

- [X] T108 [P] Update Roadmap 38 runtime documentation and rollback notes in `docs/runtime/hosted-billing-quotas.md`.
- [X] T109 [P] Update live validation quota gate documentation in `docs/harness/sandbox-execution-plane.md`.
- [X] T110 [P] Update integration operation quota documentation in `docs/providers/provider-identity-and-profiles.md`.
- [X] T111 [P] Update hosted roadmap cross-reference documentation in `docs/product/hosted-productization-roadmap-split.md`.
- [X] T112 [P] Update generated TypeScript SDK distribution files after SDK source changes in `sdk/ts/dist/index.js`, `sdk/ts/dist/index.d.ts`, `sdk/ts/dist/index.test.js`, and `sdk/ts/dist/index.test.d.ts`.
- [X] T113 [P] Add or refresh billing API and event schema fixtures required by `make daemon-contract-test` in `daemon/internal/api/testdata/legacy_payloads/billing-usage-resource.json` and `daemon/internal/contracts/billing_contracts_test.go`.
- [X] T114 Run `make daemon-contract-test` from repository root and record the result in `specs/023-billing-quotas-usage/quickstart.md`.
- [X] T115 Run targeted Go tests from `specs/023-billing-quotas-usage/quickstart.md` in `daemon/` and record the result in `specs/023-billing-quotas-usage/quickstart.md`.
- [X] T116 Run `go test ./...` from `daemon/` and record the result in `specs/023-billing-quotas-usage/quickstart.md`.
- [X] T117 Run `pnpm test:clients` and `pnpm build` from repository root and record the result in `specs/023-billing-quotas-usage/quickstart.md`.
- [X] T118 Run the manual test-environment smoke from `specs/023-billing-quotas-usage/quickstart.md` using `make daemon-run-test` and `make daemon-test-status`.
- [X] T119 Run `go mod tidy` from `daemon/` after implementation and ensure `daemon/go.mod` and `daemon/go.sum` contain only intentional changes.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion and blocks all user story phases.
- **User Stories (Phases 3-7)**: Depend on Foundational completion. US1, US2, and US3 are all P1 and can proceed in parallel after the shared billing foundation is stable if file ownership is coordinated.
- **Polish (Phase 8)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational. Provides inspection and plan/usage visibility.
- **US2 (P1)**: Can start after Foundational. Uses shared billing manager and route helpers; does not depend on US1 UI behavior.
- **US3 (P1)**: Can start after Foundational. Uses shared storage and billing manager; should be integrated before considering hosted enforcement shippable.
- **US4 (P2)**: Can start after Foundational but should integrate after US1 projection and US3 recovery semantics are available.
- **US5 (P2)**: Can start after Foundational and should run continuously as guardrails while US2/US3 entry points are wired.

### Within Each User Story

- Tests must be written and fail before implementation when they cover new behavior.
- Storage and models before service behavior.
- Service behavior before API and SDK surfaces.
- API and event schemas before contract test completion.
- Story checkpoint must pass before treating that story as independently complete.

---

## Parallel Opportunities

- Setup tasks T002-T006 can run in parallel.
- Foundational tests T007-T012 can run in parallel before implementation.
- Foundational implementation can split by file ownership: billing package T013-T018, store T019-T021, audit/events T022-T023, schemas T024-T025, app wiring T026.
- US1 test tasks T027-T032 can run in parallel; implementation can split API, SDK, docs, and billing projection work after T033.
- US2 test tasks T041-T049 can run in parallel; implementation can split run/workflow/tool-call, live validation, integration, artifact, and evaluation wiring by file ownership.
- US3 test tasks T062-T069 can run in parallel; implementation can split billing manager, store transaction, recovery startup, audit/events, and schema work.
- US4 test tasks T080-T086 can run in parallel; implementation can split billing admin service, API routes, identity permissions, SDK, schemas, and docs.
- US5 test tasks T097-T100 can run in parallel; implementation can split catalog export, matrix registration, contract docs, and runtime docs.

---

## Parallel Examples

### User Story 1

```bash
Task: "T027 [P] [US1] Add plan and usage inspection API tests in daemon/internal/api/hosted_billing_test.go"
Task: "T029 [P] [US1] Add UTC period reset and carryover projection tests in daemon/internal/billing/projection_test.go"
Task: "T031 [P] [US1] Add SDK tests for plan, usage, quota, and denial inspection types in sdk/ts/src/index.test.ts"
```

### User Story 2

```bash
Task: "T041 [P] [US2] Add run launch quota denial tests in daemon/internal/api/billing_enforcement_test.go"
Task: "T047 [P] [US2] Add artifact write estimate reservation denial tests in daemon/internal/artifacts/billing_test.go"
Task: "T048 [P] [US2] Add replay/evaluation attempt quota gate tests in daemon/internal/evaluation/billing_test.go"
```

### User Story 3

```bash
Task: "T062 [P] [US3] Add reservation idempotency tests in daemon/internal/billing/manager_test.go"
Task: "T064 [P] [US3] Add concurrent last-unit reservation tests in daemon/internal/billing/concurrency_test.go"
Task: "T065 [P] [US3] Add restart recovery tests in daemon/internal/billing/recovery_test.go"
```

### User Story 4

```bash
Task: "T080 [P] [US4] Add admin plan assignment API tests in daemon/internal/api/hosted_billing_admin_test.go"
Task: "T082 [P] [US4] Add manual adjustment validation tests in daemon/internal/billing/adjustment_test.go"
Task: "T086 [P] [US4] Add SDK tests for admin billing types in sdk/ts/src/index.test.ts"
```

### User Story 5

```bash
Task: "T097 [P] [US5] Add quota catalog contract test in daemon/internal/contracts/billing_catalog_contract_test.go"
Task: "T098 [P] [US5] Add enforcement matrix completeness test in daemon/internal/contracts/billing_enforcement_matrix_test.go"
Task: "T100 [P] [US5] Add quickstart smoke checklist fixture in daemon/internal/contracts/billing_smoke_contract_test.go"
```

---

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundational billing infrastructure.
3. Complete US1 so tenants and operators can inspect plans and usage.
4. Complete US2 and US3 before enabling hosted quota enforcement. US1 alone is a useful
   inspection increment, but Roadmap 38 is not shippable until US2 and US3 also pass.

### Roadmap Closure

1. Finish P1 stories US1, US2, and US3.
2. Finish US4 administration and audit evidence.
3. Finish US5 catalog and enforcement matrix completeness guardrails.
4. Run Phase 8 verification and manual smoke.
5. Do not describe Roadmap 38 as complete until all story checkpoints and final
   verification tasks are complete or explicitly documented as blocked.

### Parallel Team Strategy

After Phase 2:

- Developer A: US1 inspection and SDK.
- Developer B: US2 run/workflow/tool-call and integration entry-point gating.
- Developer C: US3 idempotency, concurrency, and restart recovery.
- Developer D: US5 matrix completeness tests while US2/US3 wiring progresses.
- Developer E: US4 admin surfaces after US1 projection and US3 recovery state stabilize.

---

## Notes

- `[P]` tasks touch different files and can run in parallel when prerequisites are met.
- `[US1]` through `[US5]` labels map directly to the user stories in `spec.md`.
- Keep all local verification in `~/.dope-test`; do not touch production tenants, live connectors, payment-provider credentials, invoice systems, tax systems, or revenue-recognition systems.
- Preserve backward compatibility by keeping local development/unlimited plans explicit and non-denying by default.
- Contract changes must update `schemas/`, `daemon/internal/contracts/`, SDK types, and docs together.
- Stop at any checkpoint to validate the story independently before continuing.
