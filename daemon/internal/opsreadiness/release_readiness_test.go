package opsreadiness

import (
	"testing"
	"time"
)

func TestReleaseReadinessRequiresAllEvidenceAndThirtyMinuteReview(t *testing.T) {
	evidence := ReleaseReadinessEvidence{
		InstallRunbookPassed: true, UpgradeRunbookPassed: true, BackupArtifactPassed: true,
		RestoreVerificationPassed: true, MigrationVerificationPassed: true, RollbackGuidancePresent: true,
		SoakReportPassed: true, ResourceGrowthChecksPassed: true, CredentialRedactionPassed: true,
		FakeBackendCoveragePassed: true, Roadmap40RerunGatePresent: true, Roadmap41RerunGatePresent: true,
		Roadmap42DiagnosticsPresent: true, Roadmap42SmokeEvidencePresent: true,
		ReviewElapsed: 25 * time.Minute, Decision: ResultShipWithRecordedSkips,
		RealAccountSmoke: []RealAccountSmokeStatus{
			{Domain: "calendar", SafeCredentialsAvailable: false, SkipReason: "no safe account", FakeBackendCoveragePassing: true},
			{Domain: "mail", SafeCredentialsAvailable: false, SkipReason: "no safe account", FakeBackendCoveragePassing: true},
		},
		DiagnosticSmokeReports: []SmokeMatrixReport{{SmokeReportID: "smoke_r42", TenantID: "ten_r42", Status: SmokeReportCompleted}},
	}
	assertValid(t, ValidateReleaseReadiness(evidence))

	evidence.ReviewElapsed = 31 * time.Minute
	assertInvalidContains(t, ValidateReleaseReadiness(evidence), "exceeds")
}
