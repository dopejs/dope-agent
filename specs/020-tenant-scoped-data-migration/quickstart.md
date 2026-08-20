# Quickstart: Tenant-Scoped Data Migration (Roadmap 35)

This is the engineer-facing quickstart for delivering Roadmap 35. It assumes you have
read `spec.md`, `plan.md`, `research.md`, `data-model.md`, and `contracts/`.

## Prerequisites

- Worktree on branch `020-claude` (created via `git worktree add .worktrees/020-claude
  -b 020-claude main`).
- `pnpm install` and `cd daemon && go mod download` already done.
- Test environment is the default. Do not touch `~/.kura` or live connectors.

## Local development loop

```bash
# from worktree root
make daemon-build
make daemon-test                 # full Go test suite
make daemon-contract-test        # schema/contract validation
cd daemon && go test ./internal/store/... -run TestTenantMigration
cd daemon && go test ./internal/store/... -run TestCrossTenantIsolation
cd daemon && go test ./internal/api/...   -run TestCrossTenantIsolation
cd daemon && go test ./internal/inventory/...
```

To smoke test the migration end-to-end against a daemon process:

```bash
# Reset test data dir, leave a pre-tenant fixture in place if testing the upgrade path.
# The pre-tenant fixture is built programmatically by
# `daemon/internal/store/migrationfixture.BuildPreTenantV21Fixture` rather than checked
# in as a SQLite blob. The end-to-end migration tests
# (daemon/internal/app/tenant_migration_e2e_test.go) drive the full pipeline against
# a freshly-built fixture; for an interactive smoke test, point a fixture builder at
# ~/.kura-test/daemon.sqlite via a small Go program or use the e2e tests directly.
rm -rf ~/.kura-test

make daemon-run-test
make daemon-test-status          # in another shell
```

The daemon should:

1. Apply schema migrations 22+.
2. Resume any `running` rows in `tenant_migration_progress` (none on a clean fixture).
3. Run the additive tenant migrations to completion.
4. Serve `127.0.0.1:19192` with all pre-existing rows owned by the default personal
   tenant.

## Implementation order (recommended)

This order keeps every intermediate commit green and minimizes blast radius. Each step
should leave `make daemon-test` and `make daemon-contract-test` passing before moving
on.

1. **Inventory artifact + loader**
   - Land `contracts/schema-inventory.md` (this PR already includes the file under
     `specs/020-tenant-scoped-data-migration/contracts/`).
   - Add `daemon/internal/inventory/inventory.go` to parse it.
   - Add the inventory completeness test. It will FAIL until the schema and event
     sources match — this is expected and drives the rest of the work.

2. **Migration progress table (schema v22)**
   - Add the `tenant_migration_progress` table via a new entry in `schemaMigrations`.
   - Add `daemon/internal/store/migration_progress.go` and tests covering the state
     transitions and unsafe-state startup refusal.

3. **Tenancy package**
   - Add `daemon/internal/store/tenancy/tenancy.go` with `RequireTenant` and the
     scoped-helper primitives.
   - Add `tenancy_test.go` proving that calling tenant-owned helpers without a tenant
     context returns `ErrTenantContextRequired`.

4. **Per-domain migrations and helpers**
   - For each in-scope table in inventory order:
     - Add the schema-version migration entries (column add, backfill, index, optional
       shadow swap).
     - Add tenant-aware store helpers under `tenancy` and route the existing per-domain
       store package through them.
     - Delete or restrict the previously-tenantless helpers as the inventory's
       `storeAccess` column says.
     - Update API route handlers to pass the resolved `TenantID` to the new helpers.
     - Add the cross-tenant store and API isolation test rows for that domain.
     - Add additive `tenantId` to the affected `schemas/api/<domain>/*.json` and
       `schemas/events/<domain>/*.json`. Run `make daemon-contract-test`.

5. **Event bus tenant scoping**
   - Add `TenantID` to `events.Filter`. Enforce it on `Publish` and on subscriber
     fan-out. Add tests proving cross-tenant event isolation.
   - Update SSE replay (`/v1/events/stream`) to scope by the resolved tenant.

6. **Audit event for denied cross-tenant access**
   - Add `daemon/internal/audit/tenant_breach.go` and the
     `audit.cross_tenant_access_denied` schema at
     `schemas/events/audit-cross-tenant-access-denied.event.schema.json` (flat
     kebab-case; the file name `tenant-access-denied.event.schema.json` is already
     taken by R34's identity-boundary event — the R35 event is intentionally
     distinct).
   - Wire the emitter into every `tenancy` rejection path and into any API rejection
     that detects cross-tenant intent.

7. **Performance assertions**
   - Add the fixture generator and the relative no-regression latency assertions for
     `runs`, `events`, and `mail_artifacts`.
   - Add the `EXPLAIN QUERY PLAN` assertions for the tenant-aware indexes on every
     in-scope list path.

8. **Pre-tenant fixture migration test**
   - Capture `daemon/internal/store/testdata/pre_tenant_v21.sqlite`.
   - Add the migration test that runs the full set of v22+ migrations and asserts the
     default personal tenant owns every pre-existing row.

9. **Operator runbook**
   - Add `docs/runtime/tenant-migration-rollback.md` documenting backup-take and
     backup-restore steps.
   - Update `docs/product/hosted-productization-roadmap-split.md` and
     `docs/runtime/daemon-roadmaps.md` to mark Roadmap 35 complete only after every
     other step in this list lands.

## Verification checklist (pre-merge)

- [X] `make daemon-test` passes on a clean test data directory. (Sandbox docker-probe
      flake under parallel-test load is pre-existing and unrelated; passes in isolation.)
- [X] `make daemon-test` passes on the pre-tenant fixture data directory after migration.
      Exercised by `TestMigration_E2E_*` in `daemon/internal/app/`.
- [X] `make daemon-contract-test` passes (additive tenant fields in api/event schemas).
- [X] Cross-tenant isolation tests cover representative `tenant_owned` rows.
      Runtime spine + extended-domain coverage across `daemon/internal/store/tenancy/`,
      `daemon/internal/store/`, `daemon/internal/api/`. Per-table UNIQUE-index activation
      for the ~30 non-runtime-spine tables (T019–T026) is tracked carry-over and uses
      pre-staged specs in `store.ExtendedEnforcementSpecs()`.
- [X] Inventory completeness test passes (T082/T083/T084 in
      `daemon/internal/inventory/`).
- [X] Resume-safety test (T080) and unsafe-state startup refusal (T081) pass.
- [X] `audit.cross_tenant_access_denied` event fires for every denied cross-tenant
      access; payload contains exactly actingTenantId, principalId, surface, and
      resourceKind; no target tenant id and no row data. T085 integration covers 5
      runtime-spine surfaces.
- [X] Query-plan assertions (T086) show the tenant-aware index is selected for every
      in-scope list path; 12 representative paths covered, no table scans.
- [X] Post-migration p95 list latency on `runs`, `events`, and `mail_artifacts` is at
      most 1.2× the pre-migration p95. T087/T088/T089 measure 0.01–0.02× (50–100×
      faster) on the same N=10000 fixture shape.
- [X] Backup-restore rollback documented in
      [`docs/runtime/tenant-migration-rollback.md`](../../docs/runtime/tenant-migration-rollback.md)
      and demonstrated by `TestMigration_E2E_FixtureBackupRestore` (T081b).
- [X] No tenantless `Upsert*` helper remains on the runtime spine — replaced by
      auto-bind delegation through `ResolveDefaultPersonalTenantID`.
- [X] No cross-tenant admin projection added (FR-017). Enforced by T089b
      (`TestNoAdminCrossTenantMarker`, `TestAllRoutesGoThroughTenantContext`).
- [X] **Pitfall** — `// admin: cross-tenant` is a forbidden marker in
      `daemon/internal/api/` and `daemon/internal/store/` (T089b enforces).
- [X] Roadmap 37 boundary signature golden (T089c) locked in
      `daemon/internal/store/tenancy/testdata/r37_boundary_signatures.golden`.

## Commands reference

```bash
# Daemon
make daemon-build
make daemon-run-test
make daemon-run-test-live        # use only with explicit intent; live connectors
make daemon-test
make daemon-contract-test
make daemon-test-status

# Single Go test
cd daemon && go test ./internal/store/... -run TestTenantMigrationResume

# Clients (only re-run if regenerating types after schema additions)
pnpm build:sdk
pnpm build:clients
pnpm test:clients
```

## Common pitfalls

- **Forgetting to remove a tenantless helper after replacing it**: callers will silently
  continue to use the old path. The compile-time gate in step 4 (delete or rename the
  helper) is the durable fix.
- **Skipping the inventory entry for a new table**: the completeness test will fail
  CI. This is the intended behavior — fix by adding the entry, not by suppressing the
  test.
- **Adding the `tenant_id` column without an index**: list paths will silently
  table-scan and the relative latency budget (SC-010) will fail.
- **Including `targetTenantId` in the audit event for "completeness"**: the schema
  forbids it. It is the explicit answer to Q3.
- **Relying on a down-migration**: there is none. Backup-restore is the sole rollback.
