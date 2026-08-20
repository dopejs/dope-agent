# Tasks: Integration Health And Permission Diagnostics

**Input**: Design documents from `/specs/027-integration-diagnostics/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Constitution rules apply. Every production change MUST include targeted
verification tasks. Contract-style verification is REQUIRED whenever API, schema, event,
config, or persistence surfaces change.

**Organization**: Tasks are grouped by user story to enable independent implementation
and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish shared diagnostic contract files, fixtures, and repo anchors before implementation.

- [X] T001 Create diagnostic API schema placeholders from contracts in schemas/api/integration-diagnostic-result.schema.json, schemas/api/integration-diagnostic-run.schema.json, schemas/api/integration-diagnostic-reason-code.schema.json, schemas/api/integration-diagnostic-list.response.schema.json, schemas/api/create-integration-diagnostic-run.request.schema.json, schemas/api/smoke-matrix-report.schema.json, and schemas/api/smoke-probe-outcome.schema.json
- [X] T002 [P] Create diagnostic event schema placeholders from contracts in schemas/events/integration-diagnostic-run-started.event.schema.json, schemas/events/integration-diagnostic-run-completed.event.schema.json, schemas/events/integration-diagnostic-state-changed.event.schema.json, schemas/events/integration-diagnostic-redaction-failed.event.schema.json, schemas/events/integration-diagnostic-smoke-completed.event.schema.json, and schemas/events/integration-diagnostic-retention-applied.event.schema.json
- [X] T003 [P] Add Feishu/Lark provider classification fixture skeletons in daemon/internal/integrations/testdata/diagnostics/feishu_lark_reason_codes.json
- [X] T004 [P] Add smoke report and release-readiness fixture skeletons in daemon/internal/opsreadiness/testdata/r42_smoke_report.json and daemon/internal/opsreadiness/testdata/r42_release_readiness.json
- [X] T005 [P] Add diagnostic contract fixture skeletons in daemon/internal/contracts/testdata/integration_diagnostics/
- [X] T006 [P] Add Roadmap 42 diagnostic docs stubs in docs/providers/integration-diagnostics.md, docs/harness/integration-diagnostic-smoke.md, and docs/runtime/integration-diagnostic-readiness.md

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core persistence, domain types, reason-code catalog, permissions, redaction, and routing that MUST be complete before user stories.

**CRITICAL**: No user story work can begin until this phase is complete.

- [X] T007 Define diagnostic domain types, statuses, reason codes, retry-safety values, remediation owners, freshness states, and retention constants in daemon/internal/integrations/diagnostics_types.go
- [X] T008 Define smoke report and probe outcome shared domain types in daemon/internal/opsreadiness/integration_diagnostic_smoke_types.go
- [X] T009 Add diagnostic persistence schema, indexes, and migrations for diagnostic runs, latest results, provider classifications, smoke reports, probe outcomes, and retention records in daemon/internal/store/integration_diagnostics.go
- [X] T010 [P] Add migration fixture coverage for Roadmap 42 diagnostic tables in daemon/internal/store/migrationfixture/r42_integration_diagnostics.go and daemon/internal/store/migrationfixture/r42_integration_diagnostics_test.go
- [X] T011 Add tenant-safe diagnostic store accessors and cross-tenant guards in daemon/internal/store/tenancy/integration_diagnostics.go
- [X] T012 Add store tests for diagnostic schema, deterministic pagination, idempotency keys, freshness timestamps, retention expiry columns, and tenant isolation in daemon/internal/store/integration_diagnostics_test.go and daemon/internal/store/tenancy/integration_diagnostics_test.go
- [X] T013 [P] Add diagnostic permissions and denial reason constants in daemon/internal/identity/permissions.go
- [X] T014 Add diagnostic redaction helpers and fail-closed redaction result types in daemon/internal/integrations/diagnostics_redaction.go
- [X] T015 Add integration diagnostic audit helpers in daemon/internal/audit/integration_diagnostics.go
- [X] T016 [P] Add integration diagnostic event builders in daemon/internal/events/integration_diagnostics.go
- [X] T017 Add API response/request resource types for diagnostics and smoke references in daemon/internal/api/types.go
- [X] T018 Add route registration stubs for diagnostic routes in daemon/internal/api/server.go and daemon/internal/api/integration_diagnostics.go
- [X] T019 Add contract tests that validate all new diagnostic API and event schemas against fixture payloads in daemon/internal/contracts/integration_diagnostics_test.go
- [X] T020 Add TypeScript SDK diagnostic resource and method type placeholders in sdk/ts/src/index.ts

**Checkpoint**: Foundation ready - user story implementation can now begin, with shared SDK and web file edits serialized as noted in Dependencies.

---

## Phase 3: User Story 1 - Inspect Integration Readiness (Priority: P1) MVP

**Goal**: Authorized operators can inspect a Feishu/Lark integration and see tenant-scoped readiness, reason codes, freshness, remediation owner, and redacted evidence.

**Independent Test**: Seed representative Feishu/Lark diagnostic states for one tenant, request diagnostics as authorized and unauthorized users, and verify scoped readiness, 15-minute stale marking, remediation hints, timestamps, and absence of secret material.

### Tests for User Story 1

- [X] T021 [P] [US1] Add integration diagnostic result store tests for latest-state lookup, stale-after-15-minutes behavior, and retention visibility in daemon/internal/store/integration_diagnostics_test.go
- [X] T022 [P] [US1] Add tenant access and non-disclosure tests for diagnostic reads and run starts in daemon/internal/api/integration_diagnostics_test.go
- [X] T023 [P] [US1] Add SDK tests for list diagnostics, start diagnostic run, list runs, inspect run, and list reason codes in sdk/ts/src/index.test.ts
- [X] T024 [P] [US1] Add web operator-shell fixture tests for healthy, blocked, limited, unsupported, stale, and redaction-failed diagnostic states in web/src/features/integration-diagnostics.test.tsx
- [X] T025 [US1] Add SC-001 inspection-time assertions proving an authorized operator can determine Feishu/Lark diagnostic status and remediation owner within the 2-minute target in daemon/internal/api/integration_diagnostics_test.go and daemon/internal/integrations/diagnostics_manager_test.go

### Implementation for User Story 1

- [X] T026 [US1] Implement diagnostic result and diagnostic run persistence methods in daemon/internal/store/integration_diagnostics.go
- [X] T027 [US1] Implement tenant-safe diagnostic read/write wrappers in daemon/internal/store/tenancy/integration_diagnostics.go
- [X] T028 [US1] Implement diagnostic manager for inspection, run creation, cached-state freshness, and result projection in daemon/internal/integrations/diagnostics_manager.go
- [X] T029 [US1] Implement limited and unsupported diagnostic projection for non-Feishu/Lark domains in daemon/internal/integrations/diagnostics_domains.go
- [X] T030 [US1] Implement GET /v1/integrations/{integrationId}/diagnostics, POST /v1/integrations/{integrationId}/diagnostics/runs, GET /v1/integration-diagnostics/runs, GET /v1/integration-diagnostics/runs/{runId}, and GET /v1/integration-diagnostics/reason-codes in daemon/internal/api/integration_diagnostics.go
- [X] T031 [US1] Emit diagnostic run started, completed, failed, and state-changed audit/events from daemon/internal/integrations/diagnostics_manager.go, daemon/internal/audit/integration_diagnostics.go, and daemon/internal/events/integration_diagnostics.go
- [X] T032 [US1] Implement TypeScript SDK methods and resource exports for integration diagnostics in sdk/ts/src/index.ts
- [X] T033 [US1] Implement web operator diagnostic inspection surface in web/src/features/integration-diagnostics.tsx
- [X] T034 [US1] Update diagnostic API schemas and contract fixtures for US1 routes in schemas/api/integration-diagnostic-result.schema.json, schemas/api/integration-diagnostic-run.schema.json, schemas/api/integration-diagnostic-reason-code.schema.json, schemas/api/integration-diagnostic-list.response.schema.json, schemas/api/create-integration-diagnostic-run.request.schema.json, and daemon/internal/contracts/testdata/integration_diagnostics/

**Checkpoint**: User Story 1 is fully functional and independently testable.

---

## Phase 4: User Story 2 - Remediate User-Facing Failures (Priority: P1)

**Goal**: Product users receive clear remediation owner, next step, and retry-safety guidance for integration failures without raw provider errors or stale diagnostic truth.

**Independent Test**: Trigger representative permission, authorization, provider-outage, rate-limit, and unsafe-retry failures and verify each failure includes current diagnostic truth, stable remediation fields, and no raw provider error text.

### Tests for User Story 2

- [X] T035 [P] [US2] Add calendar failure remediation tests for tenant approval, user authorization, provider outage, and stale cached state in daemon/internal/api/calendar_execution_test.go
- [X] T036 [P] [US2] Add mail, reminders, and delivery remediation projection tests for current diagnostic truth and redaction in daemon/internal/api/mail_test.go, daemon/internal/api/reminders_test.go, and daemon/internal/api/delivery_projection_test.go
- [X] T037 [US2] Add SDK tests for user-facing diagnostic failure projection payloads in sdk/ts/src/index.test.ts

### Implementation for User Story 2

- [X] T038 [US2] Implement remediation hint catalog and rendering rules in daemon/internal/integrations/diagnostics_remediation.go
- [X] T039 [US2] Implement current-diagnostic-truth refresh on integration action failure in daemon/internal/integrations/diagnostics_manager.go
- [X] T040 [US2] Add diagnostic failure projections to calendar operation responses in daemon/internal/calendar/types.go, daemon/internal/calendar/manager.go, daemon/internal/api/calendar_execution.go, and schemas/api/calendar-operation-resource.schema.json
- [X] T041 [US2] Add diagnostic failure projections to mail, reminders, and delivery operation responses in daemon/internal/mail/types.go, daemon/internal/reminders/types.go, daemon/internal/delivery/types.go, daemon/internal/api/mail_execution.go, daemon/internal/api/reminders.go, daemon/internal/api/delivery.go, schemas/api/mail-operation-resource.schema.json, schemas/api/reminder-operation-resource.schema.json, and schemas/api/delivery-operation-resource.schema.json
- [X] T042 [US2] Add retry-safety projection from live-validation ambiguous commit evidence in daemon/internal/livevalidation/ambiguous_commit.go and daemon/internal/integrations/diagnostics_remediation.go
- [X] T043 [US2] Update TypeScript SDK operation resource types for diagnostic failure projections in sdk/ts/src/index.ts
- [X] T044 [US2] Update web client display for user-facing remediation owner, next step, and retry safety in web/src/features/integration-diagnostics.tsx

**Checkpoint**: User Stories 1 and 2 both work independently.

---

## Phase 5: User Story 3 - Classify Provider Failures Consistently (Priority: P2)

**Goal**: Engineers and release reviewers can verify provider-specific failures map to stable reason codes, retry-safety categories, and remediation owners across diagnostics, smoke, audit, SDK, and web fixtures.

**Independent Test**: Replay Feishu/Lark provider error fixtures and verify expected reason code, retry-safety classification, remediation owner, redaction behavior, and consistent projection across all surfaces.

### Tests for User Story 3

- [X] T045 [P] [US3] Add Feishu/Lark reason-code fixture table tests covering all required provider cases in daemon/internal/integrations/diagnostics_classifier_test.go
- [X] T046 [US3] Add ambiguous provider evidence and side-effect retry-safety tests in daemon/internal/integrations/diagnostics_classifier_test.go and daemon/internal/livevalidation/ambiguous_commit_test.go
- [X] T047 [P] [US3] Add reason-code catalog contract tests for API schemas, SDK fixtures, and event fixtures in daemon/internal/contracts/integration_diagnostics_test.go
- [X] T048 [P] [US3] Add fake-backend diagnostic classifier fixtures for every stable reason-code outcome in daemon/internal/integrations/testdata/diagnostics/fake_backend_reason_codes.json
- [X] T049 [US3] Add fake-backend diagnostic coverage tests for every classified outcome in daemon/internal/integrations/fake_backend_test.go and daemon/internal/integrations/diagnostics_classifier_test.go

### Implementation for User Story 3

- [X] T050 [US3] Implement provider classification interfaces and stable reason-code catalog in daemon/internal/integrations/diagnostics_classifier.go
- [X] T051 [US3] Implement Feishu/Lark provider classification adapter using redacted evidence and fixture-backed mappings in daemon/internal/integrations/feishu_lark_diagnostics.go
- [X] T052 [US3] Implement classification persistence and projection linkage in daemon/internal/store/integration_diagnostics.go and daemon/internal/integrations/diagnostics_manager.go
- [X] T053 [US3] Implement redaction-uncertain provider evidence fail-closed classification in daemon/internal/integrations/diagnostics_redaction.go and daemon/internal/integrations/diagnostics_classifier.go
- [X] T054 [US3] Update reason-code catalog schemas, SDK union types, and fixture payloads in schemas/api/integration-diagnostic-reason-code.schema.json, sdk/ts/src/index.ts, and daemon/internal/contracts/testdata/integration_diagnostics/
- [X] T055 [US3] Update provider diagnostic documentation for Feishu/Lark mappings and ambiguous evidence behavior in docs/providers/integration-diagnostics.md

**Checkpoint**: User Story 3 provider classification is independently verifiable.

---

## Phase 6: User Story 4 - Produce Real-Account Smoke Evidence (Priority: P2)

**Goal**: Engineers can run a safe smoke matrix that records pass, fail, blocked, and skipped outcomes with remediation fields, artifact links, retention, and dual approval for risky probes.

**Independent Test**: Run the smoke matrix with fake and configured safe accounts, including unavailable credentials and missing approvals, and verify report fields, skip reasons, redaction, and release-readiness linkage.

### Tests for User Story 4

- [X] T056 [P] [US4] Add smoke report fixture tests for pass, fail, blocked, skipped, limited, and unsupported outcomes in daemon/internal/opsreadiness/real_account_smoke_test.go
- [X] T057 [US4] Add risky-probe dual approval tests for missing tenant-admin approval and missing operator approval in daemon/internal/opsreadiness/real_account_smoke_test.go
- [X] T058 [P] [US4] Add release-readiness tests requiring Roadmap 42 diagnostic and smoke evidence in daemon/internal/opsreadiness/release_readiness_test.go
- [X] T059 [US4] Add smoke report API/schema contract tests in daemon/internal/contracts/integration_diagnostics_test.go
- [X] T060 [US4] Add SC-007 smoke timeout and structured blocked/skipped assertions for the 10-minute approved-path target in daemon/internal/opsreadiness/real_account_smoke_test.go

### Implementation for User Story 4

- [X] T061 [US4] Implement smoke matrix report and probe outcome persistence in daemon/internal/store/integration_diagnostics.go
- [X] T062 [US4] Implement real-account diagnostic smoke matrix builder, safe-probe defaults, skip/block reason mapping, and report publication in daemon/internal/opsreadiness/integration_diagnostic_smoke.go
- [X] T063 [US4] Implement dual approval enforcement for non-idempotent or externally visible diagnostic smoke probes in daemon/internal/opsreadiness/integration_diagnostic_smoke.go and daemon/internal/livevalidation/approval.go
- [X] T064 [US4] Link smoke report outcomes to diagnostic results and provider classifications in daemon/internal/integrations/diagnostics_manager.go and daemon/internal/opsreadiness/integration_diagnostic_smoke.go
- [X] T065 [US4] Add smoke matrix report and probe outcome schemas and fixtures in schemas/api/smoke-matrix-report.schema.json, schemas/api/smoke-probe-outcome.schema.json, and daemon/internal/contracts/testdata/integration_diagnostics/
- [X] T066 [US4] Add Roadmap 42 diagnostic evidence to release-readiness summaries and fixtures in daemon/internal/opsreadiness/release_readiness.go, daemon/internal/opsreadiness/testdata/r42_release_readiness.json, and docs/runtime/integration-diagnostic-readiness.md
- [X] T067 [US4] Update SDK and web surfaces for smoke report links and outcome summaries in sdk/ts/src/index.ts and web/src/features/integration-diagnostics.tsx

**Checkpoint**: User Story 4 smoke evidence is independently verifiable.

---

## Phase 7: User Story 5 - Audit Diagnostic Runs And State Changes (Priority: P3)

**Goal**: Operators and auditors can review diagnostic runs, remediation-relevant state transitions, smoke publication, retention application, and redaction failures for authorized tenants only.

**Independent Test**: Run diagnostics, change authorization state, publish smoke reports, apply retention, and verify scoped audit evidence for authorized users and non-disclosing denials for unauthorized users.

### Tests for User Story 5

- [X] T068 [P] [US5] Add audit event tests for diagnostic run lifecycle, state change, permission denial, redaction failure, smoke publication, skipped and blocked outcomes, and retention application in daemon/internal/audit/integration_diagnostics_test.go
- [X] T069 [US5] Add event bus and schema tests for integration diagnostic events in daemon/internal/events/integration_diagnostics_test.go and daemon/internal/contracts/integration_diagnostics_test.go
- [X] T070 [US5] Add 90-day retention expiry tests for diagnostic runs, result payloads, smoke reports, and normal inspection visibility in daemon/internal/store/integration_diagnostics_test.go

### Implementation for User Story 5

- [X] T071 [US5] Implement diagnostic audit event emission and redacted audit resource projection in daemon/internal/audit/integration_diagnostics.go
- [X] T072 [US5] Implement integration diagnostic event payloads and event bus publishing in daemon/internal/events/integration_diagnostics.go and daemon/internal/integrations/diagnostics_manager.go
- [X] T073 [US5] Implement diagnostic retention application, expiry queries, and minimal audit preservation in daemon/internal/store/integration_diagnostics.go and daemon/internal/integrations/diagnostics_retention.go
- [X] T074 [US5] Implement audit inspection and event schema fixtures for diagnostic evidence in schemas/events/integration-diagnostic-run-started.event.schema.json, schemas/events/integration-diagnostic-run-completed.event.schema.json, schemas/events/integration-diagnostic-state-changed.event.schema.json, schemas/events/integration-diagnostic-redaction-failed.event.schema.json, schemas/events/integration-diagnostic-smoke-completed.event.schema.json, and schemas/events/integration-diagnostic-retention-applied.event.schema.json
- [X] T075 [US5] Update operator documentation for audit, retention, redaction failure, and rollback behavior in docs/harness/integration-diagnostic-smoke.md and docs/runtime/integration-diagnostic-readiness.md

**Checkpoint**: User Story 5 audit and retention behavior is independently verifiable.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final verification, compatibility, documentation, and cleanup across all stories.

- [X] T076 [P] Run targeted daemon package tests from quickstart in daemon/ with go test ./internal/integrations ./internal/api ./internal/store ./internal/store/tenancy ./internal/audit ./internal/events ./internal/opsreadiness ./internal/livevalidation
- [X] T077 [P] Run schema and contract validation with make daemon-contract-test from Makefile
- [X] T078 [P] Run client tests with pnpm test:clients from package.json
- [X] T079 [P] Run client build with pnpm build from package.json
- [X] T080 Run full daemon tests with go test ./... from daemon/
- [X] T081 Run go mod tidy from daemon/
- [X] T082 Validate isolated test daemon health with make daemon-run-test and make daemon-test-status from Makefile
- [X] T083 [P] Update release and operator documentation links for Roadmap 42 in docs/runtime/release-readiness.md and docs/providers/provider-identity-and-profiles.md
- [X] T084 [P] Add final task-completion notes and any residual verification gaps to specs/027-integration-diagnostics/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately.
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories.
- **User Stories (Phase 3+)**: All depend on Foundational phase completion.
- **Polish (Phase 8)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - establishes operator inspection MVP.
- **User Story 2 (P1)**: API, store, and service work can start after Foundational but benefits from US1 diagnostic manager and result projection.
- **User Story 3 (P2)**: Classifier, fixture, and contract work can start after Foundational; should complete before final US1/US2 verification for full Feishu/Lark coverage.
- **User Story 4 (P2)**: Smoke and release-readiness work can start after Foundational and uses US3 reason-code mappings for smoke outcomes.
- **User Story 5 (P3)**: Audit, event, and retention work can start after Foundational; audit hooks integrate with all stories and final verification depends on it.
- Shared edits to sdk/ts/src/index.ts and web/src/features/integration-diagnostics.tsx must be serialized in US1 -> US2 -> US3 -> US4 order unless the implementer first splits story-specific modules with explicit ownership boundaries.

### Within Each User Story

- Tests are listed before implementation and should fail before behavior is implemented.
- Store/domain models before services/managers.
- Services/managers before API/SDK/web projections.
- Contract/schema fixtures before cross-surface validation.
- Story checkpoint must pass before relying on that story in later phases.
- Shared SDK and web files are not parallel-safe across stories; serialize those edits or split them into story-owned files before parallel implementation.

### Parallel Opportunities

- Setup tasks T002-T006 can run in parallel after T001 creates schema placeholders.
- Foundational tasks T010, T013, and T016 can run in parallel with store and API skeleton work.
- US1 tests T021-T024 can run in parallel; T025 is serialized with T022 unless those assertions move to a separate file.
- US2 tests T035-T036 can run in parallel; T037 is serialized with earlier SDK index.test edits unless SDK tests are split into separate files.
- US3 tests T045, T047, and T048 can run in parallel; T046 and T049 are serialized with T045 unless classifier tests are split into separate files.
- US4 tests T056 and T058 can run in parallel; T057 and T060 are serialized with T056 unless smoke tests are split into separate files, and T059 is serialized with contract-test ownership unless that file is split.
- US5 test T068 can run in parallel with non-overlapping files; T069 and T070 are serialized with contract/store test ownership unless those files are split.
- Final verification T076-T079 and docs task T083 can run in parallel after implementation stabilizes.
- Do not parallelize tasks that edit the shared sdk/ts/src/index.ts or web/src/features/integration-diagnostics.tsx files across different stories unless those files are split into story-owned modules first.

---

## Parallel Example: User Story 1

```bash
Task: "T021 [P] [US1] Add integration diagnostic result store tests in daemon/internal/store/integration_diagnostics_test.go"
Task: "T022 [P] [US1] Add tenant access and non-disclosure tests in daemon/internal/api/integration_diagnostics_test.go"
Task: "T023 [P] [US1] Add SDK tests in sdk/ts/src/index.test.ts"
Task: "T024 [P] [US1] Add web operator-shell fixture tests in web/src/features/integration-diagnostics.test.tsx"
```

## Parallel Example: User Story 3

```bash
Task: "T045 [P] [US3] Add Feishu/Lark reason-code fixture table tests in daemon/internal/integrations/diagnostics_classifier_test.go"
Task: "T047 [P] [US3] Add reason-code catalog contract tests in daemon/internal/contracts/integration_diagnostics_test.go"
Task: "T048 [P] [US3] Add fake-backend diagnostic classifier fixtures in daemon/internal/integrations/testdata/diagnostics/fake_backend_reason_codes.json"
```

## Parallel Example: User Story 4

```bash
Task: "T056 [P] [US4] Add smoke report fixture tests in daemon/internal/opsreadiness/real_account_smoke_test.go"
Task: "T058 [P] [US4] Add release-readiness tests in daemon/internal/opsreadiness/release_readiness_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Validate US1 with store, API, SDK, and web tests for diagnostic inspection.
5. Stop before claiming Roadmap 42 completion; MVP does not close the whole roadmap.

### Incremental Delivery

1. Setup + Foundational establishes durable diagnostic contracts and storage.
2. US1 adds operator inspection and freshness behavior.
3. US2 adds user-facing remediation and current diagnostic truth.
4. US3 adds full Feishu/Lark provider classification and stable reason-code consistency.
5. US4 adds real-account smoke evidence and release-readiness linkage.
6. US5 completes audit, retention, and operational traceability.
7. Polish runs full verification and documentation updates.

### Parallel Team Strategy

1. One engineer owns store/domain foundations.
2. One engineer owns API/schema contracts.
3. One engineer owns provider classification fixtures.
4. One engineer owns ops-readiness smoke and release-readiness linkage.
5. One engineer owns audit/events/retention.
6. One engineer serializes shared SDK/web file edits across story checkpoints, or splits those files into story-owned modules before parallel work.
7. Integrate at story checkpoints, not only at the final phase.

## Notes

- [P] tasks = different files, no dependency on incomplete non-foundational tasks.
- [US] labels map tasks to specific user stories for traceability.
- Every user story includes verification tasks because this is production control-plane work with API, schema, event, persistence, tenant, and secret-handling surfaces.
- Default validation uses `~/.kura-test`; live connectors and real-account smoke require explicit safe credentials and approvals.
- Roadmap 42 is incomplete until all five user stories and final verification tasks are complete.
