# Operator Shell Tenant Contract

## Goal

The operator shell makes tenant context visible, allows switching among allowed tenants,
and ensures tenant-scoped projections never display stale data from another tenant as
current data.

## Bootstrap Sequence

1. User provides the existing daemon URL and access token.
2. Shell calls the SDK identity/tenant helpers to load current identity and allowed
   tenants.
3. Shell resolves the active tenant:
   - restore last selected tenant only if it is still allowed
   - otherwise use the server/default allowed tenant when available
   - otherwise show a denied selection state
4. Shell creates tenant-scoped SDK requests with the active tenant.
5. Shell concurrently refreshes onboarding, activity, diagnostics, approvals, and
   evaluation projections.

Tenant-scoped projections must not load before the active tenant is resolved.

## Tenant Switcher

The tenant switcher shows:

- active tenant display name and kind
- allowed tenant choices only
- clear loading/stale/denied states during switch

Acceptance rules:

- A single personal tenant user sees the personal tenant as active and is not forced into
  organization setup.
- A multi-tenant user can switch from personal to organization tenant.
- A denied tenant selection shows a stable denial state.
- The shell never falls back to global data or previous-tenant data.

## Tenant-Scoped Projections

Projection set:

- onboarding
- activity
- diagnostics
- approvals
- evaluation replay candidates
- evaluation replay attempts
- evaluation comparisons
- evaluation fixtures

Acceptance rules:

- Every projection request carries active tenant intent through the SDK.
- Switching tenant clears or marks stale all projection rows before new-tenant rows are
  shown.
- Open detail panes are cleared, closed, or marked stale before new-tenant detail data is
  shown.
- Stale responses from older tenant refresh generations are ignored.
- Event streams are closed and reopened under the new active tenant.

## Active Tenant Revocation

When active tenant access is revoked during a shell session:

- clear tenant-scoped projection rows and detail panes
- show a stable denied state
- require the user to explicitly choose another allowed tenant
- do not automatically switch to another tenant while preserving old rows

## In-Flight Work

Work or requests already submitted before a tenant switch remain attributed to the tenant
they started under. Responses from the previous tenant are ignored after switch unless the
response tenant/generation still matches the current active tenant.

## Verification

Web tests must cover:

- first load with one personal tenant and no organization tenants
- multi-tenant switch from personal to organization tenant
- persisted selection restored only after allowed-tenant revalidation
- stale rows/detail panes cleared or marked stale during switch
- out-of-order previous-tenant responses ignored
- denied tenant access does not fall back to old data
- active-tenant revocation clears views and requires user selection
