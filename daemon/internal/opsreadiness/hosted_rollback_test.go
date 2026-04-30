package opsreadiness

import "testing"

func sampleHostedRollbackDecision() HostedRollbackDecisionRecord {
	return HostedRollbackDecisionRecord{
		RollbackDecisionID:      "rollback_hosted_1",
		RunID:                   sampleHostedRun().RunID,
		Trigger:                 "postflight_failed",
		Decision:                HostedRollbackRestoreFromBackupRequired,
		Rationale:               "migration is not safely reversible in place",
		RequiredBackupID:        "backup_hosted_1",
		SupportingEvidenceLinks: []string{"preflight.json", "postflight.json", "backup.json", "restore.json"},
		Operator:                "operator@example.test",
		DecidedAt:               hostedNow,
	}
}

func TestHostedRollbackDecisionStatesRecoveryPathAndEvidence(t *testing.T) {
	decision := sampleHostedRollbackDecision()
	assertValid(t, ValidateHostedRollbackDecision(sampleHostedRun(), decision))

	decision.Decision = "maybe"
	assertInvalidContains(t, ValidateHostedRollbackDecision(sampleHostedRun(), decision), "decision")

	decision = sampleHostedRollbackDecision()
	decision.SupportingEvidenceLinks = nil
	assertInvalidContains(t, ValidateHostedRollbackDecision(sampleHostedRun(), decision), "supporting")
}
