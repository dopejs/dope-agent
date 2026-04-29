package opsreadiness

import (
	"testing"
	"time"
)

func TestInstallAndUpgradeRunbookEvidenceEnforcesElapsedAndEnvironment(t *testing.T) {
	install := RunbookEvidence{
		Name: "production install", Steps: []string{"prepare host", "start daemon"},
		Elapsed: 55 * time.Minute, HealthChecks: []string{"daemon health"},
		Diagnostics: []string{"logs"}, FailureModes: []string{"port in use"},
		RollbackOrCleanup: []string{"stop daemon"}, TestEnvironment: true,
	}
	assertValid(t, ValidateInstallRunbook(install))

	install.Elapsed = 61 * time.Minute
	assertInvalidContains(t, ValidateInstallRunbook(install), "exceeds")

	upgrade := RunbookEvidence{
		Name: "production upgrade", Steps: []string{"preflight", "upgrade", "postflight"},
		Elapsed: 89 * time.Minute, HealthChecks: []string{"postflight evidence"},
		Diagnostics: []string{"migration events"}, FailureModes: []string{"failed migration"},
		RollbackOrCleanup: []string{"restore_from_backup"}, TestEnvironment: true,
	}
	assertValid(t, ValidateUpgradeRunbook(upgrade))
}

func TestMigrationEvidenceRequiresPreflightPostflightAndRollback(t *testing.T) {
	report := MigrationVerificationReport{
		SourceVersion: "v38", TargetVersion: "v39",
		PreflightChecks:   []string{"backup integrity", "tenant integrity"},
		PostflightChecks:  []string{"tenant integrity", "quota consistency"},
		MigrationProgress: "completed", TenantIntegritySummary: "all tenants bound",
		QuotaAccountingSummary: "usage totals match", CredentialRemediation: "reconnect required",
		RollbackPath: "restore_from_backup", Result: StatusPass,
		OperatorDiagnostics: []string{"daemon.migration.completed"},
	}
	assertValid(t, ValidateMigrationEvidence(report))

	report.PostflightChecks = nil
	assertInvalidContains(t, ValidateMigrationEvidence(report), "postflight")
}
