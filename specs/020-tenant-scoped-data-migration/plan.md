# Implementation Plan: Tenant-Scoped Data Migration

**Branch**: `020-claude` | **Date**: 2026-04-25 | **Spec**: [`spec.md`](./spec.md)
**Input**: Feature specification from `specs/020-tenant-scoped-data-migration/spec.md`

## Summary

Add tenant ownership to every persisted runtime, product, and harness record in the daemon
SQLite store, migrate existing rows into the operator's default personal tenant, and route
all reads, writes, events, SSE replay, and operator projections through resolved tenant
context. The work delivers (a) a checked-in schema inventory that classifies every
persisted table and event source as `tenant_owned`, `global`, or `derived`, (b) additive,
restart-safe forward migrations with durable per-step progress tracking, (c) a tenant-aware
store package that replaces or restricts the tenantless helpers for tenant-owned tables,
(d) tenant-aware indexes and per-tenant uniqueness constraints, (e) cross-tenant isolation
regressions for every in-scope domain, (f) a typed audit event plus greppable log line on
every denied cross-tenant access, and (g) backup-restore as the sole supported rollback.

This closes Roadmap 35. Per-tenant physical databases, billing counters, tenant switcher
UI, live side-effect replay, and all credential/OAuth/secret semantics (Roadmap 37) remain
explicitly out of scope.

## Technical Context

**Language/Version**: Go 1.24 (daemon); TypeScript 5.x (clients) — clients only consume
additive tenant fields, no logic changes required by this roadmap.
**Primary Dependencies**: stdlib `net/http` mux, `database/sql` with the existing
`modernc.org/sqlite` driver (pure-Go, already in `daemon/go.mod`), existing identity
package from Roadmap 34, existing in-process event bus (`daemon/internal/events`),
existing schema-migration framework (`daemon/internal/store/store.go`
`schemaMigrations` array, current head `CurrentSchemaVersion = 21`).
**Storage**: SQLite (single-file, shared schema with strong tenant scoping). Test env
`~/.kura-test`, prod env `~/.kura`. No new database technology introduced.
**Testing**: `go test ./...` for daemon; `make daemon-contract-test` for schema/contract
validation; new cross-tenant isolation regressions live alongside existing
`daemon/internal/store/*_test.go` files; pre-tenant fixture suite uses a checked-in
SQLite snapshot taken at schema version 21 (current).
**Target Platform**: Local-first daemon binary (macOS / Linux), single-process. Hosted
deployment uses the same binary; this roadmap does not change deploy topology.
**Project Type**: Server daemon (Go) + shared JSON Schema contracts (`schemas/`) + JS
client SDK (consumer of additive fields only).
**Performance Goals**: Per SC-010, post-migration p95 list latency MUST be ≤ 1.2× the
pre-migration p95 measured on the same representative fixture. Per FR-008, the
tenant-aware index MUST be selected for every in-scope tenant-scoped list path
(query-plan assertion).
**Constraints**: Migration is additive, restart-safe, and resume-safe per Q1 (durable
per-step progress; refuse to start only on detected unsafe state). Rollback is
backup-restore only per Q2 (no down-migration shipped). No cross-tenant admin projection
ships per Q4. Audit event MUST omit target tenant id and target row data per Q3.
**Scale/Scope**: Single-operator installs scaling into multi-tenant hosting. Assume up to
~100 tenants per daemon and ~10⁵ rows per tenant-owned table for the first hosted
release. Larger scale is a future concern; tenant-aware indexes are sized for this band.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Roadmap closure** — PASS. The plan closes Roadmap 35 end-to-end: schema inventory,
  forward migration, tenant-aware store access, indexes and uniqueness, cross-tenant
  isolation regressions, audit event, and rollback documentation. No partial slice is
  shipped.
- **Production-grade, minimal, reversible change** — PASS. Migration is additive only
  (column adds, backfill, index/constraint adds). Existing API routes and event payloads
  gain only additive tenant fields. Backup-restore is documented as the sole rollback;
  no breaking change is in scope. Blast radius is bounded to the in-scope persisted
  tables and their store helpers.
- **Contracts and auditability** — PASS. Every API/event/schema/persistence surface that
  changes is paired with: (a) a checked-in schema inventory entry under
  `specs/020-tenant-scoped-data-migration/contracts/`, (b) updated `schemas/api/` and
  `schemas/events/` payloads when additive fields are exposed, (c) `make
  daemon-contract-test` coverage. The inventory completeness test (FR-013) prevents
  silent table additions from bypassing classification.
- **Verification and observability** — PASS. Required tests: pre-tenant fixture
  migration test, cross-tenant isolation regressions per in-scope domain, schema
  inventory classification + completeness tests, store-layer tenantless-access rejection
  tests, query-plan assertions for tenant-aware indexes, relative no-regression latency
  check for high-volume list paths, contract tests for additive tenant fields, and the
  full daemon test suite after migration. Observability: typed audit event +
  greppable log on every denied cross-tenant access (Q3), structured migration progress
  log + event for operator visibility, no leakage of target tenant id or row data.
- **Environment and secrets** — PASS. Default execution is `~/.kura-test` on
  `127.0.0.1:19192`. No new secrets introduced. No live connector behavior changes. All
  credential, OAuth, secret-reference, and redaction semantics remain owned by Roadmap 37
  per FR-015; tables touched in common are made tenant-safe without redefining
  credential semantics.

No violations require justification. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/020-tenant-scoped-data-migration/
├── plan.md                # This file
├── research.md            # Phase 0 output
├── data-model.md          # Phase 1 output
├── quickstart.md          # Phase 1 output
├── contracts/
│   ├── schema-inventory.md         # Authoritative tenant classification per table/event
│   ├── tenant-audit-event.json     # JSON Schema for the cross-tenant denial audit event
│   └── migration-progress.md       # Per-step migration progress contract
├── checklists/
│   └── requirements.md    # /speckit.specify quality checklist (existing)
└── tasks.md               # /speckit.tasks output (NOT created by /speckit.plan)
```

### Source Code (repository root)

The work changes the daemon Go module, the JSON Schema contracts, and the cross-language
generated types. The clients (web, TUI, SDK) only consume additive tenant fields and need
no source changes from this roadmap beyond regeneration.

```text
daemon/
├── cmd/kura/                                  # entry point — unchanged
├── internal/
│   ├── tenantctx/                             # NEW: shared tenant-context carrier
│   │   ├── tenantctx.go                       # WithContext, FromContext, ErrTenantContextRequired
│   │   └── tenantctx_test.go
│   ├── store/
│   │   ├── store.go                           # add schemaMigrations entries (v22, v23, …)
│   │   ├── tenancy/                           # NEW: tenant-aware store helpers package
│   │   │   ├── tenancy.go                     # RequireTenant, MustTenant, scoped helpers
│   │   │   └── tenancy_test.go                # tenantless-access rejection + import-isolation tests
│   │   ├── migration_progress.go              # NEW: durable per-step progress tracking
│   │   ├── migration_progress_test.go         # NEW: resume-safety tests
│   │   ├── isolation_test.go                  # NEW: cross-tenant store isolation per domain
│   │   ├── store_test.go                      # update to include tenant-context fixtures
│   │   ├── identity_test.go                   # unchanged shape; reused for tenant context
│   │   └── *_test.go                          # extend per-domain tests with isolation cases
│   ├── api/
│   │   ├── server.go                          # update protected handlers to require tenant
│   │   ├── runs.go / schedules.go / …         # per-domain route handlers — scope by tenant
│   │   └── isolation_test.go                  # NEW: cross-tenant API isolation per domain
│   ├── events/
│   │   ├── bus.go                             # add TenantID to Filter; enforce on Publish
│   │   └── bus_test.go                        # NEW: cross-tenant event isolation
│   ├── identity/                              # Roadmap 34 — read-only consumer here
│   ├── inventory/                             # NEW: inventory loader + completeness test
│   │   ├── inventory.go                       # parses contracts/schema-inventory.md
│   │   └── inventory_test.go                  # FAILS if SQLite schema or event sources drift
│   └── audit/                                 # NEW: cross-tenant denial audit event emitter
│       ├── tenant_breach.go
│       └── tenant_breach_test.go
└── go.mod / go.sum                            # no new external deps expected

schemas/
├── api/<domain>/*.json                        # add additive `tenantId` where exposed
├── events/<domain>/*.json                     # add additive `tenantId` where exposed
└── events/audit/tenant-access-denied.json     # NEW: typed audit event schema (Q3)

docs/
└── runtime/
    └── tenant-migration-rollback.md           # NEW: backup-take + backup-restore runbook

specs/020-tenant-scoped-data-migration/
└── contracts/
    └── schema-inventory.md                    # authoritative inventory (FR-002, FR-013)
```

**Structure Decision**: Single Go module (`daemon/`) plus shared `schemas/` contracts.
The new tenant-aware store helpers live under `daemon/internal/store/tenancy/` so the
existing per-domain stores remain thin and the rule "tenant-owned reads require tenant
context" is enforced in one auditable place. The tenant-context carrier itself lives in
the new `daemon/internal/tenantctx/` package so both the api middleware and the
tenancy package can depend on it (the tenancy package never imports `daemon/internal/api`).
The schema inventory is checked in under `specs/020-tenant-scoped-data-migration/contracts/`
and consumed by a runtime test under `daemon/internal/inventory/` so that drift fails CI
rather than silently rotting.

## Complexity Tracking

> Filled only when Constitution Check has unjustified violations. None for this plan.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| (none)    |            |                                      |
