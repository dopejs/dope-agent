# Tasks: Hosted Signup And Tenant Activation

**Input**: Design documents from `specs/030-hosted-tenant-activation/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the project constitution and Roadmap 45 spec. API/schema,
persistence, SDK, web, restart, tenant isolation, concurrency, audit, redaction, and
manual test-environment verification tasks are included before implementation is
considered complete.

**Organization**: Tasks are grouped by user story so each story can be implemented and
tested as an independently reviewable increment after shared activation contracts and
storage foundations are in place.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel with other marked tasks in the same phase because it
  touches different files or only adds independent tests/fixtures.
- **[Story]**: Maps to user stories from `spec.md`.
- Every task includes concrete repository paths.

---

## Phase 1: Setup

**Purpose**: Prepare implementation anchors without changing existing tenant, billing,
chat, or shell behavior.

- [X] T001 Create activation package scaffolding with package docs and empty service/type files in `daemon/internal/activation/doc.go`, `daemon/internal/activation/types.go`, and `daemon/internal/activation/service.go`
- [X] T002 [P] Create activation contract fixture directory notes in `daemon/internal/contracts/testdata/activation/README.md`
- [X] T003 [P] Add activation test fixture helper stubs for SDK and web tests in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

---

## Phase 2: Foundational

**Purpose**: Shared schemas, persistence, service boundaries, and route/client shells that
block all user-story implementation.

**Critical**: No user story phase should begin until this phase is complete.

- [X] T004 [P] Add activation API schema files for state, response, test-chat request/response, and diagnostics in `schemas/api/activation-state-resource.schema.json`, `schemas/api/activation.response.schema.json`, `schemas/api/activation-test-chat.request.schema.json`, `schemas/api/activation-test-chat.response.schema.json`, and `schemas/api/activation-diagnostic-list.response.schema.json`
- [X] T005 [P] Register activation schema inventory and compatibility notes in `schemas/api/README.md`
- [X] T006 [P] Add activation API contract fixture tests for additive schema shape and forbidden transcript fields in `daemon/internal/contracts/activation_contract_test.go`
- [X] T007 Add SQLite activation persistence migration and store methods in `daemon/internal/store/migrations.go`, `daemon/internal/store/activation.go`, and `daemon/internal/store/activation_test.go`
- [X] T008 Define activation statuses, reason codes, readiness items, quota baseline, first action, test chat metadata, and audit metadata types in `daemon/internal/activation/types.go`
- [X] T009 Implement activation service dependency interfaces for identity, billing, chat, audit, and storage in `daemon/internal/activation/service.go`
- [X] T010 Register protected activation route shell handlers in `daemon/internal/api/activation.go` and `daemon/internal/api/server.go`
- [X] T011 Add activation resource, diagnostic resource, and method signatures to the TypeScript SDK in `sdk/ts/src/index.ts`
- [X] T012 Add active activation state slots and stale-state placeholders to the web shell in `web/src/app/App.tsx` and `web/src/styles.css`

**Checkpoint**: Shared activation schemas, persistence hooks, service boundaries, API route shells, SDK method shells, and web state slots exist.

---

## Phase 3: User Story 1 - Activate A Personal Tenant From Hosted Entry (Priority: P1) MVP

**Goal**: An authenticated hosted user can sign up or accept an invite, resolve exactly
one personal tenant, and see stable blocked/denied behavior when disabled or denied.

**Independent Test**: Start from a user with no active hosted setup, complete signup or
invite acceptance, confirm one active personal tenant exists, repeat activation without a
duplicate, and verify disabled/denied users receive stable reason codes.

### Tests for User Story 1

- [X] T013 [P] [US1] Add API tests for new-user activation, returning-user activation, invite-acceptance activation, and hosted signup/invite landing without developer-only calls, manual state edits, or operator-run setup in `daemon/internal/api/activation_test.go`
- [X] T014 [P] [US1] Add activation service tests for idempotent personal tenant resolution and concurrent activation convergence in `daemon/internal/activation/service_test.go`
- [X] T015 [P] [US1] Add activation eligibility denial tests for disabled principals, denied tokens, revoked tenant access, and stable reason codes in `daemon/internal/activation/service_test.go`

### Implementation for User Story 1

- [X] T016 [US1] Implement personal tenant create-or-resolve logic using existing identity primitives in `daemon/internal/activation/service.go`
- [X] T017 [US1] Implement transactional activation upsert and uniqueness by principal/personal tenant in `daemon/internal/store/activation.go`
- [X] T018 [US1] Implement `GET /v1/activation` and `POST /v1/activation` start/resolution handlers in `daemon/internal/api/activation.go`
- [X] T019 [US1] Implement metadata-only activation started, denied, and tenant-resolution audit writes in `daemon/internal/activation/audit.go`
- [X] T020 [US1] Wire activation service construction and hosted signup/invite first-run bootstrap into daemon API setup without developer-only calls, manual state edits, operator-run setup, or changed existing tenant/auth behavior in `daemon/internal/api/server.go` and `daemon/internal/api/operator_projection.go`

**Checkpoint**: User Story 1 is independently functional when `go test ./internal/activation ./internal/api ./internal/store` passes from `daemon/`.

---

## Phase 4: User Story 2 - Understand Hosted Readiness And Next Steps (Priority: P1)

**Goal**: A newly activated hosted user can see active tenant, environment, quota
baseline, readiness state, and the required `test_chat` next action.

**Independent Test**: Open the first-run shell after activation and verify tenant,
environment, quota baseline, readiness blockers, and next action are visible; missing
quota baseline blocks activation completion with a retryable reason.

### Tests for User Story 2

- [X] T021 [P] [US2] Add activation service tests for quota baseline projection, missing-quota blocking, and retryable readiness reason in `daemon/internal/activation/readiness_test.go`
- [X] T022 [P] [US2] Add API contract tests for activation projection shape, quota baseline fields, and blocked readiness response in `daemon/internal/contracts/activation_contract_test.go`
- [X] T023 [P] [US2] Add SDK tests for `getActivation`, `activate`, quota baseline parsing, activation blocked error metadata, and activation diagnostic parsing in `sdk/ts/src/index.test.ts`
- [X] T024 [P] [US2] Add web shell tests for active tenant, environment, quota baseline, activation status, blocked quota state, `test_chat` action rendering, and stale activation state after tenant switch or revocation in `web/src/app/App.test.tsx`

### Implementation for User Story 2

- [X] T025 [US2] Implement activation readiness projection and quota-baseline blocking in `daemon/internal/activation/readiness.go`
- [X] T026 [US2] Integrate activation projection into existing first-run onboarding without making organization onboarding blocking in `daemon/internal/api/operator_projection.go`
- [X] T027 [US2] Implement SDK `getActivation` and `activate` methods with tenant option support in `sdk/ts/src/index.ts`
- [X] T028 [US2] Implement web shell activation panel rendering, disabled quota-blocked states, and stale/cleared activation state after tenant switch or access revocation in `web/src/app/App.tsx` and `web/src/styles.css`

**Checkpoint**: User Story 2 is independently functional when focused daemon, SDK, and web activation projection tests pass.

---

## Phase 5: User Story 3 - Complete A Safe First Action (Priority: P1)

**Goal**: The required v1 `test_chat` action completes under the active personal tenant
without live connectors or production secrets, marks activation complete, and retains
metadata only.

**Independent Test**: Choose the test chat action from the activation surface, verify it
completes under the active personal tenant, reload or restart the daemon, and confirm
activation remains `first_action_completed` without retaining transcript content.

### Tests for User Story 3

- [X] T029 [P] [US3] Add API tests for `POST /v1/activation/test-chat` success, readiness failure, tenant mismatch, and stable failure reason codes in `daemon/internal/api/activation_test.go`
- [X] T030 [P] [US3] Add activation redaction tests proving test chat query, reply, transcript, streamed deltas, prompts, and raw provider payloads are not persisted in `daemon/internal/activation/redaction_test.go`
- [X] T031 [P] [US3] Add activation restart durability tests for state after tenant activation before first action and after completed test chat in `daemon/internal/activation/service_test.go`
- [X] T032 [P] [US3] Add SDK and web tests for `runActivationTestChat`, completion metadata parsing, UI completion state, pre-action reload, and post-completion reload behavior in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

### Implementation for User Story 3

- [X] T033 [US3] Implement test chat readiness guard and chat execution adapter in `daemon/internal/activation/test_chat.go`
- [X] T034 [US3] Implement `POST /v1/activation/test-chat` handler and metadata-only response mapping in `daemon/internal/api/activation.go`
- [X] T035 [US3] Implement SDK `runActivationTestChat` method and metadata-only response types in `sdk/ts/src/index.ts`
- [X] T036 [US3] Implement web shell test chat action handler, running state, completion state, and reload refresh in `web/src/app/App.tsx`
- [X] T037 [US3] Implement activation completion audit metadata and persistence transition for `first_action_completed` in `daemon/internal/activation/audit.go` and `daemon/internal/store/activation.go`

**Checkpoint**: User Story 3 is independently functional when test chat completes activation and restart durability tests pass.

---

## Phase 6: User Story 4 - Diagnose Activation Failures (Priority: P2)

**Goal**: Operators can diagnose activation failures through stable reason codes,
metadata-only audit records, and diagnostics without direct storage inspection or secret
exposure.

**Independent Test**: Induce tenant resolution, eligibility, quota, authorization, test
chat, audit, persistence, and unexpected failures; confirm diagnostics identify stage,
reason, retryability, remediation owner, tenant scope when accessible, and no forbidden
data.

### Tests for User Story 4

- [X] T038 [P] [US4] Add diagnostics API tests for representative activation failure classes and remediation owners in `daemon/internal/api/activation_diagnostics_test.go`
- [X] T039 [P] [US4] Add activation audit tests for metadata-only events, retry preservation, and fail-closed audit behavior in `daemon/internal/activation/audit_test.go`
- [X] T040 [P] [US4] Add redaction contract tests for activation diagnostics, audit events, fixtures, and logs in `daemon/internal/contracts/activation_contract_test.go`
- [X] T041 [P] [US4] Add web shell tests for activation diagnostic display, retryable blocked state, denied/revoked state, stale-state clearing, and absence of message content in `web/src/app/App.test.tsx`

### Implementation for User Story 4

- [X] T042 [US4] Implement activation diagnostics projection with stage, reason code, retryability, remediation owner, tenant scope, and quota/test-chat metadata in `daemon/internal/activation/diagnostics.go`
- [X] T043 [US4] Implement `GET /v1/activation/diagnostics` handler and operator diagnostic integration in `daemon/internal/api/activation.go` and `daemon/internal/api/operator_projection.go`
- [X] T044 [US4] Implement SDK activation diagnostics types and `getActivationDiagnostics` client method in `sdk/ts/src/index.ts`
- [X] T045 [US4] Implement web shell diagnostic, retry, denied/revoked, and stale-state affordances without rendering test chat message content in `web/src/app/App.tsx` and `web/src/styles.css`

**Checkpoint**: User Story 4 is independently functional when diagnostics identify every representative failure class with no forbidden data exposure.

---

## Phase 7: Polish & Cross-Cutting Verification

**Purpose**: Final compatibility, documentation, generated output, and test-environment
verification for Roadmap 45 closure.

- [X] T046 [P] Update hosted activation operator notes and rollback guidance in `docs/runtime/hosted-tenant-activation.md` and link them from `docs/runtime/production-operations.md`
- [X] T047 [P] Update final implementation evidence, first-run 30-second review evidence, operator 10-minute diagnostic drill evidence, and any residual gaps in `specs/030-hosted-tenant-activation/quickstart.md`
- [X] T048 Run `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build` from `/Users/John/Code/kura-agent` and fix activation-related failures in `schemas/`, `sdk/ts/`, and `web/`
- [X] T049 Run `go test ./...` and `go mod tidy` from `/Users/John/Code/kura-agent/daemon` and fix activation-related failures or unintended module drift in `daemon/`
- [X] T050 Run the manual `KURA_ENV=test` walkthrough from `specs/030-hosted-tenant-activation/quickstart.md` using `make daemon-run-test` and `make daemon-test-status`, including signup/invite landing, pre-first-action restart, 30-second first-run review, and 10-minute diagnostic drill evidence, then record verification evidence in `specs/030-hosted-tenant-activation/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1 and blocks all user stories.
- **US1 (Phase 3)**: Depends on Phase 2.
- **US2 (Phase 4)**: Depends on Phase 2 and can proceed in parallel with US1 tests, but final projection expects US1 activation state.
- **US3 (Phase 5)**: Depends on Phase 2 and requires US2 quota readiness for completion behavior.
- **US4 (Phase 6)**: Depends on Phase 2 and can develop diagnostics fixtures in parallel, but final validation consumes failure states from US1-US3.
- **Phase 7 Polish**: Depends on all selected user stories being complete.

### User Story Dependencies

- **US1 Activate Personal Tenant**: MVP. No dependency on other user stories after foundation.
- **US2 Hosted Readiness**: Requires activation state from US1 for complete validation; quota-blocked projection can be tested independently with fixtures.
- **US3 Safe First Action**: Requires activation state and quota readiness from US1/US2 before marking completion.
- **US4 Diagnostics**: Can test failure projections independently, but full coverage depends on failure states introduced by US1-US3.

### Within Each User Story

- Write story tests before implementation.
- Add/adjust contract tests before changing client-facing shapes.
- Persistence before service transitions that require durability.
- Service logic before API handlers.
- API/SDK contracts before web shell integration.
- Story checkpoint must pass before claiming that story complete.

---

## Parallel Opportunities

- Setup tasks T002-T003 can run in parallel.
- Foundational schema/contract tasks T004-T006 can run in parallel.
- US1 tests T013-T015 can run in parallel.
- US2 tests T021-T024 can run in parallel across daemon, contract, SDK, and web files.
- US3 tests T029-T032 can run in parallel across daemon, SDK, and web files.
- US4 tests T038-T041 can run in parallel across API, activation, contract, and web files.
- Polish docs T046-T047 can run in parallel with final verification preparation.

---

## Parallel Example: User Story 2

```text
Task: "T021 [P] [US2] Add activation service tests for quota baseline projection in daemon/internal/activation/readiness_test.go"
Task: "T022 [P] [US2] Add API contract tests for activation projection shape in daemon/internal/contracts/activation_contract_test.go"
Task: "T023 [P] [US2] Add SDK tests for getActivation, activate, and diagnostic parsing in sdk/ts/src/index.test.ts"
Task: "T024 [P] [US2] Add web shell tests for activation readiness rendering and stale state in web/src/app/App.test.tsx"
```

## Parallel Example: User Story 3

```text
Task: "T029 [P] [US3] Add API tests for POST /v1/activation/test-chat in daemon/internal/api/activation_test.go"
Task: "T030 [P] [US3] Add activation redaction tests in daemon/internal/activation/redaction_test.go"
Task: "T031 [P] [US3] Add pre-action and post-completion restart durability tests in daemon/internal/activation/service_test.go"
Task: "T032 [P] [US3] Add SDK and web test chat tests in sdk/ts/src/index.test.ts and web/src/app/App.test.tsx"
```

## Parallel Example: User Story 4

```text
Task: "T038 [P] [US4] Add diagnostics API tests in daemon/internal/api/activation_diagnostics_test.go"
Task: "T039 [P] [US4] Add activation audit tests in daemon/internal/activation/audit_test.go"
Task: "T040 [P] [US4] Add redaction contract tests in daemon/internal/contracts/activation_contract_test.go"
Task: "T041 [P] [US4] Add web diagnostic display and stale-state clearing tests in web/src/app/App.test.tsx"
```

---

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundation.
3. Complete Phase 3 US1.
4. Stop and validate personal tenant activation independently.

US1 is the MVP for proving personal tenant activation. Roadmap 45 is not closed until
US1, US2, US3, US4, and Phase 7 verification are complete.

### Incremental Delivery

1. US1: Personal tenant activation and eligibility.
2. US2: Readiness, quota baseline, and shell projection.
3. US3: Required test chat completion and restart durability.
4. US4: Operator diagnostics, audit, and redaction.
5. Phase 7: Full contract/client/daemon/manual verification.

### Parallel Team Strategy

After Phase 2, separate owners can work on:

- Daemon activation service and persistence: `daemon/internal/activation/`, `daemon/internal/store/`
- API/contracts: `daemon/internal/api/`, `daemon/internal/contracts/`, `schemas/api/`
- SDK/web shell: `sdk/ts/src/`, `web/src/app/`, `web/src/styles.css`

Coordinate edits to `daemon/internal/api/activation.go`, `sdk/ts/src/index.ts`, and
`web/src/app/App.tsx` because multiple stories touch those files.

## Notes

- All API/schema/SDK changes must be additive unless a breaking change is explicitly
  approved.
- Keep activation diagnostics and audit metadata-only for test chat.
- Do not use live connectors, production secrets, payment checkout, enterprise SSO, or
  organization administration to satisfy Roadmap 45.
- Run `go mod tidy` from `daemon/` after implementation before marking the spec complete.
