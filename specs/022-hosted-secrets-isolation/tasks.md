---
description: "Task list for Roadmap 37 — Hosted Secrets, Integrations, And Connector Isolation"
---

# Tasks: Hosted Secrets, Integrations, And Connector Isolation

**Input**: Design documents from `specs/022-hosted-secrets-isolation/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the project constitution and feature spec. Every behavior change
below includes targeted daemon, contract, migration, redaction, audit, or smoke
verification before the implementation is considered complete.

**Organization**: Tasks are grouped by user story so each story can be implemented and
verified as an independently testable increment. US1 and US2 are both P1 and together
form the hosted credential isolation MVP. US3 and US4 are P2 and close operator
debuggability plus local upgrade continuity.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: parallelizable with other marked tasks in the same phase because it touches
  different files and has no dependency on incomplete tasks
- **[Story]**: maps to user stories from `spec.md`
- Every task includes concrete repository paths

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm active context, preserve the R37 boundary, and add shared fixtures
used by multiple story phases.

- [X] T001 Verify active feature context and record any mismatch in `specs/022-hosted-secrets-isolation/quickstart.md`: current branch must be `022-hosted-secrets-isolation`, `.specify/feature.json` must point to `specs/022-hosted-secrets-isolation`, and `AGENTS.md` must point to `specs/022-hosted-secrets-isolation/plan.md`.
- [X] T002 [P] Add shared two-tenant credential fixture helpers and fake leak sentinel constants in `daemon/internal/secrets/test_fixtures_test.go`.
- [X] T003 [P] Add shared tenant/principal permission fixture helpers, including `credentials.inspect`, for hosted credential API tests in `daemon/internal/api/hosted_credentials_test.go`.
- [X] T004 [P] Add shared fake MCP, connector, integration binding, provider auth, and sandbox fixture builders for R37 isolation tests in `daemon/internal/store/tenancy/r37_fixtures_test.go`.
- [X] T005 [P] Add fake redaction sentinel helpers for contract and event tests in `daemon/internal/contracts/r37_redaction_contracts_test.go`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Land failing tests and shared infrastructure for tenant secret metadata,
versioned value storage, R37 boundary ownership, redaction, stable denials, and audit
helpers.

**CRITICAL**: No user story phase should begin until this phase is complete.

### Foundational Tests

- [X] T006 [P] Add handoff table completeness test for every required R37 shared resource in `daemon/internal/store/tenancy/r37_handoff_test.go`.
- [X] T007 [P] Add schema migration test for `tenant_secrets`, `tenant_secret_versions`, and tenant columns on `provider_auth_states`, `connectors`, `mcp_servers`, `mcp_server_states`, and `mcp_tools` in `daemon/internal/store/r37_schema_test.go`.
- [X] T008 [P] Add tenant-aware uniqueness and same-name cross-tenant fixture tests for secret refs, connector ids, MCP server ids, and provider auth ids in `daemon/internal/store/r37_uniqueness_test.go`.
- [X] T009 [P] Add redaction contract tests that fail when R37 fake secret values appear in API responses, event payloads, logs, replay fixtures, evaluation artifacts, diagnostics, or contract fixtures in `daemon/internal/contracts/r37_redaction_contracts_test.go`.
- [X] T010 [P] Add stable credential-denial tests for missing tenant, missing permission, and cross-tenant credential access in `daemon/internal/api/hosted_credentials_test.go`.
- [X] T011 [P] Add audit helper tests for credential audit fields, successful-use granularity, and failed-closed redaction in `daemon/internal/audit/credential_audit_test.go`.

### Foundational Implementation

- [X] T012 Create tenant secret, secret version, resolution, redaction, disabled resource, and audit action types in `daemon/internal/secrets/types.go`.
- [X] T013 Implement local test value backend with restrictive file permissions and no loggable values in `daemon/internal/secrets/local_backend.go`.
- [X] T014 Implement secret redaction helpers and leak sentinel scanning helpers in `daemon/internal/secrets/redaction.go`.
- [X] T015 Implement versioned tenant secret service skeleton with create, metadata update, rotate, disable, and resolve methods in `daemon/internal/secrets/manager.go`.
- [X] T016 Add SQLite schema migrations, indexes, and backup-restore comments for R37 secret tables and boundary resource tenant columns in `daemon/internal/store/store.go`.
- [X] T017 Add store methods for tenant secret metadata and secret version metadata in `daemon/internal/store/secrets.go`.
- [X] T018 Add tenant-aware store helpers for `provider_auth_states`, `connectors`, `mcp_servers`, `mcp_server_states`, and `mcp_tools` in `daemon/internal/store/tenancy/r37_resources.go`.
- [X] T019 Update R37 boundary signature expectations for the new `daemon/internal/secrets` surface in `daemon/internal/store/tenancy/testdata/r37_boundary_signatures.golden`.
- [X] T020 Add credential audit event construction helpers that reuse the tenant audit store without raw secret material in `daemon/internal/audit/credential.go`.
- [X] T021 Add hosted credential API schemas for tenant secret resources and rotation requests in `schemas/api/tenant-secret-resource.schema.json`, `schemas/api/tenant-secret-list.response.schema.json`, `schemas/api/create-tenant-secret.request.schema.json`, `schemas/api/rotate-tenant-secret.request.schema.json`, and `schemas/api/rotate-tenant-secret.response.schema.json`.
- [X] T022 Add credential audit event or tenant-audit schema updates with redacted fields in `schemas/events/credential-audit-recorded.event.schema.json` and `schemas/api/tenant-audit-event-resource.schema.json`.

**Checkpoint**: Foundation ready. Secret metadata can be stored, resolved through tenant context, audited, redacted, and guarded by stable denials.

---

## Phase 3: User Story 1 - Tenant Admin Manages Tenant-Owned Credentials (Priority: P1) MVP

**Goal**: Tenant administrators can create, rotate, disable, inspect redacted metadata,
connect, disconnect, and verify integration/provider credential ownership only inside the
active tenant. Viewers cannot inspect or mutate credential-bearing resources.

**Independent Test**: Create two tenants with same-shaped secrets, integration accounts,
and provider auth state. Sign in as tenant A admin and verify tenant A can manage only
tenant A material, tenant B is inaccessible, rotation is versioned, same external account
bindings are tenant-local, and viewer actions are denied without leaks.

### Tests for User Story 1

- [X] T023 [P] [US1] Add tenant secret create/list/metadata/disable API tests with redacted responses in `daemon/internal/api/hosted_credentials_test.go`.
- [X] T024 [P] [US1] Add secret rotation tests proving new resolutions use the new active version and already-started work keeps the prior version in `daemon/internal/secrets/manager_test.go`.
- [X] T025 [P] [US1] Add cross-tenant secret read, rotate, disable, and same `secretRef` isolation tests in `daemon/internal/secrets/manager_test.go`.
- [X] T026 [P] [US1] Add integration account same-human same-external-account tenant-local ownership tests in `daemon/internal/integrations/tenant_credentials_test.go`.
- [X] T027 [P] [US1] Add provider auth tenant ownership, no global fallback, expiry, refresh, revoke, and rotation lifecycle tests in `daemon/internal/providers/auth_tenant_test.go`.
- [X] T028 [P] [US1] Add viewer and unauthorized operator denial tests for secret, integration, and provider auth mutation paths in `daemon/internal/api/hosted_credentials_test.go`.
- [X] T029 [P] [US1] Add contract tests for tenant secret and provider auth schemas in `daemon/internal/contracts/hosted_credentials_contracts_test.go`.

### Implementation for User Story 1

- [X] T030 [US1] Register tenant secret administration routes and stable denial responses in `daemon/internal/api/server.go`.
- [X] T031 [US1] Implement tenant secret create/list/update metadata/rotate/disable handlers in `daemon/internal/api/hosted_credentials.go`.
- [X] T032 [US1] Wire tenant secret manager construction into daemon app startup in `daemon/internal/app/app.go`.
- [X] T033 [US1] Update integration manager account binding and readiness paths to use active tenant credential state in `daemon/internal/integrations/manager.go`.
- [X] T034 [US1] Update integration API connect/disconnect/readiness/default behavior to require active tenant permissions in `daemon/internal/api/integrations.go`.
- [X] T035 [US1] Update provider auth persistence and list/read/update paths to use tenant-aware helpers in `daemon/internal/store/secrets.go` and `daemon/internal/providers/manager.go`.
- [X] T036 [US1] Update managed provider bridge code so local provider auth metadata is redacted and tenant-local in `daemon/internal/managedproviders/bridges.go`.
- [X] T037 [US1] Update integration and provider auth schemas for tenant id, disabled reason, and redacted credential metadata in `schemas/api/integration-resource.schema.json`, `schemas/api/integration-list.response.schema.json`, and `schemas/api/provider-auth-state.response.schema.json`.
- [X] T038 [US1] Emit redacted credential audit records for secret create/update/rotate/disable and integration/provider auth lifecycle actions in `daemon/internal/audit/credential.go`.

**Checkpoint**: User Story 1 is independently functional and testable through `go test ./internal/secrets ./internal/api ./internal/integrations ./internal/providers ./internal/contracts`.

---

## Phase 4: User Story 2 - Runtime Resolves Credentials Only In The Active Tenant (Priority: P1)

**Goal**: Connector, MCP, provider, skill, and sandbox runtime paths resolve secret
references only through the active tenant, fail closed on missing or mismatched tenant
context, disable dependent MCP/connector uses after disconnect, and emit one successful
secret-use audit event per work item.

**Independent Test**: Create same-shaped connector, MCP, provider auth, skill secret, and
sandbox policy resources in tenants A and B. Invoke each from both tenants and verify the
active tenant's credential is the only value used, tenant mismatch fails with stable
errors, disconnect disables dependent uses, and repeated internal resolutions produce one
successful-use audit event per work item.

### Tests for User Story 2

- [X] T039 [P] [US2] Add MCP secret resolution tests proving `ResolveMCPSecrets` replacement resolves through active tenant and never process environment or another tenant in `daemon/internal/mcp/manager_test.go`.
- [X] T040 [P] [US2] Add MCP tool exposure and invocation cross-tenant denial tests for same server/tool names in `daemon/internal/api/mcp_server_test.go`.
- [X] T041 [P] [US2] Add connector tenant ownership, invoke denial, and disabled-after-disconnect tests in `daemon/internal/connectors/supervisor_test.go`.
- [X] T042 [P] [US2] Add sandbox preparation tests proving secret scope fails closed on missing, unauthorized, or mismatched tenant context in `daemon/internal/sandbox/manager_test.go`.
- [X] T043 [P] [US2] Add executable skill secret resolution tests proving `skill-secrets.json` bridge uses active tenant and redacts unavailable refs in `daemon/internal/skills/registry_test.go`.
- [X] T044 [P] [US2] Add provider runtime fallback tests proving expired, revoked, or disconnected tenant provider auth never falls back to global or other-tenant state in `daemon/internal/providers/manager_test.go`.
- [X] T045 [P] [US2] Add successful secret-use audit granularity tests for run, connector invocation, MCP invocation, and sandbox preparation in `daemon/internal/audit/credential_audit_test.go`.
- [X] T046 [P] [US2] Add replay and evaluation artifact redaction tests for credential-backed runtime work in `daemon/internal/evaluation/runtime_recorder_test.go`.

### Implementation for User Story 2

- [X] T047 [US2] Replace MCP file-based secret resolution call sites with tenant secret manager resolution in `daemon/internal/mcp/catalog.go` and `daemon/internal/mcp/manager.go`.
- [X] T048 [US2] Update MCP server, state, tool, and exposure store operations to require tenant context in `daemon/internal/store/tenancy/r37_resources.go`.
- [X] T049 [US2] Update MCP API handlers to pass tenant context and stable denials through all server, lifecycle, tool, and exposure routes in `daemon/internal/api/server.go`.
- [X] T050 [US2] Update connector supervisor and connector store persistence to carry tenant ownership and disabled dependent use state in `daemon/internal/connectors/supervisor.go` and `daemon/internal/store/store.go`.
- [X] T051 [US2] Update connector API handlers to require active tenant permissions and return redacted connector resources in `daemon/internal/api/server.go`.
- [X] T052 [US2] Update sandbox secret scope resolution to call the tenant secret manager and fail closed on missing tenant context in `daemon/internal/sandbox/manager.go`.
- [X] T053 [US2] Update executable skill secret resolution to use tenant secret manager during availability checks and execution preparation in `daemon/internal/skills/registry.go` and `daemon/internal/api/server.go`.
- [X] T054 [US2] Implement integration disconnect propagation that revokes provider auth, disables dependent connector and MCP uses, and preserves redacted config in `daemon/internal/integrations/manager.go`.
- [X] T055 [US2] Update MCP, connector, sandbox, and executable skill schemas for tenant ownership, disabled reason, and redacted secret summaries in `schemas/api/mcp-server-resource.schema.json`, `schemas/api/mcp-tool-resource.schema.json`, `schemas/api/connector-resource.schema.json`, `schemas/api/sandbox-profile.schema.json`, `schemas/api/sandbox-consumer-view.schema.json`, `schemas/api/skill-summary.schema.json`, `schemas/api/skill-detail.response.schema.json`, and `schemas/api/skill-registry.response.schema.json`.
- [X] T056 [US2] Emit one successful-use audit event per credential-bearing run, connector invocation, MCP invocation, or sandbox preparation in `daemon/internal/audit/credential.go`.

**Checkpoint**: User Story 2 is independently functional and testable through `go test ./internal/mcp ./internal/connectors ./internal/sandbox ./internal/skills ./internal/providers ./internal/audit ./internal/evaluation`.

---

## Phase 5: User Story 3 - Tenant-Scoped Operator Inspects Connector And MCP Ownership Safely (Priority: P2)

**Goal**: Tenant-scoped operators with `credentials.inspect` can inspect redacted
ownership, status, and non-secret metadata for connector, MCP, integration, provider
auth, and sandbox credential state only within tenants where the caller has
`credentials.inspect`. Viewers and unpermitted operators cannot inspect or mutate.

**Independent Test**: Provision connector and MCP resources for two tenants. Sign in as an
operator with tenant A `credentials.inspect` and verify tenant A ownership/status/
redacted references are visible, tenant B is not visible, mutation remains denied, audit
records are redacted, and no raw secret values appear in API responses, logs, events,
replay fixtures, or evaluation artifacts.

### Tests for User Story 3

- [X] T057 [P] [US3] Add `credentials.inspect` operator redacted inspection API tests for tenant secrets, integrations, provider auth, connectors, MCP, and sandbox secret policy in `daemon/internal/api/hosted_credentials_test.go`.
- [X] T058 [P] [US3] Add viewer and unpermitted operator denial tests proving `read_only.inspect` does not grant credential inspection or mutation in `daemon/internal/api/hosted_credentials_test.go`.
- [X] T059 [P] [US3] Add redaction log/event/API/replay/evaluation sentinel scan tests for operator inspection flows in `daemon/internal/contracts/r37_redaction_contracts_test.go`.
- [X] T060 [P] [US3] Add connector and MCP admin audit tests proving tenant, actor, resource kind, action, timestamp, and outcome are recorded without secret material in `daemon/internal/audit/credential_audit_test.go`.
- [X] T061 [P] [US3] Add API schema contract tests for redacted connector, MCP, tenant secret, provider auth, and tenant audit resources in `daemon/internal/contracts/hosted_credentials_contracts_test.go`.

### Implementation for User Story 3

- [X] T062 [US3] Add explicit `credentials.inspect` permission constants and evaluation helpers for tenant admins, tenant-scoped operators, and viewers in `daemon/internal/identity/permissions.go`.
- [X] T063 [US3] Update hosted credential API read paths to return redacted metadata for tenant-scoped operators with `credentials.inspect` and stable denials for viewers in `daemon/internal/api/hosted_credentials.go`.
- [X] T064 [US3] Update connector list/get response projection to include tenant ownership, status, disabled reason, and redacted secret summaries in `daemon/internal/api/server.go`.
- [X] T065 [US3] Update MCP list/get/tool response projection to include tenant ownership, status, disabled reason, and redacted auth summaries in `daemon/internal/mcp/manager.go`.
- [X] T066 [US3] Update sandbox explain and profile projections to include redacted secret scope outcomes only for tenants where the caller has `credentials.inspect` in `daemon/internal/sandbox/manager.go`.
- [X] T067 [US3] Update tenant audit listing or event projection to include credential audit records with redacted payload fields in `daemon/internal/api/tenants.go`.
- [X] T068 [US3] Update operator-facing schema docs for redacted credential inspection in `schemas/api/tenant-secret-resource.schema.json`, `schemas/api/connector-resource.schema.json`, `schemas/api/mcp-server-resource.schema.json`, and `schemas/api/tenant-audit-event-resource.schema.json`.

**Checkpoint**: User Story 3 is independently functional and testable through `go test ./internal/api ./internal/mcp ./internal/connectors ./internal/sandbox ./internal/audit ./internal/contracts`.

---

## Phase 6: User Story 4 - Existing Local Credential Configuration Upgrades Safely (Priority: P2)

**Goal**: Existing local secret files, provider auth state, integration bindings,
connector configuration, and MCP installs bridge into the default personal tenant without
printing values. Unsafe or ambiguous resources start disabled with redacted metadata and
require operator remediation before use.

**Independent Test**: Start from a pre-R37 fixture containing fake `mcp-secrets.json`,
`skill-secrets.json`, integration bindings, provider auth, connector config, and MCP
installs. Run upgrade or startup bridge and verify resources are owned by the default
personal tenant, fake values never appear in output, safe resources remain usable, unsafe
resources start disabled, and tenant B cannot use bridged state.

### Tests for User Story 4

- [X] T069 [P] [US4] Add migration fixture seeds for pre-R37 local MCP secrets, skill secrets, integration bindings, provider auth, connector config, and MCP installs in `daemon/internal/store/migrationfixture/r37_credentials.go`.
- [X] T070 [P] [US4] Add startup bridge test for safe local credential configuration into the default personal tenant in `daemon/internal/app/tenant_credentials_bridge_test.go`.
- [X] T071 [P] [US4] Add startup bridge test proving unsafe or ambiguous local credential state starts disabled with redacted metadata in `daemon/internal/app/tenant_credentials_bridge_test.go`.
- [X] T072 [P] [US4] Add no-print/no-log bridge redaction tests using `R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK` sentinels in `daemon/internal/app/tenant_credentials_bridge_test.go`.
- [X] T073 [P] [US4] Add post-bridge tenant B misuse tests for bridged MCP, skill, provider auth, connector, and integration binding state in `daemon/internal/app/tenant_credentials_bridge_test.go`.
- [X] T074 [P] [US4] Add restart idempotency tests proving the bridge does not duplicate tenant secret versions or re-enable unsafe resources in `daemon/internal/app/tenant_credentials_bridge_test.go`.

### Implementation for User Story 4

- [X] T075 [US4] Implement bridge scanner for `mcp-secrets.json`, `skill-secrets.json`, integration binding snapshots, local provider auth metadata, connector config, and MCP installs in `daemon/internal/secrets/bridge.go`.
- [X] T076 [US4] Wire the credential bridge into tenant migration startup after default personal tenant resolution in `daemon/internal/app/tenant_migration_startup.go`.
- [X] T077 [US4] Add bridge progress and idempotency persistence for local credential bridge steps in `daemon/internal/store/migration_progress.go`.
- [X] T078 [US4] Implement disabled bridged credential resource projection and remediation reason handling in `daemon/internal/secrets/manager.go`.
- [X] T079 [US4] Update managed provider bridge logic to create default-personal-tenant redacted provider auth state or disabled remediation state in `daemon/internal/managedproviders/bridges.go`.
- [X] T080 [US4] Update MCP catalog/server restore logic to associate bridged installs and secret refs with the default personal tenant in `daemon/internal/mcp/manager.go`.
- [X] T081 [US4] Update executable skill availability checks to recognize bridged default-personal-tenant secrets without reading raw values into logs in `daemon/internal/skills/registry.go`.
- [X] T082 [US4] Update integration binding restore/readiness logic to associate bridged bindings with the default personal tenant or disabled remediation state in `daemon/internal/integrations/manager.go` and `daemon/internal/api/integrations.go`.
- [X] T083 [US4] Document bridge behavior, unsafe remediation, backup-restore rollback, and no-log secret recovery constraints in `docs/runtime/tenant-credential-bridge.md`.

**Checkpoint**: User Story 4 is independently functional and testable through `go test ./internal/app ./internal/secrets ./internal/managedproviders ./internal/mcp ./internal/skills`.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final compatibility, documentation, generated artifacts, and verification
work across all stories.

- [X] T084 [P] Update hosted credential operator documentation for rotation, disconnect/reconnect, redaction, and test-environment smoke in `docs/runtime/hosted-credential-isolation.md`.
- [X] T085 [P] Update provider documentation for tenant-owned provider auth lifecycle and local bridge behavior in `docs/providers/provider-identity-and-profiles.md`.
- [X] T086 [P] Update MCP and sandbox documentation for tenant secret resolution and redacted unavailable states in `docs/harness/sandbox-execution-plane.md`.
- [X] T087 [P] Update `specs/022-hosted-secrets-isolation/quickstart.md` with final commands and any smoke-test changes discovered during implementation.
- [X] T088 [P] Update TypeScript SDK tenant secret resources and redacted response types for hosted credential APIs in `sdk/ts/src/`.
- [X] T089 [P] Run `make daemon-contract-test` from `/Users/John/Code/dope-agent` and fix schema or event contract drift in `schemas/api/`, `schemas/events/`, or `daemon/internal/contracts/`.
- [X] T090 [P] Run targeted R37 package tests from `/Users/John/Code/dope-agent/daemon`: `go test ./internal/secrets ./internal/store ./internal/store/tenancy ./internal/api ./internal/integrations ./internal/providers ./internal/mcp ./internal/connectors ./internal/sandbox ./internal/audit ./internal/contracts`.
- [X] T091 Run `go test ./...` from `/Users/John/Code/dope-agent/daemon` and fix failures in `daemon/internal/`.
- [X] T092 Run `go mod tidy` from `/Users/John/Code/dope-agent/daemon` after daemon-side changes and commit any legitimate `daemon/go.mod` or `daemon/go.sum` updates.
- [X] T093 Run `pnpm test:clients` from `/Users/John/Code/dope-agent` and fix hosted credential SDK or client regressions in `sdk/ts/`, `web/`, or `tui/`.
- [X] T094 Run `pnpm build` from `/Users/John/Code/dope-agent` and fix generated client or build failures in `sdk/ts/dist/`, `web/dist/`, or `tui/dist/`.
- [X] T095 Manually validate the fake two-tenant smoke flow in `specs/022-hosted-secrets-isolation/quickstart.md` against the test daemon, confirm the end-to-end smoke completes in under 15 minutes, and record elapsed time plus any unverified paths in the final implementation notes.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: No dependencies.
- **Phase 2 Foundational**: Depends on Phase 1 and blocks all user stories.
- **US1 (Phase 3)**: Depends on Phase 2 and is part of the P1 MVP.
- **US2 (Phase 4)**: Depends on Phase 2 and shares the P1 MVP; final validation expects US1 tenant secret administration and provider auth lifecycle to exist.
- **US3 (Phase 5)**: Depends on Phase 2 and can run after US1/US2 tests are written; inspection projections rely on the redacted resource shapes.
- **US4 (Phase 6)**: Depends on Phase 2 and can run in parallel with US3 once tenant secret storage and R37 boundary migration are in place.
- **Phase 7 Polish**: Depends on selected user stories being complete.

### User Story Dependencies

- **US1**: MVP credential administration and tenant-local integration/provider ownership; no dependency on US3/US4 after Phase 2.
- **US2**: Runtime credential isolation; depends on the shared secret manager and benefits from US1 provider/integration lifecycle but can write failing runtime tests independently.
- **US3**: Operator redacted inspection; depends on redacted schemas and tenant-owned resource projections from US1/US2.
- **US4**: Local credential bridge; depends on secret manager, store migration, and R37 resource ownership from Phase 2.

### Within Each User Story

- Write the story's test tasks before implementation tasks.
- Store/data model before service logic.
- Service logic before API handlers.
- API/schema changes before contract verification.
- Runtime resolution before smoke validation.
- Story checkpoint must pass before claiming that story complete.

---

## Parallel Opportunities

- T002-T005 can run in parallel.
- Foundational tests T006-T011 can run in parallel.
- US1 tests T023-T029 can run in parallel across secrets, integrations, providers, API, and contracts.
- US2 tests T039-T046 can run in parallel across MCP, connectors, sandbox, skills, providers, audit, and evaluation.
- US3 tests T057-T061 can run in parallel across API, contracts, and audit.
- US4 tests T069-T074 can run in parallel after the migration fixture shape is agreed.
- Polish documentation tasks T084-T087 can run in parallel.
- Verification tasks T089 and T090 can run in parallel once implementation is complete.

---

## Parallel Example: User Story 2

```text
Task: "Add MCP secret resolution tests proving active tenant resolution in daemon/internal/mcp/manager_test.go"
Task: "Add sandbox preparation tests proving missing or mismatched tenant fails closed in daemon/internal/sandbox/manager_test.go"
Task: "Add successful secret-use audit granularity tests in daemon/internal/audit/credential_audit_test.go"
Task: "Add replay and evaluation artifact redaction tests in daemon/internal/evaluation/runtime_recorder_test.go"
```

---

## Parallel Example: User Story 4

```text
Task: "Add startup bridge test for safe local credential configuration in daemon/internal/app/tenant_credentials_bridge_test.go"
Task: "Add startup bridge test proving unsafe local credential state starts disabled in daemon/internal/app/tenant_credentials_bridge_test.go"
Task: "Add post-bridge tenant B misuse tests for bridged integration binding state in daemon/internal/app/tenant_credentials_bridge_test.go"
Task: "Add no-print/no-log bridge redaction tests in daemon/internal/app/tenant_credentials_bridge_test.go"
Task: "Add restart idempotency tests for local credential bridge in daemon/internal/app/tenant_credentials_bridge_test.go"
```

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational.
3. Complete Phase 3: US1 tenant credential administration and integration/provider ownership.
4. Complete Phase 4: US2 active-tenant runtime credential resolution.
5. **STOP and VALIDATE**: run the US1/US2 targeted package tests, redaction contract tests, and the two-tenant fake runtime misuse checks.

### Incremental Delivery

1. Setup + Foundational -> shared secret storage, schema, redaction, denial, audit, and handoff verification.
2. US1 -> admin can manage tenant-owned credentials and provider/integration lifecycle safely.
3. US2 -> runtime paths cannot use another tenant's credentials.
4. US3 -> operators can inspect only permitted redacted ownership/status.
5. US4 -> existing local credential state bridges safely into default personal tenant.
6. Polish -> docs, contracts, full verification, smoke, and final implementation notes.

### Parallel Team Strategy

With multiple engineers:

1. Complete Phase 1 and Phase 2 together.
2. After Phase 2:
   - Engineer A: US1 secret/admin/provider/integration ownership.
   - Engineer B: US2 MCP/connector/sandbox/skill runtime resolution.
   - Engineer C: US3 redacted inspection/audit contracts.
   - Engineer D: US4 migration fixture and local bridge behavior.
3. Rejoin for Phase 7 verification because redaction, audit, and contract failures are cross-cutting.

---

## Notes

- `[P]` tasks touch different files and can run without waiting on incomplete tasks in the same phase.
- `[US#]` labels map to user stories in `spec.md`.
- Any task that changes API, event, schema, persistence, or execution boundaries must update matching schemas, fixtures, docs, and contract tests before completion.
- Use `~/.dope-test` and fake credentials for local validation. Do not touch production secrets or live connectors.
- Stop if any test, contract check, redaction scan, or smoke step exposes raw credential material.
