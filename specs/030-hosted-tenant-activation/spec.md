# Feature Specification: Hosted Signup And Tenant Activation

**Feature Branch**: `030-hosted-tenant-activation`  
**Created**: 2026-05-06  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/030-hosted-signup-and-tenant-activation.md 完成 phase 45 的工作"

**Upstream authority**: `docs/specs/030-hosted-signup-and-tenant-activation.md` is the authoritative upstream document for this work (Roadmap 45). This specification translates that document into testable scenarios, requirements, and success criteria. Where the upstream document and this spec disagree, the upstream document wins and this spec must be updated.

## Clarifications

### Session 2026-05-06

- Q: Which safe first action must satisfy v1 hosted activation? → A: Test chat.
- Q: How should activation behave when quota baseline is unavailable? → A: Block activation completion until quota baseline is available.
- Q: Which hosted users are eligible for personal tenant activation? → A: Any authenticated hosted user unless disabled or denied.
- Q: What test chat data may activation audit and diagnostics retain? → A: Test chat metadata only.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Activate A Personal Tenant From Hosted Entry (Priority: P1)

As a new hosted user, I can sign up or accept an invitation, confirm that I have access, and land in an active personal tenant without manual setup.

**Why this priority**: A public hosted product cannot be usable if a newly authenticated user does not have a clear tenant context and must rely on developer-only calls or operator intervention.

**Independent Test**: Can be fully tested by starting from a user with no active hosted setup, completing signup or invite acceptance, and confirming the user reaches an active personal tenant with access confirmed.

**Acceptance Scenarios**:

1. **Given** an eligible new hosted user has no existing personal tenant, **When** the user completes signup or accepts an invite, **Then** the system creates or resolves a personal tenant and shows it as active.
2. **Given** an eligible hosted user already has a personal tenant, **When** the user returns through signup or invite acceptance, **Then** the system resolves the existing personal tenant rather than creating a duplicate.
3. **Given** an authenticated hosted user is not disabled or denied, **When** the user attempts activation, **Then** the user is eligible for personal tenant activation.
4. **Given** the user is disabled or denied, **When** the user attempts activation, **Then** the user sees a stable denied or blocked state and operators can identify the reason without inspecting storage directly.

---

### User Story 2 - Understand Hosted Readiness And Next Steps (Priority: P1)

As a newly activated hosted user, I can see the active tenant, environment, quota baseline, readiness state, and the next safe activation actions so I know what to do first.

**Why this priority**: The first-run surface must reduce uncertainty and prevent users from confusing test, hosted, organization, and personal contexts.

**Independent Test**: Can be fully tested by opening the first-run hosted shell after activation and verifying that tenant, environment, quota baseline, readiness checks, and next actions are visible and consistent.

**Acceptance Scenarios**:

1. **Given** a newly activated user is in a personal tenant, **When** the first-run surface loads, **Then** the active tenant, hosted environment, quota baseline, readiness state, and available next actions are visible.
2. **Given** readiness checks are incomplete or blocked, including missing quota baseline, **When** the first-run surface loads, **Then** the user sees the blocked readiness state and a recovery or retry path rather than a generic failure.
3. **Given** the user has access to an organization tenant through an invitation, **When** the first-run surface loads, **Then** personal activation remains complete and organization onboarding appears only as an additive next step.

---

### User Story 3 - Complete A Safe First Action (Priority: P1)

As a new hosted user, I can complete one useful action that does not require live connector credentials or production secrets.

**Why this priority**: Activation is not complete until the user can experience a useful product outcome inside a safe tenant boundary.

**Independent Test**: Can be fully tested by choosing the test chat action from the activation surface and confirming that it completes under the active personal tenant without live connector setup.

**Acceptance Scenarios**:

1. **Given** the user has an active personal tenant and no live connector credentials, **When** the user chooses the test chat first action, **Then** the action completes successfully under the personal tenant.
2. **Given** a safe first action fails because required hosted readiness is unavailable, **When** the user views the result, **Then** the failure is tied to a stable reason and the activation state remains recoverable.
3. **Given** a first action completes, **When** the user reloads or the daemon restarts, **Then** the completed activation state remains visible without retaining the test chat transcript.

---

### User Story 4 - Diagnose Activation Failures (Priority: P2)

As an operator, I can inspect activation failures by user and tenant context using stable reason codes, audit records, and safe diagnostics without querying storage by hand or exposing secrets.

**Why this priority**: Public hosted signup will create support and operations load; failures must be diagnosable without ad hoc state inspection.

**Independent Test**: Can be fully tested by inducing representative activation failures and confirming that operator-facing diagnostics show the tenant scope, activation state, reason, and retry or remediation expectation without leaking credential material.

**Acceptance Scenarios**:

1. **Given** activation fails during tenant resolution, readiness checks, quota projection, or first action completion, **When** an operator reviews activation diagnostics, **Then** the failure includes a stable reason, affected tenant scope when available, and remediation guidance.
2. **Given** activation is retried after a recoverable failure, **When** the retry succeeds, **Then** the audit trail preserves the failed attempt and the successful completion.
3. **Given** activation diagnostics are generated, **When** an operator reviews them, **Then** raw secrets, tokens, inaccessible tenant details, and test chat message content are absent.

### Edge Cases

- A returning user already has a personal tenant; activation must resolve it idempotently rather than creating duplicates.
- Signup and invitation acceptance happen more than once or concurrently for the same user; the user must end with one active personal tenant and one coherent activation state.
- The user has an organization invitation but no personal tenant; organization onboarding must not block personal activation.
- The user is disabled or denied after authentication; activation must show a stable blocked state with a diagnosable reason.
- Tenant access is revoked during first-run activation; tenant-scoped views and actions must stop using the revoked tenant.
- Quota or plan projection is unavailable; activation completion must remain blocked until quota baseline is available.
- Hosted readiness checks are degraded, stale, or partially unavailable; the surface must distinguish blocked, retryable, and completed states.
- No live connectors, production secrets, or external provider credentials are configured; the test chat first action must remain available.
- The test chat first action fails after partial progress; activation must preserve tenant scope, support retry, and avoid marking completion incorrectly.
- The daemon restarts after tenant activation but before first action completion; activation state must remain durable and resumable.
- Activation diagnostics encounter credential-bearing values or test chat message content; diagnostics must suppress those values and retain only test chat metadata.
- Activation flow must not introduce memory, context recall, or personalized knowledge behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose the current activation state for the resolved hosted user and tenant.
- **FR-002**: The system MUST create or resolve exactly one default personal tenant for each eligible new hosted user.
- **FR-002a**: Any authenticated hosted user MUST be eligible for personal tenant activation unless the user is disabled or denied.
- **FR-003**: Signup and invitation acceptance MUST land eligible users in an active personal tenant without requiring developer-only calls, manual state edits, or operator-run setup.
- **FR-004**: Activation MUST be idempotent for returning users and repeated signup or invitation acceptance attempts.
- **FR-005**: The first-run hosted surface MUST show the active tenant, hosted environment, quota baseline, readiness state, and remaining activation actions before the user performs tenant-scoped work.
- **FR-006**: The quota and plan projection shown during activation MUST use safe defaults for new personal tenants, and activation completion MUST remain blocked while quota baseline is unavailable.
- **FR-007**: Activation MUST offer test chat as the required v1 safe first action, and it MUST complete only after required readiness checks, including quota baseline, pass without live connectors, production secrets, payment checkout, or external organization setup.
- **FR-008**: The test chat first action MUST run under the active personal tenant and MUST NOT use tenantless state or inaccessible organization tenant state.
- **FR-009**: Activation progress and first-action completion MUST remain durable across reloads and daemon restarts.
- **FR-010**: Activation failures MUST expose stable user-visible and operator-visible reason codes for tenant resolution, eligibility, readiness, quota projection, authorization, first-action execution, and unexpected failure classes.
- **FR-011**: Activation MUST be tenant-scoped and audit-visible, including who activated, which tenant was activated, when activation changed state, and which first action completed or failed.
- **FR-012**: Operator diagnostics MUST allow activation failures to be diagnosed without direct storage inspection and without exposing raw secrets, tokens, or credential-bearing values.
- **FR-012a**: Activation audit records and diagnostics for test chat MUST retain completion and failure metadata only and MUST NOT retain test chat transcripts or message content.
- **FR-013**: Organization onboarding MUST remain additive and MUST NOT block personal tenant activation or the safe first action.
- **FR-014**: If the user's active tenant access is revoked during activation, the activation surface MUST stop showing tenant-scoped data for that tenant and require a valid allowed tenant before continuing.
- **FR-015**: Client-facing activation state MUST be available consistently to the hosted shell and automated clients that need to display or react to first-run activation state.
- **FR-016**: Activation MUST preserve compatibility with existing tenant identity, token grant, tenant-aware shell, quota, and hosted operational profile behavior.
- **FR-017**: The feature MUST NOT introduce enterprise SSO, payment checkout, organization administration, memory, context recall, or personalized knowledge behavior.

### Key Entities *(include if feature involves data)*

- **Hosted User**: A person using the hosted product through signup or invitation acceptance.
- **Personal Tenant**: The default tenant every hosted user receives for personal use and first-run activation.
- **Activation State**: The user's current first-run state for a tenant, including not started, in progress, blocked, active, first action completed, and failure information.
- **Readiness Check**: A user- and operator-visible condition that determines whether activation can continue safely, such as tenant access, environment availability, quota baseline, and first-action availability.
- **Quota Baseline**: The default plan and usage projection shown to a new personal tenant before paid checkout or organization administration exists.
- **Safe First Action**: The v1 test chat task that can complete without live connectors, production secrets, payment checkout, or organization setup.
- **Activation Failure Reason**: A stable reason class used by users, clients, tests, and operators to understand blocked or failed activation states.
- **Activation Audit Record**: Tenant-scoped metadata-only evidence of activation state transitions, failed attempts, retries, and completed first actions.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: This feature adds hosted activation behavior, activation-state projection, first-run shell behavior, client-facing activation representation, and tenant-scoped audit expectations. Existing tenant identity, token grant, tenant-aware shell, quota, and hosted operational profile behavior must remain compatible.
- **Migration / Rollback**: Existing hosted users and tenants must be resolved without duplicate personal tenants. Rollback should disable the guided activation surface and safe first-action workflow while preserving existing tenant identity and audit records. Activation records already created must remain readable for support and review.
- **Verification Strategy**: Required validation includes first-run signup and invite flows, idempotent personal tenant resolution, activation state projection, quota baseline projection, tenant isolation, failure reason coverage, audit visibility, restart durability, client representation coverage, and a manual `KURA_ENV=test` walkthrough from no active setup to first useful action.
- **Observability Impact**: The feature must add or update operator-visible activation diagnostics, stable failure reasons, audit records, and metadata-only first-action completion evidence so activation failures can be investigated without direct storage inspection.
- **Environment & Secrets**: Development and automated validation must default to the test environment. Activation must not require live connectors, production secrets, payment provider credentials, enterprise identity credentials, or privileged organization setup. Diagnostics and audit evidence must avoid raw secrets and credential-bearing values.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of eligible new hosted users in first-run tests reach an active personal tenant from signup or invitation acceptance without manual setup.
- **SC-001a**: 100% of covered authenticated hosted users who are not disabled or denied are treated as eligible for personal tenant activation.
- **SC-002**: 100% of repeated or concurrent activation attempts for the same hosted user resolve to one personal tenant and one coherent activation state.
- **SC-003**: A new hosted user can identify the active tenant, hosted environment, quota baseline, readiness state, and next action in 30 seconds or less during first-run review.
- **SC-004**: The test chat first action completes successfully after quota baseline is available and without live connectors, production secrets, payment checkout, or organization setup in 100% of covered happy-path activation tests.
- **SC-004a**: In 100% of covered missing-quota scenarios, activation completion remains blocked and the user sees a retryable quota readiness reason.
- **SC-005**: Restart durability tests show that 100% of completed personal tenant activations and first-action results remain visible after daemon restart.
- **SC-006**: Tenant isolation tests show zero cross-tenant activation state, quota baseline, audit, or first-action result leakage across covered personal and organization tenant scenarios.
- **SC-007**: 100% of covered activation failure classes produce stable reason codes and operator-visible diagnostics without direct storage inspection.
- **SC-008**: Redaction validation finds zero raw secrets, tokens, credential-bearing environment values, authorization headers, app secrets, refresh credentials, test chat transcripts, or test chat message content in activation diagnostics and audit evidence.
- **SC-009**: Operators can identify the failing activation stage and remediation class in 10 minutes or less for representative tenant resolution, eligibility, readiness, quota, authorization, and first-action failures.
- **SC-010**: Personal activation remains available in 100% of covered organization-invite scenarios where organization onboarding is incomplete or unavailable.

## Assumptions

- Roadmap 45 consumes the tenant identity and access foundation from Roadmap 34, tenant-aware shell and SDK behavior from Roadmap 36, and hosted operational profile behavior from Roadmap 43.
- Every authenticated hosted user has or can receive a personal tenant unless disabled or denied; organization tenants are additional contexts and are not required for personal activation.
- The v1 safe first action is test chat; reminder and provider setup can remain available as additional next actions but do not satisfy the required v1 activation completion signal.
- Hosted activation is product behavior and must be visible from the hosted shell; it is not a developer-only runbook.
- Default quota and plan projection for new users can be represented before payment checkout exists, but checkout and billing administration remain out of scope.
- Enterprise SSO, organization administration, memory, context recall, and personalized knowledge behavior remain out of scope for this phase.
- Test verification uses `KURA_ENV=test` behavior by default and does not touch production user data.
