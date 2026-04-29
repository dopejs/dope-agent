package opsreadiness

import "testing"

func TestRestoreValidationRejectsLeakageRawCredentialsAndPartialSuccess(t *testing.T) {
	result := RestoreVerificationResult{
		BackupArtifactID: "backup_r39", RestoreEnvironment: EnvironmentTest,
		TenantRecordChecksPassed: 12, TenantRecordChecksTotal: 12,
		SecretReferenceChecks: []string{"references restored"}, QuotaStateChecks: []string{"quota matches"},
		WorkStateChecks:             []string{"work attributed to tenant"},
		CredentialRemediationStates: []string{"reconnect_required", "revalidation_required"},
		InvalidBackupFailedClearly:  true, Result: StatusPass,
	}
	assertValid(t, ValidateRestoreResult(result))

	result.CrossTenantLeakageObserved = true
	assertInvalidContains(t, ValidateRestoreResult(result), "cross-tenant")
}
