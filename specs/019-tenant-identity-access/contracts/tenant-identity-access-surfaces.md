# Contract Surfaces: Tenant Identity And Access Foundation

## Goal

Add schema-backed tenant identity and access surfaces so every accepted protected daemon
request can resolve a principal and tenant context, while denied requests receive stable
authorization responses that do not leak inaccessible tenant existence.

## Request Tenant Selection

### Header: `X-Dope-Tenant-ID`

- Purpose: Select a non-default tenant for a protected request.
- Required behavior:
  - omitted header resolves the token/principal default tenant
  - present header is accepted only when the authenticated principal and token grant allow
    the tenant
  - unknown, inaccessible, disabled, removed, revoked, and expired authority all produce
    the same stable authorization denial shape
  - accepted protected requests attach resolved `principalId`, `tokenId`, `tenantId`, and
    permission context before handler execution

## Tenant And Principal Routes

### `GET /v1/auth/me`

- Purpose: Extend the current authenticated identity response with tenant context.
- Response additions:
  - `principal`
  - `defaultTenant`
  - `currentTenant`
  - `allowedTenants`
  - `tokenGrants`
  - `permissions`
- Compatibility rule: existing auth token fields remain additive and backward-compatible.

Schema surfaces:

- update `schemas/api/auth-access-token-resource.schema.json`
- add `schemas/api/auth-me.response.schema.json` if not already schema-backed
- add `schemas/api/tenant-context-resource.schema.json`

### `GET /v1/tenants`

- Purpose: List tenants the authenticated principal can inspect.
- Query parameters:
  - `tenantKind`
  - `status`
  - `limit`
- Response requirements:
  - ordered `items`
  - each item includes `tenantId`, `tenantKind`, `displayName`, `status`, role summary,
    default marker, and timestamps

Schema surfaces:

- add `schemas/api/tenant-resource.schema.json`
- add `schemas/api/tenant-list.response.schema.json`

### `POST /v1/tenants`

- Purpose: Create an organization tenant.
- Permission: `tenant.manage` on the caller's resolved current tenant. Local bootstrap
  administration uses the owner role on the caller's default personal tenant; this phase
  does not introduce a separate authority model.
- Request requirements:
  - `tenantKind` must be `organization`
  - `displayName`
- Response requirements:
  - tenant resource
  - owner membership resource for the creator
  - audit event reference

Schema surfaces:

- add `schemas/api/create-tenant.request.schema.json`
- add `schemas/api/create-tenant.response.schema.json`

### `GET /v1/tenants/{tenantId}`

- Purpose: Inspect one allowed tenant.
- Response requirements:
  - tenant resource
  - caller membership role
  - caller permissions

Schema surfaces:

- add `schemas/api/tenant-detail.response.schema.json`

### `GET /v1/principals`

- Purpose: List principals visible for tenant administration.
- Query parameters:
  - `tenantId`
  - `status`
  - `limit`
- Permission: callers with `tenant.manage` on the requested tenant are allowed to list
  principals visible to that tenant. Callers without `tenant.manage` receive only their
  own principal record when `tenantId` is omitted or matches their resolved current
  tenant; otherwise the request is denied with the stable authorization denial shape.

Schema surfaces:

- add `schemas/api/principal-resource.schema.json`
- add `schemas/api/principal-list.response.schema.json`

### `PATCH /v1/principals/{principalId}`

- Purpose: Disable, re-enable, or remove a principal.
- Permission: `tenant.manage`.
- Request requirements:
  - `status`: `active`, `disabled`, or `removed`
  - optional `reason`
- Response requirements:
  - updated principal resource
  - audit event reference for lifecycle changes
- Truthfulness rule:
  - disabled and removed principals lose tenant access on the next authorization check

Schema surfaces:

- add `schemas/api/update-principal.request.schema.json`
- add `schemas/api/update-principal.response.schema.json`

## Membership And Invitation Routes

### `GET /v1/tenants/{tenantId}/memberships`

- Purpose: Inspect tenant memberships.
- Permission: current tenant members are allowed to list non-sensitive membership
  resources for the resolved tenant. Fields that expose lifecycle reason text,
  invitation internals, or audit-only detail are not returned by this route; tenant
  administration and audit routes that expose those details require `tenant.manage`.
- Query parameters:
  - `status`
  - `role`
  - `limit`

Schema surfaces:

- add `schemas/api/membership-resource.schema.json`
- add `schemas/api/membership-list.response.schema.json`

### `POST /v1/tenants/{tenantId}/invitations`

- Purpose: Invite a principal to an organization tenant.
- Permission: `tenant.manage`.
- Request requirements:
  - `principalId`
  - `role`: `owner`, `admin`, `operator`, or `viewer`
  - optional `expiresAt`
- Response requirements:
  - invitation resource
  - pending membership resource
  - audit event reference
- Fail-closed rule:
  - if required audit recording fails, the invite is denied and no membership access is
    granted

Schema surfaces:

- add `schemas/api/create-tenant-invitation.request.schema.json`
- add `schemas/api/tenant-invitation-resource.schema.json`
- add `schemas/api/create-tenant-invitation.response.schema.json`

### `GET /v1/tenant-invitations`

- Purpose: List pending and historical invitations visible to the caller.
- Query parameters:
  - `tenantId`
  - `status`
  - `limit`

Schema surfaces:

- add `schemas/api/tenant-invitation-list.response.schema.json`

### `POST /v1/tenant-invitations/{invitationId}/accept`

- Purpose: Accept a pending invitation.
- Request requirements:
  - optional `reason`
- Response requirements:
  - invitation resource
  - active membership resource
  - updated allowed tenant set
- Denial rules:
  - disabled or removed invited principal cannot accept
  - expired, rejected, revoked, or already accepted invitations cannot grant access

Schema surfaces:

- add `schemas/api/accept-tenant-invitation.request.schema.json`
- add `schemas/api/tenant-invitation-decision.response.schema.json`

### `POST /v1/tenant-invitations/{invitationId}/reject`

- Purpose: Reject a pending invitation.
- Request requirements:
  - optional `reason`
- Response requirements:
  - invitation resource
  - removed pending membership when one existed
  - audit event reference

Schema surfaces:

- add `schemas/api/reject-tenant-invitation.request.schema.json`

### `PATCH /v1/tenants/{tenantId}/memberships/{membershipId}`

- Purpose: Update an active membership role.
- Permission: `tenant.manage`.
- Request requirements:
  - `role`
  - optional `reason`
- Validation:
  - update must not leave an organization tenant with zero active owners
  - permissions remain role-derived with no per-member overrides

Schema surfaces:

- add `schemas/api/update-membership.request.schema.json`

### `DELETE /v1/tenants/{tenantId}/memberships/{membershipId}`

- Purpose: Remove tenant membership.
- Permission: `tenant.manage`.
- Response requirements:
  - removed membership resource
  - audit event reference
- Validation:
  - removal must not leave an organization tenant with zero active owners
  - removed memberships deny access on subsequent authorization checks

## Token Lifecycle And Grant Routes

### `GET /v1/auth/tokens`

- Purpose: List tokens visible to the authenticated principal or tenant administrator.
- Query parameters:
  - `principalId`
  - `status`
  - `tenantId`
  - `limit`

Schema surfaces:

- update `schemas/api/auth-access-token-resource.schema.json`
- add `schemas/api/auth-token-list.response.schema.json`
- add `schemas/api/token-tenant-grant-resource.schema.json`

### `POST /v1/auth/tokens`

- Purpose: Issue a new token with explicit tenant grants.
- Request requirements:
  - `label`
  - optional `expiresAt`
  - `defaultTenantId`
  - `allowedTenantIds`
- Response requirements:
  - token resource
  - raw `accessToken` returned once
  - grant resources

Schema surfaces:

- add `schemas/api/create-auth-token.request.schema.json`
- add `schemas/api/create-auth-token.response.schema.json`

### `POST /v1/auth/tokens/{tokenId}/rotate`

- Purpose: Rotate a token without widening the old token's allowed tenant set.
- Request requirements:
  - optional `expiresAt`
  - optional `reason`
- Response requirements:
  - old token resource with rotated lifecycle state
  - new token resource
  - raw replacement `accessToken` returned once
  - audit event reference

Schema surfaces:

- add `schemas/api/rotate-auth-token.request.schema.json`
- add `schemas/api/rotate-auth-token.response.schema.json`

### `POST /v1/auth/tokens/{tokenId}/revoke`

- Purpose: Revoke a token.
- Request requirements:
  - optional `reason`
- Response requirements:
  - token resource with revoked lifecycle state
  - audit event reference

Schema surfaces:

- add `schemas/api/revoke-auth-token.request.schema.json`

### `PATCH /v1/auth/tokens/{tokenId}/tenant-grants`

- Purpose: Replace token default tenant and allowed tenant set.
- Request requirements:
  - `defaultTenantId`
  - `allowedTenantIds`
  - optional `reason`
- Validation:
  - caller must be allowed to manage the affected tenant grants
  - token grants cannot include tenants outside the token owner's allowed tenant set
  - old grants stop authorizing access on the next authorization check after durable write

Schema surfaces:

- add `schemas/api/update-token-tenant-grants.request.schema.json`
- add `schemas/api/update-token-tenant-grants.response.schema.json`

## Permission And Audit Routes

### `GET /v1/tenants/{tenantId}/permissions`

- Purpose: Inspect effective permission baseline for the caller in a tenant.
- Response requirements:
  - `tenantId`
  - `principalId`
  - `membershipId`
  - `role`
  - `permissions`
  - lifecycle state inputs used to compute permission

Schema surfaces:

- add `schemas/api/tenant-permission-resource.schema.json`

### `GET /v1/tenant-audit-events`

- Purpose: Inspect tenant identity and access audit history.
- Query parameters:
  - `tenantId`
  - `principalId`
  - `tokenId`
  - `eventKind`
  - `outcome`
  - `limit`
- Response requirements:
  - ordered `items`
  - redacted audit event resources
- Security rule:
  - audit event detail must not reveal tenant existence to callers who cannot inspect the
    tenant

Schema surfaces:

- add `schemas/api/tenant-audit-event-resource.schema.json`
- add `schemas/api/tenant-audit-event-list.response.schema.json`

## Denial Contract

Stable authorization denials must use one externally visible response shape for
inaccessible and unknown tenants.

Response requirements:

- HTTP status: `403` for authenticated callers lacking tenant access.
- JSON body:
  - `error`
  - `errorCode`: stable value such as `tenant_access_denied`
  - optional `requestId` if the daemon has request identifiers available.
- The body must not include tenant existence, tenant display name, or membership details.

Schema surfaces:

- add `schemas/api/error-response.schema.json` if the repository does not already have a
  reusable error schema.

## Event Surfaces

Persisted event and audit contracts must cover:

- `tenant.context_resolved` for explicit tenant switching that succeeds.
- `tenant.access_denied` for denied tenant selection or lifecycle denial.
- `tenant.membership_changed` for role update and membership removal.
- `tenant.invitation_created`
- `tenant.invitation_accepted`
- `tenant.invitation_rejected`
- `tenant.invitation_revoked`
- `tenant.invitation_expired`
- `tenant.token_issued`
- `tenant.token_rotated`
- `tenant.token_revoked`
- `tenant.token_expiry_denied`
- `tenant.token_grants_changed`
- `tenant.audit_failed_closed`

Schema surfaces:

- add event schemas under `schemas/events/` for every emitted event name.
- add contract tests in `daemon/internal/contracts` validating representative persisted
  event payloads.

## Reused Existing Routes

Existing protected routes keep their current response shapes unless later roadmaps add
tenant-owned resource fields. For roadmap 34, they gain only the protected-route tenant
resolution precondition:

- `/v1/config`
- `/v1/operator/*`
- `/v1/events`
- `/v1/evaluation/*`
- `/v1/runs*`
- `/v1/schedules*`
- `/v1/reminders*`
- `/v1/delivery*`
- `/v1/sessions*`
- `/v1/policy/approvals*`
- `/v1/llm/dispatches*`
- `/v1/chat/query*`
- `/v1/skills*`
- `/v1/sandboxes*`
- `/v1/mcp*`
- `/v1/integrations*`
- `/v1/provider*`
- `/v1/capabilities*`
- `/v1/connectors*`
- `/v1/calendar*`
- `/v1/mail*`

## Deferred Client Ergonomics

The later tenant-aware operator shell and SDK roadmap owns:

- web tenant switcher UI
- operator projection refetch after tenant switch
- SDK default tenant option
- SDK per-request tenant override helpers

Roadmap 34 may add typed SDK route methods only if implementation needs immediate client
contract coverage, but it should not turn those helpers into a full tenant switching
experience.
