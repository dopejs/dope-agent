# Tasks: Hosted Credential And OAuth Setup Wizard

**Input**: Design documents from `specs/031-credential-oauth-wizard/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the project constitution and Roadmap 46 spec. API/schema,
persistence, SDK, web, restart, tenant isolation, redaction, permission, diagnostics,
audit, and manual test-environment verification tasks are included before implementation
is considered complete.

**Organization**: Tasks are grouped by user story so each story can be implemented and
tested as an independently reviewable increment after shared setup-session contracts and
storage foundations are in place.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel with other marked tasks in the same phase because it
  touches different files or only adds independent tests/fixtures.
- **[Story]**: Maps to user stories from `spec.md`.
- Every task includes concrete repository paths.

---

## Phase 1: Setup

**Purpose**: Prepare implementation anchors without changing existing secret, provider,
integration, diagnostic, or shell behavior.

- [X] T001 Create setup wizard package scaffolding with package docs and empty service/type files in `daemon/internal/setupwizard/doc.go`, `daemon/internal/setupwizard/types.go`, and `daemon/internal/setupwizard/service.go`
- [X] T002 [P] Create setup wizard contract fixture directory notes in `daemon/internal/contracts/testdata/setupwizard/README.md`
- [X] T003 [P] Add setup wizard test fixture helper stubs for SDK and web tests in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

---

## Phase 2: Foundational

**Purpose**: Shared schemas, persistence, service boundaries, permissions, and route/client
shells that block all user-story implementation.

**Critical**: No user story phase should begin until this phase is complete.

- [X] T004 [P] Add setup API schema files for target list, session resource, session response, secret submit request, OAuth start/callback requests, diagnostics list, and setup error responses in `schemas/api/setup-target-list.response.schema.json`, `schemas/api/setup-session-resource.schema.json`, `schemas/api/setup-session.response.schema.json`, `schemas/api/setup-secret-submit.request.schema.json`, `schemas/api/setup-oauth-start.request.schema.json`, `schemas/api/setup-oauth-callback.request.schema.json`, `schemas/api/setup-diagnostic-list.response.schema.json`, and `schemas/api/setup-error.response.schema.json`
- [X] T005 [P] Register setup schema inventory and compatibility notes in `schemas/api/README.md`
- [X] T006 [P] Add setup API contract fixture tests for state unions, permission-denial shape, and forbidden credential/OAuth fields in `daemon/internal/contracts/setupwizard_contract_test.go`
- [X] T007 Add SQLite setup-session persistence migration and store methods for sessions, attempts, diagnostic links, and current-state uniqueness in `daemon/internal/store/migrations.go`, `daemon/internal/store/setupwizard.go`, and `daemon/internal/store/setupwizard_test.go`
- [X] T008 Define setup statuses, setup styles, target kinds, safe-use modes, reason codes, remediation owners, redaction statuses, target/session/attempt/diagnostic/audit types in `daemon/internal/setupwizard/types.go`
- [X] T009 Implement setup wizard service dependency interfaces for secrets, providers, integrations, identity, audit/events, diagnostics, and storage in `daemon/internal/setupwizard/service.go`
- [X] T010 Implement setup wizard target catalog for `provider.openai_compatible`, `integration.feishu_lark`, and unsupported/action-required target projection in `daemon/internal/setupwizard/targets.go`
- [X] T011 Implement setup wizard permission helpers for mutation and redacted inspection requirements in `daemon/internal/setupwizard/permissions.go`
- [X] T012 Implement setup wizard redaction and forbidden-field detection helpers in `daemon/internal/setupwizard/redaction.go`
- [X] T013 Register protected setup route shell handlers in `daemon/internal/api/setupwizard.go` and `daemon/internal/api/server.go`
- [X] T014 Add setup target/session/diagnostic resource types and method signatures to the TypeScript SDK in `sdk/ts/src/index.ts`
- [X] T015 Add web shell setup wizard state slots and placeholder view containers in `web/src/app/App.tsx` and `web/src/styles.css`

**Checkpoint**: Shared setup schemas, persistence hooks, service boundaries, target catalog,
permission helpers, API route shells, SDK method shells, and web state slots exist.

---

## Phase 3: User Story 1 - Connect A Provider Account (Priority: P1) MVP

**Goal**: A hosted user can connect OpenAI-compatible submitted-secret credentials and
Feishu/Lark OAuth authorization from the product setup surface and see ready or
remediable state without raw technical setup.

**Independent Test**: Start from an activated tenant with no connected account, complete
OpenAI-compatible submitted-secret setup and Feishu/Lark OAuth fixture setup, then confirm
both proof targets produce setup state, diagnostic linkage, retry behavior, and redaction
evidence.

### Tests for User Story 1

- [X] T016 [P] [US1] Add setup wizard service tests for OpenAI-compatible submitted-secret happy path, Feishu/Lark OAuth happy path, OAuth denial/abandonment/expired/replay/tenant-mismatch/provider-error outcomes, target catalog projection, and no terminal failed setup state in `daemon/internal/setupwizard/service_test.go`
- [X] T017 [P] [US1] Add setup API tests for target listing, session start, secret submission, OAuth start/callback, OAuth callback denial/abandonment/expired/replay/tenant-mismatch/provider-error mapping, and tenant-scoped permission success in `daemon/internal/api/setupwizard_test.go`
- [X] T018 [P] [US1] Add setup contract tests for target/session/resource response fixtures and OpenAI-compatible/Feishu-Lark proof target fixtures in `daemon/internal/contracts/setupwizard_contract_test.go`
- [X] T019 [P] [US1] Add SDK tests for setup target/session parsing, start setup, submit secret, OAuth start/callback, and tenant options in `sdk/ts/src/index.test.ts`
- [X] T020 [P] [US1] Add web shell tests for target list rendering, OpenAI-compatible secret submission, Feishu/Lark OAuth fixture completion, and hidden raw submitted values in `web/src/app/App.test.tsx`

### Implementation for User Story 1

- [X] T021 [US1] Implement setup target list and session start behavior in `daemon/internal/setupwizard/service.go` and `daemon/internal/setupwizard/targets.go`
- [X] T022 [US1] Implement OpenAI-compatible submitted-secret setup orchestration through tenant secret creation/rotation and provider readiness/check linkage in `daemon/internal/setupwizard/submitted_secret.go`
- [X] T023 [US1] Implement Feishu/Lark OAuth start and callback fixture orchestration with redacted OAuth attempt metadata in `daemon/internal/setupwizard/oauth.go`
- [X] T024 [US1] Implement diagnostic linkage projection for ready, degraded, action-required, unavailable, and unsupported setup outcomes in `daemon/internal/setupwizard/diagnostics.go`
- [X] T025 [US1] Implement setup API handlers for `GET /v1/setup/targets`, `GET /v1/setup/sessions`, `POST /v1/setup/sessions`, `POST /v1/setup/sessions/{sessionId}/submit-secret`, `POST /v1/setup/sessions/{sessionId}/oauth/start`, `POST /v1/setup/sessions/{sessionId}/oauth/callback`, and `GET /v1/setup/sessions/{sessionId}` in `daemon/internal/api/setupwizard.go`
- [X] T026 [US1] Implement SDK setup methods `listSetupTargets`, `listSetupSessions`, `startSetup`, `getSetupSession`, `submitSetupSecret`, `startSetupOAuth`, and `completeSetupOAuth` in `sdk/ts/src/index.ts`
- [X] T027 [US1] Implement web shell setup wizard rendering and proof-target flows for OpenAI-compatible credentials and Feishu/Lark OAuth fixtures in `web/src/app/App.tsx` and `web/src/styles.css`

**Checkpoint**: User Story 1 is independently functional when focused setup wizard daemon,
contract, SDK, and web tests for proof-target connection pass.

---

## Phase 4: User Story 2 - Retry, Replace, Cancel, Or Disable Setup (Priority: P1)

**Goal**: A hosted user can safely recover from failed or obsolete setup by retrying,
replacing, cancelling, or disabling setup without leaking secrets or deleting unrelated
integration/provider metadata.

**Independent Test**: Induce action-required and unavailable setup states for both proof
targets, retry or replace with corrected evidence, cancel in-progress setup, disable
connected setup, and confirm unrelated metadata and redacted historical evidence remain.

### Tests for User Story 2

- [X] T028 [P] [US2] Add setup wizard service tests for retry, replace, cancel, disable, current-state uniqueness, historical attempt preservation, and concurrent retry convergence in `daemon/internal/setupwizard/recovery_test.go`
- [X] T029 [P] [US2] Add setup store tests for append-only attempts, tenant/target/style current-state uniqueness, and restart durability before/after recovery transitions in `daemon/internal/store/setupwizard_test.go`
- [X] T030 [P] [US2] Add setup API tests for retry, replace, cancel, disable, missing `credentials.inspect` read/list/diagnostics denials, and stale or revoked tenant context behavior in `daemon/internal/api/setupwizard_recovery_test.go`
- [X] T031 [P] [US2] Add SDK tests for `retrySetup`, `replaceSetup`, `cancelSetup`, `disableSetup`, inspection-denial parsing, and disabled/action-required state parsing in `sdk/ts/src/index.test.ts`
- [X] T032 [P] [US2] Add web shell tests for retry, replace, cancel, disable, inspection-denial display, stale-state clearing, and absence of prior secret/OAuth material in `web/src/app/App.test.tsx`

### Implementation for User Story 2

- [X] T033 [US2] Implement setup retry, replace, cancel, and disable state transitions in `daemon/internal/setupwizard/recovery.go`
- [X] T034 [US2] Implement current-state upsert and append-only attempt persistence for recovery transitions in `daemon/internal/store/setupwizard.go`
- [X] T035 [US2] Implement dependent-use decision projection for ready, degraded, action-required, unavailable, cancelled, and disabled setup states, including target-declared and diagnostic-confirmed `allowedCapabilities` validation, in `daemon/internal/setupwizard/dependent_use.go`
- [X] T036 [US2] Integrate dependent-use setup gating and degraded `allowedCapabilities` validation with OpenAI-compatible provider credential resolution and Feishu/Lark integration readiness paths in `daemon/internal/providers/manager.go` and `daemon/internal/integrations/manager.go`
- [X] T037 [US2] Implement setup API handlers for `POST /v1/setup/sessions/{sessionId}/retry`, `POST /v1/setup/sessions/{sessionId}/replace`, `POST /v1/setup/sessions/{sessionId}/cancel`, and `POST /v1/setup/sessions/{sessionId}/disable` in `daemon/internal/api/setupwizard.go`
- [X] T038 [US2] Implement SDK recovery methods `retrySetup`, `replaceSetup`, `cancelSetup`, and `disableSetup` in `sdk/ts/src/index.ts`
- [X] T039 [US2] Implement web shell retry, replace, cancel, disable, disabled-state, and degraded limited-use affordances in `web/src/app/App.tsx` and `web/src/styles.css`

**Checkpoint**: User Story 2 is independently functional when recovery and dependent-use
tests pass for both proof targets without deleting unrelated metadata.

---

## Phase 5: User Story 3 - Diagnose Setup Failures (Priority: P2)

**Goal**: Operators can inspect setup attempts and redacted diagnostics to identify
missing scope, tenant approval, token failure, provider outage, network failure,
unsupported target, permission denial, and redaction failure without raw credential or
OAuth payload exposure.

**Independent Test**: Induce representative setup failures and confirm setup diagnostics,
operator diagnostics, audit records, SDK/web projections, and redaction checks show stage,
reason, retry safety, remediation owner, tenant scope, and safe evidence only.

### Tests for User Story 3

- [X] T040 [P] [US3] Add setup diagnostics service tests for credential missing, scope missing, tenant approval pending, token missing/expired/revoked, OAuth denial/abandonment/expired/replay, tenant mismatch, provider unavailable, network failed, rate limited, unsupported target, degraded allowed-capability confirmation, and redaction failed closed in `daemon/internal/setupwizard/diagnostics_test.go`
- [X] T041 [P] [US3] Add setup audit tests for metadata-only event families, retry/replacement history, fail-closed redaction, and no forbidden evidence in `daemon/internal/setupwizard/audit_test.go`
- [X] T042 [P] [US3] Add operator diagnostics API tests for setup findings, detail routes, remediation owners, dependent-use blocked state, missing inspection permission, cross-tenant setup/session/evidence isolation, and tenant-scoped non-disclosure in `daemon/internal/api/operator_test.go`
- [X] T043 [P] [US3] Add setup redaction and tenant-isolation contract tests for setup state, diagnostics, audit events, fixtures, logs, SDK output, web output, same external account in two tenants, and same secret ref in two tenants in `daemon/internal/contracts/setupwizard_contract_test.go`
- [X] T044 [P] [US3] Add SDK and web tests for setup diagnostics rendering, operator remediation fields, redaction failed closed, and unsupported/action-required targets in `sdk/ts/src/index.test.ts` and `web/src/app/App.test.tsx`

### Implementation for User Story 3

- [X] T045 [US3] Implement setup diagnostic reason-code mapping and diagnostic-list projection in `daemon/internal/setupwizard/diagnostics.go`
- [X] T046 [US3] Implement setup audit metadata records and event publication hooks for setup transitions in `daemon/internal/setupwizard/audit.go`
- [X] T047 [US3] Implement redaction fail-closed enforcement across setup attempts, diagnostics, audit documents, and response mapping in `daemon/internal/setupwizard/redaction.go`
- [X] T048 [US3] Implement `GET /v1/setup/sessions/{sessionId}/diagnostics` and setup error response mapping in `daemon/internal/api/setupwizard.go`
- [X] T049 [US3] Integrate setup findings into operator diagnostics projection in `daemon/internal/api/operator_projection.go`
- [X] T050 [US3] Implement SDK setup diagnostics types and `getSetupDiagnostics` client method in `sdk/ts/src/index.ts`
- [X] T051 [US3] Implement web shell diagnostic display, remediation states, unsupported/action-required targets, and redaction failed closed messages in `web/src/app/App.tsx` and `web/src/styles.css`

**Checkpoint**: User Story 3 is independently functional when diagnostics identify every
representative failure class with no forbidden evidence exposure.

---

## Phase 6: Polish & Cross-Cutting Verification

**Purpose**: Final compatibility, documentation, generated output, and test-environment
verification for Roadmap 46 closure.

- [X] T052 [P] Update hosted credential setup operator notes and rollback guidance in `docs/runtime/hosted-credential-setup.md` and link them from `docs/runtime/production-operations.md`
- [X] T053 [P] Update OpenAI-compatible and Feishu/Lark setup guidance in `docs/providers/openai-compatible-setup.md` and `docs/providers/feishu-lark-setup.md`
- [X] T054 [P] Update schema inventory and setup contract fixture documentation in `schemas/api/README.md` and `daemon/internal/contracts/testdata/setupwizard/README.md`
- [X] T055 [P] Update final implementation evidence, 30-second setup review evidence, 10-minute operator diagnostic drill evidence, restart recovery evidence, and residual gaps in `specs/031-credential-oauth-wizard/quickstart.md`
- [X] T056 Run `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build` from `/Users/John/Code/kura-agent` and fix setup-related failures in `schemas/`, `sdk/ts/`, and `web/`
- [X] T057 Run `go test ./...` and `go mod tidy` from `/Users/John/Code/kura-agent/daemon` and fix setup-related failures or unintended module drift in `daemon/`
- [X] T058 Run the manual `KURA_ENV=test` walkthrough from `specs/031-credential-oauth-wizard/quickstart.md` using `make daemon-run-test` and `make daemon-test-status`, including activated tenant setup, OpenAI-compatible fake secret redaction proof, Feishu/Lark OAuth fixture proof, retry/replace/cancel/disable, restart recovery, and operator diagnostic drill evidence

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1 and blocks all user stories.
- **US1 (Phase 3)**: Depends on Phase 2 and is the MVP.
- **US2 (Phase 4)**: Depends on Phase 2 and can be developed after US1 proof target routes exist; recovery tests can be drafted in parallel with US1 implementation.
- **US3 (Phase 5)**: Depends on Phase 2 and consumes failure states from US1/US2; diagnostic contract tests can be drafted in parallel.
- **Phase 6 Polish**: Depends on all selected user stories being complete.

### User Story Dependencies

- **User Story 1 Connect Provider Account**: MVP. No dependency on other user stories after foundation.
- **User Story 2 Retry/Replace/Cancel/Disable**: Requires setup sessions and proof targets from US1 for full validation, but recovery state-machine tests can run against fixtures.
- **User Story 3 Diagnose Setup Failures**: Requires setup sessions and representative failure states; operator projection depends on diagnostic and audit metadata from US1/US2.

### Within Each User Story

- Write story tests before implementation.
- Add/adjust contract tests before changing client-facing shapes.
- Persistence before service transitions that require durability.
- Service logic before API handlers.
- API/SDK contracts before web shell integration.
- Story checkpoint must pass before claiming that story complete.

### Parallel Opportunities

- Setup tasks T002-T003 can run in parallel.
- Foundational schema/contract tasks T004-T006 can run in parallel.
- US1 tests T016-T020 can run in parallel across daemon, API, contract, SDK, and web files.
- US2 tests T028-T032 can run in parallel across daemon, store, API, SDK, and web files.
- US3 tests T040-T044 can run in parallel across daemon, API, contract, SDK, and web files.
- Polish docs T052-T055 can run in parallel after implementation is stable.

---

## Parallel Example: User Story 1

```text
Task: "T016 [P] [US1] Add setup wizard service tests for proof-target happy paths in daemon/internal/setupwizard/service_test.go"
Task: "T017 [P] [US1] Add setup API tests for target listing and setup starts in daemon/internal/api/setupwizard_test.go"
Task: "T019 [P] [US1] Add SDK tests for setup methods in sdk/ts/src/index.test.ts"
Task: "T020 [P] [US1] Add web shell tests for setup wizard rendering in web/src/app/App.test.tsx"
```

## Parallel Example: User Story 2

```text
Task: "T028 [P] [US2] Add setup wizard service tests for retry/replace/cancel/disable in daemon/internal/setupwizard/recovery_test.go"
Task: "T029 [P] [US2] Add setup store tests for append-only attempts in daemon/internal/store/setupwizard_test.go"
Task: "T031 [P] [US2] Add SDK tests for recovery methods in sdk/ts/src/index.test.ts"
Task: "T032 [P] [US2] Add web shell tests for recovery UI in web/src/app/App.test.tsx"
```

## Parallel Example: User Story 3

```text
Task: "T040 [P] [US3] Add setup diagnostics service tests, including OAuth negative lifecycle and degraded allowed-capability confirmation, in daemon/internal/setupwizard/diagnostics_test.go"
Task: "T041 [P] [US3] Add setup audit tests in daemon/internal/setupwizard/audit_test.go"
Task: "T042 [P] [US3] Add operator diagnostics API tests in daemon/internal/api/operator_test.go"
Task: "T044 [P] [US3] Add SDK and web diagnostics tests in sdk/ts/src/index.test.ts and web/src/app/App.test.tsx"
```

---

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundation.
3. Complete Phase 3 US1.
4. Stop and validate proof-target setup independently.

### Incremental Delivery

1. Add setup-session contracts and persistence before route behavior.
2. Add OpenAI-compatible and Feishu/Lark proof target setup with redaction first.
3. Add recovery operations and dependent-use gating.
4. Add diagnostics, audit, operator projection, SDK/web remediation, and docs.
5. Run full verification and record quickstart evidence.

### Completion Criteria

- All tasks T001-T058 complete.
- `go test ./...` passes from `daemon/`.
- `go mod tidy` from `daemon/` produces no unintended drift.
- `make daemon-contract-test`, `pnpm test:clients`, and `pnpm build` pass.
- Manual `KURA_ENV=test` walkthrough evidence is recorded in `quickstart.md`.
- No raw credential or OAuth material appears in setup state, diagnostics, audit, logs,
  fixtures, SDK output, or web output.
- Cross-tenant setup/session/evidence isolation and degraded allowed-capability gating
  are verified for the v1 proof targets.
