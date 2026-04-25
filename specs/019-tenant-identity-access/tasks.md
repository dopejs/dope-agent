# Tasks: Tenant Identity And Access Foundation

**Input**: Design documents from `/specs/019-tenant-identity-access/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Required. The specification and constitution require targeted unit, API,
store, contract, restart, and migration coverage for this production change.

**Organization**: Tasks are grouped by user story to enable independent implementation and
testing. Complete Phase 1 and Phase 2 before beginning any user story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it touches different files and does not depend on
  incomplete tasks in the same phase.
- **[Story]**: User story label for story phases only.
- Every task includes exact file paths.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the identity package and initial contract files without changing behavior.

- [X] T001 Create identity package skeleton files in daemon/internal/identity/types.go, daemon/internal/identity/permissions.go, daemon/internal/identity/resolver.go, daemon/internal/identity/audit.go, and daemon/internal/identity/manager.go
- [X] T002 [P] Create identity unit test skeleton files in daemon/internal/identity/permissions_test.go, daemon/internal/identity/resolver_test.go, daemon/internal/identity/audit_test.go, and daemon/internal/identity/manager_test.go
- [X] T003 [P] Create API test skeleton files for tenant identity behavior in daemon/internal/api/tenant_identity_test.go and daemon/internal/api/tenant_auth_test.go
- [X] T004 [P] Create store test skeleton file for tenant identity persistence in daemon/internal/store/identity_test.go
- [X] T005 [P] Create contract test skeleton file for tenant identity contracts in daemon/internal/contracts/tenant_identity_contracts_test.go
- [X] T006 [P] Create initial tenant identity schema files under schemas/api/tenant-resource.schema.json and schemas/events/tenant-access-denied.event.schema.json

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add shared identity types, persistence, and restore/bootstrap infrastructure required by all user stories.

**Critical**: No user story work can begin until this phase is complete.

- [X] T007 Define Tenant, Principal, Membership, TenantInvitation, TokenTenantGrant, TenantContext, TenantAuditEvent, lifecycle enum, role enum, permission enum, and stable denial types in daemon/internal/identity/types.go
- [X] T008 Implement role-derived permission bundles and no per-member override behavior in daemon/internal/identity/permissions.go
- [X] T009 [P] Add unit tests for owner/admin/operator/viewer permission bundles and lifecycle-denied permissions in daemon/internal/identity/permissions_test.go
- [X] T010 Add identity persistence record structs, filters, and store method signatures in daemon/internal/store/store.go
- [X] T011 Add the next SQLite schema migration for tenants, principals, memberships, tenant_invitations, token_tenant_grants, token lifecycle metadata, and tenant_audit_events in daemon/internal/store/store.go
- [X] T012 Add store CRUD/list methods for tenants, principals, memberships, invitations, token grants, and tenant audit events in daemon/internal/store/store.go
- [X] T013 Add store migration and restart persistence tests for identity tables and token grant records in daemon/internal/store/identity_test.go
- [X] T014 Extend auth.AccessToken with principalId, status, expiresAt, revokedAt, rotatedFromTokenId, rotatedToTokenId, and defaultTenantId fields in daemon/internal/auth/auth.go
- [X] T015 Update auth token store read/write scans for lifecycle and tenant fields in daemon/internal/store/store.go
- [X] T016 Update auth unit and store tests for backward-compatible token restore with new lifecycle fields in daemon/internal/auth/auth_test.go and daemon/internal/store/store_test.go
- [X] T017 Add identity manager construction and persisted identity restore wiring to daemon/internal/app/app.go
- [X] T018 Add app restart tests proving identity state restores before serving protected routes in daemon/internal/app/app_test.go

**Checkpoint**: Identity package compiles, persistence is migrated, and restored state is available to API wiring.

---

## Phase 3: User Story 1 - Resolve Tenant For Every Request (Priority: P1) MVP

**Goal**: Every accepted protected request has principal and tenant context, or fails with a stable authorization denial before tenant-owned resource access.

**Independent Test**: Send authenticated protected requests with no tenant header, an allowed tenant header, and a disallowed tenant header; verify default resolution, explicit resolution, and denial without tenant existence leakage.

### Tests for User Story 1

- [X] T019 [P] [US1] Add resolver unit tests for default tenant selection, explicit X-Dope-Tenant-ID selection, disabled principal denial, removed membership denial, and no existence leakage in daemon/internal/identity/resolver_test.go
- [X] T020 [P] [US1] Add audit unit tests for fail-closed tenant switch and denied access audit writes in daemon/internal/identity/audit_test.go
- [X] T021 [P] [US1] Add API tests for /v1/auth/me tenant context, allowed and disallowed /v1/tenants/{tenantId} inspection, and protected route tenant resolution in daemon/internal/api/tenant_auth_test.go
- [X] T022 [P] [US1] Add contract tests for auth/me, tenant context, tenant list, tenant detail, and stable error response schemas in daemon/internal/contracts/tenant_identity_contracts_test.go
- [X] T023 [P] [US1] Add bootstrap and request-resolution restart test for default personal tenant and pre-tenant local token grant in daemon/internal/app/app_test.go

### Implementation for User Story 1

- [X] T024 [US1] Implement personal tenant and active principal bootstrap for local-first installations in daemon/internal/identity/manager.go
- [X] T025 [US1] Implement token grant limitation for existing local tokens during bootstrap in daemon/internal/identity/manager.go
- [X] T026 [US1] Implement tenant resolver for default tenant and X-Dope-Tenant-ID override rules in daemon/internal/identity/resolver.go
- [X] T027 [US1] Implement tenant audit writer and fail-closed tenant switch denial behavior in daemon/internal/identity/audit.go
- [X] T028 [US1] Add identity manager dependency and tenant context middleware to protected routes in daemon/internal/api/server.go
- [X] T029 [US1] Add context helpers for resolved TenantContext in daemon/internal/api/types.go
- [X] T030 [US1] Extend /v1/auth/me response with principal, defaultTenant, currentTenant, allowedTenants, tokenGrants, and permissions in daemon/internal/api/server.go
- [X] T031 [US1] Add /v1/tenants and /v1/tenants/{tenantId} inspection routes in daemon/internal/api/tenants.go
- [X] T032 [US1] Add tenant, tenant list, tenant detail, tenant context, auth/me, auth access token update, token grant, and stable error response schemas in schemas/api/tenant-resource.schema.json, schemas/api/tenant-list.response.schema.json, schemas/api/tenant-detail.response.schema.json, schemas/api/tenant-context-resource.schema.json, schemas/api/auth-me.response.schema.json, schemas/api/auth-access-token-resource.schema.json, schemas/api/token-tenant-grant-resource.schema.json, and schemas/api/error-response.schema.json
- [X] T033 [US1] Add tenant access denied and tenant context resolved event schemas in schemas/events/tenant-access-denied.event.schema.json and schemas/events/tenant-context-resolved.event.schema.json
- [X] T034 [US1] Run gofmt on daemon/internal/identity, daemon/internal/api, daemon/internal/app, and daemon/internal/store packages

**Checkpoint**: User Story 1 is independently functional and can be validated before organization membership, permission-gated capabilities, or token lifecycle routes are complete.

---

## Phase 4: User Story 2 - Manage Organization Memberships (Priority: P2)

**Goal**: Organization owners/admins can create organization tenants, invite principals, accept/reject invitations, update roles, remove memberships, and see durable audit records.

**Independent Test**: Create an organization tenant, invite a principal, accept/reject the invitation, change a role, remove a member, restart the daemon, and verify access and audit state match each transition.

### Tests for User Story 2

- [X] T035 [P] [US2] Add identity manager tests for organization creation, owner invariant, invite accept/reject/revoke/expire, role update, membership removal, and fail-closed denial when required membership or invitation audit writes fail in daemon/internal/identity/manager_test.go
- [X] T036 [P] [US2] Add API tests for tenant creation, principal list including tenant.manage and self-only authorization behavior, principal lifecycle update response, membership list redaction behavior, invitation list, invitation accept/reject, membership update, membership removal, fail-closed membership/invitation audit write failures, and tenant audit event list inspection in daemon/internal/api/tenant_identity_test.go
- [X] T037 [P] [US2] Add contract tests for create tenant, tenant, principal, principal lifecycle update response, membership, invitation, invitation decision, invitation revoked/expired event, membership update, and tenant audit event schemas in daemon/internal/contracts/tenant_identity_contracts_test.go
- [X] T038 [P] [US2] Add store restart tests for organization memberships and invitations in daemon/internal/store/identity_test.go

### Implementation for User Story 2

- [X] T039 [US2] Implement organization tenant creation and owner membership creation in daemon/internal/identity/manager.go
- [X] T040 [US2] Implement invitation create, accept, reject, revoke, and expire state transitions in daemon/internal/identity/manager.go
- [X] T041 [US2] Implement role update and membership removal with last-owner protection in daemon/internal/identity/manager.go
- [X] T042 [US2] Add tenant creation, membership with non-sensitive list redaction, invitation, and invitation decision handlers in daemon/internal/api/tenants.go, principal list with self-only fallback and lifecycle handlers with updated principal plus audit reference responses in daemon/internal/api/principals.go, and tenant audit event list handler in daemon/internal/api/tenant_audit.go
- [X] T043 [US2] Register /v1/tenants/{tenantId}/memberships, /v1/tenants/{tenantId}/invitations, /v1/tenant-invitations, /v1/principals, /v1/principals/{principalId}, and /v1/tenant-audit-events routes in daemon/internal/api/server.go
- [X] T044 [US2] Add create tenant, membership, invitation, principal, principal lifecycle update, and tenant audit event schemas in schemas/api/create-tenant.request.schema.json, schemas/api/create-tenant.response.schema.json, schemas/api/membership-resource.schema.json, schemas/api/membership-list.response.schema.json, schemas/api/create-tenant-invitation.request.schema.json, schemas/api/tenant-invitation-resource.schema.json, schemas/api/create-tenant-invitation.response.schema.json, schemas/api/tenant-invitation-list.response.schema.json, schemas/api/accept-tenant-invitation.request.schema.json, schemas/api/reject-tenant-invitation.request.schema.json, schemas/api/tenant-invitation-decision.response.schema.json, schemas/api/update-membership.request.schema.json, schemas/api/principal-resource.schema.json, schemas/api/principal-list.response.schema.json, schemas/api/update-principal.request.schema.json, schemas/api/update-principal.response.schema.json, schemas/api/tenant-audit-event-resource.schema.json, and schemas/api/tenant-audit-event-list.response.schema.json
- [X] T045 [US2] Add membership changed, invitation created, invitation accepted, invitation rejected, invitation revoked, invitation expired, and audit failed closed event schemas in schemas/events/tenant-membership-changed.event.schema.json, schemas/events/tenant-invitation-created.event.schema.json, schemas/events/tenant-invitation-accepted.event.schema.json, schemas/events/tenant-invitation-rejected.event.schema.json, schemas/events/tenant-invitation-revoked.event.schema.json, schemas/events/tenant-invitation-expired.event.schema.json, and schemas/events/tenant-audit-failed-closed.event.schema.json
- [X] T046 [US2] Add audit event persistence and fail-closed denial behavior for organization creation, invitation decisions, invitation revoke/expire, role updates, and membership removals when required audit recording fails in daemon/internal/identity/audit.go

**Checkpoint**: User Story 2 is independently functional on top of the resolved tenant context from US1.

---

## Phase 5: User Story 3 - Enforce Permission-Gated Capabilities (Priority: P3)

**Goal**: Sensitive capabilities are evaluated through shared permission checks derived from role and lifecycle state.

**Independent Test**: Assign owner/admin/operator/viewer roles, attempt representative sensitive capabilities, and verify allowed/denied outcomes plus audit records match the role-derived permission baseline.

### Tests for User Story 3

- [X] T047 [P] [US3] Add permission evaluation tests for tenant.manage, secrets.manage, integrations.manage, connectors.manage, mcp.manage, runs.execute, approvals.resolve, live_validation.execute, evaluation.manage, and billing.view in daemon/internal/identity/permissions_test.go
- [X] T048 [P] [US3] Add API tests proving viewer, operator, admin, owner, disabled principal, removed membership, and revoked token permission outcomes in daemon/internal/api/tenant_identity_test.go
- [X] T049 [P] [US3] Add contract tests for tenant permission resource schema in daemon/internal/contracts/tenant_identity_contracts_test.go

### Implementation for User Story 3

- [X] T050 [US3] Implement permission evaluator API for role-derived sensitive capabilities in daemon/internal/identity/permissions.go
- [X] T051 [US3] Add RequirePermission helper that reads TenantContext and returns stable tenant authorization denials in daemon/internal/api/types.go
- [X] T052 [US3] Gate tenant creation, membership changes, principal lifecycle updates, and token grant management with tenant.manage in daemon/internal/api/tenants.go, daemon/internal/api/principals.go, and daemon/internal/api/auth_tokens.go
- [X] T053 [US3] Add GET /v1/tenants/{tenantId}/permissions handler in daemon/internal/api/tenants.go
- [X] T054 [US3] Add tenant permission resource schema in schemas/api/tenant-permission-resource.schema.json
- [X] T055 [US3] Add permission denial audit records for sensitive capability denial in daemon/internal/identity/audit.go

**Checkpoint**: User Story 3 is independently testable by role and lifecycle state without per-member permission overrides.

---

## Phase 6: User Story 4 - Control Token Tenant Grants (Priority: P4)

**Goal**: Operators and API client owners can issue, rotate, revoke, expire, and update tenant grants for tokens without widening authority accidentally.

**Independent Test**: Issue a token with specific tenant grants, change grants, rotate, revoke, expire, restart, and verify subsequent authorization checks and audit records reflect current token state.

### Tests for User Story 4

- [X] T056 [P] [US4] Add auth manager tests for token issue, expiry, revocation, rotation lineage, no raw token persistence, and fail-closed denial when required token lifecycle audit writes fail in daemon/internal/auth/auth_test.go
- [X] T057 [P] [US4] Add identity manager tests for token default tenant grants, grant replacement, old grant denial, and rotation no-widening behavior in daemon/internal/identity/manager_test.go
- [X] T058 [P] [US4] Add API tests for /v1/auth/tokens list, issue, rotate, revoke, tenant grant update routes, and fail-closed token lifecycle audit write failures in daemon/internal/api/tenant_auth_test.go
- [X] T059 [P] [US4] Add contract tests for token list, create token, rotate token, revoke token, token grant update, and token grant resource schemas in daemon/internal/contracts/tenant_identity_contracts_test.go
- [X] T060 [P] [US4] Add restart tests for token expiry, revocation, rotation lineage, and tenant grants in daemon/internal/app/app_test.go

### Implementation for User Story 4

- [X] T061 [US4] Implement token issue, expiry, revocation, rotation, and grant-change logic in daemon/internal/auth/auth.go
- [X] T062 [US4] Implement token grant validation against principal allowed tenants in daemon/internal/identity/manager.go
- [X] T063 [US4] Add token lifecycle and tenant grant route handlers in daemon/internal/api/auth_tokens.go
- [X] T064 [US4] Register /v1/auth/tokens routes in daemon/internal/api/server.go
- [X] T065 [US4] Add auth access token lifecycle and grant schemas in schemas/api/auth-access-token-resource.schema.json, schemas/api/auth-token-list.response.schema.json, schemas/api/create-auth-token.request.schema.json, schemas/api/create-auth-token.response.schema.json, schemas/api/rotate-auth-token.request.schema.json, schemas/api/rotate-auth-token.response.schema.json, schemas/api/revoke-auth-token.request.schema.json, schemas/api/update-token-tenant-grants.request.schema.json, and schemas/api/update-token-tenant-grants.response.schema.json
- [X] T066 [US4] Add token issued, token rotated, token revoked, token expiry denied, and token grants changed event schemas in schemas/events/tenant-token-issued.event.schema.json, schemas/events/tenant-token-rotated.event.schema.json, schemas/events/tenant-token-revoked.event.schema.json, schemas/events/tenant-token-expiry-denied.event.schema.json, and schemas/events/tenant-token-grants-changed.event.schema.json
- [X] T067 [US4] Add token lifecycle audit persistence, expiry-based denial audit behavior, and fail-closed denial for token issue, rotate, revoke, and grant-change operations when required audit recording fails in daemon/internal/identity/audit.go
- [X] T068 [US4] Ensure Authenticate rejects expired, revoked, and rotated tokens before tenant resolution in daemon/internal/auth/auth.go

**Checkpoint**: User Story 4 is independently functional and restart-safe on top of shared tenant resolution.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Align contracts, docs, quickstart, migration notes, and full verification across all user stories.

- [X] T069 [P] Update operator trust model for tenant identity, grants, audit, denial, and local-first bootstrap behavior in docs/runtime/operator-trust-model.md
- [X] T070 [P] Update daemon API and event model docs for tenant routes, token routes, tenant header behavior, and audit events in docs/runtime/daemon-api-and-event-model.md
- [X] T071 [P] Update migration versioning docs if the current schema version examples are stale after the new migration in docs/runtime/migration-versioning.md
- [X] T072 [P] Update roadmap 34 status and verification notes in docs/runtime/daemon-roadmaps.md and docs/specs/019-tenant-identity-and-access-foundation.md
- [X] T073 [P] Update quickstart verification commands and expected tenant responses in specs/019-tenant-identity-access/quickstart.md
- [X] T074 Add performance verification for tenant resolution bounded store lookup behavior and low-hundreds tenant or membership list responses in daemon/internal/identity/resolver_test.go and daemon/internal/api/tenant_identity_test.go
- [X] T075 Run make daemon-contract-test from repository root using Makefile
- [X] T076 Run cd daemon && go test ./... for daemon/go.mod
- [X] T077 Run cd daemon && go mod tidy for daemon/go.mod and daemon/go.sum
- [X] T078 Run pnpm test:clients using package.json only if sdk/ts, web, or tui files changed during implementation

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup**: no dependencies.
- **Phase 2 Foundational**: depends on Phase 1 and blocks all user stories.
- **Phase 3 US1**: depends on Phase 2 and is the MVP.
- **Phase 4 US2**: depends on Phase 2, and uses US1 tenant context for end-to-end API tests.
- **Phase 5 US3**: depends on Phase 2, and uses US1 tenant context plus US2 memberships for role coverage.
- **Phase 6 US4**: depends on Phase 2, and uses US1 tenant context for route enforcement.
- **Phase 7 Polish**: depends on all implemented story phases.

### User Story Dependencies

- **US1 Resolve Tenant For Every Request**: can start after Phase 2 and should complete first for MVP validation.
- **US2 Manage Organization Memberships**: can start after Phase 2, but final API validation assumes US1 middleware exists.
- **US3 Enforce Permission-Gated Capabilities**: can start after Phase 2, but useful end-to-end tests need US2 memberships.
- **US4 Control Token Tenant Grants**: can start after Phase 2, but route tests need US1 middleware.

### Within Each User Story

- Write tests before implementation tasks for that story.
- Store and domain behavior before API handlers.
- API handlers before schema contract finalization.
- Contract tests before final verification.
- Each story checkpoint must pass its independent test before moving to lower-priority work.

## Parallel Opportunities

- Setup tasks T002-T006 can run in parallel after T001.
- Foundational tests T009, T013, T016, and T018 can be prepared in parallel while implementation tasks land in order.
- US1 tests T019-T023 can be written in parallel.
- US2 tests T035-T038 can be written in parallel.
- US3 tests T047-T049 can be written in parallel.
- US4 tests T056-T060 can be written in parallel.
- Documentation tasks T069-T073 can run in parallel after the implemented contract surfaces are known.

## Parallel Example: User Story 1

```text
Task: "T019 [P] [US1] Add resolver unit tests in daemon/internal/identity/resolver_test.go"
Task: "T020 [P] [US1] Add audit unit tests in daemon/internal/identity/audit_test.go"
Task: "T021 [P] [US1] Add API tests in daemon/internal/api/tenant_auth_test.go"
Task: "T022 [P] [US1] Add contract tests in daemon/internal/contracts/tenant_identity_contracts_test.go"
Task: "T023 [P] [US1] Add restart test in daemon/internal/app/app_test.go"
```

## Parallel Example: User Story 2

```text
Task: "T035 [P] [US2] Add identity manager tests in daemon/internal/identity/manager_test.go"
Task: "T036 [P] [US2] Add API tests in daemon/internal/api/tenant_identity_test.go"
Task: "T037 [P] [US2] Add contract tests in daemon/internal/contracts/tenant_identity_contracts_test.go"
Task: "T038 [P] [US2] Add store restart tests in daemon/internal/store/identity_test.go"
```

## Parallel Example: User Story 4

```text
Task: "T056 [P] [US4] Add auth manager tests in daemon/internal/auth/auth_test.go"
Task: "T057 [P] [US4] Add identity manager tests in daemon/internal/identity/manager_test.go"
Task: "T058 [P] [US4] Add API tests in daemon/internal/api/tenant_auth_test.go"
Task: "T059 [P] [US4] Add contract tests in daemon/internal/contracts/tenant_identity_contracts_test.go"
Task: "T060 [P] [US4] Add restart tests in daemon/internal/app/app_test.go"
```

## Implementation Strategy

### MVP First

1. Complete Phase 1 setup.
2. Complete Phase 2 foundation.
3. Complete Phase 3 US1.
4. Validate default tenant bootstrap, explicit tenant selection, stable denial, contract fixtures, and restart behavior.
5. Stop before lower-priority stories if the MVP needs review.

### Incremental Delivery

1. US1 establishes the required tenant context invariant for protected requests.
2. US2 adds organization membership lifecycle and auditable delegated access.
3. US3 adds the shared permission gate for sensitive capabilities.
4. US4 adds durable token lifecycle and grant management.
5. Polish updates docs and runs full verification.

### Release Verification

Before considering roadmap 34 complete:

1. All story checkpoints are satisfied.
2. `make daemon-contract-test` passes.
3. `cd daemon && go test ./...` passes.
4. `cd daemon && go mod tidy` leaves no unexpected module diff.
5. `pnpm test:clients` is run if client files changed.
6. Rollback and migration notes are documented.
