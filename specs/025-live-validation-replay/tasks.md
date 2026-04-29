# Tasks: Live Validation And Side-Effect Replay

**Input**: Design documents from `specs/025-live-validation-replay/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the constitution and by the feature specification. Contract, unit, integration, restart, fake-backend, SDK/client, and operator-evidence validation tasks are included before implementation tasks in each user story.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after the shared foundation is complete.

## Phase 1: Setup

**Purpose**: Establish the Roadmap 40 implementation locations without changing live-validation behavior.

- [X] T001 Create the live validation package scaffold and package documentation in `daemon/internal/livevalidation/doc.go`
- [X] T002 [P] Create Roadmap 40 fake-backend fixture notes in `specs/025-live-validation-replay/fixtures/README.md`
- [X] T003 [P] Create the live validation operator documentation index in `docs/harness/live-validation.md`
- [X] T004 [P] Create the live validation rollback and retention documentation stub in `docs/runtime/live-validation-operations.md`
- [X] T005 [P] Create the real-account live validation smoke documentation stub in `docs/providers/live-validation-smoke.md`
- [X] T006 [P] Add live validation contract-test fixture placeholders in `daemon/internal/contracts/testdata/live_validation/README.md`

---

## Phase 2: Foundational

**Purpose**: Shared types, persistence contracts, schema placeholders, permission constants, and support-matrix scaffolding that block all user stories.

**Critical**: No user story work should begin until this phase is complete.

- [X] T007 [P] Define live validation attempt, scope, approval, ledger, kill-switch, ambiguous-commit, reconciliation, comparison, and retention types in `daemon/internal/livevalidation/types.go`
- [X] T008 [P] Define replay support matrix row types, safety classes, retry policies, compensation types, and validation errors in `daemon/internal/livevalidation/matrix.go`
- [X] T009 [P] Define side-effect ledger outcome constants and transition validation helpers in `daemon/internal/livevalidation/ledger.go`
- [X] T010 [P] Define live validation store interface and query filters in `daemon/internal/livevalidation/store.go`
- [X] T011 [P] Add live validation permission constants and owner/admin reconciliation evaluation helpers in `daemon/internal/identity/types.go`
- [X] T012 [P] Add role-derived permission tests for live validation reconciliation authority in `daemon/internal/identity/permissions_test.go`
- [X] T013 Add SQLite schema migrations for live validation attempts, scopes, approvals, ledger entries, kill switches, ambiguous commits, reconciliation resolutions, comparisons, and retention policy records in `daemon/internal/store/store.go`
- [X] T014 Add store persistence methods for live validation attempts, support matrix snapshots, scopes, approvals, ledger entries, kill switches, ambiguous commits, reconciliations, comparisons, and retention policies in `daemon/internal/store/evaluation.go`
- [X] T015 [P] Add tenant isolation helpers for live validation records in `daemon/internal/store/tenancy/evaluation.go`
- [X] T016 [P] Add persistence schema tests for live validation tables and indexes in `daemon/internal/store/live_validation_schema_test.go`
- [X] T017 [P] Add store round-trip and restart persistence tests for live validation records in `daemon/internal/store/live_validation_test.go`
- [X] T018 [P] Add tenant isolation tests for live validation records in `daemon/internal/store/tenancy/live_validation_test.go`
- [X] T019 Add live validation attempt, attempt list, and denial API schema files in `schemas/api/live-validation-attempt-resource.schema.json`, `schemas/api/live-validation-attempt-list.response.schema.json`, and `schemas/api/live-validation-denial.schema.json`
- [X] T020 [P] Add side-effect ledger and support matrix API schema files in `schemas/api/live-validation-ledger-resource.schema.json`, `schemas/api/live-validation-ledger-list.response.schema.json`, and `schemas/api/live-validation-support-matrix-resource.schema.json`
- [X] T021 [P] Add kill-switch, reconciliation, retention, and comparison API schema files in `schemas/api/live-validation-kill-switch-resource.schema.json`, `schemas/api/live-validation-reconciliation-resource.schema.json`, and `schemas/api/live-validation-retention-resource.schema.json`, and `schemas/api/live-validation-comparison-resource.schema.json`
- [X] T022 [P] Add live validation lifecycle and side-effect event schema files in `schemas/events/live-validation-started.event.schema.json`, `schemas/events/live-validation-blocked.event.schema.json`, `schemas/events/live-validation-side-effect-recorded.event.schema.json`, and `schemas/events/live-validation-reconciliation-resolved.event.schema.json`
- [X] T023 [P] Add contract coverage for Roadmap 40 planning artifacts in `daemon/internal/contracts/live_validation_contracts_test.go`
- [X] T024 Add live validation manager constructor, dependencies, and no-op disabled behavior in `daemon/internal/livevalidation/manager.go`
- [X] T025 Add API server dependency wiring for the live validation manager without mounting side-effect execution in `daemon/internal/api/server.go`
- [X] T026 Update TypeScript permission union with reconciliation permission and placeholder live validation resource exports in `sdk/ts/src/index.ts`

**Checkpoint**: Foundation ready. User story implementation can now begin with shared types, storage, permission constants, and schema placeholders in place.

---

## Phase 3: User Story 1 - Run Permissioned Live Validation (Priority: P1) 🎯 MVP

**Goal**: Operators can request live validation for an eligible replay candidate only after tenant permission, quota, kill-switch, support readiness, explicit scope, and required fresh approvals are satisfied.

**Independent Test**: Attempt live validation with missing permission, exhausted quota, active kill switch, missing approvals, and then a fully authorized fake-backed request; verify all denials occur before external side effects and the accepted attempt records gate evidence.

### Tests for User Story 1

- [X] T027 [P] [US1] Add manager tests for missing `live_validation.execute` denial before quota or side effects in `daemon/internal/livevalidation/manager_start_test.go`
- [X] T028 [P] [US1] Add manager tests for hosted quota denial and quota-state-unavailable fail-closed behavior in `daemon/internal/livevalidation/quota_test.go`
- [X] T029 [P] [US1] Add manager tests for tenant/global kill-switch start denial before side effects in `daemon/internal/livevalidation/kill_switch_test.go`
- [X] T030 [P] [US1] Add manager tests for scope-level versus per-action fresh approval requirements in `daemon/internal/livevalidation/approval_test.go`
- [X] T031 [P] [US1] Add API contract tests for live validation start, blocked, awaiting-approval, and accepted responses in `daemon/internal/contracts/live_validation_api_contract_test.go`
- [X] T032 [P] [US1] Add API route tests for permission, quota, kill-switch, support, and approval denials in `daemon/internal/api/live_validation_test.go`
- [X] T033 [P] [US1] Add SDK tests for starting live validation and mapping stable denial payloads in `sdk/ts/src/index.test.ts`
- [X] T034 [P] [US1] Add web operator shell tests for explicit scope selection and gate-denial display in `web/src/app/App.test.tsx`

### Implementation for User Story 1

- [X] T035 [US1] Implement live validation start orchestration with permission, quota, kill-switch, support, scope, and approval gates in `daemon/internal/livevalidation/manager.go`
- [X] T036 [US1] Reuse Roadmap 38 live-validation quota preflight gate and commit/refund lifecycle in `daemon/internal/api/billing_enforcement.go`
- [X] T037 [US1] Implement tenant/global kill-switch read checks for start gating in `daemon/internal/livevalidation/kill_switch.go`
- [X] T038 [US1] Implement fresh approval requirement calculation for scope-level and per-action approval in `daemon/internal/livevalidation/approval.go`
- [X] T039 [US1] Implement live validation attempt persistence and gate evidence updates in `daemon/internal/livevalidation/attempts.go`
- [X] T040 [US1] Add live validation start, list, get, and abort route handlers in `daemon/internal/api/live_validation.go`
- [X] T041 [US1] Mount live validation routes and tenant guards in `daemon/internal/api/server.go`
- [X] T042 [US1] Extend replay candidate API behavior so `mode: live_validation` cannot bypass Roadmap 40 gates in `daemon/internal/api/evaluation.go`
- [X] T043 [US1] Extend evaluation manager handoff from replay candidate to live validation manager in `daemon/internal/evaluation/manager.go`
- [X] T044 [US1] Emit live validation started, blocked, and awaiting-approval events in `daemon/internal/events/live_validation.go`
- [X] T045 [US1] Record audit evidence for live validation start gate decisions in `daemon/internal/audit/live_validation.go`
- [X] T046 [US1] Add live validation start request and start response API schemas in `schemas/api/create-live-validation.request.schema.json` and `schemas/api/create-live-validation.response.schema.json`
- [X] T047 [US1] Add awaiting-approval event schema in `schemas/events/live-validation-awaiting-approval.event.schema.json`
- [X] T048 [US1] Add SDK resource types and methods for start/list/get/abort live validation in `sdk/ts/src/index.ts`
- [X] T049 [US1] Add web operator shell controls for explicit live validation scope and gate status inspection in `web/src/app/App.tsx`
- [X] T050 [US1] Add live validation scope and gate styling in `web/src/styles.css`
- [X] T051 [US1] Update live validation operator start workflow docs in `docs/harness/live-validation.md`

**Checkpoint**: User Story 1 is complete when live validation can be accepted or denied through structured gates without executing unsupported or unapproved side effects.

---

## Phase 4: User Story 2 - Replay Only Explicitly Supported Tool Classes (Priority: P1)

**Goal**: Every reachable replay tool-call class has an explicit safety classification and proving test; missing and unsupported classes never live-replay silently, and mixed candidates can proceed only after unsupported work is explicitly excluded.

**Independent Test**: Review and execute matrix completeness tests for all required classes, then validate that a mixed supported/unsupported candidate runs only supported steps after explicit exclusion.

### Tests for User Story 2

- [X] T052 [P] [US2] Add replay support matrix validation tests for required columns and missing-row unsupported behavior in `daemon/internal/livevalidation/matrix_test.go`
- [X] T053 [P] [US2] Add matrix completeness contract tests for every required tool class in `daemon/internal/contracts/live_validation_matrix_contract_test.go`
- [X] T054 [P] [US2] Add mixed supported/unsupported candidate readiness tests in `daemon/internal/livevalidation/readiness_test.go`
- [X] T055 [P] [US2] Add runtime local tool-call classification tests in `daemon/internal/runtime/live_validation_test.go`
- [X] T056 [P] [US2] Add MCP unsupported-by-default classification tests in `daemon/internal/mcp/live_validation_test.go`
- [X] T057 [P] [US2] Add calendar matrix classification tests for create/update/cancel in `daemon/internal/calendar/live_validation_test.go`
- [X] T058 [P] [US2] Add mail matrix classification tests for draft/send/reply/forward in `daemon/internal/mail/live_validation_test.go`
- [X] T059 [P] [US2] Add delivery and connector message-send classification tests in `daemon/internal/delivery/live_validation_test.go`
- [X] T060 [P] [US2] Add SDK tests for support matrix inspection and unsupported class responses in `sdk/ts/src/index.test.ts`
- [X] T061 [P] [US2] Add web tests for support matrix inspection and unsupported class display in `web/src/app/App.test.tsx`
- [X] T062 [P] [US2] Add reminder lifecycle matrix classification tests in `daemon/internal/reminders/live_validation_test.go`

### Implementation for User Story 2

- [X] T063 [US2] Implement support matrix registration, validation, and lookup APIs in `daemon/internal/livevalidation/matrix.go`
- [X] T064 [US2] Implement readiness evaluation for unsupported, excluded, supported, and mixed candidates in `daemon/internal/livevalidation/readiness.go`
- [X] T065 [US2] Add daemon inspection and runtime local tool-call matrix rows in `daemon/internal/runtime/live_validation.go`
- [X] T066 [US2] Add MCP unsupported-by-default matrix rows and override extension points in `daemon/internal/mcp/live_validation.go`
- [X] T067 [US2] Add integration probe read and mutation matrix rows in `daemon/internal/integrations/live_validation.go`
- [X] T068 [US2] Add calendar create/update/cancel matrix rows in `daemon/internal/calendar/live_validation.go`
- [X] T069 [US2] Add mail draft/create/update/send/reply/forward matrix rows in `daemon/internal/mail/live_validation.go`
- [X] T070 [US2] Add reminder lifecycle matrix row wiring in `daemon/internal/reminders/live_validation.go`
- [X] T071 [US2] Add delivery dispatch and connector message-send matrix rows in `daemon/internal/delivery/live_validation.go`
- [X] T072 [US2] Add provider/sandbox unsupported matrix rows in `daemon/internal/livevalidation/matrix.go`
- [X] T073 [US2] Add support matrix API route and schema response in `daemon/internal/api/live_validation.go`
- [X] T074 [US2] Update support matrix planning contract with implementation row names and test references in `specs/025-live-validation-replay/contracts/replay-support-matrix.md`
- [X] T075 [US2] Add SDK support matrix types and list method in `sdk/ts/src/index.ts`
- [X] T076 [US2] Add web support matrix and unsupported class inspection view in `web/src/app/App.tsx`

**Checkpoint**: User Story 2 is complete when every required tool class has a matrix row and unsupported work cannot run unless explicitly excluded from live-validation scope.

---

## Phase 5: User Story 3 - Inspect Side-Effect Ledger And Outcome Comparison (Priority: P1)

**Goal**: Engineers and operators can inspect durable side-effect ledger entries, ambiguous commits, reconciliation states, retention behavior, and original-versus-live comparisons without raw log reconstruction.

**Independent Test**: Run fake-backend live-validation scenarios for completed, failed, skipped, denied, aborted, timeout-after-submit, restart-after-submit, duplicate retry, ambiguous commit, unauthorized reconciliation, authorized reconciliation, and comparison output.

### Tests for User Story 3

- [X] T077 [P] [US3] Add ledger transition tests for attempted, skipped, completed, failed, aborted, denied, and operator-action-needed outcomes in `daemon/internal/livevalidation/ledger_test.go`
- [X] T078 [P] [US3] Add side-effect executor tests for correlation/idempotency key propagation in `daemon/internal/livevalidation/executor_test.go`
- [X] T079 [P] [US3] Add fake integration backend tests for completed, failed, timeout-after-submit, and duplicate retry behavior in `daemon/internal/integrations/live_validation_fake_test.go`
- [X] T080 [P] [US3] Add calendar fake-backend ambiguous commit and reconciliation tests in `daemon/internal/calendar/live_validation_fake_test.go`
- [X] T081 [P] [US3] Add mail fake-backend non-idempotent send/reply/forward tests in `daemon/internal/mail/live_validation_fake_test.go`
- [X] T082 [P] [US3] Add delivery fake-backend dispatch tests for completed, failed, duplicate retry, submit-unknown, and approval evidence in `daemon/internal/delivery/live_validation_fake_test.go`
- [X] T083 [P] [US3] Add connector message-send fake-backend tests for completed, failed, submit-unknown, ambiguous commit, and no automatic retry in `daemon/internal/connectors/live_validation_fake_test.go`
- [X] T084 [P] [US3] Add reminder lifecycle fake-backend tests for completed, failed, duplicate retry, submit-unknown, and operator-action-needed evidence in `daemon/internal/reminders/live_validation_fake_test.go`
- [X] T085 [P] [US3] Add restart-after-submit persistence tests for ambiguous commits in `daemon/internal/store/live_validation_restart_test.go`
- [X] T086 [P] [US3] Add reconciliation authority tests for unauthorized and authorized resolution in `daemon/internal/livevalidation/reconciliation_test.go`
- [X] T087 [P] [US3] Add retention default and active reconciliation protection tests in `daemon/internal/livevalidation/retention_test.go`
- [X] T088 [P] [US3] Add comparison tests for matched, drifted, blocked, unsupported, denied, and operator-action-needed outcomes in `daemon/internal/livevalidation/comparison_test.go`
- [X] T089 [P] [US3] Add API contract tests for ledger, reconciliation, retention, and comparison shapes in `daemon/internal/contracts/live_validation_ledger_contract_test.go`
- [X] T090 [P] [US3] Add API route tests for ledger listing, reconciliation resolution, and comparison creation in `daemon/internal/api/live_validation_ledger_test.go`
- [X] T091 [P] [US3] Add SDK tests for ledger, reconciliation, retention, and comparison methods in `sdk/ts/src/index.test.ts`
- [X] T092 [P] [US3] Add web tests for ledger, ambiguous commit, reconciliation, and comparison inspection in `web/src/app/App.test.tsx`

### Implementation for User Story 3

- [X] T093 [US3] Implement side-effect ledger append, update, and transition enforcement in `daemon/internal/livevalidation/ledger.go`
- [X] T094 [US3] Implement live side-effect executor orchestration with matrix retry policy and approval checks in `daemon/internal/livevalidation/executor.go`
- [X] T095 [US3] Implement correlation and idempotency key generation for external side-effect attempts in `daemon/internal/livevalidation/idempotency.go`
- [X] T096 [US3] Implement ambiguous commit detection and automatic retry stop logic in `daemon/internal/livevalidation/ambiguous_commit.go`
- [X] T097 [US3] Implement reconciliation resolution and permission enforcement in `daemon/internal/livevalidation/reconciliation.go`
- [X] T098 [US3] Implement retention policy defaults and active reconciliation protection in `daemon/internal/livevalidation/retention.go`
- [X] T099 [US3] Implement original-versus-live comparison enrichment in `daemon/internal/livevalidation/comparison.go`
- [X] T100 [US3] Extend fake integration backend with live validation completion, failure, timeout-after-submit, and duplicate retry controls in `daemon/internal/integrations/fake_backend.go`
- [X] T101 [US3] Extend calendar fake backend with live validation mutation outcomes and ambiguous commit controls in `daemon/internal/calendar/fake_backend.go`
- [X] T102 [US3] Extend mail fake backend with draft/send/reply/forward live validation outcomes and ambiguous commit controls in `daemon/internal/mail/fake_backend.go`
- [X] T103 [US3] Extend delivery test sink with live validation completion, failure, duplicate retry, and submit-unknown controls in `daemon/internal/delivery/test_sink.go`
- [X] T104 [US3] Extend connector message-send fake path with live validation outcome and ambiguous commit controls in `daemon/internal/connectors/supervisor.go`
- [X] T105 [US3] Extend reminder manager fake path with live validation lifecycle completion, failure, duplicate retry, and submit-unknown controls in `daemon/internal/reminders/manager.go`
- [X] T106 [US3] Add ledger list, reconciliation resolve, retention inspect, and comparison route handlers in `daemon/internal/api/live_validation.go`
- [X] T107 [US3] Emit live validation side-effect recorded, operator-action-needed, reconciliation-resolved, completed, and comparison-completed events in `daemon/internal/events/live_validation.go`
- [X] T108 [US3] Record audit events for ledger outcomes and reconciliation decisions in `daemon/internal/audit/live_validation.go`
- [X] T109 [US3] Add reconciliation resolve request and comparison create response API schemas in `schemas/api/resolve-live-validation-reconciliation.request.schema.json` and `schemas/api/create-live-validation-comparison.response.schema.json`
- [X] T110 [US3] Add operator-action-needed, completed, and comparison-completed event schemas in `schemas/events/live-validation-operator-action-needed.event.schema.json`, `schemas/events/live-validation-completed.event.schema.json`, and `schemas/events/live-validation-comparison-completed.event.schema.json`
- [X] T111 [US3] Add SDK resource types and methods for ledger, reconciliation, retention, and comparison operations in `sdk/ts/src/index.ts`
- [X] T112 [US3] Add web operator shell ledger, ambiguous commit, reconciliation, retention, and comparison views in `web/src/app/App.tsx`
- [X] T113 [US3] Add ledger and reconciliation styling in `web/src/styles.css`
- [X] T114 [US3] Update side-effect ledger contract with final event names, resource names, and fake-backend tests in `specs/025-live-validation-replay/contracts/side-effect-ledger.md`
- [X] T115 [US3] Update live validation smoke instructions for ledger, comparison, and reconciliation evidence in `specs/025-live-validation-replay/quickstart.md`

**Checkpoint**: User Story 3 is complete when every live side-effect path produces durable ledger evidence and operators can reconcile ambiguous commits and compare outcomes from structured surfaces.

---

## Phase 6: User Story 4 - Disable Live Validation During Operational Risk (Priority: P2)

**Goal**: Tenant owners and authorized operators can enable tenant or global kill switches that block new live validation starts and abort pending/future side effects in running attempts while preserving history and non-live replay.

**Independent Test**: Enable tenant and global kill switches, attempt new live validation, activate a switch during a running attempt, and verify starts are blocked, pending/future side effects abort, already-submitted side effects reconcile truthfully, and non-live replay remains inspectable.

### Tests for User Story 4

- [X] T116 [P] [US4] Add kill-switch state transition and authorization tests in `daemon/internal/livevalidation/kill_switch_test.go`
- [X] T117 [P] [US4] Add running-attempt kill-switch abort tests for pending, future, and submitted side effects in `daemon/internal/livevalidation/abort_test.go`
- [X] T118 [P] [US4] Add API tests for tenant/global kill-switch set, list, and effective-state responses in `daemon/internal/api/live_validation_kill_switch_test.go`
- [X] T119 [P] [US4] Add non-live replay unaffected-by-kill-switch tests in `daemon/internal/evaluation/manager_test.go`
- [X] T120 [P] [US4] Add SDK tests for kill-switch controls and running-attempt abort responses in `sdk/ts/src/index.test.ts`
- [X] T121 [P] [US4] Add web tests for kill-switch controls and running-attempt abort status display in `web/src/app/App.test.tsx`

### Implementation for User Story 4

- [X] T122 [US4] Implement kill-switch persistence, effective-state resolution, and authorization checks in `daemon/internal/livevalidation/kill_switch.go`
- [X] T123 [US4] Implement operator abort and kill-switch abort handling for pending and future side effects in `daemon/internal/livevalidation/abort.go`
- [X] T124 [US4] Integrate kill-switch checks into live validation start, approval wait, executor loop, and retry decisions in `daemon/internal/livevalidation/manager.go`
- [X] T125 [US4] Add kill-switch set/list/effective API route handlers in `daemon/internal/api/live_validation.go`
- [X] T126 [US4] Add kill-switch changed and live validation aborted event emission in `daemon/internal/events/live_validation.go`
- [X] T127 [US4] Add kill-switch audit records with actor, tenant/global scope, reason, and timestamp in `daemon/internal/audit/live_validation.go`
- [X] T128 [US4] Add kill-switch mutation request and aborted attempt event schemas in `schemas/api/update-live-validation-kill-switch.request.schema.json` and `schemas/events/live-validation-aborted.event.schema.json`
- [X] T129 [US4] Add SDK methods for kill-switch list and mutation in `sdk/ts/src/index.ts`
- [X] T130 [US4] Add web operator shell kill-switch controls and running-attempt abort status display in `web/src/app/App.tsx`
- [X] T131 [US4] Update runtime operations docs with kill-switch containment and rollback guidance in `docs/runtime/live-validation-operations.md`

**Checkpoint**: User Story 4 is complete when live validation can be stopped during incidents without hiding historical evidence or breaking non-live replay.

---

## Phase 7: Polish & Cross-Cutting Verification

**Purpose**: Final schema consistency, docs, client coverage, operator smoke, and repository-level verification.

- [X] T132 [P] Reconcile final live validation route, event, SDK, and schema names in `specs/025-live-validation-replay/contracts/live-validation-surfaces.md`
- [X] T133 [P] Reconcile final matrix rows, tool class names, and proving test names in `specs/025-live-validation-replay/contracts/replay-support-matrix.md`
- [X] T134 [P] Reconcile final data model names, state transitions, and retention notes in `specs/025-live-validation-replay/data-model.md`
- [X] T135 [P] Update live validation architecture and operator docs in `docs/harness/live-validation.md`
- [X] T136 [P] Update live validation smoke policy and skip evidence guidance in `docs/providers/live-validation-smoke.md`
- [X] T137 [P] Update Roadmap 40 references and completion notes in `docs/runtime/daemon-roadmaps.md`
- [X] T138 [P] Update upstream Roadmap 40 spec status and artifact links in `docs/specs/025-live-validation-and-side-effect-replay.md`
- [X] T139 [P] Run targeted Go tests for livevalidation, evaluation, billing, identity, store, API, fake backends, events, audit, and contracts; record results in `specs/025-live-validation-replay/quickstart.md`
- [X] T140 Run `go test ./...` from `daemon/` and record results in `specs/025-live-validation-replay/quickstart.md`
- [X] T141 Run `go mod tidy` from `daemon/` and record module diff status in `specs/025-live-validation-replay/quickstart.md`
- [X] T142 Run `make daemon-contract-test` from the repository root and record results in `specs/025-live-validation-replay/quickstart.md`
- [X] T143 Run `pnpm test:clients` and record results in `specs/025-live-validation-replay/quickstart.md`
- [X] T144 Run `pnpm build` and record results in `specs/025-live-validation-replay/quickstart.md`
- [X] T145 Run the test daemon smoke with `make daemon-run-test` and `make daemon-test-status`, then record health and shutdown evidence in `specs/025-live-validation-replay/quickstart.md`
- [X] T146 Run the `DOPE_ENV=test` manual live validation smoke for success, denial, unsupported class, ambiguous commit, reconciliation, kill-switch abort, restart inspection, and non-live replay compatibility; record results in `specs/025-live-validation-replay/quickstart.md`
- [X] T147 Run optional `make daemon-run-test-live` real-account smoke only when safe credentials are explicitly provided, or record skip rationale and fake-backend coverage in `specs/025-live-validation-replay/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Phase 1 and blocks all user stories.
- **User Stories (Phases 3-6)**: Depend on Phase 2. User Stories 1, 2, and 3 are P1 and should be completed before Roadmap 40 is considered closed. User Story 4 is P2 but required for full Roadmap 40 completion.
- **Polish (Phase 7)**: Depends on all user stories selected for implementation; full roadmap closure requires all user stories.

### User Story Dependencies

- **US1 Run Permissioned Live Validation**: Can start after foundation. This is the MVP checkpoint for live-validation start gates.
- **US2 Replay Only Explicitly Supported Tool Classes**: Can start after foundation and may proceed in parallel with US1 if matrix file ownership is coordinated. US1 cannot execute live side effects safely until US2 support rows are complete.
- **US3 Inspect Side-Effect Ledger And Outcome Comparison**: Can start after foundation, but full execution depends on US1 start gates and US2 matrix behavior.
- **US4 Disable Live Validation During Operational Risk**: Can start after foundation, but running-attempt kill-switch behavior depends on US3 ledger and executor semantics.

### Within Each User Story

- Write story tests before implementation.
- Add contract/schema tests before route and SDK implementation.
- Implement store/types before services.
- Implement services before API routes.
- Implement API routes before SDK/web surfaces.
- Update docs and quickstart after implementation names stabilize.

## Parallel Opportunities

- Setup tasks T002-T006 can run in parallel after T001 is assigned.
- Foundational type, schema, identity, and contract tasks T007-T012 and T015-T023 can run in parallel if `daemon/internal/store/store.go` ownership is isolated to T013.
- US1 tests T027-T034 can run in parallel.
- US2 classification, SDK, and web support-matrix tests T052-T062 can run in parallel.
- US3 ledger, fake-backend, restart, reconciliation, retention, comparison, API, SDK, and web tests T077-T092 can run in parallel.
- US4 kill-switch tests T116-T121 can run in parallel.
- Polish documentation tasks T132-T138 can run in parallel with final verification tasks once implementation names are stable.

## Parallel Example: User Story 1

```text
Task: "T027 Add manager tests for missing live_validation.execute denial before quota or side effects in daemon/internal/livevalidation/manager_start_test.go"
Task: "T028 Add manager tests for hosted quota denial and quota-state-unavailable fail-closed behavior in daemon/internal/livevalidation/quota_test.go"
Task: "T029 Add manager tests for tenant/global kill-switch start denial before side effects in daemon/internal/livevalidation/kill_switch_test.go"
Task: "T030 Add manager tests for scope-level versus per-action fresh approval requirements in daemon/internal/livevalidation/approval_test.go"
Task: "T031 Add API contract tests for live validation start, blocked, awaiting-approval, and accepted responses in daemon/internal/contracts/live_validation_api_contract_test.go"
Task: "T033 Add SDK tests for starting live validation and mapping stable denial payloads in sdk/ts/src/index.test.ts"
```

## Parallel Example: User Story 2

```text
Task: "T052 Add replay support matrix validation tests for required columns and missing-row unsupported behavior in daemon/internal/livevalidation/matrix_test.go"
Task: "T055 Add runtime local tool-call classification tests in daemon/internal/runtime/live_validation_test.go"
Task: "T056 Add MCP unsupported-by-default classification tests in daemon/internal/mcp/live_validation_test.go"
Task: "T057 Add calendar matrix classification tests for create/update/cancel in daemon/internal/calendar/live_validation_test.go"
Task: "T058 Add mail matrix classification tests for draft/send/reply/forward in daemon/internal/mail/live_validation_test.go"
Task: "T059 Add delivery and connector message-send classification tests in daemon/internal/delivery/live_validation_test.go"
Task: "T061 Add web tests for support matrix inspection and unsupported class display in web/src/app/App.test.tsx"
Task: "T062 Add reminder lifecycle matrix classification tests in daemon/internal/reminders/live_validation_test.go"
```

## Parallel Example: User Story 3

```text
Task: "T077 Add ledger transition tests for attempted, skipped, completed, failed, aborted, denied, and operator-action-needed outcomes in daemon/internal/livevalidation/ledger_test.go"
Task: "T079 Add fake integration backend tests for completed, failed, timeout-after-submit, and duplicate retry behavior in daemon/internal/integrations/live_validation_fake_test.go"
Task: "T080 Add calendar fake-backend ambiguous commit and reconciliation tests in daemon/internal/calendar/live_validation_fake_test.go"
Task: "T081 Add mail fake-backend non-idempotent send/reply/forward tests in daemon/internal/mail/live_validation_fake_test.go"
Task: "T082 Add delivery fake-backend dispatch tests for completed, failed, duplicate retry, submit-unknown, and approval evidence in daemon/internal/delivery/live_validation_fake_test.go"
Task: "T083 Add connector message-send fake-backend tests for completed, failed, submit-unknown, ambiguous commit, and no automatic retry in daemon/internal/connectors/live_validation_fake_test.go"
Task: "T084 Add reminder lifecycle fake-backend tests for completed, failed, duplicate retry, submit-unknown, and operator-action-needed evidence in daemon/internal/reminders/live_validation_fake_test.go"
Task: "T085 Add restart-after-submit persistence tests for ambiguous commits in daemon/internal/store/live_validation_restart_test.go"
Task: "T088 Add comparison tests for matched, drifted, blocked, unsupported, denied, and operator-action-needed outcomes in daemon/internal/livevalidation/comparison_test.go"
```

## Parallel Example: User Story 4

```text
Task: "T116 Add kill-switch state transition and authorization tests in daemon/internal/livevalidation/kill_switch_test.go"
Task: "T117 Add running-attempt kill-switch abort tests for pending, future, and submitted side effects in daemon/internal/livevalidation/abort_test.go"
Task: "T118 Add API tests for tenant/global kill-switch set, list, and effective-state responses in daemon/internal/api/live_validation_kill_switch_test.go"
Task: "T119 Add non-live replay unaffected-by-kill-switch tests in daemon/internal/evaluation/manager_test.go"
Task: "T120 Add SDK tests for kill-switch controls and running-attempt abort responses in sdk/ts/src/index.test.ts"
Task: "T121 Add web tests for kill-switch controls and running-attempt abort status display in web/src/app/App.test.tsx"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 for explicit live-validation start gates.
3. Stop and validate that live validation is accepted or denied truthfully without executing unsupported or unapproved side effects.

### Roadmap Closure

1. Complete US1 for permission, quota, kill-switch, scope, and approval gates.
2. Complete US2 for replay support matrix completeness and unsupported class safety.
3. Complete US3 for side-effect ledger, fake-backend execution, ambiguous commit, reconciliation, retention, and comparison evidence.
4. Complete US4 for tenant/global kill switches and running-attempt containment.
5. Complete Phase 7 verification and record residual risks.

### Parallel Team Strategy

With multiple implementers:

1. Complete Setup and Foundational phases together.
2. Assign US1 to live-validation start/API owner.
3. Assign US2 to matrix/domain classification owner.
4. Assign US3 to ledger/executor/fake-backend owner.
5. Assign US4 to kill-switch/abort/operator-controls owner.
6. Reserve `daemon/internal/api/live_validation.go`, `daemon/internal/livevalidation/manager.go`, and `sdk/ts/src/index.ts` for coordinated integration to avoid file conflicts.

## Notes

- `[P]` tasks use different files or can be done without depending on incomplete tasks.
- `[US1]`, `[US2]`, `[US3]`, and `[US4]` map directly to the feature spec user stories.
- Every story includes tests first because Roadmap 40 changes production safety boundaries.
- Non-live replay must remain the default and must be rechecked during US1, US4, and final verification.
- Optional real-account smoke never replaces fake-backend automated coverage.
