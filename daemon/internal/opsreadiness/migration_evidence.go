package opsreadiness

import "fmt"

func ValidateMigrationEvidence(report MigrationVerificationReport) error {
	return JoinErrors(
		RequireNonEmpty("source version", report.SourceVersion),
		RequireNonEmpty("target version", report.TargetVersion),
		RequireItems("preflight checks", report.PreflightChecks),
		RequireItems("postflight checks", report.PostflightChecks),
		RequireNonEmpty("migration progress", report.MigrationProgress),
		RequireNonEmpty("tenant integrity summary", report.TenantIntegritySummary),
		RequireNonEmpty("quota accounting summary", report.QuotaAccountingSummary),
		RequireNonEmpty("credential remediation", report.CredentialRemediation),
		RequireNonEmpty("rollback path", report.RollbackPath),
		RequireNonEmpty("result", report.Result),
		RequireItems("operator diagnostics", report.OperatorDiagnostics),
	)
}

func ValidateUpgradeRunbook(evidence RunbookEvidence) error {
	return JoinErrors(
		RequireNonEmpty("runbook name", evidence.Name),
		RequireItems("upgrade steps", evidence.Steps),
		RequireElapsedAtMost("upgrade", evidence.Elapsed, MaxUpgradeElapsed),
		RequireItems("health checks", evidence.HealthChecks),
		RequireItems("diagnostics", evidence.Diagnostics),
		RequireItems("failure modes", evidence.FailureModes),
		RequireItems("rollback decision points", evidence.RollbackOrCleanup),
		requireNoProductionData("upgrade runbook", evidence),
	)
}

func ValidateInstallRunbook(evidence RunbookEvidence) error {
	return JoinErrors(
		RequireNonEmpty("runbook name", evidence.Name),
		RequireItems("install steps", evidence.Steps),
		RequireElapsedAtMost("install", evidence.Elapsed, MaxInstallElapsed),
		RequireItems("health checks", evidence.HealthChecks),
		RequireItems("diagnostics", evidence.Diagnostics),
		RequireItems("failure modes", evidence.FailureModes),
		RequireItems("cleanup", evidence.RollbackOrCleanup),
		requireNoProductionData("install runbook", evidence),
	)
}

func requireNoProductionData(label string, evidence RunbookEvidence) error {
	if evidence.UsedProductionData && !evidence.ProductionOptIn {
		return fmt.Errorf("%s used production data without explicit opt-in", label)
	}
	return nil
}
