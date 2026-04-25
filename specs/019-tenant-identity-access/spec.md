# Feature Specification: Tenant Identity And Access Foundation

**Feature Branch**: `019-tenant-identity-access`  
**Created**: 2026-04-24  
**Status**: Draft  
**Input**: User description: "$speckit-specify 结合 docs/specs/019-tenant-identity-and-access-foundation.md 完成 phase 34 的工作"

## Clarifications

### Session 2026-04-24

- Q: What baseline sensitive permissions should each tenant role receive? → A: Tiered least privilege: owner all permissions; admin tenant/secrets/integrations/connectors/MCP/evaluation/billing; operator run/approval/live-validation execution; viewer read-only inspection.
- Q: How should security-relevant operations behave when required audit recording cannot complete? → A: Fail closed: deny security-relevant tenant switching, membership changes, and token lifecycle changes when required audit recording cannot complete.
- Q: What tenant grants should existing local tokens receive during personal-tenant bootstrap? → A: Existing tokens receive only the default personal tenant grant during bootstrap.
- Q: When must membership, principal, and token grant revocations take effect? → A: Next-check enforcement: changes apply to all authorization checks after the change is durably recorded; already-authorized in-flight work is not cancelled by this phase.
- Q: Should this phase support per-member permission overrides? → A: No per-member permission overrides in this phase; permissions derive only from the member role and current lifecycle state.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Resolve Tenant For Every Request (Priority: P1)

As an operator or hosted user, I need every request to have a clear tenant owner so resources are never read, changed, or audited under an ambiguous authority.

**Why this priority**: Tenant resolution is the foundation that later hosted product work depends on. Without a resolved tenant context, every domain feature risks inventing separate and inconsistent ownership rules.

**Independent Test**: Can be tested by sending requests as a principal with a default personal tenant, with an explicitly selected allowed tenant, and with an explicitly selected disallowed tenant, then verifying that each request either resolves to the expected tenant or receives the stable authorization denial.

**Acceptance Scenarios**:

1. **Given** an active principal with a default personal tenant, **When** the principal makes a request without selecting another tenant, **Then** the request is associated with the default personal tenant.
2. **Given** an active principal with access to more than one tenant, **When** the principal selects an allowed tenant for a request, **Then** the request is associated with the selected tenant.
3. **Given** an active principal without access to a selected tenant, **When** the principal attempts to use that tenant, **Then** the request is denied with a stable authorization error that does not reveal whether the tenant exists.

---

### User Story 2 - Manage Organization Memberships (Priority: P2)

As an organization owner or administrator, I need to invite users, assign roles, and remove access so organization tenants can delegate work without sharing global daemon authority.

**Why this priority**: Hosted organization usage requires a durable membership model before shared integrations, resources, and client workflows can safely land.

**Independent Test**: Can be tested by creating an organization tenant, inviting a principal, accepting or rejecting the invitation, assigning roles, removing membership, and verifying that access decisions and audit records reflect each state transition.

**Acceptance Scenarios**:

1. **Given** an organization tenant owned by an active principal, **When** the owner invites another principal with an organization role, **Then** the invitation is recorded and the invited principal has no tenant access until acceptance.
2. **Given** a pending organization invitation, **When** the invited principal accepts it, **Then** the principal becomes an active member with the assigned role and allowed tenant access.
3. **Given** an active organization member, **When** the member is removed, **Then** subsequent tenant access is denied and the removal is auditable.

---

### User Story 3 - Enforce Permission-Gated Capabilities (Priority: P3)

As a hosted administrator, I need sensitive actions to be allowed by explicit permissions, not broad role names alone, so high-impact operations can be governed consistently across tenants.

**Why this priority**: Role names are too coarse for hosted administration. A shared permission contract gives later roadmap phases one authorization model for secrets, integrations, runs, approvals, live validation, evaluation, and billing visibility.

**Independent Test**: Can be tested by assigning roles with known permissions, attempting sensitive actions as principals with and without those permissions, and verifying that allowed and denied outcomes match the permission contract.

**Acceptance Scenarios**:

1. **Given** a principal with a role that includes a sensitive capability, **When** the principal attempts that capability within an allowed tenant, **Then** the action is allowed and attributed to the resolved tenant.
2. **Given** a principal whose role does not include a sensitive capability, **When** the principal attempts that capability within an allowed tenant, **Then** the action is denied and the denial is auditable.
3. **Given** a disabled principal, revoked token, expired token, or removed membership, **When** any tenant-scoped capability is attempted, **Then** all tenant access is denied regardless of previous role or permission grants.

---

### User Story 4 - Control Token Tenant Grants (Priority: P4)

As an operator or API client owner, I need tokens to carry durable tenant grants, expiry, revocation, and rotation behavior so automated access can be limited and changed without widening authority by accident.

**Why this priority**: Long-lived or automated clients are common in hosted environments. Token lifecycle behavior must be enforced before tenant-scoped automation becomes available.

**Independent Test**: Can be tested by issuing a token with specific tenant grants, changing grants, rotating the token, revoking it, expiring it, and verifying that access decisions and audit records follow the token lifecycle.

**Acceptance Scenarios**:

1. **Given** a token with a default tenant and allowed tenant set, **When** the token is used for a permitted tenant, **Then** the request is resolved to that tenant.
2. **Given** a token whose grants are changed, **When** the token is used after the change, **Then** only the current allowed tenant set is honored.
3. **Given** a rotated token, **When** the previous token is used, **Then** it cannot gain access beyond the authority it had before rotation and must be denied if revoked or expired.

### Edge Cases

- Existing single-user installations with no explicit tenant records must receive or resolve an implicit default personal tenant without changing normal local-first workflows.
- Existing local tokens without tenant grants must receive only the bootstrapped default personal tenant grant; organization access must be granted explicitly later.
- A principal may be invited to an organization and later disabled before accepting; acceptance must not grant access while the principal is disabled.
- A membership may be removed while a token issued earlier still exists; the token must not continue to authorize access through the removed membership.
- A selected tenant may not exist or may be inaccessible to the principal; denial responses must not reveal which condition applies.
- Role changes and tenant-grant changes must be durable so restart does not restore old authority.
- Membership, principal, and token grant changes must affect every authorization check after the change is durably recorded, but this foundation does not cancel already-authorized in-flight work.
- Token rotation must not widen the old token's tenant access or preserve grants that were revoked before rotation.
- Audit recording failures must fail closed for security-relevant tenant switching, membership changes, and token lifecycle changes; the action must not complete without the required audit record.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support personal tenants and organization tenants as first-class ownership boundaries.
- **FR-002**: System MUST create or resolve a default personal tenant for existing local-first installations without requiring an organization selection.
- **FR-003**: System MUST limit existing local tokens created before tenant grants exist to the bootstrapped default personal tenant until explicit additional tenant grants are issued.
- **FR-004**: System MUST persist tenants, principals, memberships, principal lifecycle state, token tenant grants, and token lifecycle state durably across restart.
- **FR-005**: System MUST associate every accepted inbound request with both a principal and a tenant, or deny the request with a stable authorization error before tenant-owned resources are accessed.
- **FR-006**: System MUST allow an active principal to use the default tenant when no explicit tenant is selected.
- **FR-007**: System MUST allow an active principal to select a non-default tenant only when the principal and token are both allowed to access that tenant.
- **FR-008**: System MUST reject requests for tenants outside the principal's allowed tenant set, outside the token's allowed tenant set, or outside the current membership grants.
- **FR-009**: System MUST prevent denial responses from revealing whether an inaccessible tenant exists.
- **FR-010**: System MUST expose stable tenant and membership inspection surfaces sufficient for operators and clients to list allowed tenants, default tenant, membership roles, membership state, and pending invitations.
- **FR-011**: System MUST support organization invitation, acceptance, rejection, role assignment, membership removal, and membership state inspection through auditable state transitions.
- **FR-012**: System MUST define tenant roles `owner`, `admin`, `operator`, and `viewer` with tiered least-privilege membership behavior: owners receive all tenant permissions; admins receive tenant, secrets, integrations, connectors, MCP, evaluation, and billing-view permissions; operators receive run execution, approval resolution, and live-validation execution permissions; viewers receive read-only inspection access only.
- **FR-013**: System MUST evaluate sensitive capabilities through a shared permission model instead of relying only on broad role names.
- **FR-014**: System MUST derive membership permissions only from the member role and current lifecycle state in this phase; per-member permission overrides are out of scope.
- **FR-015**: System MUST include permission coverage for `tenant.manage`, `secrets.manage`, `integrations.manage`, `connectors.manage`, `mcp.manage`, `runs.execute`, `approvals.resolve`, `live_validation.execute`, `evaluation.manage`, and `billing.view`.
- **FR-016**: System MUST deny all tenant access for disabled principals, removed principals, removed memberships, revoked tokens, expired tokens, and tokens without the requested tenant grant.
- **FR-017**: System MUST support token issue, expiry, revocation, rotation, default tenant assignment, and tenant-grant changes without widening authority unexpectedly.
- **FR-018**: System MUST audit tenant switching, denied tenant access, membership creation and changes, invitation decisions, token issue, token rotation, token revocation, expiry-based denial, and tenant-grant changes.
- **FR-019**: System MUST deny security-relevant tenant switching, membership changes, and token lifecycle changes when required audit recording cannot complete.
- **FR-020**: System MUST apply membership, principal lifecycle, token revocation, token expiry, and tenant-grant changes to every authorization check after the change is durably recorded; cancellation of already-authorized in-flight work is out of scope for this foundation.
- **FR-021**: System MUST keep tenant identity and permission behavior additive for existing local-first clients wherever possible.
- **FR-022**: System MUST provide contract coverage for tenant, principal, membership, role, permission, invitation, token grant, token lifecycle, and denial representations.

### Key Entities *(include if feature involves data)*

- **Tenant**: An ownership boundary for resources. A tenant is either personal or organization-scoped and has lifecycle metadata, display identity, and membership relationships.
- **Principal**: A user or client identity that can authenticate to the system. A principal has lifecycle state, one default tenant, and an allowed tenant set derived from memberships and token grants.
- **Membership**: A relationship between a principal and an organization tenant, including role, invitation state, active or removed state, and audit history.
- **Role**: A named membership assignment: owner, admin, operator, or viewer. Roles provide tiered least-privilege default permission bundles but do not replace explicit sensitive capability checks.
- **Permission**: A capability gate for sensitive actions such as tenant management, secrets management, integration management, run execution, approval resolution, live validation, evaluation management, and billing visibility.
- **Token Tenant Grant**: The tenant authority attached to an access token, including default tenant, allowed tenants, issue state, expiry, revocation, rotation lineage, and grant-change history.
- **Audit Event**: A durable record of security-relevant tenant, membership, permission, denial, and token lifecycle activity.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds tenant, principal, membership, permission, token grant, denial, and audit contract surfaces. Existing local-first behavior remains supported through a default personal tenant, and contract changes should be additive wherever possible.
- **Migration / Rollback**: Requires a bootstrap path for existing single-user installations to receive or resolve a personal tenant and constrain existing local tokens to that default personal tenant. Rollback must preserve existing local-first operation and must not leave principals or tokens with broader tenant authority than they had before rollback.
- **Verification Strategy**: Requires tenant resolution tests, default tenant behavior tests, explicit tenant selection tests, denial tests, role and permission evaluation tests, principal lifecycle tests, token lifecycle tests, invite and membership tests, contract fixtures, and restart coverage for memberships and tenant grants.
- **Observability Impact**: Must add audit coverage for tenant switching, denied tenant access, membership changes, invitation decisions, token issue, token rotation, token revocation, expiry-based denial, and tenant-grant changes. Denial observability must help operators debug access problems without leaking inaccessible tenant existence to callers.
- **Environment & Secrets**: Development and verification must use the test environment by default. The feature does not require live connectors or new operator secrets; any future hosted secret or billing integration remains out of scope for this foundation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of accepted tenant-scoped requests have both a resolved principal and resolved tenant before tenant-owned resources are accessed.
- **SC-002**: 100% of requests for disallowed, removed, disabled, revoked, or expired access are denied with a stable authorization outcome that does not reveal inaccessible tenant existence.
- **SC-003**: Existing single-user local installations can continue their primary workflows through a default personal tenant with no manual organization setup.
- **SC-004**: Existing local tokens created before tenant grants exist authorize only the bootstrapped default personal tenant in migration verification.
- **SC-005**: Organization owners can invite, activate, update, and remove a member in a complete audited workflow, and the resulting access state is reflected immediately in authorization decisions.
- **SC-006**: Permission checks cover all listed sensitive capabilities, and the tiered least-privilege role-to-permission baseline can be verified independently for owner, admin, operator, and viewer roles.
- **SC-007**: Permission verification confirms that two active members with the same role and lifecycle state receive the same sensitive permissions in this phase.
- **SC-008**: Token issue, expiry, revocation, rotation, and tenant-grant changes remain enforced after restart in all covered lifecycle tests.
- **SC-009**: Contract fixtures describe every externally visible tenant, principal, membership, invitation, permission, token grant, lifecycle, audit, and denial representation introduced by the feature.
- **SC-010**: Security-relevant tenant switching, membership changes, and token lifecycle changes cannot complete in verification scenarios where required audit recording fails.
- **SC-011**: After a membership, principal lifecycle, token revocation, token expiry, or tenant-grant change is durably recorded, subsequent authorization checks reflect the new access state in verification.

## Assumptions

- Roadmap 34 scope is limited to the tenant identity and access foundation described in `docs/specs/019-tenant-identity-and-access-foundation.md`.
- Migrating every existing domain resource to tenant scope is out of scope for this feature; later roadmaps will consume the shared tenant context and permission model.
- Cancelling already-authorized in-flight tenant-scoped work after revocation is out of scope for this foundation and may be specified by later domain-specific roadmaps.
- Billing enforcement, quotas, per-tenant storage backends, and full tenant-switcher UI are out of scope.
- Per-member permission overrides are out of scope; membership permissions are role-derived for this phase.
- Local-first deployments remain supported and must not require live connectors or managed provider access.
- Sensitive capability names listed in the upstream document are the initial permission set for this feature.
- "Removed principal" means a principal no longer has tenant access; durable identity history may remain available for audit purposes.
