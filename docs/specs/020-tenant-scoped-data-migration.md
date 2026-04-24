# Tenant-Scoped Data Migration

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 35, the migration
that makes daemon-owned runtime, product, and harness records tenant-scoped.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md`
- `docs/specs/019-tenant-identity-and-access-foundation.md`
- `docs/runtime/daemon-roadmaps.md`

## Background

Tenant identity by itself is not sufficient. Existing resources must be owned by a tenant
and every read, write, list, event, replay, and operator projection must enforce that owner
boundary.

## Goal

Add tenant ownership to core persisted records and API paths while migrating existing
single-user data into the default personal tenant.

## Fixed Decisions

- The first implementation uses shared database tables with strong tenant scoping.
- A storage resolver abstraction should be introduced where needed so later roadmaps can
  support per-tenant storage without rewriting every domain.
- Existing data migrates into the default personal tenant.
- Cross-tenant reads and writes are correctness bugs, not UI filtering issues.

## Dependencies On Completed Phases

- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 33: Evaluation And Replay Harness

## In Scope

- additive `tenantId` ownership on core daemon resources
- migration of existing rows into the default personal tenant
- tenant-scoped list, get, create, update, delete, event, and SSE behavior
- tenant-aware replay, schedule, workflow, run, delivery, integration, calendar, mail,
  reminder, computer-use, and evaluation records
- cross-tenant isolation tests
- operational rollback notes for migration failure

## Out Of Scope

- per-tenant physical databases
- billing usage counters
- tenant switcher UI
- live side-effect replay

## Operator Or User Problems To Solve

- Hosted users must not see or mutate another tenant's records.
- Existing local users must not lose data during tenant migration.
- Operators need clear migration and rollback behavior under production pressure.

## User Stories

- As a hosted user, I only see runs, schedules, integrations, and evaluations owned by the
  selected tenant.
- As an operator upgrading from local-first data, my existing records appear in my personal
  tenant after migration.
- As an engineer, I can prove cross-tenant API access fails in every migrated domain.

## Functional Requirements

- The migration MUST add tenant ownership to all in-scope persisted records.
- Existing rows MUST be assigned to the default personal tenant during migration.
- APIs MUST scope all in-scope resource access by resolved tenant context.
- Events and SSE replay MUST not leak events across tenants.
- The system MUST include tests that intentionally create same-shaped resources in two
  tenants and prove isolation.

## Compatibility And Operational Notes

- Migration must be additive and restart-safe.
- Backups taken before migration must remain restorable through the documented upgrade path.
- Rollback must be documented as a database backup restore or explicit down-migration if a
  down-migration is implemented.

## Verification Expectations

- Migration tests from pre-tenant fixtures.
- Cross-tenant API and store isolation regressions.
- Contract tests for additive tenant fields where exposed.
- Full daemon tests after migration.

## Definition Of Done

- Core daemon state is tenant-owned and cannot be accessed cross-tenant through normal
  APIs, event streams, or replay surfaces.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/020-tenant-scoped-data-migration.md 完成 phase 35 的工作`
