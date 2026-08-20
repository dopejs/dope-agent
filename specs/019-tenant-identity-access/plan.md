# Implementation Plan: Tenant Identity And Access Foundation

**Branch**: `019-tenant-identity-access` | **Date**: 2026-04-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/019-tenant-identity-access/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/plan-template.md` for the execution workflow.

## Summary

Close roadmap 34 by adding a first-class tenant identity and access foundation to the
local-first daemon before hosted multi-tenant product work lands. The design introduces a
focused `daemon/internal/identity` domain for tenants, principals, memberships,
invitations, role-derived permissions, tenant resolution, and security audit decisions;
extends `daemon/internal/auth` token records with durable lifecycle and tenant grants;
adds additive SQLite migrations and schema-backed `/v1/tenants`, `/v1/principals`, and
token lifecycle surfaces; and wraps protected daemon routes with request tenant
resolution that either produces a principal and tenant context or returns a stable
authorization denial before tenant-owned resources are accessed. Existing single-user
local installations continue through a bootstrapped default personal tenant, with
pre-tenant local tokens limited to that personal tenant unless later grants are explicit.

## Technical Context

**Language/Version**: Go 1.24.0; TypeScript 5.7; React 19; Vite 7; Markdown docs; JSON
Schema contracts  
**Primary Dependencies**: `daemon/internal/auth`, new `daemon/internal/identity`,
`daemon/internal/api`, `daemon/internal/app`, `daemon/internal/store`,
`daemon/internal/events`, `daemon/internal/contracts`, `schemas/api`, `schemas/events`,
`docs/runtime`, `docs/product`, `docs/specs`, and `AGENTS.md`  
**Storage**: existing SQLite daemon state with additive tenant, principal, membership,
invitation, token grant, token lifecycle, and tenant audit tables; current schema version
is 20 at planning time, so implementation should add the next migration version unless
another migration lands first  
**Testing**: `cd daemon && go test ./internal/identity ./internal/auth ./internal/api ./internal/store ./internal/contracts ./internal/app`; `make daemon-contract-test`; `cd daemon && go test ./...`; `cd daemon && go mod tidy`; optional `pnpm test:clients` if SDK or client code changes are needed during implementation  
**Target Platform**: local daemon and operator APIs in `KURA_ENV=test` by default, with
compatibility for existing local-first installations and no live connector requirement  
**Project Type**: Go daemon control-plane service with committed JSON Schema API/event
contracts and local SQLite persistence  
**Performance Goals**: tenant resolution adds no more than one bounded local store lookup
per protected request after in-memory cache warmup; tenant and membership list routes
return in `<=1 s` for low-hundreds tenants and memberships in test state; permission
evaluation is deterministic and side-effect free  
**Constraints**: additive API and schema changes only; denial responses must not leak
tenant existence; audit recording for security-relevant tenant switching, membership
changes, and token lifecycle changes must fail closed; permissions derive only from role
and lifecycle state in this phase; already-authorized in-flight work is not cancelled by
this phase; no full tenant switcher UI; no migration of existing domain tables to tenant
scope; no billing/quota enforcement; no per-tenant physical storage  
**Scale/Scope**: one local operator installation plus hosted-ready contracts; personal and
organization tenants; owner/admin/operator/viewer role bundles; one default personal
tenant per principal; explicit token tenant grants; low-hundreds memberships/tokens in
local test state; foundation only for later tenant-scoped data and client roadmap work

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Roadmap closure: PASS. This plan closes roadmap 34 only: tenant/principal resource
  model, default personal tenant bootstrap, organization membership/invite lifecycle,
  token grants and lifecycle, request tenant resolution, permission evaluation, and audit
  events. Domain table tenant migration, tenant switcher UI, billing/quota enforcement,
  and per-tenant storage remain out of scope for later roadmaps.
- Production-grade change control: PASS. The change set is additive and reversible at the
  contract level: new identity records, new route families, new schemas, new audit events,
  and new middleware. Existing local-first requests continue through a default personal
  tenant, and pre-tenant local tokens receive only that tenant grant.
- Contracts and auditability: PASS. The plan identifies API, schema, event, persistence,
  auth token, and audit surfaces that must change together, including stable denial
  semantics and fail-closed audit behavior.
- Verification and observability: PASS. The plan names unit, API, store migration,
  contract, restart, and daemon-wide tests, plus operator-visible audit records for
  tenant switching, denial, membership, invite, and token lifecycle changes.
- Environment and secrets: PASS. Development and verification default to `KURA_ENV=test`
  and `~/.kura-test`; no live connectors, managed-provider auth, or new operator secrets
  are required.

If any gate fails, stop and resolve the gap before Phase 0 research proceeds.

Post-design re-check:

- PASS. The design remains roadmap-closed to phase 34 and does not drift into tenant data
  migration, tenant-aware shell UI, hosted secrets isolation, quota enforcement, or live
  validation product flows.
- PASS. Migration sequencing is forward-only and additive, with explicit bootstrap and
  rollback expectations that preserve local-first operation without widening existing
  token authority.
- PASS. API, schema, event, store, auth, docs, and contract-test artifacts are explicitly
  scoped, with denial and audit behavior testable before downstream roadmaps consume the
  tenant context.

## Project Structure

### Documentation (this feature)

```text
specs/019-tenant-identity-access/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── tenant-identity-access-surfaces.md
└── tasks.md
```

### Source Code (repository root)

```text
daemon/
├── internal/
│   ├── api/
│   ├── app/
│   ├── auth/
│   ├── contracts/
│   ├── events/
│   ├── identity/
│   └── store/
└── go.mod

schemas/
├── api/
└── events/

docs/
├── product/
├── runtime/
└── specs/

AGENTS.md
```

**Structure Decision**: Add `daemon/internal/identity` for tenant, principal,
membership, invitation, role bundle, permission evaluation, tenant resolution, and audit
domain behavior. Keep bearer token issuance and secret hashing in `daemon/internal/auth`,
but extend token records with lifecycle and grant fields coordinated by the identity
manager. Keep HTTP routing and middleware in `daemon/internal/api`, startup bootstrap and
state restore wiring in `daemon/internal/app`, and durable schema evolution in
`daemon/internal/store`. Keep JSON Schema contracts in `schemas/api` and
`schemas/events`, with live-route and fixture validation in `daemon/internal/contracts`.
Do not build tenant switcher UI in `web/` or `tui/`; tenant-aware client ergonomics remain
for the later operator shell and SDK roadmap.

## Complexity Tracking

No constitution violations remain. A new `daemon/internal/identity` package is justified
because tenant resolution, lifecycle-gated authorization, role-derived permissions, and
fail-closed audit decisions are shared cross-cutting domain rules. Placing this logic
inside `api` would hide authorization behavior in handlers, and placing it entirely
inside `auth` would conflate bearer-token authentication with tenant-scoped
authorization. The design avoids broad tenant-scoping refactors and does not migrate
existing domain tables in this roadmap.

## Implementation Notes

- Add `daemon/internal/identity`:
  - `types.go` for tenant, principal, membership, invitation, permission, role bundle,
    tenant context, denial, and audit types.
  - `manager.go` for bootstrap, invite lifecycle, membership lifecycle, principal
    lifecycle, token grant coordination, and audit orchestration.
  - `permissions.go` for the clarified role-derived permission baseline and no
    per-member override support.
  - `resolver.go` for default tenant and `X-Kura-Tenant-ID` selection rules.
  - `audit.go` for fail-closed security audit writes and stable denial reasons.
- Extend `daemon/internal/auth`:
  - add token lifecycle fields for expiry, revocation, rotation lineage, and principal
    ownership.
  - add token issue, rotate, revoke, and grant-change entry points that never return raw
    token material except at issue or rotation completion.
  - preserve existing pairing behavior by creating or resolving the principal/default
    personal tenant and granting only that tenant to pre-tenant local tokens.
- Add store support:
  - increment `CurrentSchemaVersion` from 20 to the next available version at
    implementation time.
  - add idempotent SQLite migrations for tenants, principals, memberships, invitations,
    token tenant grants, token lifecycle metadata, and tenant audit events.
  - add store APIs and restart tests for bootstrap, grants, memberships, invitations,
    token lifecycle state, and audit records.
- Add protected-route tenant resolution in `daemon/internal/api`:
  - after bearer authentication and token persistence, resolve principal and tenant
    context before protected handler execution.
  - use `X-Kura-Tenant-ID` only when both principal and token grant allow the tenant.
  - return one stable authorization denial for unknown and inaccessible tenants.
  - attach resolved context to `context.Context` for later roadmap consumers.
- Add route families:
  - tenant and membership inspection plus organization creation.
  - invitation create/list/accept/reject.
  - principal inspection and lifecycle updates.
  - token list/issue/rotate/revoke/grant-change inspection.
  - tenant audit event inspection.
- Add contract artifacts:
  - API schemas for tenant, principal, membership, invitation, permission, token grant,
    token lifecycle, tenant context, audit event, and denial resources.
  - event schemas for tenant denial, tenant switch, membership changes, invite decisions,
    token issue, rotation, revocation, expiry denial, tenant-grant changes, and
    fail-closed audit denial.
- Update docs:
  - `docs/runtime/operator-trust-model.md`
  - `docs/runtime/daemon-api-and-event-model.md`
  - `docs/runtime/migration-versioning.md` if schema-version examples stay stale
  - `docs/runtime/daemon-roadmaps.md`
  - `docs/specs/019-tenant-identity-and-access-foundation.md` only if implementation
    clarifies upstream roadmap wording without changing scope.
- Do not add per-member permission overrides, tenant switcher UI, domain table tenant
  columns, billing/quota enforcement, or per-tenant storage backends in this roadmap.

## Migration Plan

1. Add identity tables and token lifecycle/grant columns/tables as an additive SQLite
   migration. Existing rows remain readable by older code only after restoring a backup;
   the daemon's normal rollback path remains SQLite snapshot restore plus compatible
   binary.
2. On startup, bootstrap a personal tenant and active principal for existing local-first
   installations that have auth tokens but no tenant records.
3. Grant pre-tenant local tokens only the bootstrapped default personal tenant.
4. Persist the bootstrap audit event and fail startup if the required bootstrap write
   cannot complete safely.
5. Restore tenant, principal, membership, invitation, token lifecycle, and token grant
   state before serving protected routes.
6. Enable tenant resolution middleware after state restore so no accepted protected
   request executes without principal and tenant context.

Rollback path:

- Before release, take a SQLite backup for operator installations where persistent auth
  state matters.
- To roll back after migration, stop the daemon, restore the pre-upgrade SQLite file, and
  restart a daemon binary compatible with that schema.
- Do not attempt automatic down migrations; this matches the repository's migration
  versioning policy.

Failure modes and guardrails:

- Bootstrap audit write fails: fail closed and do not serve protected routes.
- Token grant table missing or unreadable: fail startup rather than allowing global token
  authority.
- Selected tenant missing or disallowed: return the stable authorization denial without
  existence leakage.
- Grant or membership update races with request handling: next-check enforcement applies
  after the change is durably recorded; in-flight cancellation remains out of scope.

## Automated Verification

- `cd daemon && go test ./internal/identity ./internal/auth ./internal/api ./internal/store ./internal/contracts ./internal/app`
- `make daemon-contract-test`
- `cd daemon && go test ./...`
- `cd daemon && go mod tidy`
- `pnpm test:clients` only if implementation touches SDK, web, or TUI packages

These commands are expected to cover:

- default personal tenant bootstrap for new and existing local-first state
- pre-tenant token grant limitation to the bootstrapped default personal tenant
- explicit tenant header resolution and denial without existence leakage
- owner/admin/operator/viewer role-derived permission baseline
- absence of per-member permission overrides
- organization invite, accept, reject, role update, and removal lifecycle
- disabled and removed principal denial
- revoked, expired, rotated, and grant-changed token denial
- next-check enforcement after durable membership, principal, and token changes
- fail-closed audit behavior for tenant switching, membership changes, and token lifecycle
  changes
- restart restoration for tenants, principals, memberships, invitations, token grants,
  token lifecycle state, and audit records
- schema validation for every new API and event contract

## Manual Verification

- `make daemon-run-test`
- `make daemon-test-status`
- Pair or reuse a local bearer token in the test environment.
- Confirm `GET /v1/auth/me` returns the principal, default tenant, allowed tenants, and
  current tenant context.
- Call a protected route without `X-Kura-Tenant-ID` and confirm it resolves to the default
  personal tenant.
- Create an organization tenant, invite a second principal, accept the invitation, and
  verify membership and audit records.
- Retry a protected request with an allowed `X-Kura-Tenant-ID` and then with a disallowed
  tenant id; confirm the disallowed response is stable and does not reveal existence.
- Revoke or expire a token and confirm the next protected request is denied.
- Restart the daemon and confirm tenant grants, memberships, token lifecycle state, and
  audit history remain intact.
