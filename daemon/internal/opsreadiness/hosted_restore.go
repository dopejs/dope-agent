package opsreadiness

import "fmt"

func ValidateHostedRestoreRehearsal(run HostedRun, result HostedRestoreRehearsalResult) error {
	errs := []error{
		RequireNonEmpty("restore result id", result.RestoreResultID),
		requireHostedRunIdentity(run, result.RunID, "", ""),
		RequireNonEmpty("backup id", result.BackupID),
		RequireNonEmpty("target profile id", result.TargetProfileID),
		RequireNonEmpty("target data directory", result.TargetDataDirectory),
		validateRepresentativeTenants(result.TenantCount, result.TenantStates),
		requireStatusPass("tenant state result", result.TenantStateResult),
		requireStatusPass("migration state result", result.MigrationStateResult),
		requireStatusPass("credential remediation result", result.CredentialRemediationResult),
		requireStatusPass("quota state result", result.QuotaStateResult),
		requireStatusPass("daemon health result", result.DaemonHealthResult),
		requireStatusPass("raw credential scan result", result.RawCredentialScanResult),
		requireStatusPass("restore result", result.Result),
		requireGeneratedAt("restore", result.GeneratedAt),
		ValidateHostedRedaction("restore", result),
	}
	if !result.TargetIsAlternate {
		errs = append(errs, fmt.Errorf("restore rehearsal must use an alternate target"))
	}
	if result.TenantCount < MinimumTenantCount {
		errs = append(errs, fmt.Errorf("restore rehearsal must cover at least 3 tenants"))
	}
	if result.CrossTenantLeakage {
		errs = append(errs, fmt.Errorf("restore rehearsal observed cross-tenant leakage"))
	}
	for _, tenant := range result.TenantStates {
		errs = append(errs, validateTenantStateSummary("restore tenant", tenant))
	}
	return JoinErrors(errs...)
}
