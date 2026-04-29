package opsreadiness

import "fmt"

func ValidateRestoreResult(result RestoreVerificationResult) error {
	errs := []error{
		RequireNonEmpty("backup artifact id", result.BackupArtifactID),
		RequireNonEmpty("restore environment", result.RestoreEnvironment),
		RequireItems("secret reference checks", result.SecretReferenceChecks),
		RequireItems("quota state checks", result.QuotaStateChecks),
		RequireItems("work state checks", result.WorkStateChecks),
		RequireItems("credential remediation states", result.CredentialRemediationStates),
		RequireNonEmpty("restore result", result.Result),
	}
	if result.TenantRecordChecksTotal <= 0 || result.TenantRecordChecksPassed != result.TenantRecordChecksTotal {
		errs = append(errs, fmt.Errorf("100%% of tenant record checks must pass"))
	}
	if result.CrossTenantLeakageObserved {
		errs = append(errs, fmt.Errorf("cross-tenant leakage observed"))
	}
	if result.RawCredentialMaterialFound {
		errs = append(errs, fmt.Errorf("raw credential material found"))
	}
	if result.PartialRestoreReportedPassed {
		errs = append(errs, fmt.Errorf("partial restore reported as passed"))
	}
	if !result.InvalidBackupFailedClearly {
		errs = append(errs, fmt.Errorf("invalid backup behavior is not proven"))
	}
	return JoinErrors(errs...)
}
