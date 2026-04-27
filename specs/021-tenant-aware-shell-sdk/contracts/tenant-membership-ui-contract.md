# Tenant Membership UI Contract

## Goal

The shell provides a minimal production-shaped active-tenant membership surface for
authorized users, without becoming a full organization administration suite.

## Visibility

The membership panel is scoped to the active tenant.

Behavior:

- Users without `tenant.manage` do not see enabled role-change controls.
- Users with `tenant.manage` can inspect members, roles, and membership states.
- Owner-only tenants show a useful empty/owner-only state rather than loading forever.
- Denied membership access shows a stable denial state and does not retain stale member
  rows from another tenant.

## Role Update

Authorized users can update an active member's role.

Request behavior:

- Use the SDK membership helper for the active tenant.
- Do not optimistically change the visible role before daemon confirmation.
- On success, refresh or replace the membership row with the daemon-confirmed state.
- On failure, keep the previous visible role and show the stable error state.

## Last Owner Protection

Role changes or removals that would leave an organization tenant with no active owner are
prevented by the daemon and surfaced by the UI.

Acceptance rules:

- Attempted downgrade/removal of the last active owner fails.
- The visible membership list still shows at least one active owner after the failure.
- The failure is testable by API and shell tests.

## Audit-Visible Result

Successful role changes must leave audit-visible state sufficient for operators to confirm:

- acting principal
- active tenant
- target membership/member
- old role and new role when available
- timestamp

The UI does not need to render a full audit log in this phase, but it must show the
daemon-confirmed resulting role state after success.

## Verification

Tests must cover:

- controls hidden or disabled without `tenant.manage`
- authorized user sees membership list and role controls
- successful role update refreshes visible role
- denied role update preserves previous visible role
- last-owner downgrade/removal is prevented
