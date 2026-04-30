package opsreadiness

import "testing"

func sampleHostedRestoreResult() HostedRestoreRehearsalResult {
	return HostedRestoreRehearsalResult{
		RestoreResultID:             "restore_hosted_1",
		RunID:                       sampleHostedRun().RunID,
		BackupID:                    "backup_hosted_1",
		TargetProfileID:             "profile_hosted_restore",
		TargetDataDirectory:         "~/.dope-test-restore",
		TargetIsAlternate:           true,
		TenantCount:                 3,
		TenantStates:                sampleTenantState(),
		TenantStateResult:           StatusPass,
		MigrationStateResult:        StatusPass,
		CredentialRemediationResult: StatusPass,
		QuotaStateResult:            StatusPass,
		DaemonHealthResult:          StatusPass,
		CrossTenantLeakage:          false,
		RawCredentialScanResult:     StatusPass,
		Result:                      StatusPass,
		GeneratedAt:                 hostedNow,
	}
}

func TestHostedRestoreRequiresAlternateTargetThreeTenantsAndNoLeakage(t *testing.T) {
	result := sampleHostedRestoreResult()
	assertValid(t, ValidateHostedRestoreRehearsal(sampleHostedRun(), result))

	result.TargetIsAlternate = false
	assertInvalidContains(t, ValidateHostedRestoreRehearsal(sampleHostedRun(), result), "alternate")

	result = sampleHostedRestoreResult()
	result.TenantCount = 2
	result.TenantStates = result.TenantStates[:2]
	assertInvalidContains(t, ValidateHostedRestoreRehearsal(sampleHostedRun(), result), "3 tenants")

	result = sampleHostedRestoreResult()
	result.CrossTenantLeakage = true
	assertInvalidContains(t, ValidateHostedRestoreRehearsal(sampleHostedRun(), result), "cross-tenant")
}
