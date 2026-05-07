# Tasks: Public Quota UX

**Input**: Design documents from `/specs/032-public-quota-ux/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/public-quota-ux.md, quickstart.md

**Tests**: Required. This feature changes daemon API, JSON schemas, TypeScript SDK, and web shell behavior, so each story includes contract, unit, integration, SDK, and/or web tests before implementation tasks.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it touches different files and has no dependency on another incomplete task.
- **[Story]**: Maps task to a user story from `spec.md`.
- Every checklist task includes an exact file path.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish shared contract fixtures and test locations before story work begins.

- [X] T001 Create placeholder JSON schema files for dashboard, denial detail, and evidence export in `schemas/api/billing-quota-dashboard.response.schema.json`, `schemas/api/billing-denial-detail.response.schema.json`, and `schemas/api/billing-evidence-export.response.schema.json`
- [X] T002 [P] Add phase 47 contract fixture helpers to `daemon/internal/contracts/billing_contracts_test.go`
- [X] T003 [P] Add public quota UX API test fixture builders to `daemon/internal/api/hosted_billing_test.go`
- [X] T004 [P] Add TypeScript SDK fixture builders for public quota UX resources to `sdk/ts/src/index.test.ts`
- [X] T005 [P] Add web shell fixture builders for public quota UX states to `web/src/app/App.test.tsx`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared daemon types, permission model, schemas, and SDK contracts that all user stories use.

**Critical**: No user story implementation should begin until these shared contracts exist.

- [X] T006 Define shared public quota UX Go types for dashboard, quota status item, period summary, denial detail, explicit abuse restriction records, override summary, evidence export, redaction, and category-defined typical operation amount in `daemon/internal/billing/types.go`
- [X] T007 Implement shared quota UX classification helpers for status, near-limit reason, category-defined typical operation amount, recovery actions, safe operation references, and category grouping in `daemon/internal/billing/projection.go`
- [X] T008 [P] Add unit tests for quota status classification, category grouping, recovery action selection, and near-limit helper boundaries for count, attempt, and byte quota categories in `daemon/internal/billing/projection_test.go`
- [X] T009 Add canonical `billing.evidence_export` permission to `daemon/internal/identity/permissions.go`
- [X] T010 [P] Add permission contract coverage proving `billing.evidence_export` is required and `billing.view` alone is insufficient in `daemon/internal/identity/permissions_test.go`
- [X] T011 [P] Add tenant permission schema entry for `billing.evidence_export` in `schemas/api/tenant-permission-resource.schema.json`
- [X] T012 [P] Define TypeScript SDK public quota UX resource types and stable literal unions in `sdk/ts/src/index.ts`
- [X] T013 [P] Define initial web shell state types and formatting helpers for public quota UX in `web/src/app/App.tsx`

**Checkpoint**: Shared contracts and helpers are ready; user story phases can proceed.

---

## Phase 3: User Story 1 - Understand Current Limits And Usage (Priority: P1) MVP

**Goal**: Authorized users can view the active tenant plan, all enforced quota categories grouped into readable sections, current and previous-period usage, reset timing, near-limit warnings, and finite/unlimited/not-measurable states.

**Independent Test**: View tenants with finite usage, exhausted usage, near-limit usage, and unlimited development usage; confirm the dashboard shows correct plan, grouped categories, current and previous-period usage, remaining allowance, reset/recovery state, and no cross-tenant data.

### Tests for User Story 1

- [X] T014 [P] [US1] Add contract tests for `billing-quota-dashboard.response.schema.json` and additive quota resource fields in `daemon/internal/contracts/billing_contracts_test.go`
- [X] T015 [P] [US1] Add daemon unit tests for current plus immediately previous completed period projection in `daemon/internal/billing/projection_test.go`
- [X] T016 [P] [US1] Add daemon unit tests for 80% near-limit and below-one-typical-operation warning thresholds for count, attempt, and byte quota categories in `daemon/internal/billing/projection_test.go`
- [X] T017 [P] [US1] Add API tests for `/v1/billing/quota-dashboard` tenant scoping, all enforced categories, unlimited/not-measurable plans, and unauthorized denial in `daemon/internal/api/hosted_billing_test.go`
- [X] T018 [P] [US1] Add SDK tests for `getBillingQuotaDashboard` typing and tenant override propagation in `sdk/ts/src/index.test.ts`
- [X] T019 [P] [US1] Add web tests for grouped quota dashboard rendering, current/previous-period usage, near-limit states, unlimited state, and unauthorized state in `web/src/app/App.test.tsx`

### Implementation for User Story 1

- [X] T020 [US1] Implement previous-period lookup support for quota summaries in `daemon/internal/store/billing.go`
- [X] T021 [US1] Implement quota dashboard projection using all enforced catalog categories, readable section groups, current period, previous period, category-defined typical operation amount, and near-limit status in `daemon/internal/billing/projection.go`
- [X] T022 [US1] Add `/v1/billing/quota-dashboard` routing and permission-gated response handling in `daemon/internal/api/hosted_billing.go`
- [X] T023 [US1] Finalize dashboard and additive quota JSON schemas in `schemas/api/billing-quota-dashboard.response.schema.json`, `schemas/api/billing-quota-resource.schema.json`, and `schemas/api/billing-usage.response.schema.json`
- [X] T024 [US1] Add `getBillingQuotaDashboard` SDK method and exported dashboard resource types in `sdk/ts/src/index.ts`
- [X] T025 [US1] Implement quota dashboard UI, grouped quota sections, status formatting, current/previous-period display, and unauthorized state in `web/src/app/App.tsx`

**Checkpoint**: User Story 1 is independently testable as the MVP.

---

## Phase 4: User Story 2 - Explain Quota And Abuse Denials (Priority: P1)

**Goal**: Users can inspect quota and abuse denial details with source operation, stable reason, category, measured amount, recovery actions, and abuse restriction information that hides detection signals and thresholds.

**Independent Test**: Attempt or seed ordinary quota exhaustion, quota-state unavailable, temporary abuse restriction, and operator-action-needed denial; confirm each detail view shows correct classification and recovery guidance without leaking cross-tenant data or abuse internals.

### Tests for User Story 2

- [X] T026 [P] [US2] Add contract tests for `billing-denial-detail.response.schema.json` including classifications and stable reason codes in `daemon/internal/contracts/billing_contracts_test.go`
- [X] T027 [P] [US2] Add daemon unit tests for denial classification and safe operation reference mapping across run launches, workflow launches, runtime tool calls, live validation attempts, integration operations, artifact storage bytes, replay evaluation attempts, quota-state unavailable, operator-action-needed, and abuse restriction cases in `daemon/internal/billing/denial_test.go`
- [X] T028 [P] [US2] Add API tests for `/v1/billing/denials/{denialId}` across all Roadmap 38 guarded categories plus quota-state unavailable, operator-action-needed, explicit abuse restriction, unauthorized, and cross-tenant denial in `daemon/internal/api/hosted_billing_test.go`
- [X] T029 [P] [US2] Add SDK tests for `getBillingDenialDetail` stable classification handling and tenant override propagation in `sdk/ts/src/index.test.ts`
- [X] T030 [P] [US2] Add web tests for denial detail rendering, recovery actions, abuse restriction messaging, and hidden detection signals in `web/src/app/App.test.tsx`

### Implementation for User Story 2

- [X] T031 [US2] Implement denial detail projection, classification, recovery actions, and abuse restriction visibility rules in `daemon/internal/billing/denial.go`
- [X] T032 [US2] Add tenant-scoped denial lookup repository support by denial ID in `daemon/internal/store/billing.go`
- [X] T033 [US2] Add `/v1/billing/denials/{denialId}` routing and permission-gated response handling in `daemon/internal/api/hosted_billing.go`
- [X] T034 [US2] Finalize denial detail and additive denial JSON schemas in `schemas/api/billing-denial-detail.response.schema.json` and `schemas/api/billing-denial-resource.schema.json`
- [X] T035 [US2] Add `getBillingDenialDetail` SDK method and exported denial detail resource types in `sdk/ts/src/index.ts`
- [X] T036 [US2] Implement denial detail UI, recovery action display, unavailable state, operator-action-needed state, and abuse restriction copy in `web/src/app/App.tsx`

**Checkpoint**: User Story 2 is independently testable without support export or override admin UI.

---

## Phase 5: User Story 3 - Make Plan Overrides And Restrictions Visible (Priority: P2)

**Goal**: Tenant owners and administrators can understand why effective limits differ from base plan limits and can see temporary restriction state without changing accounting semantics.

**Independent Test**: View tenants with no override, increased override, lowered override, and temporary restriction; confirm the product shows base versus effective limits, visible reason, duration when available, recovery guidance, and no detection signals.

### Tests for User Story 3

- [X] T037 [P] [US3] Add daemon unit tests for override summary projection and explicit abuse restriction record projection in `daemon/internal/billing/projection_test.go`
- [X] T038 [P] [US3] Add API tests for quota dashboard override visibility, lowered override behavior, explicit abuse restriction record visibility, and hidden detection thresholds in `daemon/internal/api/hosted_billing_test.go`
- [X] T039 [P] [US3] Add SDK tests for override and abuse restriction summary fields in `sdk/ts/src/index.test.ts`
- [X] T040 [P] [US3] Add web tests for base-versus-effective limit display, visible override reason, restriction duration, and billing-visibility permission-gated controls in `web/src/app/App.test.tsx`

### Implementation for User Story 3

- [X] T041 [US3] Extend quota dashboard projection with base limit, effective limit, visible override summary, explicit abuse restriction record summary, and restriction recovery action in `daemon/internal/billing/projection.go`
- [X] T042 [US3] Extend billing repository/projection reads for visible active override summaries and explicit abuse restriction records in `daemon/internal/store/billing.go`
- [X] T043 [US3] Add additive override and restriction fields to quota dashboard schema in `schemas/api/billing-quota-dashboard.response.schema.json`
- [X] T044 [US3] Add override and abuse restriction summary SDK types to `sdk/ts/src/index.ts`
- [X] T045 [US3] Implement base-versus-effective limit and restriction visibility in the quota dashboard UI in `web/src/app/App.tsx`

**Checkpoint**: User Story 3 is independently testable after dashboard foundation exists.

---

## Phase 6: User Story 4 - Export Support Evidence For Disputes (Priority: P3)

**Goal**: Authorized support operators can export structured redacted JSON evidence for ordinary quota denials and abuse-restriction denials without exposing secrets, connector payloads, unrelated run content, or cross-tenant data.

**Independent Test**: Select an ordinary quota denial and an abuse-restriction denial as a support operator and export JSON evidence for each denial; confirm tenant-scoped denial metadata, usage snapshot, effective limit state, recovery state, audit references, and redaction records are present, while unauthorized users receive no partial export.

### Tests for User Story 4

- [X] T046 [P] [US4] Add contract tests for `billing-evidence-export.response.schema.json` requiring schema version, redactions, denial, usage snapshot, effective limit state, and audit refs for ordinary quota denials and abuse-restriction denials in `daemon/internal/contracts/billing_contracts_test.go`
- [X] T047 [P] [US4] Add daemon unit tests for ordinary quota denial export, abuse-restriction denial export, evidence redaction, and cross-tenant exclusion in `daemon/internal/billing/denial_test.go`
- [X] T048 [P] [US4] Add API tests for `/v1/billing/denials/{denialId}/evidence-export` requiring `billing.evidence_export`, denying `billing.view` alone, returning structured JSON for ordinary quota and abuse-restriction denials, redaction records, and unauthorized no-partial-export behavior in `daemon/internal/api/hosted_billing_test.go`
- [X] T049 [P] [US4] Add SDK tests for `exportBillingDenialEvidence` typed JSON response and tenant override propagation in `sdk/ts/src/index.test.ts`
- [X] T050 [P] [US4] Add web tests for support evidence export action visibility, successful export state, and permission-denied export state in `web/src/app/App.test.tsx`

### Implementation for User Story 4

- [X] T051 [US4] Implement structured redacted JSON evidence export assembly and redaction records for ordinary quota denials and abuse-restriction denials in `daemon/internal/billing/denial.go`
- [X] T052 [US4] Add tenant-scoped audit and usage evidence reference lookup support for exports in `daemon/internal/store/billing.go`
- [X] T053 [US4] Add `/v1/billing/denials/{denialId}/evidence-export` routing, `billing.evidence_export` permission check, and structured JSON response in `daemon/internal/api/hosted_billing.go`
- [X] T054 [US4] Finalize evidence export JSON schema in `schemas/api/billing-evidence-export.response.schema.json`
- [X] T055 [US4] Add `exportBillingDenialEvidence` SDK method and exported evidence resource types in `sdk/ts/src/index.ts`
- [X] T056 [US4] Implement support evidence export action, redaction summary, success state, and denial state in `web/src/app/App.tsx`

**Checkpoint**: User Story 4 is independently testable as support evidence workflow.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Align documentation, generated outputs, and full-roadmap verification.

- [X] T057 [P] Update hosted billing quota operator documentation for public quota dashboard, denial details, abuse restriction visibility, and evidence export in `docs/runtime/hosted-billing-quotas.md`
- [X] T058 [P] Update phase 47 quickstart with any final command or fixture names discovered during implementation in `specs/032-public-quota-ux/quickstart.md`
- [X] T059 [P] Add or update any generated SDK distribution artifacts required by the repository build in `sdk/ts/dist/index.d.ts` and `sdk/ts/dist/index.js`
- [X] T060 Run `gofmt` for modified daemon Go files under `daemon/internal/api`, `daemon/internal/billing`, `daemon/internal/store`, `daemon/internal/contracts`, and `daemon/internal/identity`
- [X] T061 Run `go test ./...` from `daemon/` to verify daemon packages under `daemon/internal`
- [X] T062 Run `go mod tidy` from `daemon/` and verify `daemon/go.mod` and `daemon/go.sum`
- [X] T063 Run `make daemon-contract-test` using `Makefile`
- [X] T064 Run `pnpm test:clients` using root `package.json`
- [X] T065 Run `pnpm build` using root `package.json`
- [X] T066 Perform the manual `DOPE_ENV=test` walkthrough from `specs/032-public-quota-ux/quickstart.md`
- [X] T067 Record verification results, rollback path, residual risks, and any unverified gaps in `specs/032-public-quota-ux/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1 and blocks all user story phases.
- **US1 Dashboard MVP**: Depends on Phase 2.
- **US2 Denial Detail**: Depends on Phase 2; can run alongside US1 after shared helpers exist, but web integration is easier after US1 dashboard state exists.
- **US3 Override And Restriction Visibility**: Depends on US1 dashboard projection and Phase 2.
- **US4 Evidence Export**: Depends on US2 denial detail projection and Phase 2.
- **Polish**: Depends on all implemented user stories.

### User Story Dependencies

- **US1 (P1)**: Start after Phase 2; suggested MVP.
- **US2 (P1)**: Start after Phase 2; shares denial/recovery helpers with US1 but remains independently testable.
- **US3 (P2)**: Start after US1 dashboard projection exists.
- **US4 (P3)**: Start after US2 denial detail projection exists.

### Parallel Opportunities

- Setup fixture tasks T002-T005 can run in parallel.
- Foundational tasks T008-T013 can run in parallel after T006-T007 define shared shape.
- Test tasks within each story can run in parallel before implementation.
- SDK and web implementation tasks can run in parallel with daemon implementation after schemas are stable.
- US1 and US2 can proceed in parallel after Phase 2 if teams coordinate shared helper changes.

---

## Parallel Execution Examples

### User Story 1

```text
Task: "T014 [P] [US1] Add contract tests in daemon/internal/contracts/billing_contracts_test.go"
Task: "T015 [P] [US1] Add period projection tests in daemon/internal/billing/projection_test.go"
Task: "T018 [P] [US1] Add SDK tests in sdk/ts/src/index.test.ts"
Task: "T019 [P] [US1] Add web tests in web/src/app/App.test.tsx"
```

### User Story 2

```text
Task: "T026 [P] [US2] Add denial detail contract tests in daemon/internal/contracts/billing_contracts_test.go"
Task: "T027 [P] [US2] Add denial classification tests in daemon/internal/billing/denial_test.go"
Task: "T029 [P] [US2] Add SDK denial detail tests in sdk/ts/src/index.test.ts"
Task: "T030 [P] [US2] Add web denial detail tests in web/src/app/App.test.tsx"
```

### User Story 3

```text
Task: "T037 [P] [US3] Add override projection tests in daemon/internal/billing/projection_test.go"
Task: "T039 [P] [US3] Add SDK override and restriction tests in sdk/ts/src/index.test.ts"
Task: "T040 [P] [US3] Add web override and restriction tests in web/src/app/App.test.tsx"
```

### User Story 4

```text
Task: "T046 [P] [US4] Add evidence export contract tests in daemon/internal/contracts/billing_contracts_test.go"
Task: "T047 [P] [US4] Add redaction tests in daemon/internal/billing/denial_test.go"
Task: "T049 [P] [US4] Add SDK evidence export tests in sdk/ts/src/index.test.ts"
Task: "T050 [P] [US4] Add web evidence export tests in web/src/app/App.test.tsx"
```

---

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup tasks.
2. Complete Phase 2 foundational shared contracts.
3. Complete US1 dashboard tasks T014-T025.
4. Validate US1 independently with daemon tests, schema tests, SDK tests, and web tests.

### Incremental Delivery

1. Ship US1 to make quota status understandable before work starts.
2. Add US2 so denials and abuse restrictions are explainable and actionable.
3. Add US3 so tenant owners/admins can explain base versus effective limit differences.
4. Add US4 so support operators can export redacted dispute evidence.
5. Complete polish and full verification tasks T057-T067.

### Production Safety

Keep all changes additive and reversible. Roadmap 38 enforcement remains authoritative,
and rollback should disable the new projection routes/views/SDK calls without deleting
usage counters, denials, audit records, reservations, or plan records.
