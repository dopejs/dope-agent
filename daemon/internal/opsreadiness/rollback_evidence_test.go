package opsreadiness

import "testing"

func TestRollbackDecisionRequiresBackupRestoreForIrreversibleState(t *testing.T) {
	assertValid(t, ValidateRollbackDecision(RollbackDecision{
		InPlaceSafe: false, PersistedStateReversible: false, BackupVerified: true,
		SelectedPath: "restore_from_backup", Reason: "schema changed",
	}))

	err := ValidateRollbackDecision(RollbackDecision{
		InPlaceSafe: false, PersistedStateReversible: false, BackupVerified: true,
		SelectedPath: "in_place", Reason: "schema changed",
	})
	assertInvalidContains(t, err, "restore_from_backup")
}
