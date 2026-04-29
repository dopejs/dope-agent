package opsreadiness

import "fmt"

func ValidateRollbackDecision(decision RollbackDecision) error {
	if err := RequireNonEmpty("rollback path", decision.SelectedPath); err != nil {
		return err
	}
	if !decision.PersistedStateReversible && decision.SelectedPath != "restore_from_backup" {
		return fmt.Errorf("irreversible persisted state must use restore_from_backup")
	}
	if decision.SelectedPath == "restore_from_backup" && !decision.BackupVerified {
		return fmt.Errorf("restore_from_backup requires a verified backup")
	}
	return RequireNonEmpty("rollback reason", decision.Reason)
}
