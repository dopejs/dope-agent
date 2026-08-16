# Tenant-Scoped Data Migration

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 35, the migration
that makes daemon-owned runtime, product, and harness records tenant-scoped.

Primary source documents:
- `docs/product/hosted-productization-roadmap-split.md` (removed 2026-08, in git history)
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
- Every persisted table must be explicitly classified as `tenant_owned`, `global`, or
  `derived`; unclassified tables block roadmap completion.
- Tenant-owned tables must reject tenantless writes and should use tenant-aware indexes and
  uniqueness constraints where the existing access pattern requires them.

## Dependencies On Completed Phases

- Roadmap 34: Tenant Identity And Access Foundation
- Roadmap 33: Evaluation And Replay Harness

## In Scope

- additive `tenantId` ownership on core daemon resources
- migration of existing rows into the default personal tenant
- tenant-scoped list, get, create, update, delete, event, and SSE behavior
- tenant-aware replay, schedule, workflow, run, delivery, integration, calendar, mail,
  reminder, computer-use, and evaluation records
- schema inventory that classifies every persisted table and event-bearing record
- tenant-aware store helpers or query guards that prevent tenant-owned lookups without
  tenant context
- tenant-aware indexes and uniqueness constraints for migrated tenant-owned tables
- cross-tenant isolation tests
- operational rollback notes for migration failure

## Required Design Artifacts

Implementation planning MUST produce a tenant-scope inventory before code changes begin.
The inventory can be Markdown or JSON, but it MUST include one row per persisted table and
event-bearing record with these fields:

- `name`: table, event stream, or persisted projection name
- `classification`: `tenant_owned`, `global`, or `derived`
- `tenantIdSource`: existing column, backfilled default personal tenant, parent resource,
  authenticated tenant context, or `not_applicable`
- `migrationAction`: add column, backfill, rebuild index, leave global, derive at read
  time, or remove tenantless access
- `affectedAPIs`: list/get/create/update/delete routes that must read resolved tenant
  context
- `affectedEvents`: event names and SSE replay filters that must include tenant scope
- `storeAccess`: tenant-aware helper/query required and tenantless helper to delete or
  restrict
- `indexesAndUniqueness`: tenant-aware list indexes and natural-key uniqueness changes
- `isolationTests`: same-shaped cross-tenant fixtures required for API and store coverage
- `rollback`: restore-from-backup, reversible down migration, or operator action

The implementation plan MUST fail if any persisted table, replay fixture source, event
history source, or operator projection source is absent from this inventory.

## 020/022 Ownership Boundary

This roadmap owns the generic tenant migration mechanics:

- adding and backfilling tenant ownership for existing persisted records
- enforcing tenant-aware store access and API/event/SSE filtering
- classifying every persisted table and event-bearing record
- adding tenant-aware indexes and uniqueness constraints

Roadmap 37 owns credential and external-account semantics after the generic migration:

- tenant-scoped secret value storage and redaction policy
- OAuth/provider auth lifecycle and credential rotation behavior
- connector, MCP, and sandbox policy administration permissions
- secret reference resolution at execution time
- replay, fixture, log, and event redaction for secret material

If a table is touched by both roadmaps, Roadmap 35 must make ownership explicit and safe
for tenant filtering; Roadmap 37 must make the credential semantics safe for hosted use.

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
- The migration MUST classify every persisted table as `tenant_owned`, `global`, or
  `derived`, and the implementation plan MUST reject unclassified tables.
- Existing rows MUST be assigned to the default personal tenant during migration.
- APIs MUST scope all in-scope resource access by resolved tenant context.
- Events and SSE replay MUST not leak events across tenants.
- Store-layer access for tenant-owned records MUST require tenant context rather than
  relying only on API-layer filtering.
- Tenant-owned tables MUST have tenant-aware indexes for common list/get paths and
  tenant-aware uniqueness constraints for names or natural keys that were previously global.
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
- Schema inventory test proving every persisted table is classified.
- Inventory completeness test proving the checked-in inventory matches the actual SQLite
  schema and persisted event sources.
- Store tests proving tenant-owned queries without tenant context fail or are unavailable.
- Index or query-plan smoke for high-volume tenant-owned list paths where supported by the
  local database test environment.
- Contract tests for additive tenant fields where exposed.
- Full daemon tests after migration.

## Definition Of Done

- Core daemon state is tenant-owned and cannot be accessed cross-tenant through normal
  APIs, event streams, or replay surfaces.
- No tenant-owned persisted table remains unclassified, tenantless, or reachable through a
  tenantless store path.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/020-tenant-scoped-data-migration.md 完成 phase 35 的工作`
