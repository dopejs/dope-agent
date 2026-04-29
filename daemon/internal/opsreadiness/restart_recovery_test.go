package opsreadiness

import (
	"testing"
	"time"
)

func TestRestartRecoveryRequiresThreeClassifiedFastRestarts(t *testing.T) {
	report := sampleSoakReport()
	assertValid(t, ValidateRestartRecovery(report.RestartEvents))

	report.RestartEvents[0].RecoveryTime = 6 * time.Minute
	assertInvalidContains(t, ValidateRestartRecovery(report.RestartEvents), "exceeds")
}
