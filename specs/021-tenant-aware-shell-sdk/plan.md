# Implementation Plan: Tenant-Aware Operator Shell And SDK

**Branch**: `022-tenant-aware-shell-sdk` | **Date**: 2026-04-26 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/021-tenant-aware-shell-sdk/spec.md`

## Summary

Close Roadmap 36 by making tenant context visible and selectable in the operator shell,
making all shell projections reload under the selected tenant without stale cross-tenant
data, exposing minimal tenant membership inspection and role management, and adding
first-class tenant intent to the TypeScript SDK. The implementation uses the Roadmap 34
tenant identity surfaces and Roadmap 35 tenant-scoped data behavior, adds SDK wrappers for
tenant resources and per-request override support, and updates the React shell to drive
activity, diagnostics, approvals, onboarding, and evaluation views through one validated
active-tenant state.

This roadmap is client/operator-surface work only. It does not add billing checkout, a
full organization administration suite, native mobile tenant switching, a new persistence
migration, or live connector behavior.

## Technical Context

**Language/Version**: Go 1.24 daemon API/contracts; TypeScript 5.7 SDK; React 19 web
shell; Vite 7/Vitest 3 client tests.
**Primary Dependencies**: Existing `daemon/internal/identity`,
`daemon/internal/tenantctx`, `daemon/internal/api/tenants.go`, `daemon/internal/api/operator.go`,
`daemon/internal/api/evaluation.go`, `sdk/ts/src/index.ts`, `web/src/app/App.tsx`,
existing JSON Schema files under `schemas/api/`, React Testing Library for shell tests,
Vitest for SDK and web tests.
**Storage**: No daemon schema migration. Shell persists only the last selected tenant id
as non-secret browser-local client state, then revalidates it against `GET /v1/tenants`
before showing tenant-scoped data.
**Testing**: `pnpm test:clients` for SDK and web behavior, package-local `vitest run`
where useful, `make daemon-contract-test` when schemas/contracts are touched, `go test
./...` in `daemon/` for API membership/last-owner regressions, `go mod tidy` in
`daemon/` after implementation.
**Target Platform**: Local-first daemon and browser operator shell against the default
test daemon (`~/.dope-test`, `127.0.0.1:19192`); hosted deployment uses the same API and
SDK contracts.
**Project Type**: Multi-surface product change: Go HTTP daemon API + TypeScript SDK
library + React operator shell + shared JSON Schema contracts.
**Performance Goals**: Tenant switching and shell refresh should not add avoidable extra
round trips beyond identity/tenant bootstrap plus one concurrent refresh of the in-scope
projections. Stale responses must be ignored deterministically even when requests finish
out of order.
**Constraints**: Preserve existing SDK calls that omit tenant configuration. Tenant
override must be explicit, per request, and must not mutate the client default. Active
tenant access revocation clears tenant-scoped views and requires explicit user selection.
Last-owner role changes/removals are prevented. No secrets or live connectors are touched.
**Scale/Scope**: One shell session handles the current user's allowed tenant list, expected
to be small enough for direct display (tens of tenants). The membership surface is scoped
to one active tenant and bounded list sizes from the existing tenant APIs.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** — PASS. The plan closes Roadmap 36 end-to-end: SDK default tenant
  and override support, tenant-aware client types, shell tenant switcher, scoped
  projections, denied/empty states, membership inspection, role changes, stale response
  handling, selection persistence, and acceptance tests.
- **Production-grade, minimal, reversible change** — PASS. The change is additive to SDK
  options and method overloads, additive to shell UI state, and reuses existing daemon
  tenant/membership endpoints. Rollback is to remove or hide the switcher/membership UI
  and stop passing tenant override headers while leaving server-resolved default tenant
  behavior intact.
- **Contracts and auditability** — PASS. SDK-facing tenant types map to existing
  `schemas/api/*tenant*`, `membership`, `principal`, `permission`, `token grant`, and
  stable denial resources. Membership role changes use existing daemon audit-visible
  state; any schema additions or response shape changes must update `schemas/api/`,
  generated/client types, and `make daemon-contract-test` together.
- **Verification and observability** — PASS. Required verification covers SDK header and
  error mapping, shell first-load and tenant switch flows, stale detail/response
  protection, denial states, active-tenant revocation behavior, membership permission
  gating, last-owner protection, role update audit-visible results, and full client
  regression tests.
- **Environment and secrets** — PASS. All verification defaults to the test daemon and
  browser test environment. Tenant ids persisted in the browser are not secrets. Access
  tokens remain user-entered/operator-owned and must not be logged or stored by this
  roadmap.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/021-tenant-aware-shell-sdk/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── sdk-tenant-contract.md
│   ├── operator-shell-tenant-contract.md
│   └── tenant-membership-ui-contract.md
├── checklists/
│   └── requirements.md
└── tasks.md                # /speckit.tasks output, not created by /speckit.plan
```

### Source Code (repository root)

```text
sdk/ts/
├── src/
│   ├── index.ts            # DopeClientOptions default tenant, per-request override,
│   │                       # tenant/membership/principal/permission/token types,
│   │                       # tenant API wrappers, stable denial mapping
│   └── index.test.ts       # default tenant, override, stream, denial mapping tests
└── dist/                   # regenerated by pnpm build

web/
├── src/
│   ├── app/
│   │   ├── App.tsx         # active tenant state, switcher, scoped refresh,
│   │   │                   # stale response guards, membership management
│   │   └── App.test.tsx    # tenant switch, denial, stale, membership tests
│   └── styles.css          # tenant switcher and membership management styling
└── dist/                   # regenerated by pnpm build

daemon/
└── internal/
    ├── api/
    │   ├── tenants.go      # verify/update role and last-owner behavior as needed
    │   └── tenant_identity_test.go / operator_test.go
    └── identity/           # read-only consumer unless last-owner gap is found

schemas/
└── api/                    # existing tenant, membership, principal, permission,
                            # token grant, auth/me, and denial shapes; update only
                            # if implementation exposes a missing contract field
```

**Structure Decision**: The SDK remains the single tenant transport abstraction used by
the web shell. The web shell never constructs tenant headers directly; it selects an
active tenant and passes tenant intent through SDK request options. Daemon changes are
limited to filling any missing membership or last-owner contract gaps found during
implementation; this roadmap should not add new daemon storage or cross-tenant admin
surfaces.

## Post-Design Constitution Check

- **Roadmap closure** — PASS. `research.md`, `data-model.md`, and the three contract
  artifacts cover every Roadmap 36 surface named by the spec: SDK tenant intent,
  shell tenant selection, tenant-scoped projection refresh, denial handling, membership
  inspection, role updates, and last-owner protection.
- **Production-grade, minimal, reversible change** — PASS. Design artifacts keep daemon
  persistence out of scope, keep browser persistence non-secret and revalidated, and make
  rollback a client-surface rollback that preserves existing server-default tenant
  behavior.
- **Contracts and auditability** — PASS. `contracts/sdk-tenant-contract.md` defines the
  SDK API and denial mapping; `contracts/operator-shell-tenant-contract.md` defines
  tenant switching and stale response behavior; `contracts/tenant-membership-ui-contract.md`
  defines membership role update and audit-visible result expectations.
- **Verification and observability** — PASS. `quickstart.md` names required client,
  daemon, contract, build, and tidy checks. Contracts require stable denial states and
  daemon-confirmed membership results rather than optimistic or silent behavior.
- **Environment and secrets** — PASS. The design remains test-environment-first, stores
  only non-secret tenant ids locally, and introduces no live connector or payment secrets.

No post-design violations require justification.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                      |
