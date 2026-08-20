# Data Model: Tenant Identity And Access Foundation

## Tenant

Represents a resource ownership boundary.

Fields:

- `tenantId`: stable identifier, unique.
- `tenantKind`: `personal` or `organization`.
- `displayName`: operator-visible label.
- `status`: `active` or `disabled`.
- `createdAt`, `updatedAt`: audit timestamps.
- `createdByPrincipalId`: principal that created the tenant when known.
- `defaultOwnerPrincipalId`: owner for personal tenants and initial owner for
  organization tenants.

Relationships:

- One tenant has many memberships.
- One personal tenant belongs to exactly one default principal.
- One organization tenant has one or more owner memberships.
- Tenant audit events may reference a tenant.

Validation rules:

- A personal tenant must have exactly one default owner principal.
- An organization tenant must have at least one active owner after creation and accepted
  membership changes.
- Disabled tenants deny new tenant-scoped access.

## Principal

Represents a user or client identity that can authenticate and receive tenant access.

Fields:

- `principalId`: stable identifier, unique.
- `principalKind`: `local_operator`, `user`, or `service_client`.
- `displayName`: operator-visible label.
- `status`: `invited`, `active`, `disabled`, or `removed`.
- `defaultTenantId`: tenant selected when no explicit tenant is requested.
- `createdAt`, `updatedAt`: audit timestamps.
- `disabledAt`, `removedAt`: lifecycle timestamps when applicable.

Relationships:

- One principal has one default tenant.
- One principal may have many memberships.
- One principal may own many access tokens.
- Principal lifecycle changes are referenced by audit events.

State transitions:

- `invited -> active`: invitation accepted or local bootstrap completed.
- `invited -> removed`: invitation rejected or withdrawn.
- `active -> disabled`: administrative disable.
- `disabled -> active`: administrative re-enable if still a member of allowed tenants.
- `active|disabled -> removed`: identity no longer has tenant access.

Validation rules:

- Disabled and removed principals cannot access any tenant.
- Removed principals remain referenceable for audit history.
- A principal default tenant must be in the principal's current allowed tenant set.

## Membership

Represents a principal's relationship to a tenant.

Fields:

- `membershipId`: stable identifier, unique.
- `tenantId`: tenant relationship.
- `principalId`: member identity.
- `role`: `owner`, `admin`, `operator`, or `viewer`.
- `status`: `invited`, `active`, or `removed`.
- `invitationId`: invitation that created the pending membership when applicable.
- `createdAt`, `updatedAt`: audit timestamps.
- `acceptedAt`, `removedAt`: lifecycle timestamps when applicable.

Relationships:

- Belongs to one tenant and one principal.
- May be created from one invitation.
- Produces audit events for role, status, and removal changes.

State transitions:

- `invited -> active`: invited principal accepts.
- `invited -> removed`: invited principal rejects or invitation is revoked.
- `active -> removed`: owner/admin removes membership.
- Role may change only while membership is active.

Validation rules:

- Active membership requires active principal and active tenant.
- Removed membership denies access immediately on subsequent authorization checks.
- Owner removal or role downgrade must not leave an organization tenant with zero active
  owners.
- Membership permissions derive only from role and lifecycle state in this phase.

## Tenant Invitation

Represents an auditable organization invite.

Fields:

- `invitationId`: stable identifier, unique.
- `tenantId`: organization tenant receiving the invite.
- `invitedPrincipalId`: principal invited to join.
- `invitedByPrincipalId`: actor that created the invite.
- `role`: role to grant on acceptance.
- `status`: `pending`, `accepted`, `rejected`, `revoked`, or `expired`.
- `createdAt`, `updatedAt`, `expiresAt`: lifecycle timestamps.
- `decidedAt`: timestamp for accepted/rejected/revoked decisions.

Relationships:

- Belongs to one organization tenant.
- Creates or references one invited membership.
- Produces audit events for create, accept, reject, revoke, and expiry decisions.

Validation rules:

- Invitations are only valid for organization tenants.
- Invitation acceptance is denied when the invited principal is disabled or removed.
- Accepted invitations create or activate the membership with the invited role.
- Rejected, revoked, or expired invitations do not grant tenant access.

## Role

Represents a named membership bundle.

Values:

- `owner`
- `admin`
- `operator`
- `viewer`

Role-to-permission baseline:

- Owner: all permissions.
- Admin: `tenant.manage`, `secrets.manage`, `integrations.manage`,
  `connectors.manage`, `mcp.manage`, `evaluation.manage`, `billing.view`.
- Operator: `runs.execute`, `approvals.resolve`, `live_validation.execute`.
- Viewer: read-only inspection access only.

Validation rules:

- Role bundles are static in this phase.
- Per-member permission overrides are out of scope.
- Sensitive capability checks must still evaluate explicit permissions, not only role
  names.

## Permission

Represents a sensitive capability gate.

Values:

- `tenant.manage`
- `secrets.manage`
- `integrations.manage`
- `connectors.manage`
- `mcp.manage`
- `runs.execute`
- `approvals.resolve`
- `live_validation.execute`
- `evaluation.manage`
- `billing.view`

Relationships:

- Derived from active membership role and lifecycle state.
- Evaluated against the resolved tenant context.

Validation rules:

- Disabled or removed principals receive no permissions.
- Removed memberships receive no permissions.
- Revoked or expired tokens receive no permissions.
- A token without the resolved tenant grant receives no permissions for that tenant.

## Access Token

Represents an authenticated bearer credential.

Fields:

- `tokenId`: stable identifier, unique.
- `principalId`: owner principal.
- `label`: operator-visible label.
- `mode`: existing pairing mode.
- `tokenHash`: persisted token hash, never raw token material.
- `tokenPreview`: short display prefix.
- `status`: `active`, `revoked`, `expired`, or `rotated`.
- `defaultTenantId`: tenant selected when token caller does not explicitly select one.
- `createdAt`, `updatedAt`, `lastUsedAt`: lifecycle timestamps.
- `expiresAt`: optional expiry timestamp.
- `revokedAt`: revocation timestamp when applicable.
- `rotatedFromTokenId`, `rotatedToTokenId`: rotation lineage.

Relationships:

- Belongs to one principal.
- Has many token tenant grants.
- Token lifecycle changes produce audit events.

State transitions:

- `active -> revoked`: administrative revocation.
- `active -> expired`: current time passes `expiresAt`.
- `active -> rotated`: rotation issues a replacement token and closes old token use.

Validation rules:

- Raw token material is returned only at issue or rotation completion.
- Expired, revoked, and rotated tokens cannot authorize tenant access.
- Existing local tokens created before tenant grants exist receive only the bootstrapped
  default personal tenant grant.
- Rotation must not widen the old token's allowed tenant set.

## Token Tenant Grant

Represents tenant authority attached to one access token.

Fields:

- `grantId`: stable identifier, unique.
- `tokenId`: token receiving the grant.
- `tenantId`: tenant granted.
- `isDefault`: whether this grant is the token default tenant.
- `status`: `active` or `revoked`.
- `createdAt`, `updatedAt`, `revokedAt`: lifecycle timestamps.
- `grantedByPrincipalId`: actor for audit.

Relationships:

- Belongs to one token and one tenant.
- Produces audit events for create, revoke, default change, and grant changes.

Validation rules:

- A token can have at most one active default tenant grant.
- Requested tenant access requires an active grant for the token and active access for the
  token's principal.
- Grant revocation affects every authorization check after the revocation is durably
  recorded.

## Tenant Context

Represents the resolved request authority.

Fields:

- `principalId`: authenticated principal.
- `tokenId`: authenticated token.
- `tenantId`: resolved tenant.
- `tenantSource`: `default` or `explicit_header`.
- `permissions`: computed sensitive permissions for the principal within the tenant.
- `resolvedAt`: timestamp.

Validation rules:

- Accepted protected requests must have tenant context.
- `X-Kura-Tenant-ID` may override default tenant only when principal and token grant allow
  the tenant.
- Denied tenant resolution must use the stable authorization denial without existence
  leakage.

## Tenant Audit Event

Represents durable security-relevant evidence.

Fields:

- `auditEventId`: stable identifier, unique.
- `eventKind`: tenant switch, denied tenant access, membership change, invitation
  decision, token issue, token rotation, token revocation, token expiry denial,
  tenant-grant change, or audit failure denial.
- `tenantId`: tenant context when safe to record.
- `principalId`: actor principal when known.
- `targetPrincipalId`: affected principal when applicable.
- `tokenId`: affected token when applicable.
- `outcome`: `succeeded`, `denied`, or `failed_closed`.
- `reasonCode`: stable operator-facing reason.
- `createdAt`: audit timestamp.
- `document`: redacted detail payload.

Validation rules:

- Audit payloads must not include raw token material or secrets.
- Tenant access denials must not leak whether the selected tenant exists.
- Security-relevant tenant switching, membership changes, and token lifecycle changes must
  fail closed when required audit recording cannot complete.
