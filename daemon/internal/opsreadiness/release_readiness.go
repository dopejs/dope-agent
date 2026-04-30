package opsreadiness

import "fmt"

func ValidateReleaseReadiness(evidence ReleaseReadinessEvidence) error {
	errs := []error{
		RequireElapsedAtMost("release readiness review", evidence.ReviewElapsed, MaxReleaseReviewElapsed),
		ValidateRealAccountSmoke(evidence.RealAccountSmoke),
	}
	required := map[string]bool{
		"install runbook":        evidence.InstallRunbookPassed,
		"upgrade runbook":        evidence.UpgradeRunbookPassed,
		"backup artifact":        evidence.BackupArtifactPassed,
		"restore verification":   evidence.RestoreVerificationPassed,
		"migration verification": evidence.MigrationVerificationPassed,
		"rollback guidance":      evidence.RollbackGuidancePresent,
		"soak report":            evidence.SoakReportPassed,
		"resource growth checks": evidence.ResourceGrowthChecksPassed,
		"credential redaction":   evidence.CredentialRedactionPassed,
		"fake-backend coverage":  evidence.FakeBackendCoveragePassed,
		"Roadmap 40 rerun gate":  evidence.Roadmap40RerunGatePresent,
		"Roadmap 41 rerun gate":  evidence.Roadmap41RerunGatePresent,
		"Roadmap 42 diagnostics": evidence.Roadmap42DiagnosticsPresent,
		"Roadmap 42 smoke":       evidence.Roadmap42SmokeEvidencePresent,
	}
	for label, ok := range required {
		if !ok {
			errs = append(errs, fmt.Errorf("%s is required for release readiness", label))
		}
	}
	if evidence.Decision != ResultShip && evidence.Decision != ResultShipWithRecordedSkips {
		errs = append(errs, fmt.Errorf("release decision must be ship or ship_with_recorded_skips when evidence passes"))
	}
	if evidence.Roadmap42SmokeEvidencePresent && len(evidence.DiagnosticSmokeReports) == 0 {
		errs = append(errs, fmt.Errorf("Roadmap 42 smoke evidence requires at least one diagnostic smoke report"))
	}
	return JoinErrors(errs...)
}
