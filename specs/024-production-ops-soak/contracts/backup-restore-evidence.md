# Contract: Backup, Restore, Migration, And Rollback Evidence

This contract defines the evidence required for install/upgrade, backup/restore, migration
verification, and rollback readiness.

## Backup Contents

Backup workflows must document and verify:

- daemon state source path and environment
- schema or product version of the source state
- tenant-scoped records included in the snapshot
- quota and usage accounting state included in the snapshot
- work, schedule, delivery, approval, and evaluation state included in the snapshot
- secret metadata and references included in the snapshot
- excluded material

Excluded material must include:

- raw secret values
- OAuth authorization codes
- access tokens
- refresh tokens
- provider tokens
- local CLI auth material
- derived credential material

## Representative Data Set

Backup/restore regression evidence must use at least three tenants:

| Tenant Shape | Required State |
|--------------|----------------|
| Tenant A | active credential references, finite quota usage, in-progress or recently completed work |
| Tenant B | distinct credential references, different quota state, same-shaped resources for isolation checks |
| Tenant C | disabled/reconnect-needed credential state, operator-action-needed or retry/recovery state where practical |

The exact tenant ids may be fixture-specific, but they must be non-production fake ids.

## Restore Validation

Restore evidence must prove:

- 100% of expected tenant-owned records are available to the correct tenant
- no tenant can read or mutate another tenant's restored records
- quota and usage state match expected tenant ownership
- work, schedule, delivery, approval, and evaluation state remains attributable to the
  correct tenant
- secret references and redacted metadata are present
- credential-bearing integrations remain disabled until reconnect or revalidation
- zero raw credential material appears in restored state, logs, reports, fixtures, events,
  or diagnostics

## Invalid Backup Behavior

Restore must fail clearly when:

- the backup is incomplete
- the backup is incompatible with the target version
- integrity checks fail
- required metadata is missing
- tenant ownership cannot be verified

Partial restore must not be reported as success.

## Migration Verification

Upgrade evidence must include:

- preflight check result before migration starts
- backup presence and integrity result
- migration progress or lifecycle state
- postflight tenant integrity checks
- quota/accounting consistency checks
- credential remediation state
- rollback decision

Failed migrations must surface operator-visible diagnostics. If persisted state cannot be
safely reversed in place, restore from verified backup is the only acceptable rollback.

## Contract Tests

Required tests or checks:

- representative multi-tenant backup fixture has at least three tenants with distinct
  credential, quota, and work states
- restore validation fails on cross-tenant leakage
- restore validation fails when raw credential material appears
- restore validation fails when credential-bearing integrations are usable before
  reconnect/revalidation
- migration preflight/postflight evidence is required before upgrade readiness passes
- rollback guidance names backup-restore when no safe in-place rollback exists

Final implementation fixtures:

- `daemon/internal/store/migrationfixture/r39_production_ops.go`
- `specs/024-production-ops-soak/fixtures/README.md`
