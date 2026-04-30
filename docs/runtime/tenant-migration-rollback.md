# Tenant-Scoped Data Migration — Operator Rollback Runbook

**Roadmap**: 35 — Tenant-Scoped Data Migration
**Spec**: [`specs/020-tenant-scoped-data-migration/`](../../specs/020-tenant-scoped-data-migration/)
**Audience**: operator running the daemon for themselves; on-call engineer triaging a failed upgrade.

Roadmap 39 production upgrade evidence reuses this rollback boundary: in-place
rollback is acceptable only when persisted state remains compatible with the
previous binary. If migration changed persisted state in a way that cannot be
safely reversed, restore from a verified backup is the canonical rollback path.
Roadmap 43 hosted evidence records that decision in a rollback decision record
linked from the release evidence index; the record must state whether in-place
rollback is safe, restore from backup is required, no rollback is needed, or
rollback is blocked.

This document describes (a) the **hard prerequisite** before upgrading the daemon to a release that crosses schema v22+, (b) the **rollback procedure** when the upgrade refuses to start or fails part-way, and (c) the **boundary conventions** that the regression suite enforces for the lifetime of Roadmap 35.

---

## (a) Hard prerequisite: take a backup before upgrade

Roadmap 35 introduces SQLite schema versions 22+ which:

- Add `tenant_id` columns to ~30 tenant-owned tables.
- Backfill those columns from parent FKs or the bootstrapped personal tenant.
- Recreate the runtime-spine tables with `NOT NULL` + `CHECK (tenant_id GLOB 'ten_*')` via shadow-table swap.
- Add tenant-aware UNIQUE indexes that replace some pre-existing global UNIQUE constraints.

**No down-migration is shipped.** Rolling back the schema means restoring a pre-upgrade backup — there is no `daemon migrate down` path.

Before running an upgrade across schema v22+ (any release tagged `>= roadmap-35-cut`):

```bash
# Stop the running daemon.
pkill -TERM dope || true

# Snapshot the database file. Use a date-tagged path so multiple upgrade
# attempts do not overwrite each other.
TS=$(date -u +%Y%m%dT%H%M%S)
cp ~/.dope/daemon.sqlite       ~/.dope/daemon.sqlite.pre-r35.${TS}.bak
cp ~/.dope-test/daemon.sqlite  ~/.dope-test/daemon.sqlite.pre-r35.${TS}.bak 2>/dev/null || true

# Verify the backup hash is stable (no concurrent writes during copy).
shasum -a 256 ~/.dope/daemon.sqlite ~/.dope/daemon.sqlite.pre-r35.${TS}.bak
```

The two SHA-256 lines MUST be identical. If they differ, the daemon was still writing — repeat after confirming `pgrep dope` returns nothing.

The migration's correctness is proven by `TestMigration_E2E_FixtureBackupRestore` (T081b): SHA-256 of the file before migration MUST match SHA-256 after copying the backup back.

---

## (b) Rollback procedure

### Symptom: daemon refuses to start

Look at the structured log for one of the migration lifecycle events:

| Log code / event name | Meaning | Action |
|---|---|---|
| `daemon.migration.failed` | At least one migration step is in `failed` status. The daemon will refuse to serve traffic and exit non-zero. | See "Recover from `daemon.migration.failed`" below. |
| `daemon.migration.orphan_detected` | A backfill found a row whose tenant binding cannot be derived. The driver fails fast rather than mis-classify the row. | See "Recover from `daemon.migration.orphan_detected`" below. |
| `daemon.migration.completed` | All steps moved to `completed`. The daemon proceeds to serve traffic. | No action. |

### Restore from backup (the canonical rollback)

If the daemon will not start AND you cannot resolve the underlying cause without shipping a fix, restore the pre-upgrade backup:

```bash
# Stop any half-started daemon.
pkill -TERM dope || true

# Replace the migrated DB with the pre-r35 backup.
cp ~/.dope/daemon.sqlite.pre-r35.${TS}.bak ~/.dope/daemon.sqlite

# Verify SHA matches the backup taken BEFORE the upgrade.
shasum -a 256 ~/.dope/daemon.sqlite ~/.dope/daemon.sqlite.pre-r35.${TS}.bak

# Downgrade the daemon binary to the pre-r35 release (the upgraded binary
# refuses to start against an older schema version).
brew install ./dope-pre-r35.bottle.tar.gz   # or your equivalent install path

# Start the pre-r35 daemon.
make daemon-run-prod
```

### Recover from `daemon.migration.failed`

A step is `failed` when its underlying SQL aborted (e.g., a schema swap detected a NULL tenant_id post-backfill, indicating a code bug). The recovery options, in order of preference:

1. **Investigate first** — the step name in the log identifies the offending domain. Reproduce locally with `migrationfixture.BuildPreTenantV21Fixture` + `app.New()`. Most failed steps reflect a regression that needs a code patch.
2. **Patch and retry** — once the cause is fixed, ship a patched binary. The daemon re-runs incomplete steps on next boot; `completed` steps are idempotent and skipped.
3. **Restore from backup** — if no patch is possible in the available recovery window, follow the restore procedure above.

`failed` rows are **not** auto-cleared. The daemon refuses to serve until every step is `completed`. `tenant_migration_progress` is queried by `MigrationGate` at boot — see `daemon/internal/app/tenant_migration_startup.go`.

### Recover from `daemon.migration.orphan_detected`

A row exists in a tenant-owned table whose tenant binding cannot be derived (e.g., a `connector_messages` row with both `session_id` and `run_id` NULL, or a `delivery_*` row with no resolvable parent). The driver fails fast with the table + key in the event payload.

1. Inspect the offending row(s) via `sqlite3 ~/.dope/daemon.sqlite`.
2. If the row is **garbage** (left over from a prior crash, never referenced), delete it manually then restart the daemon — the backfill will resume from `last_processed_key` and skip the now-deleted row.
3. If the row is **legitimate but un-bindable** (this should not happen post-Roadmap-35, since the events backfill carries a default-fallback for events with any populated linkage column), file a bug and restore from backup.

---

## (c) Boundary conventions enforced by the regression suite

Two conventions are kept in force for the lifetime of Roadmap 35. Both are enforced by tests that fail CI on violation:

### No admin / cross-tenant route

The literal comment marker `// admin: cross-tenant` is **forbidden** anywhere under `daemon/internal/api/` and `daemon/internal/store/`. The R35 daemon has NO sanctioned admin surface that crosses tenant boundaries; every route is wrapped in `protected(...)` (resolves a tenant context) or `withEnvironment(...)` (unauthenticated environment endpoint).

When Roadmap 36 lands the admin surface, the marker convention moves into the admin sub-package and this runbook section is updated.

Enforced by `TestNoAdminCrossTenantMarker` and `TestAllRoutesGoThroughTenantContext` in `daemon/internal/api/no_admin_route_test.go` (T089b).

### Roadmap 37 boundary

Roadmap 35 grants tenant ownership ONLY to `secret_scope_bindings`, `provider_preferences`, and `mcp_tool_exposure_rules` (Group A). It MUST NOT change DDL or store access for `provider_auth_states`, `mcp_servers`, `mcp_server_states`, `mcp_tools`, `connectors` (Group B) — those tables are owned by Roadmap 37.

Enforced by `TestR37GroupASchemaHasTenantID`, `TestR37GroupBSchemaHasNoTenantID`, `TestR37GroupBHasNoTenantHelper`, and the exported-signature golden `TestR37BoundarySignaturesGolden` in `daemon/internal/store/tenancy/r37_boundary_test.go` (T089c). Updates to the golden MUST be co-reviewed with the Roadmap 37 owner.

---

## Reference

- Migration progress table: `tenant_migration_progress` (added in schema v22).
- Migration step constants: `daemon/internal/store/migration_progress.go`.
- Drivers: `daemon/internal/app/tenant_backfill_*.go`, `tenant_enforcement_runner.go`, `tenant_migration_startup.go`.
- E2E regression suite: `daemon/internal/app/tenant_migration_e2e_test.go`.
- Schema inventory contract: [`specs/020-tenant-scoped-data-migration/contracts/schema-inventory.md`](../../specs/020-tenant-scoped-data-migration/contracts/schema-inventory.md).
