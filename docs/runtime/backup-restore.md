# Backup And Restore Runbook

Roadmap 39 backup and restore validation covers a representative multi-tenant
test data set with at least three tenants that have distinct credential, quota,
and work states.

## Backup Contents

Backups include tenant-scoped daemon records, quota and usage accounting state,
work/schedule/delivery/approval/evaluation state, secret metadata, and secret
references.

Backups exclude raw secret values, OAuth authorization codes, access tokens,
refresh tokens, provider tokens, local CLI auth material, and derived credential
material.

## Backup

```bash
scripts/production/backup-test-state.sh
```

Record source version, source environment, artifact path, SHA-256 output,
tenant summary, included material, excluded material, and compatibility notes.

## Restore

Restore only into an isolated environment unless this is an explicit production
recovery:

```bash
scripts/production/restore-test-state.sh <backup-artifact>
```

Verify that 100% of expected tenant-owned records are available to the correct
tenant, cross-tenant leakage is zero, quota state matches tenant ownership, work
state remains attributable to the correct tenant, and raw credential material is
absent from restored state, logs, reports, fixtures, events, and diagnostics.

When the Roadmap 39 fixture tables are present, `restore-test-state.sh` validates
that at least three tenants exist and scans restored secret references for raw
credential markers before reporting success.

Credential-bearing integrations must remain blocked until reconnect or
revalidation.

Invalid, incomplete, or incompatible backups must fail clearly. Partial restore
must not be reported as success.

## Hosted Alternate-Target Rehearsal

For Roadmap 43 hosted evidence, set a run identity and restore into an
alternate target:

```bash
DOPE_HOSTED_RUN_ID=hosted_$(date -u +%Y%m%dT%H%M%SZ) \
DOPE_RESTORE_TARGET_DIR=~/.dope-test-restore \
scripts/production/restore-test-state.sh <backup-artifact>
```

The hosted restore evidence records `targetIsAlternate`, tenant count, tenant
state, migration state, credential remediation, quota state, daemon health,
cross-tenant leakage, and raw-credential scan result. Release readiness requires
at least three tenants with distinct credential, quota, and work states.
