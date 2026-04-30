package opsreadiness

import (
	"fmt"
	"time"
)

func ValidateHostedSupervisorEvent(run HostedRun, event HostedSupervisorEvent) error {
	errs := []error{
		RequireNonEmpty("event id", event.EventID),
		requireHostedRunIdentity(run, event.RunID, "", ""),
		requireAllowed("event type", event.EventType, []string{HostedEventStart, HostedEventStop, HostedEventRestart, HostedEventStatus, HostedEventHealthCheck, HostedEventCrashDetected, HostedEventRebootRecovery, HostedEventManualStop, HostedEventFailedRestart, HostedEventRepeatedCrash}),
		RequireNonEmpty("daemon health", event.DaemonHealth),
		requireAllowed("result", event.Result, []string{HostedResultPassed, HostedResultFailed, HostedResultBlocked, HostedResultUnsupported, HostedResultOperatorActionNeeded}),
		RequireNonEmpty("evidence path", event.EvidencePath),
		ValidateHostedRedaction("supervisor event", event),
	}
	if event.StartedAt.IsZero() {
		errs = append(errs, fmt.Errorf("event started at is required"))
	}
	if event.CompletedAt.IsZero() {
		errs = append(errs, fmt.Errorf("event completed at is required"))
	}
	if isOperatorInitiatedSupervisorEvent(event.EventType) {
		errs = append(errs, RequireNonEmpty("requested by", event.RequestedBy))
	}
	if event.Result != HostedResultPassed {
		errs = append(errs, ValidateHostedFailureOwner(event.FailureOwner))
	}
	switch event.EventType {
	case HostedEventCrashDetected, HostedEventRebootRecovery:
		if event.RecoverySeconds <= 0 {
			errs = append(errs, fmt.Errorf("%s recovery seconds are required", event.EventType))
		}
		if time.Duration(event.RecoverySeconds)*time.Second > MaxRestartRecoveryElapsed {
			errs = append(errs, fmt.Errorf("%s recovery exceeds 5 minutes", event.EventType))
		}
	case HostedEventManualStop:
		if event.RecoverySeconds != 0 {
			errs = append(errs, fmt.Errorf("manual stop must not be classified as crash recovery"))
		}
	case HostedEventRepeatedCrash:
		if event.Result == HostedResultPassed {
			errs = append(errs, fmt.Errorf("repeated crash must surface failed restart or operator action needed"))
		}
	case HostedEventFailedRestart:
		if event.Result == HostedResultPassed {
			errs = append(errs, fmt.Errorf("failed restart cannot pass"))
		}
	}
	return JoinErrors(errs...)
}

func isOperatorInitiatedSupervisorEvent(eventType string) bool {
	switch eventType {
	case HostedEventStart, HostedEventStop, HostedEventRestart, HostedEventStatus, HostedEventHealthCheck, HostedEventManualStop:
		return true
	default:
		return false
	}
}
