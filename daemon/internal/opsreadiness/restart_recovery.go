package opsreadiness

import "fmt"

func ValidateRestartRecovery(events []RestartEvent) error {
	if len(events) < MinimumRestartCount {
		return fmt.Errorf("soak requires at least %d daemon restarts", MinimumRestartCount)
	}
	for _, event := range events {
		if err := requireAllowed("restart classification", event.Classification, []string{
			ClassificationRecovered,
			ClassificationInterrupted,
			ClassificationRetried,
			ClassificationOperatorActionNeeded,
		}); err != nil {
			return err
		}
		if event.RecoveryTime > MaxRestartRecoveryElapsed {
			return fmt.Errorf("restart %s recovery %s exceeds %s", event.RestartID, event.RecoveryTime, MaxRestartRecoveryElapsed)
		}
	}
	return nil
}
