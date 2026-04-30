package opsreadiness

import (
	"fmt"
	"strings"
)

func ValidateHostedObservationReport(run HostedRun, report HostedObservationReport) error {
	errs := []error{
		RequireNonEmpty("observation report id", report.ObservationReportID),
		requireHostedRunIdentity(run, report.RunID, "", ""),
		RequireNonEmpty("sample window", report.SampleWindow),
		RequireNonEmpty("daemon health", report.DaemonHealth),
		requireGeneratedAt("observation report", report.GeneratedAt),
		ValidateHostedRedaction("observation report", report),
		validateHostedObservation("databaseSize", report.DatabaseSize, report.UnsupportedFields),
		validateHostedObservation("logSize", report.LogSize, report.UnsupportedFields),
		validateHostedObservation("memory", report.Memory, report.UnsupportedFields),
		validateHostedObservation("goroutines", report.Goroutines, report.UnsupportedFields),
		validateHostedObservation("fileDescriptors", report.FileDescriptors, report.UnsupportedFields),
		validateHostedObservation("queueOrBacklog", report.QueueOrBacklog, report.UnsupportedFields),
		validateHostedObservation("connectorHealth", report.ConnectorHealth, report.UnsupportedFields),
		validateHostedObservation("mcpHealth", report.MCPHealth, report.UnsupportedFields),
		validateHostedObservation("integrationDiagnosticState", report.IntegrationDiagnosticState, report.UnsupportedFields),
	}
	if report.DaemonHealth != StatusPass {
		errs = append(errs, fmt.Errorf("observation report has blocking daemon health finding"))
	}
	if report.MonotonicResourceGrowth {
		errs = append(errs, fmt.Errorf("observation report has blocking resource growth finding"))
	}
	if strings.Contains(strings.ToLower(report.QueueOrBacklog.Value), "backlog") {
		errs = append(errs, fmt.Errorf("observation report has blocking backlog finding"))
	}
	if len(report.BlockingFindings) > 0 || report.DaemonHealth != StatusPass || report.MonotonicResourceGrowth {
		errs = append(errs, ValidateHostedFailureOwner(report.FailureOwner))
	}
	return JoinErrors(errs...)
}

func validateHostedObservation(label string, observation HostedObservation, unsupportedFields []string) error {
	if strings.TrimSpace(observation.Value) != "" {
		return nil
	}
	if observation.Unsupported && containsString(unsupportedFields, label) {
		return nil
	}
	return fmt.Errorf("%s is required or must be listed as unsupported", label)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
