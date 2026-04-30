package opsreadiness

import "testing"

func sampleHostedObservationReport() HostedObservationReport {
	return HostedObservationReport{
		ObservationReportID:        "obs_hosted_1",
		RunID:                      sampleHostedRun().RunID,
		SampleWindow:               "2026-04-30T11:00:00Z/2026-04-30T12:00:00Z",
		DaemonHealth:               StatusPass,
		DatabaseSize:               HostedObservation{Value: "10MiB"},
		LogSize:                    HostedObservation{Value: "1MiB"},
		Memory:                     HostedObservation{Value: "100MiB"},
		Goroutines:                 HostedObservation{Value: "12"},
		FileDescriptors:            HostedObservation{Unsupported: true},
		QueueOrBacklog:             HostedObservation{Value: "0"},
		ConnectorHealth:            HostedObservation{Value: StatusPass},
		MCPHealth:                  HostedObservation{Value: StatusPass},
		IntegrationDiagnosticState: HostedObservation{Value: StatusPass},
		UnsupportedFields:          []string{"fileDescriptors"},
		MonotonicResourceGrowth:    false,
		FailureOwner:               "",
		GeneratedAt:                hostedNow,
	}
}

func TestHostedObservabilityRequiresFieldsOrUnsupportedMarkers(t *testing.T) {
	report := sampleHostedObservationReport()
	assertValid(t, ValidateHostedObservationReport(sampleHostedRun(), report))

	report.FileDescriptors = HostedObservation{}
	assertInvalidContains(t, ValidateHostedObservationReport(sampleHostedRun(), report), "fileDescriptors")

	report = sampleHostedObservationReport()
	report.DaemonHealth = StatusFail
	report.FailureOwner = FailureOwnerDaemon
	report.BlockingFindings = []string{"daemon health failed"}
	assertInvalidContains(t, ValidateHostedObservationReport(sampleHostedRun(), report), "blocking")
}
