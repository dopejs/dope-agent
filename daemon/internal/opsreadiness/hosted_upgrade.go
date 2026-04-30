package opsreadiness

import "fmt"

func ValidateHostedUpgradeEvidence(run HostedRun, evidence HostedUpgradeEvidence) error {
	errs := []error{
		RequireNonEmpty("upgrade evidence id", evidence.UpgradeEvidenceID),
		requireHostedRunIdentity(run, evidence.RunID, evidence.ProfileIdentity, ""),
		requireAllowed("upgrade phase", evidence.Phase, []string{HostedUpgradePhasePreflight, HostedUpgradePhasePostflight}),
		requireGeneratedAt("upgrade", evidence.GeneratedAt),
		ValidateHostedRedaction("upgrade", evidence),
	}
	if len(evidence.BlockingFindings) > 0 {
		errs = append(errs, fmt.Errorf("upgrade evidence has blocking findings"))
		if evidence.FailureOwner != "" {
			errs = append(errs, ValidateHostedFailureOwner(evidence.FailureOwner))
		}
	}
	switch evidence.Phase {
	case HostedUpgradePhasePreflight:
		errs = append(errs,
			RequireNonEmpty("deployment identity", evidence.DeploymentIdentity),
			RequireNonEmpty("profile identity", evidence.ProfileIdentity),
			RequireNonEmpty("data location", evidence.DataLocation),
			RequireNonEmpty("artifact location", evidence.ArtifactLocation),
			requireStatusPass("required backup state", evidence.RequiredBackupState),
			requireStatusPass("daemon health", evidence.DaemonHealth),
			requireStatusPass("configuration readiness", evidence.ConfigurationReadiness),
		)
	case HostedUpgradePhasePostflight:
		errs = append(errs,
			requireStatusPass("daemon health", evidence.DaemonHealth),
			requireStatusPass("tenant data verification", evidence.TenantDataVerification),
			requireStatusPass("migration state", evidence.MigrationState),
			requireStatusPass("credential remediation state", evidence.CredentialRemediationState),
			requireStatusPass("quota state", evidence.QuotaState),
			requireStatusPass("operational diagnostics", evidence.OperationalDiagnostics),
			RequireNonEmpty("rollback guidance", evidence.RollbackGuidance),
		)
	}
	return JoinErrors(errs...)
}
