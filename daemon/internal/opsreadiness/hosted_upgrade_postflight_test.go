package opsreadiness

import "testing"

func sampleHostedUpgradePostflight() HostedUpgradeEvidence {
	run := sampleHostedRun()
	return HostedUpgradeEvidence{
		UpgradeEvidenceID:          "upgrade_postflight_1",
		RunID:                      run.RunID,
		Phase:                      HostedUpgradePhasePostflight,
		DaemonHealth:               StatusPass,
		TenantDataVerification:     StatusPass,
		MigrationState:             StatusPass,
		CredentialRemediationState: StatusPass,
		QuotaState:                 StatusPass,
		OperationalDiagnostics:     StatusPass,
		RollbackGuidance:           "restore_from_backup_required if postflight fails",
		GeneratedAt:                hostedNow,
	}
}

func TestHostedUpgradePostflightRequiresTenantDiagnosticsAndRollbackGuidance(t *testing.T) {
	evidence := sampleHostedUpgradePostflight()
	assertValid(t, ValidateHostedUpgradeEvidence(sampleHostedRun(), evidence))

	evidence.OperationalDiagnostics = StatusFail
	evidence.FailureOwner = FailureOwnerUnknown
	evidence.BlockingFindings = []string{"diagnostics failed"}
	assertInvalidContains(t, ValidateHostedUpgradeEvidence(sampleHostedRun(), evidence), "blocking")
}
