# Migration And Versioning Plan

## Purpose

This document defines how daemon persisted state evolves without silent corruption.

P0 uses SQLite as the local durable store. Schema evolution is versioned and explicit.

## Current Strategy

- current supported schema version: `21`
- migration ledger table: `schema_migrations`
- version rule: the daemon applies forward migrations in ascending order
- compatibility rule: a database newer than the daemon-supported version is rejected on startup

The migration ledger records:

- `version`
- `name`
- `applied_at`

## Migration Semantics

### New Databases

For a brand-new database:

1. create `schema_migrations`
2. apply migration `1` baseline schema
3. apply later migrations in order

### Legacy Unversioned Databases

Older local databases may already contain the baseline tables but no migration ledger.

The daemon handles that case explicitly:

1. detect known legacy tables
2. bootstrap migration `1` in `schema_migrations`
3. continue applying newer migrations

This avoids forcing a manual destructive reset for early local installations.

### Future Databases

If the local database reports a schema version greater than the daemon understands, startup fails.

This is intentional. Running an older daemon binary against newer persisted state is a rollback risk.

## Rollback Expectations

P0 migrations are forward-only.

There are no automatic down migrations.

Rollback expectation:

1. stop the daemon
2. restore the previous SQLite file from backup or snapshot
3. restart a daemon binary compatible with that restored schema version

For any migration that changes persisted semantics beyond additive indexes or metadata, release notes must say whether a pre-upgrade backup is mandatory.

## Authoring Rules

Every new persisted schema change must:

1. increment `CurrentSchemaVersion`
2. add a named migration entry in `daemon/internal/store/store.go`
3. keep the migration idempotent
4. document rollback expectations
5. add at least one store-level migration test

## Current Coverage

The store test suite now covers:

- new database reaches current schema version
- legacy baseline schema upgrades to current version
- future schema version is rejected
- tenant identity tables, token lifecycle fields, token tenant grants, organization
  memberships, and invitations persist across restart

That is the minimum P0 migration confidence bar.

## Latest Tenant Identity Migration

Schema version `21` adds the Roadmap 34 tenant identity foundation:

- `tenants`
- `principals`
- `memberships`
- `tenant_invitations`
- `token_tenant_grants`
- token lifecycle columns on `auth_tokens`
- `tenant_audit_events`

Rollback requires restoring a pre-upgrade SQLite backup. The daemon does not down-migrate
tenant identity records or widen token authority during rollback.
