# Web Client

This package is the operator-facing Web shell surface for Kura.

Role:

- client only
- consumes daemon APIs through `@kura/client`
- does not own runtime truth
- does not assume daemon-side multi-turn memory

Operator flow:

- configure daemon URL
- provide access token
- resolve identity and allowed tenants before loading tenant-scoped views
- select an allowed active tenant
- inspect onboarding, approvals, activity, diagnostics, evaluation replay, and
  active-tenant memberships under that tenant

Tenant-aware smoke checks:

- single personal-tenant users should load with the personal tenant active and
  no required organization setup
- multi-tenant users should be able to switch to an allowed organization tenant
  and see projections clear or mark stale before refetching
- approval, first-use, replay, detail, and stream actions must carry the active
  tenant through the SDK, not hand-built headers
- users without `tenant.manage` must not receive enabled role-change controls
- authorized membership role updates must show daemon-confirmed state, while
  denied or failed updates keep the previous visible role
