# Phase 0 Research: Tenant-Aware Operator Shell And SDK

This document resolves the technical-design choices needed to plan Roadmap 36 without
unresolved clarification markers. Product clarifications are recorded in `spec.md` under
`## Clarifications`.

## R1. SDK tenant API shape

**Decision**: Add `defaultTenantId?: string` to `KuraClientOptions` and add a small
`TenantRequestOptions` options bag with `tenantId?: string` for per-request override.
Public tenant-scoped SDK methods accept the options bag as an optional trailing argument;
methods that already accept query/input arguments keep those arguments unchanged and add
the tenant options after them. Internally, all SDK request helpers accept the resolved
tenant option and emit `X-Kura-Tenant-ID` only when a request override or client default
is present.

**Rationale**:
- Optional trailing arguments preserve existing call sites and avoid mixing tenant
  selection into query strings or request bodies.
- A client-level default plus explicit per-request override directly matches the upstream
  contract.
- Centralizing header propagation in `buildHeaders()` keeps callers from constructing
  tenant transport details manually.
- Stream and non-stream requests need the same tenant option path so event and chat
  subscriptions are not a special case.

**Alternatives considered**:
- Put `tenantId` in every query/input object: rejected because it pollutes domain inputs
  and risks serializing tenant selection as business data.
- Require users to instantiate one client per tenant only: rejected because it does not
  provide a one-request override.
- Expose raw header hooks: rejected because the upstream spec explicitly forbids ad hoc
  tenant header construction by SDK callers.

## R2. Stable tenant denial mapping

**Decision**: Preserve `KuraClientError` as the public error class and add tenant-denial
metadata fields where the daemon supplies stable error information: `code`,
`tenantDenied?: boolean`, and optional `denial?: TenantDenialResource`. The SDK maps
known tenant-denial status/code combinations into these stable fields without requiring
callers to parse raw message text.

**Rationale**:
- Preserves the existing SDK error surface while giving shell and SDK consumers a stable
  branch for authorization-denied UI.
- Does not require every route to return a new response wrapper.
- Aligns with Roadmap 34 denial behavior: callers should not learn whether an inaccessible
  tenant exists.

**Alternatives considered**:
- Introduce a separate `TenantAuthorizationError` subclass: deferred. It may be useful
  later, but adding metadata to the existing error class keeps compatibility and avoids
  subtle `instanceof` differences for downstream callers.
- Use only HTTP status codes: rejected because 403/404 alone cannot distinguish stable
  tenant denial from unrelated failures.

## R3. Tenant selection persistence

**Decision**: The shell stores only the last selected tenant id in browser-local state,
keyed by daemon base URL and principal identity when available. On load, the shell fetches
the allowed tenant list before any tenant-scoped projection and restores the saved tenant
only if it is still allowed; otherwise it selects the server/default allowed tenant or
shows a denied selection state.

**Rationale**:
- Restores operator continuity without trusting stale client state.
- The stored value is only an id, not a secret or grant; access is revalidated by the
  daemon on every load and every request.
- Keying by daemon/principal avoids leaking a previous user's tenant selection into a
  different login on the same browser.

**Alternatives considered**:
- Always start on the default tenant: rejected by Q2 because it loses useful operator
  context for multi-tenant users.
- Require selection every reload: rejected as unnecessary friction.
- Persist server-side profile state: deferred. It would require new daemon persistence
  and is not needed to close this roadmap.

## R4. Tenant switch and in-flight response handling

**Decision**: The shell maintains a monotonic tenant refresh generation. Switching tenants
immediately clears or marks stale tenant-scoped lists and detail panes, starts a new
generation, and ignores any response whose generation or tenant id does not match the
current active tenant. Work/actions already submitted remain attributed to the tenant they
started under.

**Rationale**:
- Prevents previous-tenant rows from reappearing when slower requests finish after a
  switch.
- Does not pretend already-submitted work moved tenants.
- Gives deterministic tests for stale response handling without relying on browser
  cancellation semantics.

**Alternatives considered**:
- Block tenant switch until all requests finish: rejected as poor UX and still fragile
  for long streams.
- Best-effort cancel only: rejected as insufficient because cancellation is not guaranteed
  across every fetch/stream boundary.
- Re-associate in-flight work with the new tenant: rejected because it would break audit
  attribution and tenant ownership.

## R5. Tenant-scoped shell bootstrap sequence

**Decision**: Shell load order is: authenticate/bootstrap identity and allowed tenants,
resolve active tenant, construct an SDK client with that active tenant, then concurrently
fetch onboarding, activity, diagnostics, approvals, replay candidates, replay attempts,
comparisons, and fixtures. Event streams are also opened with the active tenant and are
closed/reopened on tenant switch.

**Rationale**:
- Prevents tenant-scoped views from loading before tenant selection is validated.
- Keeps operator projections consistent by using the same tenant id for all concurrent
  refreshes.
- Reopening streams on switch avoids cross-tenant event refreshes from the previous tenant.

**Alternatives considered**:
- Fetch projections first and filter in the browser: rejected because Roadmap 35 requires
  server-side tenant scoping and the shell must not hold previous-tenant rows as current
  data.
- Maintain one stream per allowed tenant: rejected because it widens data exposure and is
  out of scope for a tenant-focused operator shell.

## R6. Membership management behavior

**Decision**: The shell exposes a minimal active-tenant membership surface: list members,
show role/status, show role-change controls only for callers with `tenant.manage`, submit
role updates through the tenant membership endpoint, and display stable denial/failure
states without changing local role state optimistically. Last-owner protection is enforced
by the daemon and covered by shell and daemon tests.

**Rationale**:
- Meets the production-shaped but minimal upstream scope.
- Avoids optimistic UI that can misrepresent authorization-sensitive role state.
- Keeps the source of truth in the daemon so hidden/disabled controls are not the only
  security barrier.

**Alternatives considered**:
- Full organization administration suite: rejected by upstream out-of-scope.
- Client-only last-owner validation: rejected because direct API callers must also be
  protected.

## R7. Contract artifact boundaries

**Decision**: Keep Roadmap 36 contracts at the client/operator-surface level:
`sdk-tenant-contract.md`, `operator-shell-tenant-contract.md`, and
`tenant-membership-ui-contract.md`. Reuse existing JSON Schema files from Roadmap 34 for
tenant, membership, principal, permission, denial, token grant, and auth/me shapes. Add or
update `schemas/api/` only if implementation discovers a missing field required by these
contracts.

**Rationale**:
- Avoids duplicating Roadmap 34 API contracts while still making SDK and shell obligations
  explicit for task generation.
- Keeps implementation tasks focused on the client/operator surfaces this roadmap owns.
- Preserves `make daemon-contract-test` as the gate for any schema drift.

**Alternatives considered**:
- Re-declare every tenant HTTP endpoint in this spec: rejected as duplication of
  `specs/019-tenant-identity-access/contracts/tenant-identity-access-surfaces.md`.
- Skip contracts because the work is "just UI": rejected because SDK method shape and
  tenant switch behavior are externally visible contracts.
