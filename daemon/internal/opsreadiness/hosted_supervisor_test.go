package opsreadiness

import (
	"testing"
	"time"
)

func sampleSupervisorEvent(eventType string) HostedSupervisorEvent {
	event := HostedSupervisorEvent{
		EventID:         "supervisor_event_1",
		RunID:           sampleHostedRun().RunID,
		EventType:       eventType,
		RequestedBy:     "operator@example.test",
		StartedAt:       hostedNow.Add(-time.Minute),
		CompletedAt:     hostedNow,
		DaemonHealth:    StatusPass,
		RecoverySeconds: 60,
		Result:          HostedResultPassed,
		EvidencePath:    "~/.dope-test/artifacts/hosted_run_20260430/supervisor.json",
	}
	if eventType == HostedEventManualStop {
		event.RecoverySeconds = 0
	}
	return event
}

func TestHostedSupervisorLifecycleAndRecoveryThresholds(t *testing.T) {
	for _, eventType := range []string{HostedEventStart, HostedEventStop, HostedEventRestart, HostedEventStatus, HostedEventHealthCheck, HostedEventManualStop, HostedEventRebootRecovery} {
		assertValid(t, ValidateHostedSupervisorEvent(sampleHostedRun(), sampleSupervisorEvent(eventType)))
	}

	crash := sampleSupervisorEvent(HostedEventCrashDetected)
	crash.RecoverySeconds = int((5 * time.Minute / time.Second) + 1)
	crash.Result = HostedResultFailed
	crash.FailureOwner = FailureOwnerDaemon
	assertInvalidContains(t, ValidateHostedSupervisorEvent(sampleHostedRun(), crash), "5 minutes")

	manual := sampleSupervisorEvent(HostedEventManualStop)
	manual.RecoverySeconds = 30
	assertInvalidContains(t, ValidateHostedSupervisorEvent(sampleHostedRun(), manual), "manual stop")

	repeated := sampleSupervisorEvent(HostedEventRepeatedCrash)
	repeated.Result = HostedResultPassed
	assertInvalidContains(t, ValidateHostedSupervisorEvent(sampleHostedRun(), repeated), "repeated crash")
}
