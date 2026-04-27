# Data Model: Tenant-Aware Operator Shell And SDK

Roadmap 36 introduces no daemon persistence migration. The entities below are client and
operator-surface data models that bind existing Roadmap 34 tenant identity resources to
SDK and shell behavior.

## Tenant

Represents an ownership boundary available to a user.

Fields:

- `tenantId`: stable tenant identifier.
- `tenantKind`: `personal` or `organization`.
- `displayName`: operator-visible label.
- `status`: active or disabled state from the daemon.
- `callerMembershipRole`: caller role when present.
- `callerMembershipStatus`: caller membership state when present.
- `callerPermissions`: permissions available to the caller in this tenant.
- `isDefault`: whether this tenant is the server-resolved default for the current caller.

Relationships:

- Appears in an `AllowedTenantList`.
- May have many `Membership` records.
- May be selected as the `ActiveTenant`.

Validation rules:

- The shell may display or select only tenants present in the current allowed tenant list.
- Disabled or inaccessible tenants produce a stable denial state, not fallback data.

## Allowed Tenant List

Represents the current caller's selectable tenants.

Fields:

- `items`: ordered tenant resources returned by the daemon.
- `defaultTenantId`: tenant selected when no explicit tenant is configured.
- `loadedAt`: client-side timestamp for the loaded list.

Relationships:

- Used to validate persisted tenant selection.
- Supplies choices for the tenant switcher.

Validation rules:

- A previously persisted tenant selection is valid only when its id appears in `items`.
- Empty allowed tenant lists are treated as denied or unauthenticated states, not as a
  global data mode.

## Active Tenant

Represents the tenant currently selected by the shell or SDK request.

Fields:

- `tenantId`: selected tenant identifier.
- `source`: `server_default`, `restored`, `user_selected`, or `request_override`.
- `status`: `resolving`, `active`, `denied`, or `stale`.
- `generation`: monotonically increasing client-side refresh generation.
- `selectedAt`: client-side timestamp for the selection.

Relationships:

- Drives every `Tenant-Scoped Projection`.
- Drives SDK default tenant configuration for shell requests.
- Produces persisted `Tenant Selection Preference` after successful user selection.

State transitions:

- `resolving -> active`: tenant appears in the allowed tenant list and scoped refresh
  succeeds.
- `resolving -> denied`: tenant is inaccessible or no allowed fallback exists.
- `active -> stale`: user initiates tenant switch or active tenant access is revoked.
- `stale -> active`: new allowed tenant is selected and projections refresh.
- `active -> denied`: active tenant is revoked or denied during refresh.

Validation rules:

- Tenant-scoped data can be shown only when status is `active`.
- Responses whose tenant id or generation do not match the current active tenant are
  ignored.
- Active tenant revocation clears tenant-scoped views and requires explicit user action to
  choose another allowed tenant.

## Tenant-Scoped Projection

Represents a shell view whose records belong to exactly one active tenant.

Projection kinds:

- `onboarding`
- `activity`
- `diagnostics`
- `approvals`
- `evaluationReplayCandidates`
- `evaluationReplayAttempts`
- `evaluationComparisons`
- `evaluationFixtures`

Fields:

- `kind`: projection kind.
- `tenantId`: tenant id used for the fetch.
- `generation`: refresh generation that produced the data.
- `status`: `idle`, `loading`, `ready`, `stale`, `denied`, or `error`.
- `items` or `payload`: projection data.
- `error`: stable error state when denied or failed.
- `loadedAt`: client-side timestamp for successful data.

Relationships:

- Belongs to one `ActiveTenant`.
- May feed `DetailView` records.

Validation rules:

- A projection must be cleared or marked stale before fetching under a different tenant.
- Previous-tenant rows must not be rendered as current data after a switch completes.
- Denied projections must not fall back to global or previous-tenant data.

## Detail View

Represents an operator-selected detail pane for a tenant-scoped row.

Fields:

- `title`: display title.
- `route`: authoritative route for detail refresh when present.
- `tenantId`: tenant id under which the detail was loaded.
- `generation`: active tenant generation that loaded the detail.
- `payload`: detail payload.
- `status`: `ready`, `loading`, `stale`, `denied`, or `error`.

Validation rules:

- Detail views are cleared, closed, or marked stale before new-tenant data is displayed.
- A stale response from a previous tenant must not repopulate the detail pane.

## SDK Tenant Configuration

Represents client-level tenant intent for SDK consumers.

Fields:

- `defaultTenantId`: optional tenant id configured when the client is created.
- `accessToken`: existing bearer token configuration.
- `baseURL`: existing daemon URL configuration.

Relationships:

- Provides the default tenant for `TenantRequestOptions` when no per-request override is
  supplied.

Validation rules:

- Omitted `defaultTenantId` preserves server-resolved default tenant behavior.
- Configured default tenant is used for tenant-scoped requests unless overridden.
- Default tenant is immutable for the lifetime of the client instance.

## Tenant Request Options

Represents one-request tenant override behavior.

Fields:

- `tenantId`: optional tenant id override for a single SDK request.

Relationships:

- Overrides `SDK Tenant Configuration.defaultTenantId` for only the request where it is
  supplied.

Validation rules:

- Override must not mutate the client default tenant.
- Request helpers propagate tenant intent through the shared tenant transport header.
- Empty tenant id values are treated as omitted.

## Membership

Represents a user's role and state in an organization tenant.

Fields:

- `membershipId`: stable identifier.
- `tenantId`: active tenant id.
- `principalId`: member principal.
- `role`: `owner`, `admin`, `operator`, or `viewer`.
- `status`: invited, active, or removed state.
- `createdAt`, `updatedAt`: audit timestamps.

Relationships:

- Belongs to one tenant and one principal.
- Is displayed and updated by `Membership Management State`.

Validation rules:

- Role update controls are available only when the caller has `tenant.manage`.
- Role changes/removals must not leave an organization tenant with no active owner.
- Failed or denied role changes must leave the prior visible role intact.

## Membership Management State

Represents the shell's active-tenant membership panel.

Fields:

- `tenantId`: active tenant being managed.
- `canManage`: whether the current caller has `tenant.manage`.
- `members`: active-tenant membership list.
- `status`: `hidden`, `loading`, `ready`, `empty`, `denied`, or `error`.
- `pendingMembershipId`: membership currently being changed.
- `error`: stable denial or failure state when present.

Validation rules:

- Management controls are hidden or disabled when `canManage` is false.
- No optimistic role update is committed locally before daemon confirmation.
- Empty state for owner-only tenants must be distinct from loading and error states.

## Tenant Authorization Denial

Represents stable denied tenant access for UI and SDK callers.

Fields:

- `code`: stable denial code.
- `message`: caller-safe display message.
- `tenantDenied`: boolean SDK convenience marker.
- `status`: HTTP status when available.

Validation rules:

- The denial must not reveal whether an inaccessible tenant exists.
- Shell denial states must clear tenant-scoped data rather than retaining old rows.
- SDK callers must not parse raw error text to identify tenant denials.

## Tenant Selection Preference

Represents non-secret browser-local continuity state.

Fields:

- `daemonURL`: daemon URL key.
- `principalId`: principal key when known.
- `tenantId`: last successfully selected tenant.
- `savedAt`: client-side timestamp.

Validation rules:

- Preference is only a hint; it is revalidated against the allowed tenant list before use.
- The preference does not grant access and must never contain tokens or secrets.
