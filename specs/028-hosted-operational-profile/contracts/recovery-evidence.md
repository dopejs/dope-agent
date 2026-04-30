# Contract: Recovery Evidence

## Goal

Define hosted upgrade, backup, restore, and rollback evidence.

## Upgrade Preflight Evidence

Required fields:

- `runId`
- `deploymentIdentity`
- `profileIdentity`
- `dataLocation`
- `artifactLocation`
- `requiredBackupState`
- `daemonHealth`
- `configurationReadiness`
- `blockingFindings`
- `generatedAt`

Missing or failed preflight evidence is a no-ship condition.

## Upgrade Postflight Evidence

Required fields:

- `runId`
- `daemonHealth`
- `tenantDataVerification`
- `migrationState`
- `credentialRemediationState`
- `quotaState`
- `operationalDiagnostics`
- `rollbackGuidance`
- `blockingFindings`
- `generatedAt`

Missing or failed postflight evidence is a no-ship condition.

## Backup Evidence

Required fields:

- `backupId`
- `runId`
- `sourceProfileId`
- `sourceCommitOrVersion`
- `artifactPath`
- `checksum`
- `tenantSummary`
- `includedMaterial`
- `excludedMaterial`
- `compatibilityNotes`
- `redactionStatus`
- `generatedAt`

Backups include secret metadata and references only. Backups exclude raw credential
material, OAuth authorization codes, access tokens, refresh tokens, provider tokens,
local CLI auth material, and derived credential material.

## Restore Rehearsal Evidence

Required fields:

- `restoreResultId`
- `runId`
- `backupId`
- `targetDataDirectory`
- `targetIsAlternate`
- `tenantCount`
- `tenantStateResult`
- `migrationStateResult`
- `credentialRemediationResult`
- `quotaStateResult`
- `daemonHealthResult`
- `crossTenantLeakage`
- `rawCredentialScanResult`
- `result`
- `generatedAt`

Restore rehearsal must use an alternate directory or instance. The representative data
set must contain at least three tenants with distinct credential, quota, and work states.

## Rollback Decision Record

Required fields:

- `rollbackDecisionId`
- `runId`
- `trigger`
- `decision`
- `rationale`
- `requiredBackupId`
- `supportingEvidenceLinks`
- `operator`
- `decidedAt`

Allowed decisions:

- `in_place_rollback`
- `restore_from_backup_required`
- `no_rollback_needed`
- `blocked`

## No-Ship Rules

Release readiness is `no_ship` when:

- upgrade preflight is missing or failed
- upgrade postflight is missing or failed
- backup evidence is missing, incomplete, incompatible, or unredacted
- restore rehearsal does not use an alternate target
- restore rehearsal covers fewer than three tenants
- restored tenant data, migration state, credential remediation, quota state, or daemon
  health fails
- cross-tenant leakage is observed
- raw credential material is detected
- rollback decision does not state whether restore from backup is required

## Contract Tests

- Fixture file: `daemon/internal/opsreadiness/testdata/hosted/recovery_evidence.json`
- Implemented commands: `scripts/production/backup-test-state.sh`,
  `scripts/production/restore-test-state.sh`,
  `scripts/production/upgrade-preflight.sh`, and
  `scripts/production/upgrade-postflight.sh`
- restore validation fails with fewer than three representative tenants
- restore validation fails when target equals active hosted profile data directory
- redaction checks fail when raw credential markers appear
- rollback decision fixture links preflight, postflight, backup, and restore evidence
- release index treats any failed recovery evidence as no-ship
