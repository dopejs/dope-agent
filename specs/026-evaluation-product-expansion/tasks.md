# Tasks: Evaluation Product Expansion

**Input**: Design documents from `specs/026-evaluation-product-expansion/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the constitution and by this feature's compatibility, persistence, API, schema, SDK, web, redaction, tenant-isolation, and release-readiness changes. Contract, unit, integration, query-plan, SDK/client, web, and operator-evidence validation tasks are included before implementation tasks in each user story.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently after the shared foundation is complete.

## Phase 1: Setup

**Purpose**: Establish Roadmap 41 implementation locations and planning traceability without changing runtime behavior.

- [X] T001 Create the Roadmap 41 evaluation product documentation stub in `docs/harness/evaluation-product.md`
- [X] T002 [P] Create the Roadmap 41 operations and rollback documentation stub in `docs/runtime/evaluation-product-operations.md`
- [X] T003 [P] Create evaluation product contract-test fixture notes in `daemon/internal/contracts/testdata/evaluation_product/README.md`
- [X] T004 [P] Add Roadmap 41 migration fixture seed notes in `daemon/internal/store/migrationfixture/r41_evaluation_product.go`
- [X] T005 [P] Create the evaluation product package documentation in `daemon/internal/evaluation/product_doc.go`
- [X] T006 [P] Add web evaluation product test fixture notes in `web/src/app/evaluation-product.fixtures.test.ts`
- [X] T007 [P] Add SDK evaluation product test fixture helpers in `sdk/ts/src/evaluation-product.fixtures.test.ts`

---

## Phase 2: Foundational

**Purpose**: Shared types, redaction, persistence, permission, schema, event, and contract scaffolding that block all user stories.

**Critical**: No user story work should begin until this phase is complete.

- [X] T008 [P] Define discovery policy lifecycle, discovery, suppression, product fixture, campaign, dashboard, inspection, retention, and pagination types in `daemon/internal/evaluation/product_types.go`
- [X] T009 [P] Define evaluation product errors, validation helpers, and stable reason codes in `daemon/internal/evaluation/product_validation.go`
- [X] T010 [P] Define evaluation product redaction policy types and redacted evidence helpers in `daemon/internal/evaluation/product_redaction.go`
- [X] T011 [P] Define evaluation product store interfaces, discovery policy filters, retention/deletion filters, and list filters in `daemon/internal/evaluation/product_store.go`
- [X] T012 [P] Add evaluation product permission constants and role mapping tests in `daemon/internal/identity/permissions_test.go`
- [X] T013 Add evaluation product permission constants to tenant permission resources in `daemon/internal/identity/types.go`
- [X] T014 Add SQLite tables and indexes for discovery policies, discovery, evidence, suppressions, product fixtures, fixture revisions, campaigns, campaign items, attempt groups, dashboard projections, tool-call inspections, and cross-resource retention/deletion metadata in `daemon/internal/store/store.go`
- [X] T015 [P] Add migration fixture seeds for Roadmap 41 tables and representative rows in `daemon/internal/store/migrationfixture/r41_evaluation_product.go`
- [X] T016 [P] Add schema migration tests for Roadmap 41 tables, indexes, and tenant columns in `daemon/internal/store/evaluation_product_schema_test.go`
- [X] T017 [P] Add query-plan tests for tenant-scoped discovery, fixture, campaign, dashboard, and inspection lists in `daemon/internal/store/queryplan_test.go`
- [X] T018 Add store persistence methods and cross-resource retention/deletion application methods for Roadmap 41 product resources in `daemon/internal/store/evaluation_product.go`
- [X] T019 [P] Add tenant-safe accessors for Roadmap 41 product resources in `daemon/internal/store/tenancy/evaluation_product.go`
- [X] T020 [P] Add store round-trip and cross-resource retention/deletion enforcement tests for Roadmap 41 product resources in `daemon/internal/store/evaluation_product_test.go`
- [X] T021 [P] Add tenant isolation tests for Roadmap 41 product resources in `daemon/internal/store/tenancy/evaluation_product_test.go`
- [X] T022 [P] Add shared API schemas for cursor pagination, suppression records, retention state, retention/deletion application outcomes, and redaction metadata in `schemas/api/evaluation-product-pagination.schema.json`
- [X] T023 [P] Add shared event schemas for evaluation product audit outcomes, cross-resource redaction failures, and retention/deletion applications in `schemas/events/evaluation-product-audit-recorded.event.schema.json`
- [X] T024 [P] Add API contract tests that require all Roadmap 41 schema files, discovery policy routes, and retention/deletion route fixtures to be referenced from route fixtures in `daemon/internal/contracts/evaluation_product_contracts_test.go`
- [X] T025 Add API route scaffold and tenant guards for Roadmap 41 product routes, discovery policy routes, and retention/deletion actions in `daemon/internal/api/evaluation_product.go`
- [X] T026 Add API server wiring for Roadmap 41 product routes without enabling product mutations in `daemon/internal/api/server.go`
- [X] T027 [P] Add audit helpers for Roadmap 41 evaluation product actions, retention/deletion applications, and cross-resource redaction failures in `daemon/internal/audit/evaluation_product.go`
- [X] T028 [P] Add event helpers for Roadmap 41 evaluation product events, retention/deletion applications, and cross-resource redaction failures in `daemon/internal/events/evaluation_product.go`
- [X] T029 [P] Add SDK base types for Roadmap 41 resources, discovery policies, cursors, denials, retention/deletion outcomes, and tenant options in `sdk/ts/src/index.ts`

**Checkpoint**: Foundation ready. User story implementation can now begin with shared types, storage, permissions, redaction, schema scaffolding, and route wiring in place.

---

## Phase 3: User Story 1 - Discover Replay Candidates (Priority: P1) MVP

**Goal**: Operators can review tenant-scoped replay candidates automatically suggested from historical runs and workflows, with explanations, provenance, scan bounds, redaction, and suppression controls.

**Independent Test**: Seed representative historical runs for multiple tenants, start bounded discovery, and verify that only eligible candidates for the selected tenant appear with explanation, score, source provenance, redacted evidence, and suppression behavior.

### Tests for User Story 1

- [X] T030 [P] [US1] Add discovery redaction tests for secrets, credentials, raw tokens, nested sensitive fields, configured sensitive fields, and failed-closed redaction audit behavior in `daemon/internal/evaluation/product_redaction_test.go`
- [X] T031 [P] [US1] Add discovery policy validation, bounds, cursor, partial status, and idempotency tests in `daemon/internal/evaluation/discovery_test.go`
- [X] T032 [P] [US1] Add discovery source access tests for runs, workflows, tool calls, replay evidence, and live-validation ledger links in `daemon/internal/evaluation/discovery_sources_test.go`
- [X] T033 [P] [US1] Add suppression matching tests for runs, workflows, discovered candidates, product fixtures, repo fixtures, and source families in `daemon/internal/evaluation/suppression_test.go`
- [X] T034 [P] [US1] Add store tests for discovery runs, discovered candidates, candidate evidence, and suppression records in `daemon/internal/store/evaluation_product_discovery_test.go`
- [X] T035 [P] [US1] Add API route tests for discovery policy create/update/list/detail, discovery run start/list/detail, candidate list/detail, and suppression creation in `daemon/internal/api/evaluation_product_discovery_test.go`
- [X] T036 [P] [US1] Add contract tests for discovery policy, candidate discovery resources, events, pagination, and stable denial shapes in `daemon/internal/contracts/evaluation_product_discovery_contract_test.go`
- [X] T037 [P] [US1] Add SDK tests for discovery policy, discovery run, discovered candidate, and suppression methods in `sdk/ts/src/index.test.ts`
- [X] T038 [P] [US1] Add web tests for discovery policy management, candidate discovery review, empty state, partial bounds, redaction indicators, suppression controls, and the SC-001 two-minute review smoke path in `web/src/app/App.test.tsx`

### Implementation for User Story 1

- [X] T039 [P] [US1] Add discovery policy lifecycle and discovery run domain logic in `daemon/internal/evaluation/discovery.go`
- [X] T040 [P] [US1] Add tenant-safe historical source readers for runs, workflows, tool calls, replay evidence, and live-validation ledger evidence in `daemon/internal/evaluation/discovery_sources.go`
- [X] T041 [P] [US1] Add candidate scoring and explanation generation in `daemon/internal/evaluation/discovery_scoring.go`
- [X] T042 [US1] Add redaction-before-persist enforcement and failed-closed redaction audit handling for discovery evidence in `daemon/internal/evaluation/product_redaction.go`
- [X] T043 [US1] Add suppression creation, lookup, revocation, and selection filtering in `daemon/internal/evaluation/suppression.go`
- [X] T044 [US1] Add discovery policy, discovery run, and candidate persistence methods to `daemon/internal/store/evaluation_product.go`
- [X] T045 [US1] Add tenant-bound discovery policy and discovery accessors to `daemon/internal/store/tenancy/evaluation_product.go`
- [X] T046 [US1] Add discovery policy, discovery run, discovered candidate, candidate evidence, and suppression API schemas in `schemas/api/evaluation-discovery-policy-resource.schema.json` and `schemas/api/evaluation-discovery-run-resource.schema.json`
- [X] T047 [US1] Add discovered candidate list/detail and suppression API schemas in `schemas/api/evaluation-discovered-candidate-resource.schema.json`
- [X] T048 [US1] Add discovery policy, discovery, suppression, discovery redaction-failed, and discovery retention-applied event schemas in `schemas/events/evaluation-discovery-started.event.schema.json`
- [X] T049 [US1] Implement discovery policy, discovery, and suppression API handlers in `daemon/internal/api/evaluation_product.go`
- [X] T050 [US1] Emit discovery policy changed, discovery started/completed/partial/failed/candidate-suggested/suppressed/redaction-failed, and discovery retention-applied audit events in `daemon/internal/audit/evaluation_product.go`
- [X] T051 [US1] Add SDK discovery policy, discovery, and suppression resource types and client methods in `sdk/ts/src/index.ts`
- [X] T052 [US1] Add web discovery policy controls, discovery review, bounds, redaction, and suppression UI in `web/src/app/App.tsx`
- [X] T053 [US1] Add discovery UI styling and responsive states in `web/src/styles.css`
- [X] T054 [US1] Document candidate discovery policy operation, redaction, suppression, retention/deletion behavior, and rollback in `docs/harness/evaluation-product.md`

**Checkpoint**: User Story 1 is complete when bounded tenant discovery is explainable, redacted before persistence/display, suppressible, contract-tested, SDK-visible, and usable from the web shell without unbounded page-load scans.

---

## Phase 4: User Story 2 - Edit Product Fixtures With Provenance (Priority: P1)

**Goal**: Engineers and authorized operators can create, edit, review, and inspect product-managed fixtures with immutable revisions and provenance, without silently changing repo-managed fixtures.

**Independent Test**: Convert an eligible discovered candidate into a product-managed fixture, edit it with an authorized role, attempt the same actions with unauthorized roles, and verify revision history, provenance, repo fixture immutability, and audit evidence.

### Tests for User Story 2

- [X] T055 [P] [US2] Add product fixture lifecycle tests for create, revise, review, suppress, archive, retention expiry, and delete states in `daemon/internal/evaluation/product_fixture_test.go`
- [X] T056 [P] [US2] Add product fixture provenance and immutable revision tests in `daemon/internal/evaluation/product_fixture_revision_test.go`
- [X] T057 [P] [US2] Add repo-managed fixture immutability tests for product editing attempts in `daemon/internal/evaluation/fixtures_test.go`
- [X] T058 [P] [US2] Add permission denial tests for fixture create, edit, review, and suppress actions in `daemon/internal/api/evaluation_product_fixture_test.go`
- [X] T059 [P] [US2] Add store tests for product fixtures, fixture revisions, and retention/deletion effects on fixture selectability in `daemon/internal/store/evaluation_product_fixture_test.go`
- [X] T060 [P] [US2] Add contract tests for product fixture resources, revision resources, review responses, redaction-failed events, and audit events in `daemon/internal/contracts/evaluation_product_fixture_contract_test.go`
- [X] T061 [P] [US2] Add SDK tests for product fixture create/list/detail/revision/review/suppress methods in `sdk/ts/src/index.test.ts`
- [X] T062 [P] [US2] Add web tests for product fixture creation, editing, review state, revision history, and repo fixture read-only behavior in `web/src/app/App.test.tsx`

### Implementation for User Story 2

- [X] T063 [P] [US2] Add product fixture creation, revision, review, retention/deletion, and eligibility domain logic in `daemon/internal/evaluation/product_fixture.go`
- [X] T064 [P] [US2] Add fixture payload validation, redacted evidence materialization, and failed-closed redaction handling in `daemon/internal/evaluation/product_fixture_validation.go`
- [X] T065 [US2] Add product fixture, revision, and fixture retention/deletion persistence methods to `daemon/internal/store/evaluation_product.go`
- [X] T066 [US2] Add tenant-bound product fixture accessors to `daemon/internal/store/tenancy/evaluation_product.go`
- [X] T067 [US2] Add product fixture, fixture revision, materialization, revision creation, and review API schemas in `schemas/api/evaluation-product-fixture-resource.schema.json`
- [X] T068 [US2] Add product fixture lifecycle and redaction-failed event schemas in `schemas/events/evaluation-fixture-created.event.schema.json`
- [X] T069 [US2] Implement product fixture API handlers in `daemon/internal/api/evaluation_product.go`
- [X] T070 [US2] Add fixture permission checks and stable denial responses to `daemon/internal/api/evaluation_product.go`
- [X] T071 [US2] Emit product fixture created/revision-created/reviewed/suppressed/archived/deleted/redaction-failed audit events in `daemon/internal/audit/evaluation_product.go`
- [X] T072 [US2] Add SDK product fixture and revision resource types and client methods in `sdk/ts/src/index.ts`
- [X] T073 [US2] Add web product fixture editor, revision inspector, review controls, and repo fixture read-only indicators in `web/src/app/App.tsx`
- [X] T074 [US2] Add product fixture editor styling and validation states in `web/src/styles.css`
- [X] T075 [US2] Document product fixture editing, revision rollback, review, retention/deletion behavior, and repo fixture immutability in `docs/harness/evaluation-product.md`

**Checkpoint**: User Story 2 is complete when product fixtures are permission-gated, revisioned, auditable, SDK/web-visible, and unable to mutate repo-managed fixtures.

---

## Phase 5: User Story 3 - Run Replay Campaigns (Priority: P2)

**Goal**: Tenant admins can group eligible candidates and fixtures into replay campaigns, track lifecycle, and inspect grouped attempts, comparisons, live-validation outcomes, and operator-action-needed summaries.

**Independent Test**: Create a campaign from approved fixtures and candidates, run it through completion, and verify immutable source snapshots, grouped attempts, comparison summaries, live-validation linkage, lifecycle state, suppression checks, and audit evidence.

### Tests for User Story 3

- [X] T076 [P] [US3] Add campaign lifecycle tests for draft, queued, running, completed, published, failed, cancelled, retention-expired, and result-deleted states in `daemon/internal/evaluation/campaign_test.go`
- [X] T077 [P] [US3] Add campaign source snapshot and post-edit source stability tests in `daemon/internal/evaluation/campaign_snapshot_test.go`
- [X] T078 [P] [US3] Add campaign selection rejection tests for suppressed, expired, deleted, draft, and cross-tenant sources in `daemon/internal/evaluation/campaign_selection_test.go`
- [X] T079 [P] [US3] Add campaign aggregation tests for replay attempts, comparisons, live-validation links, drift, failures, unsupported replay, redaction-failed result publication, and operator-action-needed counts in `daemon/internal/evaluation/campaign_aggregation_test.go`
- [X] T080 [P] [US3] Add store tests for campaigns, campaign items, attempt groups, and retention/deletion effects on campaign results in `daemon/internal/store/evaluation_product_campaign_test.go`
- [X] T081 [P] [US3] Add API route tests for campaign create/start/cancel/publish/detail/items/attempt-groups in `daemon/internal/api/evaluation_product_campaign_test.go`
- [X] T082 [P] [US3] Add contract tests for campaign resources, dashboard linkage fields, idempotency, and event schemas in `daemon/internal/contracts/evaluation_product_campaign_contract_test.go`
- [X] T083 [P] [US3] Add SDK tests for campaign create/list/detail/start/cancel/publish/items/attempt-groups methods in `sdk/ts/src/index.test.ts`
- [X] T084 [P] [US3] Add web tests for campaign creation, lifecycle, source snapshots, aggregate results, and live-validation linkage in `web/src/app/App.test.tsx`

### Implementation for User Story 3

- [X] T085 [P] [US3] Add campaign lifecycle, idempotency, source selection, and snapshot domain logic in `daemon/internal/evaluation/campaign.go`
- [X] T086 [P] [US3] Add campaign attempt group aggregation over replay attempts, comparisons, and live-validation ledger links in `daemon/internal/evaluation/campaign_aggregation.go`
- [X] T087 [US3] Add campaign, item, attempt group, and campaign result retention/deletion persistence methods to `daemon/internal/store/evaluation_product.go`
- [X] T088 [US3] Add tenant-bound campaign accessors to `daemon/internal/store/tenancy/evaluation_product.go`
- [X] T089 [US3] Add campaign, campaign item, campaign attempt group, and campaign result API schemas in `schemas/api/evaluation-campaign-resource.schema.json`
- [X] T090 [US3] Add campaign lifecycle and redaction-failed event schemas in `schemas/events/evaluation-campaign-created.event.schema.json`
- [X] T091 [US3] Implement campaign API handlers in `daemon/internal/api/evaluation_product.go`
- [X] T092 [US3] Emit campaign created/started/cancelled/completed/failed/results-published/redaction-failed audit events in `daemon/internal/audit/evaluation_product.go`
- [X] T093 [US3] Add campaign worker orchestration hooks that launch non-live replay attempts and record live-validation links in `daemon/internal/evaluation/campaign_runner.go`
- [X] T094 [US3] Add SDK campaign resource types and client methods in `sdk/ts/src/index.ts`
- [X] T095 [US3] Add web campaign creation, lifecycle, item, attempt group, and aggregate result views in `web/src/app/App.tsx`
- [X] T096 [US3] Add campaign UI styling and result status states in `web/src/styles.css`
- [X] T097 [US3] Document campaign lifecycle, immutable source snapshots, live-validation linkage, campaign retention/deletion behavior, and rollback in `docs/harness/evaluation-product.md`

**Checkpoint**: User Story 3 is complete when campaigns can be started and reviewed independently with immutable sources, grouped replay/live-validation evidence, stable contracts, and tenant-scoped audit trails.

---

## Phase 6: User Story 4 - Monitor Evaluation Dashboards (Priority: P3)

**Goal**: Operators and product users can view tenant-scoped dashboard summaries for drift, failures, unsupported replay, live-validation linkage, operator-action-needed states, campaign trends, pagination, and release-readiness evidence.

**Independent Test**: Load dashboard projections for tenants with known campaign and replay outcomes, verify scoped aggregate values, deterministic pagination, no cross-tenant records, trend summaries, and release-readiness evidence links.

### Tests for User Story 4

- [X] T098 [P] [US4] Add dashboard projection aggregation tests for campaign status, candidate, fixture, drift, failure, unsupported, live-validation, operator-action-needed, and retention/deletion summaries in `daemon/internal/evaluation/dashboard_test.go`
- [X] T099 [P] [US4] Add dashboard deterministic cursor pagination tests in `daemon/internal/evaluation/dashboard_pagination_test.go`
- [X] T100 [P] [US4] Add store tests for dashboard projection persistence and retention-aware filtering in `daemon/internal/store/evaluation_product_dashboard_test.go`
- [X] T101 [P] [US4] Add API route tests for dashboard summary, filters, paging, and read-only permission behavior in `daemon/internal/api/evaluation_product_dashboard_test.go`
- [X] T102 [P] [US4] Add contract tests for dashboard projection resource and pagination schema compatibility in `daemon/internal/contracts/evaluation_product_dashboard_contract_test.go`
- [X] T103 [P] [US4] Add SDK tests for dashboard projection methods and cursor paging in `sdk/ts/src/index.test.ts`
- [X] T104 [P] [US4] Add web tests for dashboard aggregate cards, trend summaries, paging, empty state, retention/deletion filtering, and release-readiness links in `web/src/app/App.test.tsx`

### Implementation for User Story 4

- [X] T105 [P] [US4] Add dashboard projection domain logic over campaigns, candidates, fixtures, retention/deletion states, comparisons, and live-validation ledger links in `daemon/internal/evaluation/dashboard.go`
- [X] T106 [US4] Add dashboard projection persistence and retention-aware read methods to `daemon/internal/store/evaluation_product.go`
- [X] T107 [US4] Add tenant-bound dashboard projection accessors to `daemon/internal/store/tenancy/evaluation_product.go`
- [X] T108 [US4] Add dashboard projection and dashboard detail API schemas in `schemas/api/evaluation-dashboard-resource.schema.json`
- [X] T109 [US4] Add dashboard projection generated event schema in `schemas/events/evaluation-dashboard-projection-generated.event.schema.json`
- [X] T110 [US4] Implement dashboard API handlers and filter parsing in `daemon/internal/api/evaluation_product.go`
- [X] T111 [US4] Emit dashboard projection generated audit/event records in `daemon/internal/audit/evaluation_product.go`
- [X] T112 [US4] Add SDK dashboard resource types and client methods in `sdk/ts/src/index.ts`
- [X] T113 [US4] Add web dashboard aggregates, trends, paging, and release-readiness evidence links in `web/src/app/App.tsx`
- [X] T114 [US4] Add dashboard UI styling and pagination states in `web/src/styles.css`
- [X] T115 [US4] Document dashboard interpretation, retention/deletion behavior, and Roadmap 39 rerun evidence in `docs/runtime/release-readiness.md`

**Checkpoint**: User Story 4 is complete when dashboard reads are tenant-scoped, bounded to product projections, deterministic under pagination, and sufficient for release-readiness review.

---

## Phase 7: User Story 5 - Inspect Tool-Call Replay Evidence (Priority: P3)

**Goal**: Engineers can compare original behavior, non-live replay behavior, and live-validation evidence for tool calls with explicit unsupported, missing, expired, denied, aborted, failed, operator-action-needed, and completed states.

**Independent Test**: Run replay/campaign cases with original, non-live, unsupported, missing, and live-validation evidence, then verify inspection views show aligned redacted evidence and preserve links to runtime truth and Roadmap 40 ledger records.

### Tests for User Story 5

- [X] T116 [P] [US5] Add tool-call inspection classification tests for matched, drifted, failed, unsupported, missing, denied, aborted, failed live-validation, operator-action-needed, and completed states in `daemon/internal/evaluation/tool_call_inspection_test.go`
- [X] T117 [P] [US5] Add tool-call inspection redaction tests for original, replay, live-validation, and diff payloads in `daemon/internal/evaluation/tool_call_inspection_redaction_test.go`
- [X] T118 [P] [US5] Add store tests for tool-call inspection persistence, retention expiry, and evidence link stability in `daemon/internal/store/evaluation_product_inspection_test.go`
- [X] T119 [P] [US5] Add API route tests for campaign inspection list and inspection detail routes in `daemon/internal/api/evaluation_product_inspection_test.go`
- [X] T120 [P] [US5] Add contract tests for tool-call inspection resources, classification enum, and redaction-failed events in `daemon/internal/contracts/evaluation_product_inspection_contract_test.go`
- [X] T121 [P] [US5] Add SDK tests for tool-call inspection list/detail methods and classification mapping in `sdk/ts/src/index.test.ts`
- [X] T122 [P] [US5] Add web tests for tool-call inspection diff display, unsupported state, missing-evidence state, live-validation ledger links, and redaction indicators in `web/src/app/App.test.tsx`

### Implementation for User Story 5

- [X] T123 [P] [US5] Add tool-call inspection domain logic and evidence alignment in `daemon/internal/evaluation/tool_call_inspection.go`
- [X] T124 [P] [US5] Add tool-call inspection redacted diff generation in `daemon/internal/evaluation/tool_call_inspection_diff.go`
- [X] T125 [US5] Add tool-call inspection persistence and retention methods to `daemon/internal/store/evaluation_product.go`
- [X] T126 [US5] Add tenant-bound tool-call inspection accessors to `daemon/internal/store/tenancy/evaluation_product.go`
- [X] T127 [US5] Add tool-call inspection API schemas in `schemas/api/evaluation-tool-call-inspection-resource.schema.json`
- [X] T128 [US5] Add tool-call inspection event schemas in `schemas/events/evaluation-tool-call-inspection-generated.event.schema.json`
- [X] T129 [US5] Implement tool-call inspection API handlers in `daemon/internal/api/evaluation_product.go`
- [X] T130 [US5] Emit tool-call inspection generated, redaction-failed, and retention-applied audit events in `daemon/internal/audit/evaluation_product.go`
- [X] T131 [US5] Add SDK tool-call inspection resource types and client methods in `sdk/ts/src/index.ts`
- [X] T132 [US5] Add web tool-call inspection diff, classification, and ledger-link views in `web/src/app/App.tsx`
- [X] T133 [US5] Add tool-call inspection UI styling and evidence-state indicators in `web/src/styles.css`
- [X] T134 [US5] Document tool-call inspection evidence rules and Roadmap 40 ledger linkage in `docs/harness/evaluation-product.md`

**Checkpoint**: User Story 5 is complete when inspection records are redacted, tenant-scoped, linked to replay/live-validation evidence, and usable for debugging without replacing runtime truth.

---

## Phase 8: Polish & Cross-Cutting Verification

**Purpose**: Final documentation, contract reconciliation, release-readiness updates, and repository-level verification.

- [X] T135 [P] Reconcile final discovery policy route, schema, SDK, and event names in `specs/026-evaluation-product-expansion/contracts/candidate-discovery.md`
- [X] T136 [P] Reconcile final product fixture states, permissions, and revision rules in `specs/026-evaluation-product-expansion/contracts/fixture-editing.md`
- [X] T137 [P] Reconcile final campaign, dashboard, pagination, and live-validation linkage contracts in `specs/026-evaluation-product-expansion/contracts/campaign-dashboard.md`
- [X] T138 [P] Reconcile final tool-call inspection classifications and evidence rules in `specs/026-evaluation-product-expansion/contracts/tool-call-inspection.md`
- [X] T139 [P] Update final data model fields, state transitions, and cross-resource retention/deletion notes in `specs/026-evaluation-product-expansion/data-model.md`
- [X] T140 [P] Update evaluation product workflow and operator docs in `docs/harness/evaluation-product.md`
- [X] T141 [P] Update evaluation product rollback and cross-resource retention/deletion operations in `docs/runtime/evaluation-product-operations.md`
- [X] T142 [P] Update Roadmap 41 status and artifact links in `docs/runtime/daemon-roadmaps.md`
- [X] T143 [P] Update upstream Roadmap 41 spec status and artifact links in `docs/specs/026-evaluation-product-expansion.md`
- [X] T144 [P] Update release-readiness guidance for Roadmap 39 rerun with Roadmaps 40 and 41 included in `docs/runtime/release-readiness.md`
- [X] T145 [P] Run targeted Go tests for evaluation, store, tenancy, API, audit, events, identity, and contracts; record results in `specs/026-evaluation-product-expansion/quickstart.md`
- [X] T146 Run `go test ./...` from `daemon/` and record results in `specs/026-evaluation-product-expansion/quickstart.md`
- [X] T147 Run `go mod tidy` from `daemon/` and record module diff status in `specs/026-evaluation-product-expansion/quickstart.md`
- [X] T148 Run `make daemon-contract-test` from the repository root and record results in `specs/026-evaluation-product-expansion/quickstart.md`
- [X] T149 Run `pnpm test:clients` and record results in `specs/026-evaluation-product-expansion/quickstart.md`
- [X] T150 Run `pnpm build` and record results in `specs/026-evaluation-product-expansion/quickstart.md`
- [X] T151 Run the test daemon smoke with `make daemon-run-test` and `make daemon-test-status`, then record health and shutdown evidence in `specs/026-evaluation-product-expansion/quickstart.md`
- [X] T152 Run the Roadmap 41 product smoke flow for discovery, suppression, product fixture editing, campaign start, dashboard projection, tool-call inspection, SC-001 two-minute candidate review, and SC-005 five-minute fixture create/edit; record timed results in `specs/026-evaluation-product-expansion/quickstart.md`
- [ ] T153 Run the Roadmap 39 soak rerun with Roadmap 40 live validation and Roadmap 41 evaluation product workflows included; record pass/fail evidence in `specs/026-evaluation-product-expansion/quickstart.md` and leave Roadmap 41 incomplete if the rerun is blocked
  - Blocked 2026-04-30 Asia/Shanghai: targeted-validation rerun passed, but the required 24-hour rerun was not completed in this implementation window and must not be run as release evidence on a movable developer laptop. Owner: release owner. Unblock path: seed the combined Roadmap 40/41 workload on a stable always-on test host and run `scripts/production/run-soak.sh` with `DOPE_SOAK_DURATION=24h`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies.
- **Foundational (Phase 2)**: Depends on Phase 1 and blocks all user stories.
- **User Stories (Phases 3-7)**: Depend on Phase 2. User Stories 1 and 2 are both P1 and close the everyday discovery-to-fixture workflow. User Stories 3, 4, and 5 add campaign, dashboard, and inspection product completeness.
- **Polish (Phase 8)**: Depends on all user stories selected for implementation; full Roadmap 41 closure requires all user stories.

### User Story Dependencies

- **US1 Discover Replay Candidates**: Can start after foundation. This is the MVP checkpoint for bounded, redacted, tenant-scoped candidate discovery.
- **US2 Edit Product Fixtures With Provenance**: Can start after foundation using seeded candidate evidence, but full product flow benefits from US1 materialization.
- **US3 Run Replay Campaigns**: Can start after foundation, but complete campaign selection depends on US1 discovered candidates and US2 product fixture eligibility.
- **US4 Monitor Evaluation Dashboards**: Can start after foundation with seeded campaigns, but complete dashboard coverage depends on US3 campaign aggregates.
- **US5 Inspect Tool-Call Replay Evidence**: Can start after foundation with seeded replay/live-validation evidence, but full product navigation depends on US3 campaign items.

### Within Each User Story

- Tests must be written and fail before implementation when the change introduces verifiable behavior.
- Domain types and validation before persistence.
- Persistence before API handlers.
- API schemas and contract tests before SDK/web client integration.
- SDK methods before web UI integration.
- Documentation and smoke evidence at the story checkpoint.

---

## Parallel Opportunities

- Setup tasks T002-T007 can run in parallel after T001 ownership is clear.
- Foundational tasks T008-T012, T015-T017, T022-T024, and T027-T029 can run in parallel by file ownership.
- US1 tests T030-T038 can run in parallel; implementation tasks T039-T041 can run in parallel before T042-T054.
- US2 tests T055-T062 can run in parallel; implementation tasks T063-T064 can run in parallel before persistence/API/client work.
- US3 tests T076-T084 can run in parallel; implementation tasks T085-T086 can run in parallel before persistence/API/client work.
- US4 tests T098-T104 can run in parallel; implementation tasks T105 and T106 can proceed in parallel before API/client work.
- US5 tests T116-T122 can run in parallel; implementation tasks T123-T124 can proceed in parallel before persistence/API/client work.
- Polish documentation tasks T135-T144 can run in parallel with final verification tasks once implementation is complete.

## Parallel Example: User Story 1

```bash
Task: "T030 [US1] Add discovery redaction tests in daemon/internal/evaluation/product_redaction_test.go"
Task: "T031 [US1] Add discovery bounds tests in daemon/internal/evaluation/discovery_test.go"
Task: "T035 [US1] Add API route tests in daemon/internal/api/evaluation_product_discovery_test.go"
Task: "T037 [US1] Add SDK discovery tests in sdk/ts/src/index.test.ts"
Task: "T038 [US1] Add web discovery review tests in web/src/app/App.test.tsx"
```

## Parallel Example: User Story 2

```bash
Task: "T055 [US2] Add product fixture lifecycle tests in daemon/internal/evaluation/product_fixture_test.go"
Task: "T056 [US2] Add immutable revision tests in daemon/internal/evaluation/product_fixture_revision_test.go"
Task: "T060 [US2] Add contract tests in daemon/internal/contracts/evaluation_product_fixture_contract_test.go"
Task: "T062 [US2] Add web fixture editing tests in web/src/app/App.test.tsx"
```

## Parallel Example: User Story 3

```bash
Task: "T076 [US3] Add campaign lifecycle tests in daemon/internal/evaluation/campaign_test.go"
Task: "T079 [US3] Add campaign aggregation tests in daemon/internal/evaluation/campaign_aggregation_test.go"
Task: "T081 [US3] Add API route tests in daemon/internal/api/evaluation_product_campaign_test.go"
Task: "T084 [US3] Add web campaign tests in web/src/app/App.test.tsx"
```

## Parallel Example: User Story 4

```bash
Task: "T098 [US4] Add dashboard aggregation tests in daemon/internal/evaluation/dashboard_test.go"
Task: "T099 [US4] Add dashboard pagination tests in daemon/internal/evaluation/dashboard_pagination_test.go"
Task: "T102 [US4] Add dashboard contract tests in daemon/internal/contracts/evaluation_product_dashboard_contract_test.go"
Task: "T104 [US4] Add web dashboard tests in web/src/app/App.test.tsx"
```

## Parallel Example: User Story 5

```bash
Task: "T116 [US5] Add inspection classification tests in daemon/internal/evaluation/tool_call_inspection_test.go"
Task: "T117 [US5] Add inspection redaction tests in daemon/internal/evaluation/tool_call_inspection_redaction_test.go"
Task: "T120 [US5] Add inspection contract tests in daemon/internal/contracts/evaluation_product_inspection_contract_test.go"
Task: "T122 [US5] Add web inspection tests in web/src/app/App.test.tsx"
```

---

## Implementation Strategy

### MVP First

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: User Story 1.
4. Stop and validate bounded candidate discovery independently.

### Production Roadmap Closure

1. Complete Setup and Foundational phases.
2. Complete US1 and US2 to make discovery-to-fixture workflows usable.
3. Complete US3 to make evaluation campaigns usable.
4. Complete US4 and US5 to make dashboards and tool-call inspection operationally complete.
5. Complete Phase 8 verification, including contract tests, client tests, daemon smoke, and Roadmap 39 soak rerun evidence.

### Parallel Team Strategy

After Phase 2, split by file ownership:

- Team A: US1 discovery domain, store, API, SDK, and web.
- Team B: US2 fixture domain, revision model, API, SDK, and web.
- Team C: US3 campaign domain and aggregation.
- Team D: US4 dashboard projections and US5 inspection once campaign evidence shapes stabilize.

## Notes

- `[P]` tasks use different files or can be worked independently after their phase prerequisites.
- `[US#]` labels map directly to the user stories in `spec.md`.
- Contract, schema, SDK, web, and docs changes must stay aligned in the same story phase.
- Existing Roadmap 33 replay routes and Roadmap 40 live-validation routes must remain backward compatible unless a task explicitly updates the matching contract and compatibility tests.
- Stop at each checkpoint to validate the story independently before depending on it from later phases.
